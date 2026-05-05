'use client';

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Lock, Navigation, Hexagon, Loader2, KeyRound } from 'lucide-react';
import { motion, AnimatePresence } from 'motion/react';

export default function LoginPage() {
  const [code, setCode] = useState('');
  const [error, setError] = useState(false);
  const [loading, setLoading] = useState(false);
  const router = useRouter();

  // Handle visual feedback for error
  useEffect(() => {
    if (error) {
      const timer = setTimeout(() => {
        setCode('');
        setError(false);
      }, 800);
      return () => clearTimeout(timer);
    }
  }, [error]);

  const handleKeyPress = (key: string) => {
    if (loading || error) return;
    if (code.length < 12) {
      setCode(prev => prev + key);
    }
  };

  const handleDelete = () => {
    if (loading || error) return;
    setCode(prev => prev.slice(0, -1));
  };

  const handleSubmit = async () => {
    if (code.length === 0) return;
    setLoading(true);

    try {
      // Admin Check
      const adminCode = process.env.NEXT_PUBLIC_ADMIN_PASSWORD || '123456';
      if (code === adminCode) {
        localStorage.setItem('solidbit_admin_auth', code);
        router.push('/admin');
        return;
      }

      // If it starts with 'D' or is any other code, treat as driver
      if (code.length >= 4) {
        // Try calling the backend
        const res = await fetch('/api/driver/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ code })
        });
        
        if (res.ok) {
          const data = await res.json();
          localStorage.setItem('solidbit_driver_auth', data.driver.id);
          localStorage.setItem('solidbit_driver_name', data.driver.name);
          router.push('/dashboard');
          return;
        }
      }

      // Invalid logic
      setError(true);
      setLoading(false);

    } catch (err) {
      setError(true);
      setLoading(false);
    }
  };

  // Keyboard support
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key >= '0' && e.key <= '9') {
         handleKeyPress(e.key);
      } else if (e.key === 'Backspace') {
         handleDelete();
      } else if (e.key === 'Enter') {
         handleSubmit();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [code, loading, error]);

  return (
    <div className="min-h-screen bg-neutral-950 flex flex-col items-center justify-center p-4 font-mono overflow-hidden">
      
      {/* Dynamic Background */}
      <div className="absolute inset-0 z-0">
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] bg-indigo-500/10 blur-[100px] rounded-full mix-blend-screen" />
      </div>

      <div className="z-10 w-full max-w-sm">
        <div className="flex flex-col items-center mb-10">
          <div className="w-16 h-16 bg-gradient-to-br from-indigo-500 to-cyan-500 rounded-2xl flex items-center justify-center mb-6 shadow-2xl shadow-indigo-500/20 relative">
             <div className="absolute inset-0 bg-black/20 rounded-2xl" />
             <Navigation className="w-8 h-8 text-white relative z-10 -mr-1" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight text-white mb-1">SOLIDBIT</h1>
          <p className="text-neutral-500 text-xs tracking-widest uppercase">Motor Logístico</p>
        </div>

        <motion.div 
          animate={error ? { x: [-10, 10, -10, 10, 0] } : {}}
          transition={{ duration: 0.4 }}
          className="bg-neutral-900/80 backdrop-blur-xl border border-white/10 rounded-[2rem] p-8 shadow-2xl relative"
        >
          {/* Status Bar */}
          <div className="absolute top-6 right-6">
            <AnimatePresence>
               {error ? (
                  <motion.div initial={{opacity:0, scale: 0.5}} animate={{opacity:1, scale:1}} exit={{opacity:0}} className="w-3 h-3 rounded-full bg-red-500 shadow-[0_0_15px_rgba(239,68,68,0.6)]" />
               ) : loading ? (
                  <motion.div initial={{opacity:0}} animate={{opacity:1}} exit={{opacity:0}}>
                     <Loader2 className="w-4 h-4 text-indigo-400 animate-spin" />
                  </motion.div>
               ) : (
                  <div className="w-3 h-3 rounded-full bg-emerald-500 shadow-[0_0_15px_rgba(16,185,129,0.4)]" />
               )}
            </AnimatePresence>
          </div>

          <div className="flex justify-center items-center gap-1.5 mb-10 h-10 w-full max-w-[280px] mx-auto flex-wrap">
            {[...Array(12)].map((_, i) => (
              <div 
                key={i}
                className={`w-3 h-3 rounded-full transition-all duration-300 ${
                  i < code.length 
                    ? error ? 'bg-red-500 scale-110' : 'bg-indigo-400 shadow-[0_0_8px_rgba(129,140,248,0.8)] scale-110' 
                    : 'bg-white/10'
                }`}
              />
            ))}
          </div>

          <div className="grid grid-cols-3 gap-3">
            {[1, 2, 3, 4, 5, 6, 7, 8, 9].map((num) => (
              <button
                key={num}
                onClick={() => handleKeyPress(num.toString())}
                className="h-16 rounded-2xl bg-white/5 hover:bg-white/10 active:bg-white/20 border border-white/5 hover:border-white/10 text-2xl font-medium text-white transition-all focus:outline-none"
              >
                {num}
              </button>
            ))}
            <button
               onClick={handleDelete}
               className="h-16 rounded-2xl bg-transparent hover:bg-white/5 active:bg-white/10 text-neutral-400 hover:text-white transition-all flex items-center justify-center focus:outline-none"
            >
               DEL
            </button>
            <button
              onClick={() => handleKeyPress('0')}
              className="h-16 rounded-2xl bg-white/5 hover:bg-white/10 active:bg-white/20 border border-white/5 hover:border-white/10 text-2xl font-medium text-white transition-all focus:outline-none"
            >
              0
            </button>
            <button
               onClick={handleSubmit}
               disabled={code.length === 0}
               className={`h-16 rounded-2xl flex items-center justify-center transition-all focus:outline-none focus:ring-2 focus:ring-indigo-500 ${
                 code.length > 0 
                  ? 'bg-indigo-600 hover:bg-indigo-500 text-white shadow-lg shadow-indigo-600/30' 
                  : 'bg-white/5 text-neutral-600 cursor-not-allowed'
               }`}
            >
               <KeyRound className="w-6 h-6" />
            </button>
          </div>
        </motion.div>

        <p className="text-center mt-8 text-neutral-500 text-xs">Uso exclusivo para personal autorizado</p>
      </div>
    </div>
  );
}

