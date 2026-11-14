/**
 * Personalakte des Trägers über alle Schulen (#4705, ADR 0089, ADR 0232).
 *
 * Die Personalverwaltung führt eine Person mit einer Akte über alle Schulen:
 * moto schlägt vor, welche Beschäftigungen eine Person sind, die
 * Personalverwaltung bestätigt jede Verknüpfung und wählt bei verschiedenen
 * Werten je Feld. Die Akte führt die Personalnummer des Trägers und die Bank-
 * und Steuerdaten. Personendaten werden nie zwischengespeichert; jede Anfrage
 * steht im Zugriffsprotokoll der Schulen. Backend-IDs werden hier zu Strings.
 */
import { traegerRequest } from "~/lib/traeger-api";

const BASE = "/api/traeger/staff-records";

/** Die Felder einer Akte, in der Reihenfolge der Vorschau. */
export type StaffRecordField =
  | "first_name"
  | "last_name"
  | "birthday"
  | "personnel_number"
  | "iban"
  | "tax_id"
  | "social_security_number";

/** Warum moto Beschäftigungen zusammen vorschlägt (ADR 0089 Abschnitt 5). */
export type StaffMatchReason =
  "same_account" | "same_personnel_number" | "same_name_birthday" | "single";

type StaffRepresentation = "agreement" | "none";

interface StaffRelease {
  readonly representation: StaffRepresentation;
  readonly releasedAt: string;
}

interface StaffSchool {
  readonly id: string;
  readonly name: string;
}

/** Ein Einsatzort einer Akte: die Beschäftigung an einer Schule. */
interface StaffEmployment {
  readonly schoolId: string;
  readonly schoolName: string;
  readonly staffId: string;
  /** `false`, wenn die Schule die Beschäftigung beendet hat. */
  readonly active: boolean;
}

export interface StaffRecordSummary {
  readonly id: string;
  readonly firstName: string;
  readonly lastName: string;
  /** Kalendertag `YYYY-MM-DD` oder leer. */
  readonly birthday: string;
  readonly personnelNumber: string | null;
  readonly hasFinancial: boolean;
  readonly employments: readonly StaffEmployment[];
}

/** Eine Beschäftigung, die noch zu keiner Akte gehört. */
export interface StaffCandidate {
  readonly schoolId: string;
  readonly schoolName: string;
  readonly staffId: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly birthday: string;
  readonly personnelNumber: string | null;
  readonly hasAccount: boolean;
}

export interface StaffSuggestion {
  readonly reason: StaffMatchReason;
  /** Gesetzt, wenn die Beschäftigungen zu einer bestehenden Akte passen. */
  readonly recordId: string | null;
  readonly members: readonly StaffCandidate[];
}

export interface StaffOverview {
  readonly release: StaffRelease | null;
  readonly schools: readonly StaffSchool[];
  readonly records: readonly StaffRecordSummary[];
  readonly suggestions: readonly StaffSuggestion[];
}

export interface StaffFinancial {
  readonly iban: string | null;
  readonly taxId: string | null;
  readonly socialSecurityNumber: string | null;
}

export interface StaffChange {
  readonly kind: "linked" | "not_chosen" | "changed";
  readonly field: StaffRecordField | "school";
  readonly value: string;
  readonly schoolName: string;
  readonly occurredAt: string;
}

export interface StaffRecordDetail extends StaffRecordSummary {
  readonly financial: StaffFinancial;
  readonly changes: readonly StaffChange[];
}

interface StaffFieldOption {
  /** `record` oder die ID der Schule, deren Beschäftigung den Wert bringt. */
  readonly source: string;
  readonly schoolName: string;
  readonly value: string;
}

interface StaffFieldPreview {
  readonly field: StaffRecordField;
  readonly options: readonly StaffFieldOption[];
  readonly conflict: boolean;
}

export interface StaffLinkPreview {
  readonly record: StaffRecordSummary | null;
  readonly members: readonly StaffCandidate[];
  readonly fields: readonly StaffFieldPreview[];
}

interface StaffMemberRef {
  readonly schoolId: string;
  readonly staffId: string;
}

export interface StaffLinkInput {
  readonly recordId: string | null;
  readonly members: readonly StaffMemberRef[];
  readonly choices: Readonly<Partial<Record<StaffRecordField, string>>>;
}

interface BackendEmployment {
  school_id: number;
  school_name: string;
  staff_id: number;
  active: boolean;
}

interface BackendSummary {
  id: number;
  first_name: string;
  last_name: string;
  birthday?: string;
  personnel_number: string | null;
  has_financial: boolean;
  employments: BackendEmployment[] | null;
}

interface BackendCandidate {
  school_id: number;
  school_name: string;
  staff_id: number;
  first_name: string;
  last_name: string;
  birthday?: string;
  personnel_number: string | null;
  has_account: boolean;
}

interface BackendFinancial {
  iban: string | null;
  tax_id: string | null;
  social_security_number: string | null;
}

interface BackendDetail extends BackendSummary {
  financial: BackendFinancial;
  changes:
    | {
        kind: StaffChange["kind"];
        field: StaffChange["field"];
        value: string;
        school_name?: string;
        occurred_at: string;
      }[]
    | null;
}

interface BackendOverview {
  release: { representation: StaffRepresentation; released_at: string } | null;
  schools: { id: number; name: string }[] | null;
  records: BackendSummary[] | null;
  suggestions:
    | {
        reason: StaffMatchReason;
        staff_record_id?: number;
        members: BackendCandidate[] | null;
      }[]
    | null;
}

