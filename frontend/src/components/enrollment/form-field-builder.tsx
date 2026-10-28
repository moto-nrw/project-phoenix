"use client";

// Gemeinsame Bausteine des Formular-Baukastens (#5198): eine Frage
// bearbeiten, ihre Sichtbarkeitsregel setzen und das Elternformular als
// Vorschau zeigen. Der Schul-Editor der Online-Anmeldung und das
// Anmeldeformular eines Ferienzeitraums im Träger-Portal nutzen dieselben
// Bausteine; welche Feldtypen und Bedingungsquellen eine Seite anbietet,
// bestimmt sie über Props.

import {
  createContext,
  useContext,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  Check,
  ChevronDown,
  Eye,
  GripVertical,
  HelpCircle,
  Info,
  Lock,
  Plus,
  Trash2,
} from "lucide-react";
import { AttachmentEditor } from "~/components/enrollment/attachment-editor";
import {
  FileFieldOptionsEditor,
  FilePreviewInput,
  fieldTypeChangePatch,
} from "~/components/enrollment/file-field-editor";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { EmptyState } from "~/components/ui/empty-state";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { Textarea } from "~/components/ui/textarea";
import {
  RESERVED_TARGETS,
  type ConditionOperator,
  type ConditionSource,
  type FormAttachment,
  type FormField,
  type FormFieldType,
  type VisibilityCondition,
} from "~/lib/enrollment-form-schema-api";
import { getStableObjectKey } from "~/lib/stable-object-key";

export const fieldTypeLabels: Record<FormFieldType, string> = {
  boolean: "Ja/Nein",
  number: "Zahl",
  text: "Text",
  textarea: "Mehrzeiliger Text",
  date: "Datum",
  select: "Auswahl",
  information: "Infotext",
  phone_list: "Telefonliste",
  weekday_schedule: "Wochenzeiten",
  weekday_boolean: "Wochentage",
  weekday_mode: "Geh- und Abholregelung",
  weekday_multi_mode: "Erlaubte Heimwege",
  contact_list: "Kontaktliste",
  file: "Datei",
};

// German labels for the visibility-condition editor.
const conditionSourceLabels: Record<ConditionSource, string> = {
  field: "Antwort auf eine andere Frage",
  grade_level: "Klassenstufe des Kindes",
  care_offering: "Gewähltes Betreuungsangebot",
};

const conditionOperatorLabels: Record<ConditionOperator, string> = {
  eq: "ist gleich",
  neq: "ist nicht gleich",
  not_empty: "ist ausgefüllt",
  includes: "enthält",
};

const freeFieldTypes = [
  "boolean",
  "number",
  "text",
  "textarea",
  "date",
  "select",
  "file",
] satisfies FormFieldType[];

/**
 * The hint the save path marks a control with, by the control's `name`
 * (#2515). The names follow the request body (`name`, `fields.2.label`,
 * `legal_blocks.0.title`), so a field error from the API and a local check
 * mark the same control.
 */
export const FieldErrorContext = createContext<
  (name: string) => string | undefined
>(() => undefined);

/** `name`, `aria-invalid` and `aria-describedby` for a marked control. */
export function fieldErrorProps(
  fieldError: (name: string) => string | undefined,
  name: string,
) {
  const message = fieldError(name);
  const hintId = `${name.replaceAll(".", "-")}-error`;
  return {
    message,
    hintId,
    controlProps: {
      name,
      "aria-invalid": message ? true : undefined,
      "aria-describedby": message ? hintId : undefined,
    },
  } as const;
}

// Snake-case-ify a free-form label into a schema-safe field key.
//
// Implementation is character-loop based on purpose: the regex form
// would need `+` / `*` quantifiers on character classes, which are the
// shape static analysis flags as potentially super-linear under
// backtracking. The patterns here aren't actually catastrophic (single
// char classes, no overlapping alternation), but the loop has the same
// O(n) cost and sidesteps the question. Inputs are also capped \u2014 admin
// labels never legitimately exceed a few hundred chars.
export function normalizeFieldKey(value: string): string {
  const MAX_INPUT = 256;
  const source = value
    .slice(0, MAX_INPUT)
    .trim()
    .toLowerCase()
    .normalize("NFD");

  let out = "";
  let pendingUnderscore = false;
  for (let i = 0; i < source.length; i++) {
    const code = source.charCodeAt(i);
    // Combining diacritical marks introduced by NFD \u2014 skip.
    if (code >= 0x0300 && code <= 0x036f) continue;
    const isLower = code >= 0x61 && code <= 0x7a; // a-z
    const isDigit = code >= 0x30 && code <= 0x39; // 0-9
    if (isLower || isDigit) {
      if (pendingUnderscore && out.length > 0) {
        out += "_";
      }
      out += source[i];
      pendingUnderscore = false;
    } else {
      // Any other character \u2192 collapse into a single underscore
      // (deferred so we don't leave a trailing one).
      pendingUnderscore = true;
    }
  }
  return out;
}

