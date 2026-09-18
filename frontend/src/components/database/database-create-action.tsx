"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import {
  MOBILE_CREATE_FAB_MEDIA_QUERY,
  useFloatingFabOffset,
} from "~/lib/hooks/use-floating-fab-offset";

interface DatabaseCreateActionProps {
  label: string;
  ariaLabel: string;
  disabled?: boolean;
  showMobileFab?: boolean;
  onClick: () => void;
}

/**
 * Renders both the desktop "+ Label" button and the mobile FAB. Visibility
 * is controlled by Tailwind responsive utilities (no JS-side viewport check).
 */
export function DatabaseCreateAction({
  label,
  ariaLabel,
  disabled = false,
  showMobileFab = true,
  onClick,
}: DatabaseCreateActionProps) {
  // `TenantPage` renders actions inside a blurred content surface. A backdrop
  // filter establishes a containing block for fixed descendants, so the mobile
  // button must live outside that surface to remain viewport-fixed.
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  useFloatingFabOffset({
    active: showMobileFab,
    mediaQuery: MOBILE_CREATE_FAB_MEDIA_QUERY,
  });

  return (
    <>
      <button
        type="button"
        onClick={onClick}
        disabled={disabled}
        className="bg-moto-green hover:bg-moto-green-hover hidden h-10 items-center gap-2 rounded-lg px-4 text-sm font-semibold text-gray-950 disabled:cursor-not-allowed disabled:opacity-50 md:flex"
        aria-label={ariaLabel}
      >
        + {label}
      </button>
      {showMobileFab &&
        mounted &&
        createPortal(
          <button
            type="button"
            onClick={onClick}
            disabled={disabled}
            className="bg-moto-green hover:bg-moto-green-hover fixed right-4 bottom-24 z-40 flex h-14 w-14 items-center justify-center rounded-full text-gray-950 shadow-lg disabled:cursor-not-allowed disabled:opacity-50 md:hidden"
            // Schwebender Symbolknopf: Das Portal löst ihn aus dem
            // Filter-Kontext der Kopfkarte, damit `fixed` den Viewport meint.
            data-icon-only=""
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
          </button>,
          document.body,
        )}
    </>
  );
}
