'use client';

import { AnimatedBackground } from './animated-background';

interface BackgroundWrapperProps {
  children: React.ReactNode;
}

export function BackgroundWrapper({ children }: BackgroundWrapperProps) {
  return (
    <div className="min-h-screen">
      {/* Background elements (positioned at the back) */}
      <AnimatedBackground />
      <div 
        className="fixed inset-0 backdrop-blur-lg bg-white/12"
        style={{ zIndex: -5 }}
      />
      
      {/* Content (positioned on top) */}
      <div className="relative">{children}</div>
    </div>
  );
}