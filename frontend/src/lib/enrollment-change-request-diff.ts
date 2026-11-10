import type { Translations } from "~/lib/enrollment-translations";
import { WEEKDAY_CODES, weekdayShortLabel } from "~/lib/weekdays";

export interface EnrollmentChangeRequestDiffInput {
  readonly baseSnapshot: Record<string, unknown>;
  readonly proposedSnapshot: Record<string, unknown>;
  readonly diff: Record<string, unknown>;
  readonly copy?: EnrollmentChangeRequestDiffCopy;
  /** Questions of the request's form version; without them rows show keys. */
  readonly formLabels?: EnrollmentFormLabels | null;
}

interface EnrollmentFormLabelOption {
  readonly label: string;
  readonly value: string;
  readonly translations?: Translations;
}

interface EnrollmentFormLabelField {
  readonly key: string;
  readonly label: string;
  readonly applies_to_child?: boolean;
  readonly options?: EnrollmentFormLabelOption[];
  readonly translations?: Translations;
}

interface EnrollmentFormLabelLegalBlock {
  readonly key: string;
  readonly title: string;
  readonly translations?: Translations;
}

/**
 * The questions and consent titles of the form version a request is pinned
 * to (#6142), as `GET …/form-labels` answers them. Callers localize them
 * before passing them in.
 */
export interface EnrollmentFormLabels {
  readonly fields: EnrollmentFormLabelField[];
  readonly legal_blocks: EnrollmentFormLabelLegalBlock[];
}

interface EnrollmentChangeRequestDiffRow {
  readonly id: string;
  readonly label: string;
  readonly before: unknown;
  readonly after: unknown;
}

export interface EnrollmentChangeRequestDiffGroup {
  readonly key: string;
  readonly label: string;
  readonly rows: EnrollmentChangeRequestDiffRow[];
}

export interface EnrollmentChangeRequestDiffCopy {
  readonly snapshotLabels: Record<string, string>;
  readonly childSnapshotLabels: Record<string, string>;
  readonly guardianSnapshotLabels: Record<string, string>;
  readonly recordValueLabels: Record<string, string>;
  readonly weekdayLabels: Record<string, string>;
  /** Titles of the standard consents for requests without form titles. */
  readonly consentLabels?: Record<string, string>;
  readonly childEntryLabel: string;
  readonly additionalGuardianEntryLabel: string;
  readonly emptyLabel: string;
  readonly yesLabel: string;
  readonly noLabel: string;
}

export const DEFAULT_ENROLLMENT_CHANGE_REQUEST_DIFF_COPY: EnrollmentChangeRequestDiffCopy =
  {
    snapshotLabels: {
      additional_guardians: "Weitere erziehungsberechtigte Personen",
      children: "Kinder",
      consent_flags: "Zustimmungen",
      custom_data: "Zusatzangaben",
      guardian_email: "E-Mail",
      guardian_first_name: "Vorname",
      guardian_last_name: "Nachname",
      guardian_phone: "Telefon",
      phase_id: "Anmeldephase",
    },
    childSnapshotLabels: {
      custom_data: "Zusatzangaben",
      date_of_birth: "Geburtsdatum",
      first_name: "Vorname",
      holiday_care_wish: "Ferienwunsch",
      last_name: "Nachname",
      offering_days: "Betreuungstage",
      offering_ids: "Betreuungsangebote",
      target_grade_level: "Zielklasse",
      target_school_class: "Klasse",
    },
    guardianSnapshotLabels: {
      email: "E-Mail",
      first_name: "Vorname",
      last_name: "Nachname",
      phone: "Telefon",
    },
    recordValueLabels: {
      automatic_selected_days: "Automatisch",
      available_days: "Verfügbare Tage",
      days_of_week_mode: "Modus",
      email: "E-Mail",
      first_name: "Vorname",
      last_name: "Nachname",
      manual_selected_days: "Manuell",
      offering_id: "Betreuungsangebot",
      offering_ids: "Betreuungsangebote",
      phone: "Telefon",
      selected_days: "Tage",
    },
    weekdayLabels: Object.fromEntries(
      WEEKDAY_CODES.map((code) => [code, weekdayShortLabel(code)]),
    ),
    consentLabels: {
      agb: "AGB der Schule",
      data_processing: "Datenverarbeitung (DSGVO)",
      email_contact: "E-Mail-Kontakt",
      photo: "Fotos bei Schulveranstaltungen",
    },
    childEntryLabel: "Kind",
    additionalGuardianEntryLabel: "Weitere Person",
    emptyLabel: "Nicht gesetzt",
    yesLabel: "Ja",
    noLabel: "Nein",
  };

