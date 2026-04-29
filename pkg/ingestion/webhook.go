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

	if inference.Intent != "order" {
		log.Printf("[Ingestion] Intención desconocida o no manejada: %s", inference.Intent)
		return nil
	}

	log.Printf("[Ingestion EXITO] -> Producto: %s | Qty: %d | Zona: %s",
		inference.Producto, inference.Cantidad, inference.DireccionAproximada)

	// Lógica segura de Base de Datos
	// Extraemos temporalmente un Merchant para respetar la foreign key e inyectar un fallback local.
	var merchantID string
	var merchLon, merchLat float64
	var merchantPhone string
	err = s.db.Pool.QueryRow(ctx, "SELECT id, ST_X(location::geometry), ST_Y(location::geometry), coalesce(merchant_phone, '') FROM merchants LIMIT 1").Scan(&merchantID, &merchLon, &merchLat, &merchantPhone)
	if err != nil {
		log.Println("[Ingestion WARN] Tabla merchants vacía. Se salta el flujo de guardado y despacho de la db.")
		return nil
	}

	// Geocodificación Real
	lat, lon, geoErr := s.geocoder.ResolveAddress(ctx, inference.DireccionAproximada)
	if geoErr != nil {
		// Fallback: usar la ubicación del Merchant para no perder la venta
		log.Printf("[Ingestion ERROR] Fallo al geocodificar '%s': %v. Haciendo fallback a ubicación de Merchant.", inference.DireccionAproximada, geoErr)
		lon = merchLon
		lat = merchLat
	} else {
		log.Printf("[Ingestion GEO] Geocodificación Exitosa -> Lat: %.5f, Lon: %.5f", lat, lon)
	}

	var orderID string
	itemsDesc := fmt.Sprintf("%dx %s", inference.Cantidad, inference.Producto)
	insertQuery := `
		INSERT INTO orders (merchant_id, customer_name, customer_phone, items_description, delivery_location, payment_method, payment_status)
		VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326)::geography, 'stripe', 'pending')
		RETURNING id
	`

	err = s.db.Pool.QueryRow(ctx, insertQuery, merchantID, "Client IA", numEnvio, itemsDesc, lon, lat).Scan(&orderID)
	if err != nil {
		return fmt.Errorf("fallo la persistencia del pedido: %w", err)
	}

	log.Printf("[Ingestion OK] Persistiendo Pedido UUID -> %s", orderID)

	// Cálculo Inteligente de Rutas y Precios
	// Obtenemos distancia real usando Google Routes API
	distanceMeters := 0
	if geoErr == nil {
		distanceMeters, err = s.routing.GetDistanceMeters(ctx, routing.Location{Lat: merchLat, Lng: merchLon}, routing.Location{Lat: lat, Lng: lon})
		if err != nil {
			log.Printf("[Motor Logístico WARN] No se pudo obtener distancia exacta: %v. Usando distancia fallback de 5km.", err)
			distanceMeters = 5000
		}
	} else {
		distanceMeters = 5000 // Fallback distance if precise geo failed
	}

	// Costo de los productos ficticio temporalmente (si no lo extrae la IA)
	itemsPrice := 100.00
	
	totalAmount, amountCents := s.pricing.CalculateOrderTotal(ctx, distanceMeters, itemsPrice)

	breakdown := map[string]interface{}{
		"items_price": itemsPrice,
		"distance_m":  distanceMeters,
		"base_price":  s.pricing.BasePrice,
		"price_km":    s.pricing.PricePerKM,
		"service_fee": s.pricing.ServiceFee,
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
		msg := fmt.Sprintf("✅ ¡Pedido recibido! Total a pagar: $%.2f. Un repartidor ha sido asignado y va en camino a recoger tu pedido.", totalAmount)
		if sendErr := s.metaClient.SendTextMessage(context.Background(), numEnvio, msg); sendErr != nil {
			log.Printf("[WhatsApp API OUTBOUND ERR] %v", sendErr)
		} else {
			log.Printf("[WhatsApp API OUTBOUND] Confirmación de pedido enviada a %s", numEnvio)
		}
	}()

	// Activación Asíncrona (Background Pool) del Motor Geográfico PostGIS
	log.Printf("[Ingestion] Despachando repartidor automáticamente para orden %s", orderID)
	s.dispatcher.DispatchAsynchronous(orderID, lon, lat)

	return nil
}

func (s *IngestionService) HandleDriverComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		OrderID          string `json:"order_id"`
		EvidenceURL      string `json:"delivery_evidence_url"`
		DriverID         string `json:"driver_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	
	// Actualizar la orden con la url de evidencia y marcar como entregada
	var customerPhone string
	var totalAmount float64
	var paymentMethod string
	query := `
		UPDATE orders 
		SET status = 'delivered', delivery_evidence_url = $1, updated_at = now() 
		WHERE id = $2 
		RETURNING customer_phone, total_amount, payment_method
	`
	err := s.db.Pool.QueryRow(ctx, query, reqBody.EvidenceURL, reqBody.OrderID).Scan(&customerPhone, &totalAmount, &paymentMethod)
	if err != nil {
		log.Printf("[Driver] Error completando orden %s: %v", reqBody.OrderID, err)
		http.Error(w, "Error completando orden", http.StatusInternalServerError)
		return
	}

	// Si fue pago en efectivo, actualizar la cartera del repartidor
	if paymentMethod == "cash" && reqBody.DriverID != "unknown" {
		walletQuery := `
			INSERT INTO driver_wallets (driver_id, cash_on_hand, updated_at)
			VALUES ($1, $2, now())
			ON CONFLICT (driver_id) 
			DO UPDATE SET cash_on_hand = driver_wallets.cash_on_hand + $2, updated_at = now()
		`
		_, err := s.db.Pool.Exec(ctx, walletQuery, reqBody.DriverID, totalAmount)
		if err != nil {
			log.Printf("[Driver WALLET ERR] No se pudo actualizar cartera para driver %s: %v", reqBody.DriverID, err)
		} else {
			log.Printf("[Driver WALLET OK] Cartera de %s incrementada en $%.2f", reqBody.DriverID, totalAmount)
		}
	}

	log.Printf("[Driver] Orden %s entregada por %s. Evidencia: %s", reqBody.OrderID, reqBody.DriverID, reqBody.EvidenceURL)

	// Notificar al cliente vía WhatsApp
	if customerPhone != "" {
		go func(phone, evidenceURL string) {
			msg := fmt.Sprintf("✅ ¡Tu pedido ha sido entregado! Puedes ver la evidencia fotográfica aquí: %s", evidenceURL)
			if sendErr := s.metaClient.SendTextMessage(context.Background(), phone, msg); sendErr != nil {
				log.Printf("[WhatsApp API OUTBOUND ERR] (Delivery Confirmation): %v", sendErr)
			}
		}(customerPhone, reqBody.EvidenceURL)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "delivered"})
}
