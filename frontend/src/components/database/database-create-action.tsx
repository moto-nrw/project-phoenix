"use client";

import { ChevronDown, Plus } from "lucide-react";
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Button, buttonClassName } from "~/components/ui/button";
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

// Anlegen im Kopf: Kit `success` `md`; mobil der schwebende Kit-Knopf `fab`.
const DESKTOP_CLASS = buttonClassName({
  variant: "success",
  size: "md",
  className: "flex gap-2",
});
const FAB_CLASS = buttonClassName({
  variant: "fab",
  size: "icon",
  className: "flex",
});

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
            triggerClassName={DESKTOP_CLASS}
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
                triggerClassName={FAB_CLASS}
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
      <Button
        type="button"
        variant="success"
        size="md"
        onClick={onClick}
        disabled={disabled}
        className="hidden gap-2 md:flex"
        aria-label={ariaLabel}
        // Ziel der geführten Tour der ersten Schritte (#2832).
        data-setup-tour="create"
      >
        + {label}
      </Button>
      {showMobileFab &&
        mounted &&
        createPortal(
          <Button
            type="button"
            variant="fab"
            size="icon"
            onClick={onClick}
            disabled={disabled}
            className="fixed right-4 bottom-24 z-40 flex md:hidden"
            // Schwebender Symbolknopf: Das Portal löst ihn aus dem
            // Filter-Kontext der Kopfkarte, damit `fixed` den Viewport meint.
            data-icon-only=""
            data-setup-tour="create"
            aria-label={ariaLabel}
          >
            <Plus className="h-6 w-6" strokeWidth={2.5} aria-hidden />
          </Button>,
          document.body,
        )}
    </>
  );
}
