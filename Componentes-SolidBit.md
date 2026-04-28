# SolidBit: Ecosistema y Estándares de Desarrollo

Bienvenido al manual de marca técnica de SolidBit.
Este documento establece las normativas arquitectónicas, los patrones de desarrollo y las reglas inflexibles que aseguran el empaquetado de código seguro, mantenible y del más alto rendimiento en nuestro entorno.

## 1. Variables de Entorno (Configuration as Code)
**Regla de Oro:** "Fail-Fast" (Fallar rápido y con ruido).
En SolidBit, ninguna aplicación o script entra a producción (o desarrollo/testing) si falta una variable de configuración vital.
- No delegamos responsabilidades de verificación al momento en el que el recurso sea consumido. Toda clave vital se verifica en el **inicio (bootstrap)** de la aplicación a través de nuestra librería interna.
- Las variables opcionales siempre tendrán un **Fallback** o por defecto configurado de forma explícita.
- Los secretos compartidos NUNCA se suben al control de versiones. Se utiliza siempre un archivo `.env.example` en repositorios para denotar requerimientos.

## 2. Logging Estructurado (Observabilidad)
**Regla de Oro:** Nada de texto suelto (`fmt.Println`), todo en formato JSON / Estructurado (K/V).
Para que plataformas como Datadog, Grafana o GCP logren rastrear problemas en milisegundos, todos los logs en SolidBit deben tener formato estructurado (ej. a través de paquetes como `slog` o `zap` en Go).
- Todo log debe contar con los parámetros estándar: `[timestamp, nivel (INFO, WARN, ERROR, FATAL), origen (modulo), mensaje, traza (request_id, si aplica)]`.
- **Nivel ERROR**: Únicamente si el sistema no puede procesar una tarea en específico, y requiere intervención o atención del equipo de desarrollo.
- **Nivel FATAL / PANIC**: Únicamente cuando la aplicación NO PUEDE continuar funcionando y el proceso maestro debe reiniciarse.

## 3. Manejo de Errores (Resiliencia)
**Regla de Oro:** Los errores son datos, no excepciones que se tiran al aire. (Error as Values).
- **Envoltorio (Wrapping):** Cuando un sistema inferior devuelva un error (ej. capa de BD), se debe "envolver" (`fmt.Errorf("fallo la ingesta: %w", err)`) para preservar el origen del error inicial mientras se añade contexto sobre qué acción causó el fallo.
- Al final de la cadena (normalmente el Handler HTTP, el Worker principal o la interfaz Webhook), el error recién ahí es registrado formalmente en nuestro Logger.
- El cliente o usuario externo JAMÁS debe recibir un Stack Trace. Siempre se debe devolver un código de respuesta limpio (HTTP 500/400) acompañado de un identificador de traza.

--- 
*Sello de Calidad de SolidBit Architecture. Redactado para un escalado inteligente.*
