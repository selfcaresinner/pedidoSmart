'use client';

import React from 'react';
import { motion } from 'motion/react';
import { ShieldCheck, Lock, Eye } from 'lucide-react';

export default function PrivacyPage() {
  return (
    <div className="min-h-screen bg-white font-sans text-gray-800 p-6 md:p-12 lg:p-24 overflow-auto">
      <motion.div 
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        className="max-w-3xl mx-auto"
      >
        <div className="flex items-center gap-3 mb-8 text-indigo-600">
           <ShieldCheck className="w-8 h-8" />
           <h1 className="text-4xl font-bold tracking-tight">Política de Privacidad</h1>
        </div>
        
        <div className="prose prose-indigo max-w-none space-y-8">
          <section>
            <h2 className="text-2xl font-bold text-gray-900 mb-4">1. Datos que Recolectamos</h2>
            <p>
              En SolidBit recolectamos la información estrictamente necesaria para completar su entrega:
            </p>
            <ul className="list-disc pl-6 space-y-2">
              <li>Número de teléfono (vía WhatsApp).</li>
              <li>Nombre del cliente.</li>
              <li>Ubicación geográfica aproximada o exacta para el delivery.</li>
              <li>Detalles del pedido.</li>
            </ul>
          </section>

          <section>
            <h2 className="text-2xl font-bold text-gray-900 mb-4">2. Uso de la Información</h2>
            <p>
              Su información se utiliza exclusivamente para:
            </p>
            <ul className="list-disc pl-6 space-y-2">
              <li>Notificarle sobre el estado de su pedido.</li>
              <li>Permitir que el repartidor encuentre su ubicación.</li>
              <li>Procesar el pago de forma segura a través de Stripe.</li>
              <li>Mejorar nuestro servicio de inteligencia artificial (Gemini) mediante el análisis de texto de pedidos.</li>
            </ul>
          </section>

          <section>
            <h2 className="text-2xl font-bold text-gray-900 mb-4">3. Seguridad de Pagos</h2>
            <p>
              SolidBit **no almacena** los datos de su tarjeta de crédito o débito. Todas las transacciones financieras son manejadas directamente por **Stripe**, cumpliendo con los estándares de seguridad PCI-DSS.
            </p>
          </section>

          <section>
            <h2 className="text-2xl font-bold text-gray-900 mb-4">4. Terceros con Acceso</h2>
            <p>
              Compartimos datos limitados con:
            </p>
            <ul className="list-disc pl-6 space-y-2">
              <li>**Meta (WhatsApp Business API):** Para la comunicación.</li>
              <li>**Google Maps:** Para la geocodificación de su dirección.</li>
              <li>**Google Cloud (Gemini):** Para el procesamiento de lenguaje de su pedido.</li>
            </ul>
          </section>

          <div className="pt-12 border-t border-gray-100 mt-12 italic text-sm text-gray-500">
            SolidBit se compromete a proteger su privacidad bajo la Ley Federal de Protección de Datos Personales en Posesión de Particulares de México.
          </div>
        </div>
      </motion.div>
    </div>
  );
}
