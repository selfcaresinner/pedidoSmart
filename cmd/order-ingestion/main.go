package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"solidbit/pkg/core"
	"solidbit/pkg/ingestion"
)

func main() {
	log.Println("[SolidBit] Booting Microservice: Order Ingestion Engine")

	// 1. Cargamos config + Fail-Fast standard
	cfg := core.LoadConfig()

	// Solicitud explícita para la IA
	geminiApiKey := os.Getenv("GEMINI_API_KEY")
	if geminiApiKey == "" {
		panic("[CRÍTICO SOLIDBIT] Arranque abortado. GEMINI_API_KEY requerido para motor de Ingestion.")
	}

	aiParser := ingestion.NewAIParser(geminiApiKey)

	// 2. Patrón de Resiliencia y Control de Tráfico RAM/CPU
	// 20 workers simultáneos (protege de ban en API gratuita IA) , Buffer de 1000 requests.
	workerPool := core.NewWorkerPool(20, 1000)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerPool.Start(ctx)
	log.Println("[SolidBit WorkerPool] Escucha concurrente inicializada (Max: 20 goroutines)")

	// 3. Montura de Controlador HTTP
	service := ingestion.NewIngestionService(workerPool, aiParser)
	http.HandleFunc("/webhook/meta/inbound", service.HandleMetaWebhook)

	// 4. Servidor HTTP Asíncrono
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
