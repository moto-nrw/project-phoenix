"use client";

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
}: SelectFieldProps) {
  return (
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
      triggerClassName="moto-content-surface h-10 w-auto min-w-[8rem] max-w-xs hover:border-gray-300"
    />
  );
}
