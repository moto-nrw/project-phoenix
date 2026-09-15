"use client";

import { ChevronDown } from "lucide-react";
import { type CSSProperties, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
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
  readonly ariaLabelledBy?: string;
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

// The trigger is a <button>, which is barred from constraint validation, and
// so is the hidden input carrying the value — a `required` CustomSelect would
// otherwise submit empty unless every caller hand-rolls a check. This mirror
// <select> restores the native behaviour: the form refuses to submit, the
// browser reports the message, and submit handlers never run.
//
// It is portaled into the closest <form> instead of rendering in place so it
// can never be captured by a wrapping <label> (which would then label two
// controls) and never disturbs the field's own layout. It is invisible but
// still rendered and focusable — Chrome only reports validity for a focusable
// control, otherwise it blocks the submit silently with a console warning.
const VALIDATION_SELECT_STYLE: CSSProperties = {
  position: "fixed",
  width: 1,
  height: 1,
  padding: 0,
  margin: 0,
  border: 0,
  opacity: 0,
  pointerEvents: "none",
};

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
  ariaLabelledBy,
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
  const containerRef = useRef<HTMLDivElement | null>(null);
  const validationRef = useRef<HTMLSelectElement | null>(null);
  const [form, setForm] = useState<HTMLFormElement | null>(null);
  const hasEmptyOption = options.some((option) => option.value === "");

  useEffect(() => {
    if (!required) {
      setForm(null);
      return;
    }
    setForm(containerRef.current?.closest("form") ?? null);
  }, [required]);

  // The browser anchors its validation message to the invalid control, so park
  // the mirror over the trigger right before the message appears — written
  // straight to the DOM because React would not flush a state update in time.
  const alignValidationSelect = () => {
    const select = validationRef.current;
    const trigger = containerRef.current?.querySelector("button");
    if (!select || !trigger) return;
    const rect = trigger.getBoundingClientRect();
    select.style.left = `${rect.left}px`;
    select.style.top = `${rect.bottom}px`;
    select.style.width = `${rect.width}px`;
  };

  return (
    <>
      {name ? (
        // Mirror a disabled native <select>: a disabled control is excluded
        // from FormData, so the hidden input must not submit a stale value.
        <input type="hidden" name={name} value={value} disabled={disabled} />
      ) : null}
      <ListboxDropdown
        containerNodeRef={containerRef}
        id={id}
        value={value}
        options={options}
        onChange={onChange}
        ariaLabel={ariaLabel}
        ariaLabelledBy={ariaLabelledBy}
        ariaDescribedBy={ariaDescribedBy}
        required={required}
        invalid={invalid}
        disabled={disabled}
        placeholder={placeholder}
        triggerRole="combobox"
        testId={testId}
        className={`${TRIGGER_BASE_CLASS} ${triggerClassName ?? DEFAULT_TRIGGER_CLASS} ${
          invalid ? "border-moto-red bg-moto-red/5" : ""
        } ${className}`}
        menuClassName={`scrollbar-thin overflow-y-auto moto-popover-surface rounded-xl border py-1 ${menuClassName}`}
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
      {form
        ? createPortal(
            <select
              ref={validationRef}
              value={value}
              onChange={(event) => onChange(event.target.value)}
              onInvalid={alignValidationSelect}
              required
              // A disabled control is barred from validation, exactly like a
              // disabled native <select>.
              disabled={disabled}
              aria-hidden="true"
              tabIndex={-1}
              data-validation-mirror={id ?? name ?? true}
              style={VALIDATION_SELECT_STYLE}
            >
              {/* A required select is only "missing" while the selected
                  option's value is empty — so an empty option must exist to
                  hold "". Options carry no labels: only their values matter,
                  and repeating the labels would duplicate visible text. */}
              {hasEmptyOption ? null : <option value="" />}
              {options.map((option) => (
                <option key={option.value} value={option.value} />
              ))}
            </select>,
            form,
          )
        : null}
    </>
  );
}
