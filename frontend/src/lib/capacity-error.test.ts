import { describe, expect, it } from "vitest";

import {
  ACTIVITY_PARTICIPANT_LIMIT_CODE,
  capacityErrorMessage,
  ROOM_CAPACITY_CODE,
} from "./capacity-error";

function codedError(code: string, details?: unknown) {
  return Object.assign(new Error("HTTP 409"), { status: 409, code, details });
}

describe("capacityErrorMessage", () => {
  it("names a full activity, its occupancy and where to change the limit", () => {
    const error = codedError(ACTIVITY_PARTICIPANT_LIMIT_CODE, {
      activity_id: 7,
      activity_name: "Betreuung",
      current_occupancy: 45,
      max_participants: 45,
      incoming_students: 1,
    });

    expect(capacityErrorMessage(error)).toBe(
      "Die Aktivität „Betreuung“ ist voll (45 von 45 Kindern). Die Grenze ändern Sie unter Datenverwaltung → Aktivitäten bei „Maximale Teilnehmer“.",
    );
  });

  it("names a full room, its occupancy and where to change the capacity", () => {
    const error = codedError(ROOM_CAPACITY_CODE, {
      room_id: 12,
      room_name: "Turnhalle",
      current_occupancy: 30,
      max_capacity: 30,
      incoming_students: 1,
    });

    expect(capacityErrorMessage(error)).toBe(
      "Der Raum „Turnhalle“ ist voll (30 von 30 Plätzen). Die Grenze ändern Sie unter Datenverwaltung → Räume bei „Maximale Belegung“.",
    );
  });

  it("says how many places are left when a group of children does not fit", () => {
    const activity = codedError(ACTIVITY_PARTICIPANT_LIMIT_CODE, {
      activity_name: "Fußball",
      current_occupancy: 43,
      max_participants: 45,
      incoming_students: 3,
    });
    const room = codedError(ROOM_CAPACITY_CODE, {
      room_name: "Mensa",
      current_occupancy: 29,
      max_capacity: 30,
      incoming_students: 2,
    });

    expect(capacityErrorMessage(activity)).toBe(
      "In der Aktivität „Fußball“ sind nur noch 2 Plätze frei (43 von 45 Kindern). Es sollen 3 Kinder dazukommen. Die Grenze ändern Sie unter Datenverwaltung → Aktivitäten bei „Maximale Teilnehmer“.",
    );
    expect(capacityErrorMessage(room)).toBe(
      "Im Raum „Mensa“ ist nur noch 1 Platz frei (29 von 30 Plätzen). Es sollen 2 Kinder dazukommen. Die Grenze ändern Sie unter Datenverwaltung → Räume bei „Maximale Belegung“.",
    );
  });

  it("stays clear about room or activity without details", () => {
    expect(capacityErrorMessage(codedError(ROOM_CAPACITY_CODE))).toBe(
      "Der Raum ist voll. Die Grenze ändern Sie unter Datenverwaltung → Räume bei „Maximale Belegung“.",
    );
    expect(
      capacityErrorMessage(codedError(ACTIVITY_PARTICIPANT_LIMIT_CODE)),
    ).toBe(
      "Die Aktivität ist voll. Die Grenze ändern Sie unter Datenverwaltung → Aktivitäten bei „Maximale Teilnehmer“.",
    );
  });

  it("reads code and details from the raw response body", () => {
    const error = Object.assign(new Error("HTTP 409"), {
      status: 409,
      body: JSON.stringify({
        error: "room capacity exceeded: Turnhalle (30/30)",
        code: ROOM_CAPACITY_CODE,
        details: {
          room_name: "Turnhalle",
          current_occupancy: 30,
          max_capacity: 30,
        },
      }),
    });

    expect(capacityErrorMessage(error)).toContain(
      "Der Raum „Turnhalle“ ist voll (30 von 30 Plätzen).",
    );
  });

  it("points to the OGS where the reader cannot change the limit", () => {
    const error = codedError(ACTIVITY_PARTICIPANT_LIMIT_CODE, {
      activity_name: "Betreuung",
      current_occupancy: 1,
      max_participants: 1,
    });

    expect(capacityErrorMessage(error, { canChangeLimit: false })).toBe(
      "Die Aktivität „Betreuung“ ist voll (1 von 1 Kind). Mehr Plätze kann die OGS freigeben.",
    );
  });

  it("ignores every other error, whatever its text says", () => {
    expect(capacityErrorMessage(codedError("room_not_released"))).toBeNull();
    expect(
      capacityErrorMessage(new Error("room capacity exceeded: Turnhalle")),
    ).toBeNull();
    expect(capacityErrorMessage(undefined)).toBeNull();
  });
});
