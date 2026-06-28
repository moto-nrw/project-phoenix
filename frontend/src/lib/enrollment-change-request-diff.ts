export interface EnrollmentChangeRequestDiffInput {
  readonly baseSnapshot: Record<string, unknown>;
  readonly proposedSnapshot: Record<string, unknown>;
  readonly diff: Record<string, unknown>;
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

const SNAPSHOT_LABELS: Record<string, string> = {
  additional_guardians: "Weitere erziehungsberechtigte Personen",
  children: "Kinder",
  consent_flags: "Zustimmungen",
  custom_data: "Zusatzangaben",
  guardian_email: "E-Mail",
  guardian_first_name: "Vorname",
  guardian_last_name: "Nachname",
  guardian_phone: "Telefon",
  phase_id: "Anmeldephase",
};

const CHILD_SNAPSHOT_LABELS: Record<string, string> = {
  custom_data: "Zusatzangaben",
  date_of_birth: "Geburtsdatum",
  first_name: "Vorname",
  last_name: "Nachname",
  offering_days: "Betreuungstage",
  offering_ids: "Betreuungsangebote",
  target_grade_level: "Zielklasse",
};

const GUARDIAN_SNAPSHOT_LABELS: Record<string, string> = {
  email: "E-Mail",
  first_name: "Vorname",
  last_name: "Nachname",
  phone: "Telefon",
};

const RECORD_VALUE_LABELS: Record<string, string> = {
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
};

const CHILD_METADATA_KEYS = new Set(["id", "status"]);
const GUARDIAN_METADATA_KEYS = new Set(["id"]);

const WEEKDAY_LABELS: Record<string, string> = {
  mon: "Mo",
  tue: "Di",
  wed: "Mi",
  thu: "Do",
  fri: "Fr",
  sat: "Sa",
  sun: "So",
};

export function buildEnrollmentChangeRequestDiffGroups({
  baseSnapshot,
  proposedSnapshot,
  diff,
}: EnrollmentChangeRequestDiffInput): EnrollmentChangeRequestDiffGroup[] {
  return changedSnapshotKeys(baseSnapshot, proposedSnapshot, diff).flatMap(
    (key) => {
      const before = baseSnapshot[key];
      const after = proposedSnapshot[key];
      const rows = snapshotDetailRows(key, before, after);
      if (rows.length === 0) {
        if (suppressesMetadataOnlyDiffs(key)) return [];
        if (snapshotValuesEqual(before, after)) return [];
        return [
          {
            key,
            label: snapshotLabel(key),
            rows: [
              {
                id: key,
                label: snapshotLabel(key),
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
          label: snapshotLabel(key),
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
  emptyLabel = "Nicht gesetzt",
): string {
  if (value === null || value === undefined || value === "") {
    return emptyLabel;
  }
  if (typeof value === "boolean") return value ? "Ja" : "Nein";
  if (typeof value === "string" || typeof value === "number") {
    return formatScalar(value);
  }
  if (Array.isArray(value)) {
    return formatArrayValue(value, emptyLabel);
  }
  const record = snapshotRecord(value);
  if (record) {
    return formatRecordValue(record, emptyLabel);
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
): EnrollmentChangeRequestDiffRow[] {
  if (key === "additional_guardians") {
    return recordArrayDiffRows(
      before,
      after,
      "Weitere Person",
      GUARDIAN_SNAPSHOT_LABELS,
      GUARDIAN_METADATA_KEYS,
    );
  }
  if (key === "children") {
    return recordArrayDiffRows(
      before,
      after,
      "Kind",
      CHILD_SNAPSHOT_LABELS,
      CHILD_METADATA_KEYS,
    );
  }
  if (key === "custom_data") {
    return objectSnapshotDiffRows(before, after, "", new Set());
  }
  if (key === "consent_flags") {
    return objectSnapshotDiffRows(before, after, "", new Set());
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
            metadataKeys,
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

function objectSnapshotDiffRows(
  before: unknown,
  after: unknown,
  labelPrefix: string,
  metadataKeys: ReadonlySet<string>,
): EnrollmentChangeRequestDiffRow[] {
  const beforeRecord = snapshotRecord(before) ?? {};
  const afterRecord = snapshotRecord(after) ?? {};
  return snapshotKeys(beforeRecord, afterRecord).flatMap((key) => {
    if (metadataKeys.has(key)) return [];
    if (snapshotValuesEqual(beforeRecord[key], afterRecord[key])) return [];
    const label = labelPrefix ? `${labelPrefix} · ${key}` : key;
    return [
      {
        id: label,
        label,
        before: beforeRecord[key],
        after: afterRecord[key],
      },
    ];
  });
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

function snapshotLabel(key: string): string {
  return SNAPSHOT_LABELS[key] ?? key;
}

function formatArrayValue(
  value: readonly unknown[],
  emptyLabel: string,
): string {
  if (value.length === 0) return emptyLabel;
  if (value.every(isScalarValue)) {
    return value.map((item) => formatScalar(item)).join(", ");
  }
  return value
    .map((item, index) => {
      const record = snapshotRecord(item);
      if (record) return formatRecordValue(record, emptyLabel);
      return `${index + 1}. ${formatEnrollmentChangeRequestValue(item, emptyLabel)}`;
    })
    .join("; ");
}

function formatRecordValue(
  value: Record<string, unknown>,
  emptyLabel: string,
): string {
  const weekdaySummary = formatWeekdayRecord(value);
  if (weekdaySummary) return weekdaySummary;

  const entries = snapshotKeys(value, {}).filter(
    (key) =>
      value[key] !== null && value[key] !== undefined && value[key] !== "",
  );
  if (entries.length === 0) return emptyLabel;

  return entries
    .map(
      (key) =>
        `${RECORD_VALUE_LABELS[key] ?? key}: ${formatEnrollmentChangeRequestValue(
          value[key],
          emptyLabel,
        )}`,
    )
    .join("; ");
}

function formatWeekdayRecord(value: Record<string, unknown>): string | null {
  const weekdayKeys = Object.keys(WEEKDAY_LABELS);
  const presentWeekdays = weekdayKeys.filter((key) => key in value);
  if (presentWeekdays.length === 0) return null;

  const selectedBooleanDays = presentWeekdays
    .filter((key) => value[key] === true)
    .map((key) => WEEKDAY_LABELS[key]);
  if (selectedBooleanDays.length > 0) return selectedBooleanDays.join(", ");

  const stringDays = presentWeekdays
    .filter((key) => typeof value[key] === "string" && value[key] !== "")
    .map((key) => `${WEEKDAY_LABELS[key]}: ${value[key] as string}`);
  if (stringDays.length > 0) return stringDays.join(", ");

  const arrayDays = presentWeekdays
    .filter(
      (key) =>
        Array.isArray(value[key]) && (value[key] as unknown[]).length > 0,
    )
    .map(
      (key) =>
        `${WEEKDAY_LABELS[key]}: ${formatArrayValue(
          value[key] as unknown[],
          "",
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

function formatScalar(value: string | number | boolean): string {
  if (typeof value === "boolean") return value ? "Ja" : "Nein";
  if (typeof value === "string") return WEEKDAY_LABELS[value] ?? value;
  return String(value);
}
