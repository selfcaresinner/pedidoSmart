package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"solidbit/pkg/core"
)

// AdminService maneja el Business Intelligence de SolidBit.
type AdminService struct {
	db       *core.DBWrapper
	password string
}

func NewAdminService(db *core.DBWrapper, password string) *AdminService {
	return &AdminService{
		db:       db,
		password: password,
	}
}

// AuthMiddleware es un envoltorio super rápido para proteger la Torre de Control
func (s *AdminService) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pass := r.Header.Get("X-Admin-Password")
		if pass != s.password {
			log.Printf("[Admin API SEC] Intento de acceso denegado.")
			http.Error(w, "Unauthorized Focus", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// GetGlobalMetrics obtiene la sumatoria de ingresos de la Vista admin_metrics
func (s *AdminService) GetGlobalMetrics(w http.ResponseWriter, r *http.Request) {
	query := `SELECT total_transfers, total_cash, total_settled, net_profit, delivered_today FROM admin_metrics`
	var transfers, cash, settled, netProfit float64
	var delivered int
	err := s.db.Pool.QueryRow(r.Context(), query).Scan(&transfers, &cash, &settled, &netProfit, &delivered)
	if err != nil {
		log.Printf("[Admin API ERR] Fallo consultando admin_metrics: %v", err)
		http.Error(w, "Error interno en metricas", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_transfers": transfers,
		"total_cash":      cash,
		"total_settled":   settled,
		"net_profit":      netProfit,
		"delivered_today": delivered,
	})
}

// GetActiveLiveMap retorna los pedidos activos y los repartidores para renderizar el mapa
func (s *AdminService) GetActiveLiveMap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Buscar Pedidos Activos
	ordersQuery := `
		SELECT id, status, customer_name, ST_X(delivery_location::geometry) as lon, ST_Y(delivery_location::geometry) as lat 
		FROM orders 
		WHERE status IN ('assigned', 'picked_up')
	`
	rows, err := s.db.Pool.Query(ctx, ordersQuery)
	if err != nil {
		log.Printf("[Admin API ERR] Fallo consultando live map orders: %v", err)
		http.Error(w, "Error on fetching map points", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Location struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	}

	type OrderOutput struct {
		ID       string   `json:"id"`
		Status   string   `json:"status"`
		Customer string   `json:"customer_name"`
		Loc      Location `json:"location"`
	}

	var activeOrders []OrderOutput
	for rows.Next() {
		var o OrderOutput
		if err := rows.Scan(&o.ID, &o.Status, &o.Customer, &o.Loc.Lng, &o.Loc.Lat); err == nil {
			activeOrders = append(activeOrders, o)
		}
	}

	// Buscar Repartidores Online (o todos con current_location)
	driversQuery := `
		SELECT id, name, status, ST_X(current_location::geometry) as lon, ST_Y(current_location::geometry) as lat 
		FROM drivers 
		WHERE current_location IS NOT NULL
	`
	dRows, err := s.db.Pool.Query(ctx, driversQuery)
	if err != nil {
		log.Printf("[Admin API ERR] Fallo consultando live map drivers: %v", err)
		http.Error(w, "Error on fetching drivers points", http.StatusInternalServerError)
		return
	}
	defer dRows.Close()

	type DriverOutput struct {
		ID     string   `json:"id"`
		Name   string   `json:"name"`
		Status string   `json:"status"`
		Loc    Location `json:"location"`
	}

	var activeDrivers []DriverOutput
	for dRows.Next() {
		var dr DriverOutput
		if err := dRows.Scan(&dr.ID, &dr.Name, &dr.Status, &dr.Loc.Lng, &dr.Loc.Lat); err == nil {
			activeDrivers = append(activeDrivers, dr)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"active_orders":  activeOrders,
		"active_drivers": activeDrivers,
	})
}

func (s *AdminService) HandleSettleDriver(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DriverID string  `json:"driver_id"`
		Amount   float64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, "Amount must be positive", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	// 1. Verificar balance actual
	var currentBalance float64
	err = tx.QueryRow(ctx, "SELECT cash_on_hand FROM driver_wallets WHERE driver_id = $1", req.DriverID).Scan(&currentBalance)
	if err != nil {
		http.Error(w, "Driver wallet not found", http.StatusNotFound)
		return
	}

	if req.Amount > currentBalance {
		http.Error(w, "Amount exceeds current balance", http.StatusBadRequest)
		return
	}

	// 2. Restar del wallet
	_, err = tx.Exec(ctx, "UPDATE driver_wallets SET cash_on_hand = cash_on_hand - $1, last_liquidation_at = now(), updated_at = now() WHERE driver_id = $2", req.Amount, req.DriverID)
	if err != nil {
		http.Error(w, "Error updating wallet", http.StatusInternalServerError)
		return
	}

	// 3. Registrar liquidación
	_, err = tx.Exec(ctx, "INSERT INTO settlements (driver_id, amount, created_at) VALUES ($1, $2, now())", req.DriverID, req.Amount)
	if err != nil {
		http.Error(w, "Error creating settlement record", http.StatusInternalServerError)
		return
	}

	// 4. Auditoría de Transacción (Salida de efectivo)
	auditQuery := `
		INSERT INTO wallet_transactions (wallet_id, amount, transaction_type, description)
		VALUES ($1, $2, 'exit', 'Liquidación administrativa de efectivo')
	`
	_, _ = tx.Exec(ctx, auditQuery, req.DriverID, req.Amount)

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "Error committing transaction", http.StatusInternalServerError)
		return
	}

	log.Printf("[Admin Settle] Repartidor %s liquidado por $%.2f", req.DriverID, req.Amount)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "settled"})
}
