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

  it("surfaces an aggregate failure instead of returning partial empty data", async () => {
    mockApiGet.mockRejectedValueOnce(new ApiResponseError(500, "failed"));

    const request = createMockRequest("/api/user-context");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
