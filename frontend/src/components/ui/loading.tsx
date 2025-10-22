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
      <div className="flex flex-col items-center gap-2 w-full max-w-xs px-4 mx-auto">
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

// Alternative: Card-style loader (more shadcn-like)
export function LoadingCard({ message = "Lädt..." }: { message?: string }) {
  return (
    <div className="flex items-center justify-center p-8">
      <div className="relative overflow-hidden rounded-2xl border border-gray-200 bg-white p-8 shadow-sm">
        {/* Shimmer effect */}
        <div className="absolute inset-0 -translate-x-full animate-[shimmer_2s_infinite] bg-gradient-to-r from-transparent via-white/60 to-transparent" />

        <div className="relative flex flex-col items-center gap-4">
          {/* Spinner */}
          <div className="relative h-12 w-12">
            <div className="absolute inset-0 rounded-full border-4 border-gray-100" />
            <div className="absolute inset-0 animate-spin rounded-full border-4 border-transparent border-t-[#5080D8] border-r-[#83CD2D]" />
          </div>

          {/* Message */}
          <p className="text-sm font-medium text-gray-600">{message}</p>
        </div>
      </div>
    </div>
  );
}

// Skeleton loader for content
export function LoadingSkeleton() {
  return (
    <div className="space-y-4 p-4">
      <div className="h-8 w-3/4 animate-pulse rounded-lg bg-gray-200" />
      <div className="h-4 w-full animate-pulse rounded-lg bg-gray-100" />
      <div className="h-4 w-5/6 animate-pulse rounded-lg bg-gray-100" />
      <div className="mt-6 space-y-3">
        <div className="h-20 w-full animate-pulse rounded-xl bg-gray-200" />
        <div className="h-20 w-full animate-pulse rounded-xl bg-gray-200" />
      </div>
    </div>
  );
}
