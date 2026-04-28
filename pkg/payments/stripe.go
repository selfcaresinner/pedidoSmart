package payments

import (
	"context"
	"fmt"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/checkout/session"
)

// StripeClient gestiona la integración oficial con la API de Stripe Payments
type StripeClient struct {
	secretKey string
	appURL    string
}

// NewStripeClient inicializa el cliente inyectando la llave secreta
func NewStripeClient(secretKey, appURL string) *StripeClient {
	// Initialize global Stripe configuration
	stripe.Key = secretKey

	return &StripeClient{
		secretKey: secretKey,
		appURL:    appURL,
	}
}

// CreatePaymentLink genera una sesión de Checkout en Stripe y devuelve la URL de pago.
// amount se espera en la moneda más baja (p.ej. centavos de MXN/USD, 10000 = $100.00)
func (s *StripeClient) CreatePaymentLink(ctx context.Context, orderID string, amount int64) (string, error) {
	// Parámetros obligatorios en Stripe y URLs de retorno (Callback) apuntando a APP_URL
	successURL := fmt.Sprintf("%s/order/%s/success?session_id={CHECKOUT_SESSION_ID}", s.appURL, orderID)
	cancelURL := fmt.Sprintf("%s/order/%s/cancel", s.appURL, orderID)

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("mxn"), // O la moneda de nuestra operativa
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(fmt.Sprintf("Pedido #%s", orderID)),
					},
					UnitAmount: stripe.Int64(amount),
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		ClientReferenceID: stripe.String(orderID),
		Metadata: map[string]string{
			"order_id": orderID,
		},
		Context: ctx, // Context passing to allow clean interruptions
	}

	// Ejecución asíncrona pero manejada internamente por la librería de Stripe
	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("fallo la generación del PaymentLink en Stripe: %w", err)
	}

	return sess.URL, nil
}
