import type {Metadata} from 'next';
import './globals.css'; // Global styles

export const metadata: Metadata = {
  title: 'My Google AI Studio App',
  description: 'My Google AI Studio App',
};

export default function RootLayout({children}: {children: React.ReactNode}) {
  return (
    <html lang="es">
      <body suppressHydrationWarning className="flex flex-col min-h-screen bg-neutral-950 text-neutral-50">
        <main className="flex-grow">
          {children}
        </main>
        <footer className="bg-neutral-950 border-t border-white/5 py-8 px-6">
          <div className="max-w-7xl mx-auto flex flex-col md:flex-row justify-between items-center gap-4 text-xs text-neutral-500 font-sans">
            <p>© 2026 SolidBit. Todos los derechos reservados.</p>
            <div className="flex gap-6 uppercase tracking-wider font-semibold">
              <span className="text-neutral-600">Confidencial - Uso Interno</span>
            </div>
            <p>{process.env.NEXT_PUBLIC_BUSINESS_ADDRESS || 'Guaymas, Sonora, México'}</p>
          </div>
        </footer>
      </body>
    </html>
  );
}
