package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"solidbit/pkg/core"
	"solidbit/pkg/dispatch"
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
}

func NewIngestionService(pool *core.WorkerPool, parser *AIParser, db *core.DBWrapper, dispatcher *dispatch.Dispatcher) *IngestionService {
	return &IngestionService{
		pool:       pool,
		parser:     parser,
		db:         db,
		dispatcher: dispatcher,
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
	// Extraemos temporalmente un Merchant para respetar la foreign key.
	var merchantID string
	err = s.db.Pool.QueryRow(ctx, "SELECT id FROM merchants LIMIT 1").Scan(&merchantID)
	if err != nil {
		log.Println("[Ingestion WARN] Tabla merchants vacía. Se salta el flujo de guardado y despacho de la db.")
		return nil
	}

	var orderID string
	insertQuery := `
		INSERT INTO orders (merchant_id, customer_name, customer_phone, delivery_location)
		VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography)
		RETURNING id
	`
	
	// Temporal Mock de coordenadas hasta iterar el API de Google Maps Geocoding
	testLon := -77.02824
	testLat := -12.04318

	err = s.db.Pool.QueryRow(ctx, insertQuery, merchantID, "Client IA", numEnvio, testLon, testLat).Scan(&orderID)
	if err != nil {
		return fmt.Errorf("fallo la persistencia del pedido: %w", err)
	}

	log.Printf("[Ingestion OK] Persistiendo Pedido UUID -> %s", orderID)

	// Activación Asíncrona (Background Pool) del Motor Geográfico PostGIS
	s.dispatcher.DispatchAsynchronous(orderID, testLon, testLat)

	return nil
}
