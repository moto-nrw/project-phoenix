import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-client", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
}));

vi.mock("~/lib/api-helpers", () => ({
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(404)")
        ? 404
        : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url);
}

function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/students/[id]/current-location", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/123/current-location");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("returns not_present when student is at home", async () => {
    const mockStudent = {
      data: {
        data: {
          id: 123,
          current_location: "Zuhause",
          group_id: 5,
          group_name: "OGS A",
        },
      },
    };
    mockApiGet.mockResolvedValueOnce(mockStudent);

    const request = createMockRequest("/api/students/123/current-location");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: {
        status: string;
        location: string;
        isGroupRoom: boolean;
      };
    }>(response);
    expect(json.data.status).toBe("not_present");
    expect(json.data.location).toBe("Zuhause");
    expect(json.data.isGroupRoom).toBe(false);
  });

  it("returns present with room when student is in a room", async () => {
    const mockStudent = {
      data: {
        data: {
          id: 123,
          current_location: "Anwesend",
          group_id: 5,
          group_name: "OGS A",
        },
      },
    };
    const mockGroup = {
      data: {
        data: {
          room_id: 10,
        },
      },
    };
    const mockRoomStatus = {
      data: {
        data: {
          student_room_status: {
            "123": {
              current_room_id: 10,
              check_in_time: "2024-01-15T09:00:00Z",
            },
          },
        },
      },
    };
    const mockRoom = {
      data: {
        data: {
          id: 10,
          name: "Room 101",
          building: "Main",
          floor: 1,
        },
      },
    };

    mockApiGet
      .mockResolvedValueOnce(mockStudent)
      .mockResolvedValueOnce(mockGroup)
      .mockResolvedValueOnce(mockRoomStatus)
      .mockResolvedValueOnce(mockRoom);

    const request = createMockRequest("/api/students/123/current-location");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: {
        status: string;
        room: { id: string; name: string } | null;
        isGroupRoom: boolean;
      };
    }>(response);
    expect(json.data.status).toBe("present");
    expect(json.data.room?.name).toBe("Room 101");
    expect(json.data.isGroupRoom).toBe(true);
  });

  it("returns transit when student is present but not in a room", async () => {
    const mockStudent = {
      data: {
        data: {
          id: 123,
          current_location: "Anwesend",
          group_id: 5,
          group_name: "OGS A",
        },
      },
    };
    const mockGroup = {
      data: {
        data: {
          room_id: 10,
        },
      },
    };
    const mockRoomStatus = {
      data: {
        data: {
          student_room_status: {
            "123": {
              current_room_id: null,
            },
          },
        },
      },
    };

    mockApiGet
      .mockResolvedValueOnce(mockStudent)
      .mockResolvedValueOnce(mockGroup)
      .mockResolvedValueOnce(mockRoomStatus);

    const request = createMockRequest("/api/students/123/current-location");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: {
        status: string;
        location: string;
        room: null;
        isGroupRoom: boolean;
      };
    }>(response);
    expect(json.data.status).toBe("present");
    expect(json.data.location).toBe("Unterwegs");
    expect(json.data.room).toBe(null);
    expect(json.data.isGroupRoom).toBe(false);
  });

  it("returns unknown with error code on network error", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Network Error"));

    const request = createMockRequest("/api/students/123/current-location");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: {
        status: string;
        location: string;
        errorCode?: string;
      };
    }>(response);
    expect(json.data.status).toBe("unknown");
    expect(json.data.location).toBe("Unbekannt");
  });

  it("returns unknown with UNAUTHORIZED error code on 401", async () => {
    const mockError = Object.assign(new Error("Unauthorized"), {
      response: { status: 401 },
      isAxiosError: true,
    });
    mockApiGet.mockRejectedValueOnce(mockError);

    const request = createMockRequest("/api/students/123/current-location");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: {
        status: string;
        errorCode?: string;
      };
    }>(response);
    expect(json.data.status).toBe("unknown");
    expect(json.data.errorCode).toBe("UNAUTHORIZED");
  });

  it("returns transit when student has no group", async () => {
    const mockStudent = {
      data: {
        data: {
          id: 123,
          current_location: "Anwesend",
          group_id: null,
        },
      },
    };

    mockApiGet.mockResolvedValueOnce(mockStudent);

    const request = createMockRequest("/api/students/123/current-location");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: {
        status: string;
        location: string;
        room: null;
        group: null;
      };
    }>(response);
    expect(json.data.status).toBe("present");
    expect(json.data.location).toBe("Unterwegs");
    expect(json.data.room).toBe(null);
    expect(json.data.group).toBe(null);
  });

  it("handles room status fetch failure gracefully", async () => {
    const mockStudent = {
      data: {
        data: {
          id: 123,
          current_location: "Anwesend",
          group_id: 5,
          group_name: "OGS A",
        },
      },
    };
    const mockGroup = {
      data: {
        data: {
          room_id: 10,
        },
      },
    };

    mockApiGet
      .mockResolvedValueOnce(mockStudent)
      .mockResolvedValueOnce(mockGroup)
      .mockRejectedValueOnce(new Error("Room status unavailable"));

    const request = createMockRequest("/api/students/123/current-location");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: {
        status: string;
        location: string;
      };
    }>(response);
    expect(json.data.status).toBe("present");
    expect(json.data.location).toBe("Unterwegs");
  });

  it("returns present with isGroupRoom false when in different room", async () => {
    const mockStudent = {
      data: {
        data: {
          id: 123,
          current_location: "Anwesend",
          group_id: 5,
          group_name: "OGS A",
        },
      },
    };
    const mockGroup = {
      data: {
        data: {
          room_id: 10,
        },
      },
    };
    const mockRoomStatus = {
      data: {
        data: {
          student_room_status: {
            "123": {
              current_room_id: 20,
              check_in_time: "2024-01-15T09:00:00Z",
            },
          },
        },
      },
    };
    const mockRoom = {
      data: {
        data: {
          id: 20,
          name: "Room 201",
          building: "Annex",
          floor: 2,
        },
      },
    };

    mockApiGet
      .mockResolvedValueOnce(mockStudent)
      .mockResolvedValueOnce(mockGroup)
      .mockResolvedValueOnce(mockRoomStatus)
      .mockResolvedValueOnce(mockRoom);

    const request = createMockRequest("/api/students/123/current-location");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: {
        status: string;
        room: { id: string; name: string } | null;
        isGroupRoom: boolean;
      };
    }>(response);
    expect(json.data.status).toBe("present");
    expect(json.data.room?.name).toBe("Room 201");
    expect(json.data.isGroupRoom).toBe(false);
  });
});
