package notifications

import (
	"context"
	"log"
	"time"

	"solidbit/pkg/core"
)

// ProximityMonitor supervisa la ubicación de los repartidores y envía alertas de geofence.
type ProximityMonitor struct {
	db *core.DBWrapper
}

// NewProximityMonitor inicializa el monitor.
func NewProximityMonitor(db *core.DBWrapper) *ProximityMonitor {
	return &ProximityMonitor{
		db: db,
	}
}

// Start ejecuta el proceso de fondo (Ticker) en una goroutine.
func (m *ProximityMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	log.Println("[Proximity Monitor] Iniciado - escaneando cada 30 segundos")
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				log.Println("[Proximity Monitor] Detenido limpiamente.")
				return
			case <-ticker.C:
				m.checkProximity(ctx)
			}
		}
	}()
}

func (m *ProximityMonitor) checkProximity(ctx context.Context) {
	// Lógica Geoespacial: Consultamos la base de datos para pedidos en tránsito no notificados
	// y verificamos la distancia entre su punto de entrega y el repartidor.
	query := `
		SELECT o.id, o.customer_phone, ST_Distance(d.current_location, o.delivery_location) as distance_m
		FROM orders o
		JOIN drivers d ON o.driver_id = d.id
		WHERE o.status = 'picked_up' 
		  AND o.proximity_notified = FALSE 
		  AND d.current_location IS NOT NULL
	`

	rows, err := m.db.Pool.Query(ctx, query)
	if err != nil {
		log.Printf("[Proximity Monitor ERR] Error consultando ubicaciones para Geofencing: %v", err)
		return
	}
	defer rows.Close()

	type Alert struct {
		OrderID       string
		CustomerPhone string
		DistanceM     float64
	}

	var alerts []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.OrderID, &a.CustomerPhone, &a.DistanceM); err == nil {
			if a.DistanceM < 300.0 {
				alerts = append(alerts, a)
			}
		}
	}

	// 2. Procesamos las alertas encontradas
	for _, alert := range alerts {
		// Mock de integración con Meta API WhatsApp
		// En un entorno productivo se inyectaría un Http.Client hacia Meta.
		log.Printf("[WhatsApp API OUTBOUND] Enviando a %s: '¡Hola! Tu repartidor de SolidBit está a punto de llegar con tu pedido.' (Distancia actual: %.1f m)", alert.CustomerPhone, alert.DistanceM)

		// Actualizamos de forma atómica la DB para que no suene dos veces.
		updateQuery := `UPDATE orders SET proximity_notified = TRUE, updated_at = now() WHERE id = $1 AND proximity_notified = FALSE`
		tag, err := m.db.Pool.Exec(ctx, updateQuery, alert.OrderID)
		if err != nil {
			log.Printf("[Proximity Monitor ERR] Fallo confirmando alerta de proximidad en DB: %v", err)
			continue
		}

		if tag.RowsAffected() > 0 {
			log.Printf("[Proximity Monitor] Pedido [%s] marcado como proximity_notified exitosamente.", alert.OrderID)
		}
	}
}
