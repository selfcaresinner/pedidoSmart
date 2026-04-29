package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"solidbit/pkg/core"
	"solidbit/pkg/dispatch"
	"solidbit/pkg/geocoding"
	"solidbit/pkg/pricing"
	"solidbit/pkg/routing"
	"solidbit/pkg/messenger"
)

// MetaWebhook payload simplificado para extraer mensajes de WhatsApp Business API.
type MetaWebhook struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Messages []struct {
					From string `json:"from"`
					Text struct {
						Body string `json:"body"`
					} `json:"text"`
				} `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

// IngestionService coordina el Webhook, el Parseo mediante IA y la concurrencia segura del Worker Pool.
type IngestionService struct {
	pool       *core.WorkerPool
	parser     *AIParser
	db         *core.DBWrapper
	dispatcher *dispatch.Dispatcher
	geocoder   *geocoding.Client
	routing    *routing.RoutingClient
	pricing    *pricing.PricingEngine
	metaClient *messenger.MetaClient
	appURL     string
}

func NewIngestionService(pool *core.WorkerPool, parser *AIParser, db *core.DBWrapper, dispatcher *dispatch.Dispatcher, geocoder *geocoding.Client, routingClient *routing.RoutingClient, pricingEngine *pricing.PricingEngine, metaClient *messenger.MetaClient, appURL string) *IngestionService {
	return &IngestionService{
		pool:       pool,
		parser:     parser,
		db:         db,
		dispatcher: dispatcher,
		geocoder:   geocoder,
		routing:    routingClient,
		pricing:    pricingEngine,
		metaClient: metaClient,
		appURL:     appURL,
	}
}

// HandleMetaWebhook captura los postbacks emitidos por Meta.
// SOLIDBIT STANDARD: Retorna 200 OK inmediatamente (Time to Ack) y deposita el proceso pesado en RAM mediante WorkerPool.
func (s *IngestionService) HandleMetaWebhook(w http.ResponseWriter, r *http.Request) {
	// Permitir solo métodos POST de los servidores de Meta
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload MetaWebhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// "Fire and Forget" protegido
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			for _, msg := range change.Value.Messages {
				if msg.Text.Body == "" {
					continue
				}

				// Aislamos variables para evitar "Race Conditions" dentro del for loop closure
				textoCliente := msg.Text.Body
				numeroCliente := msg.From

				// Emitimos al buffer de workers
				s.pool.Submit(func(ctx context.Context) error {
					return s.processOrder(ctx, numeroCliente, textoCliente)
				})
			}
		}
	}

	// Requisito de las APIs de Meta: Si tardamos más de unos segundos nos marcarán webhook fallido
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ACK"))
}

// processOrder funciona desacoplado en una subrutina propia e individual (Background Process)
func (s *IngestionService) processOrder(ctx context.Context, numEnvio, texto string) error {
	log.Printf("[Ingestion] Iniciando Inferencia IA para SMS de %s", numEnvio)

	inference, err := s.parser.ParseOrderText(ctx, texto)
	if err != nil {
		return fmt.Errorf("error de extracción NLP mediante Gemini: %w", err)
	}

	// ENRUTADOR DE INTENCIONES
	if inference.Intent == "query" || inference.Intent == "chit_chat" {
		log.Printf("[Ingestion Conversational] Intención: %s. Enviando respuesta: %s", inference.Intent, inference.ResponseText)
		if sendErr := s.metaClient.SendTextMessage(ctx, numEnvio, inference.ResponseText); sendErr != nil {
			log.Printf("[WhatsApp API OUTBOUND ERR] %v", sendErr)
		}
		return nil
	}

	if inference.Intent == "cancel" {
		log.Printf("[Ingestion] Intención: cancel. Procesando para %s", numEnvio)
		// Buscar el último pedido activo del cliente
		var orderID string
		var driverID string
		query := `
			UPDATE orders 
			SET status = 'cancelled', updated_at = now() 
			WHERE id = (
				SELECT id FROM orders 
				WHERE customer_phone = $1 AND status IN ('pending', 'assigned', 'picked_up')
				ORDER BY created_at DESC 
				LIMIT 1
			)
			RETURNING id, COALESCE(driver_id::text, '')
		`
		err := s.db.Pool.QueryRow(ctx, query, numEnvio).Scan(&orderID, &driverID)
		if err != nil {
			log.Printf("[Ingestion Cancel] No se encontró pedido activo para %s", numEnvio)
			s.metaClient.SendTextMessage(ctx, numEnvio, "No encontré un pedido activo tuyo para cancelar. Si crees que hay un error, contacta a soporte.")
			return nil
		}

		// Si el repartidor estaba asignado, liberarlo
		if driverID != "" && driverID != "" {
			_, _ = s.db.Pool.Exec(ctx, "UPDATE drivers SET status = 'available' WHERE id = $1", driverID)
		}

		log.Printf("[Ingestion Cancel] Pedido %s cancelado exitosamente", orderID)
		s.metaClient.SendTextMessage(ctx, numEnvio, "✅ Tu pedido ha sido cancelado correctamente. ¡Esperamos servirte pronto!")
		return nil
	}

	if inference.Intent != "order" {
		log.Printf("[Ingestion] Intención desconocida o no manejada: %s", inference.Intent)
		return nil
	}

	log.Printf("[Ingestion EXITO] -> Producto: %s | Qty: %d | Recolección: %s | Entrega: %s",
		inference.Producto, inference.Cantidad, inference.PuntoRecoleccion, inference.PuntoEntrega)

	// Lógica segura de Base de Datos
	// Extraemos temporalmente un Merchant para respetar la foreign key e inyectar un fallback local.
	var merchantID string
	err = s.db.Pool.QueryRow(ctx, "SELECT id FROM merchants LIMIT 1").Scan(&merchantID)
	if err != nil {
		log.Println("[Ingestion WARN] Tabla merchants vacía. Se requiere al menos uno para persistencia.")
		return nil
	}

	// Geocodificación de Origen y Destino
	latOrig, lonOrig, geoErrOrig := s.geocoder.ResolveAddress(ctx, inference.PuntoRecoleccion)
	latDest, lonDest, geoErrDest := s.geocoder.ResolveAddress(ctx, inference.PuntoEntrega)

	if geoErrDest != nil {
		log.Printf("[Ingestion ERROR] Fallo al geocodificar entrega '%s': %v.", inference.PuntoEntrega, geoErrDest)
		s.metaClient.SendTextMessage(ctx, numEnvio, "❌ Lo siento, no logré ubicar tu dirección de entrega. ¿Podrías ser más específico?")
		return nil
	}

	if geoErrOrig != nil {
		log.Printf("[Ingestion WARN] Fallo al geocodificar recolección '%s'. Usando ubicación de merchant.", inference.PuntoRecoleccion)
		// Fallback to merchant location if source fails
		var merchLon, merchLat float64
		_ = s.db.Pool.QueryRow(ctx, "SELECT ST_X(location::geometry), ST_Y(location::geometry) FROM merchants WHERE id = $1", merchantID).Scan(&merchLon, &merchLat)
		lonOrig = merchLon
		latOrig = merchLat
	}

	var orderID string
	itemsDesc := fmt.Sprintf("%dx %s", inference.Cantidad, inference.Producto)
	insertQuery := `
		INSERT INTO orders (merchant_id, customer_name, customer_phone, items_description, delivery_location, payment_method, payment_status)
		VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326)::geography, 'cash', 'pending')
		RETURNING id
	`

	err = s.db.Pool.QueryRow(ctx, insertQuery, merchantID, "Cliente WhatsApp", numEnvio, itemsDesc, lonDest, latDest).Scan(&orderID)
	if err != nil {
		return fmt.Errorf("fallo la persistencia del pedido: %w", err)
	}

	log.Printf("[Ingestion OK] Persistiendo Pedido UUID -> %s", orderID)

	// Cálculo Inteligente de Rutas y Precios
	// Obtenemos distancia real usando Google Routes API
	distanceMeters, err := s.routing.GetDistanceMeters(ctx, routing.Location{Lat: latOrig, Lng: lonOrig}, routing.Location{Lat: latDest, Lng: lonDest})
	if err != nil {
		log.Printf("[Motor Logístico WARN] No se pudo obtener distancia exacta: %v. Usando distancia fallback de 5km.", err)
		distanceMeters = 5000
	}

	// Costo de los productos ficticio temporalmente (si no lo extrae la IA)
	itemsPrice := 0.00
	
	totalAmount, _ := s.pricing.CalculateOrderTotal(ctx, distanceMeters, itemsPrice)

	breakdown := map[string]interface{}{
		"items_price": itemsPrice,
		"distance_m":  distanceMeters,
		"base_price":  s.pricing.BasePrice,
		"price_km":    s.pricing.PricePerKM,
		"service_fee": s.pricing.ServiceFee,
		"recoleccion": inference.PuntoRecoleccion,
		"entrega":     inference.PuntoEntrega,
	}
	breakdownJSON, _ := json.Marshal(breakdown)

	log.Printf("[Pricing] Pedido %s: Distancia %dm, Total Calculado: $%.2f MXN", orderID, distanceMeters, totalAmount)

	// Actualizar la orden con los importes
	_, err = s.db.Pool.Exec(ctx, "UPDATE orders SET total_amount = $1, price_breakdown = $2 WHERE id = $3", totalAmount, breakdownJSON, orderID)
	if err != nil {
		log.Printf("[Pricing WARN] No se guardo metadata en la DB para Pedido [%s]: %v", orderID, err)
	}

	// Enviar WhatsApp al cliente
	go func() {
		msg := fmt.Sprintf("✅ ¡Pedido recibido! Total estimado: $%.2f. Un repartidor ha sido asignado y va en camino a recoger tu pedido en '%s'.", totalAmount, inference.PuntoRecoleccion)
		if sendErr := s.metaClient.SendTextMessage(context.Background(), numEnvio, msg); sendErr != nil {
			log.Printf("[WhatsApp API OUTBOUND ERR] %v", sendErr)
		}
	}()

	// Activación Asíncrona (Background Pool) del Motor Geográfico PostGIS
	log.Printf("[Ingestion] Despachando repartidor automáticamente para orden %s", orderID)
	s.dispatcher.DispatchAsynchronous(orderID, lonDest, latDest)

	return nil
}

func (s *IngestionService) HandleOrderStatusUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		OrderID string `json:"order_id"`
		Status  string `json:"status"` // picked_up, cancelled, etc.
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var customerPhone string
	err := s.db.Pool.QueryRow(ctx, "UPDATE orders SET status = $1, updated_at = now() WHERE id = $2 RETURNING customer_phone", reqBody.Status, reqBody.OrderID).Scan(&customerPhone)
	if err != nil {
		log.Printf("[StatusUpdate] Error actualizando orden %s a %s: %v", reqBody.OrderID, reqBody.Status, err)
		http.Error(w, "Error actualizando orden", http.StatusInternalServerError)
		return
	}

	if reqBody.Status == "picked_up" && customerPhone != "" {
		go func(phone string) {
			msg := "🛵 ¡Tu repartidor ya tiene tu pedido! Va en camino a tu ubicación."
			s.metaClient.SendTextMessage(context.Background(), phone, msg)
		}(customerPhone)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": reqBody.Status})
}

func (s *IngestionService) HandleDriverComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		OrderID     string `json:"order_id"`
		EvidenceURL string `json:"delivery_evidence_url"`
		DriverID    string `json:"driver_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	// Validación estricta de evidencia
	if len(reqBody.EvidenceURL) < 10 || reqBody.EvidenceURL == "" {
		http.Error(w, "Evidence URL is required and must be valid", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	// Actualizar la orden
	var customerPhone string
	var totalAmount float64
	var paymentMethod string
	query := `
		UPDATE orders 
		SET status = 'delivered', delivery_evidence_url = $1, updated_at = now() 
		WHERE id = $2 
		RETURNING customer_phone, total_amount, payment_method
	`
	err = tx.QueryRow(ctx, query, reqBody.EvidenceURL, reqBody.OrderID).Scan(&customerPhone, &totalAmount, &paymentMethod)
	if err != nil {
		log.Printf("[Driver] Error completando orden %s: %v", reqBody.OrderID, err)
		http.Error(w, "Error completando orden", http.StatusInternalServerError)
		return
	}

	// Actualizar Cartera y Auditoría si es efectivo
	if paymentMethod == "cash" && reqBody.DriverID != "unknown" {
		walletQuery := `
			INSERT INTO driver_wallets (driver_id, cash_on_hand, updated_at)
			VALUES ($1, $2, now())
			ON CONFLICT (driver_id) 
			DO UPDATE SET cash_on_hand = driver_wallets.cash_on_hand + $2, updated_at = now()
		`
		_, err := tx.Exec(ctx, walletQuery, reqBody.DriverID, totalAmount)
		if err != nil {
			log.Printf("[Driver WALLET ERR] No se pudo actualizar cartera para driver %s: %v", reqBody.DriverID, err)
		} else {
			// Auditoría de Transacción
			auditQuery := `
				INSERT INTO wallet_transactions (wallet_id, order_id, amount, transaction_type, description)
				VALUES ($1, $2, $3, 'entry', 'Cobro de pedido entregado')
			`
			tx.Exec(ctx, auditQuery, reqBody.DriverID, reqBody.OrderID, totalAmount)
			log.Printf("[Driver WALLET OK] Cartera de %s incrementada en $%.2f", reqBody.DriverID, totalAmount)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "Error finalizando transacción", http.StatusInternalServerError)
		return
	}

	log.Printf("[Driver] Orden %s entregada. Evidencia: %s", reqBody.OrderID, reqBody.EvidenceURL)

	// Notificar al cliente vía WhatsApp
	if customerPhone != "" {
		go func(phone, evidenceURL string) {
			msg := fmt.Sprintf("✅ ¡Tu pedido ha sido entregado! Puedes ver la evidencia fotográfica aquí: %s", evidenceURL)
			s.metaClient.SendTextMessage(context.Background(), phone, msg)
		}(customerPhone, reqBody.EvidenceURL)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "delivered"})
}