const CHILD_METADATA_KEYS = new Set(["id", "status"]);
const NO_METADATA_KEYS: ReadonlySet<string> = new Set();
const NO_KEY_LABELS: ObjectKeyLabels = new Map();

/** How one stored key reads: its question and its option labels. */
interface ObjectKeyLabel {
  readonly label: string;
  readonly options?: ReadonlyMap<string, string>;
}

type ObjectKeyLabels = ReadonlyMap<string, ObjectKeyLabel>;

interface FormKeyLabels {
  readonly guardian: ObjectKeyLabels;
  readonly child: ObjectKeyLabels;
  readonly consent: ObjectKeyLabels;
}

function formKeyLabels(
  formLabels: EnrollmentFormLabels | null | undefined,
  copy: EnrollmentChangeRequestDiffCopy,
): FormKeyLabels {
  const guardian = new Map<string, ObjectKeyLabel>();
  const child = new Map<string, ObjectKeyLabel>();
  for (const field of formLabels?.fields ?? []) {
    const options = field.options?.length
      ? new Map(field.options.map((option) => [option.value, option.label]))
      : undefined;
    (field.applies_to_child ? child : guardian).set(field.key, {
      label: field.label,
      options,
    });
  }
  const consent = new Map<string, ObjectKeyLabel>(
    Object.entries(copy.consentLabels ?? {}).map(([key, label]) => [
      key,
      { label },
    ]),
  );
  for (const block of formLabels?.legal_blocks ?? []) {
    if (block.title.trim()) consent.set(block.key, { label: block.title });
  }
  return { guardian, child, consent };
}
const GUARDIAN_METADATA_KEYS = new Set(["id"]);

export function buildEnrollmentChangeRequestDiffGroups({
  baseSnapshot,
  copy = DEFAULT_ENROLLMENT_CHANGE_REQUEST_DIFF_COPY,
  proposedSnapshot,
  diff,
  formLabels,
}: EnrollmentChangeRequestDiffInput): EnrollmentChangeRequestDiffGroup[] {
  const labels = formKeyLabels(formLabels, copy);
  return changedSnapshotKeys(baseSnapshot, proposedSnapshot, diff).flatMap(
    (key) => {
      const before = baseSnapshot[key];
      const after = proposedSnapshot[key];
      const rows = snapshotDetailRows(key, before, after, copy, labels);
      if (rows.length === 0) {
        if (suppressesMetadataOnlyDiffs(key)) return [];
        if (snapshotValuesEqual(before, after)) return [];
        return [
          {
            key,
            label: snapshotLabel(key, copy),
            rows: [
              {
                id: key,
                label: snapshotLabel(key, copy),
                before,
                after,
              },
            ],
          },
        ];
      }
      return [
        {
          key,
          label: snapshotLabel(key, copy),
          rows,
        },
      ];
    },
  );
}

export function changedEnrollmentChangeRequestLabels(
  input: EnrollmentChangeRequestDiffInput,
): string[] {
  return buildEnrollmentChangeRequestDiffGroups(input).map(
    (group) => group.label,
  );
}

export function formatEnrollmentChangeRequestValue(
  value: unknown,
  emptyLabel = DEFAULT_ENROLLMENT_CHANGE_REQUEST_DIFF_COPY.emptyLabel,
  copy: EnrollmentChangeRequestDiffCopy = DEFAULT_ENROLLMENT_CHANGE_REQUEST_DIFF_COPY,
): string {
  if (value === null || value === undefined || value === "") {
    return emptyLabel;
  }
  if (typeof value === "boolean") return value ? copy.yesLabel : copy.noLabel;
  if (typeof value === "string" || typeof value === "number") {
    return formatScalar(value, copy);
  }
  if (Array.isArray(value)) {
    return formatArrayValue(value, emptyLabel, copy);
  }
  const record = snapshotRecord(value);
  if (record) {
    return formatRecordValue(record, emptyLabel, copy);
  }
  return String(value);
}

function changedSnapshotKeys(
  baseSnapshot: Record<string, unknown>,
  proposedSnapshot: Record<string, unknown>,
  diff: Record<string, unknown>,
): string[] {
  const changed = diff.changed;
  if (Array.isArray(changed)) {
    return changed
      .filter((value): value is string => typeof value === "string")
      .filter(
        (key) => !snapshotValuesEqual(baseSnapshot[key], proposedSnapshot[key]),
      );
  }

  const explicitKeys = Object.keys(diff).filter((key) => key !== "changed");
  if (explicitKeys.length > 0) {
    return explicitKeys.filter(
      (key) => !snapshotValuesEqual(baseSnapshot[key], proposedSnapshot[key]),
    );
  }

  return snapshotKeys(baseSnapshot, proposedSnapshot).filter(
    (key) => !snapshotValuesEqual(baseSnapshot[key], proposedSnapshot[key]),
  );
}

