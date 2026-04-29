# Guía de Despliegue en Producción (Railway / Docker)

Este documento detalla los requerimientos físicos y de configuración para llevar **SolidBit** a su máxima capacidad en un entorno de producción (Railway, AWS, GCP, etc.).

## 1. Topología del Sistema
SolidBit es un microservicio monolítico segmentado. Tiene dos aplicaciones:
1. **El Backend (Golang):** El núcleo de Ingesta, Logística GIS (Geográfica) y Webhooks (Ingestion Engine). Se despliega usando `Dockerfile.backend`.
2. **El Frontend (Next.js):** El Dashboard / Panel visual (Torre de Control) para administración de flujos y acceso de los repartidores. Se despliega usando `Dockerfile.frontend`.

> **Nota para Railway**: Puedes crear 2 "Services" en el mismo proyecto, apuntando al mismo repositorio pero cambiando la ruta del Dockerfile por defecto en los settings de cada servicio (`Dockerfile.backend` y `Dockerfile.frontend`).

## 2. Checklist de Variables de Entorno (SECRETS)

Antes del despliegue, debes configurar estrictamente estas variables en el entorno de producción. SolidBit consta de una configuración `Fail-Fast`, si alguna de las siguientes no está presente, el servicio **no arrancará** devolviendo un `panic()`.

### Backend (Golang - order-ingestion)
- `DATABASE_URL`: URL remota a Supabase o servidor Postgres con PostGIS activado (ej. `postgres://postgres.[ID]:[PASSWORD]@aws-0-us-east-1.pooler.supabase.com:6543/postgres`).
- `PORT`: Puerto (normalmente `8080`, si no se asigna lo sobrescribe el orquestador).
- `ENVIRONMENT`: `production`.
- `NEXT_PUBLIC_MAPS_API_KEY`: API Key de Google Maps (Requerida por el RoutingClient en backend para la API Directions).
- `GEMINI_API_KEY`: Clave de acceso a la API REST de Google Gemini (Extrae Inteligencia de WhatsApp).
- `STRIPE_SECRET_KEY`: Llave de Producción/Test de la cuenta de Stripe (`sk_test_...` o `sk_live_...`).
- `STRIPE_WEBHOOK_SECRET`: Firma para validar respuestas al webhook (Se obtiene creando un endpoint webhook endpoint apuntando a `TU_DOMINIO/webhook/stripe`).
- `WHATSAPP_ACCESS_TOKEN`: Token de acceso del sistema Meta Developer Graph API.
- `WHATSAPP_PHONE_NUMBER_ID`: ID del número de WhatsApp enlazado de Business API.
- `ADMIN_PASSWORD`: Código/Contraseña compartida temporal para los repartidores (Ej: `54321` o `SolidBit2026`).
- `APP_URL`: URL root donde habitará SolidBit en la web (Ej: `https://solidbit.app`). Requerida por Stripe para return url.

### Frontend (Next.js - dashboard)
- `NEXT_PUBLIC_SUPABASE_URL`: Endpoint de tu Supabase.
- `NEXT_PUBLIC_SUPABASE_ANON_KEY`: LLave de cliente anónima autorizada.
- `NEXT_PUBLIC_MAPS_API_KEY`: Replicada del backend (Renderiza mapa interactivo en los browsers).
- `NEXT_PUBLIC_ADMIN_PASSWORD`: Código compartido de lectura a los repartidores que empata con `ADMIN_PASSWORD`.

## 3. Comandos Importantes de validación y test
1. Ejecuta Tests locales Financieros y Lógicos: `go test ./...`
2. Construir local Frontend: `npm install && npm run build`
3. Construir local Backend: `docker build -f Dockerfile.backend -t solidbit-engine .`

## 4. Estándar de Logging
Al arrancar, el backend generará automáticamente un `"Pre-Flight Check"` para reconfirmarte visualmente en los dashboards de Digital Ocean o Railway que cada API se inyectó de forma limpia y exitosa al binario `order-ingestion` sin crashear.
