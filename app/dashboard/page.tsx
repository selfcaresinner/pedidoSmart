'use client';

import React, { useEffect, useState, useRef } from 'react';
import { supabase } from '@/lib/supabase';
import { Phone, MapPin, Package, CheckCircle2, Navigation, CircleDot, Clock, Lock, Camera, Loader2, Box } from 'lucide-react';
import { motion, AnimatePresence } from 'motion/react';
import { GoogleMap, useLoadScript, Marker, Polyline } from '@react-google-maps/api';

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
  delivery_location?: any;
  payment_method?: 'cash' | 'transfer';
  payment_status?: 'pending' | 'paid' | 'failed';
  total_amount: number;
  delivery_sequence_priority?: number;
}

interface Wallet {
  driver_id: string;
  cash_on_hand: number;
  total_earned: number;
  updated_at: string;
}

// Valores por defecto para el mapa
const DefaultMerchantLoc = { lat: 27.9678, lng: -110.8988 }; // Empalme/Guaymas reference
const DefaultCustomerLoc = { lat: 27.9712, lng: -110.8931 };

const parsePoint = (geo: any) => {
  if (!geo) return { lat: 27.9667, lng: -110.8167 }; // Fallback centro si no hay data
  
  // Si Supabase devuelve un objeto con coordenadas (lon, lat)
  if (typeof geo === 'object' && geo.coordinates) {
    return { lat: geo.coordinates[1], lng: geo.coordinates[0] };
  }

  // Si devuelve una cadena tipo "POINT(-110.8167 27.9667)"
  if (typeof geo === 'string') {
    const match = geo.match(/POINT\(([^ ]+) ([^ ]+)\)/);
    if (match) {
      return { 
        lat: parseFloat(match[2]), 
        lng: parseFloat(match[1]) 
      };
    }
  }

  return { lat: 27.9667, lng: -110.8167 };
};

interface Driver {
  id: string;
  name: string;
  status: DriverStatus;
}