function getRequiredHint(field: FormField): string {
  if (field.type === "boolean") {
    return "Eltern müssen Ja oder Nein auswählen.";
  }
  if (field.type === "weekday_schedule") {
    return "Eltern müssen für jeden Betreuungstag eine Uhrzeit angeben (ohne Betreuungsangebote: mindestens eine).";
  }
  if (field.type === "weekday_boolean") {
    // Legacy pickup (student.pickup_status) accepts an empty selection ("geht
    // alleine nach Hause") -- required only forces parents to confirm the
    // field once, not to tick a day. Every other weekday_boolean (Buskind)
    // genuinely needs at least one day.
    if (field.target === "student.pickup_status") {
      return "Eltern müssen die Abholregelung bestätigen (Tage auswählen oder leer lassen).";
    }
    return "Eltern müssen mindestens einen Wochentag auswählen.";
  }
  if (field.type === "weekday_multi_mode") {
    return "Eltern müssen pro Betreuungstag mindestens einen Heimweg auswählen.";
  }
  if (field.type === "contact_list") {
    return "Eltern müssen mindestens einen vollständigen Kontakt angeben.";
  }
  if (field.type === "phone_list") {
    return "Eltern müssen mindestens eine Telefonnummer angeben.";
  }
  if (field.type === "file") {
    return "Eltern müssen mindestens eine Datei hochladen.";
  }
  return "Eltern müssen diese Frage beantworten.";
}

interface FieldEditorRowProps {
  readonly field: FormField;
  readonly index: number;
  readonly total: number;
  readonly allFields: FormField[];
  readonly onChange: (patch: Partial<FormField>) => void;
  readonly onRemove: () => void;
  readonly onMoveUp: () => void;
  readonly onMoveDown: () => void;
  readonly disabled: boolean;
  /** The types a free question may take; the online registration's by default. */
  readonly fieldTypes?: readonly FormFieldType[];
  /** The sources a visibility condition may use; all by default. */
  readonly conditionSources?: readonly ConditionSource[];
  /** Whether an information block may carry PDF attachments. */
  readonly allowAttachments?: boolean;
  /** The explanation of a free question under its heading. */
  readonly freeQuestionNote?: { readonly title: string; readonly text: string };
  /**
   * The switch „Gesundheitsangabe“ of the holiday registration (#5198). The
   * marker is not part of a school form field, so the page owns its value.
   */
  readonly healthToggle?: {
    readonly checked: boolean;
    readonly onChange: (checked: boolean) => void;
  };
}

const ALL_CONDITION_SOURCES: readonly ConditionSource[] = [
  "field",
  "grade_level",
  "care_offering",
];

const SCHOOL_FREE_QUESTION_NOTE = {
  title: "Bleibt nur als Zusatzangabe bei der Anmeldung.",
  text: "Für Angaben, die nicht automatisch in die Stammdaten übernommen werden sollen.",
} as const;

interface AllowedTimeRow {
  readonly id: string;
  readonly value: string;
}

function createAllowedTimeRows(
  values: readonly string[],
  idPrefix: string,
  sequence: { current: number },
): AllowedTimeRow[] {
  return values.map((value) => {
    sequence.current += 1;
    return { id: `${idPrefix}-${sequence.current}`, value };
  });
}

