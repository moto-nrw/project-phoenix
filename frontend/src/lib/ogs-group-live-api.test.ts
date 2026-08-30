/**
 * Tests for ogs-group-live-api.ts (#2056).
 *
 * Covers the wire→view mapping (mapOgsGroupLiveResponse) — array→Map pickup
 * time conversion, day notes, groups mapping, substitution→transfer mapping
 * — and the fetcher (fetchOgsGroupLive), including the 403-with-group_id
 * stale-selection fallback and the non-ok error contract. This coverage
 * used to live inline in page.test.tsx as "BFF pickup times array→Map" /
 * "converts substitutions to active transfers format" style tests; it moved
 * here once the page stopped doing any of this mapping itself (#2056).
 */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { MockInstance } from "vitest";
import {
  mapOgsGroupLiveResponse,
  fetchOgsGroupLive,
} from "./ogs-group-live-api";
import type { OgsGroupLiveWireResponse } from "./ogs-group-live-api";

function wireResponse(
  overrides: Partial<OgsGroupLiveWireResponse> = {},
): OgsGroupLiveWireResponse {
  return {
    groups: [],
    group_id: undefined,
    students: [],
    room_status: {},
    pickup_times: [],
    tracking_indicators: { labels: [], results: {} },
    transfers: [],
    ...overrides,
  };
}

describe("mapOgsGroupLiveResponse", () => {
  it("maps groups and preserves string IDs", () => {
    const view = mapOgsGroupLiveResponse(
      wireResponse({
        groups: [
          {
            id: "1",
            name: "Gruppe A",
            room_id: "10",
            room_name: "Raum 101",
            via_substitution: false,
            is_personal: true,
          },
          {
            id: "2",
            name: "Gruppe B",
            via_substitution: true,
            is_personal: true,
          },
        ],
        group_id: "1",
      }),
    );

    expect(view.groupId).toBe("1");
    expect(view.groups).toEqual([
      {
        id: "1",
        name: "Gruppe A",
        roomId: "10",
        roomName: "Raum 101",
        viaSubstitution: false,
        isPersonal: true,
      },
      {
        id: "2",
        name: "Gruppe B",
        roomId: undefined,
        roomName: undefined,
        viaSubstitution: true,
        isPersonal: true,
      },
    ]);
  });

  it("maps group_id null/undefined to a null groupId (caller has no groups)", () => {
    expect(mapOgsGroupLiveResponse(wireResponse()).groupId).toBeNull();
    expect(
      mapOgsGroupLiveResponse(wireResponse({ group_id: undefined })).groupId,
    ).toBeNull();
  });

  it("passes students through unchanged", () => {
    const students = [
      {
        id: "1",
        first_name: "Max",
        last_name: "Mustermann",
        school_class: "1a",
        current_location: "Raum 101",
        sick: false,
        excused: false,
        class_trip: false,
      },
    ];
    const view = mapOgsGroupLiveResponse(wireResponse({ students }));
    expect(view.students).toBe(students);
  });

  it("defaults room_status to an empty object when absent", () => {
    const view = mapOgsGroupLiveResponse(
      wireResponse({ room_status: undefined as never }),
    );
    expect(view.roomStatus).toEqual({});
  });

  it("converts the pickup_times array to a Map keyed by student id, including day notes", () => {
    const view = mapOgsGroupLiveResponse(
      wireResponse({
        pickup_times: [
          {
            student_id: "1",
            pickup_time: "15:30",
            is_exception: false,
            notes: "Parent pickup",
            day_notes: [
              { id: "5", content: "Arzttermin" },
              { id: "6", content: "Frühabholung" },
            ],
          },
          {
            student_id: "2",
            pickup_time: "16:00",
            is_exception: true,
          },
        ],
      }),
    );

    expect(view.pickupTimes).toBeInstanceOf(Map);
    expect(view.pickupTimes.size).toBe(2);
    expect(view.pickupTimes.get("1")).toEqual({
      pickupTime: "15:30",
      isException: false,
      notes: "Parent pickup",
      dayNotes: [
        { id: "5", content: "Arzttermin" },
        { id: "6", content: "Frühabholung" },
      ],
    });
    expect(view.pickupTimes.get("2")).toEqual({
      pickupTime: "16:00",
      isException: true,
      notes: undefined,
      dayNotes: [],
    });
  });

  it("defaults pickup_times to an empty Map when absent", () => {
    const view = mapOgsGroupLiveResponse(
      wireResponse({ pickup_times: undefined as never }),
    );
    expect(view.pickupTimes).toBeInstanceOf(Map);
    expect(view.pickupTimes.size).toBe(0);
  });

  it("defaults tracking_indicators to empty labels/results when absent", () => {
    const view = mapOgsGroupLiveResponse(
      wireResponse({ tracking_indicators: undefined as never }),
    );
    expect(view.trackingIndicators).toEqual({ labels: [], results: {} });
  });

  it("maps transfers to the GroupTransfer view shape", () => {
    const view = mapOgsGroupLiveResponse(
      wireResponse({
        transfers: [
          {
            id: "100",
            group_id: "1",
            substitute_staff_id: "200",
            substitute_name: "Anna Lehrer",
            end_date: "2024-01-20",
          },
          {
            id: "101",
            group_id: "1",
            substitute_staff_id: "201",
            end_date: "2024-01-21",
          },
        ],
      }),
    );

    expect(view.transfers).toEqual([
      {
        substitutionId: "100",
        groupId: "1",
        targetStaffId: "200",
        targetName: "Anna Lehrer",
        validUntil: "2024-01-20",
      },
      {
        substitutionId: "101",
        groupId: "1",
        targetStaffId: "201",
        targetName: "Unbekannt",
        validUntil: "2024-01-21",
      },
    ]);
  });

  it("defaults transfers to an empty array when absent", () => {
    const view = mapOgsGroupLiveResponse(
      wireResponse({ transfers: undefined as never }),
    );
    expect(view.transfers).toEqual([]);
  });

  it("preserves IDs above JavaScript's safe-integer limit", () => {
    const id = "9007199254740993";
    const view = mapOgsGroupLiveResponse(
      wireResponse({
        groups: [
          {
            id,
            name: "Große IDs",
            room_id: id,
            via_substitution: false,
            is_personal: true,
          },
        ],
        group_id: id,
        students: [
          {
            id,
            first_name: "Max",
            last_name: "Mustermann",
            school_class: "1a",
            current_location: "Raum 1",
            sick: false,
            excused: false,
            class_trip: false,
          },
        ],
        room_status: {
          [id]: { in_group_room: true, current_room_id: id },
        },
        pickup_times: [
          {
            student_id: id,
            is_exception: false,
            day_notes: [{ id, content: "Hinweis" }],
          },
        ],
        tracking_indicators: {
          labels: ["Hausaufgaben"],
          results: { [id]: [true] },
        },
        transfers: [
          {
            id,
            group_id: id,
            substitute_staff_id: id,
            end_date: "2026-08-01",
          },
        ],
      }),
    );

    expect(view.groupId).toBe(id);
    expect(view.groups[0]).toMatchObject({ id, roomId: id });
    expect(view.students[0]?.id).toBe(id);
    expect(view.roomStatus[id]?.current_room_id).toBe(id);
    expect(view.pickupTimes.get(id)?.dayNotes[0]?.id).toBe(id);
    expect(view.trackingIndicators.results[id]).toEqual([true]);
    expect(view.transfers[0]).toMatchObject({
      substitutionId: id,
      groupId: id,
      targetStaffId: id,
    });
  });
});

