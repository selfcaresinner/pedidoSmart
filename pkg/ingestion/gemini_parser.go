package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OrderData es el contrato de salida JSON de la IA.
type OrderData struct {
	Producto            string `json:"producto"`
	Cantidad            int    `json:"cantidad"`
	DireccionAproximada string `json:"direccion_aproximada"`
}

// AIParser comunica con Gemini.
// Optamos por un cliente HTTP nativo para evitar dependencias extra (Zero-Dep) y tener control absoluto sobre latencia y fallos.
type AIParser struct {
	APIKey string
	Client *http.Client
}

// NewAIParser inicializa el analizador con un Timeout estricto (Fail-Fast)
func NewAIParser(apiKey string) *AIParser {
	return &AIParser{
		APIKey: apiKey,
		Client: &http.Client{
			Timeout: 15 * time.Second, // Timeout para no acumular goroutines bloqueadas
		},
	}
}

// ParseOrderText procesa el texto puro del cliente y retorna nuestro modelo OrderData con JSON extraído.
func (p *AIParser) ParseOrderText(ctx context.Context, text string) (*OrderData, error) {
	// Gemini 1.5 Flash minimiza latencia y costo en extracciones.
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", p.APIKey)

	// Payload que acata el requerimiento estricto de Gemini para dictarle JSON nativo.
	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": text},
				},
			},
		},
		"systemInstruction": map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": `Eres el Procesador de Lenguaje para un sistema de delivery logístico (SolidBit).
Tu ÚNICA función es extraer información de pedidos hacia JSON estructurado.
REGLAS ESTRICTAS:
1. Retorna ÚNICAMENTE JSON válido. No uses bloques markdown.
2. Estructura requerida: {"producto": "string", "cantidad": number, "direccion_aproximada": "string"}.
3. Si en el texto no hallas la cantidad, asume 1.
4. Si la dirección no se especifica, envia "".`},
			},
		},
		"generationConfig": map[string]interface{}{
			"responseMimeType": "application/json",
			"temperature":      0.1, // Temperatura baja: precisión analítica
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error serializando payload hacia Gemini: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("error construyendo request a Gemini: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falla de red comunicando con IA: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("Gemini API falló con HTTP %d: %s", res.StatusCode, string(respBody))
	}

	var aiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(res.Body).Decode(&aiResp); err != nil {
		return nil, fmt.Errorf("error mapeando raíz JSON de Gemini: %w", err)
	}

	if len(aiResp.Candidates) == 0 || len(aiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("la respuesta de la inteligencia artificial llegó vacía")
	}

	jsonString := aiResp.Candidates[0].Content.Parts[0].Text
	jsonString = strings.TrimSpace(jsonString)

	var order OrderData
	if err := json.Unmarshal([]byte(jsonString), &order); err != nil {
		return nil, fmt.Errorf("el modelo no generó la estructura JSON correcta: %w (Valor: %s)", err, jsonString)
	}

	return &order, nil
}
