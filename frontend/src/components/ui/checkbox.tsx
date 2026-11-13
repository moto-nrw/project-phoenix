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
// ComponentProps (not InputHTMLAttributes) so a caller can pass `ref`, e.g.
// to set the native `indeterminate` state of a "select all" box.
type CheckboxProps = Omit<React.ComponentProps<"input">, "type">;

const BOX_CLASS =
  "pointer-events-none flex h-5 w-5 shrink-0 items-center justify-center rounded-md border shadow-sm transition-all";
const BOX_UNCHECKED_CLASS = "border-gray-300 bg-white";
const BOX_CHECKED_CLASS = "border-gray-900 bg-gray-900";

export function Checkbox({ className, ...props }: CheckboxProps) {
  return (
    <>
      <input type="checkbox" className="peer sr-only" {...props} />
      <span
        className={cn(
          BOX_CLASS,
          BOX_UNCHECKED_CLASS,
          "peer-checked:border-gray-900 peer-checked:bg-gray-900",
          "peer-checked:[&>svg]:opacity-100",
          "peer-focus-visible:ring-2 peer-focus-visible:ring-gray-600 peer-focus-visible:outline-none",
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

/**
 * The checkbox's box without the native input, for a row that already is the
 * control: an option of a multi-select listbox carries its checked state in
 * `aria-selected`, and an input inside that option button would be an
 * interactive element inside another. Same look as {@link Checkbox}.
 */
export function CheckboxIndicator({
  checked,
  disabled = false,
}: Readonly<{ checked: boolean; disabled?: boolean }>) {
  return (
    <span
      className={cn(
        BOX_CLASS,
        checked ? BOX_CHECKED_CLASS : BOX_UNCHECKED_CLASS,
        disabled && "opacity-50",
      )}
      aria-hidden="true"
    >
      {checked ? <Check className="h-3.5 w-3.5 text-white" /> : null}
    </span>
  );
}