interface BackendPreview {
  record?: BackendSummary | null;
  members: BackendCandidate[] | null;
  fields:
    | {
        field: StaffRecordField;
        conflict: boolean;
        options:
          { source: string; school_name?: string; value: string }[] | null;
      }[]
    | null;
}

function mapSummary(record: BackendSummary): StaffRecordSummary {
  return {
    id: String(record.id),
    firstName: record.first_name,
    lastName: record.last_name,
    birthday: record.birthday ?? "",
    personnelNumber: record.personnel_number,
    hasFinancial: record.has_financial,
    employments: (record.employments ?? []).map((employment) => ({
      schoolId: String(employment.school_id),
      schoolName: employment.school_name,
      staffId: String(employment.staff_id),
      active: employment.active,
    })),
  };
}

function mapCandidate(candidate: BackendCandidate): StaffCandidate {
  return {
    schoolId: String(candidate.school_id),
    schoolName: candidate.school_name,
    staffId: String(candidate.staff_id),
    firstName: candidate.first_name,
    lastName: candidate.last_name,
    birthday: candidate.birthday ?? "",
    personnelNumber: candidate.personnel_number,
    hasAccount: candidate.has_account,
  };
}

function mapFinancial(financial: BackendFinancial): StaffFinancial {
  return {
    iban: financial.iban,
    taxId: financial.tax_id,
    socialSecurityNumber: financial.social_security_number,
  };
}

function mapDetail(record: BackendDetail): StaffRecordDetail {
  return {
    ...mapSummary(record),
    financial: mapFinancial(record.financial),
    changes: (record.changes ?? []).map((change) => ({
      kind: change.kind,
      field: change.field,
      value: change.value,
      schoolName: change.school_name ?? "",
      occurredAt: change.occurred_at,
    })),
  };
}

function mapStaffOverview(data: BackendOverview): StaffOverview {
  return {
    release: data.release
      ? {
          representation: data.release.representation,
          releasedAt: data.release.released_at,
        }
      : null,
    schools: (data.schools ?? []).map((school) => ({
      id: String(school.id),
      name: school.name,
    })),
    records: (data.records ?? []).map(mapSummary),
    suggestions: (data.suggestions ?? []).map((suggestion) => ({
      reason: suggestion.reason,
      recordId:
        suggestion.staff_record_id === undefined
          ? null
          : String(suggestion.staff_record_id),
      members: (suggestion.members ?? []).map(mapCandidate),
    })),
  };
}

function linkBody(input: StaffLinkInput) {
  return {
    staff_record_id: input.recordId === null ? null : Number(input.recordId),
    members: input.members.map((member) => ({
      school_id: Number(member.schoolId),
      staff_id: Number(member.staffId),
    })),
    choices: input.choices,
  };
}

const recordPath = (id: string, suffix = "") =>
  `${BASE}/${encodeURIComponent(id)}${suffix}`;

export async function fetchStaffOverview(): Promise<StaffOverview> {
  return mapStaffOverview(await traegerRequest<BackendOverview>(`${BASE}`));
}

export async function fetchStaffRecord(id: string): Promise<StaffRecordDetail> {
  return mapDetail(await traegerRequest<BackendDetail>(recordPath(id)));
}

export async function previewStaffLink(
  input: StaffLinkInput,
): Promise<StaffLinkPreview> {
  const data = await traegerRequest<BackendPreview>(`${BASE}/link-preview`, {
    method: "POST",
    body: linkBody(input),
  });
  return {
    record: data.record ? mapSummary(data.record) : null,
    members: (data.members ?? []).map(mapCandidate),
    fields: (data.fields ?? []).map((field) => ({
      field: field.field,
      conflict: field.conflict,
      options: (field.options ?? []).map((option) => ({
        source: option.source,
        schoolName: option.school_name ?? "",
        value: option.value,
      })),
    })),
  };
}

export async function linkStaff(
  input: StaffLinkInput,
): Promise<StaffRecordDetail> {
  return mapDetail(
    await traegerRequest<BackendDetail>(`${BASE}`, {
      method: "POST",
      body: linkBody(input),
    }),
  );
}

export async function updateStaffPersonnelNumber(
  id: string,
  value: string | null,
): Promise<StaffRecordDetail> {
  return mapDetail(
    await traegerRequest<BackendDetail>(recordPath(id, "/personnel-number"), {
      method: "PUT",
      body: { personnel_number: value },
    }),
  );
}

export async function revealStaffFinancial(
  id: string,
): Promise<StaffFinancial> {
  return mapFinancial(
    await traegerRequest<BackendFinancial>(
      recordPath(id, "/financial/reveal"),
      { method: "POST" },
    ),
  );
}

export async function updateStaffFinancial(
  id: string,
  values: StaffFinancial,
): Promise<void> {
  await traegerRequest<unknown>(recordPath(id, "/financial"), {
    method: "PUT",
    body: {
      iban: values.iban,
      tax_id: values.taxId,
      social_security_number: values.socialSecurityNumber,
    },
  });
}

export async function dissolveStaffRecord(id: string): Promise<void> {
  await traegerRequest<unknown>(recordPath(id), { method: "DELETE" });
}

/** Der Name einer Person, wie die Liste ihn zeigt. */
export function staffName(person: {
  readonly firstName: string;
  readonly lastName: string;
}): string {
  return `${person.firstName} ${person.lastName}`.trim();
}
