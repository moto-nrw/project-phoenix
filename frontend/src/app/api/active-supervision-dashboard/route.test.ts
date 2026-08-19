/**
 * Tests for the Active Supervision Dashboard proxy route (#2096).
 *
 * The former BFF fan-out (~11 backend calls, silent-empty fallbacks) moved
 * into the aggregated Go endpoint /api/active/supervision-dashboard; its
 * semantics are pinned by backend tests. This suite covers the thin proxy:
 * one backend call, group_id forwarding, wire→camelCase mapping, and error
 * propagation (no swallowed sub-loads).
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";
import { mockSessionData } from "~/test/mocks/next-auth";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(404)")
        ? 404
        : message.includes("(403)")
          ? 403
          : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url);
}

function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}

const defaultSession = mockSessionData() as ExtendedSession;

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

const emptyWire = {
  groups: [],
  unclaimed_groups: [],
  educational_groups: [],
  schulhof_status: null,
  capabilities: { web_spontaneous_activities_enabled: false },
  active_sessions: [],
  planned_now: [],
  visits: [],
  tracking_indicators: { labels: [], results: {} },
  pickup_times: [],
  arrival_times: [],
};

describe("GET /api/active-supervision-dashboard", () => {
  beforeEach(() => {
    mockApiGet.mockReset();
    mockAuth.mockReset();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await GET(
      createMockRequest("/api/active-supervision-dashboard"),
      createMockContext(),
    );

    expect(response.status).toBe(401);
  });

  it("issues exactly one backend request without group_id", async () => {
    mockApiGet.mockResolvedValueOnce({ data: emptyWire });

    const response = await GET(
      createMockRequest("/api/active-supervision-dashboard"),
      createMockContext(),
    );

    expect(response.status).toBe(200);
    expect(mockApiGet).toHaveBeenCalledTimes(1);
    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/supervision-dashboard",
      expect.any(String),
    );
  });

  it("forwards a valid group_id and drops an invalid one", async () => {
    mockApiGet.mockResolvedValue({ data: emptyWire });

    await GET(
      createMockRequest("/api/active-supervision-dashboard?group_id=42"),
      createMockContext(),
    );
    expect(mockApiGet).toHaveBeenLastCalledWith(
      "/api/active/supervision-dashboard?group_id=42",
      expect.any(String),
    );

    await GET(
      createMockRequest("/api/active-supervision-dashboard?group_id=abc"),
      createMockContext(),
    );
    expect(mockApiGet).toHaveBeenLastCalledWith(
      "/api/active/supervision-dashboard",
      expect.any(String),
    );
  });

  it("maps the wire projection to the camelCase dashboard shape", async () => {
    mockApiGet.mockResolvedValueOnce({
      data: {
        ...emptyWire,
        groups: [
          {
            id: "7",
            room_id: "10",
            room_name: "Raum 101",
            room_color: "#83CD2D",
          },
        ],
        selected_group_id: "7",
        unclaimed_groups: [{ id: "9", room_name: "Schulhof" }],
        current_staff_id: "5",
        educational_groups: [{ id: "2", name: "OGS A", room_name: "Raum 101" }],
        schulhof_status: {
          exists: true,
          room_id: 30,
          room_name: "Schulhof",
          activity_group_id: 31,
          active_group_id: 32,
          is_user_supervising: true,
          supervision_id: 33,
          supervisor_count: 1,
          student_count: 4,
          supervisors: [
            { id: 33, staff_id: 5, name: "Max Muster", is_current_user: true },
          ],
        },
        capabilities: { web_spontaneous_activities_enabled: true },
        active_sessions: [
          {
            active_group_id: 7,
            instance_id: 70,
            title: "Fußball",
            start_time: "14:00",
            end_time: "15:00",
          },
        ],
        visits: [
          {
            student_id: "100",
            student_name: "Kind Eins",
            school_class: "1a",
            group_name: "OGS A",
            active_group_id: "7",
            check_in_time: "2026-08-19T10:00:00Z",
            actual_arrival_time: "12:00",
            sick: false,
            excused: false,
          },
        ],
        tracking_indicators: {
          labels: ["Hausaufgaben"],
          results: { "100": [true] },
        },
        pickup_times: [
          {
            student_id: "100",
            date: "2026-08-19",
            weekday_name: "Mittwoch",
            pickup_time: "15:30",
            is_exception: true,
            day_notes: [{ id: "1", content: "Oma holt ab" }],
          },
        ],
        arrival_times: [
          {
            student_id: "100",
            date: "2026-08-19",
            weekday_name: "Mittwoch",
            expected_arrival: "11:45",
            is_exception: false,
          },
        ],
      },
    });

    const response = await GET(
      createMockRequest("/api/active-supervision-dashboard"),
      createMockContext(),
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<
      ApiResponse<{
        supervisedGroups: Array<{
          id: string;
          room_id?: string;
          room?: { id: string; name: string; color?: string | null };
        }>;
        unclaimedGroups: Array<{ id: string; room?: { name: string } }>;
        currentStaff: { id: string } | null;
        educationalGroups: Array<{ id: string; name: string }>;
        firstRoomVisits: Array<{
          studentId: string;
          studentName: string;
          activeGroupId: string;
          isActive: boolean;
          actualArrivalTime?: string;
        }>;
        firstRoomId: string | null;
        selectedGroupId: string | null;
        schulhofStatus: {
          exists: boolean;
          activeGroupId: string | null;
          supervisors: Array<{ staffId: string; isCurrentUser: boolean }>;
        } | null;
        capabilities?: { webSpontaneousActivitiesEnabled: boolean };
        activeSessions: Array<{ activeGroupId: string; title: string }>;
        trackingIndicators: {
          labels: string[];
          results: Record<string, boolean[]>;
        };
        pickupTimes: Array<{
          studentId: string;
          pickupTime: string | null;
          dayNotes: Array<{ content: string }>;
        }>;
        arrivalTimes: Array<{
          studentId: string;
          expectedArrival: string | null;
        }>;
      }>
    >(response);

    const data = json.data;
    expect(data.supervisedGroups).toEqual([
      {
        id: "7",
        name: "",
        room_id: "10",
        room: { id: "10", name: "Raum 101", color: "#83CD2D" },
      },
    ]);
    expect(data.unclaimedGroups[0]).toMatchObject({
      id: "9",
      room: { name: "Schulhof" },
    });
    expect(data.currentStaff).toEqual({ id: "5" });
    expect(data.educationalGroups[0]).toMatchObject({ id: "2", name: "OGS A" });
    expect(data.firstRoomVisits[0]).toMatchObject({
      studentId: "100",
      studentName: "Kind Eins",
      activeGroupId: "7",
      isActive: true,
      actualArrivalTime: "12:00",
    });
    expect(data.firstRoomId).toBe("7");
    expect(data.selectedGroupId).toBe("7");
    expect(data.schulhofStatus).toMatchObject({
      exists: true,
      activeGroupId: "32",
      supervisors: [{ staffId: "5", isCurrentUser: true }],
    });
    expect(data.capabilities?.webSpontaneousActivitiesEnabled).toBe(true);
    expect(data.activeSessions[0]).toMatchObject({
      activeGroupId: "7",
      title: "Fußball",
    });
    expect(data.trackingIndicators).toEqual({
      labels: ["Hausaufgaben"],
      results: { "100": [true] },
    });
    expect(data.pickupTimes[0]).toMatchObject({
      studentId: "100",
      pickupTime: "15:30",
      dayNotes: [{ id: "1", content: "Oma holt ab" }],
    });
    expect(data.arrivalTimes[0]).toMatchObject({
      studentId: "100",
      expectedArrival: "11:45",
    });
  });

  it("propagates backend errors instead of degrading to empty sections", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("API error (403): forbidden"));

    const response = await GET(
      createMockRequest("/api/active-supervision-dashboard?group_id=42"),
      createMockContext(),
    );

    expect(response.status).toBe(403);
  });

  it("propagates backend 500s", async () => {
    mockApiGet.mockRejectedValueOnce(
      new Error("API error (500): schulhof load failed"),
    );

    const response = await GET(
      createMockRequest("/api/active-supervision-dashboard"),
      createMockContext(),
    );

    expect(response.status).toBe(500);
  });
});
