package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type Location struct {
	Lat float64
	Lng float64
}

type OrderData struct {
	ID  string
	Loc Location
}

// RoutingClient se encarga de llamar a Google Routes/Directions API para optimizar secuencias
type RoutingClient struct {
	apiKey string
	http   *http.Client
	cache  sync.Map // caché simple en memoria
}

func NewRoutingClient(apiKey string) *RoutingClient {
	return &RoutingClient{
		apiKey: apiKey,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// generateCacheKey redondea coords ligeramente para evitar recálculos en movimientos menores
func generateCacheKey(drv Location, orders []OrderData) string {
	ids := make([]string, len(orders))
	for i, o := range orders {
		ids[i] = o.ID
	}
	sort.Strings(ids)
	return fmt.Sprintf("%.3f,%.3f|%s", drv.Lat, drv.Lng, strings.Join(ids, ","))
}

// OptimizeSequence devuelve la lista de IDs de pedidos en el orden más eficiente calculado
func (c *RoutingClient) OptimizeSequence(ctx context.Context, driverLoc Location, orders []OrderData) ([]string, error) {
	if len(orders) <= 1 {
		res := make([]string, len(orders))
		for i, o := range orders {
			res[i] = o.ID
		}
		return res, nil
	}

	cacheKey := generateCacheKey(driverLoc, orders)
	if val, ok := c.cache.Load(cacheKey); ok {
		return val.([]string), nil
	}

	// Formulamos la consulta a Directions API para TSP
	// Origen y destino en el driver, pasando por todos los pedidos como waypoints optimizables.
	origin := fmt.Sprintf("%f,%f", driverLoc.Lat, driverLoc.Lng)
	
	var waypoints []string
	for _, o := range orders {
		waypoints = append(waypoints, fmt.Sprintf("%f,%f", o.Loc.Lat, o.Loc.Lng))
	}

	reqURL, _ := url.Parse("https://maps.googleapis.com/maps/api/directions/json")
	q := reqURL.Query()
	q.Set("origin", origin)
	q.Set("destination", origin) // Loop cerrado
	q.Set("waypoints", "optimize:true|"+strings.Join(waypoints, "|"))
	q.Set("key", c.apiKey)
	reqURL.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error HTTP en Directions API: %w", err)
	}
	defer res.Body.Close()

	var apiResp struct {
		Status string `json:"status"`
		Routes []struct {
			WaypointOrder []int `json:"waypoint_order"`
		} `json:"routes"`
	}

	if err := json.NewDecoder(res.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("error deserializando respuesta Directions API: %w", err)
	}

	if apiResp.Status != "OK" || len(apiResp.Routes) == 0 {
		return nil, fmt.Errorf("fallo del API de rutas o no se encontraron rutas (Status: %s)", apiResp.Status)
	}

	waypointOrder := apiResp.Routes[0].WaypointOrder
	
	// Si falló el optimize, devolver el original
	if len(waypointOrder) != len(orders) {
		res := make([]string, len(orders))
		for i, o := range orders {
			res[i] = o.ID
		}
		return res, nil
	}

	orderedIds := make([]string, len(orders))
	for i, idx := range waypointOrder {
		orderedIds[i] = orders[idx].ID
	}

	c.cache.Store(cacheKey, orderedIds)

	return orderedIds, nil
}

// GetDistanceMeters consulta la API de Google Directions para obtener la ruta y la distancia entre dos puntos.
func (c *RoutingClient) GetDistanceMeters(ctx context.Context, origin, dest Location) (int, error) {
	reqURL, _ := url.Parse("https://maps.googleapis.com/maps/api/directions/json")
	q := reqURL.Query()
	q.Set("origin", fmt.Sprintf("%f,%f", origin.Lat, origin.Lng))
	q.Set("destination", fmt.Sprintf("%f,%f", dest.Lat, dest.Lng))
	q.Set("key", c.apiKey)
	reqURL.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	res, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error HTTP consultando distancia en Directions API: %w", err)
	}
	defer res.Body.Close()

	var apiResp struct {
		Status string `json:"status"`
		Routes []struct {
			Legs []struct {
				Distance struct {
					Value int `json:"value"` // en metros
				} `json:"distance"`
			} `json:"legs"`
		} `json:"routes"`
	}

	if err := json.NewDecoder(res.Body).Decode(&apiResp); err != nil {
		return 0, fmt.Errorf("error deserializando respuesta Directions API: %w", err)
	}

	if apiResp.Status != "OK" || len(apiResp.Routes) == 0 || len(apiResp.Routes[0].Legs) == 0 {
		return 0, fmt.Errorf("fallo del API o sin ruta (Status: %s)", apiResp.Status)
	}

	return apiResp.Routes[0].Legs[0].Distance.Value, nil
}
