"use client";

import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";

interface SelectOption {
  readonly label: string;
  readonly value: unknown;
}

interface SelectFieldProps {
  readonly ariaLabel?: string;
  readonly value: unknown;
  readonly onChange: (value: unknown) => void;
  readonly options: SelectOption[];
  readonly disabled?: boolean;
  /**
   * Multi-select: one checkbox per option. `value` is the list of chosen
   * option values, and `onChange` receives the new list in option order.
   */
  readonly multiple?: boolean;
}

function toStr(v: unknown): string {
  if (v == null) return "";
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return v.toString();
  return JSON.stringify(v);
}

export function SelectField({
  ariaLabel = "Einstellung",
  value,
  onChange,
  options,
  disabled = false,
  multiple = false,
}: SelectFieldProps) {
  if (multiple) {
    const chosen = new Set(Array.isArray(value) ? value.map(toStr) : []);
    return (
      <div role="group" aria-label={ariaLabel} className="flex flex-col gap-2">
        {options.map((opt) => {
          const key = toStr(opt.value);
          return (
            <label
              key={key}
              className="flex items-start gap-2 text-sm text-gray-700"
            >
              <Checkbox
                checked={chosen.has(key)}
                disabled={disabled}
                onChange={(event) => {
                  const next = new Set(chosen);
                  if (event.target.checked) next.add(key);
                  else next.delete(key);
                  onChange(
                    options
                      .filter((option) => next.has(toStr(option.value)))
                      .map((option) => option.value),
                  );
                }}
              />
              <span>{opt.label}</span>
            </label>
          );
        })}
      </div>
    );
  }
  return (
    <div className="max-w-full min-w-0">
      <CustomSelect
        ariaLabel={ariaLabel}
        value={toStr(value)}
        options={options.map((opt) => ({
          value: toStr(opt.value),
          label: opt.label,
        }))}
        onChange={(next) => {
          const selected = options.find((opt) => toStr(opt.value) === next);
          if (selected) {
            onChange(selected.value);
          }
        }}
        disabled={disabled}
        // moto-content-surface is unlayered and would paint over the kit's
        // grey disabled state, so a read-only select looked editable.
        triggerClassName={
          disabled
            ? "h-10 w-auto min-w-[8rem] max-w-full border-gray-200 sm:max-w-xs"
            : "moto-content-surface h-10 w-auto min-w-[8rem] max-w-full hover:border-gray-300 sm:max-w-xs"
        }
      />
    </div>
  );
}
