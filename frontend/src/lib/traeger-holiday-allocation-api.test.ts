import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  decideHolidayAllocation,
  fetchHolidayAllocation,
  saveHolidayCriteria,
} from "./traeger-holiday-allocation-api";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const BACKEND_BOARD = {
  allocation_mode: "criteria",
  criteria: [
    { id: 1, name: "Beide Eltern berufstätig", scope: "family", position: 1 },
  ],
  weeks: [
    {
      week_id: 51,
      starts_on: "2027-07-05",
      ends_on: "2027-07-09",
      host_school_id: 31,
      booking_unit: "week",
      capacity: 2,
      placed: 1,
      free: 1,
      pending: [
        {
          booking_id: 100,
          rank: 1,
          registration_id: 7,
          submitted_at: "2026-09-01T08:00:00Z",
          child_id: 9,
          child_first_name: "Mia",
          child_last_name: "Muster",
          time_model_name: "bis 13 Uhr",
          days: null,
          criteria: [{ criterion_id: 1, value: "unknown", source: "parent" }],
          proposal: "placed",
        },
      ],
    },
  ],
  decisions: [
    {
      id: 3,
      week_id: 51,
      week_starts_on: "2027-07-05",
      week_ends_on: "2027-07-09",
      registration_id: 8,
      child_id: 10,
      child_first_name: "Ben",
      child_last_name: "Beispiel",
      placement: "placed",
      proposal: "waitlisted",
      rank: 2,
      criteria_met: null,
      reason: "Härtefall",
      decided_at: "2026-09-02T08:00:00Z",
      decided_by_account_id: 4,
    },
  ],
};

describe("traeger holiday allocation api (#4700)", () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => vi.unstubAllGlobals());

  it("maps the allocation list with IDs as strings", async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { data: BACKEND_BOARD }));
    const board = await fetchHolidayAllocation("5");
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/traeger/holiday-periods/5/allocation",
    );
    expect(board.weeks[0]?.pending[0]).toEqual({
      bookingId: "100",
      rank: 1,
      registrationId: "7",
      submittedAt: "2026-09-01T08:00:00Z",
      childId: "9",
      childFirstName: "Mia",
      childLastName: "Muster",
      timeModelName: "bis 13 Uhr",
      days: [],
      criteria: [{ criterionId: "1", value: "unknown", source: "parent" }],
      proposal: "placed",
    });
    expect(board.decisions[0]).toMatchObject({
      id: "3",
      placement: "placed",
      proposal: "waitlisted",
      criteriaMet: [],
      reason: "Härtefall",
    });
  });

  it("sends decisions and criteria with numeric IDs", async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { data: BACKEND_BOARD }));
    await decideHolidayAllocation("5", [
      {
        bookingId: "100",
        placement: "waitlisted",
        proposal: "placed",
        reason: "Grund",
      },
    ]);
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/traeger/holiday-periods/5/allocation/decisions");
    expect(init?.method).toBe("POST");
    expect(JSON.parse(String(init?.body))).toEqual({
      decisions: [
        {
          booking_id: 100,
          placement: "waitlisted",
          proposal: "placed",
          reason: "Grund",
        },
      ],
    });

    fetchMock.mockResolvedValue(jsonResponse(200, { data: { criteria: [] } }));
    await saveHolidayCriteria([
      {
        id: "",
        name: "Neu",
        question: "Trifft das zu?",
        scope: "child",
        active: true,
      },
      {
        id: "4",
        name: "Alt",
        question: "Und das?",
        scope: "family",
        active: false,
      },
    ]);
    const [, saveInit] = fetchMock.mock.calls[1]!;
    expect(saveInit?.method).toBe("PUT");
    expect(JSON.parse(String(saveInit?.body))).toEqual({
      criteria: [
        {
          id: 0,
          name: "Neu",
          question: "Trifft das zu?",
          scope: "child",
          active: true,
        },
        {
          id: 4,
          name: "Alt",
          question: "Und das?",
          scope: "family",
          active: false,
        },
      ],
    });
  });
});
