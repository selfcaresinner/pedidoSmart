'use client';

import React, { useEffect, useState } from 'react';
import { supabase } from '@/lib/supabase';
import { Phone, MapPin, Package, CheckCircle2, Navigation, CircleDot, Clock } from 'lucide-react';
import { motion, AnimatePresence } from 'motion/react';

// Tipos basados en nuestro schema.sql
type OrderStatus = 'pending' | 'assigned' | 'picked_up' | 'delivered' | 'cancelled';
type DriverStatus = 'offline' | 'available' | 'busy';

interface Order {
  id: string;
  merchant_id: string;
  driver_id: string;
  status: OrderStatus;
  customer_name: string;
  customer_phone: string;
  created_at: string;
  // Solo la usaremos mockeada acá visualmente o con los datos geográficos guardados
  // Note: Si queremos obtener datos geográficos crudos postgis, se debería hacer un RPC temporal
  // o utilizar un helper en el schema, para simplificar visualizaremos los pedidos devueltos.
}

interface Driver {
  id: string;
  name: string;
  status: DriverStatus;
}

export default function DashboardPage() {
  const [orders, setOrders] = useState<Order[]>([]);
  const [driverStatus, setDriverStatus] = useState<DriverStatus>('offline');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // 1. Obtener estado inicial
    fetchInitialData();

    // 2. Suscribir a Realtime de Supabase (Escuchar INSERT y UPDATE)
    const channel = supabase
      .channel('public:orders')
      .on(
        'postgres_changes',
        { event: '*', schema: 'public', table: 'orders' },
        (payload) => {
          handleOrderChange(payload);
        }
      )
      .subscribe();

    return () => {
      supabase.removeChannel(channel);
    };
  }, []);

  const fetchInitialData = async () => {
    try {
      setLoading(true);
      // Extraemos los pedidos asignados que aún están activos
      const { data: dbOrders, error: ordersError } = await supabase
        .from('orders')
        .select('*')
        .in('status', ['assigned', 'picked_up'])
        .order('created_at', { ascending: false });

      if (ordersError) throw ordersError;
      
      if (dbOrders) {
        setOrders(dbOrders as Order[]);
      }

      // Podríamos hacer fetch al driver usando el user id
      // Mock estado disponible
      setDriverStatus('available');
    } catch (error) {
      console.error("[SolidBit][UI] Error fetching init data:", error);
    } finally {
      setLoading(false);
    }
  };

  const handleOrderChange = (payload: any) => {
    const newOrder = payload.new as Order;
    const eventType = payload.eventType;

    setOrders((prev) => {
      if (eventType === 'INSERT') {
        if (newOrder.status === 'assigned' || newOrder.status === 'picked_up') {
          return [newOrder, ...prev];
        }
        return prev;
      } else if (eventType === 'UPDATE') {
        // Removemos si ya se entregó
        if (newOrder.status === 'delivered' || newOrder.status === 'cancelled') {
          return prev.filter((o) => o.id !== newOrder.id);
        }
        // Actualizamos o insertamos
        const exists = prev.some((o) => o.id === newOrder.id);
        if (exists) {
          return prev.map((o) => (o.id === newOrder.id ? newOrder : o));
        } else if (newOrder.status === 'assigned' || newOrder.status === 'picked_up') {
          return [newOrder, ...prev];
        }
      } else if (eventType === 'DELETE') {
        return prev.filter((o) => o.id !== payload.old.id);
      }
      return prev;
    });
  };

  const updateOrderStatus = async (orderId: string, currentStatus: OrderStatus) => {
    let nextStatus: OrderStatus = 'delivered';
    if (currentStatus === 'assigned') {
      nextStatus = 'picked_up';
    } else if (currentStatus === 'picked_up') {
      nextStatus = 'delivered';
    } else {
      return; // No transitions
    }

    try {
      const { error } = await supabase
        .from('orders')
        .update({ status: nextStatus, updated_at: new Date().toISOString() })
        .eq('id', orderId);

      if (error) throw error;
      
      // Realtime escuchará el UPDATE y modificará la UI localmente.
    } catch (error) {
      console.error("[SolidBit][UI] Error updating order:", error);
      alert("Hubo un error sincronizando con la base de datos.");
    }
  };

  const updateDriverStatus = async () => {
    const nextStatus = driverStatus === 'available' ? 'offline' : 'available';
    // MOCK: Aquí ejecutaría un update a la DB
    setDriverStatus(nextStatus);
  };

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col font-sans text-gray-900 pb-20">
      {/* Header Sticky */}
      <header className="sticky top-0 z-20 bg-white shadow-sm border-b border-gray-100 flex items-center justify-between px-4 py-4">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-gray-950">SolidBit</h1>
          <p className="text-xs text-gray-500 font-medium tracking-wide">PANEL DE DESPACHO</p>
        </div>

        {/* Badge Disponibilidad */}
        <button 
          onClick={updateDriverStatus}
          className={`flex items-center gap-2 px-3 py-1.5 rounded-full text-sm font-semibold transition-colors duration-200 ${
          driverStatus === 'available' 
            ? 'bg-emerald-100 text-emerald-700 hover:bg-emerald-200'
            : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
        }`}>
          <CircleDot className={`w-4 h-4 ${driverStatus === 'available' ? 'animate-pulse' : ''}`} />
          {driverStatus === 'available' ? 'Disponible' : 'Desconectado'}
        </button>
      </header>

      {/* Main Content (Mobile First) */}
      <main className="flex-1 px-4 py-6 max-w-lg mx-auto w-full">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-gray-800">Tus Rutas Activas</h2>
          <span className="text-xs font-bold bg-indigo-100 text-indigo-700 px-2 py-1 rounded-md">
            {orders.length} Pedidos
          </span>
        </div>

        {loading ? (
          <div className="flex justify-center py-10">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600"></div>
          </div>
        ) : orders.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 text-center">
            <div className="w-16 h-16 bg-gray-100 text-gray-400 rounded-full flex items-center justify-center mb-4">
              <Clock className="w-8 h-8" />
            </div>
            <h3 className="text-gray-900 font-semibold mb-1">Sin entregas por ahora</h3>
            <p className="text-gray-500 text-sm max-w-[250px]">
              Al parecer no tienes pedidos asignados. Mantente disponible para recibir nuevas rutas.
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            <AnimatePresence>
              {orders.map((order) => (
                <OrderCard 
                  key={order.id} 
                  order={order} 
                  onStatusUpdate={() => updateOrderStatus(order.id, order.status)} 
                />
              ))}
            </AnimatePresence>
          </div>
        )}
      </main>
    </div>
  );
}

