'use client';

import React, { useEffect, useState } from 'react';
import { supabase } from '@/lib/supabase';
import { motion } from 'motion/react';
import { 
  Building2, LineChart, Wallet, DollarSign, PackageCheck, Users, Box, Navigation
} from 'lucide-react';
import { GoogleMap, useLoadScript, Marker } from '@react-google-maps/api';

// Types
interface Metrics {
  total_stripe: number;
  total_cash: number;
  delivered_today: number;
}

interface PerformanceDriver {
  id: string;
  name: string;
  status: string;
  deliveries: number;
}

export default function AdminDashboardPage() {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [passInput, setPassInput] = useState("");

  const [metrics, setMetrics] = useState<Metrics>({ total_stripe: 0, total_cash: 0, delivered_today: 0 });
  const [drivers, setDrivers] = useState<PerformanceDriver[]>([]);
  const [liveMapData, setLiveMapData] = useState<{ active_orders: any[], active_drivers: any[] }>({ active_orders: [], active_drivers: [] });

  const { isLoaded } = useLoadScript({
    googleMapsApiKey: process.env.NEXT_PUBLIC_MAPS_API_KEY || '',
  });

  const handleLogin = (e: React.FormEvent) => {
    e.preventDefault();
    if (passInput === process.env.NEXT_PUBLIC_ADMIN_PASSWORD) {
      setIsAuthenticated(true);
    } else {
      alert("Contraseña incorrecta");
    }
  };

  const fetchInitialData = async () => {
    // Para simplificar, en lugar de llamar al backend de Go o lidiar con CORS si no corren en el mismo puerto,
    // usaremos peticiones directas a Supabase para la vista materializada y datos básicos como solicitaba SolidBit.

    try {
      // Fetch Analytics View (Requiere que la vista sea seleccionable anónimamente si no usamos RLS avanzado)
      const { data: metricsData, error: metricsErr } = await supabase
        .from('admin_metrics')
        .select('*')
        .single();
      
      if (!metricsErr && metricsData) {
        setMetrics(metricsData);
      } else {
        // En caso de que RLS no permita leer la vista, podríamos sumar aquí en el cliente
        // console.error(metricsErr);
      }

      // Fetch Drivers para la tabla de rendimiento
      const { data: driversData, error: drvErr } = await supabase
        .from('drivers')
        .select('id, name, status');
      
      if (!drvErr && driversData) {
        // Mock de entregas, o lo ideal sería un COUNT agrupado
        const mapped = driversData.map(d => ({ ...d, deliveries: Math.floor(Math.random() * 10) }));
        setDrivers(mapped);
        
        // Populate live map data mock
        setLiveMapData({
          active_orders: [], 
          active_drivers: driversData.map(d => ({...d, location: { lat: 27.9667 + (Math.random()*0.01), lng: -110.8988 + (Math.random()*0.01) }}))
        });
      }

    } catch(e) {
      console.error(e);
    }
  };

  useEffect(() => {
    if (!isAuthenticated) return;

    fetchInitialData();

    // Supabase Realtime para actualizar la info si cambia
    // (Por ej, si se completa un pago o se entrega un pedido)
    const channel = supabase
      .channel('admin:changes')
      .on('postgres_changes', { event: '*', schema: 'public', table: 'orders' }, () => {
        fetchInitialData();
      })
      .subscribe();

    return () => {
      supabase.removeChannel(channel);
    };
  }, [isAuthenticated]);

  if (!isAuthenticated) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center font-sans text-gray-900">
        <form onSubmit={handleLogin} className="bg-white p-8 rounded-2xl shadow-sm border border-gray-100 max-w-sm w-full">
          <div className="flex justify-center mb-6">
            <div className="w-12 h-12 bg-indigo-600 rounded-xl flex items-center justify-center text-white shadow-lg shadow-indigo-200">
              <Building2 className="w-6 h-6" />
            </div>
          </div>
          <h1 className="text-2xl font-bold text-center mb-6">SolidBit Admin</h1>
          <input 
            type="password"
            value={passInput}
            onChange={(e)=>setPassInput(e.target.value)}
            className="w-full px-4 py-3 border border-gray-200 rounded-xl mb-4 focus:outline-none focus:ring-2 focus:ring-indigo-500"
            placeholder="Contraseña Maestra"
          />
          <button type="submit" className="w-full bg-indigo-900 text-white font-bold py-3 rounded-xl transition hover:bg-indigo-800">
            Ingresar a la Torre
          </button>
        </form>
      </div>
    );
  }

  const defaultCenter = { lat: 27.9678, lng: -110.8988 }; // Empalme

  return (
    <div className="min-h-screen bg-gray-100 font-sans text-gray-900 flex flex-col">
      <header className="bg-indigo-950 text-white px-6 py-4 flex items-center justify-between">
         <div className="flex items-center gap-3">
            <Building2 className="w-6 h-6 text-indigo-300" />
            <div>
               <h1 className="text-xl font-bold tracking-tight">SolidBit Torret</h1>
               <p className="text-xs text-indigo-300 font-medium tracking-wide">BUSINESS INTELLIGENCE & CONTROL</p>
            </div>
         </div>
      </header>

      <main className="flex-1 p-6 max-w-7xl mx-auto w-full grid grid-cols-1 lg:grid-cols-3 gap-6">
        
        {/* Lado Izquierdo: KPI & TABLA */}
        <div className="lg:col-span-1 space-y-6">
          <h2 className="text-lg font-bold text-gray-800 flex items-center gap-2">
            <LineChart className="w-5 h-5" /> Métricas Globales
          </h2>
          
          <div className="grid grid-cols-2 gap-4">
            <motion.div initial={{opacity:0, y: 10}} animate={{opacity:1, y:0}} className="bg-white p-4 rounded-2xl shadow-sm border border-gray-100">
               <div className="text-gray-400 mb-2"><Wallet className="w-5 h-5"/></div>
               <p className="text-xs text-gray-500 font-semibold mb-1">STRIPE DIGITAL</p>
               <h3 className="text-xl font-bold text-indigo-600">${metrics.total_stripe?.toFixed(2) || '0.00'}</h3>
            </motion.div>
            <motion.div initial={{opacity:0, y: 10}} animate={{opacity:1, y:0}} transition={{delay: 0.1}} className="bg-white p-4 rounded-2xl shadow-sm border border-gray-100">
               <div className="text-gray-400 mb-2"><DollarSign className="w-5 h-5"/></div>
               <p className="text-xs text-gray-500 font-semibold mb-1">EFECTIVO FÍSICO</p>
               <h3 className="text-xl font-bold text-emerald-600">${metrics.total_cash?.toFixed(2) || '0.00'}</h3>
            </motion.div>
            <motion.div initial={{opacity:0, y: 10}} animate={{opacity:1, y:0}} transition={{delay: 0.2}} className="col-span-2 bg-white p-4 rounded-2xl shadow-sm border border-gray-100 flex items-center justify-between">
               <div>
                  <p className="text-xs text-gray-500 font-semibold mb-1">ENTREGAS DE HOY</p>
                  <h3 className="text-2xl font-bold text-gray-900">{metrics.delivered_today || 0}</h3>
               </div>
               <div className="w-12 h-12 bg-green-50 text-green-600 rounded-full flex items-center justify-center">
                  <PackageCheck className="w-6 h-6"/>
               </div>
            </motion.div>
          </div>

          <h2 className="text-lg font-bold text-gray-800 flex items-center gap-2 mt-8">
            <Users className="w-5 h-5" /> Rendimiento de Flota
          </h2>
          <div className="bg-white rounded-2xl shadow-sm border border-gray-100 overflow-hidden">
             {drivers.map((drv, i) => (
               <div key={drv.id} className={`flex items-center justify-between p-4 ${i !== drivers.length - 1 ? 'border-b border-gray-50' : ''}`}>
                  <div className="flex items-center gap-3">
                     <div className={`w-2 h-2 rounded-full ${drv.status === 'available' ? 'bg-emerald-500' : 'bg-gray-300'}`} />
                     <p className="font-semibold text-sm text-gray-900">{drv.name}</p>
                  </div>
                  <div className="text-xs font-bold bg-gray-100 text-gray-600 px-2 py-1 rounded">
                     {drv.deliveries} Entregas
                  </div>
               </div>
             ))}
          </div>

        </div>

        {/* Lado Derecho: MAPA GLOBAL */}
        <div className="lg:col-span-2 flex flex-col">
          <h2 className="text-lg font-bold text-gray-800 flex items-center gap-2 mb-6">
            <Navigation className="w-5 h-5" /> Mapa en Tiempo Real
          </h2>
          <div className="flex-1 bg-white rounded-2xl overflow-hidden shadow-sm border border-gray-100 min-h-[500px] relative">
            {isLoaded ? (
              <GoogleMap
                 mapContainerStyle={{ width: '100%', height: '100%' }}
                 center={defaultCenter}
                 zoom={13}
                 options={{ disableDefaultUI: true }}
               >
                 {liveMapData.active_drivers.map(drv => (
                    <Marker key={drv.id} position={drv.location} label={`R-${drv.name.charAt(0)}`} icon={{ path: google.maps.SymbolPath.CIRCLE, scale: 6, fillColor: "#22c55e", fillOpacity: 1, strokeColor: "white", strokeWeight: 2 }} />
                 ))}
                 {liveMapData.active_orders.map(ord => (
                    <Marker key={ord.id} position={ord.location} label="P" />
                 ))}
               </GoogleMap>
            ) : (
              <div className="absolute inset-0 flex items-center justify-center bg-gray-50">
                 <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600"></div>
              </div>
            )}
          </div>
        </div>

      </main>
    </div>
  );
}
