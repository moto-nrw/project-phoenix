'use client';

import { Header } from './header';
import { Sidebar } from './sidebar';
import { BottomNavigation } from './bottom-navigation';

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
        <Sidebar className="hidden md:block" />
        
        {/* Main content with bottom padding on mobile for bottom navigation */}
        <main className="flex-1 p-4 md:p-8 pb-20 md:pb-8">
          {children}
        </main>
      </div>
      
      {/* Mobile bottom navigation - only visible on mobile */}
      <BottomNavigation />
    </div>
  );
}