"use client";

import { useEffect } from "react";
import { CheckSquare, Loader2 } from "lucide-react";
import { GROUP_ROOM_SHADES } from "~/lib/location-helper";
import { useAttendanceWebEnabled } from "~/lib/tenant-context";

const CHECKIN_BAR_OFFSET_VAR = "--moto-checkin-bar-offset";
const CHECKIN_BAR_OFFSET = "4.5rem";

interface SchoolCheckinModeMobileProps {
  /** Whether the page is currently in check-in/out mode. */
  readonly isActive: boolean;
  /** Toggle the mode on/off. */
  readonly onToggle: () => void;
  /** Successful per-session toggles — surfaced on the active sticky bar. */
  readonly successCount: number;
  /** Open API calls in flight — surfaces as a "X laufen" hint. */
  readonly pendingCount: number;
  /** Selection sub-mode on (#2359) — the sticky bar then counts marks, not writes. */
  readonly selectionActive?: boolean;
  /** Currently marked students while the selection sub-mode is on. */
  readonly selectedCount?: number;
  /**
   * Locks the trigger, e.g. while a bulk request is in flight (#2359): the
   * hook ignores mode exits during a run, so "Fertig" mirrors that as a
   * visible disabled state instead of a dead tap.
   */
  readonly disabled?: boolean;
}

/**
 * Mobile-only (`<md` / <768px) check-in mode trigger.
 *
 * - **OFF**: full-width inline pill rendered in the page flow at the top
 *   of the card list. Scrolls away with the rest of the (non-sticky)
 *   header — re-accessible via scroll-up. Mode activation is a once-per-
 *   shift action so this is fine.
 * - **ON**: fixed sticky bar above the mobile bottom nav. Counter +
 *   "Fertig" button stay in the thumb zone for the duration of the
 *   multi-tap session (iOS Photos / Apple Mail multi-select pattern).
 *
 * Tablet (md..lg) uses the floating FAB; desktop (lg+) uses the inline
 * pill inside the page header's primaryAction slot.
 */
export function SchoolCheckinModeMobile({
  isActive,
  onToggle,
  successCount,
  pendingCount,
  selectionActive = false,
  selectedCount = 0,
  disabled = false,
}: SchoolCheckinModeMobileProps) {
  const attendanceWebEnabled = useAttendanceWebEnabled();

  useEffect(() => {
    if (!attendanceWebEnabled || !isActive) return;
    const root = document.documentElement;
    const mediaQuery = window.matchMedia("(max-width: 767px)");
    const publishOffset = () => {
      if (mediaQuery.matches) {
        root.style.setProperty(CHECKIN_BAR_OFFSET_VAR, CHECKIN_BAR_OFFSET);
      } else {
        root.style.removeProperty(CHECKIN_BAR_OFFSET_VAR);
      }
    };
    publishOffset();
    mediaQuery.addEventListener("change", publishOffset);
    return () => {
      mediaQuery.removeEventListener("change", publishOffset);
      root.style.removeProperty(CHECKIN_BAR_OFFSET_VAR);
    };
  }, [attendanceWebEnabled, isActive]);

  if (!attendanceWebEnabled) return null;
  if (!isActive) {
    return (
      <button
        type="button"
        onClick={onToggle}
        aria-pressed={false}
        aria-label="Kinder an- und abmelden"
        className="flex w-full items-center justify-center gap-2.5 rounded-full px-5 py-3 text-base font-semibold transition-colors duration-150 outline-none focus-visible:ring-2 focus-visible:ring-offset-2 active:scale-[0.99]"
        style={{
          backgroundColor: "var(--color-white)",
          color: GROUP_ROOM_SHADES.text,
          boxShadow: `inset 0 0 0 1.5px ${GROUP_ROOM_SHADES.base}, 0 1px 2px rgb(0 0 0 / 0.04)`,
          // @ts-expect-error CSS custom property for focus ring colour
          "--tw-ring-color": `${GROUP_ROOM_SHADES.base}80`,
        }}
      >
        <span
          className="flex size-5 flex-shrink-0 items-center justify-center rounded-full"
          style={{ backgroundColor: `${GROUP_ROOM_SHADES.base}1a` }}
        >
          <CheckSquare className="size-3.5" strokeWidth={2.5} aria-hidden />
        </span>
        <span>Kinder an- und abmelden</span>
      </button>
    );
  }

  // Sticky bar sits above the mobile bottom nav. The nav's actual visible
  // height is ~4.75rem (pill min-h-44px + py-2 + outer pb-4) plus its own
  // safe-area-inset. Offset = 5.5rem (nav + breathing gap) + safe-area so
  // the bar visually floats above the nav with a clear separator.
  return (
    <div
      className="fixed right-0 left-0 z-40 px-3"
      style={{ bottom: "calc(5.5rem + env(safe-area-inset-bottom))" }}
      data-checkin-mode-mobile="active"
    >
      <div
        className="flex items-center gap-3 rounded-2xl px-3 py-2.5"
        style={{
          backgroundColor: GROUP_ROOM_SHADES.base,
          color: "var(--color-white)",
          boxShadow:
            "0 -4px 20px rgb(0 0 0 / 0.10), 0 8px 24px rgb(0 0 0 / 0.08)",
        }}
      >
        <span
          className="flex size-9 flex-shrink-0 items-center justify-center rounded-full"
          style={{ backgroundColor: "rgb(255 255 255 / 0.20)" }}
        >
          <CheckSquare className="size-4" strokeWidth={2.5} aria-hidden />
        </span>

        <div className="flex min-w-0 flex-1 flex-col">
          <span className="text-sm leading-tight font-bold">
            An- &amp; Abmelde-Modus aktiv
          </span>
          <span className="text-xs leading-tight opacity-90">
            <span className="tabular-nums">
              {selectionActive
                ? `${selectedCount} ausgewählt`
                : successCount === 0
                  ? "Tippe auf ein Kind"
                  : `${successCount} bearbeitet`}
            </span>
            {pendingCount > 0 ? (
              <span className="tabular-nums"> · {pendingCount} laufen</span>
            ) : null}
          </span>
        </div>

        {pendingCount > 0 ? (
          <Loader2
            className="size-4 flex-shrink-0 animate-spin opacity-90"
            aria-hidden
          />
        ) : null}

        <button
          type="button"
          onClick={onToggle}
          disabled={disabled}
          aria-label="An- und Abmelde-Modus beenden"
          className="flex h-9 flex-shrink-0 items-center rounded-full bg-white px-4 text-sm font-semibold transition-colors duration-150 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-60 disabled:active:scale-100"
          style={{ color: GROUP_ROOM_SHADES.text }}
        >
          Fertig
        </button>
      </div>
    </div>
  );
}
