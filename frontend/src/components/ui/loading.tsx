// components/ui/loading.tsx
// Neutral loading state for guards and token flows where the future layout is
// unknown. Pages with a known shape use colocated skeletons instead.

"use client";

import { cn } from "~/lib/utils";
import { Z_LAYER } from "./overlay-styles";
import { SpinnerIcon } from "~/components/ui/icons";

interface LoadingProps {
  readonly message?: string;
  readonly fullPage?: boolean;
  /** Kompakte Zeile für einen begrenzten Bereich wie einen Picker. */
  readonly compact?: boolean;
  readonly className?: string;
}

export function Loading({
  message = "Lädt...",
  fullPage = true,
  compact = false,
  className,
}: LoadingProps) {
  const containerClasses =
    fullPage && !compact
      ? `fixed inset-0 flex items-center justify-center bg-white/80 backdrop-blur-sm ${Z_LAYER.sheet}`
      : compact
        ? "flex min-h-0 items-center justify-center py-3"
        : "flex min-h-40 items-center justify-center py-8";

  return (
    <output
      className={cn(containerClasses, className)}
      aria-label={message}
      aria-live="polite"
      aria-busy="true"
    >
      <div className="flex items-center gap-3 text-gray-600">
        <SpinnerIcon className="h-6 w-6" />
        <span className="text-sm font-medium">{message}</span>
      </div>
    </output>
  );
}
