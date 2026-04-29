-- =========================================================================
-- SOLIDBIT DATACENTER: Esquema Base y Lógica Geoespacial (Supabase / PostGIS)
-- =========================================================================

-- Habilitar extensión PostGIS para cálculos geográficos (Requerido en Supabase)
CREATE EXTENSION IF NOT EXISTS postgis;

-- =========================================================================
-- 1. DEFINICIÓN DE ESTRUCTURAS DE DATOS (TABLAS Y TIPOS)
-- =========================================================================

-- Tabla: Merchants (Comerciantes/Restaurantes)
CREATE TABLE merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    location geography(POINT, 4326) NOT NULL, -- Coordenadas del local
    merchant_phone VARCHAR(50),
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Enumeración: Estados de un Repartidor
CREATE TYPE driver_status AS ENUM ('offline', 'available', 'busy');

-- Tabla: Drivers (Repartidores)
CREATE TABLE drivers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth.users(id), -- Vinculado a Supabase Auth
    name TEXT NOT NULL,
    status driver_status DEFAULT 'offline',
    current_location geography(POINT, 4326),  -- Última posición reportada
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Enumeración: Estados de un Pedido
CREATE TYPE order_status AS ENUM ('pending', 'assigned', 'picked_up', 'delivered', 'cancelled');

-- Enumeración: Métodos de Pago
CREATE TYPE payment_method AS ENUM ('cash', 'transfer');

-- Enumeración: Estados de Pago
CREATE TYPE payment_status AS ENUM ('pending', 'paid', 'failed');

-- Tabla: Orders (Pedidos)
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID REFERENCES merchants(id) NOT NULL,
    driver_id UUID REFERENCES drivers(id), -- Puede ser Nulo hasta que se asigne a un repartidor
    status order_status DEFAULT 'pending',
    customer_name TEXT NOT NULL,
    customer_phone TEXT,
    items_description TEXT,
    delivery_location geography(POINT, 4326) NOT NULL, -- Punto de entrega del cliente
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

-- Tabla: Driver Wallets (Efectivo por Liquidar)
CREATE TABLE driver_wallets (
    driver_id UUID PRIMARY KEY REFERENCES drivers(id),
    cash_on_hand NUMERIC(10, 2) DEFAULT 0.00,
    last_liquidation_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Tabla: Settlements (Auditoría de Liquidaciones de Efectivo)
CREATE TABLE settlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    driver_id UUID REFERENCES drivers(id) NOT NULL,
    amount NUMERIC(10, 2) NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ DEFAULT now()
 );
 
 -- Tabla: Wallet Transactions (Auditoría Total de Movimientos de Efectivo)
CREATE TABLE wallet_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id UUID REFERENCES drivers(id) NOT NULL, -- Uso simplificado vinculando a drivers id
    order_id UUID REFERENCES orders(id),
    amount NUMERIC(10, 2) NOT NULL,
    transaction_type TEXT NOT NULL CHECK (transaction_type IN ('entry', 'exit')),
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Tabla: Tracking History (Registro de coordenadas temporal para optimización y soporte)
CREATE TABLE tracking_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID REFERENCES orders(id) NOT NULL,
    driver_id UUID REFERENCES drivers(id) NOT NULL,
    location geography(POINT, 4326) NOT NULL,
    recorded_at TIMESTAMPTZ DEFAULT now()
);

-- Tabla: Frontend Errors (Alertas de interfaz)
CREATE TABLE frontend_errors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    error_message TEXT NOT NULL,
    error_stack TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);


-- =========================================================================
-- 2. ÍNDICES GEOGRÁFICOS (CRÍTICO PARA RENDIMIENTO)
-- =========================================================================
-- Utilizamos índices GiST (Generalized Search Tree) que permiten a Postgres
-- hacer búsquedas de proximidad K-NN (K-Nearest Neighbors) ultrarrápidas.
CREATE INDEX drivers_location_idx ON drivers USING GIST (current_location);
CREATE INDEX merchants_location_idx ON merchants USING GIST (location);
CREATE INDEX orders_delivery_location_idx ON orders USING GIST (delivery_location);
CREATE INDEX tracking_history_location_idx ON tracking_history USING GIST (location);
CREATE INDEX settlements_driver_idx ON settlements(driver_id);
CREATE INDEX wallet_transactions_wallet_idx ON wallet_transactions(wallet_id);

-- Trigger: Reseteo de prioridad al entregar
CREATE OR REPLACE FUNCTION reset_sequence_priority()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'delivered' THEN
        NEW.delivery_sequence_priority = 0;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER reset_sequence_priority_trigger
