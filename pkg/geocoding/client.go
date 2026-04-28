package geocoding

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client encapsula la comunicación con la API de Geocodificación para convertir direcciones.
type Client struct {
	apiKey string
	http   *http.Client
}

// NewClient inicializa el cliente con el estándar de timeout estricto de SolidBit.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http: &http.Client{
			Timeout: 10 * time.Second, // Timeout para no bloquear a los Workers
		},
	}
}

// ResolveAddress transforma una dirección postal en coordenadas espaciales.
// Tiene sesgo (bias) para priorizar direcciones en Sonora (para evitar malentendidos internacionales).
func (c *Client) ResolveAddress(ctx context.Context, address string) (float64, float64, error) {
	if address == "" {
		return 0, 0, fmt.Errorf("dirección vacía no puede ser resuelta")
	}

	baseURL := "https://maps.googleapis.com/maps/api/geocode/json"
	reqURL, err := url.Parse(baseURL)
	if err != nil {
		return 0, 0, fmt.Errorf("error construyendo URL de geocodificación: %w", err)
	}

	// Parámetros de petición y sesgo geográfico
	q := reqURL.Query()
	q.Set("address", address)
	
	// Usamos `components` para sesgar resultados a México/Sonora
	q.Set("components", "country:MX|administrative_area:Sonora")
	
	// Opcionalmente podemos usar `bounds` si quisiéramos encerrar la búsqueda en el radio de Empalme/Guaymas.
	// q.Set("bounds", "27.84,-111.02|28.02,-110.74") // Bound box (sur-oeste|nor-este)

	q.Set("key", c.apiKey)
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return 0, 0, fmt.Errorf("error formateando request HTTP de mapas: %w", err)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("fallo de red comunicando con Google Maps API: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("respuesta fallida de Maps (status HTTP %d)", res.StatusCode)
	}

	var jsonResp struct {
		Status  string `json:"status"`
		Results []struct {
			Geometry struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`
		} `json:"results"`
	}

	if err := json.NewDecoder(res.Body).Decode(&jsonResp); err != nil {
		return 0, 0, fmt.Errorf("error decodificando respuesta JSON de mapas: %w", err)
	}

	if jsonResp.Status == "ZERO_RESULTS" || len(jsonResp.Results) == 0 {
		return 0, 0, fmt.Errorf("geocodificación falló: sin resultados para la dirección")
	}

	if jsonResp.Status != "OK" {
		return 0, 0, fmt.Errorf("error interno en Maps API status: '%s'", jsonResp.Status)
	}

	// Retorna latitud y longitud. (Nota: Muchas APIs usan el formato Y/X que corresponde a Lat/Lon)
	return jsonResp.Results[0].Geometry.Location.Lat, jsonResp.Results[0].Geometry.Location.Lng, nil
}
