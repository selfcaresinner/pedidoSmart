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
	"solidbit/pkg/payments"
	"solidbit/pkg/pricing"
	"solidbit/pkg/routing"
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
	payments   *payments.StripeClient
	routing    *routing.RoutingClient
	pricing    *pricing.PricingEngine
}

func NewIngestionService(pool *core.WorkerPool, parser *AIParser, db *core.DBWrapper, dispatcher *dispatch.Dispatcher, geocoder *geocoding.Client, paymentsClient *payments.StripeClient, routingClient *routing.RoutingClient, pricingEngine *pricing.PricingEngine) *IngestionService {
	return &IngestionService{
		pool:       pool,
		parser:     parser,
		db:         db,
		dispatcher: dispatcher,
		geocoder:   geocoder,
		payments:   paymentsClient,
		routing:    routingClient,
		pricing:    pricingEngine,
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

	orderData, err := s.parser.ParseOrderText(ctx, texto)
	if err != nil {
		return fmt.Errorf("error de extracción NLP mediante Gemini: %w", err)
	}

	log.Printf("[Ingestion EXITO] -> Producto: %s | Qty: %d | Zona: %s",
		orderData.Producto, orderData.Cantidad, orderData.DireccionAproximada)

	// Lógica segura de Base de Datos
	// Extraemos temporalmente un Merchant para respetar la foreign key e inyectar un fallback local.
	var merchantID string
	var merchLon, merchLat float64
	err = s.db.Pool.QueryRow(ctx, "SELECT id, ST_X(location::geometry), ST_Y(location::geometry) FROM merchants LIMIT 1").Scan(&merchantID, &merchLon, &merchLat)
	if err != nil {
		log.Println("[Ingestion WARN] Tabla merchants vacía. Se salta el flujo de guardado y despacho de la db.")
		return nil
	}

	// Geocodificación Real
	lat, lon, geoErr := s.geocoder.ResolveAddress(ctx, orderData.DireccionAproximada)
	if geoErr != nil {
		// Fallback: usar la ubicación del Merchant para no perder la venta
		log.Printf("[Ingestion ERROR] Fallo al geocodificar '%s': %v. Haciendo fallback a ubicación de Merchant.", orderData.DireccionAproximada, geoErr)
		lon = merchLon
		lat = merchLat
	} else {
		log.Printf("[Ingestion GEO] Geocodificación Exitosa -> Lat: %.5f, Lon: %.5f", lat, lon)
	}

	var orderID string
	insertQuery := `
		INSERT INTO orders (merchant_id, customer_name, customer_phone, delivery_location, payment_method, payment_status)
		VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography, 'stripe', 'pending')
		RETURNING id
	`

	err = s.db.Pool.QueryRow(ctx, insertQuery, merchantID, "Client IA", numEnvio, lon, lat).Scan(&orderID)
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

	// Crear Link de Pago de Stripe
	stripeLink, err := s.payments.CreatePaymentLink(ctx, orderID, amountCents)
	if err != nil {
		log.Printf("[Pagos WARN] No se pudo generar url de pago para Pedido [%s]: %v", orderID, err)
		// Solo actualizamos el total devuelto por Pricing
		_, _ = s.db.Pool.Exec(ctx, "UPDATE orders SET total_amount = $1, price_breakdown = $2 WHERE id = $3", totalAmount, breakdownJSON, orderID)
	} else {
		log.Printf("[Pagos] Link creado para Pedido [%s]: %s", orderID, stripeLink)
		// Actualizar la orden con el link y los importes
		_, err = s.db.Pool.Exec(ctx, "UPDATE orders SET stripe_link_url = $1, total_amount = $2, price_breakdown = $3 WHERE id = $4", stripeLink, totalAmount, breakdownJSON, orderID)
		if err != nil {
			log.Printf("[Pagos WARN] No se guardo metadata en la DB para Pedido [%s]: %v", orderID, err)
		}
	}

	// Activación Asíncrona (Background Pool) del Motor Geográfico PostGIS
	s.dispatcher.DispatchAsynchronous(orderID, lon, lat)

	return nil
}
