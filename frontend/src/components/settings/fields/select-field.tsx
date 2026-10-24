"use client";

import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
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
   * Mehrfachauswahl (Feldtyp `multiselect`, #4794): der Wert ist die Liste der
   * angehakten Optionen, jede Option ist eine eigene Zeile zum Abhaken.
   */
  readonly multiple?: boolean;
}

function toStr(v: unknown): string {
  if (v == null) return "";
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return v.toString();
  return JSON.stringify(v);
}

/** Die angehakten Werte einer Mehrfachauswahl; alles andere ist leer. */
function selectedValues(value: unknown): string[] {
  return Array.isArray(value) ? value.map(toStr) : [];
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
    const selected = selectedValues(value);
    // Die gespeicherte Liste folgt der Reihenfolge der Optionen.
    const toggle = (option: SelectOption, checked: boolean) =>
      onChange(
        options
          .filter((opt) =>
            opt === option ? checked : selected.includes(toStr(opt.value)),
          )
          .map((opt) => opt.value),
      );
    return (
      <div
        role="group"
        aria-label={ariaLabel}
        className="flex flex-col gap-1.5"
      >
        {options.map((opt) => {
          const checked = selected.includes(toStr(opt.value));
          return (
            <ChoiceTile
              key={toStr(opt.value)}
              selected={checked}
              disabled={disabled}
              tone="green"
            >
              <Checkbox
                checked={checked}
                disabled={disabled}
                onChange={(event) => toggle(opt, event.target.checked)}
              />
              <span className="min-w-0">{opt.label}</span>
            </ChoiceTile>
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
