package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config contiene los valores validados de las variables de entorno para usar en el ecosistema SolidBit.
type Config struct {
	DatabaseURL     string
	Port            string
	Environment     string
	MapsAPIKey      string
	StripeSecretKey     string
	StripeWebhookSecret string
	AppURL              string
	AdminPassword       string
	WhatsAppAccessToken   string
	WhatsAppPhoneNumberID string
	AdminPhone          string
}

// LoadConfig lee las variables de entorno, de '.env', de inyecciones del sistema (Docker/K8s/Railway) y valida las claves críticas.
//
// SOLIDBIT STANDARD: Cumple con la regla 'Fail-Fast'.
// Si falta una variable crítica detiene el proceso principal con 'panic()'.
func LoadConfig() *Config {
	// Intentamos cargar .env asumiendo un entorno local en la mayoría de los casos.
	// Ignoramos el error intencionalmente para entornos productivos donde un orquestador provee variables.
	_ = godotenv.Load()

	// Consolidación de variables validadas y predeterminadas (Fallbacks).
	cfg := &Config{
		DatabaseURL:     getEnvOrPanic("DATABASE_URL"),
		Port:            getEnvOrDefault("PORT", "8080"),
		Environment:     getEnvOrDefault("ENVIRONMENT", "development"),
		MapsAPIKey:      getEnvOrPanic("NEXT_PUBLIC_MAPS_API_KEY"),
		StripeSecretKey:     getEnvOrPanic("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: getEnvOrPanic("STRIPE_WEBHOOK_SECRET"),
		AppURL:              getEnvOrPanic("APP_URL"),
		AdminPassword:       getEnvOrPanic("ADMIN_PASSWORD"),
		WhatsAppAccessToken:   getEnvOrPanic("WHATSAPP_ACCESS_TOKEN"),
		WhatsAppPhoneNumberID: getEnvOrPanic("WHATSAPP_PHONE_NUMBER_ID"),
		AdminPhone:          getEnvOrPanic("ADMIN_PHONE"),
	}

	return cfg
}

// PreFlightCheck valida y loguea amigablemente el arranque y el cumplimiento del checklist de seguridad para DevOps.
func PreFlightCheck(cfg *Config) {
	fmt.Println("=========================================================")
	fmt.Println("🚀 SOLIDBIT PRE-FLIGHT CHECK DE SISTEMAS")
	fmt.Println("=========================================================")
	fmt.Println("✅ DATABASE_URL provisto")
	fmt.Println("✅ STRIPE_SECRET_KEY y STRIPE_WEBHOOK_SECRET provistos")
	fmt.Println("✅ WHATSAPP_ACCESS_TOKEN y WHATSAPP_PHONE_NUMBER_ID provistos")
	fmt.Println("✅ APP_URL y NEXT_PUBLIC_MAPS_API_KEY provistos")
	fmt.Println("✅ ADMIN_PHONE provisto")
	fmt.Printf("🔥 Entorno: %s | Puerto: %s\n", strings.ToUpper(cfg.Environment), cfg.Port)
	fmt.Println("=========================================================")
}

// getEnvOrPanic extrae una clave vital.
// Fuerza la caída inmediata del programa (panic) indicando al desarrollador / administrador exactamente qué falta.
func getEnvOrPanic(key string) string {
	val := os.Getenv(key)
	if strings.TrimSpace(val) == "" {
		panic(fmt.Sprintf("[CRÍTICO SOLIDBIT] Arranque abortado. Variable de entorno faltante o vacía: %s", key))
	}
	return val
}

// getEnvOrDefault extrae la clave y, en el caso de no estar definida, aplica nuestro valor por defecto estándar.
func getEnvOrDefault(key, fallback string) string {
	val := os.Getenv(key)
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}
