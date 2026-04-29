-- db/test_sql_rls.sql
-- Script de validación de lógica SQL y Row Level Security.
-- Para probar manualmente en Supabase SQL Editor o pg_prove.

BEGIN;

-- 1. Prueba de Función Búsqueda Geoespacial
SAVEPOINT sp1;
INSERT INTO drivers (id, name, status, current_location) 
VALUES ('drv-test-1', 'Flash', 'available', ST_SetSRID(ST_Point(-110.92345, 27.91234), 4326));

-- Debería retornar resultados por cercanía a esa coordenada
SELECT * FROM get_closest_available_drivers(-110.92345, 27.91234, 5000);
ROLLBACK TO SAVEPOINT sp1;

-- 2. Prueba de Políticas RLS para Repartidores
SAVEPOINT sp2;
-- El repartidor 'drv-alpha' intenta ver sus pedidos
SET LOCAL ROLE authenticated;
SET LOCAL request.jwt.claims TO '{"sub": "drv-alpha"}';

-- Esto probará que la policy 'Drivers can ONLY read their ASSIGNED orders' funciona.
-- Si hay pedidos pertenecientes a otros, no los debe devolver.
SELECT * FROM orders; 

ROLLBACK TO SAVEPOINT sp2;

COMMIT;
