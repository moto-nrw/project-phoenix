"use client";

import { cn } from "~/lib/utils";

/**
 * Radio: the shared kit radio button.
 *
 * Mirrors the Checkbox pattern: a native input stays in the DOM for forms,
 * labels, and keyboard interaction, visually replaced by a styled sibling.
 *
 * It is only the control. Wrap it in your own <label> for the row layout you
 * need (one control per label), and group exclusive options via a shared
 * `name`.
 */
type RadioProps = Omit<React.InputHTMLAttributes<HTMLInputElement>, "type">;

export function Radio({ className, ...props }: RadioProps) {
  return (
    <>
      <input type="radio" className="peer sr-only" {...props} />
      <span
        className={cn(
          "pointer-events-none flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-gray-300 bg-white shadow-sm transition-all",
          "peer-checked:border-gray-900 peer-checked:bg-gray-900",
          "peer-checked:[&>span]:opacity-100",
          "peer-focus-visible:ring-2 peer-focus-visible:ring-gray-300 peer-focus-visible:outline-none",
          "peer-disabled:opacity-50",
          className,
        )}
        aria-hidden="true"
      >
        <span className="h-2 w-2 rounded-full bg-white opacity-0 transition-opacity" />
      </span>
    </>
  );
}
