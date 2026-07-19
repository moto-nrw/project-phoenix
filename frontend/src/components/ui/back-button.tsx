"use client";

import { useTenantRouter } from "~/lib/tenant-router";

/**
 * Mobile-optimized back button component
 * Only visible on mobile (breadcrumb handles desktop navigation)
 */
export function BackButton({ referrer }: Readonly<{ referrer: string }>) {
  const router = useTenantRouter();

  return (
    <button
      type="button"
      onClick={() => router.push(referrer)}
      className="mb-4 -ml-1 flex items-center gap-2 py-2 pl-1 text-gray-600 transition-colors hover:text-gray-900 md:hidden"
    >
      <svg
        className="h-5 w-5"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M15 19l-7-7 7-7"
        />
      </svg>
      <span className="text-sm font-medium">Zurück</span>
    </button>
  );
}