export function FieldEditorRow({
  field,
  index,
  total,
  allFields,
  onChange,
  onRemove,
  onMoveUp,
  onMoveDown,
  disabled,
  fieldTypes = freeFieldTypes,
  conditionSources = ALL_CONDITION_SOURCES,
  allowAttachments = true,
  freeQuestionNote = SCHOOL_FREE_QUESTION_NOTE,
  healthToggle,
}: FieldEditorRowProps) {
  const fieldError = useContext(FieldErrorContext);
  // The key is derived from the question text, so a problem with the key
  // marks the text field.
  const labelError = fieldErrorProps(
    (name) => fieldError(name) ?? fieldError(`fields.${index}.key`),
    `fields.${index}.label`,
  );
  const contentError = fieldErrorProps(fieldError, `fields.${index}.content`);
  const optionsError = fieldErrorProps(fieldError, `fields.${index}.options`);
  const allowedTimeIdPrefix = useId();
  const allowedTimeSequence = useRef(0);
  const target = field.target || null;
  const isTargetField = target !== null;
  const isInfo = field.type === "information";
  const displayLabel = isInfo
    ? field.label.trim() || "Infotext"
    : target
      ? field.label.trim() || RESERVED_TARGETS[target].label
      : field.label.trim() || "Neue Zusatzfrage";
  const optionSignature = useMemo(
    () => (field.options ?? []).map((option) => option.label).join("\n"),
    [field.options],
  );
  const [optionsDraft, setOptionsDraft] = useState(optionSignature);

  useEffect(() => {
    setOptionsDraft(optionSignature);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [field.key, field.type]);

  const updateOptions = (value: string) => {
    setOptionsDraft(value);
    const options = value
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => ({
        label: line,
        value: normalizeFieldKey(line),
      }));
    onChange({ options });
  };

  // Local editing rows for the fixed pickup times: each row is one HH:MM
  // time picker. Kept separate from field.allowed_times so a half-filled
  // (empty) row can stay on screen while the admin picks a time; only the
  // trimmed, de-duplicated, non-empty values are pushed up to the field.
  const [allowedTimesRows, setAllowedTimesRows] = useState<AllowedTimeRow[]>(
    () =>
      createAllowedTimeRows(
        field.allowed_times ?? [],
        allowedTimeIdPrefix,
        allowedTimeSequence,
      ),
  );
  // Signature of the allowed_times we last pushed up via onChange. Used to
  // tell an *external* change (a schema/template switch reusing this row at
  // the same index — the row is keyed by index, not field identity, so the
  // component instance is not remounted) from a *self-induced* one (our own
  // commit echoing back through field.allowed_times). Only the former resets
  // the local rows; the latter must not, or a half-filled empty row would be
  // wiped on every keystroke.
  const lastCommittedSig = useRef<string>(
    (field.allowed_times ?? []).join("\n"),
  );

  useEffect(() => {
    const incoming = field.allowed_times ?? [];
    const incomingSig = incoming.join("\n");
    if (incomingSig !== lastCommittedSig.current) {
      lastCommittedSig.current = incomingSig;
      setAllowedTimesRows(
        createAllowedTimeRows(
          incoming,
          allowedTimeIdPrefix,
          allowedTimeSequence,
        ),
      );
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [field.key, field.type, field.allowed_times]);

  const commitAllowedTimes = (rows: AllowedTimeRow[]) => {
    setAllowedTimesRows(rows);
    const allowed_times = Array.from(
      new Set(rows.map((row) => row.value.trim()).filter(Boolean)),
    );
    lastCommittedSig.current = allowed_times.join("\n");
    onChange({ allowed_times });
  };

  const updateAllowedTime = (index: number, value: string) => {
    const rows = [...allowedTimesRows];
    const row = rows[index];
    if (!row) return;
    rows[index] = { ...row, value };
    commitAllowedTimes(rows);
  };

  const addAllowedTime = () => {
    const [row] = createAllowedTimeRows(
      [""],
      allowedTimeIdPrefix,
      allowedTimeSequence,
    );
    if (row) commitAllowedTimes([...allowedTimesRows, row]);
  };

  const removeAllowedTime = (index: number) =>
    commitAllowedTimes(allowedTimesRows.filter((_, i) => i !== index));

  // Draft text for the Heimweg-Beschränkung grade list (#2381). Kept as a
  // string so a trailing comma or space survives while typing; only the
  // parsed, deduplicated grades are pushed up to the field.
  const [singleModeDraft, setSingleModeDraft] = useState(() =>
    (field.single_mode_grades ?? []).join(", "),
  );

  useEffect(() => {
    setSingleModeDraft((field.single_mode_grades ?? []).join(", "));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [field.key, field.type]);

  const updateSingleModeGrades = (value: string) => {
    setSingleModeDraft(value);
    const grades = Array.from(
      new Set(
        value
          .split(/[,;\s]+/)
          .map((part) => Number(part))
          .filter((n) => Number.isInteger(n) && n >= 1 && n <= 13),
      ),
    );
    onChange({ single_mode_grades: grades });
  };

  return (
    <article className="moto-content-surface rounded-2xl border p-4 shadow-sm">
      <div className="flex items-start gap-3">
        <span className="mt-1 hidden text-gray-300 sm:block">
          <GripVertical className="h-5 w-5" aria-hidden="true" />
        </span>
        <div className="min-w-0 flex-1 space-y-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <p className="text-sm font-semibold text-gray-900">
                  {isInfo ? "Infotext" : "Frage"} {index + 1}
                </p>
                {isInfo ? (
                  <span className="bg-moto-blue/10 text-moto-blue-hover inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-sm font-medium">
                    <Info className="h-3 w-3" aria-hidden="true" />
                    Hinweis
                  </span>
                ) : isTargetField ? (
                  <span className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-sm font-medium text-gray-600">
                    <Lock className="h-3 w-3" aria-hidden="true" />
                    Fester Vorschlag
                  </span>
                ) : (
                  <span className="rounded-full bg-gray-100 px-2 py-0.5 text-sm font-medium text-gray-600">
                    Freie Zusatzfrage
                  </span>
                )}
              </div>
              <h3 className="mt-1 text-sm font-semibold text-gray-900">
                {displayLabel}
              </h3>
            </div>
            <div className="flex gap-1.5">
              <button
                type="button"
                onClick={onMoveUp}
                disabled={disabled || index === 0}
                className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-gray-200 bg-white text-sm font-medium text-gray-500 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
                aria-label="Frage nach oben verschieben"
              >
                ↑
              </button>
              <button
                type="button"
                onClick={onMoveDown}
                disabled={disabled || index === total - 1}
                className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-gray-200 bg-white text-sm font-medium text-gray-500 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
                aria-label="Frage nach unten verschieben"
              >
                ↓
              </button>
              <button
                type="button"
                onClick={onRemove}
                disabled={disabled}
                className="hover:border-moto-red/30 hover:bg-moto-red/10 hover:text-moto-red-strong inline-flex h-8 w-8 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-500 shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
                aria-label="Frage entfernen"
              >
                <Trash2 className="h-4 w-4" aria-hidden="true" />
              </button>
            </div>
          </div>

          {isInfo ? (
            <div className="border-moto-blue/20 bg-moto-blue/5 rounded-xl border px-3 py-2 text-xs leading-5 text-gray-600">
              <p className="text-moto-blue-hover font-medium">
                Wird Eltern als Hinweis angezeigt.
              </p>
              <p className="mt-0.5">
                Ein Infotext sammelt keine Antwort. Nutzen Sie ihn für
                Erklärungen, Hinweise oder Zwischenüberschriften.
              </p>
            </div>
          ) : isTargetField ? (
            <div className="rounded-xl border border-gray-200 bg-gray-50/80 px-3 py-2 text-xs leading-5 text-gray-600">
              <p className="font-medium text-gray-800">
                Wird bei bestätigter Anmeldung in die Stammdaten übernommen.
              </p>
              <p className="mt-0.5">
                Sie können die angezeigte Frage umbenennen. Inhalt und Typ sind
                fest vorgegeben. Entfernen Sie den Vorschlag, wenn diese Angabe
                nicht abgefragt werden soll.
              </p>
            </div>
          ) : (
            <div className="rounded-xl border border-gray-200 bg-gray-50/80 px-3 py-2 text-xs leading-5 text-gray-600">
              <p className="font-medium text-gray-800">
                {freeQuestionNote.title}
              </p>
              <p className="mt-0.5">{freeQuestionNote.text}</p>
            </div>
          )}

          {isInfo ? (
            <>
              <div>
                <label className="block">
                  <span className="text-xs font-medium text-gray-700">
                    Titel (optional)
                  </span>
                  <input
                    type="text"
                    value={field.label}
                    onChange={(event) =>
                      onChange({ label: event.target.value })
                    }
                    placeholder="z. B. Wichtiger Hinweis"
                    disabled={disabled}
                    {...labelError.controlProps}
                    className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  />
                </label>
                <FieldErrorHint
                  id={labelError.hintId}
                  message={labelError.message}
                />
              </div>
              <div>
                <Textarea
                  id={`field-info-content-${index}`}
                  name={contentError.controlProps.name}
                  error={contentError.message}
                  label="Infotext für Eltern"
                  value={field.content ?? ""}
                  onChange={(event) =>
                    onChange({ content: event.target.value })
                  }
                  placeholder="Dieser Text wird Eltern im Formular angezeigt."
                  disabled={disabled}
                  rows={3}
                />
              </div>
              {allowAttachments ? (
                <AttachmentEditor
                  fieldPath={`fields.${index}.attachments`}
                  fieldError={fieldError}
                  attachments={field.attachments}
                  onChange={(attachments) => onChange({ attachments })}
                  disabled={disabled}
                />
              ) : null}
              <div className="grid gap-2 sm:grid-cols-2">
                <FormChoice
                  checked={Boolean(field.applies_to_child)}
                  onChange={(checked) =>
                    onChange({ applies_to_child: checked })
                  }
                  label="Pro Kind anzeigen"
                  hint="Der Hinweis erscheint bei jedem angemeldeten Kind."
                  disabled={disabled}
                />
              </div>
            </>
          ) : (
            <>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label className="block">
                    <span className="text-xs font-medium text-gray-700">
                      Frage im Elternformular
                    </span>
                    <input
                      type="text"
                      value={field.label}
                      onChange={(event) => {
                        const nextLabel = event.target.value;
                        if (isTargetField) {
                          // Suggested fields keep their fixed key + type; only
                          // the displayed question text is editable.
                          onChange({ label: nextLabel });
                          return;
                        }
                        const currentAutoKey = normalizeFieldKey(field.label);
                        const shouldUpdateKey =
                          field.key.trim() === "" ||
                          field.key === currentAutoKey;
                        onChange({
                          label: nextLabel,
                          key: shouldUpdateKey
                            ? normalizeFieldKey(nextLabel)
                            : field.key,
                        });
                      }}
                      placeholder={
                        isTargetField && target
                          ? RESERVED_TARGETS[target].label
                          : "z. B. Wie haben Sie von uns erfahren?"
                      }
                      disabled={disabled}
                      {...labelError.controlProps}
                      className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                    />
                  </label>
                  <FieldErrorHint
                    id={labelError.hintId}
                    message={labelError.message}
                  />
                </div>
                <label className="block">
                  <span className="text-xs font-medium text-gray-700">
                    Typ
                    {isTargetField ? (
                      <span className="ml-1 text-sm font-normal text-gray-500">
                        (automatisch festgelegt)
                      </span>
                    ) : null}
                  </span>
                  <CustomSelect
                    ariaLabel="Typ"
                    value={field.type}
                    onChange={(value) =>
                      onChange(
                        fieldTypeChangePatch(field, value as FormFieldType),
                      )
                    }
                    disabled={disabled || isTargetField}
                    className="mt-1"
                    options={(isTargetField ? [field.type] : fieldTypes).map(
                      (value) => ({
                        value,
                        label: fieldTypeLabels[value],
                      }),
                    )}
                  />
                </label>
              </div>

              <label className="block">
                <span className="text-xs font-medium text-gray-700">
                  Hilfetext für Eltern
                </span>
                <input
                  type="text"
                  value={field.help_text ?? ""}
                  onChange={(event) =>
                    onChange({ help_text: event.target.value })
                  }
                  placeholder="Optionaler kurzer Hinweis unter der Frage"
                  disabled={disabled}
                  className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                />
              </label>

              {field.type === "select" ? (
                <div>
                  <Textarea
                    id={`field-options-${index}`}
                    name={optionsError.controlProps.name}
                    error={optionsError.message}
                    label="Auswahloptionen"
                    value={optionsDraft}
                    onChange={(event) => updateOptions(event.target.value)}
                    placeholder={"Eine Option pro Zeile\nz. B. Ja\nz. B. Nein"}
                    disabled={disabled || isTargetField}
                    rows={3}
                  />
                </div>
              ) : null}

              {field.type === "file" ? (
                <FileFieldOptionsEditor
                  field={field}
                  index={index}
                  onChange={onChange}
                  disabled={disabled}
                  error={fieldError(`fields.${index}.file_options`)}
                />
              ) : null}

              {field.target === "schedule.pickup" ? (
                <div>
                  <span className="text-xs font-medium text-gray-700">
                    Feste Auswahlzeiten (optional)
                  </span>
                  <p className="mt-1 text-xs leading-5 text-gray-500">
                    Ohne Zeiten geben Eltern die Uhrzeit frei ein. Sobald Zeiten
                    hinterlegt sind, wählen Eltern pro Wochentag nur aus dieser
                    Liste.
                  </p>
                  <Button
                    type="button"
                    onClick={addAllowedTime}
                    disabled={disabled}
                    variant="outline"
                    size="md"
                    className="mt-2 inline-flex items-center justify-center gap-2"
                  >
                    <Plus className="h-4 w-4" aria-hidden="true" />
                    Zeit hinzufügen
                  </Button>
                  {allowedTimesRows.length > 0 ? (
                    <ul className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2">
                      {allowedTimesRows.map((row, index) => (
                        <li key={row.id} className="flex items-center gap-2">
                          <input
                            type="time"
                            value={row.value}
                            onChange={(event) =>
                              updateAllowedTime(index, event.target.value)
                            }
                            disabled={disabled}
                            aria-label={`Auswahlzeit ${index + 1}`}
                            className="h-10 w-full min-w-0 flex-1 rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                          />
                          <OverflowMenu
                            ariaLabel={`Aktionen für Auswahlzeit ${index + 1}`}
                            triggerSize="sm"
                            items={[
                              {
                                label: "Auswahlzeit entfernen",
                                icon: (
                                  <Trash2
                                    className="h-4 w-4"
                                    aria-hidden="true"
                                  />
                                ),
                                onClick: () => removeAllowedTime(index),
                                destructive: true,
                                disabled,
                              },
                            ]}
                          />
                        </li>
                      ))}
                    </ul>
                  ) : null}
                </div>
              ) : null}

              {field.target === "student.allowed_departure_modes" ? (
                <div>
                  <label
                    className="block"
                    htmlFor={`${allowedTimeIdPrefix}-single-mode-grades`}
                  >
                    <span className="text-xs font-medium text-gray-700">
                      Nur ein Heimweg pro Wochentag (optional)
                    </span>
                    <p className="mt-1 text-xs leading-5 text-gray-500">
                      Tragen Sie Jahrgänge ein, deren Eltern pro Wochentag nur
                      einen Heimweg auswählen dürfen, zum Beispiel „1“ für die
                      Erstklässler. Ohne Eintrag bleibt die Mehrfachauswahl für
                      alle Jahrgänge erlaubt. Voraussetzung: Die Abfrage der
                      Klassenstufe ist in den Einstellungen aktiv.
                    </p>
                    <input
                      id={`${allowedTimeIdPrefix}-single-mode-grades`}
                      type="text"
                      inputMode="numeric"
                      value={singleModeDraft}
                      onChange={(event) =>
                        updateSingleModeGrades(event.target.value)
                      }
                      placeholder="z. B. 1 oder 1, 2"
                      disabled={disabled}
                      className="mt-2 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                    />
                  </label>
                </div>
              ) : null}

              <div className="grid gap-2 sm:grid-cols-2">
                <FormChoice
                  checked={Boolean(field.required)}
                  onChange={(checked) => onChange({ required: checked })}
                  label="Pflichtfrage"
                  hint={getRequiredHint(field)}
                  disabled={disabled}
                />
                <FormChoice
                  checked={Boolean(field.applies_to_child)}
                  onChange={(checked) =>
                    onChange({ applies_to_child: checked })
                  }
                  label="Pro Kind abfragen"
                  hint={
                    field.type === "file"
                      ? "Dateien gelten immer für die ganze Anmeldung."
                      : "Für Fragen zum Kind, etwa zu Allergien oder zum Schwimmen. Die Frage erscheint dann für jedes angemeldete Kind."
                  }
                  disabled={disabled || isTargetField || field.type === "file"}
                />
                {healthToggle ? (
                  <FormChoice
                    checked={healthToggle.checked}
                    onChange={healthToggle.onChange}
                    label="Gesundheitsangabe"
                    hint="Für Angaben zur Gesundheit. Eltern stimmen dann der Verarbeitung zu. Die Antwort steht nur in der Anmeldung, nicht in Liste und Export."
                    disabled={disabled}
                  />
                ) : null}
              </div>
            </>
          )}

          <ConditionEditor
            field={field}
            index={index}
            allFields={allFields}
            onChange={onChange}
            disabled={disabled}
            sources={conditionSources}
          />
        </div>
      </div>
    </article>
  );
}

function FormChoice({
  checked,
  onChange,
  label,
  hint,
  disabled,
}: Readonly<{
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  hint: string;
  disabled: boolean;
}>) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      onClick={() => onChange(!checked)}
      disabled={disabled}
      className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors ${
        checked
          ? "border-moto-green/40 bg-moto-green/10"
          : "border-gray-200 bg-white hover:bg-gray-50"
      } ${disabled ? "cursor-not-allowed opacity-60" : ""}`}
    >
      <span
        className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-md border ${
          checked
            ? "border-moto-green-fill bg-moto-green-fill text-white"
            : "border-gray-300 bg-white"
        }`}
        aria-hidden="true"
      >
        {checked ? <Check className="h-3.5 w-3.5" /> : null}
      </span>
      <span className="min-w-0">
        <span className="block text-sm font-medium text-gray-900">{label}</span>
        <span className="mt-0.5 block text-xs text-gray-500">{hint}</span>
      </span>
    </button>
  );
}

