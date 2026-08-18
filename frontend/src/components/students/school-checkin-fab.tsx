"use client";

import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { Check, CheckSquare, Loader2 } from "lucide-react";
import { GROUP_ROOM_SHADES } from "~/lib/location-helper";
import { useAttendanceWebEnabled } from "~/lib/tenant-context";
import { useFloatingFabOffset } from "~/lib/hooks/use-floating-fab-offset";

interface SchoolCheckinFabBaseProps {
  /** Whether the page is currently in check-in/out mode. */
  readonly isActive: boolean;
  /** Toggle the mode on/off. */
  readonly onToggle: () => void;
  /** Successful per-session toggles — surfaced as a counter on the ON state. */
  readonly successCount: number;
  /** Open API calls in flight — surfaces as a small spinner badge top-right. */
  readonly pendingCount: number;
  /**
   * Locks the trigger, e.g. while a bulk request is in flight (#2359): the
   * hook ignores mode exits during a run, so the button mirrors that as a
   * visible disabled state instead of a dead tap.
   */
  readonly disabled?: boolean;
}

type SchoolCheckinFabProps = SchoolCheckinFabBaseProps &
  (
    | {
        /** Floating trigger shown from `md` until this exclusive breakpoint. */
        readonly variant: "floating";
        readonly floatingUntil: "lg" | "xl";
      }
    | {
        /** Header action used after the page's floating breakpoint. */
        readonly variant: "inline";
        readonly floatingUntil?: never;
      }
  );

/**
 * Shared check-in/out mode trigger used on both the OGS-Groups and
 * Students/Search pages.
 *
 * - `floating` variant: anchored bottom-right with `safe-area-inset` padding
 *   so it clears the iPhone home indicator. Used from `md` to the page's
 *   explicit `floatingUntil` breakpoint.
 * - `inline` variant: relative-flow pill that lives inside the page header's
 *   primaryAction slot. Default at ≥1024px so desktop reads as a native
 *   page-level action instead of a mobile-style floating button.
 *
 * OFF state uses a subtle ghost style (white surface, brand-green border +
 * text) so it doesn't dominate the page. ON state flips to solid brand-green
 * to give an unambiguous "mode is active" cue. Icon morphs in place via
 * framer-motion + AnimatePresence.
 */
