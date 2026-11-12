"use client";

import { between } from "~/lib/hooks/use-media-query";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { Check, CheckSquare } from "lucide-react";
import { CHECKIN_HINTS } from "~/components/students/school-checkin-hints";
import { useAttendanceWebEnabled } from "~/lib/tenant-context";
import { useFloatingFabOffset } from "~/lib/hooks/use-floating-fab-offset";
import { Button } from "~/components/ui/button";
import { SpinnerIcon } from "~/components/ui/icons";

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
 *   explicit `floatingUntil` breakpoint (exclusive), matching the wrapper's
 *   `md:block lg:hidden` or `md:block xl:hidden` classes.
 * - `inline` variant: relative-flow pill that lives inside the page header's
 *   primaryAction slot. Shown from the page's floating breakpoint so desktop reads as a native
 *   page-level action instead of a mobile-style floating button.
 *
 * Kit button (#5608): OFF is `surface` (opaque white) so it doesn't dominate
 * the page and stays readable while floating over cards (`surface_success`
 * is translucent); ON is `success` (solid green) to give an unambiguous
 * "mode is active" cue. Icon morphs in place via framer-motion +
 * AnimatePresence.
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
  useFloatingFabOffset({
    active: occupiesBottomSpace,
    mediaQuery: between("md", floatingUntil ?? "xl"),
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
      <Button
        type="button"
        onClick={onToggle}
        disabled={disabled}
        aria-pressed={isActive}
        aria-label={label}
        tooltip={isActive ? CHECKIN_HINTS.finish : CHECKIN_HINTS.start}
        data-checkin-fab-variant={resolved}
        data-checkin-fab-active={isActive || undefined}
        variant={isActive ? "success" : "surface"}
        size={resolved === "floating" ? "base" : "md"}
        className="relative gap-2"
      >
        <AnimatePresence mode="wait" initial={false}>
          {isActive ? (
            <motion.span key="check" {...animationProps} className="flex">
              <Check className="size-4" strokeWidth={3} aria-hidden />
            </motion.span>
          ) : (
            <motion.span key="checkbox" {...animationProps} className="flex">
              <CheckSquare className="size-4" strokeWidth={2.5} aria-hidden />
            </motion.span>
          )}
        </AnimatePresence>

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
            className="text-moto-green-strong absolute -top-1 -right-1 flex size-5 items-center justify-center rounded-full bg-white shadow-sm"
            aria-label={`${pendingCount} Aktionen laufen`}
          >
            <SpinnerIcon className="size-3" />
          </span>
        ) : null}
      </Button>
    </div>
  );
}