const conditionInputClass =
  "moto-content-surface mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:bg-gray-100 disabled:text-gray-600";

// Field-source operators offered in the condition editor. "includes" is
// reserved for the care-offering source and is not listed here.
const FIELD_CONDITION_OPERATORS: ConditionOperator[] = [
  "eq",
  "neq",
  "not_empty",
];

function defaultConditionForController(
  controller: FormField,
): VisibilityCondition {
  if (controller.type === "select") {
    return {
      source: "field",
      field: controller.key,
      operator: "eq",
      value: controller.options?.[0]?.value ?? "",
    };
  }
  return {
    source: "field",
    field: controller.key,
    operator: "eq",
    value: true,
  };
}

function defaultConditionForSource(
  source: ConditionSource,
  controllers: FormField[],
): VisibilityCondition {
  if (source === "grade_level") {
    return { source: "grade_level", operator: "eq", value: 1 };
  }
  if (source === "care_offering") {
    return { source: "care_offering", operator: "includes", value: "" };
  }
  const controller = controllers[0];
  return controller
    ? defaultConditionForController(controller)
    : { source: "field", field: "", operator: "eq", value: "" };
}

/**
 * Per-field visibility editor. Lets an admin gate a question or info
 * block behind another answer, the child's grade level, or a chosen
 * care offering. Eligible controlling questions are limited to yes/no
 * and selection fields observable from this field's scope.
 */
