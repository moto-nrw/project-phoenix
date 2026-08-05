"use client";

import {
  MOBILE_CREATE_FAB_MEDIA_QUERY,
  useFloatingFabOffset,
} from "~/lib/hooks/use-floating-fab-offset";

interface DatabaseCreateActionProps {
  label: string;
  ariaLabel: string;
  onClick: () => void;
}

/**
 * Renders both the desktop "+ Label" button and the mobile FAB. Visibility
 * is controlled by Tailwind responsive utilities (no JS-side viewport check).
 */
export function DatabaseCreateAction({
  label,
  ariaLabel,
  onClick,
}: DatabaseCreateActionProps) {
  useFloatingFabOffset({
    active: true,
    mediaQuery: MOBILE_CREATE_FAB_MEDIA_QUERY,
  });

  return (
    <>
      <button
        type="button"
        onClick={onClick}
        className="bg-moto-green hover:bg-moto-green-hover hidden h-10 items-center gap-2 rounded-lg px-4 text-sm font-semibold text-gray-950 md:flex"
        aria-label={ariaLabel}
      >
        + {label}
      </button>
      <button
        type="button"
        onClick={onClick}
        className="bg-moto-green hover:bg-moto-green-hover fixed right-4 bottom-24 z-40 flex h-14 w-14 items-center justify-center rounded-full text-gray-950 shadow-lg md:hidden"
        aria-label={ariaLabel}
      >
        <svg
          className="h-6 w-6"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2.5}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M12 4.5v15m7.5-7.5h-15"
          />
        </svg>
      </button>
    </>
  );
}
