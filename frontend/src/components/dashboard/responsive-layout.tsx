'use client';

import { Header } from './header';
import { Sidebar } from './sidebar';
import { MobileBottomNav } from './mobile-bottom-nav';

interface ResponsiveLayoutProps {
  children: React.ReactNode;
  userName: string;
}

export default function ResponsiveLayout({ children, userName }: ResponsiveLayoutProps) {
  return (
    <div className="min-h-screen">
      <Header userName={userName} />
      
      <div className="flex">
        {/* Desktop sidebar - only visible on md+ screens */}
        <Sidebar className="hidden lg:block" />
        
        {/* Main content with bottom padding on mobile for bottom navigation */}
        <main className="flex-1 p-4 md:p-8">
          {children}
        </main>
      </div>
      
      {/* Mobile bottom navigation - only visible on mobile */}
      <MobileBottomNav />
    </div>
  );
}