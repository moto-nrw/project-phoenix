"use client";

import { UserCheck, UserX } from "lucide-react";
import { LOCATION_COLORS } from "~/lib/location-helper";

interface SchoolCheckinToggleProps {
  /** Whether the page is currently in check-in/out mode. */
  readonly isActive: boolean;
  /** Fires when the user taps the button to flip the mode. */
  readonly onToggle: () => void;
  /** Optional count of students whose toggle is still in flight. */
  readonly pendingCount?: number;
}

/**
 * Desktop pill button for toggling school check-in/out mode. Mirrors the
 * "Gruppe übergeben" button styling so the two actions read as peers in the
 * page header. Colour flips to brand green when active so the user has an
 * unambiguous signal that the whole page is now in mutation mode.
 *
 * Visibility is not gated client-side — the backend enforces the
 * users:checkin permission and the attendance.web_checkin_access setting;
 * a toast surfaces the 403 if a user without rights reaches this button.
 */
export function SchoolCheckinToggle({
  isActive,
  onToggle,
  pendingCount,
}: SchoolCheckinToggleProps) {
  const label = isActive ? "An-/Abmelden beenden" : "Schüler An- & Abmelden";
  const Icon = isActive ? UserX : UserCheck;

  return (
    <button
      type="button"
      onClick={onToggle}
      aria-pressed={isActive}
      aria-label={label}
      data-active={isActive || undefined}
      className={
        isActive
          ? "flex h-10 items-center gap-2 rounded-full border border-[#83CD2D] bg-[#83CD2D] px-4 text-white shadow-sm transition-colors duration-150 hover:bg-[#74b827] active:bg-[#669f21]"
          : "flex h-10 items-center gap-2 rounded-full border border-[#83CD2D] bg-white px-4 text-[#4a7a15] transition-colors duration-150 hover:bg-[#f0f9e4] active:bg-[#e4f3d3]"
      }
      style={isActive ? { borderColor: LOCATION_COLORS.GROUP_ROOM } : undefined}
    >
      <Icon className="h-4 w-4" aria-hidden="true" />
      <span className="text-sm font-medium">
        {label}
        {pendingCount && pendingCount > 0 ? ` (${pendingCount})` : ""}
      </span>
    </button>
  );
}

/**
 * Compact icon-only variant used in the mobile header where the full pill
 * doesn't fit. Behaviour matches the desktop toggle; only styling differs.
 */
export function SchoolCheckinToggleMobile({
  isActive,
  onToggle,
  pendingCount,
}: SchoolCheckinToggleProps) {
  const label = isActive ? "An-/Abmelden beenden" : "Schüler An- & Abmelden";
  const Icon = isActive ? UserX : UserCheck;

  return (
    <button
      type="button"
      onClick={onToggle}
      aria-pressed={isActive}
      aria-label={label}
      data-active={isActive || undefined}
      className={
        isActive
          ? "relative flex h-8 w-8 items-center justify-center rounded-full border border-[#83CD2D] bg-[#83CD2D] text-white shadow-sm active:bg-[#669f21]"
          : "relative flex h-8 w-8 items-center justify-center rounded-full border border-[#83CD2D] bg-white text-[#4a7a15] transition-colors duration-150 active:bg-[#e4f3d3]"
      }
    >
      <Icon className="h-4 w-4" aria-hidden="true" />
      {pendingCount && pendingCount > 0 ? (
        <span className="absolute -top-1 -right-1 flex h-4 w-4 items-center justify-center rounded-full border border-[#83CD2D] bg-white text-[10px] font-bold text-[#4a7a15]">
          {pendingCount}
        </span>
      ) : null}
    </button>
  );
}
