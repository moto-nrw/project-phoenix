"use client";

import { useTenantRouter } from "~/lib/tenant-router";

interface MobileBackButtonProps {
  /** Destination URL when button is clicked */
  href?: string;
  /** Accessible label for the button */
  ariaLabel?: string;
}

/**
 * Back button that navigates to the parent page whenever the shell header is
 * hidden. It is visible below `lg`, matching the staff shell breakpoint.
 *
 * Extracted to eliminate code duplication across database pages.
 */
export function MobileBackButton({
  href = "/database",
  ariaLabel = "Zurück zur Datenverwaltung",
}: Readonly<MobileBackButtonProps>) {
  const router = useTenantRouter();

  return (
    <button
      type="button"
      onClick={() => router.push(href)}
      className="relative z-10 mb-3 flex items-center gap-2 text-gray-600 transition-colors duration-200 hover:text-gray-900 lg:hidden"
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
