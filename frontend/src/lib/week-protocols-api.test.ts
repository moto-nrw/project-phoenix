import { beforeEach, describe, expect, it, vi } from "vitest";

const mockSessionFetch = vi.hoisted(() => vi.fn());

vi.mock("./session-cache", () => ({ sessionFetch: mockSessionFetch }));

import {
  weekProtocolPath,
  weekProtocolsPath,
  weekProtocolsService,
} from "./week-protocols-api";

const backendProtocol = {
  id: "41",
  scope: "group",
  group_id: "7",
  group_name: "Bären",
  week_start: "2026-09-07",
  week_end: "2026-09-13",
  status: "draft",
  draft_from: "2026-09-11",
  auto_close_on: "2026-09-21",
  week_note: "",
  closed_automatically: false,
  days: [
    {
      date: "2026-09-07",
      captured: true,
      kind: "care",
      present: 9,
      sick: 1,
      excused: 0,
      class_trip: 2,
      absent: 1,
      missing: [
        { student_id: "1", name: "Mia Kurz", reason: "sick", label: "Krank" },
      ],
      blocks: [
        {
          title: "Fußball",
          start_time: "14:00",
          end_time: "15:00",
          status: "cancelled",
          cancel_reason: "Regen",
        },
      ],
      notices: [{ title: "Elternabend" }],
      note: "",
    },
    {
      date: "2026-09-08",
      captured: false,
      present: 0,
      sick: 0,
      excused: 0,
      class_trip: 0,
      absent: 0,
      missing: [],
      blocks: [],
      notices: [],
      note: "Ausflug",
    },
  ],
};

function ok(data: unknown): Response {
  return new Response(JSON.stringify({ data }), { status: 200 });
}

describe("weekProtocolsService", () => {
  beforeEach(() => {
    mockSessionFetch.mockReset();
  });

  it("maps a protocol with its days", async () => {
    mockSessionFetch.mockResolvedValue(ok(backendProtocol));
    const protocol = await weekProtocolsService.get("41");
    expect(mockSessionFetch).toHaveBeenCalledWith("/api/week-protocols/41");
    expect(protocol).toMatchObject({
      groupId: "7",
      groupName: "Bären",
      status: "draft",
      autoCloseOn: "2026-09-21",
      closedAt: null,
      closedByName: "",
    });
    expect(protocol.days[0]).toMatchObject({
      kind: "care",
      classTrip: 2,
      missing: [{ studentId: "1", label: "Krank" }],
      blocks: [{ startTime: "14:00", cancelReason: "Regen" }],
      notices: [{ title: "Elternabend", body: "" }],
    });
    expect(protocol.days[1]).toMatchObject({
      captured: false,
      kind: null,
      note: "Ausflug",
    });
  });

  it("lists the protocols of one group", async () => {
    mockSessionFetch.mockResolvedValue(ok([backendProtocol]));
    const list = await weekProtocolsService.list("7");
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/week-protocols?group_id=7",
    );
    expect(list).toHaveLength(1);
  });

  it("sends the notes as days with their date", async () => {
    mockSessionFetch.mockResolvedValue(ok(backendProtocol));
    await weekProtocolsService.saveNotes("41", {
      weekNote: "Ruhig",
      days: { "2026-09-07": "Elternabend" },
    });
    const [url, init] = mockSessionFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/week-protocols/41/notes");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body as string)).toEqual({
      week_note: "Ruhig",
      days: [{ date: "2026-09-07", note: "Elternabend" }],
    });
  });

  it("throws the backend error of a refused close", async () => {
    mockSessionFetch.mockResolvedValue(
      new Response(
        JSON.stringify({
          status: 409,
          code: "timetable.week_protocol_not_draft",
          detail: "protocol is not a draft",
        }),
        {
          status: 409,
          headers: { "Content-Type": "application/problem+json" },
        },
      ),
    );
    await expect(weekProtocolsService.close("41")).rejects.toMatchObject({
      status: 409,
    });
  });
});

describe("week protocol paths", () => {
  it("links the list of a group and one protocol", () => {
    expect(weekProtocolsPath("7")).toBe("/wochenprotokolle?group=7");
    expect(weekProtocolsPath(null)).toBe("/wochenprotokolle");
    expect(weekProtocolPath("41")).toBe("/wochenprotokolle/41");
  });
});
