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
  const [hasShownExpiryMessage, setHasShownExpiryMessage] = useState(false);

  // Check for session expiry and show notification
  useEffect(() => {
    if (status === 'loading') return;
    
    // Check if session has refresh errors
    if (session?.error === 'RefreshTokenError' && !hasShownExpiryMessage) {
      setHasShownExpiryMessage(true);
      
      // Create and show notification
      const notification = document.createElement('div');
      notification.className = 'fixed top-4 right-4 z-50 bg-red-600 text-white px-6 py-3 rounded-lg shadow-lg flex items-center gap-3 animate-slide-in-right';
      notification.innerHTML = `
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
        <div>
          <p class="font-medium">Sitzung abgelaufen</p>
          <p class="text-sm opacity-90">Bitte melden Sie sich erneut an</p>
        </div>
      `;
      document.body.appendChild(notification);
      
      // Add CSS animation
      const style = document.createElement('style');
      style.textContent = `
        @keyframes slide-in-right {
          from {
            transform: translateX(100%);
            opacity: 0;
          }
          to {
            transform: translateX(0);
            opacity: 1;
          }
        }
        .animate-slide-in-right {
          animation: slide-in-right 0.3s ease-out;
        }
      `;
      document.head.appendChild(style);
      
      // Redirect after 3 seconds
      setTimeout(() => {
        router.push('/');
      }, 3000);
      
      // Remove notification after 2.5 seconds (before redirect)
      setTimeout(() => {
        notification.remove();
        style.remove();
      }, 2500);
    }
  }, [session?.error, status, hasShownExpiryMessage, router]);

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
    <div className="min-h-screen">
      {/* Header with conditional blur */}
      <div className={`transition-all duration-300 ${isMobileModalOpen ? 'blur-md lg:blur-none' : ''}`}>
        <Header userName={userName} />
      </div>
      
      {/* Main content with conditional blur */}
      <div className={`flex transition-all duration-300 ${isMobileModalOpen ? 'blur-md lg:blur-none' : ''}`}>
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