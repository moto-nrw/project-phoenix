"use client";

import type { ReactNode } from "react";
import { cn } from "~/lib/utils";

/**
 * Ein an- und abschaltbares Merkmal in einer Reihe: Wochentage, Klassenstufen,
 * Monate, Gruppen.
 *
 * Vorher hatte fast jede Stelle ihre eigene Chip-Optik — mal dunkle Füllung,
 * mal grüne, mal ein anderer Radius und eine andere Höhe. Der aktive Zustand
 * ist jetzt überall derselbe wie beim `SegmentedControl`: dunkel gefüllt, weil
 * er eine Auswahl markiert und keine Marke.
 *
 * Unterschied zum `SegmentedControl`: dort wird GENAU EINE Option gewählt,
 * hier ist jede Kachel für sich an oder aus.
 */
export function ToggleChip({
  pressed,
  onPressedChange,
  disabled = false,
  shape = "rounded",
  ariaLabel,
  className,
  children,
}: Readonly<{
  pressed: boolean;
  onPressedChange: (next: boolean) => void;
  disabled?: boolean;
  /** `pill` für einzelne Zeichen (Klassenstufen), sonst `rounded`. */
  shape?: "rounded" | "pill";
  /** Wenn die Beschriftung gekürzt ist ("Sep" für September). */
  ariaLabel?: string;
  className?: string;
  children: ReactNode;
}>) {
  return (
    <button
      type="button"
      aria-pressed={pressed}
      aria-label={ariaLabel}
      disabled={disabled}
      onClick={() => onPressedChange(!pressed)}
      className={cn(
        // 40 px wie jede Bedienhöhe im Portal (CONTROL_HEIGHT in
        // tenant-page.tsx) — ein Merkmal-Chip ist ein Bedienelement.
        "inline-flex h-10 items-center justify-center border px-3 text-sm font-medium transition-colors",
        "focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none",
        "disabled:cursor-not-allowed disabled:opacity-50",
        shape === "pill" ? "min-w-11 rounded-full" : "rounded-lg",
        pressed
          ? "border-gray-900 bg-gray-900 text-white"
          : "border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50",
        className,
      )}
    >
      {children}
    </button>
  );
}
