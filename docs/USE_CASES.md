# Casos de Uso del Motor Logístico de SolidBit

Este documento describe los flujos principales contemplados en la arquitectura de logística automatizada e inteligente de SolidBit, abarcando todas las integraciones (Meta, OpenAI/Gemini, Google Maps, PostGIS, Stripe).

## 1. El Pedido Feliz

**Descripción**: Un flujo sin fricciones desde que el cliente escribe por WhatsApp hasta la entrega.
1. **Ingesta**: El cliente envía por WhatsApp: "Quiero 2 hamburguesas con queso a la calle Benito Juarez #123".
2. **IA Parser**: Gemini lee el mensaje y extrae los ítems y la dirección en formato JSON estructurado.
3. **Geocoding**: Google Maps convierte "Calle Benito Juarez #123" en (Lat: 27.92, Lng: -110.90).
4. **Logística y Precios**:
   - `RoutingClient` mide la distancia exacta.
   - `PricingEngine` calcula el precio base + distancia.
5. **Pagos**: Se genera el Payment Link en Stripe dinámicamente y se envía al cliente vía WhatsApp.
6. **Despacho PostGIS**: Encuentra en milisegundos al repartidor disponible más cercano al comercio.
7. **Monitoreo Cero Estrés**: El cliente hace el pago y el evento asíncrono actualiza el estado. 
8. **Geofencing**: Cuando el repartidor (monitoreado en `ProximityMonitor`) está a menos de 300 metros, el cliente recibe un WhatsApp de alerta confirmando su inminente llegada.

## 2. Fallo de Geocodificación

**Descripción**: El cliente proporciona una dirección incompleta o no rastreable.
1. **Ingesta**: Cliente envía "Quiero pizza a mi casa".
2. **IA Parser**: Observa que no hay suficiente contexto. Informa a SolidBit que la dirección es inválida o nula.
3. **Manejo Predictivo**: Si la IA extrae nulo, SolidBit responde por WhatsApp solicitando mayor exactitud y no procede al cobro o despacho para cuidar los recursos.
4. Si la dirección parece válida pero Maps API falla, el sistema aplica distancias de "Fallback" (ej. 5km por defecto) permitiendo generar un cobro manual promediado evitando la parálisis de la operación.

## 3. Pago Exitoso con Notificación de Proximidad

**Descripción**: Demuestra el control de estado de vida completo del pedido inter-sistema.
1. **Cobro Stripe**: El usuario completa el checkout y viaja vía Webhook.
2. **Transición Segura**: El `WebhookHandler` en Go valida la firma secreta de inmediato. Emite un UPDATE en DB a `payment_status = 'paid'`.
3. **Geofencing PostGIS**: La base de datos tiene `proximity_notified = false`. 
4. **Fondo Golang**: Un `Ticker` cronológico localiza al repartidor aproximándose por un radio interseccionado usando la matemática geo-espacial de PostGIS.
5. **Cierre Atómico**: Emite mensaje por WhatsApp Graph API de cercanía total al cliente y asegura en misma transacción un `proximity_notified = true` para no enviar el aviso en loop durante 30 segundos múltiples veces.

---

Estas pruebas de sistema blindan los esfuerzos de la Torre de Control y establecen un sistema altamente concurrente y limpio.
