package pricing

import (
	"context"
	"testing"
)

func TestCalculateOrderTotal(t *testing.T) {
	engine := NewPricingEngine() // Base: 25, PerKM: 8, ServiceFee: 0.05

	tests := []struct {
		name           string
		distanceMeters int
		itemsPrice     float64
		expectedTotal  float64
		expectedCents  int64
	}{
		{
			name:           "Distancia corta (1km)",
			distanceMeters: 1000,
			itemsPrice:     100.0,
			// Base: 25, KM: 8 * 1 = 8 => Delivery = 33
			// Subtotal = 100 + 33 = 133
			// Fee = 133 * 0.05 = 6.65
			// Total = 139.65
			expectedTotal: 139.65,
			expectedCents: 13965,
		},
		{
			name:           "Larga distancia (15km)",
			distanceMeters: 15000,
			itemsPrice:     200.0,
			// Base: 25, KM: 8 * 15 = 120 => Delivery = 145
			// Subtotal = 200 + 145 = 345
			// Fee = 345 * 0.05 = 17.25
			// Total = 362.25
			expectedTotal: 362.25,
			expectedCents: 36225,
		},
		{
			name:           "Pedido de valor variable con centavos",
			distanceMeters: 3500,
			itemsPrice:     150.50,
			// DistanceKM = 3.5
			// Delivery = 25 + (3.5 * 8) = 53
			// Subtotal = 150.50 + 53 = 203.50
			// Fee = 203.50 * 0.05 = 10.175
			// Total = 213.675 -> Decimales y redondeo
			// Cents = 21368
			expectedTotal: 213.675,
			expectedCents: 21368,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, cents := engine.CalculateOrderTotal(context.Background(), tt.distanceMeters, tt.itemsPrice)
			if total != tt.expectedTotal {
				t.Errorf("Se esperaba un total de %.3f, pero se obtuvo %.3f", tt.expectedTotal, total)
			}
			if cents != tt.expectedCents {
				t.Errorf("Se esperaban %d centavos, pero se obtuvieron %d", tt.expectedCents, cents)
			}
		})
	}
}
