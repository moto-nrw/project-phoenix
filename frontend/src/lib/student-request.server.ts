// What the student BFF routes accept from the browser before anything reaches
// the backend.

import { bffError } from "~/lib/api-response-error.server";
import type { StudentGuardianPayload } from "~/lib/guardian-helpers";
import type { ArrivalScheduleFormEntry } from "~/lib/arrival-schedule-helpers";
import type { BackendPickupScheduleRequest } from "~/lib/pickup-schedule-helpers";
import { prepareStudentForBackend } from "./student-wire";
import type { StudentUpdate } from "./student-helpers";
import type { ConsentPairInput } from "./student-privacy-consent.server";

/** A create also creates guardians and weekly schedules atomically. */
export type StudentCreateBody = StudentUpdate &
  ConsentPairInput & {
    guardians?: StudentGuardianPayload[];
    arrival_schedules?: ArrivalScheduleFormEntry[];
    pickup_schedules?: BackendPickupScheduleRequest[];
  };

export type StudentUpdateBody = StudentUpdate & ConsentPairInput;

const RETIRED_CONTACT_KEYS = new Set([
  "name_lg",
  "contact_lg",
  "guardian_name",
  "guardian_contact",
  "guardian_email",
  "guardian_phone",
]);

/** Guardian contacts moved to the guardian profiles; refuse the old keys. */
export function rejectRetiredStudentContacts(body: object): void {
  for (const key of Object.keys(body)) {
    if (RETIRED_CONTACT_KEYS.has(key.toLowerCase())) {
      throw bffError(
        400,
        `${key} is no longer supported on students; use guardian contacts`,
      );
    }
  }
}

/** The backend body of a create; refuses a body without the required names. */
export function buildStudentCreateRequest(body: StudentCreateBody) {
  rejectRetiredStudentContacts(body);
  const required = [
    {
      field: "first_name",
      label: "First name",
      value: body.first_name?.trim() ?? "",
    },
    {
      field: "last_name",
      label: "Last name",
      value: body.second_name?.trim() ?? "",
    },
    {
      field: "school_class",
      label: "School class",
      value: body.school_class?.trim() ?? "",
    },
  ];
  const missing = required.filter(({ value }) => !value);
  if (missing.length > 0) {
    throw bffError(
      400,
      missing.map(({ label }) => `${label} is required`).join(". "),
      { errors: missing.map(({ field }) => ({ field, reason: "required" })) },
    );
  }

  const [firstName, lastName, schoolClass] = required.map(({ value }) => value);
  return {
    ...prepareStudentForBackend(body),
    first_name: firstName,
    last_name: lastName,
    school_class: schoolClass,
    ...(body.guardians?.length && { guardians: body.guardians }),
    ...(body.arrival_schedules?.length && {
      arrival_schedules: body.arrival_schedules,
    }),
    ...(body.pickup_schedules?.length && {
      pickup_schedules: body.pickup_schedules,
    }),
  };
}
