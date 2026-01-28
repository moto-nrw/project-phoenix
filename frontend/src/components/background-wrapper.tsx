"use client";

import { AnimatedBackground } from "./animated-background";
import { useModal } from "./dashboard/modal-context";

interface BackgroundWrapperProps {
  readonly children: React.ReactNode;
}

export function BackgroundWrapper({ children }: BackgroundWrapperProps) {
  const { isModalOpen } = useModal();

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

      {/* Global modal blur overlay - z-50 above header (z-40), below modal (z-[9999]) */}
      <div
        className={`pointer-events-none fixed inset-0 z-50 transition-all duration-300 ${
          isModalOpen
            ? "bg-black/5 backdrop-blur-sm"
            : "bg-transparent backdrop-blur-none"
        }`}
        aria-hidden="true"
      />
    </div>
  );
}
