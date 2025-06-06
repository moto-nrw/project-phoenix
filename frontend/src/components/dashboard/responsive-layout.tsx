'use client';

import { useState, useEffect } from 'react';
import { useSession } from 'next-auth/react';
import { useRouter } from 'next/navigation';
import { Header } from './header';
import { Sidebar } from './sidebar';
import { MobileBottomNav } from './mobile-bottom-nav';

interface ResponsiveLayoutProps {
  children: React.ReactNode;
}

export default function ResponsiveLayout({ children }: ResponsiveLayoutProps) {
  const { data: session, status } = useSession();
  const router = useRouter();
  const userName = session?.user?.name ?? 'Root';
  const [isMobileModalOpen, setIsMobileModalOpen] = useState(false);

  // Check for invalid session and redirect
  useEffect(() => {
    if (status === 'loading') return;
    
    // If session exists but token is empty, redirect to login
    if (session && !session.user?.token) {
      router.push('/');
    }
  }, [session, status, router]);

  // Listen for modal state changes via custom events
  useEffect(() => {
    const handleModalOpen = () => setIsMobileModalOpen(true);
    const handleModalClose = () => setIsMobileModalOpen(false);

    window.addEventListener('mobile-modal-open', handleModalOpen);
    window.addEventListener('mobile-modal-close', handleModalClose);

    return () => {
      window.removeEventListener('mobile-modal-open', handleModalOpen);
      window.removeEventListener('mobile-modal-close', handleModalClose);
    };
  }, []);

  return (
    <div className="min-h-screen flex flex-col">
      {/* Header with conditional blur - sticky positioning */}
      <div className={`sticky top-0 z-40 transition-all duration-300 ${isMobileModalOpen ? 'blur-md lg:blur-none' : ''}`}>
        <Header userName={userName} />
      </div>
      
      {/* Main content with conditional blur */}
      <div className={`flex flex-1 transition-all duration-300 ${isMobileModalOpen ? 'blur-md lg:blur-none' : ''}`}>
        {/* Desktop sidebar - only visible on md+ screens */}
        <Sidebar className="hidden lg:block" />
        
        {/* Main content with bottom padding on mobile for bottom navigation */}
        <main className="flex-1 p-4 md:p-8 pb-24 lg:pb-8">
          {children}
        </main>
      </div>
      
      {/* Mobile bottom navigation with conditional blur */}
      <MobileBottomNav className={`transition-all duration-300 ${isMobileModalOpen ? 'blur-md lg:blur-none' : ''}`} />
    </div>
  );
}