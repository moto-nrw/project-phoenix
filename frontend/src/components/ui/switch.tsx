"use client";

import { useId, type ReactNode } from "react";

import { FieldLabel } from "~/components/ui/field-label";
import { cn } from "~/lib/utils";

/**
 * Switch: the shared on/off control. An on/off that takes effect as a setting;
 * for an option someone ticks in a form use `Checkbox`.
 *
 * It needs a name: `ariaLabel`, or `aria-labelledby` pointing at visible text.
 * `SwitchRow` wires its label for you.
 */
type SwitchName =
  | { ariaLabel: string; "aria-labelledby"?: undefined }
  | { ariaLabel?: undefined; "aria-labelledby": string };

export type SwitchProps = Readonly<
  {
    checked: boolean;
    onCheckedChange: (checked: boolean) => void;
    disabled?: boolean;
    id?: string;
    "aria-describedby"?: string;
    className?: string;
  } & SwitchName
>;

export function Switch({
  checked,
  onCheckedChange,
  ariaLabel,
  disabled = false,
  className,
  ...rest
}: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={cn(
        "relative inline-flex h-7 w-12 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out",
        "focus-visible:ring-2 focus-visible:ring-gray-600 focus-visible:ring-offset-2 focus-visible:outline-none",
        "disabled:cursor-not-allowed disabled:opacity-50",
        checked ? "bg-gray-900" : "bg-gray-200",
        className,
      )}
      {...rest}
    >
      <span
        aria-hidden="true"
        className={cn(
          "pointer-events-none inline-block h-6 w-6 rounded-full bg-white shadow transition duration-200 ease-in-out",
          checked ? "translate-x-5" : "translate-x-0",
        )}
      />
    </button>
  );
}

/**
 * SwitchRow: a label, an optional description and a `Switch` on the right.
 * The label names the switch, and a click on it toggles the switch.
 *
 * `framed` (default) draws the row's own surface for a row that stands alone;
 * pass `framed={false}` for rows inside a panel that already has a frame.
 * `ariaLabel` is only for a label that changes with the state: the switch then
 * keeps one stable name.
 */
export function SwitchRow({
  label,
  description,
  checked,
  onCheckedChange,
  disabled = false,
  framed = true,
  ariaLabel,
  className,
}: Readonly<{
  label: ReactNode;
  description?: ReactNode;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  framed?: boolean;
  ariaLabel?: string;
  className?: string;
}>) {
  const baseId = useId();
  const switchId = `${baseId}-switch`;
  const labelId = `${baseId}-label`;
  const descriptionId = `${baseId}-description`;
  const name = ariaLabel ? { ariaLabel } : { "aria-labelledby": labelId };
  return (
    <div
      className={cn(
        "flex items-center justify-between gap-4",
        framed && "moto-content-surface rounded-xl border px-4 py-3",
        className,
      )}
    >
      <div className="min-w-0 flex-1">
        <FieldLabel
          id={labelId}
          htmlFor={switchId}
          className={cn(
            "mb-0 text-gray-900",
            disabled ? "cursor-not-allowed" : "cursor-pointer",
          )}
        >
          {label}
        </FieldLabel>
        {description ? (
          <p id={descriptionId} className="mt-0.5 text-xs text-gray-500">
            {description}
          </p>
        ) : null}
      </div>
      <Switch
        id={switchId}
        checked={checked}
        onCheckedChange={onCheckedChange}
        disabled={disabled}
        aria-describedby={description ? descriptionId : undefined}
        {...name}
      />
    </div>
  );
}
