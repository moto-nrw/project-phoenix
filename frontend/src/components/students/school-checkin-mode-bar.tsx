"use client";

import { Check, ChevronRight, UserPlus } from "lucide-react";
import { GROUP_ROOM_SHADES, LOCATION_COLORS } from "~/lib/location-helper";

interface SchoolCheckinModeBarProps {
  /** Whether check-in/out mode is currently active. */
  readonly isActive: boolean;
  /** Toggle the mode on / off. */
  readonly onToggle: () => void;
  /** Successful per-session toggles — surfaces in the active banner copy. */
  readonly successCount: number;
  /** Open API calls in flight — surfaces as a "X laufen" hint. */
  readonly pendingCount: number;
}

/**
 * SchoolCheckinModeBar — full-width sticky banner that sits between the
 * filter row and the card grid. Same shape in both states so the visual
 * anchor never moves; only the content swaps.
 *
 * - **Mode OFF**: subtle solid brand-tinted banner, content centred, the
 *   whole banner is the trigger ("Schüler an- & abmelden" + arrow).
 * - **Mode ON**: stronger solid brand-tint with a status block centred and
 *   a "Fertig" button anchored on the right.
 *
 * Sticky `top-[73px]` keeps the banner pinned right below the global app
 * header (height 73px). Solid background (no alpha) prevents card content
 * from bleeding through while the user scrolls.
 *
 * Visibility gating (binary tenant only) is the caller's responsibility.
 */
export function SchoolCheckinModeBar({
  isActive,
  onToggle,
  successCount,
  pendingCount,
}: SchoolCheckinModeBarProps) {
  // Outer wrapper margin only — the page sticks the parent container
  // (filter row + banner) as one unit, so the banner itself doesn't need
  // its own sticky positioning anymore.
  const stickyShellClasses = "mb-2";

  if (!isActive) {
    // Mode OFF — full-width tappable banner. Content is centred so the
    // call-to-action reads as a single visual unit. Solid background
    // (GROUP_ROOM_SHADES.bgHover = #f0f9e4) so nothing bleeds through
    // during scroll.
    return (
      <div className={stickyShellClasses}>
        <button
          type="button"
          onClick={onToggle}
          className="group relative flex w-full items-center justify-center gap-3 rounded-2xl px-5 py-4 text-center transition-colors duration-150 hover:brightness-[0.98] active:scale-[0.998]"
          style={{
            backgroundColor: GROUP_ROOM_SHADES.bgHover, // solid #f0f9e4
            color: GROUP_ROOM_SHADES.text,
            boxShadow: `inset 0 0 0 1px ${LOCATION_COLORS.GROUP_ROOM}40`, // ~25% ring
          }}
          aria-label="An- und Abmelde-Modus aktivieren"
        >
          <span
            className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full"
            style={{ backgroundColor: GROUP_ROOM_SHADES.base }}
          >
            <UserPlus
              className="h-5 w-5 text-white"
              aria-hidden="true"
              strokeWidth={2.5}
            />
          </span>

          <span className="text-base font-bold sm:text-lg">
            Schüler an- &amp; abmelden
          </span>

          <ChevronRight
            className="h-5 w-5 flex-shrink-0 transition-transform duration-150 group-hover:translate-x-0.5"
            aria-hidden="true"
            strokeWidth={2.5}
          />
        </button>
      </div>
    );
  }

  // Mode ON — stronger solid tint (#e4f3d3 from GROUP_ROOM_SHADES.bgActive)
  // so the active state reads as "louder" than the inactive trigger.
  // Status block is centred; Fertig button is absolutely anchored on the
  // right so the centred content stays optically centred.
  const counterCopy =
    successCount === 0
      ? "Tippe auf einen Schüler"
      : successCount === 1
        ? "1 Schüler bearbeitet"
        : `${successCount} Schüler bearbeitet`;

  const pendingCopy =
    pendingCount > 0
      ? ` · ${pendingCount} ${pendingCount === 1 ? "läuft" : "laufen"}`
      : "";

  return (
    <div className={stickyShellClasses}>
      <div
        className="relative flex items-center justify-center gap-3 rounded-2xl px-5 py-4 pr-28 sm:pr-32"
        style={{
          backgroundColor: GROUP_ROOM_SHADES.bgActive, // solid #e4f3d3
          color: GROUP_ROOM_SHADES.text,
          boxShadow: `inset 0 0 0 1px ${LOCATION_COLORS.GROUP_ROOM}80`, // ~50% ring
        }}
        data-checkin-mode-bar="active"
      >
        <span
          className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full"
          style={{ backgroundColor: GROUP_ROOM_SHADES.base }}
        >
          <Check
            className="h-5 w-5 text-white"
            aria-hidden="true"
            strokeWidth={2.5}
          />
        </span>

        <span className="flex flex-col text-center">
          <span className="text-sm font-bold sm:text-base">
            An- &amp; Abmelde-Modus aktiv
          </span>
          <span className="text-xs opacity-80">
            {counterCopy}
            {pendingCopy}
          </span>
        </span>

        <button
          type="button"
          onClick={onToggle}
          className="absolute top-1/2 right-3 flex flex-shrink-0 -translate-y-1/2 items-center rounded-full bg-white px-4 py-1.5 text-sm font-semibold transition-colors duration-150 hover:bg-gray-50 active:bg-gray-100 sm:px-5 sm:py-2"
          style={{
            color: GROUP_ROOM_SHADES.text,
            boxShadow: `inset 0 0 0 1px ${LOCATION_COLORS.GROUP_ROOM}80`,
          }}
          aria-label="An- und Abmelde-Modus beenden"
        >
          Fertig
        </button>
      </div>
    </div>
  );
}
