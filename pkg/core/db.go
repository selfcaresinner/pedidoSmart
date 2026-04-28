package core

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBWrapper es el centro neuralgico de datos en SolidBit,
// un envoltorio seguro alrededor del pool nativo 'pgx' que es altamente compatible con Supabase / Postgis.
type DBWrapper struct {
	Pool *pgxpool.Pool
}

// NewDBWrapper inicializa la conexión hacia Supabase con el resguardo de un patrón de reconexión (Resilience Retry).
//
// SOLIDBIT STANDARD: Incluye configuración óptima en el pool para no saturar 
// la concurrencia a nivel de base de datos sin antes intentarlo de nuevo.
func NewDBWrapper(ctx context.Context, connectionString string) (*DBWrapper, error) {
	var pool *pgxpool.Pool
	var err error

	// Parseo inicial de configuración.
	config, parseErr := pgxpool.ParseConfig(connectionString)
	if parseErr != nil {
		return nil, fmt.Errorf("[SolidBit BD] error analizando configuration URI: %w", parseErr)
	}

	// Ajustes de tuning para instancias cloud y concurrencia media-alta.
	config.MaxConns = 25
	config.MinConns = 3
	config.MaxConnIdleTime = time.Minute * 5

	// Estrategia "Retry Backoff" para lidiar con topología de red inestable en la nube o inicios fríos.
	for i := 0; i < 5; i++ {
		pool, err = pgxpool.NewWithConfig(ctx, config)
		if err == nil {
			// Hacer PING para confirmar que la BD contesta antes de exponerla al servicio general.
			pingErr := pool.Ping(ctx)
			if pingErr == nil {
				return &DBWrapper{Pool: pool}, nil
			}
			err = pingErr
		}
		// Espera progresiva antes del siguiente intento (1s, 2s, 3s...).
		time.Sleep(time.Second * time.Duration(i+1))
	}

	return nil, fmt.Errorf("[SolidBit BD] pánico en inicialización. Fallo tras 5 intentos hacia Supabase: %w", err)
}

// WithTransaction ejecuta cualquier serie de queries ('fn') encapsuladas dentro de modo Transaccional.
//
// SOLIDBIT STANDARD: Si el scope interior devuelve error o entra en 'panic()',
// esta función automáticamente emite un ROLLBACK previniendo bloqueos indeseados e integridad rota en Postgres.
// Si termina OK, garantiza el COMMIT.
func (db *DBWrapper) WithTransaction(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("error iniciando la solicitud transaccional en Supabase: %w", err)
	}

	// Rescue the flow in case of a Panic inside the transactional logic
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // Hacemos cleanup y re-lanzamos el panic para el middleware/handler
		}
	}()

	// Llamada a función del usuario con Tx inyectada
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx) // Invalidez de flujo. Revertimos toda la data.
		return err          // Bubble up. Regla de "Errors as Values"
	}

	// Confirmamos cambios en Supabase PGBouncer
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("imposible commitear la transacción: %w", err)
	}

	return nil
}

// Close ejecuta un drenaje correcto a las conexiones huérfanas en el pool.
func (db *DBWrapper) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}
