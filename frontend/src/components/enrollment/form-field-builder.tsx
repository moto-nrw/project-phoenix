"use client";

// Gemeinsame Bausteine des Formular-Baukastens (#5198): eine Frage
// bearbeiten und ihre Sichtbarkeitsregel setzen; die Vorschau des
// Elternformulars steht in form-preview.tsx. Der Schul-Editor der Online-Anmeldung und das
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
} from "react";
import { Alert } from "~/components/ui/alert";
import { InsetPanel } from "~/components/ui/inset-panel";
import { ArrowDown, ArrowUp, Info, Lock, Plus, Trash2 } from "lucide-react";
import { StatusBadge } from "~/components/ui/status-badge";
import { AttachmentEditor } from "~/components/enrollment/attachment-editor";
import {
  FileFieldOptionsEditor,
  fieldTypeChangePatch,
} from "~/components/enrollment/file-field-editor";
import { Input } from "~/components/ui/input";
import {
  SectionCard,
  SectionCollapseToggle,
} from "~/components/ui/section-card";
import { CheckboxCard } from "~/components/ui/checkbox-card";
import { FieldLabel } from "~/components/ui/field-label";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { EmptyState } from "~/components/ui/empty-state";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { Textarea } from "~/components/ui/textarea";
import {
  RESERVED_TARGETS,
  type ConditionOperator,
  type ConditionSource,
  type FormField,
  type FormFieldType,
  type VisibilityCondition,
} from "~/lib/enrollment-form-schema-api";
import {
  canAskPerChild,
  canControlVisibility,
  hasControllingType,
  normalizeFieldKey,
} from "~/lib/enrollment-form-field-rules";
import { isSupportedGradeLevel } from "~/lib/grade-level";

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
  // `Input` names its hint `${id}-error`, so a kit control given `controlId`
  // carries the same hint id as a raw control spreading `controlProps`.
  const controlId = name.replaceAll(".", "-");
  const hintId = `${controlId}-error`;
  return {
    message,
    controlId,
    hintId,
    controlProps: {
      name,
      "aria-invalid": message ? true : undefined,
      "aria-describedby": message ? hintId : undefined,
    },
  } as const;
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
  /** Why `disabled` locks the row, e.g. a running upload; none while saving. */
  readonly disabledReason?: string;
  /** The types a free question may take; the online registration's by default. */
  readonly fieldTypes?: readonly FormFieldType[];
  /** The sources a visibility condition may use; all by default. */
  readonly conditionSources?: readonly ConditionSource[];
  /** Whether an information block may carry PDF attachments. */
  readonly allowAttachments?: boolean;
  /**
   * The explanation of a free question under its heading; `null` when the
   * page says it once above all questions instead of on every card.
   */
  readonly freeQuestionNote?: {
    readonly title: string;
    readonly text: string;
  } | null;
  /**
   * The switch „Gesundheitsangabe“ of the holiday registration (#5198). The
   * marker is not part of a school form field, so the page owns its value.
   */
  readonly healthToggle?: {
    readonly checked: boolean;
    readonly onChange: (checked: boolean) => void;
  };
  /**
   * The switch „OGS sieht die Antwort“ of the holiday registration (#5202).
   * A health question carries it too since #4722: its answer reaches the
   * hosting OGS with the other health information, only with the parents'
   * consent.
   */
  readonly hostShareToggle?: {
    readonly checked: boolean;
    readonly onChange: (checked: boolean) => void;
  };
  /**
   * Opt-in: the question folds to its head with a one-line summary and opens
   * with the chevron. The body stays mounted, so its drafts survive, and a
   * question with a marked field stays open. Without it every question is
   * open, as in the school's builder.
   */
  readonly fold?: {
    readonly open: boolean;
    readonly onToggle: () => void;
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
  disabledReason,
  fieldTypes = freeFieldTypes,
  conditionSources = ALL_CONDITION_SOURCES,
  allowAttachments = true,
  freeQuestionNote = SCHOOL_FREE_QUESTION_NOTE,
  healthToggle,
  hostShareToggle,
  fold,
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
  const bodyId = useId();
  // A marked field must be visible to be focused and read.
  const marked = [
    labelError.message,
    contentError.message,
    optionsError.message,
    fieldError(`fields.${index}.file_options`),
  ].some(Boolean);
  const open = !fold || fold.open || marked;
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
  // What a folded question shows under its title: type, duty and scope.
  const foldSummary = isInfo
    ? field.applies_to_child
      ? "Hinweis pro Kind"
      : "Hinweis"
    : [
        fieldTypeLabels[field.type],
        field.required ? "Pflichtfrage" : "freiwillig",
        field.applies_to_child ? "pro Kind" : null,
        healthToggle?.checked ? "Gesundheitsangabe" : null,
        hostShareToggle?.checked ? "OGS sieht die Antwort" : null,
      ]
        .filter(Boolean)
        .join(" · ");
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
  // tell an *external* change (times written into this field from outside
  // the row) from a *self-induced* one (our own
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
          .filter(isSupportedGradeLevel),
      ),
    );
    onChange({ single_mode_grades: grades });
  };

  return (
    // Die Frage bleibt ein <article>: Liste und Tests erkennen jede Frage
    // daran; die Fläche kommt aus dem Kit (#6137).
    <article>
      <SectionCard>
        <div className="flex items-start gap-3">
          {/* No grip: the questions move with the arrows, not by dragging. */}
          <div className="min-w-0 flex-1 space-y-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <div className="flex flex-wrap items-center gap-2">
                  <p className="text-sm font-semibold text-gray-900">
                    {isInfo ? "Infotext" : "Frage"} {index + 1}
                  </p>
                  {isInfo ? (
                    <StatusBadge
                      tone="blue"
                      icon={Info}
                      compact
                      label="Hinweis"
                    />
                  ) : isTargetField ? (
                    <StatusBadge
                      tone="gray"
                      icon={Lock}
                      compact
                      label="Fester Vorschlag"
                    />
                  ) : (
                    <StatusBadge
                      tone="gray"
                      compact
                      label="Freie Zusatzfrage"
                    />
                  )}
                </div>
                <h3 className="mt-1 text-sm font-semibold text-gray-900">
                  {displayLabel}
                </h3>
                {open ? null : (
                  <p className="mt-0.5 text-sm text-gray-600">{foldSummary}</p>
                )}
              </div>
              <div className="flex gap-1.5">
                <Button
                  type="button"
                  onClick={onMoveUp}
                  disabled={disabled || index === 0}
                  disabledReason={
                    index === 0
                      ? "Die Frage steht schon ganz oben."
                      : disabledReason
                  }
                  variant="surface"
                  size="icon"
                  aria-label="Frage nach oben verschieben"
                  tooltip="Setzt die Frage im Elternformular eine Stelle weiter nach oben."
                >
                  <ArrowUp className="h-4 w-4" aria-hidden="true" />
                </Button>
                <Button
                  type="button"
                  onClick={onMoveDown}
                  disabled={disabled || index === total - 1}
                  disabledReason={
                    index === total - 1
                      ? "Die Frage steht schon ganz unten."
                      : disabledReason
                  }
                  variant="surface"
                  size="icon"
                  aria-label="Frage nach unten verschieben"
                  tooltip="Setzt die Frage im Elternformular eine Stelle weiter nach unten."
                >
                  <ArrowDown className="h-4 w-4" aria-hidden="true" />
                </Button>
                <Button
                  type="button"
                  onClick={onRemove}
                  disabled={disabled}
                  variant="outline_danger"
                  size="icon"
                  aria-label="Frage entfernen"
                  disabledReason={disabledReason}
                  tooltip="Entfernt die Frage sofort aus dem Entwurf der Vorlage."
                >
                  <Trash2 className="h-4 w-4" aria-hidden="true" />
                </Button>
                {fold ? (
                  <SectionCollapseToggle
                    title={`${isInfo ? "Infotext" : "Frage"} ${index + 1}`}
                    collapsed={!open}
                    controls={bodyId}
                    onToggle={fold.onToggle}
                  />
                ) : null}
              </div>
            </div>
            <div id={bodyId} hidden={!open} className="space-y-4">
              {isInfo ? (
                <Alert
                  type="info"
                  size="compact"
                  announce="off"
                  title="Wird Eltern als Hinweis angezeigt."
                  message="Ein Infotext sammelt keine Antwort. Nutzen Sie ihn für Erklärungen, Hinweise oder Zwischenüberschriften."
                />
              ) : isTargetField ? (
                <InsetPanel
                  framed
                  padding="compact"
                  className="text-xs leading-5 text-gray-600"
                >
                  <p className="font-medium text-gray-800">
                    Wird bei bestätigter Anmeldung in die Stammdaten übernommen.
                  </p>
                  <p className="mt-0.5">
                    Sie können die angezeigte Frage umbenennen. Inhalt und Typ
                    sind fest vorgegeben. Entfernen Sie den Vorschlag, wenn
                    diese Angabe nicht abgefragt werden soll.
                  </p>
                </InsetPanel>
              ) : freeQuestionNote ? (
                <InsetPanel
                  framed
                  padding="compact"
                  className="text-xs leading-5 text-gray-600"
                >
                  <p className="font-medium text-gray-800">
                    {freeQuestionNote.title}
                  </p>
                  <p className="mt-0.5">{freeQuestionNote.text}</p>
                </InsetPanel>
              ) : null}

              {isInfo ? (
                <>
                  <Input
                    id={labelError.controlId}
                    name={labelError.controlProps.name}
                    type="text"
                    label="Titel (optional)"
                    controlSize="touch"
                    value={field.label}
                    onChange={(event) =>
                      onChange({ label: event.target.value })
                    }
                    placeholder="z. B. Wichtiger Hinweis"
                    disabled={disabled}
                    error={labelError.message}
                  />
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
                    <CheckboxCard
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
                    <Input
                      id={labelError.controlId}
                      name={labelError.controlProps.name}
                      type="text"
                      label="Frage im Elternformular"
                      controlSize="touch"
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
                      error={labelError.message}
                    />
                    <div>
                      <FieldLabel>
                        Typ
                        {isTargetField ? (
                          <span className="ml-1 font-normal text-gray-500">
                            (automatisch festgelegt)
                          </span>
                        ) : null}
                      </FieldLabel>
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
                        options={(isTargetField
                          ? [field.type]
                          : fieldTypes
                        ).map((value) => ({
                          value,
                          label: fieldTypeLabels[value],
                        }))}
                      />
                    </div>
                  </div>

                  <Input
                    id={`fields-${index}-help_text`}
                    type="text"
                    label="Hilfetext für Eltern"
                    controlSize="touch"
                    value={field.help_text ?? ""}
                    onChange={(event) =>
                      onChange({ help_text: event.target.value })
                    }
                    placeholder="Optionaler kurzer Hinweis unter der Frage"
                    disabled={disabled}
                  />

                  {field.type === "select" ? (
                    <div>
                      <Textarea
                        id={`field-options-${index}`}
                        name={optionsError.controlProps.name}
                        error={optionsError.message}
                        label="Auswahloptionen"
                        value={optionsDraft}
                        onChange={(event) => updateOptions(event.target.value)}
                        placeholder={
                          "Eine Option pro Zeile\nz. B. Ja\nz. B. Nein"
                        }
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
                      {/* The answer becomes the child's maintained pickup time,
                      which outranks an offering's Gehzeit in the care plan
                      (#6044): two sources for one time. */}
                      <Alert
                        type="info"
                        message="Haben Ihre Betreuungsangebote eine Gehzeit? Dann brauchen Sie diese Frage nicht. Eine Antwort hier ersetzt die Gehzeit des Angebots."
                        className="mb-3"
                      />
                      <span className="text-xs font-medium text-gray-700">
                        Feste Auswahlzeiten (optional)
                      </span>
                      <p className="mt-1 text-xs leading-5 text-gray-500">
                        Ohne Zeiten geben Eltern die Uhrzeit frei ein. Sobald
                        Zeiten hinterlegt sind, wählen Eltern pro Wochentag nur
                        aus dieser Liste.
                      </p>
                      <Button
                        type="button"
                        onClick={addAllowedTime}
                        disabled={disabled}
                        disabledReason={disabledReason}
                        tooltip="Fügt eine Uhrzeit hinzu, die Eltern pro Wochentag wählen können."
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
                            <li
                              key={row.id}
                              className="flex items-center gap-2"
                            >
                              <div className="min-w-0 flex-1">
                                <Input
                                  type="time"
                                  controlSize="touch"
                                  value={row.value}
                                  onChange={(event) =>
                                    updateAllowedTime(index, event.target.value)
                                  }
                                  disabled={disabled}
                                  aria-label={`Auswahlzeit ${index + 1}`}
                                />
                              </div>
                              <OverflowMenu
                                ariaLabel={`Aktionen für Auswahlzeit ${index + 1}`}
                                triggerSize="sm"
                                items={[
                                  {
                                    label: "Auswahlzeit entfernen",
                                    description:
                                      "Entfernt die Uhrzeit sofort aus der Auswahl",
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
                      <FieldLabel
                        htmlFor={`${allowedTimeIdPrefix}-single-mode-grades`}
                        className="mb-1"
                      >
                        Nur ein Heimweg pro Wochentag (optional)
                      </FieldLabel>
                      <p
                        id={`${allowedTimeIdPrefix}-single-mode-grades-hint`}
                        className="mb-2 text-xs leading-5 text-gray-500"
                      >
                        Tragen Sie Jahrgänge ein, deren Eltern pro Wochentag nur
                        einen Heimweg auswählen dürfen, zum Beispiel „1“ für die
                        Erstklässler. Ohne Eintrag bleibt die Mehrfachauswahl
                        für alle Jahrgänge erlaubt. Voraussetzung: Die Abfrage
                        der Klassenstufe ist in den Einstellungen aktiv.
                      </p>
                      <Input
                        id={`${allowedTimeIdPrefix}-single-mode-grades`}
                        type="text"
                        inputMode="numeric"
                        controlSize="touch"
                        aria-describedby={`${allowedTimeIdPrefix}-single-mode-grades-hint`}
                        value={singleModeDraft}
                        onChange={(event) =>
                          updateSingleModeGrades(event.target.value)
                        }
                        placeholder="z. B. 1 oder 1, 2"
                        disabled={disabled}
                      />
                    </div>
                  ) : null}

                  <div className="grid gap-2 sm:grid-cols-2">
                    <CheckboxCard
                      checked={Boolean(field.required)}
                      onChange={(checked) => onChange({ required: checked })}
                      label="Pflichtfrage"
                      hint={getRequiredHint(field)}
                      disabled={disabled}
                    />
                    <CheckboxCard
                      checked={Boolean(field.applies_to_child)}
                      onChange={(checked) =>
                        onChange({ applies_to_child: checked })
                      }
                      label="Pro Kind abfragen"
                      hint={
                        canAskPerChild(field)
                          ? "Für Fragen zum Kind, etwa zu Allergien oder zum Schwimmen. Die Frage erscheint dann für jedes angemeldete Kind."
                          : "Dateien gelten immer für die ganze Anmeldung."
                      }
                      disabled={
                        disabled || isTargetField || !canAskPerChild(field)
                      }
                    />
                    {healthToggle ? (
                      <CheckboxCard
                        checked={healthToggle.checked}
                        onChange={healthToggle.onChange}
                        label="Gesundheitsangabe"
                        hint="Für Angaben zur Gesundheit. Eltern stimmen dann der Verarbeitung zu. Die Antwort steht nicht in Liste und Export."
                        disabled={disabled}
                      />
                    ) : null}
                    {hostShareToggle ? (
                      <CheckboxCard
                        checked={hostShareToggle.checked}
                        onChange={hostShareToggle.onChange}
                        label="OGS sieht die Antwort"
                        hint={
                          healthToggle?.checked
                            ? "Die OGS, die die Woche ausrichtet, sieht die Antwort bei den Gesundheitsangaben des Kindes. Nur wenn die Eltern zustimmen."
                            : "Bei neuen Fragen ist das an. Die ausrichtende OGS sieht die Antwort. Sie können das ausschalten."
                        }
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
        </div>
      </SectionCard>
    </article>
  );
}

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
      i !== index && f.key.trim() !== "" && canControlVisibility(f, field),
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
    const hasQuestionWithoutText = allFields.some(
      (f, i) =>
        i !== index && f.key.trim() === "" && canControlVisibility(f, field),
    );
    const hasChildQuestion = allFields.some(
      (f, i) =>
        i !== index && hasControllingType(f) && Boolean(f.applies_to_child),
    );
    const description = hasQuestionWithoutText
      ? "Schreiben Sie zuerst den Fragetext der anderen Ja/Nein- oder Auswahlfrage. Diese Frage kann dann von deren Antwort abhängen."
      : !field.applies_to_child && hasChildQuestion
        ? "Eine allgemeine Frage kann nicht von einer Frage pro Kind abhängen. Legen Sie eine andere allgemeine Ja/Nein- oder Auswahlfrage an. Schreiben Sie auch den Fragetext dazu."
        : "Legen Sie zuerst eine andere Ja/Nein- oder Auswahlfrage mit Fragetext an. Diese Frage kann dann von deren Antwort abhängen.";
    return (
      <EmptyState
        variant="dashed"
        title="Keine Sichtbarkeitsregel möglich"
        description={`${description}${
          field.applies_to_child || childSources.length === 0
            ? ""
            : " Oder schalten Sie bei dieser Frage „Pro Kind abfragen“ ein."
        }`}
      />
    );
  }

  return (
    <div className="moto-content-surface rounded-xl border p-3">
      <CheckboxCard
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
          <div>
            <FieldLabel
              htmlFor={`condition-${index}-source`}
              id={`condition-${index}-source-label`}
            >
              Sichtbar wenn
            </FieldLabel>
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
              options={sources.map((source) => ({
                value: source,
                label: conditionSourceLabels[source],
              }))}
            />
          </div>

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
              idPrefix={`condition-${index}-offering`}
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
      <div>
        <FieldLabel
          htmlFor={`${idPrefix}-question`}
          id={`${idPrefix}-question-label`}
        >
          Frage
        </FieldLabel>
        <CustomSelect
          ariaLabelledBy={`${idPrefix}-question-label`}
          id={`${idPrefix}-question`}
          value={condition.field ?? ""}
          onChange={changeController}
          disabled={disabled}
          options={controllers.map((controller) => ({
            value: controller.key,
            label: controller.label.trim() || controller.key,
          }))}
        />
      </div>

      <div>
        <FieldLabel
          htmlFor={`${idPrefix}-operator`}
          id={`${idPrefix}-operator-label`}
        >
          Vergleich
        </FieldLabel>
        <CustomSelect
          ariaLabelledBy={`${idPrefix}-operator-label`}
          id={`${idPrefix}-operator`}
          value={condition.operator}
          onChange={(value) => changeOperator(value as ConditionOperator)}
          disabled={disabled}
          options={FIELD_CONDITION_OPERATORS.map((operator) => ({
            value: operator,
            label: conditionOperatorLabels[operator],
          }))}
        />
      </div>

      {condition.operator !== "not_empty" && controller ? (
        <div>
          <FieldLabel
            htmlFor={`${idPrefix}-value`}
            id={`${idPrefix}-value-label`}
          >
            Wert
          </FieldLabel>
          {controller.type === "boolean" ? (
            <CustomSelect
              ariaLabelledBy={`${idPrefix}-value-label`}
              id={`${idPrefix}-value`}
              value={condition.value === true ? "true" : "false"}
              onChange={(value) => onPatch({ value: value === "true" })}
              disabled={disabled}
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
              options={(controller.options ?? []).map((option) => ({
                value: option.value,
                label: option.label,
              }))}
            />
          )}
        </div>
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
      <div>
        <FieldLabel
          htmlFor={`${idPrefix}-operator`}
          id={`${idPrefix}-operator-label`}
        >
          Vergleich
        </FieldLabel>
        <CustomSelect
          ariaLabelledBy={`${idPrefix}-operator-label`}
          id={`${idPrefix}-operator`}
          value={condition.operator}
          onChange={(value) =>
            onPatch({ operator: value as ConditionOperator })
          }
          disabled={disabled}
          options={[
            { value: "eq", label: conditionOperatorLabels.eq },
            { value: "neq", label: conditionOperatorLabels.neq },
          ]}
        />
      </div>
      <Input
        id={`${idPrefix}-value`}
        type="number"
        label="Klassenstufe"
        controlSize="touch"
        min={1}
        value={String(condition.value ?? "")}
        onChange={(event) =>
          onPatch({
            value: event.target.value === "" ? "" : Number(event.target.value),
          })
        }
        disabled={disabled}
      />
    </>
  );
}

function ConditionOfferingControls({
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
    <div>
      <Input
        id={`${idPrefix}-value`}
        type="text"
        label="Angebotsname (enthält)"
        controlSize="touch"
        aria-describedby={`${idPrefix}-hint`}
        value={String(condition.value ?? "")}
        onChange={(event) => onPatch({ value: event.target.value })}
        placeholder="z. B. Mittagessen"
        disabled={disabled}
      />
      <span
        id={`${idPrefix}-hint`}
        className="mt-1 block text-sm leading-5 text-gray-500"
      >
        Sichtbar, wenn ein gewähltes Betreuungsangebot diesen Namen trägt.
      </span>
    </div>
  );
}
