// components/ui/loading.tsx
// Skeleton Loader using shadcn/ui Skeleton component

"use client";

import { Skeleton } from "~/components/ui/skeleton";

interface LoadingProps {
  message?: string;
  fullPage?: boolean;
}

export function Loading({
  message = "Lädt...",
  fullPage = true,
}: LoadingProps) {
  const containerClasses = fullPage
    ? "fixed inset-0 flex items-center justify-center bg-white/80 backdrop-blur-sm z-50"
    : "flex items-center justify-start pt-24 pb-12"; // Changed justify-center to justify-start with top padding

  return (
    <div className={containerClasses} aria-label={message} role="status">
      <div className="mx-auto flex w-full max-w-xs flex-col items-center gap-2 px-4">
        {/* Text line skeleton */}
        <Skeleton className="h-4 w-full rounded-full" />

        {/* Circular skeleton */}
        <Skeleton className="h-10 w-10 rounded-full" />

        {/* Rectangular skeleton */}
        <Skeleton className="h-14 w-52 rounded-md" />

        {/* Rounded skeleton */}
        <Skeleton className="h-14 w-52 rounded-lg" />
        {/* SR-only message for assistive tech */}
        <span className="sr-only">{message}</span>
      </div>
    </div>
  );
}
