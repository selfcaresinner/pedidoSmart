package core

import (
	"encoding/json"
	"net/http"
)

type HealthResponse struct {
	Status     string `json:"status"`
	DB         string `json:"db"`
	WorkerPool string `json:"worker_pool"`
}

type HealthMonitor struct {
	db   *DBWrapper
	pool *WorkerPool
}

func NewHealthMonitor(db *DBWrapper, pool *WorkerPool) *HealthMonitor {
	return &HealthMonitor{
		db:   db,
		pool: pool,
	}
}

func (m *HealthMonitor) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := HealthResponse{
		Status:     "ok",
		DB:         "ok",
		WorkerPool: "ok",
	}

	// Verificar DB
	if err := m.db.Pool.Ping(r.Context()); err != nil {
		response.DB = "error: " + err.Error()
		response.Status = "degraded"
	}

	// Verificar WorkerPool
	// Consideramos de momento que si existe está ok. 
	// Aunque no haya un métoda de "ping" en WorkerPool, podemos verificar que no sea nil.
	if m.pool == nil {
		response.WorkerPool = "error: not initialized"
		response.Status = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	if response.Status == "ok" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	json.NewEncoder(w).Encode(response)
}
