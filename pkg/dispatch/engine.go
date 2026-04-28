package dispatch

import (
	"context"
	"fmt"
	"log"

	"solidbit/pkg/core"
)

// Dispatcher se encarga de la logística y el enrutamiento inteligente.
type Dispatcher struct {
	db   *core.DBWrapper
	pool *core.WorkerPool
}

func NewDispatcher(db *core.DBWrapper, pool *core.WorkerPool) *Dispatcher {
	return &Dispatcher{
		db:   db,
		pool: pool,
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

	return nil
}

// DispatchAsynchronous encola la operación en el WorkerPool para un despache 100% asíncrono
func (d *Dispatcher) DispatchAsynchronous(orderID string, lon, lat float64) {
	d.pool.Submit(func(ctx context.Context) error {
		return d.AssignNearestDriver(ctx, orderID, lon, lat)
	})
}
