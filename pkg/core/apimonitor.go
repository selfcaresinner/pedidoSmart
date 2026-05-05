package core

import (
	"fmt"
	"log"
	"sync"
)

type ApiMonitor struct {
	mu          sync.Mutex
	failures    map[string]int
	alertFunc   func(msg string)
	threshold   int
}

func NewApiMonitor(threshold int, alertFunc func(msg string)) *ApiMonitor {
	return &ApiMonitor{
		failures:  make(map[string]int),
		alertFunc: alertFunc,
		threshold: threshold,
	}
}

func (m *ApiMonitor) RecordError(apiName string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.failures[apiName]++
	count := m.failures[apiName]
	
	log.Printf("[API ERROR] %s falló: %v (Fallo %d/%d)", apiName, err, count, m.threshold)

	if count >= m.threshold {
		msg := fmt.Sprintf("🚨 ALERTA SOLIDBIT: %s experimenta %d fallos seguidos. Último error: %v", apiName, count, err)
		log.Println(msg)
		if m.alertFunc != nil {
			go m.alertFunc(msg)
		}
		// Reseteamos o no? Mejor resetear para evitar spam cada vez, o dejar que vuelva a crecer.
		m.failures[apiName] = 0
	}
}

func (m *ApiMonitor) RecordSuccess(apiName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.failures[apiName] > 0 {
		m.failures[apiName] = 0
		log.Printf("[API RECOVERY] %s ha vuelto a funcionar correctamente.", apiName)
	}
}
