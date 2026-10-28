// The school's own questions in the OGS-Anmeldung (enrollment-form.tsx): one
// renderer per form-template field type, plus the read-only information block.

import { AttachmentList } from "~/components/enrollment/attachment-list";
import { FileUploadInput } from "~/components/enrollment/enrollment-file-upload";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FieldLabel, RequiredMark } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { Radio } from "~/components/ui/radio";
import { Textarea } from "~/components/ui/textarea";
import { useLocalizedDatePicker } from "~/lib/hooks/use-localized-date-picker";

import {
  ContactListInput,
  PhoneListInput,
} from "./enrollment-form-contact-inputs";
import type {
  CustomFieldInputProps,
  FormField,
} from "./enrollment-form-shared";
import {
  WeekdayBooleanInput,
  WeekdayModeInput,
  WeekdayMultiModeInput,
  WeekdayScheduleInput,
} from "./enrollment-form-weekday-inputs";

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

export function CustomFieldInput({
  field,
  inputName,
  companionNoteName,
  value,
  onChange,
  error,
  tr,
  relevantDays,
  careConstrained,
  companionNote,
  onCompanionNoteChange,
  companionNoteError,
  singleMode,
}: CustomFieldInputProps) {
  const datePicker = useLocalizedDatePicker();
  const controlId = `enrollment-${inputName}`;
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
        <FieldLabel required={field.required} invalid={Boolean(error)}>
          {field.label}
        </FieldLabel>
        <HelpText text={field.help_text} />
        <CustomSelect
          ariaLabel={field.label}
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
  if (field.type === "phone_list") {
    return (
      <PhoneListInput
        field={field}
        inputName={inputName}
        value={value}
        onChange={onChange}
        error={error}
        tr={tr}
      />
    );
  }
  if (field.type === "weekday_schedule") {
    return (
      <WeekdayScheduleInput
        field={field}
        inputName={inputName}
        value={value}
        onChange={onChange}
        error={error}
        tr={tr}
        relevantDays={relevantDays}
        careConstrained={careConstrained}
      />
    );
  }
  if (field.type === "weekday_boolean") {
    return (
      <WeekdayBooleanInput
        field={field}
        inputName={inputName}
        value={value}
        onChange={onChange}
        error={error}
        tr={tr}
      />
    );
  }
  if (field.type === "weekday_mode") {
    return (
      <WeekdayModeInput
        field={field}
        inputName={inputName}
        value={value}
        onChange={onChange}
        error={error}
        tr={tr}
      />
    );
  }
  if (field.type === "weekday_multi_mode") {
    return (
      <WeekdayMultiModeInput
        field={field}
        inputName={inputName}
        value={value}
        onChange={onChange}
        error={error}
        tr={tr}
        relevantDays={relevantDays}
        companionNote={companionNote}
        onCompanionNoteChange={onCompanionNoteChange}
        companionNoteError={companionNoteError}
        companionNoteName={companionNoteName}
        singleMode={singleMode}
      />
    );
  }
  if (field.type === "file") {
    return (
      <FileUploadInput
        field={field}
        inputName={inputName}
        value={value}
        onChange={onChange}
        error={error}
        tr={tr}
      />
    );
  }
  if (field.type === "contact_list") {
    return (
      <ContactListInput
        field={field}
        inputName={inputName}
        value={value}
        onChange={onChange}
        error={error}
        tr={tr}
      />
    );
  }

  if (field.type === "date") {
    const dateId = controlId;
    return (
      <div>
        <FieldLabel
          htmlFor={dateId}
          required={field.required}
          invalid={Boolean(error)}
        >
          {field.label}
        </FieldLabel>
        <HelpText text={field.help_text} />
        <ISODatePicker
          {...datePicker}
          id={dateId}
          name={inputName}
          controlSize="md"
          value={valueStr}
          onChange={onChange}
          invalid={Boolean(error)}
          ariaDescribedBy={error ? errorId : undefined}
          calendarLayout="popover"
          // A school-defined date field can be anything from a birthday to a
          // future appointment, so the year dropdown stays available and no
          // bound is imposed.
          monthYearNavigation
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
