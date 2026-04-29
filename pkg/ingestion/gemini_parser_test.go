package ingestion

import (
	"encoding/json"
	"testing"
)

func TestGeminiParser_Inference(t *testing.T) {
	// Caso 1: Pedido Real
	mockOrderJSON := `{
		"intent": "order",
		"producto": "Pizza Pepperoni",
		"cantidad": 2,
		"direccion_aproximada": "Calle 10, Guaymas"
	}`

	var orderInference AIInference
	if err := json.Unmarshal([]byte(mockOrderJSON), &orderInference); err != nil {
		t.Fatalf("Fallo parseando mock de pedido: %v", err)
	}

	if orderInference.Intent != "order" {
		t.Errorf("Expected intent 'order', got %s", orderInference.Intent)
	}
	if orderInference.Producto != "Pizza Pepperoni" {
		t.Errorf("Producto incorrecto: %s", orderInference.Producto)
	}

	// Caso 2: Consulta (Query) sobre horarios
	mockQueryJSON := `{
		"intent": "query",
		"response_text": "Nuestro horario es de 12:00 PM a 10:00 PM todos los días."
	}`

	var queryInference AIInference
	if err := json.Unmarshal([]byte(mockQueryJSON), &queryInference); err != nil {
		t.Fatalf("Fallo parseando mock de consulta: %v", err)
	}

	if queryInference.Intent != "query" {
		t.Errorf("Expected intent 'query', got %s", queryInference.Intent)
	}
	if queryInference.ResponseText == "" {
		t.Error("Se esperaba una respuesta de texto para la consulta")
	}

	// Caso 3: Saludo (Chit-chat)
	mockChatJSON := `{
		"intent": "chit_chat",
		"response_text": "¡Hola! Soy el asistente de SolidBit, ¿en qué puedo ayudarte hoy?"
	}`

	var chatInference AIInference
	if err := json.Unmarshal([]byte(mockChatJSON), &chatInference); err != nil {
		t.Fatalf("Fallo parseando mock de chit-chat: %v", err)
	}

	if chatInference.Intent != "chit_chat" {
		t.Errorf("Expected intent 'chit_chat', got %s", chatInference.Intent)
	}
}
