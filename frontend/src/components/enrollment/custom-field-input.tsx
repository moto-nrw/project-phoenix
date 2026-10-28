"use client";

import { AttachmentList } from "~/components/enrollment/attachment-list";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { FieldLabel, RequiredMark } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { Radio } from "~/components/ui/radio";
import { Textarea } from "~/components/ui/textarea";
import type { ReactNode } from "react";
import type { FormField } from "~/lib/enrollment-form-schema-api";

type FieldInputTranslator = (key: string) => string;
/**
 * Read-only information block (FormFieldInfo). Renders an optional heading +
 * plain-text body to parents; collects no answer. Newlines in the body are
 * preserved via `whitespace-pre-line`. Content is plain text by design (no
 * HTML/markdown), so it is safe to render directly.
 */
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

function HelpText({ text }: { readonly text?: string }) {
  if (!text) return null;
  return (
    <span className="-mt-1 mb-2 block text-xs leading-5 text-gray-500">
      {text}
    </span>
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

export function CustomFieldInput({
  field,
  inputName,
  value,
  onChange,
  error,
  tr,
  controlId: suppliedControlId,
  renderStructured,
}: CustomFieldInputProps) {
  if (!SIMPLE_TYPES.has(field.type)) return renderStructured?.() ?? null;
  const controlId = suppliedControlId ?? `enrollment-${inputName}`;
  const errorId = `${controlId}-error`;
  const valueStr = typeof value === "string" ? value : "";
  if (field.type === "boolean") {
    const selectedValue = value === true ? "yes" : value === false ? "no" : "";
    return (
      <fieldset
        aria-invalid={error ? "true" : undefined}
        aria-describedby={error ? errorId : undefined}
      >
        <legend className="mb-2 block text-sm font-medium text-gray-700">
          {field.label}
          {field.required && <RequiredMark />}
        </legend>
        <HelpText text={field.help_text} />
        <div className="grid grid-cols-2 gap-2">
          {[
            { label: tr("structured.yes"), value: "yes", raw: true },
            { label: tr("structured.no"), value: "no", raw: false },
          ].map((option) => (
            <ChoiceTile
              key={option.value}
              selected={selectedValue === option.value}
              className="h-10 justify-center py-0"
            >
              <Radio
                name={inputName}
                value={option.value}
                checked={selectedValue === option.value}
                onChange={() => onChange(option.raw)}
              />
              {option.label}
            </ChoiceTile>
          ))}
        </div>
        <FieldErrorHint id={errorId} message={error} />
      </fieldset>
    );
  }
  if (field.type === "textarea") {
    return (
      <div>
        <FieldLabel
          htmlFor={controlId}
          required={field.required}
          invalid={Boolean(error)}
        >
          {field.label}
        </FieldLabel>
        <HelpText text={field.help_text} />
        <Textarea
          id={controlId}
          name={inputName}
          value={valueStr}
          onChange={(e) => onChange(e.target.value)}
          rows={3}
          required={field.required}
          error={error}
        />
      </div>
    );
  }
  if (field.type === "select") {
    return (
      <div>
        <FieldLabel
          htmlFor={controlId}
          id={`${controlId}-label`}
          required={field.required}
          invalid={Boolean(error)}
        >
          {field.label}
        </FieldLabel>
        <HelpText text={field.help_text} />
        <CustomSelect
          ariaLabelledBy={`${controlId}-label`}
          id={controlId}
          value={valueStr}
          onChange={onChange}
          name={inputName}
          required={field.required}
          invalid={Boolean(error)}
          ariaDescribedBy={error ? errorId : undefined}
          options={[
            { value: "", label: tr("fields.choose") },
            ...(field.options ?? []).map((option) => ({
              value: option.value,
              label: option.label,
            })),
          ]}
        />
        <FieldErrorHint id={errorId} message={error} />
      </div>
    );
  }
  return (
    <div>
      <FieldLabel
        htmlFor={controlId}
        required={field.required}
        invalid={Boolean(error)}
      >
        {field.label}
      </FieldLabel>
      <HelpText text={field.help_text} />
      <Input
        id={controlId}
        name={inputName}
        type={field.type === "number" ? "number" : "text"}
        controlSize="compact"
        value={valueStr}
        onChange={(e) => onChange(e.target.value)}
        required={field.required}
        error={error}
      />
    </div>
  );
}
