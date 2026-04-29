'use client';

import React from 'react';
import { motion } from 'motion/react';
import { CheckCircle, Mail, MapPin, Truck } from 'lucide-react';
import { useParams } from 'next/navigation';

export default function OrderSuccessPage() {
  const { id } = useParams();

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col font-sans p-4">
      <motion.div 
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        className="w-full max-w-md mx-auto bg-white rounded-3xl shadow-sm border border-gray-100 overflow-hidden mt-12"
      >
        <div className="bg-green-500 p-8 text-white text-center">
            <CheckCircle className="w-16 h-16 mx-auto mb-4" />
            <h1 className="text-2xl font-bold mb-1">¡Pago Exitoso!</h1>
            <p className="text-green-100 text-sm">Tu pedido está siendo procesado.</p>
        </div>

        <div className="p-8">
          <div className="space-y-6">
            <div className="text-center">
              <p className="text-sm font-semibold text-gray-400 uppercase tracking-wider mb-1">Número de Pedido</p>
              <p className="text-lg font-mono text-gray-900">{typeof id === 'string' ? id.toUpperCase() : '...'}</p>
            </div>

            <div className="bg-blue-50 p-6 rounded-2xl border border-blue-100">
               <h3 className="text-blue-900 font-bold flex items-center gap-2 mb-2">
                 <Truck className="w-5 h-5 text-blue-600" /> ¿Qué sigue?
               </h3>
               <ul className="text-sm text-blue-800 space-y-2">
                 <li>• El restaurante confirmará tu pedido en breve.</li>
                 <li>• Recibirás una notificación por WhatsApp cuando el repartidor esté en camino.</li>
               </ul>
            </div>

            <div className="pt-6 border-t border-gray-100 space-y-4">
              <div className="flex items-start gap-4">
                 <div className="w-10 h-10 bg-gray-100 rounded-full flex items-center justify-center flex-shrink-0">
                    <Mail className="w-5 h-5 text-gray-500" />
                 </div>
                 <div>
                    <h4 className="text-sm font-bold text-gray-900">Soporte y Reclamaciones</h4>
                    <p className="text-sm text-gray-500">soporte@solidbit.app</p>
                 </div>
              </div>

              <div className="flex items-start gap-4">
                 <div className="w-10 h-10 bg-gray-100 rounded-full flex items-center justify-center flex-shrink-0">
                    <MapPin className="w-5 h-5 text-gray-500" />
                 </div>
                 <div>
                    <h4 className="text-sm font-bold text-gray-900">Dirección Fiscal</h4>
                    <p className="text-sm text-gray-500 leading-tight">
                      Av. Tecnológico 100, Guaymas, Sonora, CP 85400, México.
                    </p>
                 </div>
              </div>
            </div>

            <button 
              onClick={() => window.location.href = '/'}
              className="w-full bg-gray-900 hover:bg-black text-white font-bold py-4 rounded-xl transition-all mt-4"
            >
              Volver al inicio
            </button>
          </div>
        </div>
      </motion.div>
    </div>
  );
}
