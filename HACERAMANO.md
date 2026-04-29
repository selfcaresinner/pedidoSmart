# 🛠️ GUÍA DE CONFIGURACIÓN MANUAL (SOLIDBIT)

Este documento contiene las instrucciones paso a paso para configurar la infraestructura, obtener las variables de entorno y preparar la base de datos en Supabase.

---

## 1. 🔑 Guía de Variables de Entorno (.env)

Copia el archivo `.env.example` a uno nuevo llamado `.env` y completa los siguientes campos:

### 🧠 Inteligencia Artificial (Gemini)
*   **GEMINI_API_KEY**: Consíguela en [Google AI Studio](https://aistudio.google.com/). Es la que procesa los mensajes de WhatsApp para extraer pedidos.

### 🗺️ Mapas y Geolocalización
*   **NEXT_PUBLIC_MAPS_API_KEY**: Consíguela en [Google Cloud Console](https://console.cloud.google.com/). Debes habilitar:
    *   Maps JavaScript API
    *   Geocoding API
    *   Places API
    *   Distance Matrix API

### 💬 WhatsApp (Meta Business)
*   **WHATSAPP_ACCESS_TOKEN**: En [Meta for Developers](https://developers.facebook.com/), crea una App de tipo "Business", añade "WhatsApp" y obtén el Token de Acceso Permanente.
*   **WHATSAPP_PHONE_NUMBER_ID**: Se encuentra en la configuración de WhatsApp dentro de tu App de Meta.
*   **ADMIN_PHONE**: Tu número personal con código de país (ej: `521622...`) para recibir alertas críticas.

### 🛡️ Seguridad y Soporte
*   **ADMIN_PASSWORD**: Una clave inventada por ti para entrar al panel de administración.
*   **NEXT_PUBLIC_SUPPORT_EMAIL**: Correo donde los clientes pedirán ayuda (ej: `ayuda@solidbit.app`).
*   **NEXT_PUBLIC_BUSINESS_ADDRESS**: Dirección física legal requerida.

---

## 2. 🗄️ Configuración de Supabase (Base de Datos)

Sigue estos pasos en tu proyecto de [Supabase](https://supabase.com/):

### Paso A: Extensiones y Esquema
1.  Ve a **SQL Editor**.
2.  Pega y ejecuta el contenido del **Esquema SQL** (ver sección 3 abajo). Esto activará PostGIS y creará todas las tablas e índices.

### Paso B: Storage (Evidencia de Entrega)
1.  Ve a **Storage** en la barra lateral.
2.  Crea un nuevo **Bucket** llamado `delivery-evidence`.
3.  Configúralo como **Público** (Public: ON).

### Paso C: Variables en el .env
Copia estos valores de *Project Settings > API*:
*   **DATABASE_URL**: Usa el "Connection String" de tipo *Transaction Mode* (Puerto 6543 o 5432).
*   **NEXT_PUBLIC_SUPABASE_URL**: La URL de tu proyecto.
*   **NEXT_PUBLIC_SUPABASE_ANON_KEY**: La llave `anon` `public`.

---

## 3. 📄 Esquema Completo de la DB (PostgreSQL)

Ejecuta este código en el SQL Editor de Supabase:

```sql
-- Habilitar PostGIS para cálculos de delivery
CREATE EXTENSION IF NOT EXISTS postgis;

-- 1. Tablas Base
CREATE TABLE merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    location geography(POINT, 4326) NOT NULL,
    merchant_phone VARCHAR(50),
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TYPE driver_status AS ENUM ('offline', 'available', 'busy');

CREATE TABLE drivers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL, -- Vinculado a Supabase Auth
    name TEXT NOT NULL,
    status driver_status DEFAULT 'offline',
    current_location geography(POINT, 4326),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TYPE order_status AS ENUM ('pending', 'assigned', 'picked_up', 'delivered', 'cancelled');
CREATE TYPE payment_method AS ENUM ('cash', 'transfer');
CREATE TYPE payment_status AS ENUM ('pending', 'paid', 'failed');

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID REFERENCES merchants(id) NOT NULL,
    driver_id UUID REFERENCES drivers(id),
    status order_status DEFAULT 'pending',
    customer_name TEXT NOT NULL,
    customer_phone TEXT,
    items_description TEXT,
    delivery_location geography(POINT, 4326) NOT NULL,
    payment_method payment_method DEFAULT 'cash',
    payment_status payment_status DEFAULT 'pending',
    total_amount NUMERIC(10, 2) DEFAULT 0.00,
    price_breakdown JSONB,
    delivery_sequence_priority INT DEFAULT 0,
    proximity_notified BOOLEAN DEFAULT FALSE,
    confirmed_by_merchant BOOLEAN DEFAULT FALSE,
    delivery_evidence_url TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Cartera de Efectivo (Para repartidores)
CREATE TABLE driver_wallets (
    driver_id UUID PRIMARY KEY REFERENCES drivers(id),
    cash_on_hand NUMERIC(10, 2) DEFAULT 0.00,
    last_liquidation_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- 2. Índices para velocidad de búsqueda de cercanía
CREATE INDEX drivers_location_idx ON drivers USING GIST (current_location);
CREATE INDEX merchants_location_idx ON merchants USING GIST (location);
CREATE INDEX orders_delivery_location_idx ON orders USING GIST (delivery_location);

-- 3. Procedimiento para encontrar repartidores cercanos
CREATE OR REPLACE FUNCTION get_closest_available_drivers(
    start_lon DOUBLE PRECISION,
    start_lat DOUBLE PRECISION,
    limit_count INT DEFAULT 5
)
RETURNS TABLE (
    driver_id UUID,
    driver_name TEXT,
    distance_meters FLOAT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        d.id,
        d.name,
        ST_Distance(
            d.current_location,
            ST_SetSRID(ST_MakePoint(start_lon, start_lat), 4326)::geography
        ) AS distance_meters
    FROM 
        drivers d
    WHERE 
        d.status = 'available' 
        AND d.current_location IS NOT NULL
    ORDER BY 
        d.current_location <-> ST_SetSRID(ST_MakePoint(start_lon, start_lat), 4326)::geography
    LIMIT limit_count;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 4. Vista para el Panel de Administración (KPIs)
CREATE OR REPLACE VIEW admin_metrics AS
SELECT
    (SELECT COALESCE(SUM(total_amount), 0) FROM orders WHERE payment_method = 'transfer' AND payment_status = 'paid') AS total_transfers,
    (SELECT COALESCE(SUM(total_amount), 0) FROM orders WHERE payment_method = 'cash' AND status IN ('delivered')) AS total_cash,
    (SELECT COUNT(*) FROM orders WHERE status = 'delivered' AND DATE(updated_at) = CURRENT_DATE) AS delivered_today;
```

---
💡 **Nota Final:** Recuerda habilitar el RLS (Row Level Security) en Supabase para proteger los datos de los repartidores y pedidos.
