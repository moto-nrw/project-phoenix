import type { Teacher } from "./teacher-api";
import { createLogger } from "./logger";
import type { ExternalAccountStatus } from "./staff-api";
import type { WireID } from "./wire-id";
import { toOptionalIdString } from "./wire-id";

const logger = createLogger({ component: "TeachersConfig" });

// Map teacher response from backend to frontend format
export function mapTeacherResponse(data: unknown): Teacher {
  const typedData = data as Record<string, unknown>;

  // Extract person data if nested
  const person = typedData.person as Record<string, unknown> | undefined;

  // Get account_id from either direct data or nested person object
  const accountId =
    toOptionalIdString(typedData.account_id as WireID | undefined) ??
    toOptionalIdString(typedData.accountId as WireID | undefined) ??
    toOptionalIdString(person?.account_id as WireID | undefined) ??
    toOptionalIdString(person?.accountId as WireID | undefined);

  // Get email from either direct data or nested person object
  const email =
    (typedData.email as string | undefined) ??
    (person?.email as string | undefined);

  // The list endpoint (/api/staff) returns camelCase firstName/lastName, while
  // the detail endpoint and nested person objects use snake_case. Accept both
  // so list-side selections render names in the detail panel and edit modal.
  const firstName =
    (typedData.first_name as string | undefined) ??
    (typedData.firstName as string | undefined) ??
    (person?.first_name as string | undefined) ??
    (person?.firstName as string | undefined) ??
    "";
  const lastName =
    (typedData.last_name as string | undefined) ??
    (typedData.lastName as string | undefined) ??
    (person?.last_name as string | undefined) ??
    (person?.lastName as string | undefined) ??
    "";

  // Get tag_id from either direct data or nested person object
  const tagId =
    (typedData.tag_id as string | null | undefined) ??
    (typedData.tagId as string | null | undefined) ??
    (person?.tag_id as string | null | undefined) ??
    (person?.tagId as string | null | undefined);

  // Get avatar from either direct data or nested person object
  const avatar =
    (typedData.avatar as string | null | undefined) ??
    (person?.avatar as string | null | undefined);

  logger.debug("teacher_mapped", {
    teacher_id: typedData.id?.toString(),
    account_id: accountId,
  });

  return {
    id: typedData.id?.toString() ?? "",
    name: (typedData.name as string) ?? `${firstName} ${lastName}`,
    first_name: firstName,
    last_name: lastName,
    email: email,
    avatar: avatar,
    specialization: (typedData.specialization as string) ?? "",
    role: typedData.role as string | null | undefined,
    account_role: typedData.account_role as string | null | undefined,
    qualifications: typedData.qualifications as string | null | undefined,
    tag_id: tagId,
    staff_notes: typedData.staff_notes as string | null | undefined,
    created_at: typedData.created_at as string | undefined,
    updated_at: typedData.updated_at as string | undefined,
    // Decimal string — the id is a bigint and travels back untouched (#2222).
    person_id: toOptionalIdString(typedData.person_id as WireID | undefined),
    account_id: accountId,
    is_teacher: typedData.is_teacher as boolean | undefined,
    // Nur für externe Kräfte (#3823), damit bestehende Formen gleich bleiben.
    ...(typedData.is_external === true
      ? {
          is_external: true,
          external_organization:
            (typedData.external_organization as string | null | undefined) ??
            null,
          // Kontostand für „Kein Konto“ / „Einladung offen“ (#4573).
          external_account_status:
            (typedData.external_account_status as
              ExternalAccountStatus | null | undefined) ?? null,
        }
      : {}),
    staff_id: typedData.staff_id as string | undefined,
    teacher_id: typedData.teacher_id as string | undefined,
    person: person,
  };
}