function snapshotDetailRows(
  key: string,
  before: unknown,
  after: unknown,
  copy: EnrollmentChangeRequestDiffCopy,
  labels: FormKeyLabels,
): EnrollmentChangeRequestDiffRow[] {
  if (key === "additional_guardians") {
    return recordArrayDiffRows(
      before,
      after,
      copy.additionalGuardianEntryLabel,
      copy.guardianSnapshotLabels,
      GUARDIAN_METADATA_KEYS,
      NO_KEY_LABELS,
    );
  }
  if (key === "children") {
    return recordArrayDiffRows(
      before,
      after,
      copy.childEntryLabel,
      copy.childSnapshotLabels,
      CHILD_METADATA_KEYS,
      labels.child,
    );
  }
  if (key === "custom_data") {
    return objectSnapshotDiffRows(
      before,
      after,
      "",
      NO_METADATA_KEYS,
      labels.guardian,
    );
  }
  if (key === "consent_flags") {
    return objectSnapshotDiffRows(
      before,
      after,
      "",
      NO_METADATA_KEYS,
      labels.consent,
    );
  }
  return [];
}

function suppressesMetadataOnlyDiffs(key: string): boolean {
  return key === "additional_guardians" || key === "children";
}

function recordArrayDiffRows(
  before: unknown,
  after: unknown,
  entryLabel: string,
  labels: Record<string, string>,
  metadataKeys: ReadonlySet<string>,
  customDataLabels: ObjectKeyLabels,
): EnrollmentChangeRequestDiffRow[] {
  const beforeRows = Array.isArray(before) ? before : [];
  const afterRows = Array.isArray(after) ? after : [];
  const rows: EnrollmentChangeRequestDiffRow[] = [];
  const max = Math.max(beforeRows.length, afterRows.length);

  for (let index = 0; index < max; index += 1) {
    const rawBeforeRecord = snapshotRecord(beforeRows[index]);
    const rawAfterRecord = snapshotRecord(afterRows[index]);
    const beforeRecord = rawBeforeRecord ?? (rawAfterRecord ? {} : null);
    const afterRecord = rawAfterRecord ?? (rawBeforeRecord ? {} : null);
    const rowLabel = `${entryLabel} ${index + 1}`;

    if (!beforeRecord || !afterRecord) {
      if (!snapshotValuesEqual(beforeRows[index], afterRows[index])) {
        rows.push({
          id: `${entryLabel}-${index}`,
          label: rowLabel,
          before: beforeRows[index],
          after: afterRows[index],
        });
      }
      continue;
    }

    const keys = snapshotKeys(beforeRecord, afterRecord);
    for (const childKey of keys) {
      if (metadataKeys.has(childKey)) continue;
      if (childKey === "custom_data") {
        rows.push(
          ...objectSnapshotDiffRows(
            beforeRecord[childKey],
            afterRecord[childKey],
            `${rowLabel} · ${labels[childKey] ?? childKey}`,
            NO_METADATA_KEYS,
            customDataLabels,
            rowLabel,
          ),
        );
        continue;
      }
      if (!snapshotValuesEqual(beforeRecord[childKey], afterRecord[childKey])) {
        rows.push({
          id: `${entryLabel}-${index}-${childKey}`,
          label: `${rowLabel} · ${labels[childKey] ?? childKey}`,
          before: beforeRecord[childKey],
          after: afterRecord[childKey],
        });
      }
    }
  }

  return rows;
}

// A key the form names reads as its question under labelledPrefix; the id
// keeps the key so rows stay stable when a school renames a question.
function objectSnapshotDiffRows(
  before: unknown,
  after: unknown,
  labelPrefix: string,
  metadataKeys: ReadonlySet<string>,
  keyLabels: ObjectKeyLabels,
  labelledPrefix = labelPrefix,
): EnrollmentChangeRequestDiffRow[] {
  const beforeRecord = snapshotRecord(before) ?? {};
  const afterRecord = snapshotRecord(after) ?? {};
  return snapshotKeys(beforeRecord, afterRecord).flatMap((key) => {
    if (metadataKeys.has(key)) return [];
    if (snapshotValuesEqual(beforeRecord[key], afterRecord[key])) return [];
    const id = labelPrefix ? `${labelPrefix} · ${key}` : key;
    const known = keyLabels.get(key);
    const label = known
      ? labelledPrefix
        ? `${labelledPrefix} · ${known.label}`
        : known.label
      : id;
    return [
      {
        id,
        label,
        before: optionLabels(beforeRecord[key], known?.options),
        after: optionLabels(afterRecord[key], known?.options),
      },
    ];
  });
}

