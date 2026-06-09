"use client";

import { cn } from "~/lib/utils";

/**
 * Checkbox — the shared kit checkbox.
 *
 * One styled native <input type="checkbox"> so every checkbox in the app
 * shares the same size, radius, brand-green checked state (#83CD2D =
 * LOCATION_COLORS.GROUP_ROOM) and focus ring. Before this existed, checkboxes
 * drifted across purple-600 / green-600 / blue-600 / gray-900 accents — this
 * is the single source of truth.
 *
 * It is only the control. Wrap it in your own <label> for the row layout you
 * need so it composes into dense multi-select lists and single toggle rows
 * alike.
 */
type CheckboxProps = Omit<React.InputHTMLAttributes<HTMLInputElement>, "type">;

export function Checkbox({ className, ...props }: CheckboxProps) {
  return (
    <input
      type="checkbox"
      className={cn(
        "h-4 w-4 shrink-0 cursor-pointer rounded border-gray-300 accent-[#83CD2D]",
        "focus-visible:ring-2 focus-visible:ring-[#83CD2D]/40 focus-visible:outline-none",
        "disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  );
}
