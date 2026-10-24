"use client";

import { FileUp, Plus } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import type {
  FormField,
  FormFieldType,
} from "~/lib/enrollment-form-schema-api";
import {
  defaultFileFieldOptions,
  effectiveFileFieldOptions,
  ENROLLMENT_FILE_TYPES,
  FILE_TYPE_LABEL,
  MAX_FILES_PER_FIELD,
  MAX_UPLOAD_MB,
  MIN_UPLOAD_MB,
  type EnrollmentFileType,
  type FileFieldOptions,
} from "~/lib/enrollment-uploads";

/**
 * Editor pieces of the `file` field type (#4783): the options block, the
 * rules a file field follows on save, and the Einkommenserklärung preset.
 * Kept out of enrollment-form-editor.tsx, which is already very long.
 */

/**
 * The patch for a type switch. A file field only exists once per guardian
 * (never per child) and carries neither options nor validation; every other
 * type drops the file rules again.
 */
export function fieldTypeChangePatch(
  field: FormField,
  nextType: FormFieldType,
): Partial<FormField> {
  if (nextType === "file") {
    return {
      type: "file",
      applies_to_child: false,
      options: undefined,
      validation: undefined,
      file_options: field.file_options ?? defaultFileFieldOptions(),
    };
  }
  return { type: nextType, file_options: undefined };
}

/** The shape a field is saved in, with the backend's file rules applied. */
export function applyFileFieldRules(field: FormField): FormField {
  if (field.type !== "file") {
    if (field.file_options === undefined) return field;
    const { file_options: _dropped, ...rest } = field;
    return rest;
  }
  const { options: _options, validation: _validation, ...rest } = field;
  return {
    ...rest,
    applies_to_child: false,
    file_options: effectiveFileFieldOptions(field.file_options),
  };
}

/** A save-blocking problem of a file field, or null. */
export function fileFieldDraftProblem(
  field: FormField,
  index: number,
): { message: string; field: string; hint: string } | null {
  if (field.type !== "file") return null;
  if ((field.file_options?.allowed_types ?? []).length === 0) {
    return {
      message: `Bitte wählen Sie für Frage ${index + 1} mindestens eine Dateiart.`,
      field: `fields.${index}.file_options`,
      hint: "Bitte wählen Sie mindestens eine Dateiart.",
    };
  }
  return null;
}

const SIZE_OPTIONS = Array.from(
  { length: MAX_UPLOAD_MB - MIN_UPLOAD_MB + 1 },
  (_, i) => {
    const mb = MIN_UPLOAD_MB + i;
    return { value: String(mb), label: `${mb} MB` };
  },
);

/** The options block under a `file` question. */
export function FileFieldOptionsEditor({
  field,
  index,
  onChange,
  disabled,
  error,
}: Readonly<{
  field: FormField;
  index: number;
  onChange: (patch: Partial<FormField>) => void;
  disabled: boolean;
  error?: string;
}>) {
  const options: FileFieldOptions =
    field.file_options ?? defaultFileFieldOptions();
  const changeOptions = (patch: Partial<FileFieldOptions>) =>
    onChange({ file_options: { ...options, ...patch } });
  const toggleType = (type: EnrollmentFileType, checked: boolean) => {
    const next = new Set(options.allowed_types);
    if (checked) next.add(type);
    else next.delete(type);
    changeOptions({
      allowed_types: ENROLLMENT_FILE_TYPES.filter((t) => next.has(t)),
    });
  };
  const hintId = `fields-${index}-file_options-error`;
  const sizeLabelId = `field-${index}-file-size-label`;

  return (
    <div className="space-y-3 rounded-xl border border-gray-200 bg-gray-50/80 p-3">
      <fieldset
        aria-invalid={error ? "true" : undefined}
        aria-describedby={error ? hintId : undefined}
      >
        <legend className="text-xs font-medium text-gray-700">
          Erlaubte Dateiarten
        </legend>
        <div className="mt-1 grid grid-cols-3 gap-2">
          {ENROLLMENT_FILE_TYPES.map((type) => (
            <ChoiceTile
              key={type}
              selected={options.allowed_types.includes(type)}
              disabled={disabled}
              className="bg-white"
            >
              <Checkbox
                name={
                  type === "pdf" ? `fields.${index}.file_options` : undefined
                }
                checked={options.allowed_types.includes(type)}
                onChange={(event) => toggleType(type, event.target.checked)}
                disabled={disabled}
              />
              {FILE_TYPE_LABEL[type]}
            </ChoiceTile>
          ))}
        </div>
        <FieldErrorHint id={hintId} message={error} />
      </fieldset>

      <div className="grid gap-3 sm:grid-cols-2">
        <div>
          <span
            id={sizeLabelId}
            className="block text-xs font-medium text-gray-700"
          >
            Höchstens je Datei
          </span>
          <CustomSelect
            ariaLabelledBy={sizeLabelId}
            value={String(options.max_size_mb)}
            onChange={(value) => changeOptions({ max_size_mb: Number(value) })}
            disabled={disabled}
            className="mt-1"
            options={SIZE_OPTIONS}
          />
        </div>
        <ChoiceTile
          selected={options.multiple}
          disabled={disabled}
          className="items-start self-end bg-white"
        >
          <Checkbox
            checked={options.multiple}
            onChange={(event) =>
              changeOptions({ multiple: event.target.checked })
            }
            disabled={disabled}
          />
          <span className="min-w-0">
            <span className="block">Mehrere Dateien erlauben</span>
            <span className="mt-0.5 block text-xs font-normal text-gray-500">
              Eltern können bis zu {MAX_FILES_PER_FIELD} Dateien hochladen.
            </span>
          </span>
        </ChoiceTile>
      </div>
      <p className="text-xs leading-5 text-gray-500">
        Die Frage gilt für die ganze Anmeldung, nicht pro Kind. Wer die Dateien
        öffnen darf, regelt das Recht „Nachweise aus Anmeldungen ansehen“.
      </p>
    </div>
  );
}

