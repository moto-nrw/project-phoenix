import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET } from "./route";
import { ApiResponseError } from "~/lib/api-response-error.server";
import { mockServerSessionData } from "~/test/mocks/next-auth";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
} from "~/test/route-test-helpers";

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn(
    (await import("~/test/helpers/handle-api-error")).mockHandleApiError,
  ),
}));

const defaultSession = mockServerSessionData() as ExtendedSession;

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/user-context", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("maps backend data into user context response", async () => {
    const groups = [
      {
        id: 1,
        name: "Group A",
        room_id: 10,
        room: { id: 10, name: "Room 10" },
        via_substitution: true,
      },
    ];
    const supervised = [
      {
        id: 2,
        name: "Group B",
        room_id: 11,
        room: { id: 11, name: "Room 11" },
      },
    ];
    const staff = { id: 5, person_id: 7 };

    mockApiGet.mockResolvedValueOnce({
      data: {
        educational_groups: groups,
        supervised_groups: supervised,
        current_staff: staff,
        incomplete: true,
        unavailable_sections: ["supervised_groups"],
      },
    });

    const request = createMockRequest("/api/user-context");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledTimes(1);
    expect(mockApiGet).toHaveBeenCalledWith("/api/me/navigation", "test-token");

    const json = await parseJsonResponse<
      ApiResponse<{
        educationalGroups: Array<{
          id: string;
          name: string;
          roomId?: string;
          room?: { id: string; name: string };
          viaSubstitution?: boolean;
        }>;
        supervisedGroups: Array<{
          id: string;
          name: string;
          roomId?: string;
          room?: { id: string; name: string };
        }>;
        currentStaff: { id: string; personId: string } | null;
        educationalGroupIds: string[];
        educationalGroupRoomNames: string[];
        supervisedRoomNames: string[];
        incomplete: boolean;
        unavailableSections: string[];
      }>
    >(response);

    expect(json.success).toBe(true);
    expect(json.data.educationalGroups).toEqual([
      {
        id: "1",
        name: "Group A",
        roomId: "10",
        room: { id: "10", name: "Room 10" },
        viaSubstitution: true,
      },
    ]);
    expect(json.data.supervisedGroups).toEqual([
      {
        id: "2",
        name: "Group B",
        roomId: "11",
        room: { id: "11", name: "Room 11" },
      },
    ]);
    expect(json.data.currentStaff).toEqual({ id: "5", personId: "7" });
    expect(json.data.educationalGroupIds).toEqual(["1"]);
    expect(json.data.educationalGroupRoomNames).toEqual(["Room 10"]);
    expect(json.data.supervisedRoomNames).toEqual(["Room 11"]);
    expect(json.data.incomplete).toBe(true);
    expect(json.data.unavailableSections).toEqual(["supervised_groups"]);
  });

  it("maps a null supervised group list to an empty response list", async () => {
    mockApiGet.mockResolvedValueOnce({
      data: {
        educational_groups: [],
        supervised_groups: null,
        current_staff: null,
        incomplete: false,
        unavailable_sections: [],
      },
    });

    const response = await GET(
      createMockRequest("/api/user-context"),
      createMockContext(),
    );
    const json =
      await parseJsonResponse<ApiResponse<{ supervisedGroups: unknown[] }>>(
        response,
      );

    expect(response.status).toBe(200);
    expect(json.data.supervisedGroups).toEqual([]);
  });

  // The signed-in app shell reads these school switches here instead of the
  // settings schema (#4996).
  it.each([
    {
      name: "maps the school switches",
      wire: {
        parent_news_enabled: true,
        parent_invite_mode: "staff_approval",
        timetable_day_start_time: "07:30",
        timetable_day_end_time: "16:15",
      },
      expected: {
        parentNewsEnabled: true,
        parentInviteMode: "staff_approval",
        timetableDayStartTime: "07:30",
        timetableDayEndTime: "16:15",
      },
    },
    {
      name: "keeps an unknown invite mode undefined instead of guessing",
      wire: {
        parent_news_enabled: false,
        parent_invite_mode: "nonsense_from_a_newer_backend",
        timetable_day_start_time: "09:00",
        timetable_day_end_time: "17:00",
      },
      expected: {
        parentNewsEnabled: false,
        parentInviteMode: undefined,
        timetableDayStartTime: "09:00",
        timetableDayEndTime: "17:00",
      },
    },
    { name: "maps missing switches to null", wire: undefined, expected: null },
  ])("$name", async ({ wire, expected }) => {
    mockApiGet.mockResolvedValueOnce({
      data: {
        educational_groups: [],
        supervised_groups: [],
        current_staff: null,
        incomplete: false,
        unavailable_sections: [],
        ...(wire ? { school_switches: wire } : {}),
      },
    });

    const response = await GET(
      createMockRequest("/api/user-context"),
      createMockContext(),
    );
    const json =
      await parseJsonResponse<ApiResponse<{ schoolSwitches: unknown }>>(
        response,
      );

    expect(response.status).toBe(200);
    expect(json.data.schoolSwitches).toEqual(expected);
  });

  it("surfaces an aggregate failure instead of returning partial empty data", async () => {
    mockApiGet.mockRejectedValueOnce(new ApiResponseError(500, "failed"));

    const request = createMockRequest("/api/user-context");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
