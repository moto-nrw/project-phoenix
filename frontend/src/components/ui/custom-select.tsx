"use client";

import { type CSSProperties, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { cn } from "~/lib/utils";
import { FieldShell, useFieldIds } from "./field-shell";
import {
  ListboxDropdown,
  ListboxTriggerContent,
  useListboxSearch,
  type ListboxDropdownSize,
} from "./listbox-dropdown";

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
  /**
   * Visible label above the field. Renders the kit `FieldLabel` (with the
   * star when `required`) and names the field by it. Without it, name the
   * field through `ariaLabel` or `ariaLabelledBy`.
   */
  readonly label?: string;
  readonly ariaLabel?: string;
  readonly ariaLabelledBy?: string;
  readonly ariaDescribedBy?: string;
  readonly placeholder?: string;
  readonly disabled?: boolean;
  readonly required?: boolean;
  /** Marks the field as failed without a message of its own. */
  readonly invalid?: boolean;
  /** Message under the field; marks it as failed (`aria-invalid`). */
  readonly error?: string | null;
  /**
   * Read the error out the moment it appears (`role="alert"`), for a check
   * that runs while the person edits. A message after Speichern stays quiet:
   * the form's `FormErrorAlert` announces the failure once.
   */
  readonly errorLive?: boolean;
  /** Gray help line under the field. */
  readonly hint?: string;
  /**
   * Filter field at the top of the menu for long lists (all children, all
   * absence types). Typeahead switches off while it is there.
   */
  readonly searchable?: boolean;
  readonly searchPlaceholder?: string;
  /** Field height: 40 px (`compact`) by default, `dense` 36 px for dense rows. */
  readonly controlSize?: ListboxDropdownSize;
  /** Layout classes for the trigger (width, flex). The look is the kit's. */
  readonly className?: string;
  /** Layout classes for the menu, e.g. a minimum width. */
  readonly menuClassName?: string;
  /** Keep long labels fully visible in both the trigger and the menu. */
  readonly wrapLabels?: boolean;
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

export function CustomSelect({
  value,
  options,
  onChange,
  id,
  name,
  label,
  ariaLabel,
  ariaLabelledBy,
  ariaDescribedBy,
  placeholder = "Bitte wählen",
  disabled = false,
  required = false,
  invalid = false,
  error,
  errorLive = false,
  hint,
  searchable = false,
  searchPlaceholder = "Suchen",
  controlSize,
  className = "",
  menuClassName = "",
  wrapLabels = false,
  testId,
}: CustomSelectProps) {
  const ids = useFieldIds({
    id,
    hint,
    error,
    describedBy: ariaDescribedBy,
  });
  // Without label, hint and error the trigger gets no id unless the caller
  // sets one.
  const fieldId = id ?? (label || error || hint ? ids.id : undefined);
  const labelId = label ? `${fieldId}-label` : undefined;
  const isInvalid = invalid || Boolean(error);

  const containerRef = useRef<HTMLDivElement | null>(null);
  const validationRef = useRef<HTMLSelectElement | null>(null);
  const [form, setForm] = useState<HTMLFormElement | null>(null);
  const hasEmptyOption = options.some((option) => option.value === "");
  const selectedOption = options.find((option) => option.value === value);
  const { visibleOptions, listboxProps } = useListboxSearch(
    options,
    searchable,
    searchPlaceholder,
  );

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

  const field = (
    <>
      {name ? (
        // Mirror a disabled native <select>: a disabled control is excluded
        // from FormData, so the hidden input must not submit a stale value.
        <input type="hidden" name={name} value={value} disabled={disabled} />
      ) : null}
      <ListboxDropdown
        containerNodeRef={containerRef}
        id={fieldId}
        value={value}
        options={visibleOptions}
        onChange={onChange}
        ariaLabel={ariaLabel}
        ariaLabelledBy={ariaLabelledBy ?? labelId}
        ariaDescribedBy={ids.describedBy}
        required={required}
        invalid={isInvalid}
        disabled={disabled}
        placeholder={placeholder}
        triggerRole="combobox"
        testId={testId}
        size={controlSize}
        className={cn(
          "w-full",
          wrapLabels &&
            (controlSize === "dense"
              ? "h-auto min-h-9 py-2"
              : "h-auto min-h-10 py-2"),
          className,
        )}
        menuClassName={menuClassName}
        {...listboxProps}
        renderOption={
          wrapLabels
            ? (option) => (
                <span className="min-w-0 flex-1 wrap-anywhere whitespace-normal">
                  {option.label}
                </span>
              )
            : undefined
        }
        renderTrigger={({ open }) => (
          // The label comes from the FULL list: while a search narrows the
          // menu, the trigger must keep showing what is chosen.
          <ListboxTriggerContent
            label={selectedOption?.label ?? placeholder}
            placeholderShown={selectedOption === undefined}
            open={open}
            wrap={wrapLabels}
          />
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
              data-validation-mirror={fieldId ?? name ?? true}
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

  return (
    <FieldShell
      ids={ids}
      className="min-w-0"
      label={label}
      labelId={labelId}
      required={required}
      invalid={isInvalid}
      hint={hint}
      error={error}
      errorLive={errorLive}
    >
      {field}
    </FieldShell>
  );
}
