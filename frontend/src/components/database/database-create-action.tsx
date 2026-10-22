"use client";

import { ChevronDown, Plus } from "lucide-react";
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import {
  OverflowMenu,
  type OverflowMenuItem,
} from "~/components/ui/page-header/OverflowMenu";
import {
  MOBILE_CREATE_FAB_MEDIA_QUERY,
  useFloatingFabOffset,
} from "~/lib/hooks/use-floating-fab-offset";

interface DatabaseCreateActionProps {
  label: string;
  ariaLabel: string;
  disabled?: boolean;
  showMobileFab?: boolean;
  onClick?: () => void;
  /**
   * Several ways to create (#4573: Personal einladen, Externe Person anlegen).
   * Button and mobile FAB then open a menu with these entries instead of
   * calling `onClick`.
   */
  menuItems?: readonly OverflowMenuItem[];
}

const DESKTOP_CLASS =
  "bg-moto-green hover:bg-moto-green-hover h-10 items-center gap-2 rounded-lg px-4 text-sm font-semibold text-gray-950 disabled:cursor-not-allowed disabled:opacity-50";
const FAB_CLASS =
  "bg-moto-green hover:bg-moto-green-hover h-14 w-14 items-center justify-center rounded-full text-gray-950 shadow-lg disabled:cursor-not-allowed disabled:opacity-50";

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
  menuItems,
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

  if (menuItems && menuItems.length > 0) {
    return (
      <>
        {/* Ziel der geführten Tour der ersten Schritte (#2832). */}
        <span className="hidden md:flex" data-setup-tour="create">
          <OverflowMenu
            ariaLabel={ariaLabel}
            items={menuItems}
            triggerClassName={`${DESKTOP_CLASS} flex`}
            triggerContent={
              <>
                + {label}
                <ChevronDown className="size-4" aria-hidden />
              </>
            }
          />
        </span>
        {showMobileFab &&
          mounted &&
          createPortal(
            <span
              className="fixed right-4 bottom-24 z-40 md:hidden"
              data-setup-tour="create"
            >
              <OverflowMenu
                ariaLabel={ariaLabel}
                items={menuItems}
                triggerClassName={`${FAB_CLASS} flex`}
                triggerContent={
                  <Plus className="h-6 w-6" strokeWidth={2.5} aria-hidden />
                }
              />
            </span>,
            document.body,
          )}
      </>
    );
  }

  return (
    <>
      <button
        type="button"
        onClick={onClick}
        disabled={disabled}
        className={`${DESKTOP_CLASS} hidden md:flex`}
        aria-label={ariaLabel}
        // Ziel der geführten Tour der ersten Schritte (#2832).
        data-setup-tour="create"
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
            className={`${FAB_CLASS} fixed right-4 bottom-24 z-40 flex md:hidden`}
            // Schwebender Symbolknopf: Das Portal löst ihn aus dem
            // Filter-Kontext der Kopfkarte, damit `fixed` den Viewport meint.
            data-icon-only=""
            data-setup-tour="create"
            aria-label={ariaLabel}
          >
            <Plus className="h-6 w-6" strokeWidth={2.5} aria-hidden />
          </button>,
          document.body,
        )}
    </>
  );
}