describe("fetchOgsGroupLive", () => {
  let fetchMock: MockInstance<typeof fetch>;

  beforeEach(() => {
    fetchMock = vi.spyOn(globalThis, "fetch");
  });

  afterEach(() => {
    fetchMock.mockRestore();
  });

  it("GETs /api/ogs-group-live with the token and group_id query param", async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ success: true, data: wireResponse() }),
    } as Response);

    await fetchOgsGroupLive("test-token", "5");

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/ogs-group-live?group_id=5");
    expect(init).toMatchObject({
      credentials: "include",
      headers: {
        Authorization: "Bearer test-token",
        "Content-Type": "application/json",
      },
    });
  });

  it("omits the group_id query param when groupId is null", async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ success: true, data: wireResponse() }),
    } as Response);

    await fetchOgsGroupLive("test-token", null);

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/ogs-group-live",
      expect.any(Object),
    );
  });

  it.each(["not-a-number", "0", "-3", "9223372036854775808"])(
    "omits invalid group_id %s and loads the default group",
    async (invalidGroupId) => {
      fetchMock.mockResolvedValue({
        ok: true,
        json: async () => ({ success: true, data: wireResponse() }),
      } as Response);

      await fetchOgsGroupLive("test-token", invalidGroupId);

      expect(fetchMock).toHaveBeenCalledTimes(1);
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/ogs-group-live",
        expect.any(Object),
      );
    },
  );

  it("retries once without group_id on a 403 for a specific group (stale selection)", async () => {
    fetchMock
      .mockResolvedValueOnce({ ok: false, status: 403 } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          success: true,
          data: wireResponse({ group_id: "1" }),
        }),
      } as Response);

    const result = await fetchOgsGroupLive("test-token", "999");

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/ogs-group-live?group_id=999",
    );
    expect(fetchMock.mock.calls[1]?.[0]).toBe("/api/ogs-group-live");
    expect(result.groupId).toBe("1");
  });

  it("does not retry a 403 when no group_id was requested", async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 403 } as Response);

    await expect(fetchOgsGroupLive("test-token", null)).rejects.toThrow(
      "API error: 403",
    );
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("throws 'API error: <status>' for a non-ok response", async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 500 } as Response);

    await expect(fetchOgsGroupLive("test-token", null)).rejects.toThrow(
      "API error: 500",
    );
  });
});
