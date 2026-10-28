"use client";

// Die Renderer-Bausteine des Formular-Baukastens (#5198): eine Frage mit
// einfachem Typ (Ja/Nein, Text, langer Text, Auswahl, Zahl) und ein
// Infotext. Datum und die strukturierten Typen der Online-Anmeldung kommen
// über `renderStructured` (Datum: `custom-date-field-input.tsx`), damit die
// Ferienanmeldung den Kalender und die strukturierten Eingaben nicht lädt.

import type { ReactNode } from "react";
import { AttachmentList } from "~/components/enrollment/attachment-list";
import { CustomSelect } from "~/components/ui/custom-select";
import type { FormField } from "~/lib/enrollment-form-schema-api";

/** Reads the texts of `enrollmentForm` the renderer needs. */
export type FieldInputTranslator = (key: string) => string;

/** A read-only text block of a form: heading, text and attachments. */
export function InfoBlock({ field }: { readonly field: FormField }) {
  return (
    <div className="border-moto-blue/20 bg-moto-blue/5 rounded-lg border p-3">
      {field.label && (
        <p className="text-sm font-semibold text-gray-900">{field.label}</p>
      )}
      {field.content && (
        <p className="mt-1 text-sm leading-6 whitespace-pre-line text-gray-600">
          {field.content}
        </p>
      )}
      <AttachmentList attachments={field.attachments} />
    </div>
  );
}

/** The question with its required marker and help text. */
export function CustomFieldLabel({ field }: { readonly field: FormField }) {
  return (
    <>
      <span className="block text-sm font-semibold text-gray-700">
        {field.label}
        {field.required && <span className="text-moto-red"> *</span>}
      </span>
      {field.help_text && (
        <span className="mt-1 block text-xs leading-5 text-gray-500">
          {field.help_text}
        </span>
      )}
    </>
  );
}

export interface CustomFieldInputProps {
  readonly field: FormField;
  // The control's `name`, unique per child, so a server field error and the
  // focus jump find the right input (#2515).
  readonly inputName: string;
  readonly value: unknown;
  readonly onChange: (v: unknown) => void;
  readonly error?: string;
  readonly tr: FieldInputTranslator;
  /** The id of a date control; unique per child where a form repeats it. */
  readonly controlId?: string;
  /**
   * Renders a date or a structured type (phone list, weekdays, contacts,
   * file) the shared renderer does not know. Without it such a field
   * renders nothing.
   */
  readonly renderStructured?: () => ReactNode;
}

const SIMPLE_TYPES = new Set<FormField["type"]>([
  "boolean",
  "textarea",
  "select",
  "text",
  "number",
]);

/** One answer-collecting question with its label, help text and error. */
export function CustomFieldInput({
  field,
  inputName,
  value,
  onChange,
  error,
  tr,
  renderStructured,
}: CustomFieldInputProps) {
  if (!SIMPLE_TYPES.has(field.type)) {
    return renderStructured?.() ?? null;
  }
  const labelEl = <CustomFieldLabel field={field} />;
  const valueStr = typeof value === "string" ? value : "";

  if (field.type === "boolean") {
    const selectedValue = value === true ? "yes" : value === false ? "no" : "";
    return (
      <fieldset aria-invalid={error ? "true" : undefined}>
        {labelEl}
        <div className="mt-2 grid grid-cols-2 gap-2">
          {[
            { label: tr("structured.yes"), value: "yes", raw: true },
            { label: tr("structured.no"), value: "no", raw: false },
          ].map((option) => (
            <label
              key={option.value}
              className={`flex h-10 cursor-pointer items-center justify-center rounded-lg border px-3 text-sm font-medium transition-colors ${
                selectedValue === option.value
                  ? "border-gray-900 bg-gray-900 text-white"
                  : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
              }`}
            >
              <input
                type="radio"
                name={inputName}
                value={option.value}
                checked={selectedValue === option.value}
                onChange={() => onChange(option.raw)}
                className="sr-only"
              />
              {option.label}
            </label>
          ))}
        </div>
        {error && <p className="text-moto-red mt-1 text-xs">{error}</p>}
      </fieldset>
    );
  }
  if (field.type === "textarea") {
    return (
      <label className="block">
        {labelEl}
        <textarea
          value={valueStr}
          onChange={(e) => onChange(e.target.value)}
          rows={3}
          name={inputName}
          className={`moto-content-surface mt-1 w-full rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${error ? "border-moto-red!" : ""}`}
          aria-required={field.required}
          aria-invalid={error ? "true" : undefined}
        />
        {error && <p className="text-moto-red mt-1 text-xs">{error}</p>}
      </label>
    );
  }
  if (field.type === "select") {
    return (
      <label className="block">
        {labelEl}
        <CustomSelect
          ariaLabel={field.label}
          value={valueStr}
          onChange={onChange}
          name={inputName}
          required={field.required}
          invalid={Boolean(error)}
          className="mt-1"
          options={[
            { value: "", label: tr("fields.choose") },
            ...(field.options ?? []).map((option) => ({
              value: option.value,
              label: option.label,
            })),
          ]}
        />
        {error && <p className="text-moto-red mt-1 text-xs">{error}</p>}
      </label>
    );
  }
  const inputType = field.type === "number" ? "number" : "text";
  return (
    <label className="block">
      {labelEl}
      <input
        name={inputName}
        type={inputType}
        value={valueStr}
        onChange={(e) => onChange(e.target.value)}
        aria-required={field.required}
        aria-invalid={error ? "true" : undefined}
        className={`moto-content-surface mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${error ? "border-moto-red!" : ""}`}
      />
      {error && <p className="text-moto-red mt-1 text-xs">{error}</p>}
    </label>
  );
}