function ConditionEditor({
  field,
  index,
  allFields,
  onChange,
  disabled,
  sources: allowedSources = ALL_CONDITION_SOURCES,
}: Readonly<{
  field: FormField;
  index: number;
  allFields: FormField[];
  onChange: (patch: Partial<FormField>) => void;
  disabled: boolean;
  /** The sources this page offers; all by default. */
  sources?: readonly ConditionSource[];
}>) {
  const condition = field.visible_when ?? null;

  const controllers = allFields.filter(
    (f, i) =>
      i !== index &&
      f.key.trim() !== "" &&
      (f.type === "boolean" || f.type === "select") &&
      (field.applies_to_child || !f.applies_to_child),
  );

  const sources: ConditionSource[] = [];
  if (controllers.length > 0 && allowedSources.includes("field")) {
    sources.push("field");
  }
  const childSources = allowedSources.filter(
    (source) => source === "grade_level" || source === "care_offering",
  );
  if (field.applies_to_child) sources.push(...childSources);
  const canEnable = sources.length > 0;

  const setCondition = (next: VisibilityCondition | null) =>
    onChange({ visible_when: next });
  const patchCondition = (patch: Partial<VisibilityCondition>) => {
    if (condition) setCondition({ ...condition, ...patch });
  };

  if (!canEnable && !condition) {
    return (
      <div className="rounded-lg border border-dashed border-gray-200 bg-gray-50/60 px-3 py-2">
        <EmptyState
          variant="compact"
          title="Keine Sichtbarkeitsregel möglich"
          description={`Eine Sichtbarkeitsregel ist möglich, sobald es eine Ja/Nein- oder Auswahlfrage gibt${
            field.applies_to_child || childSources.length === 0
              ? "."
              : " (oder dieses Feld pro Kind angezeigt wird)."
          }`}
        />
      </div>
    );
  }

  return (
    <div className="moto-content-surface rounded-xl border p-3 shadow-sm">
      <FormChoice
        checked={condition !== null}
        onChange={(checked) =>
          setCondition(
            checked
              ? defaultConditionForSource(sources[0]!, controllers)
              : null,
          )
        }
        label="Nur unter einer Bedingung anzeigen"
        hint="Das Feld erscheint nur, wenn die Bedingung erfüllt ist."
        disabled={disabled || !canEnable}
      />

      {condition ? (
        <div className="mt-3 space-y-2">
          <label
            className="block"
            htmlFor={`condition-${index}-source`}
            id={`condition-${index}-source-label`}
          >
            <span className="text-xs font-medium text-gray-700">
              Sichtbar wenn
            </span>
            <CustomSelect
              ariaLabelledBy={`condition-${index}-source-label`}
              id={`condition-${index}-source`}
              value={condition.source}
              onChange={(value) =>
                setCondition(
                  defaultConditionForSource(
                    value as ConditionSource,
                    controllers,
                  ),
                )
              }
              disabled={disabled}
              className={conditionInputClass}
              options={sources.map((source) => ({
                value: source,
                label: conditionSourceLabels[source],
              }))}
            />
          </label>

          {condition.source === "field" ? (
            <ConditionFieldControls
              condition={condition}
              controllers={controllers}
              onPatch={patchCondition}
              onReplace={setCondition}
              disabled={disabled}
              idPrefix={`condition-${index}-field`}
            />
          ) : condition.source === "grade_level" ? (
            <ConditionGradeControls
              condition={condition}
              onPatch={patchCondition}
              disabled={disabled}
              idPrefix={`condition-${index}-grade`}
            />
          ) : (
            <ConditionOfferingControls
              condition={condition}
              onPatch={patchCondition}
              disabled={disabled}
            />
          )}
        </div>
      ) : null}
    </div>
  );
}