/** What a file question looks like in the editor's live preview. */
export function FilePreviewInput({ field }: Readonly<{ field: FormField }>) {
  const options = effectiveFileFieldOptions(field.file_options);
  const types = options.allowed_types
    .map((type) => FILE_TYPE_LABEL[type])
    .join(", ");
  return (
    <div className="mt-3 flex items-center gap-2 rounded-lg border border-dashed border-gray-300 bg-gray-50 px-3 py-2 text-xs text-gray-500">
      <FileUp className="h-4 w-4 shrink-0 text-gray-400" aria-hidden="true" />
      <span className="min-w-0">
        {types || "Keine Dateiart gewählt"} · höchstens {options.max_size_mb} MB
        {options.multiple ? ` · bis zu ${MAX_FILES_PER_FIELD} Dateien` : ""}
      </span>
    </div>
  );
}

// Stable keys on purpose: they differ from the label-derived key, so a school
// that renames „Einkommensstufe“ keeps the key the Nachweis condition names.
const INCOME_LEVEL_KEY = "einkommen_stufe";
const INCOME_PROOF_KEY = "einkommen_nachweis";

/** The two questions of the Einkommenserklärung preset. */
export function createIncomeDeclarationFields(sortOrder: number): FormField[] {
  return [
    {
      key: INCOME_LEVEL_KEY,
      label: "Einkommensstufe",
      type: "select",
      required: true,
      help_text: "",
      options: [
        { label: "Voller Beitrag", value: "voll" },
        { label: "Ermäßigt Stufe 1", value: "stufe_1" },
        { label: "Ermäßigt Stufe 2", value: "stufe_2" },
        { label: "Beitragsfrei", value: "frei" },
      ],
      sort_order: sortOrder,
      applies_to_child: false,
    },
    {
      key: INCOME_PROOF_KEY,
      label: "Einkommensnachweis",
      type: "file",
      required: true,
      help_text:
        "Bitte laden Sie den aktuellen Einkommensnachweis hoch, zum Beispiel den Steuerbescheid.",
      file_options: {
        allowed_types: ["pdf", "jpg", "png"],
        max_size_mb: MAX_UPLOAD_MB,
        multiple: true,
      },
      sort_order: sortOrder + 1,
      applies_to_child: false,
      visible_when: {
        source: "field",
        field: INCOME_LEVEL_KEY,
        operator: "neq",
        value: "voll",
      },
    },
  ];
}

/** True when the preset is already in the form (its keys must stay unique). */
export function hasIncomeDeclaration(fields: readonly FormField[]): boolean {
  return fields.some(
    (field) => field.key === INCOME_LEVEL_KEY || field.key === INCOME_PROOF_KEY,
  );
}

/** Toolbar button that adds the Einkommenserklärung preset. */
export function IncomeDeclarationButton({
  fields,
  onAdd,
  disabled,
}: Readonly<{
  fields: readonly FormField[];
  onAdd: () => void;
  disabled: boolean;
}>) {
  const present = hasIncomeDeclaration(fields);
  return (
    <Button
      type="button"
      onClick={onAdd}
      disabled={disabled || present}
      title={
        present
          ? "Die Einkommenserklärung ist schon im Formular."
          : "Fügt die Frage nach der Einkommensstufe und das Hochladen des Nachweises hinzu."
      }
      variant="outline"
      size="md"
      className="inline-flex items-center justify-center gap-2"
    >
      <Plus className="h-4 w-4" aria-hidden="true" />
      Einkommenserklärung
    </Button>
  );
}
