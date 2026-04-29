import type {Metadata} from 'next';
import './globals.css'; // Global styles

export const metadata: Metadata = {
  title: 'My Google AI Studio App',
  description: 'My Google AI Studio App',
};

export default function RootLayout({children}: {children: React.ReactNode}) {
  return (
    <html lang="es">
      <body suppressHydrationWarning className="flex flex-col min-h-screen">
        <main className="flex-grow">
          {children}
        </main>
        <footer className="bg-gray-50 border-t border-gray-100 py-8 px-6">
          <div className="max-w-7xl mx-auto flex flex-col md:flex-row justify-between items-center gap-4 text-xs text-gray-500 font-sans">
            <p>© 2026 SolidBit. Todos los derechos reservados.</p>
            <div className="flex gap-6 uppercase tracking-wider font-semibold">
              <a href="/legal/terms" className="hover:text-indigo-600 transition-colors">Términos</a>
              <a href="/legal/privacy" className="hover:text-indigo-600 transition-colors">Privacidad</a>
              <a href="/legal/refunds" className="hover:text-indigo-600 transition-colors">Reembolsos</a>
            </div>
            <p>{process.env.NEXT_PUBLIC_BUSINESS_ADDRESS || 'Guaymas, Sonora, México'}</p>
          </div>
        </footer>
      </body>
    </html>
  );
}