function ConditionFieldControls({
  condition,
  controllers,
  onPatch,
  onReplace,
  disabled,
  idPrefix,
}: Readonly<{
  condition: VisibilityCondition;
  controllers: FormField[];
  onPatch: (patch: Partial<VisibilityCondition>) => void;
  onReplace: (next: VisibilityCondition) => void;
  disabled: boolean;
  idPrefix: string;
}>) {
  const controller = controllers.find((c) => c.key === condition.field);

  const changeController = (key: string) => {
    const next = controllers.find((c) => c.key === key);
    if (!next) return;
    const base = defaultConditionForController(next);
    onReplace(
      condition.operator === "not_empty"
        ? { ...base, operator: "not_empty", value: undefined }
        : base,
    );
  };

  const changeOperator = (operator: ConditionOperator) => {
    if (operator === "not_empty") {
      onPatch({ operator, value: undefined });
      return;
    }
    const fallback = controller
      ? defaultConditionForController(controller).value
      : "";
    onPatch({ operator, value: condition.value ?? fallback });
  };

  return (
    <>
      <label
        className="block"
        htmlFor={`${idPrefix}-question`}
        id={`${idPrefix}-question-label`}
      >
        <span className="text-xs font-medium text-gray-700">Frage</span>
        <CustomSelect
          ariaLabelledBy={`${idPrefix}-question-label`}
          id={`${idPrefix}-question`}
          value={condition.field ?? ""}
          onChange={changeController}
          disabled={disabled}
          className={conditionInputClass}
          options={controllers.map((controller) => ({
            value: controller.key,
            label: controller.label.trim() || controller.key,
          }))}
        />
      </label>

      <label
        className="block"
        htmlFor={`${idPrefix}-operator`}
        id={`${idPrefix}-operator-label`}
      >
        <span className="text-xs font-medium text-gray-700">Vergleich</span>
        <CustomSelect
          ariaLabelledBy={`${idPrefix}-operator-label`}
          id={`${idPrefix}-operator`}
          value={condition.operator}
          onChange={(value) => changeOperator(value as ConditionOperator)}
          disabled={disabled}
          className={conditionInputClass}
          options={FIELD_CONDITION_OPERATORS.map((operator) => ({
            value: operator,
            label: conditionOperatorLabels[operator],
          }))}
        />
      </label>

      {condition.operator !== "not_empty" && controller ? (
        <label
          className="block"
          htmlFor={`${idPrefix}-value`}
          id={`${idPrefix}-value-label`}
        >
          <span className="text-xs font-medium text-gray-700">Wert</span>
          {controller.type === "boolean" ? (
            <CustomSelect
              ariaLabelledBy={`${idPrefix}-value-label`}
              id={`${idPrefix}-value`}
              value={condition.value === true ? "true" : "false"}
              onChange={(value) => onPatch({ value: value === "true" })}
              disabled={disabled}
              className={conditionInputClass}
              options={[
                { value: "true", label: "Ja" },
                { value: "false", label: "Nein" },
              ]}
            />
          ) : (
            <CustomSelect
              ariaLabelledBy={`${idPrefix}-value-label`}
              id={`${idPrefix}-value`}
              value={String(condition.value ?? "")}
              onChange={(value) => onPatch({ value })}
              disabled={disabled}
              className={conditionInputClass}
              options={(controller.options ?? []).map((option) => ({
                value: option.value,
                label: option.label,
              }))}
            />
          )}
        </label>
      ) : null}
    </>
  );
}

