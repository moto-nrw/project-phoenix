// Client-seitige API für die Aufsichten im Schul-Portal ("moto schule", #2527).
//
// Gleiche Nutzlast wie die OGS-Aufsicht (lib/timetable-operations-types), aber
// über die Next.js-Routen unter /api/school/*, die mit dem school-Token gegen
// /school/supervisions sprechen. Das Backend beschränkt jede Antwort auf die
// Blöcke, in die diese Lehrkraft im Betreuungsplan eingeteilt ist.

import type {
  AttendancePatchBody,
  BackendStartOperationResult,
  BackendTimetableRoster,
  PlannedTimetableInstance,
  StartOperationResult,
  TimetableRoster,
} from "./timetable-operations-types";
import {
  mapPlannedInstance,
  mapRoster,
  mapStartOperation,
} from "./timetable-operations-types";

interface ApiEnvelope<T> {
  data: T;
}

/** Eine Kontaktperson auf dem Kind-Infoblatt. */
export interface SupervisionContact {
  name: string;
  relationship?: string;
  phone?: string;
  note?: string;
}

/** Abhol- und Notfallinformationen eines Kindes der eigenen Aufsicht. */
export interface SupervisionStudentSheet {
  studentId: string;
  firstName: string;
  lastName: string;
  schoolClass?: string;
  date: string;
  arrival?: string;
  pickup?: string;
  departure: string;
  status?: string;
  pickupContacts: SupervisionContact[];
  emergencyContacts: SupervisionContact[];
}

interface BackendSupervisionStudentSheet {
  student_id: number;
  first_name: string;
  last_name: string;
  school_class?: string;
  date: string;
  arrival?: string;
  pickup?: string;
  departure: string;
  status?: string;
  pickup_contacts: SupervisionContact[];
  emergency_contacts: SupervisionContact[];
}

export class SchoolSupervisionApiError extends Error {
  readonly httpStatus: number;
  readonly code?: string;

  constructor(message: string, httpStatus: number, code?: string) {
    super(message);
    this.name = "SchoolSupervisionApiError";
    this.httpStatus = httpStatus;
    this.code = code;
  }
}

async function unwrap<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `Anfrage fehlgeschlagen (HTTP ${response.status})`;
    let code: string | undefined;
    try {
      const body = (await response.json()) as { error?: string; code?: string };
      if (body.error) message = body.error;
      code = body.code;
    } catch {
      // Allgemeine Meldung behalten.
    }
    throw new SchoolSupervisionApiError(message, response.status, code);
  }
  const envelope = (await response.json()) as ApiEnvelope<T>;
  return envelope.data;
}

function jsonRequest(method: string, body?: unknown): RequestInit {
  return {
    method,
    credentials: "include",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body ?? {}),
  };
}

function mapSheet(raw: BackendSupervisionStudentSheet): SupervisionStudentSheet {
  return {
    studentId: raw.student_id.toString(),
    firstName: raw.first_name,
    lastName: raw.last_name,
    schoolClass: raw.school_class,
    date: raw.date,
    arrival: raw.arrival,
    pickup: raw.pickup,
    departure: raw.departure,
    status: raw.status,
    pickupContacts: raw.pickup_contacts ?? [],
    emergencyContacts: raw.emergency_contacts ?? [],
  };
}

const base = "/api/school/supervisions";

export const schoolSupervisionsApi = {
  /** Die eigenen Aufsichten von heute, in jedem Zustand. */
  async myDay(): Promise<PlannedTimetableInstance[]> {
    const raw = await unwrap<{
      instances: Parameters<typeof mapPlannedInstance>[0][] | null;
    }>(
      await fetch(base, {
        credentials: "include",
        headers: { Accept: "application/json" },
      }),
    );
    return (raw.instances ?? []).map(mapPlannedInstance);
  },

  async roster(instanceId: string): Promise<TimetableRoster> {
    const raw = await unwrap<BackendTimetableRoster>(
      await fetch(`${base}/${instanceId}/roster`, {
        credentials: "include",
        headers: { Accept: "application/json" },
      }),
    );
    return mapRoster(raw);
  },

  async start(instanceId: string): Promise<StartOperationResult> {
    const raw = await unwrap<BackendStartOperationResult>(
      await fetch(`${base}/${instanceId}/start`, jsonRequest("POST")),
    );
    return mapStartOperation(raw);
  },

  async complete(
    instanceId: string,
    confirmedPresentStudentIds: string[],
  ): Promise<void> {
    await unwrap<unknown>(
      await fetch(
        `${base}/${instanceId}/complete`,
        jsonRequest("POST", {
          confirmed_present_student_ids: confirmedPresentStudentIds.map(Number),
        }),
      ),
    );
  },

  async checkIn(
    instanceId: string,
    studentId: string,
  ): Promise<TimetableRoster> {
    const raw = await unwrap<BackendTimetableRoster>(
      await fetch(
        `${base}/${instanceId}/students/${studentId}/check-in`,
        jsonRequest("POST"),
      ),
    );
    return mapRoster(raw);
  },

  async checkOut(
    instanceId: string,
    studentId: string,
  ): Promise<TimetableRoster> {
    const raw = await unwrap<BackendTimetableRoster>(
      await fetch(
        `${base}/${instanceId}/students/${studentId}/check-out`,
        jsonRequest("POST"),
      ),
    );
    return mapRoster(raw);
  },

  async patchAttendance(
    instanceId: string,
    studentId: string,
    body: AttendancePatchBody,
  ): Promise<void> {
    await unwrap<unknown>(
      await fetch(
        `${base}/${instanceId}/students/${studentId}/attendance`,
        jsonRequest("PATCH", body),
      ),
    );
  },

  /** Abhol- und Notfallinformationen eines Kindes. Wird protokolliert. */
  async studentSheet(
    instanceId: string,
    studentId: string,
  ): Promise<SupervisionStudentSheet> {
    const raw = await unwrap<BackendSupervisionStudentSheet>(
      await fetch(`${base}/${instanceId}/students/${studentId}/sheet`, {
        credentials: "include",
        headers: { Accept: "application/json" },
      }),
    );
    return mapSheet(raw);
  },
};
