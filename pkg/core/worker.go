package core

import (
	"context"
	"log"
	"sync"
)

// Task es la firma genérica estricta. Toda función de concurrencia en SolidBit 
// debe ser envuelta con compatibilidad al contexto (Timeout/Cancellations).
type Task func(ctx context.Context) error

// WorkerPool es nuestra armadura de despachos. Evita sobrecargar la RAM o CPU 
// en escenarios de avalanchas (ej. ráfagas altas desde Webhooks de Meta por el Black Friday).
type WorkerPool struct {
	tasks       chan Task
	wg          sync.WaitGroup
	concurrency int
	AlertFunc   func(err interface{})
}

// NewWorkerPool emite una nueva instancia permitiéndole a la aplicación definir sus límites técnicos.
// 'concurrency' dictamina cuantos Goroutines concurrentes existirán, funcionando como semáforos.
// 'maxQueue' limita la profundidad del buzón en memoria RAM para prever ataques o fugas OOM (Out Of Memory).
func NewWorkerPool(concurrency int, maxQueue int) *WorkerPool {
	return &WorkerPool{
		// Buffered channel para retener tareas mientras los workers se liberan
		tasks:       make(chan Task, maxQueue),
		concurrency: concurrency,
	}
}

// Start inyecta la vida dentro del WorkerPool. Levanta N Goroutines dedicados e independientes.
func (wp *WorkerPool) Start(ctx context.Context) {
	for i := 0; i < wp.concurrency; i++ {
		wp.wg.Add(1)
		
		go func(workerID int) {
			defer wp.wg.Done()
			
			// Daemon central del worker escuchando su respectivo buzón
			for {
				select {
				case <-ctx.Done(): // Prevención de señales SIGINT o caídas gracefully.
					// SOLIDBIT STANDARD: Terminación limpia del subproceso (Graceful shutdown)
					log.Printf("[SolidBit Worker %d] Señal de terminación. Finalizando...\n", workerID)
					return

				case task, ok := <-wp.tasks:
					if !ok {
						log.Printf("[SolidBit Worker %d] Canal de tareas cerrado. Deteniendo...\n", workerID)
						return
					}
					
					// Ejecusión en jaula de seguridad.
					// Si hay un error, el logueo predeterminado notifica, pero no tumba al Daemon.
					func() {
						defer func() {
							if r := recover(); r != nil {
								log.Printf("[SolidBit Worker %d][PANIC] Recuperado de panic en tarea: %v\n", workerID, r)
								if wp.AlertFunc != nil {
									wp.AlertFunc(r)
								}
							}
						}()
						if err := task(ctx); err != nil {
							// Aplicación del Standard: En sistemas de alto rendimiento, el logging debe
							// capturar la traza pero NO romper el loop.
							log.Printf("[SolidBit Worker %d][ERROR] Fallo en la tarea: %v\n", workerID, err)
						}
					}()
				}
			}
		}(i)
	}
}

// Submit encola una nueva función de tarea asíncrona hacia el sistema de semáforos.
// ATENCIÓN: Si el 'maxQueue' se llena, esta función hará 'block' en el main goroutine impidiendo que el server sature. 
// Esto crea presión inversa natural (Backpressure).
func (wp *WorkerPool) Submit(task Task) {
	wp.tasks <- task
}

// Stop cierra suavemente la vía de recepción e inicia una pausa pasiva `Wait`
// a que todos los despachos actualmente en ejecución logren finalizar antes de que SolidBit apague finalmente.
func (wp *WorkerPool) Stop() {
	close(wp.tasks) // Previene nuevas inserciones
	wp.wg.Wait()    // Espera un fin completo y natural
}