function ConditionGradeControls({
  condition,
  onPatch,
  disabled,
  idPrefix,
}: Readonly<{
  condition: VisibilityCondition;
  onPatch: (patch: Partial<VisibilityCondition>) => void;
  disabled: boolean;
  idPrefix: string;
}>) {
  return (
    <>
      <label
        className="block"
        htmlFor={`${idPrefix}-operator`}
        id={`${idPrefix}-operator-label`}
      >
        <span className="text-xs font-medium text-gray-700">Vergleich</span>
        <CustomSelect
          ariaLabelledBy={`${idPrefix}-operator-label`}
          id={`${idPrefix}-operator`}
          value={condition.operator}
          onChange={(value) =>
            onPatch({ operator: value as ConditionOperator })
          }
          disabled={disabled}
          className={conditionInputClass}
          options={[
            { value: "eq", label: conditionOperatorLabels.eq },
            { value: "neq", label: conditionOperatorLabels.neq },
          ]}
        />
      </label>
      <label className="block">
        <span className="text-xs font-medium text-gray-700">Klassenstufe</span>
        <input
          type="number"
          min={1}
          value={String(condition.value ?? "")}
          onChange={(event) =>
            onPatch({
              value:
                event.target.value === "" ? "" : Number(event.target.value),
            })
          }
          disabled={disabled}
          className={conditionInputClass}
        />
      </label>
    </>
  );
}

function ConditionOfferingControls({
  condition,
  onPatch,
  disabled,
}: Readonly<{
  condition: VisibilityCondition;
  onPatch: (patch: Partial<VisibilityCondition>) => void;
  disabled: boolean;
}>) {
  return (
    <label className="block">
      <span className="text-xs font-medium text-gray-700">
        Angebotsname (enthält)
      </span>
      <input
        type="text"
        value={String(condition.value ?? "")}
        onChange={(event) => onPatch({ value: event.target.value })}
        placeholder="z. B. Mittagessen"
        disabled={disabled}
        className={conditionInputClass}
      />
      <span className="mt-1 block text-sm leading-5 text-gray-500">
        Sichtbar, wenn ein gewähltes Betreuungsangebot diesen Namen trägt.
      </span>
    </label>
  );
}

export function PreviewSection({
  title,
  fields,
}: Readonly<{ title: string; fields: string[] }>) {
  return (
    <section>
      <h4 className="text-sm font-semibold text-gray-900">{title}</h4>
      <div className="mt-3 grid gap-2">
        {fields.map((field) => (
          <div
            key={field}
            className="rounded-lg border border-gray-200 bg-white px-3 py-2"
          >
            <span className="block text-sm font-medium text-gray-500">
              {field}
            </span>
            <span className="mt-1 block h-2 w-2/3 rounded-full bg-gray-100" />
          </div>
        ))}
      </div>
    </section>
  );
}

function ConditionalBadge({ field }: Readonly<{ field: FormField }>) {
  if (!field.visible_when) return null;
  return (
    <span className="bg-moto-blue/10 text-moto-blue-hover inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-sm font-medium">
      bedingt
    </span>
  );
}

export function PreviewAttachmentNames({
  attachments,
}: Readonly<{ attachments?: FormAttachment[] }>) {
  if (!attachments?.length) return null;
  return (
    <ul className="mt-2 space-y-1 text-sm text-gray-600">
      {attachments.map((attachment) => (
        <li key={attachment.document_url}>PDF: {attachment.label}</li>
      ))}
    </ul>
  );
}