BEFORE UPDATE ON orders
FOR EACH ROW
EXECUTE FUNCTION reset_sequence_priority();

-- =========================================================================
-- 3. GEO-LOGIC: MOTOR DE BUSQUEDA POSTGIS (Stored Procedure)
-- =========================================================================
-- Algoritmo para encontrar a los N repartidores más cercanos disponibles.
-- Utiliza el operador `<->` nativo de PostGIS que interactúa directamente 
-- con los índices GiST para velocidad de sub-milisegundo.

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
        -- Retorna métrica visible: Calcula distancia exacta en metros
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
        -- `<->` Es el operador de distancia KNN. Mucho más rápido que hacer un ORDER BY con ST_Distance.
        d.current_location <-> ST_SetSRID(ST_MakePoint(start_lon, start_lat), 4326)::geography
    LIMIT limit_count;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;


-- =========================================================================
-- 4. CAPA DE SEGURIDAD: ROW LEVEL SECURITY (RLS)
-- =========================================================================
-- Obligamos a que todas las consultas vía API a Supabase pasen por nuestros filtros de autorización.

-- Habilitar RLS explícitamente en todas las tablas sensibles
ALTER TABLE drivers ENABLE ROW LEVEL SECURITY;
ALTER TABLE orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE merchants ENABLE ROW LEVEL SECURITY;
ALTER TABLE tracking_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE frontend_errors ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Allow anon insert to frontend_errors"
ON frontend_errors FOR INSERT
WITH CHECK (true);

CREATE POLICY "Allow read for auth users frontend_errors"
ON frontend_errors FOR SELECT
USING (auth.uid() IS NOT NULL);


-- POLÍTICAS: REPARTIDORES (DRIVERS)
-- Un repartidor solo puede acceder y actualizar su PERFIL (donde su usuario de Supabase Auth coincida).
CREATE POLICY "Drivers access own profile"
ON drivers FOR SELECT
USING (auth.uid() = user_id);

CREATE POLICY "Drivers manage own profile"
ON drivers FOR UPDATE
USING (auth.uid() = user_id);


-- POLÍTICAS: PEDIDOS (ORDERS)
-- "RLS para que un repartidor SOLO pueda ver los pedidos asignados a él."
-- Se evalúa si el auth.uid() coincide con el user_id de la tabla drivers anidada al pedido.
CREATE POLICY "Drivers view strictly assigned orders"
ON orders FOR SELECT
USING (
    driver_id IN (SELECT id FROM drivers WHERE user_id = auth.uid())
);

CREATE POLICY "Drivers update strictly assigned orders status"
ON orders FOR UPDATE
USING (
    driver_id IN (SELECT id FROM drivers WHERE user_id = auth.uid())
);


-- POLÍTICAS: HISTORIAL DE SEGUIMIENTO (TRACKING)
CREATE POLICY "Drivers log track their assigned orders"
ON tracking_history FOR INSERT
WITH CHECK (
    driver_id IN (SELECT id FROM drivers WHERE user_id = auth.uid())
);

CREATE POLICY "Drivers view own tracking data"
ON tracking_history FOR SELECT
USING (
    driver_id IN (SELECT id FROM drivers WHERE user_id = auth.uid())
);


-- POLÍTICAS: COMERCIANTES (MERCHANTS) (Solo Lectura)
-- Le permitimos a los repartidores ver la información de un restaurante SOLAMENTE si tienen
-- un pedido en curso asigando hacia dicho restaurante.
CREATE POLICY "Drivers see merchant context for active orders"
ON merchants FOR SELECT
USING (
    id IN (SELECT merchant_id FROM orders WHERE driver_id IN (SELECT id FROM drivers WHERE user_id = auth.uid()))
);


-- =========================================================================
-- 5. VISTAS ANALÍTICAS (BUSINESS INTELLIGENCE)
-- =========================================================================

-- Vista: admin_metrics
-- Provee agregados de información crítica de la torre de control en tiempo real.
CREATE OR REPLACE VIEW admin_metrics AS
SELECT
    (SELECT COALESCE(SUM(total_amount), 0) FROM orders WHERE payment_method = 'transfer' AND payment_status = 'paid') AS total_transfers,
    (SELECT COALESCE(SUM(total_amount), 0) FROM orders WHERE payment_method = 'cash' AND status IN ('delivered')) AS total_cash,
    (SELECT COALESCE(SUM(amount), 0) FROM settlements) AS total_settled,
    (SELECT COUNT(*) FROM orders WHERE status = 'delivered' AND DATE(updated_at) = CURRENT_DATE) AS delivered_today;