// Subcomponente de la Carta de Pedido
function OrderCard({ order, onStatusUpdate }: { order: Order; onStatusUpdate: () => void }) {
  const isAssigned = order.status === 'assigned';
  const isPickedUp = order.status === 'picked_up';

  return (
    <motion.div 
      initial={{ opacity: 0, y: 15 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, scale: 0.95 }}
      transition={{ duration: 0.2 }}
      className={`bg-white rounded-2xl shadow-sm border overflow-hidden relative
        ${isAssigned ? 'border-orange-200' : 'border-indigo-200'}
      `}
    >
      {/* Cinta reflectiva indicadora de color de estado (UI delgada a un lado) */}
      <div className={`absolute left-0 top-0 bottom-0 w-1.5 
        ${isAssigned ? 'bg-orange-500' : 'bg-indigo-500'}
      `} />

      <div className="p-5 pl-6">
        <div className="flex justify-between items-start mb-3">
          <div>
            <div className={`text-xs font-bold tracking-wider mb-1 uppercase
                ${isAssigned ? 'text-orange-600' : 'text-indigo-600'}
              `}>
              {isAssigned ? 'Recién Asignado' : 'En Tránsito'}
            </div>
            <h3 className="text-lg font-bold text-gray-900 border-b border-gray-100 pb-2 mb-2 inline-flex gap-2 items-center">
               <Package className="w-4 h-4 text-gray-500" />
               ID: {order.id.slice(0, 8)}
            </h3>
          </div>
          
          <a
            href={`tel:${order.customer_phone}`}
            className="w-10 h-10 rounded-full bg-green-50 text-green-600 flex items-center justify-center hover:bg-green-100 transition-colors"
            title="Llamar al cliente"
          >
            <Phone className="w-5 h-5 fill-current" />
          </a>
        </div>

        <div className="space-y-3 mt-4">
          <div className="flex gap-3">
             <div className="mt-0.5 text-gray-400">
                <MapPin className="w-5 h-5" />
             </div>
             <div>
                <p className="text-sm font-semibold text-gray-900">Cliente: {order.customer_name}</p>
                {/* En un esquema real aquí renderizamos la Location reverse-geocoded o lo recibido desde IA */}
                <p className="text-xs text-gray-500 leading-relaxed mt-0.5">Pendiente de geocodificación final de coordenadas en tabla para mostrar domicilio.</p>
             </div>
          </div>
        </div>

        {/* Botonera de Acción */}
        <div className="mt-5 pt-4 border-t border-gray-50">
          <button
            onClick={onStatusUpdate}
            className={`w-full py-3 px-4 rounded-xl font-bold text-sm tracking-wide transition-all shadow-sm flex items-center justify-center gap-2
              ${isAssigned 
                ? 'bg-orange-500 hover:bg-orange-600 text-white' 
                : 'bg-indigo-600 hover:bg-indigo-700 text-white'
              }
            `}
          >
            {isAssigned ? (
              <>
                <CheckCircle2 className="w-5 h-5" />
                CONFIRMAR RECOLECCIÓN
              </>
            ) : (
              <>
                <Navigation className="w-5 h-5 fill-current" />
                ENTREGAR PEDIDO
              </>
            )}
          </button>
        </div>
      </div>
    </motion.div>
  );
}
