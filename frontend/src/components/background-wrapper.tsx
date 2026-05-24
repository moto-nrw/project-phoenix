"use client";

import { useModal } from "./dashboard/modal-context";

interface BackgroundWrapperProps {
  readonly children: React.ReactNode;
}

export function BackgroundWrapper({ children }: BackgroundWrapperProps) {
  const { isModalOpen } = useModal();

  return (
    <div className="min-h-screen bg-white">
      <div className="relative">{children}</div>

      <div
        className={`pointer-events-none fixed inset-0 z-50 backdrop-blur-sm transition-opacity duration-200 ${
          isModalOpen ? "bg-black/5 opacity-100" : "opacity-0"
        }`}
        aria-hidden="true"
      />
    </div>
  );
}