/** A stored select value reads as its option label; unknown values stay. */
function optionLabels(
  value: unknown,
  options: ReadonlyMap<string, string> | undefined,
): unknown {
  if (!options) return value;
  if (typeof value === "string") return options.get(value) ?? value;
  if (Array.isArray(value)) {
    return value.map((item) =>
      typeof item === "string" ? (options.get(item) ?? item) : item,
    );
  }
  return value;
}

function snapshotKeys(
  before: Record<string, unknown>,
  after: Record<string, unknown>,
): string[] {
  return Array.from(
    new Set([...Object.keys(before), ...Object.keys(after)]),
  ).sort((a, b) => a.localeCompare(b));
}

function snapshotRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function snapshotValuesEqual(before: unknown, after: unknown): boolean {
  return (
    JSON.stringify(normalizeSnapshotValue(before)) ===
    JSON.stringify(normalizeSnapshotValue(after))
  );
}

function normalizeSnapshotValue(value: unknown): unknown {
  if (value === undefined) return null;
  if (Array.isArray(value)) return value.map(normalizeSnapshotValue);
  const record = snapshotRecord(value);
  if (!record) return value;
  return snapshotKeys(record, {}).reduce<Record<string, unknown>>(
    (out, key) => {
      out[key] = normalizeSnapshotValue(record[key]);
      return out;
    },
    {},
  );
}

function snapshotLabel(
  key: string,
  copy: EnrollmentChangeRequestDiffCopy,
): string {
  return copy.snapshotLabels[key] ?? key;
}

function formatArrayValue(
  value: readonly unknown[],
  emptyLabel: string,
  copy: EnrollmentChangeRequestDiffCopy,
): string {
  if (value.length === 0) return emptyLabel;
  if (value.every(isScalarValue)) {
    return value.map((item) => formatScalar(item, copy)).join(", ");
  }
  return value
    .map((item, index) => {
      const record = snapshotRecord(item);
      if (record) return formatRecordValue(record, emptyLabel, copy);
      return `${index + 1}. ${formatEnrollmentChangeRequestValue(item, emptyLabel, copy)}`;
    })
    .join("; ");
}

function formatRecordValue(
  value: Record<string, unknown>,
  emptyLabel: string,
  copy: EnrollmentChangeRequestDiffCopy,
): string {
  const weekdaySummary = formatWeekdayRecord(value, copy);
  if (weekdaySummary) return weekdaySummary;

  const entries = snapshotKeys(value, {}).filter(
    (key) =>
      value[key] !== null && value[key] !== undefined && value[key] !== "",
  );
  if (entries.length === 0) return emptyLabel;

  return entries
    .map(
      (key) =>
        `${copy.recordValueLabels[key] ?? key}: ${formatEnrollmentChangeRequestValue(
          value[key],
          emptyLabel,
          copy,
        )}`,
    )
    .join("; ");
}

function formatWeekdayRecord(
  value: Record<string, unknown>,
  copy: EnrollmentChangeRequestDiffCopy,
): string | null {
  const weekdayKeys = Object.keys(copy.weekdayLabels);
  const presentWeekdays = weekdayKeys.filter((key) => key in value);
  if (presentWeekdays.length === 0) return null;

  const selectedBooleanDays = presentWeekdays
    .filter((key) => value[key] === true)
    .map((key) => copy.weekdayLabels[key]);
  if (selectedBooleanDays.length > 0) return selectedBooleanDays.join(", ");

  const stringDays = presentWeekdays
    .filter((key) => typeof value[key] === "string" && value[key] !== "")
    .map((key) => `${copy.weekdayLabels[key]}: ${value[key] as string}`);
  if (stringDays.length > 0) return stringDays.join(", ");

  const arrayDays = presentWeekdays
    .filter(
      (key) =>
        Array.isArray(value[key]) && (value[key] as unknown[]).length > 0,
    )
    .map(
      (key) =>
        `${copy.weekdayLabels[key]}: ${formatArrayValue(
          value[key] as unknown[],
          "",
          copy,
        )}`,
    );
  return arrayDays.length > 0 ? arrayDays.join(", ") : null;
}

function isScalarValue(value: unknown): value is string | number | boolean {
  return (
    typeof value === "string" ||
    typeof value === "number" ||
    typeof value === "boolean"
  );
}

function formatScalar(
  value: string | number | boolean,
  copy: EnrollmentChangeRequestDiffCopy,
): string {
  if (typeof value === "boolean") return value ? copy.yesLabel : copy.noLabel;
  if (typeof value === "string") return copy.weekdayLabels[value] ?? value;
  return String(value);
}
