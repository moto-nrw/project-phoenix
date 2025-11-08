'use client';

import { useState, useEffect } from 'react';
import { useSession } from 'next-auth/react';
import { useRouter } from 'next/navigation';
import { Header } from './header';
import { Sidebar } from './sidebar';
import { MobileBottomNav } from './mobile-bottom-nav';

interface ResponsiveLayoutProps {
  children: React.ReactNode;
  pageTitle?: string;
  studentName?: string; // For student detail page breadcrumbs
  roomName?: string; // For room detail page breadcrumbs
  activityName?: string; // For activity detail page breadcrumbs
  referrerPage?: string; // Where the user came from (for contextual breadcrumbs)
}

export default function ResponsiveLayout({ children, pageTitle, studentName, roomName, activityName, referrerPage }: ResponsiveLayoutProps) {
  const { data: session, status } = useSession();
  const router = useRouter();
  // eslint-disable-next-line @typescript-eslint/prefer-nullish-coalescing -- intentionally using || to treat empty strings as falsy
  const userName = session?.user?.name?.trim() || undefined;
  const userEmail = session?.user?.email ?? '';
  const userRoles = session?.user?.roles ?? [];
  const userRole = userRoles.includes('admin') ? 'Admin' : userRoles.length > 0 ? 'Betreuer' : 'Betreuer';
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
    <div className="min-h-screen">
      {/* Header with conditional blur - sticky positioning */}
      <div className={`sticky top-0 z-40 transition-all duration-300 ${isMobileModalOpen ? 'blur-md lg:blur-none' : ''}`}>
        <Header userName={userName} userEmail={userEmail} userRole={userRole} customPageTitle={pageTitle} studentName={studentName} roomName={roomName} activityName={activityName} referrerPage={referrerPage} />
      </div>

      {/* Main content with conditional blur */}
      <div className={`flex transition-all duration-300 ${isMobileModalOpen ? 'blur-md lg:blur-none' : ''}`}>
        {/* Desktop sidebar - only visible on md+ screens */}
        <Sidebar className="hidden lg:block" />

        {/* Main content with bottom padding on mobile for bottom navigation */}
        <main className="flex-1 p-2 md:p-8 pb-24 lg:pb-8">
          {children}
        </main>
      </div>

      {/* Mobile bottom navigation with conditional blur */}
      <MobileBottomNav className={`transition-all duration-300 ${isMobileModalOpen ? 'blur-md lg:blur-none' : ''}`} />
    </div>
  );
}
