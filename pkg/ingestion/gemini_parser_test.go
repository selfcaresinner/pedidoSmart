package ingestion

import (
	"encoding/json"
	"testing"
)

func TestGeminiParser_OrderIntention(t *testing.T) {
	// Simulamos respuestas JSON de la IA (Mocking)
	mockJSON := `{
		"is_order": true,
		"delivery_address": "Av. Reforma 222, CDMX",
		"customer_name": "Carlos Slim",
		"items": "3 Tacos al Pastor, 1 Coca-Cola"
	}`

	var intention OrderIntention
	err := json.Unmarshal([]byte(mockJSON), &intention)
	if err != nil {
		t.Fatalf("Fallo parseando mock JSON de la IA: %v", err)
	}

	if !intention.IsOrder {
		t.Errorf("Expected IsOrder true")
	}
	if intention.DeliveryAddress != "Av. Reforma 222, CDMX" {
		t.Errorf("Fallo en dirección extraída: %s", intention.DeliveryAddress)
	}

	// Caso: Dirección incompleta o formato inesperado
	mockIncompleteJSON := `{
		"is_order": true,
		"delivery_address": "",
		"customer_name": "",
		"items": ""
	}`

	var incIntention OrderIntention
	err = json.Unmarshal([]byte(mockIncompleteJSON), &incIntention)
	if err != nil {
		t.Fatalf("Fallo parseando JSON de IA incompleto: %v", err)
	}

	if incIntention.DeliveryAddress != "" {
		t.Errorf("Se esperaba dirección vacía para manejar faltantes de datos")
	}
}
