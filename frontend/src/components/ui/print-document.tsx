"use client";

import { useEffect, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Printer } from "lucide-react";

import { Button } from "~/components/ui/button";

/**
 * A page part that is printed on its own, without the portal around it
 * (sidebar, header, bottom navigation, scroll containers).
 *
 * The content renders twice: once in place for the screen, and once directly
 * under `<body>` for the printer. The print CSS in `globals.css`
 * (`.moto-print-document`) hides every other child of `<body>` while
 * printing, so no shell layout, fixed height or `backdrop-filter` of a card
 * can cut the printout to one page. On screen the second copy is never
 * shown, and it is hidden from assistive technology.
 *
 * Added to the kit for the Erklärung proofs (#3430); the browser's print
 * dialog is also how people save the page as PDF.
 */
export function PrintDocument({ children }: Readonly<{ children: ReactNode }>) {
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);

  return (
    <>
      <div className="print:hidden">{children}</div>
      {mounted &&
        createPortal(
          <div className="moto-print-document" aria-hidden="true">
            {children}
          </div>,
          document.body,
        )}
    </>
  );
}

/** Opens the browser's print dialog, where the page can also be saved as PDF. */
export function PrintButton({
  label,
  className = "",
  size = "md",
}: Readonly<{
  label: string;
  className?: string;
  size?: "md" | "touch";
}>) {
  return (
    <Button
      type="button"
      variant="primary"
      size={size}
      className={`gap-1.5 print:hidden ${className}`}
      onClick={() => globalThis.print()}
    >
      <Printer className="h-4 w-4" aria-hidden="true" />
      {label}
    </Button>
  );
}