function PreviewCustomField({
  field,
  badge,
}: Readonly<{ field: FormField; badge?: ReactNode }>) {
  if (field.type === "information") {
    return (
      <div className="border-moto-blue/20 bg-moto-blue/5 rounded-lg border px-3 py-2">
        <div className="flex items-start justify-between gap-3">
          {field.label.trim() ? (
            <span className="text-sm font-semibold text-gray-900">
              {field.label}
            </span>
          ) : (
            <span className="text-moto-blue-hover text-xs font-medium">
              Infotext
            </span>
          )}
          <ConditionalBadge field={field} />
        </div>
        {field.content?.trim() ? (
          <p className="mt-1 text-xs leading-5 whitespace-pre-line text-gray-600">
            {field.content}
          </p>
        ) : (
          <p className="mt-1 text-xs leading-5 text-gray-400 italic">
            Noch kein Text hinterlegt.
          </p>
        )}
        <PreviewAttachmentNames attachments={field.attachments} />
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-gray-200 bg-white px-3 py-2">
      <div className="flex items-start justify-between gap-3">
        <span className="text-sm font-medium text-gray-900">
          {field.label.trim() || "Neue Zusatzfrage"}
        </span>
        <div className="flex shrink-0 items-center gap-1.5">
          {badge}
          <ConditionalBadge field={field} />
          {field.required ? (
            <span className="rounded-full bg-gray-100 px-2 py-0.5 text-sm font-medium text-gray-600">
              Pflicht
            </span>
          ) : null}
        </div>
      </div>
      {field.help_text ? (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      ) : null}
      <PreviewInput field={field} />
    </div>
  );
}

function PreviewInput({ field }: Readonly<{ field: FormField }>) {
  if (field.type === "boolean") {
    return (
      <div className="mt-3 grid grid-cols-2 gap-2">
        <span className="flex h-9 items-center justify-center rounded-lg border border-gray-200 bg-gray-50 text-xs font-medium text-gray-500">
          Ja
        </span>
        <span className="flex h-9 items-center justify-center rounded-lg border border-gray-200 bg-gray-50 text-xs font-medium text-gray-500">
          Nein
        </span>
      </div>
    );
  }

  if (field.type === "select") {
    const firstOption = field.options?.[0]?.label ?? "Bitte wählen";
    return (
      <div className="mt-3 flex h-10 items-center justify-between rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-500 shadow-sm">
        <span className="min-w-0 truncate">{firstOption}</span>
        <ChevronDown
          className="h-4 w-4 shrink-0 text-gray-400"
          aria-hidden="true"
        />
      </div>
    );
  }

  if (field.type === "file") {
    return <FilePreviewInput field={field} />;
  }

  if (field.type === "textarea") {
    return (
      <div className="mt-3 h-16 rounded-lg border border-gray-200 bg-gray-50" />
    );
  }

  return (
    <div className="mt-3 h-9 rounded-lg border border-gray-200 bg-gray-50" />
  );
}
/**
 * The parent form as a preview: a header above it, the sections in it and
 * the send button at its end. Each page decides which fixed sections and
 * which questions it shows.
 */
export function FormPreview({
  header,
  title,
  intro = "Bitte füllen Sie das Formular vollständig aus.",
  submitLabel = "Anmeldung absenden",
  sticky = true,
  children,
}: Readonly<{
  header: ReactNode;
  title: string;
  intro?: string;
  submitLabel?: string;
  sticky?: boolean;
  children: ReactNode;
}>) {
  return (
    <div className={sticky ? "sticky top-6 space-y-4" : "space-y-4"}>
      {header}
      <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
        <div className="border-b border-gray-100 px-4 py-4">
          <h3 className="text-lg font-semibold text-gray-900">{title}</h3>
          <p className="mt-1 text-sm leading-6 text-gray-500">{intro}</p>
        </div>

        <div className="space-y-5 p-4">
          {children}
          <Button type="button" variant="primary" size="md" className="w-full">
            {submitLabel}
          </Button>
        </div>
      </div>
    </div>
  );
}

/** The heading of the preview with its eye icon and an optional status. */
export function FormPreviewHeading({
  title,
  status,
  hint,
}: Readonly<{ title: string; status?: ReactNode; hint: string }>) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div>
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="text-base font-semibold text-gray-900">{title}</h2>
          {status}
        </div>
        <p className="mt-1 max-w-sm text-xs leading-5 text-gray-500">{hint}</p>
      </div>
      <span className="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
        <Eye className="h-4 w-4" aria-hidden="true" />
      </span>
    </div>
  );
}

/** The questions of a form in the preview, or a hint when there are none. */
export function PreviewFieldsSection({
  title,
  fields,
  emptyTitle,
  emptyText,
  fieldBadge,
}: Readonly<{
  title: string;
  fields: readonly FormField[];
  emptyTitle: string;
  emptyText: string;
  fieldBadge?: (field: FormField) => ReactNode;
}>) {
  return (
    <section>
      <div className="mb-3 flex items-center justify-between gap-3">
        <h4 className="text-sm font-semibold text-gray-900">{title}</h4>
        <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
          {fields.length}
        </span>
      </div>
      {fields.length === 0 ? (
        <div className="rounded-xl border border-dashed border-gray-200 bg-gray-50/80 px-4 py-5 text-center">
          <HelpCircle className="mx-auto h-5 w-5 text-gray-400" />
          <p className="mt-2 text-sm font-medium text-gray-700">{emptyTitle}</p>
          <p className="mt-1 text-xs leading-5 text-gray-500">{emptyText}</p>
        </div>
      ) : (
        <div className="space-y-3">
          {fields.map((field) => (
            <PreviewCustomField
              key={getStableObjectKey(field, "preview-field")}
              field={field}
              badge={fieldBadge?.(field)}
            />
          ))}
        </div>
      )}
    </section>
  );
}
