"use client";

/* oxlint-disable jsx-a11y/no-noninteractive-tabindex, jsx-a11y/no-static-element-interactions -- the ARIA tooltip pattern requires a focusable trigger even on non-interactive text; the only handler is Escape-to-dismiss */

import { useId } from "react";

import { cn } from "~/lib/utils";

/**
 * Tooltip: the shared kit tooltip.
 *
 * Unlike a bare `title` attribute, the bubble is reachable on every input:
 * hover shows it, keyboard focus shows it (the trigger is focusable), and on
 * touch a tap focuses the trigger. Escape blurs the trigger to dismiss.
 *
 * CSS-only visibility keeps it dependency-free; the bubble stays in the DOM
 * and is linked via aria-describedby, so screen readers announce it with the
 * trigger. Content is plain text — for rich overlays use Drawer/Modal.
 *
 * Placement is below the trigger, left-aligned. Inside overflow containers
 * check for clipping and pass `bubbleClassName` to reposition if needed.
 */
interface TooltipProps {
  /** Plain-text content, announced via aria-describedby. */
  content: string;
  children: React.ReactNode;
  /** Extra classes for the wrapper span. */
  className?: string;
  /** Extra classes for the bubble (e.g. to adjust placement). */
  bubbleClassName?: string;
}

export function Tooltip({
  content,
  children,
  className,
  bubbleClassName,
}: TooltipProps) {
  const id = useId();
  return (
    <span className={cn("group relative inline-block", className)}>
      <span
        tabIndex={0}
        aria-describedby={id}
        className="rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-gray-300"
        onKeyDown={(event) => {
          if (event.key === "Escape") event.currentTarget.blur();
        }}
      >
        {children}
      </span>
      <span
        role="tooltip"
        id={id}
        className={cn(
          "pointer-events-none invisible absolute top-full left-0 z-20 mt-1 w-max max-w-xs rounded-md bg-gray-900 px-2 py-1 text-xs font-medium text-white opacity-0 shadow-lg transition-opacity",
          "group-focus-within:visible group-focus-within:opacity-100 group-hover:visible group-hover:opacity-100",
          bubbleClassName,
        )}
      >
        {content}
      </span>
    </span>
  );
}
