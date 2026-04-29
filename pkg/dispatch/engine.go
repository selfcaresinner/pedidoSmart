package dispatch

import (
	"context"
	"fmt"
	"log"

	"solidbit/pkg/core"
	"solidbit/pkg/routing"
)

// Dispatcher se encarga de la logística y el enrutamiento inteligente.
type Dispatcher struct {
	db      *core.DBWrapper
	pool    *core.WorkerPool
	routing *routing.RoutingClient
}

func NewDispatcher(db *core.DBWrapper, pool *core.WorkerPool, rc *routing.RoutingClient) *Dispatcher {
	return &Dispatcher{
		db:      db,
		pool:    pool,
		routing: rc,
	}
}

// AssignNearestDriver busca los repartidores más cercanos mediante el índice GiST de PostGIS en Supabase.
// La función sigue la regla SolidBit: Utiliza Error Wrapping y parámetros seguros ($1, $2) contra inyección SQL.
func (d *Dispatcher) AssignNearestDriver(ctx context.Context, orderID string, lon, lat float64) error {
	query := `
		SELECT driver_id, driver_name, distance_meters 
		FROM get_closest_available_drivers($1, $2, 5)
	`
	rows, err := d.db.Pool.Query(ctx, query, lon, lat)
	if err != nil {
		return fmt.Errorf("fallo la consulta espacial de repartidores: %w", err)
	}
	defer rows.Close()

	var closestDriverID string
	var closestDriverName string
	var closestDistance float64
	found := false

	// Extraemos el repartidor más cercano (el primero) según nuestro stored procedure SQL.
	if rows.Next() {
		if err := rows.Scan(&closestDriverID, &closestDriverName, &closestDistance); err != nil {
			return fmt.Errorf("error leyendo datos de PostGIS: %w", err)
		}
		found = true
	}

	if !found {
		// Log estructurado de advertencia (Lógica de Negocio), sin retornar error para no detener el worker brutalmente.
		log.Printf("[Motor de Despacho][WARN] No se encontraron repartidores disponibles para el Pedido [%s] en (%.4f, %.4f)", orderID, lon, lat)
		return nil
	}

	log.Printf("[Motor de Despacho][INFO] Pedido [%s] asignado a repartidor [%s] a [%.2f] metros libres", orderID, closestDriverName, closestDistance)

	// Persistir la re-asignación en la Base de Datos para materializar la ruta
	updateQuery := `
		UPDATE orders 
		SET driver_id = $1, status = 'assigned', updated_at = now() 
		WHERE id = $2
	`
	_, err = d.db.Pool.Exec(ctx, updateQuery, closestDriverID, orderID)
	if err != nil {
		return fmt.Errorf("fallo la confirmación de la asignación en Orders DB: %w", err)
	}

	// TRATAMIENTO DE OPTIMIZACIÓN LOGÍSTICA
	// Enviamos asíncronamente a optimizar la ruta del repartidor en caso de que tenga múltiples pedidos
	d.pool.Submit(func(subCtx context.Context) error {
		return d.OptimizeDriverRoute(subCtx, closestDriverID)
	})

	return nil
}

// OptimizeDriverRoute toma los pedidos activos del repartidor y los ordena para máxima eficiencia logística.
func (d *Dispatcher) OptimizeDriverRoute(ctx context.Context, driverID string) error {
	// 1. Obtener la ubicación actual del repartidor
	var dLat, dLon float64
	err := d.db.Pool.QueryRow(ctx, "SELECT ST_Y(current_location::geometry), ST_X(current_location::geometry) FROM drivers WHERE id = $1 AND current_location IS NOT NULL", driverID).Scan(&dLat, &dLon)
	if err != nil {
		return fmt.Errorf("fallo obteniendo driver location (driver no tiene ubicación activa todavía): %w", err)
	}

	// 2. Obtener todos los pedidos activos de este repartidor (assigned, picked_up)
	ordersQuery := `
		SELECT id, ST_Y(delivery_location::geometry), ST_X(delivery_location::geometry) 
		FROM orders 
		WHERE driver_id = $1 AND status IN ('assigned', 'picked_up') 
		ORDER BY created_at ASC
	`
	rows, err := d.db.Pool.Query(ctx, ordersQuery, driverID)
	if err != nil {
		return fmt.Errorf("fallo obteniendo orders de driver %s: %w", driverID, err)
	}
	defer rows.Close()

	var activeOrders []routing.OrderData
	for rows.Next() {
		var oID string
		var oLat, oLon float64
		if err := rows.Scan(&oID, &oLat, &oLon); err == nil {
			activeOrders = append(activeOrders, routing.OrderData{
				ID: oID,
				Loc: routing.Location{Lat: oLat, Lng: oLon},
			})
		}
	}

	if len(activeOrders) < 2 {
		return nil // No hay secuencia que optimizar
	}

	// 3. Ejecutar algoritmo de enrutamiento de Google (Caching Integrado)
	optimizedIds, err := d.routing.OptimizeSequence(ctx, routing.Location{Lat: dLat, Lng: dLon}, activeOrders)
	if err != nil {
		return fmt.Errorf("fallo en OptimizeSequence de Google API: %w", err)
	}

	// 4. Guardar atomicamente la prioridad en base de datos
	tx, err := d.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("fallo abriendo transaccion route: %w", err)
	}
	defer tx.Rollback(ctx)

	for i, oID := range optimizedIds {
		_, err = tx.Exec(ctx, "UPDATE orders SET delivery_sequence_priority = $1, updated_at = now() WHERE id = $2", i+1, oID)
		if err != nil {
			return fmt.Errorf("fallo actualizando sequence %s: %w", oID, err)
		}
	}
	
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("fallo commit transaccion sequence: %w", err)
	}

	log.Printf("[Motor Logístico] Secuencia optimizada para Driver [%s]: %v", driverID, optimizedIds)
	return nil
}

// DispatchAsynchronous encola la operación en el WorkerPool para un despache 100% asíncrono
func (d *Dispatcher) DispatchAsynchronous(orderID string, lon, lat float64) {

	d.pool.Submit(func(ctx context.Context) error {
		return d.AssignNearestDriver(ctx, orderID, lon, lat)
	})
}