export default function DashboardPage() {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [orders, setOrders] = useState<Order[]>([]);
  const [driverStatus, setDriverStatus] = useState<DriverStatus>('offline');
  const [wallet, setWallet] = useState<Wallet | null>(null);
  const [loading, setLoading] = useState(true);
  const [deliveredTodayCount, setDeliveredTodayCount] = useState<number>(0);
  const [driverLocation, setDriverLocation] = useState<{lat: number, lng: number} | null>(null);
  const [uploading, setUploading] = useState<string | null>(null); // orderId being uploaded
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [activeOrderId, setActiveOrderId] = useState<string | null>(null);

  const { isLoaded } = useLoadScript({
    googleMapsApiKey: process.env.NEXT_PUBLIC_MAPS_API_KEY || '',
  });

  useEffect(() => {
    // Check URL or LocalStorage
    const params = new URLSearchParams(window.location.search);
    const urlDriverId = params.get('driver_id');
    const savedCode = localStorage.getItem('solidbit_driver_auth');
    
    if (urlDriverId || savedCode) {
      setIsAuthenticated(true);
    } else {
      window.location.href = '/';
    }
  }, []);

  // GPS Tracking Loop
  useEffect(() => {
    if (!isAuthenticated) return;
    let interval: NodeJS.Timeout;

    if (orders.length > 0 && navigator.geolocation) {
      interval = setInterval(() => {
        navigator.geolocation.getCurrentPosition(
          async (position) => {
            const { latitude, longitude } = position.coords;
            setDriverLocation({ lat: latitude, lng: longitude });

            try {
              const activeOrder = orders.find(o => o.status === 'picked_up' || o.status === 'assigned');
              if (activeOrder) {
                // Update track history
                await supabase.from('tracking_history').insert({
                  order_id: activeOrder.id,
                  driver_id: activeOrder.driver_id,
                  location: `POINT(${longitude} ${latitude})`
                });

                // Update current location in driver profile
                await supabase.from('drivers')
                  .update({ current_location: `POINT(${longitude} ${latitude})`, updated_at: new Date().toISOString() })
                  .eq('id', activeOrder.driver_id);
              }
            } catch (err) {
              console.error("[SolidBit] Geoloc error syncing:", err);
            }
          },
          (err) => console.error("GPS Error:", err),
          { enableHighAccuracy: true, timeout: 5000, maximumAge: 0 }
        );
      }, 30000); // 30 seconds
    }

    return () => {
      if (interval) clearInterval(interval);
    };
  }, [orders]);

  const fetchInitialData = async () => {
    try {
      setLoading(true);

      const savedCode = localStorage.getItem('solidbit_driver_auth');
      if (!savedCode) return;

      const res = await fetch(`/api/driver/init?driver_id=${savedCode}`);
      if (!res.ok) throw new Error('Failed to fetch data');
      
      const data = await res.json();
      
      if (data.orders) setOrders(data.orders);
      if (data.wallet) setWallet(data.wallet);
      setDeliveredTodayCount(data.completedToday || 0);
      
      // For status, ideally we get it, default otherwise if missing
      const driverName = localStorage.getItem('solidbit_driver_name');

    } catch (error) {
      console.error("[SolidBit][UI] Error fetching init data:", error);
    } finally {
      setLoading(false);
    }
  };

  // Update Tracking Info
  useEffect(() => {
    const walletChannel = supabase
      .channel('driver:wallet')
      .on('postgres_changes', { event: '*', schema: 'public', table: 'driver_wallets' }, (payload) => {
        setWallet(payload.new as Wallet);
      })
      .subscribe();
    
    return () => {
      supabase.removeChannel(walletChannel);
    };
  }, []);

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

  const updateOrderStatus = async (orderId: string, currentStatus: OrderStatus, evidenceUrl?: string) => {
    let nextStatus: OrderStatus = 'delivered';
    if (currentStatus === 'assigned') {
      nextStatus = 'picked_up';
    } else if (currentStatus === 'picked_up') {
      if (!evidenceUrl) {
          setActiveOrderId(orderId);
          fileInputRef.current?.click();
          return;
      }
      nextStatus = 'delivered';
    } else {
      return; // No transitions
    }

    try {
      if (nextStatus === 'delivered') {
          // Si es entrega final, pasamos por el backend de Go para notificar WhatsApp
          const res = await fetch('/api/driver/complete', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ 
                  order_id: orderId, 
                  delivery_evidence_url: evidenceUrl,
                  driver_id: orders.find(o => o.id === orderId)?.driver_id || 'unknown'
              })
          });
          if (!res.ok) throw new Error("Fallo al completar la orden en el backend");
      } else if (nextStatus === 'picked_up') {
          // Notificar recolección para que el bot avise al cliente
          const res = await fetch('/api/order/status', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ 
                  order_id: orderId, 
                  status: 'picked_up'
              })
          });
          if (!res.ok) throw new Error("Fallo al actualizar estado a recolectado");
      } else {
        const { error } = await supabase
          .from('orders')
          .update({ status: nextStatus, updated_at: new Date().toISOString() })
          .eq('id', orderId);

        if (error) throw error;
      }
      
      // Realtime escuchará el UPDATE y modificará la UI localmente.
    } catch (error) {
      console.error("[SolidBit][UI] Error updating order:", error);
      alert("Hubo un error sincronizando con el servidor.");
    }
  };

  const handleFileUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file || !activeOrderId) return;

    setUploading(activeOrderId);
    try {
        const fileExt = file.name.split('.').pop();
        const fileName = `${activeOrderId}.${fileExt}`;
        const filePath = `evidence/${fileName}`;

        // Upload to Supabase Storage (Bucket: delivery-evidence)
        const { error: uploadError, data } = await supabase.storage
            .from('delivery-evidence')
            .upload(filePath, file, { upsert: true });

        if (uploadError) throw uploadError;

        // Get Public URL
        const { data: { publicUrl } } = supabase.storage
            .from('delivery-evidence')
            .getPublicUrl(filePath);

        // Finalize order with evidence URL
        await updateOrderStatus(activeOrderId, 'picked_up', publicUrl);
        
    } catch (err: any) {
        alert("Fallo al subir la foto: " + err.message);
    } finally {
        setUploading(null);
        setActiveOrderId(null);
        if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const updateDriverStatus = async () => {
    const nextStatus = driverStatus === 'available' ? 'offline' : 'available';
    // MOCK: Aquí ejecutaría un update a la DB
    setDriverStatus(nextStatus);
  };

  useEffect(() => {
    if (!isAuthenticated) return;

    // 1. Obtener estado inicial
    const init = async () => {
       await fetchInitialData();
    };
    init();

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
  }, [isAuthenticated]);

  if (!isAuthenticated) return null;

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col font-sans text-gray-900 pb-20">
      {/* Header Sticky */}
      <header className="sticky top-0 z-20 bg-white shadow-sm border-b border-gray-100 flex items-center justify-between px-4 py-4">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-gray-950">SolidBit</h1>
          {wallet && (
              <div className="flex flex-col gap-1 mt-1">
                <div className="inline-flex w-fit items-center gap-1 text-[10px] font-bold text-orange-600 bg-orange-50 border border-orange-100 px-2 py-0.5 rounded-full">
                    EFECTIVO POR ENTREGAR: ${wallet.cash_on_hand?.toFixed(2) || '0.00'}
                </div>
                <div className="inline-flex w-fit items-center gap-1 text-[10px] font-bold text-emerald-600 bg-emerald-50 border border-emerald-100 px-2 py-0.5 rounded-full">
                    MIS GANANCIAS DEL DÍA: ${wallet.total_earned?.toFixed(2) || '0.00'}
                </div>
              </div>
          )}
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
        
        {/* Resumen de Jornada */}
        <details className="mb-6 bg-white border border-gray-200 rounded-2xl group overflow-hidden">
           <summary className="flex items-center justify-between p-4 cursor-pointer font-bold text-gray-800 list-none [&::-webkit-details-marker]:hidden">
              <div className="flex items-center gap-2">
                 <Box className="w-5 h-5 text-indigo-600" />
                 <span>Resumen de mi Jornada</span>
              </div>
              <span className="text-gray-400 group-open:rotate-180 transition-transform">▼</span>
           </summary>
           <div className="p-4 border-t border-gray-100 bg-gray-50 flex justify-between">
              <div>
                 <p className="text-[10px] text-gray-500 font-bold mb-1">PEDIDOS ENTREGADOS HOY</p>
                 <p className="text-xl font-black text-gray-900">{deliveredTodayCount}</p>
              </div>
              <div className="text-right">
                 <p className="text-[10px] text-gray-500 font-bold mb-1">TOTAL GANADO HOY</p>
                 <p className="text-xl font-black text-emerald-600">${wallet?.total_earned?.toFixed(2) || '0.00'}</p>
              </div>
           </div>
        </details>

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
              {[...orders].sort((a, b) => (a.delivery_sequence_priority || 999) - (b.delivery_sequence_priority || 999)).map((order, index, arr) => (
                <OrderCard 
                  key={order.id} 
                  order={order} 
                  isLoadedMap={isLoaded}
                  driverLocation={driverLocation}
                  fullRoute={[DefaultMerchantLoc, ...arr.map(o => parsePoint(o.delivery_location))]}
                  isNextStop={index === 0 && arr.length > 1}
                  onStatusUpdate={() => updateOrderStatus(order.id, order.status)} 
                  uploading={uploading === order.id}
                />
              ))}
            </AnimatePresence>
          </div>
        )}
      </main>

      {/* Hidden File Input for Capture */}
      <input 
        type="file" 
        ref={fileInputRef} 
        onChange={handleFileUpload} 
        accept="image/*" 
        capture="environment" 
        className="hidden" 
      />
    </div>
  );
}

