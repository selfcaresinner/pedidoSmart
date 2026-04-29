'use client';

import React, { useEffect, useState } from 'react';
import { supabase } from '@/lib/supabase';
import { Package, MapPin, DollarSign, CheckCircle2, XCircle } from 'lucide-react';
import { motion } from 'motion/react';
import { useParams } from 'next/navigation';

interface Order {
  id: string;
  customer_name: string;
  items_description: string;
  total_amount: number;
  status: string;
  confirmed_by_merchant: boolean;
}

export default function MerchantOrderPage() {
  const { id } = useParams();
  const [order, setOrder] = useState<Order | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [processing, setProcessing] = useState(false);

  useEffect(() => {
    if (!id) return;

    async function fetchOrder() {
      try {
        const { data, error } = await supabase
          .from('orders')
          .select('id, customer_name, items_description, total_amount, status, confirmed_by_merchant')
          .eq('id', id)
          .single();

        if (error) throw error;
        setOrder(data);
      } catch (err: any) {
        setError("Error cargando el pedido: " + err.message);
      } finally {
        setLoading(false);
      }
    }
    fetchOrder();
  }, [id]);

  const handleAction = async (action: 'accept' | 'reject') => {
    setProcessing(true);
    try {
      // Llamar al backend Go
      const res = await fetch('/api/merchant/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ order_id: id, action })
      });

      if (!res.ok) {
        throw new Error("Fallo al comunicarse con el servidor.");
      }

      const updatedStatus = action === 'accept' ? 'pending' : 'cancelled';
      setOrder(prev => prev ? { ...prev, confirmed_by_merchant: action === 'accept', status: updatedStatus } : null);
      
    } catch (err: any) {
      alert("Error procesando: " + err.message);
    } finally {
      setProcessing(false);
    }
  };

  if (loading) {
    return <div className="min-h-screen flex items-center justify-center bg-gray-50 text-gray-500">Cargando pedido...</div>;
  }

  if (error || !order) {
    return <div className="min-h-screen flex items-center justify-center bg-gray-50 text-red-500 font-medium">{error || "Pedido no encontrado"}</div>;
  }

  if (order.status === 'cancelled') {
    return (
        <div className="min-h-screen bg-gray-50 flex items-center justify-center p-4">
            <div className="bg-white p-8 rounded-2xl shadow-sm text-center max-w-sm w-full">
               <XCircle className="w-16 h-16 text-red-500 mx-auto mb-4" />
               <h2 className="text-xl font-bold text-gray-900">Pedido Rechazado</h2>
               <p className="text-gray-500 mt-2">Has rechazado este pedido exitosamente.</p>
            </div>
        </div>
    )
  }

  if (order.confirmed_by_merchant) {
    return (
        <div className="min-h-screen bg-gray-50 flex items-center justify-center p-4">
            <div className="bg-white p-8 rounded-2xl shadow-sm text-center max-w-sm w-full border border-gray-100">
               <div className="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-4">
                 <CheckCircle2 className="w-8 h-8 text-green-600" />
               </div>
               <h2 className="text-xl font-bold text-gray-900 mb-2">Pedido en Preparación</h2>
               <p className="text-gray-500">El repartidor de SolidBit ha sido notificado y se dirige a tu negocio.</p>
            </div>
        </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col font-sans p-4">
      <motion.div 
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        className="w-full max-w-md mx-auto bg-white rounded-3xl shadow-sm border border-gray-100 overflow-hidden mt-8"
      >
        <div className="bg-indigo-600 p-6 text-white text-center">
            <h1 className="text-2xl font-bold mb-1">Nuevo Pedido</h1>
            <p className="text-indigo-200 text-sm opacity-80 uppercase tracking-widest">{order.id.split('-')[0]}</p>
        </div>

        <div className="p-6">
          <div className="space-y-6">
            <div>
              <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2 flex items-center gap-2">
                 <Package className="w-3 h-3" />
                 Productos
              </p>
              <p className="text-lg font-medium text-gray-900">{order.items_description || '1x Pedido Genérico'}</p>
            </div>

            <div>
              <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2 flex items-center gap-2">
                 <MapPin className="w-3 h-3" />
                 Cliente
              </p>
              <p className="text-base text-gray-900">{order.customer_name}</p>
            </div>

            <div className="pt-4 border-t border-gray-100">
              <div className="flex justify-between items-center bg-gray-50 p-4 rounded-2xl">
                 <span className="text-sm font-semibold text-gray-500 uppercase flex items-center gap-2">
                    <DollarSign className="w-4 h-4 text-indigo-500" /> Monto Total
                 </span>
                 <span className="text-2xl font-bold text-indigo-600">${order.total_amount}</span>
              </div>
            </div>
          </div>

          <div className="mt-8 space-y-3">
            <button
               onClick={() => handleAction('accept')}
               disabled={processing}
               className="w-full bg-indigo-600 hover:bg-indigo-700 text-white font-bold py-4 rounded-xl transition-all shadow-md shadow-indigo-200 flex items-center justify-center gap-2"
            >
              {processing ? 'Procesando...' : (
                  <>
                    <CheckCircle2 className="w-5 h-5" /> Aceptar y Preparar
                  </>
              )}
            </button>
            <button
               onClick={() => handleAction('reject')}
               disabled={processing}
               className="w-full bg-white border-2 border-red-100 hover:border-red-200 hover:bg-red-50 text-red-600 font-bold py-4 rounded-xl transition-all"
            >
              Rechazar Pedido
            </button>
          </div>
        </div>
      </motion.div>
    </div>
  );
}
