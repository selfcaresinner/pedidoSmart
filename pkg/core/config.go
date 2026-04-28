package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config contiene los valores validados de las variables de entorno para usar en el ecosistema SolidBit.
type Config struct {
	DatabaseURL string
	Port        string
	Environment string
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
		DatabaseURL: getEnvOrPanic("DATABASE_URL"),
		Port:        getEnvOrDefault("PORT", "8080"),
		Environment: getEnvOrDefault("ENVIRONMENT", "development"),
	}

	return cfg
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
