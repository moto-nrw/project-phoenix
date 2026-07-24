"use client";

import { ChevronDown } from "lucide-react";
import { ListboxDropdown } from "./listbox-dropdown";

interface CustomSelectOption {
  readonly value: string;
  readonly label: string;
  readonly disabled?: boolean;
}

interface CustomSelectProps {
  readonly value: string;
  readonly options: readonly CustomSelectOption[];
  readonly onChange: (value: string) => void;
  readonly id?: string;
  readonly name?: string;
  readonly ariaLabel?: string;
  readonly ariaDescribedBy?: string;
  readonly labelId?: string;
  readonly placeholder?: string;
  readonly disabled?: boolean;
  readonly required?: boolean;
  readonly invalid?: boolean;
  readonly className?: string;
  readonly menuClassName?: string;
  /** Replaces the default surface/size trigger classes (moto-content-surface h-10 w-full …) for non-standard widths/heights. */
  readonly triggerClassName?: string;
  readonly testId?: string;
}

const TRIGGER_BASE_CLASS =
  "flex items-center justify-between gap-2 rounded-lg border px-3 text-left text-sm shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-500 disabled:opacity-80";

const DEFAULT_TRIGGER_CLASS =
  "moto-content-surface h-10 w-full hover:border-gray-300";

export function CustomSelect({
  value,
  options,
  onChange,
  id,
  name,
  ariaLabel,
  ariaDescribedBy,
  labelId,
  placeholder = "Bitte wählen",
  disabled = false,
  required = false,
  invalid = false,
  className = "",
  menuClassName = "",
  triggerClassName,
  testId,
}: CustomSelectProps) {
  return (
    <>
      {name ? <input type="hidden" name={name} value={value} /> : null}
      <ListboxDropdown
        id={id}
        value={value}
        options={options}
        onChange={onChange}
        ariaLabel={ariaLabel}
        ariaDescribedBy={ariaDescribedBy}
        required={required}
        invalid={invalid}
        disabled={disabled}
        placeholder={placeholder}
        triggerRole="combobox"
        testId={testId}
        className={`${TRIGGER_BASE_CLASS} ${triggerClassName ?? DEFAULT_TRIGGER_CLASS} ${
          invalid ? "border-[#FF3130] bg-[#FF3130]/5" : ""
        } ${className}`}
        menuClassName={`scrollbar-thin absolute top-full left-0 z-50 mt-1 max-h-72 min-w-full overflow-y-auto rounded-xl border border-gray-200 bg-white py-1 shadow-lg ${menuClassName}`}
        optionClassName="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50"
        activeOptionClassName="flex w-full items-center gap-2 bg-gray-50 px-4 py-2 text-left text-sm font-medium text-gray-900 transition-colors"
        disabledOptionClassName="flex w-full cursor-not-allowed items-center gap-2 px-4 py-2 text-left text-sm text-gray-400 transition-colors"
        renderTrigger={({ selectedLabel, open }) => (
          <>
            <span
              id={labelId}
              className={`min-w-0 flex-1 truncate ${
                options.some((option) => option.value === value)
                  ? "text-gray-900"
                  : "text-gray-500"
              }`}
            >
              {selectedLabel}
            </span>
            <ChevronDown
              aria-hidden="true"
              className={`h-4 w-4 flex-shrink-0 text-gray-400 transition-transform ${
                open ? "rotate-180" : ""
              }`}
            />
          </>
        )}
      />
    </>
  );
}
