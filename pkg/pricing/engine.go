package pricing

import (
	"context"
	"math"
)

// PricingEngine maneja las tarifas dinámicas
type PricingEngine struct {
	BasePrice  float64
	PricePerKM float64
	ServiceFee float64 // porcentaje (ej. 0.05 para 5%)
}

// NewPricingEngine inicializa el motor de precios (hardcoded temporalmente, se podría cargar de db o config)
func NewPricingEngine() *PricingEngine {
	return &PricingEngine{
		BasePrice:  25.00,
		PricePerKM: 8.00,
		ServiceFee: 0.05,
	}
}

// CalculateOrderTotal calcula el total y los centavos basados en distancia y precio de items (opcional)
func (p *PricingEngine) CalculateOrderTotal(ctx context.Context, distanceMeters int, itemsPrice float64) (float64, int64) {
	// Convertimos metros a km
	distanceKM := float64(distanceMeters) / 1000.0

	// Tarifa de entrega = Base + (KM * Tarifa/KM)
	deliveryFee := p.BasePrice + (distanceKM * p.PricePerKM)

	// Total antes de fee de servicio
	subTotal := itemsPrice + deliveryFee

	// Fee de servicio
	fee := subTotal * p.ServiceFee

	// Total final
	total := subTotal + fee

	// Stripe centavos (redondeo seguro)
	cents := int64(math.Round(total * 100))

	return total, cents
}
