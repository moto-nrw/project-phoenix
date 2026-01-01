"use client";

import { AnimatedBackground } from "./animated-background";

interface BackgroundWrapperProps {
  readonly children: React.ReactNode;
}

export function BackgroundWrapper({ children }: BackgroundWrapperProps) {
  return (
    <div className="min-h-screen">
      {/* Background elements (positioned at the back) */}
      <AnimatedBackground />
      <div
        className="fixed inset-0 bg-white/20 backdrop-blur-sm"
        style={{ zIndex: -5 }}
      />

      {/* Content (positioned on top) */}
      <div className="relative">{children}</div>
    </div>
  );
}
