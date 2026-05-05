'use client';

import React, { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { supabase } from '@/lib/supabase';
import { motion } from 'motion/react';
import { 
  Building2, LineChart, Wallet, DollarSign, PackageCheck, Users, Box, Navigation, Wrench
} from 'lucide-react';
import { GoogleMap, useLoadScript, Marker } from '@react-google-maps/api';

// Types
interface Metrics {
  total_transfers: number;
  total_cash: number;
  total_settled: number;
  net_profit: number;
  pure_profit: number;
  maintenance_fund: number;
  delivered_today: number;
}

interface Wallet {
  driver_id: string;
  cash_on_hand: number;
  updated_at: string;
}

interface Settlement {
  id: string;
  driver_id: string;
  amount: number;
  created_at: string;
}

interface PerformanceDriver {
  id: string;
  name: string;
  status: string;
  deliveries: number;
  wallet?: Wallet;
}

export default function AdminDashboardPage() {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const router = useRouter();

  const [metrics, setMetrics] = useState<Metrics>({ total_transfers: 0, total_cash: 0, total_settled: 0, net_profit: 0, pure_profit: 0, maintenance_fund: 0, delivered_today: 0 });

  // Add useRouter import and check auth on mount
  useEffect(() => {
    const adminCode = localStorage.getItem('solidbit_admin_auth');
    if (!adminCode) {
      router.push('/');
    } else {
      setIsAuthenticated(true);
    }
  }, [router]);
  const [drivers, setDrivers] = useState<PerformanceDriver[]>([]);
  const [settlements, setSettlements] = useState<Settlement[]>([]);
  const [liveMapData, setLiveMapData] = useState<{ active_orders: any[], active_drivers: any[] }>({ active_orders: [], active_drivers: [] });
  const [settlingDriver, setSettlingDriver] = useState<PerformanceDriver | null>(null);
  const [settleAmount, setSettleAmount] = useState<string>("");
  const [isSettling, setIsSettling] = useState(false);

  const { isLoaded } = useLoadScript({
    googleMapsApiKey: process.env.NEXT_PUBLIC_MAPS_API_KEY || '',
  });

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

      // Fetch Drivers y Wallets
      const { data: driversData, error: drvErr } = await supabase
        .from('drivers')
        .select('*, driver_wallets(*)');
      
      if (!drvErr && driversData) {
        setDrivers(driversData.map(d => ({
          ...d, 
          deliveries: 0, // In reality count from orders
          wallet: d.driver_wallets ? d.driver_wallets[0] : undefined
        })));
        
        // Populate live map data mock
        setLiveMapData({
          active_orders: [], 
          active_drivers: driversData.map(d => ({...d, location: { lat: 27.9667 + (Math.random()*0.01), lng: -110.8988 + (Math.random()*0.01) }}))
        });
      }

      // Fetch Recent Settlements
      const { data: settleData } = await supabase
        .from('settlements')
        .select('*, drivers(name)')
        .order('created_at', { ascending: false })
        .limit(5);
      
      if (settleData) {
        setSettlements(settleData as any);
      }

    } catch(e) {
      console.error(e);
    }
  };

  useEffect(() => {
    if (!isAuthenticated) return;

    const init = async () => {
      await fetchInitialData();
    };
    init();

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

  const handleSettle = async () => {
    if (!settlingDriver || !settleAmount || parseFloat(settleAmount) <= 0) return;
    
    setIsSettling(true);
    try {
      const adminCode = localStorage.getItem('solidbit_admin_auth') || '';
      const resp = await fetch('/api/admin/settle', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Admin-Password': adminCode
        },
        body: JSON.stringify({
          driver_id: settlingDriver.id,
          amount: parseFloat(settleAmount)
        })
      });

      if (resp.ok) {
        setSettlingDriver(null);
        setSettleAmount("");
        fetchInitialData();
      } else {
        const txt = await resp.text();
        alert("Error: " + txt);
      }
    } catch (e) {
      console.error(e);
    } finally {
      setIsSettling(false);
    }
  };

  if (!isAuthenticated) return null;

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
            <motion.div initial={{opacity:0, y: 10}} animate={{opacity:1, y:0}} className="bg-indigo-900 p-4 rounded-2xl shadow-sm border border-indigo-800 col-span-2 flex flex-col sm:flex-row justify-between sm:items-end gap-4">
               <div>
                 <div className="text-indigo-300 mb-2"><Building2 className="w-5 h-5"/></div>
                 <p className="text-xs text-indigo-300 font-semibold mb-1">PROFIT PURO PLATAFORMA (Libre)</p>
                 <h3 className="text-2xl font-bold text-white">${metrics.pure_profit?.toFixed(2) || '0.00'}</h3>
               </div>
               <div className="sm:text-right border-t border-indigo-800 sm:border-t-0 sm:border-l sm:pl-4 pt-4 sm:pt-0">
                 <div className="text-rose-300 mb-2 sm:ml-auto"><Wrench className="w-5 h-5"/></div>
                 <p className="text-xs text-rose-300 font-semibold mb-1">FONDO DE MANTENIMIENTO</p>
                 <h3 className="text-xl font-bold text-rose-100">${metrics.maintenance_fund?.toFixed(2) || '0.00'}</h3>
               </div>
            </motion.div>
            <motion.div initial={{opacity:0, y: 10}} animate={{opacity:1, y:0}} className="bg-white p-4 rounded-2xl shadow-sm border border-gray-100">
               <div className="text-gray-400 mb-2"><Wallet className="w-5 h-5"/></div>
               <p className="text-xs text-gray-500 font-semibold mb-1">TRANSFERENCIAS</p>
               <h3 className="text-xl font-bold text-indigo-600">${metrics.total_transfers?.toFixed(2) || '0.00'}</h3>
            </motion.div>
            <motion.div initial={{opacity:0, y: 10}} animate={{opacity:1, y:0}} transition={{delay: 0.1}} className="bg-white p-4 rounded-2xl shadow-sm border border-gray-100">
               <div className="text-gray-400 mb-2"><DollarSign className="w-5 h-5"/></div>
               <p className="text-xs text-gray-500 font-semibold mb-1">EFECTIVO EN CALLE</p>
               <h3 className="text-xl font-bold text-orange-600">${metrics.total_cash?.toFixed(2) || '0.00'}</h3>
            </motion.div>
            <motion.div initial={{opacity:0, y: 10}} animate={{opacity:1, y:0}} transition={{delay: 0.2}} className="bg-white p-4 rounded-2xl shadow-sm border border-gray-100">
               <div className="text-gray-400 mb-2"><PackageCheck className="w-5 h-5"/></div>
               <p className="text-xs text-gray-500 font-semibold mb-1">LIQUIDADO</p>
               <h3 className="text-xl font-bold text-emerald-600">${metrics.total_settled?.toFixed(2) || '0.00'}</h3>
            </motion.div>
            <motion.div initial={{opacity:0, y: 10}} animate={{opacity:1, y:0}} transition={{delay: 0.3}} className="bg-white p-4 rounded-2xl shadow-sm border border-gray-100">
               <div className="text-gray-400 mb-2"><Box className="w-5 h-5"/></div>
               <p className="text-xs text-gray-500 font-semibold mb-1">ENTREGAS HOY</p>
               <h3 className="text-xl font-bold text-gray-900">{metrics.delivered_today || 0}</h3>
            </motion.div>
          </div>

          <h2 className="text-lg font-bold text-gray-800 flex items-center gap-2 mt-8">
            <Users className="w-5 h-5" /> Arqueo de Caja (Repartidores)
          </h2>
          <div className="bg-white rounded-2xl shadow-sm border border-gray-100 overflow-hidden">
             {drivers.map((drv, i) => (
               <div key={drv.id} className={`flex items-center justify-between p-4 ${i !== drivers.length - 1 ? 'border-b border-gray-50' : ''}`}>
                  <div className="flex items-center gap-3">
                     <div className={`w-2 h-2 rounded-full ${drv.status === 'available' ? 'bg-emerald-500' : 'bg-gray-300'}`} />
                     <div>
                        <p className="font-semibold text-sm text-gray-900">{drv.name}</p>
                        <p className="text-[10px] text-gray-400 uppercase font-bold">DEUDA: ${(drv.wallet?.cash_on_hand || 0).toFixed(2)}</p>
                     </div>
                  </div>
                  <button 
                    onClick={() => setSettlingDriver(drv)}
                    disabled={(drv.wallet?.cash_on_hand || 0) <= 0}
                    className="text-[10px] font-bold bg-indigo-50 text-indigo-700 px-3 py-1.5 rounded-lg hover:bg-indigo-100 disabled:opacity-50 disabled:bg-gray-50 disabled:text-gray-400 transition-colors"
                  >
                     LIQUIDAR
                  </button>
               </div>
             ))}
          </div>

          <h2 className="text-xs font-bold text-gray-400 uppercase tracking-widest mt-8 mb-3">
             Historial de Liquidaciones
          </h2>
          <div className="space-y-2">
             {settlements.map((s: any) => (
                <div key={s.id} className="bg-white p-3 rounded-xl border border-gray-50 flex items-center justify-between shadow-sm">
                   <div>
                      <p className="text-xs font-bold text-gray-700">{s.drivers?.name}</p>
                      <p className="text-[9px] text-gray-400">{new Date(s.created_at).toLocaleString()}</p>
                   </div>
                   <p className="text-sm font-bold text-emerald-600">+${s.amount.toFixed(2)}</p>
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

      {/* Modal de Liquidación */}
      {settlingDriver && (
         <div className="fixed inset-0 z-50 bg-indigo-950/40 backdrop-blur-sm flex items-center justify-center p-4">
            <motion.div initial={{scale: 0.9, opacity: 0}} animate={{scale: 1, opacity: 1}} className="bg-white rounded-3xl shadow-2xl p-8 max-w-sm w-full border border-gray-100">
               <h3 className="text-lg font-bold text-gray-900 mb-2">Liquidar efectivo</h3>
               <p className="text-sm text-gray-500 mb-6">Registra el dinero recibido físicamente de <b>{settlingDriver.name}</b>.</p>
               
               <div className="mb-6">
                  <label className="text-[10px] font-bold text-gray-400 uppercase mb-2 block">Monto a liquidar</label>
                  <div className="relative">
                     <span className="absolute left-4 top-1/2 -translate-y-1/2 text-gray-400 font-bold">$</span>
                     <input 
                        type="number"
                        value={settleAmount}
                        onChange={(e)=>setSettleAmount(e.target.value)}
                        placeholder={(settlingDriver.wallet?.cash_on_hand || 0).toString()}
                        className="w-full bg-gray-50 border-0 rounded-2xl py-4 pl-8 pr-4 text-xl font-bold focus:ring-2 focus:ring-indigo-600 transition-all"
                     />
                  </div>
                  <p className="text-[10px] text-indigo-600 font-bold mt-2">DEUDA TOTAL: ${(settlingDriver.wallet?.cash_on_hand || 0).toFixed(2)}</p>
               </div>

               <div className="flex gap-3">
                  <button 
                    onClick={() => setSettlingDriver(null)}
                    className="flex-1 py-3 text-gray-500 font-bold text-sm hover:bg-gray-50 rounded-2xl transition-colors"
                  >
                     Cancelar
                  </button>
                  <button 
                    onClick={handleSettle}
                    disabled={isSettling || !settleAmount || parseFloat(settleAmount) > (settlingDriver.wallet?.cash_on_hand || 0)}
                    className="flex-[2] bg-indigo-900 text-white py-3 rounded-2xl font-bold text-sm hover:bg-indigo-800 disabled:opacity-50 shadow-lg shadow-indigo-100 transition-all"
                  >
                     {isSettling ? "Procesando..." : "Confirmar Recepción"}
                  </button>
               </div>
            </motion.div>
         </div>
      )}
    </div>
  );
}
