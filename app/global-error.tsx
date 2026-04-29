'use client';

import { useEffect } from 'react';
import { supabase } from '@/lib/supabase';

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  useEffect(() => {
    // Registramos el error en Supabase para el panel de administración
    async function logError() {
      try {
        await supabase.from('frontend_errors').insert([
          {
            error_message: error.message,
            error_stack: error.stack,
          }
        ]);
        console.log("Error logged to Supabase");
      } catch (e) {
        console.error("Failed to log frontend error:", e);
      }
    }
    logError();
  }, [error]);

  return (
    <html>
      <body>
        <div className="min-h-screen bg-gray-50 flex items-center justify-center p-4">
          <div className="bg-white p-8 rounded-2xl shadow-sm border border-gray-100 max-w-sm w-full text-center">
            <div className="w-16 h-16 bg-red-100 rounded-full flex items-center justify-center mx-auto mb-4">
              <span className="text-3xl">🦺</span>
            </div>
            <h2 className="text-xl font-bold text-gray-900 mb-2">¡Algo salió mal!</h2>
            <p className="text-gray-500 text-sm mb-6">
              El panel experimentó un problema imprevisto y el administrador ya ha sido notificado silenciosamente.
            </p>
            <button
              onClick={() => reset()}
              className="w-full bg-indigo-600 hover:bg-indigo-700 text-white font-bold py-3 rounded-xl transition-all"
            >
              Intentar de nuevo
            </button>
          </div>
        </div>
      </body>
    </html>
  );
}
