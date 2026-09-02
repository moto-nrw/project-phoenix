"use client";

import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { useTenantRouter } from "~/lib/tenant-router";

interface MobileBackButtonProps {
  /** Destination URL when button is clicked */
  href?: string;
  /** Accessible label for the button */
  ariaLabel?: string;
}

/**
 * Mobile-only back button that navigates to parent page.
 * Only renders on mobile viewports (< 768px).
 *
 * Extracted to eliminate code duplication across database pages.
 */
export function MobileBackButton({
  href = "/database",
  ariaLabel = "Zurück zur Datenverwaltung",
}: Readonly<MobileBackButtonProps>) {
  const isMobile = useIsMobile();
  const router = useTenantRouter();

  if (!isMobile) return null;

  return (
    <button
      type="button"
      onClick={() => router.push(href)}
      className="relative z-10 mb-3 flex items-center gap-2 text-gray-600 transition-colors duration-200 hover:text-gray-900"
      aria-label={ariaLabel}
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
