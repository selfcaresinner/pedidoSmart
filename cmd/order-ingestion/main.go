package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"solidbit/pkg/core"
	"solidbit/pkg/dispatch"
	"solidbit/pkg/geocoding"
	"solidbit/pkg/ingestion"
	"solidbit/pkg/notifications"
	"solidbit/pkg/payments"
	"solidbit/pkg/admin"
	"solidbit/pkg/routing"
	"solidbit/pkg/pricing"
	"solidbit/pkg/messenger"
)

func main() {
	log.Println("[SolidBit] Booting Microservice: Order Ingestion Engine")

	// 1. Cargamos config + Fail-Fast standard
	cfg := core.LoadConfig()
	core.PreFlightCheck(cfg)

	// Solicitud explícita para la IA
	geminiApiKey := os.Getenv("GEMINI_API_KEY")
	if geminiApiKey == "" {
		panic("[CRÍTICO SOLIDBIT] Arranque abortado. GEMINI_API_KEY requerido para motor de Ingestion.")
	}

	metaClient := messenger.NewMetaClient(cfg.WhatsAppAccessToken, cfg.WhatsAppPhoneNumberID)

	apiMonitor := core.NewApiMonitor(3, func(msg string) {
		metaClient.SendTextMessage(context.Background(), cfg.AdminPhone, msg)
	})

	aiParser := ingestion.NewAIParser(geminiApiKey, apiMonitor)

	geocoder := geocoding.NewClient(cfg.MapsAPIKey, apiMonitor)

	paymentsClient := payments.NewStripeClient(cfg.StripeSecretKey, cfg.AppURL, apiMonitor)

	// 2. Patrón de Resiliencia y Control de Tráfico RAM/CPU
	// 20 workers simultáneos (protege de ban en API gratuita IA) , Buffer de 1000 requests.
	workerPool := core.NewWorkerPool(20, 1000)
	
	workerPool.AlertFunc = func(err interface{}) {
		msg := fmt.Sprintf("🚨 ALERTA SOLIDBIT: Error crítico en el Worker de Ingesta. Detalles: %v", err)
		go metaClient.SendTextMessage(context.Background(), cfg.AdminPhone, msg)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerPool.Start(ctx)
	log.Println("[SolidBit WorkerPool] Escucha concurrente inicializada (Max: 20 goroutines)")

	// 3. Montaje Base de datos Transaccional y Dispatcher
	db, err := core.NewDBWrapper(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("No se pudo afianzar conexión persistente con Base de Datos: %v", err)
	}
	defer db.Close()

	routingClient := routing.NewRoutingClient(cfg.MapsAPIKey, apiMonitor)

	dispatcher := dispatch.NewDispatcher(db, workerPool, routingClient)
	pricingEngine := pricing.NewPricingEngine()

	// 4. Montura de Controladores HTTP
	healthMonitor := core.NewHealthMonitor(db, workerPool)
	http.HandleFunc("/health", healthMonitor.HandleHealthCheck)
	
	service := ingestion.NewIngestionService(workerPool, aiParser, db, dispatcher, geocoder, paymentsClient, routingClient, pricingEngine, metaClient, cfg.AppURL)
	http.HandleFunc("/webhook/meta/inbound", service.HandleMetaWebhook)
	http.HandleFunc("/api/merchant/confirm", service.HandleMerchantConfirm)

	paymentsWebhook := payments.NewWebhookHandler(db, cfg.StripeWebhookSecret)
	http.HandleFunc("/webhook/stripe", paymentsWebhook.HandleStripeWebhook)

	adminService := admin.NewAdminService(db, cfg.AdminPassword)
	http.HandleFunc("/admin/metrics", adminService.AuthMiddleware(adminService.GetGlobalMetrics))
	http.HandleFunc("/admin/live-map", adminService.AuthMiddleware(adminService.GetActiveLiveMap))

	// 5. Iniciar Monitor de Proximidad
	proximityMonitor := notifications.NewProximityMonitor(db, metaClient)
	proximityMonitor.Start(ctx)

	// 6. Servidor HTTP Asíncrono
	server := &http.Server{Addr: ":" + cfg.Port}
	go func() {
		log.Printf("[SolidBit] Order Ingestion Service listo y ejecutando -> Puerto :%s\n", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Fallo en servicio HTTP: %v", err)
		}
	}()

	// 5. Apagado Elegante (Graceful Shutdown Mechanism)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("\n[SolidBit] Señal térmica/apagado (SIGTERM/SIGINT) detectada. Vaciando procesos RAM de forma segura...")

	cancel() // Evita nueva captura, avisa a subrutinas atadas al context main abortar HTTP Calls infinitos
	workerPool.Stop() // Esperamos a que los Workers liberen los JSON parseos ya procesandose

	if err := server.Shutdown(context.Background()); err != nil {
		log.Printf("[SolidBit] Servidor forzado a apagar bruscamente: %v\n", err)
	}

	log.Println("[SolidBit] Servicio terminado correctamente.")
}