// Subcomponente de la Carta de Pedido
function OrderCard({ order, isLoadedMap, driverLocation, fullRoute, isNextStop, onStatusUpdate, uploading }: { order: Order; isLoadedMap: boolean; driverLocation: {lat: number, lng: number} | null; fullRoute: {lat:number, lng:number}[]; isNextStop: boolean; onStatusUpdate: () => void; uploading: boolean }) {
  const isAssigned = order.status === 'assigned';
  const isPickedUp = order.status === 'picked_up';

  const customerLoc = parsePoint(order.delivery_location);

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
            <div className={`text-xs font-bold tracking-wider mb-1 uppercase flex items-center gap-2
                ${isAssigned ? 'text-orange-600' : 'text-indigo-600'}
              `}>
              {isAssigned ? 'Recién Asignado' : 'En Tránsito'}
              {isNextStop && (
                <span className="ml-2 bg-indigo-600 text-white px-2 py-0.5 rounded-full text-[10px] shadow-sm animate-pulse">Siguiente Parada</span>
              )}
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

        {/* Indicador de Pago */}
        <div className="mb-4 flex flex-col gap-2">
            {order.payment_method === 'cash' ? (
                <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-bold bg-orange-100 text-orange-700 w-fit">
                   💵 COBRAR EN EFECTIVO: ${order.total_amount.toFixed(2)}
                </span>
            ) : (
                <div className="bg-blue-50 border border-blue-100 rounded-lg p-3 text-xs w-full">
                    <span className="inline-flex items-center gap-1 font-bold text-blue-700 mb-2">
                       🏦 TRANSFERENCIA: ${order.total_amount.toFixed(2)}
                    </span>
                    <div className="text-blue-800 space-y-1 font-mono">
                      <p>A nombre de: <b>Joaquin Carpio Vallejo</b></p>
                      <p>CLABE: <b>002769702231479665</b></p>
                    </div>
                </div>
            )}

            {order.payment_status === 'paid' && (
                <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-semibold bg-emerald-100 text-emerald-700 w-fit">
                   <CheckCircle2 className="w-3.5 h-3.5" /> Pagado
                </span>
            )}
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
          
          {/* Mapa Interactivo Google Maps */}
          {isLoadedMap && (
             <div className="w-full h-48 mt-4 rounded-xl overflow-hidden bg-gray-100 border border-gray-200">
               <GoogleMap
                 mapContainerStyle={{ width: '100%', height: '100%' }}
                 center={DefaultMerchantLoc}
                 zoom={14}
                 options={{ disableDefaultUI: true, gestureHandling: 'cooperative' }}
               >
                 <Marker position={DefaultMerchantLoc} label="M" title="Restaurante" />
                 {fullRoute.slice(1).map((loc, i) => (
                   <Marker key={i} position={loc} label={loc === customerLoc ? "C" : `${i+1}`} title={loc === customerLoc ? "Cliente" : "Parada"} />
                 ))}
                 {driverLocation && <Marker position={driverLocation} label="Yo" icon={{ path: google.maps.SymbolPath.CIRCLE, scale: 7, fillColor: "#4f46e5", fillOpacity: 1, strokeColor: "white", strokeWeight: 2 }} />}
                 <Polyline 
                    path={driverLocation ? [driverLocation, ...fullRoute] : fullRoute} 
                    options={{ strokeColor: '#4f46e5', strokeOpacity: 0.8, strokeWeight: 4 }} 
                 />
               </GoogleMap>
             </div>
          )}
        </div>

        {/* Botonera de Acción */}
        <div className="mt-5 pt-4 border-t border-gray-50">
          <button
            onClick={onStatusUpdate}
            disabled={uploading}
            className={`w-full py-3 px-4 rounded-xl font-bold text-sm tracking-wide transition-all shadow-sm flex items-center justify-center gap-2
              ${isAssigned 
                ? 'bg-orange-500 hover:bg-orange-600 text-white' 
                : 'bg-indigo-600 hover:bg-indigo-700 text-white'
              }
              ${uploading ? 'opacity-70 cursor-not-allowed' : ''}
            `}
          >
            {uploading ? (
                <>
                  <Loader2 className="w-5 h-5 animate-spin" />
                  SUBIENDO EVIDENCIA...
                </>
            ) : isAssigned ? (
              <>
                <CheckCircle2 className="w-5 h-5" />
                CONFIRMAR RECOLECCIÓN
              </>
            ) : (
              <>
                <Camera className="w-5 h-5" />
                TOMAR FOTO Y ENTREGAR
              </>
            )}
          </button>
        </div>
      </div>
    </motion.div>
  );
}
