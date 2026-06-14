"use client";

import { Check } from "lucide-react";

import { cn } from "~/lib/utils";

/**
 * Checkbox: the shared kit checkbox.
 *
 * Visually matches the custom room drawer checkbox while keeping a native
 * input for forms, labels, and keyboard interaction.
 *
 * It is only the control. Wrap it in your own <label> for the row layout you
 * need so it composes into dense multi-select lists and single toggle rows
 * alike.
 */
type CheckboxProps = Omit<React.InputHTMLAttributes<HTMLInputElement>, "type">;

export function Checkbox({ className, ...props }: CheckboxProps) {
  return (
    <>
      <input type="checkbox" className="peer sr-only" {...props} />
      <span
        className={cn(
          "pointer-events-none flex h-5 w-5 shrink-0 items-center justify-center rounded-md border border-gray-300 bg-white shadow-sm transition-all",
          "peer-checked:border-gray-900 peer-checked:bg-gray-900",
          "peer-checked:[&>svg]:opacity-100",
          "peer-focus-visible:ring-2 peer-focus-visible:ring-gray-300 peer-focus-visible:outline-none",
          "peer-disabled:opacity-50",
          className,
        )}
        aria-hidden="true"
      >
        <Check className="h-3.5 w-3.5 text-white opacity-0 transition-opacity" />
      </span>
    </>
  );
}