export function SchoolCheckinFab({
  isActive,
  onToggle,
  successCount,
  pendingCount,
  variant,
  floatingUntil,
  disabled = false,
}: SchoolCheckinFabProps) {
  const attendanceWebEnabled = useAttendanceWebEnabled();
  const reduceMotion = useReducedMotion();
  const resolved = variant;

  // Announce the space this FAB occupies only across the viewport range where
  // its parent wrapper displays it. The two pages intentionally stop at
  // different breakpoints (`lg` for OGS groups, `xl` for student search).
  const occupiesBottomSpace = attendanceWebEnabled && variant === "floating";
  const maxFloatingWidth = floatingUntil === "lg" ? 1023 : 1279;
  useFloatingFabOffset({
    active: occupiesBottomSpace,
    mediaQuery: `(min-width: 768px) and (max-width: ${maxFloatingWidth}px)`,
  });

  if (!attendanceWebEnabled) return null;

  const label = isActive
    ? "An- und Abmelde-Modus beenden"
    : "Kinder an- und abmelden";

  const positionClasses =
    resolved === "floating"
      ? // Anchored bottom-right above the mobile bottom nav. The nav is
        // ~4.75rem visible (pill + paddings) + its own safe-area-inset.
        // Offset = 5.5rem (nav + breathing gap) + safe-area so the FAB
        // visually floats above the nav with a clear separator.
        "fixed right-4 bottom-[calc(5.5rem+env(safe-area-inset-bottom))] z-40"
      : "relative z-0";

  // OFF = subtle ghost (white pill, brand-green border + text). Reads as
  // available-but-not-dominating. ON = solid brand-green so the active state
  // is unmistakable.
  const surfaceStyle: React.CSSProperties = isActive
    ? {
        backgroundColor: GROUP_ROOM_SHADES.base,
        color: "var(--color-white)",
        boxShadow:
          resolved === "floating"
            ? `0 12px 28px ${GROUP_ROOM_SHADES.base}66, 0 4px 10px rgb(0 0 0 / 0.10)`
            : `0 1px 3px rgb(0 0 0 / 0.08)`,
      }
    : {
        backgroundColor: "var(--color-white)",
        color: GROUP_ROOM_SHADES.text,
        boxShadow:
          resolved === "floating"
            ? // Layered: a soft brand-tinted glow + a faint black drop for
              // depth. Subtle enough that the white pill doesn't shout.
              `inset 0 0 0 1.5px ${GROUP_ROOM_SHADES.base}, 0 8px 24px ${GROUP_ROOM_SHADES.base}33, 0 2px 6px rgb(0 0 0 / 0.06)`
            : `inset 0 0 0 1.5px ${GROUP_ROOM_SHADES.base}, 0 1px 2px rgb(0 0 0 / 0.04)`,
      };

  const sizeClasses =
    resolved === "floating" ? "h-14 px-5 text-base" : "h-10 px-4 text-sm";

  const iconBubbleStyle: React.CSSProperties = isActive
    ? { backgroundColor: "rgb(255 255 255 / 0.20)" }
    : { backgroundColor: `${GROUP_ROOM_SHADES.base}1a` };

  const buttonStyle: React.CSSProperties = {
    ...surfaceStyle,
    // @ts-expect-error CSS custom property for focus ring colour
    "--tw-ring-color": `${GROUP_ROOM_SHADES.base}80`,
  };

  const animationProps = reduceMotion
    ? {}
    : {
        initial: { opacity: 0, y: 4 },
        animate: { opacity: 1, y: 0 },
        exit: { opacity: 0, y: -4 },
        transition: { duration: 0.15, ease: "easeOut" as const },
      };

  return (
    <div className={positionClasses}>
      <button
        type="button"
        onClick={onToggle}
        disabled={disabled}
        aria-pressed={isActive}
        aria-label={label}
        data-checkin-fab-variant={resolved}
        data-checkin-fab-active={isActive || undefined}
        className={`group relative inline-flex items-center gap-2.5 rounded-full font-semibold transition-[background-color,color,box-shadow] duration-150 outline-none focus-visible:ring-2 focus-visible:ring-offset-2 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-60 disabled:active:scale-100 ${sizeClasses}`}
        style={buttonStyle}
      >
        <span
          className={`flex flex-shrink-0 items-center justify-center rounded-full ${
            resolved === "floating" ? "size-6" : "size-5"
          }`}
          style={iconBubbleStyle}
        >
          <AnimatePresence mode="wait" initial={false}>
            {isActive ? (
              <motion.span key="check" {...animationProps} className="flex">
                <Check
                  className={resolved === "floating" ? "size-4" : "size-3.5"}
                  strokeWidth={3}
                  aria-hidden
                />
              </motion.span>
            ) : (
              <motion.span key="checkbox" {...animationProps} className="flex">
                <CheckSquare
                  className={resolved === "floating" ? "size-4" : "size-3.5"}
                  strokeWidth={2.5}
                  aria-hidden
                />
              </motion.span>
            )}
          </AnimatePresence>
        </span>

        <AnimatePresence mode="wait" initial={false}>
          {isActive ? (
            <motion.span
              key="active-label"
              {...animationProps}
              className="flex items-baseline gap-1.5"
            >
              <span>Fertig</span>
              <span className="tabular-nums opacity-90">· {successCount}</span>
            </motion.span>
          ) : (
            <motion.span key="idle-label" {...animationProps}>
              An- &amp; Abmelden
            </motion.span>
          )}
        </AnimatePresence>

        {pendingCount > 0 ? (
          <span
            className="absolute -top-1 -right-1 flex size-5 items-center justify-center rounded-full bg-white text-[10px] font-bold tabular-nums shadow-sm"
            style={{ color: GROUP_ROOM_SHADES.text }}
            aria-label={`${pendingCount} Aktionen laufen`}
          >
            <Loader2 className="size-3 animate-spin" aria-hidden />
          </span>
        ) : null}
      </button>
    </div>
  );
}
