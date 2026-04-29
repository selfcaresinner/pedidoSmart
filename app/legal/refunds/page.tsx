'use client';

import React from 'react';
import { motion } from 'motion/react';
import { RefreshCcw, AlertCircle, CheckCircle } from 'lucide-react';

export default function RefundsPage() {
  return (
    <div className="min-h-screen bg-white font-sans text-gray-800 p-6 md:p-12 lg:p-24 overflow-auto">
      <motion.div 
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        className="max-w-3xl mx-auto"
      >
        <div className="flex items-center gap-3 mb-8 text-indigo-600">
           <RefreshCcw className="w-8 h-8" />
           <h1 className="text-4xl font-bold tracking-tight">Política de Reembolsos</h1>
        </div>
        
        <div className="prose prose-indigo max-w-none space-y-8">
          <section>
            <h2 className="text-2xl font-bold text-gray-900 mb-4">1. Política de Cancelación</h2>
            <p>
              Usted puede cancelar su pedido en cualquier momento **antes** de que el comercio (merchant) haya aceptado y comenzado la preparación del mismo. Una vez que el comercio acepta el pedido, no se podrán realizar cancelaciones ni reembolsos por arrepentimiento de compra.
            </p>
          </section>

          <section>
            <h2 className="text-2xl font-bold text-gray-900 mb-4">2. Reembolsos por Errores en el Pedido</h2>
            <p>
              Si el pedido recibido es incorrecto (productos diferentes a los solicitados) o falta algún artículo, el cliente debe notificar inmediatamente a través de nuestro soporte técnico en WhatsApp. SolidBit coordinará con el comercio para emitir un reembolso parcial o total vía Stripe.
            </p>
          </section>

          <section>
            <h2 className="text-2xl font-bold text-gray-900 mb-4">3. Plazos de Devolución</h2>
            <p>
              Una vez autorizado un reembolso por SolidBit, el tiempo de procesamiento depende de Stripe y su institución bancaria, el cual generalmente toma entre **5 a 10 días hábiles** en verse reflejado en su estado de cuenta.
            </p>
          </section>

          <section>
            <h2 className="text-2xl font-bold text-gray-900 mb-4">4. No Entrega</h2>
            <p>
              En caso de que el repartidor no logre contactar al cliente en la dirección proporcionada tras varios intentos, el pedido será marcado como fallido y **no será elegible para reembolso**, debido a los costos ya incurridos en preparación y logística.
            </p>
          </section>

          <div className="pt-12 border-t border-gray-100 mt-12 italic text-sm text-gray-500">
            Para dudas sobre cobros específicos, contáctenos en: soporte@solidbit.app
          </div>
        </div>
      </motion.div>
    </div>
  );
}
