package payments

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"solidbit/pkg/core"
)

func TestHandleStripeWebhook_Mock(t *testing.T) {
	// Esta prueba valida que la estructura del webhook de Stripe requiera las firmas de seguridad correctas
	// y prueba el comportamiento del método ante un payload de simulación fallido.
	
	db := &core.DBWrapper{} // Mock simplificado
	wh := NewWebhookHandler(db, "whsec_test_secret_123")

	payload := []byte(`{"type": "checkout.session.completed", "data": {"object": {"metadata": {"order_id": "test-uuid"}}}}`)
	req := httptest.NewRequest("POST", "/webhook/stripe", bytes.NewReader(payload))
	// No agregamos la cabecera Stripe-Signature para causar intencionalmente un error de firma
	w := httptest.NewRecorder()

	wh.HandleStripeWebhook(w, req)

	res := w.Result()
	// Esperamos HTTP 400 Bad Request porque Fail-Fast y validación rechazan falta de firma
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("Se esperaba código HTTP 400 por falta de firma, pero se recibió: %d", res.StatusCode)
	}
}
