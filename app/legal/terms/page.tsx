'use client';

import React from 'react';
import { motion } from 'motion/react';
import { ShieldCheck, FileText, Scale } from 'lucide-react';

export default function TermsPage() {
  return (
    <div className="min-h-screen bg-white font-sans text-gray-800 p-6 md:p-12 lg:p-24 overflow-auto">
      <motion.div 
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        className="max-w-3xl mx-auto"
      >
        <div className="flex items-center gap-3 mb-8 text-indigo-600">
           <Scale className="w-8 h-8" />
           <h1 className="text-4xl font-bold tracking-tight">Términos de Servicio</h1>
        </div>
        
        <div className="prose prose-indigo max-w-none space-y-8">
          <section>
            <h2 className="text-2xl font-bold text-gray-900 mb-4">1. Aceptación de los Términos</h2>
            <p>
              Al acceder y utilizar **SolidBit**, usted acepta estar sujeto a estos Términos y Condiciones. Si no está de acuerdo con alguno de estos términos, le pedimos que no utilice el servicio. SolidBit actúa como una plataforma de intermediación logística entre comerciantes (merchants) y clientes finales.
            </p>
          </section>

          <section>
            <h2 className="text-2xl font-bold text-gray-900 mb-4">2. Uso del Servicio</h2>
            <p>
              SolidBit es una plataforma automatizada vía WhatsApp que procesa pedidos de alimentos y mercancías. Usted es responsable de proporcionar información de entrega veraz y completa. Nos reservamos el derecho de suspender el servicio ante cualquier uso fraudulento o abusivo.
            </p>
          </section>

          <section>
            <h2 className="text-2xl font-bold text-gray-900 mb-4">3. Responsabilidad sobre Productos</h2>
            <p>
              SolidBit facilita la entrega. La calidad, preparación, empaque y exactitud de los alimentos o productos son responsabilidad exclusiva del comercio (merchants). Cualquier problema relacionado con la calidad del producto debe ser dirigido directamente al establecimiento emisor.
            </p>
          </section>

          <section>
            <h2 className="text-2xl font-bold text-gray-900 mb-4">4. Tarifas y Pagos</h2>
            <p>
              Los precios de los productos son fijados por los comerciantes. SolidBit añade una tarifa de envío visible antes de finalizar el pago. Los pagos se procesan a través de la pasarela segura **Stripe**. Al completar una transacción, usted autoriza el cobro del monto total especificado.
            </p>
          </section>

          <section>
            <h2 className="text-2xl font-bold text-gray-900 mb-4">5. Legislación Aplicable</h2>
            <p>
              Estos términos se rigen e interpretan de acuerdo con las leyes de los Estados Unidos Mexicanos, específicamente bajo la jurisdicción del estado de Sonora.
            </p>
          </section>

          <div className="pt-12 border-t border-gray-100 mt-12 italic text-sm text-gray-500">
            Última actualización: 29 de abril de 2026. SolidBit Logistic Solutions.
          </div>
        </div>
      </motion.div>
    </div>
  );
}
