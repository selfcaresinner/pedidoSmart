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

	"solidbit/pkg/core"
)

// AIInference es el contrato de salida JSON de la IA que incluye la intención detectada.
type AIInference struct {
	Intent              string `json:"intent"`               // order, query, chit_chat
	ResponseText        string `json:"response_text"`        // Respuesta amable si es query o chit_chat
	Producto            string `json:"producto"`             // Solo si es order
	Cantidad            int    `json:"cantidad"`             // Solo si es order
	DireccionAproximada string `json:"direccion_aproximada"` // Solo si es order
}

// AIParser comunica con Gemini.
// Optamos por un cliente HTTP nativo para evitar dependencias extra (Zero-Dep) y tener control absoluto sobre latencia y fallos.
type AIParser struct {
	APIKey  string
	Client  *http.Client
	monitor *core.ApiMonitor
}

// NewAIParser inicializa el analizador con un Timeout estricto (Fail-Fast)
func NewAIParser(apiKey string, monitor *core.ApiMonitor) *AIParser {
	return &AIParser{
		APIKey:  apiKey,
		monitor: monitor,
		Client: &http.Client{
			Timeout: 15 * time.Second, // Timeout para no acumular goroutines bloqueadas
		},
	}
}

// ParseOrderText procesa el texto puro del cliente y retorna nuestro modelo AIInference con la intención y datos extraídos.
func (p *AIParser) ParseOrderText(ctx context.Context, text string) (*AIInference, error) {
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
				{"text": `Eres el Cerebro Conversacional de SolidBit, un sistema de delivery inteligente.
Tu función es clasificar el mensaje del usuario y responder adecuadamente.

CONOCIMIENTO DE SOLIDBIT:
- Horarios: Todos los días de 12:00 PM a 10:00 PM.
- Cobertura: Operamos exclusivamente en Empalme y Guaymas, Sonora.
- Métodos de Pago: Aceptamos Tarjeta (vía Stripe) y Efectivo.

REGLAS DE RESPUESTA:
1. Retorna ÚNICAMENTE JSON válido.
2. Campos:
   - "intent": Enum ["order", "query", "chit_chat"].
   - "response_text": Tu respuesta amable al cliente (obligatoria en query y chit_chat).
   - "producto": Nombre del producto (solo si intent es "order").
   - "cantidad": Número (solo si intent es "order", default 1).
   - "direccion_aproximada": Dirección detectada (solo si intent es "order").

LOGICA DE INTENCIONES:
- "order": Cuando el usuario claramente quiere comprar/pedir algo específico.
- "query": Dudas sobre horarios, zona, precios o cómo funciona.
- "chit_chat": Saludos (Hola), despedidas (Gracias) o comentarios generales.`},
			},
		},
		"generationConfig": map[string]interface{}{
			"responseMimeType": "application/json",
			"temperature":      0.2, // Un poco más de temperatura para respuestas humanas en chat
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
		if p.monitor != nil { p.monitor.RecordError("Gemini API", err) }
		return nil, fmt.Errorf("falla de red comunicando con IA: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 500 {
		if p.monitor != nil { p.monitor.RecordError("Gemini API", fmt.Errorf("HTTP %d", res.StatusCode)) }
	} else if p.monitor != nil {
		p.monitor.RecordSuccess("Gemini API")
	}

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

	var inference AIInference
	if err := json.Unmarshal([]byte(jsonString), &inference); err != nil {
		return nil, fmt.Errorf("el modelo no generó la estructura JSON correcta: %w (Valor: %s)", err, jsonString)
	}

	return &inference, nil
}
