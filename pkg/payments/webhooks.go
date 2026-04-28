package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"solidbit/pkg/core"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/webhook"
)

// WebhookHandler coordina las peticiones de Stripe y actualiza el estado en DB.
type WebhookHandler struct {
	db                  *core.DBWrapper
	stripeWebhookSecret string
}

// NewWebhookHandler inicializa el handler con la BBDD y el secreto de webhook.
func NewWebhookHandler(db *core.DBWrapper, webhookSecret string) *WebhookHandler {
	return &WebhookHandler{
		db:                  db,
		stripeWebhookSecret: webhookSecret,
	}
}

// HandleStripeWebhook procesa notificaciones desde Stripe (e.g., pagos completados).
func (h *WebhookHandler) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	const MaxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[Stripe Webhook WARN] Error leyendo el body: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// Obtener la firma del header
	signatureHeader := r.Header.Get("Stripe-Signature")

	// Verificar la firma de Stripe
	event, err := webhook.ConstructEvent(payload, signatureHeader, h.stripeWebhookSecret)
	if err != nil {
		log.Printf("[Stripe Webhook SEC] Falló verificación de firma: %v", err)
		w.WriteHeader(http.StatusBadRequest) // Bad request
		return
	}

	// Unmarshal del cuerpo del evento según su tipo
	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		err := json.Unmarshal(event.Data.Raw, &session)
		if err != nil {
			log.Printf("[Stripe Webhook WARN] Error deserializando checkout.session: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		orderID, ok := session.Metadata["order_id"]
		if !ok || orderID == "" {
			log.Printf("[Stripe Webhook WARN] Checkout Session completada sin order_id en metadata (Session ID: %s)", session.ID)
			w.WriteHeader(http.StatusOK) // Return 200 to Stripe, but log it
			return
		}

		err = h.markOrderAsPaid(r.Context(), orderID)
		if err != nil {
			log.Printf("[Stripe Webhook ERR] Error marcando orden como pagada (Order ID: %s): %v", orderID, err)
			// Devolvemos 500 para que Stripe reintente.
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Printf("[Stripe Webhook OK] Pedido [%s] marcado como pagado exitosamente", orderID)

	default:
		log.Printf("[Stripe Webhook] Evento ignorado: %s", event.Type)
	}

	w.WriteHeader(http.StatusOK)
}

// markOrderAsPaid realiza la actualización atómica del estado del pedido.
func (h *WebhookHandler) markOrderAsPaid(ctx context.Context, orderID string) error {
	query := `
		UPDATE orders
		SET payment_status = 'paid', updated_at = now()
		WHERE id = $1 AND payment_status != 'paid'
	`
	tag, err := h.db.Pool.Exec(ctx, query, orderID)
	if err != nil {
		return fmt.Errorf("falló al ejecutar la consulta de update de pago: %w", err)
	}

	if tag.RowsAffected() == 0 {
		log.Printf("[Pagos INFO] El pedido [%s] no se actualizó (quizás ya estaba pagado o no existe).", orderID)
	}

	return nil
}
