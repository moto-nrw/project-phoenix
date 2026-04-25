"use client";

import { useState, type CSSProperties } from "react";
import { UserCheck, UserX } from "lucide-react";
import { GROUP_ROOM_SHADES } from "~/lib/location-helper";

interface SchoolCheckinToggleProps {
  /** Whether the page is currently in check-in/out mode. */
  readonly isActive: boolean;
  /** Fires when the user taps the button to flip the mode. */
  readonly onToggle: () => void;
  /** Optional count of students whose toggle is still in flight. */
  readonly pendingCount?: number;
}

// Tailwind purges arbitrary values it can't see at build time, so dynamic
// state-dependent shades have to come in via inline style instead of
// `bg-[#hex]` strings. We still keep the static layout in className.
function activeStyle(hover: boolean, pressed: boolean): CSSProperties {
  // Base = brand green; hover/active flip to the darker shades from the
  // GROUP_ROOM_SHADES family so the button reads as the same primary action
  // throughout the interaction.
  const background = pressed
    ? GROUP_ROOM_SHADES.active
    : hover
      ? GROUP_ROOM_SHADES.hover
      : GROUP_ROOM_SHADES.base;
  return {
    borderColor: GROUP_ROOM_SHADES.base,
    backgroundColor: background,
    color: "#fff",
  };
}

function ghostStyle(hover: boolean, pressed: boolean): CSSProperties {
  // "Ghost" variant = white background with brand-green border and text.
  // Hover tint stays in the green family so the relationship to the active
  // pill is obvious.
  const background = pressed
    ? GROUP_ROOM_SHADES.bgActive
    : hover
      ? GROUP_ROOM_SHADES.bgHover
      : "#fff";
  return {
    borderColor: GROUP_ROOM_SHADES.base,
    backgroundColor: background,
    color: GROUP_ROOM_SHADES.text,
  };
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
  const [hover, setHover] = useState(false);
  const [pressed, setPressed] = useState(false);
  const label = isActive ? "An-/Abmelden beenden" : "Schüler An- & Abmelden";
  const Icon = isActive ? UserX : UserCheck;

  return (
    <button
      type="button"
      onClick={onToggle}
      aria-pressed={isActive}
      aria-label={label}
      data-active={isActive || undefined}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => {
        setHover(false);
        setPressed(false);
      }}
      onMouseDown={() => setPressed(true)}
      onMouseUp={() => setPressed(false)}
      className="flex h-10 items-center gap-2 rounded-full border px-4 shadow-sm transition-colors duration-150"
      style={
        isActive ? activeStyle(hover, pressed) : ghostStyle(hover, pressed)
      }
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
  const [pressed, setPressed] = useState(false);
  const label = isActive ? "An-/Abmelden beenden" : "Schüler An- & Abmelden";
  const Icon = isActive ? UserX : UserCheck;

  // Mobile drops the hover state (touch-only) but keeps the pressed tint so
  // the user gets a tactile cue.
  const style = isActive
    ? activeStyle(false, pressed)
    : ghostStyle(false, pressed);

  return (
    <button
      type="button"
      onClick={onToggle}
      aria-pressed={isActive}
      aria-label={label}
      data-active={isActive || undefined}
      onTouchStart={() => setPressed(true)}
      onTouchEnd={() => setPressed(false)}
      className="relative flex h-8 w-8 items-center justify-center rounded-full border shadow-sm"
      style={style}
    >
      <Icon className="h-4 w-4" aria-hidden="true" />
      {pendingCount && pendingCount > 0 ? (
        <span
          className="absolute -top-1 -right-1 flex h-4 w-4 items-center justify-center rounded-full border bg-white text-[10px] font-bold"
          style={{
            borderColor: GROUP_ROOM_SHADES.base,
            color: GROUP_ROOM_SHADES.text,
          }}
        >
          {pendingCount}
        </span>
      ) : null}
    </button>
  );
}
