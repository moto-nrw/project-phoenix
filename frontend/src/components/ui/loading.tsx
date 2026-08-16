// components/ui/loading.tsx
// Neutral loading state for guards and token flows where the future layout is
// unknown. Pages with a known shape use colocated skeletons instead.

"use client";

import { SpinnerIcon } from "~/components/ui/icons";

interface LoadingProps {
  readonly message?: string;
  readonly fullPage?: boolean;
}

export function Loading({
  message = "Lädt...",
  fullPage = true,
}: LoadingProps) {
  const containerClasses = fullPage
    ? "fixed inset-0 flex items-center justify-center bg-white/80 backdrop-blur-sm z-50"
    : "flex min-h-40 items-center justify-center py-8";

  return (
    <output
      className={containerClasses}
      aria-label={message}
      aria-live="polite"
      aria-busy="true"
    >
      <div className="flex items-center gap-3 text-gray-600">
        <SpinnerIcon className="h-6 w-6 motion-reduce:animate-none" />
        <span className="text-sm font-medium">{message}</span>
      </div>
    </output>
  );
}
