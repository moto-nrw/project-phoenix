import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, POST } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: mockApiPost,
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
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

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    {
      method: options.method ?? "GET",
    };

  if (options.body) {
    requestInit.body = JSON.stringify(options.body);
    requestInit.headers = { "Content-Type": "application/json" };
  }

  return new NextRequest(url, requestInit);
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

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/time-tracking/absences", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/time-tracking/absences");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches absences for date range", async () => {
    const mockAbsences = [
      {
        id: 1,
        absence_type: "vacation",
        date_start: "2024-01-15",
        date_end: "2024-01-20",
      },
      {
        id: 2,
        absence_type: "sick",
        date_start: "2024-01-22",
        date_end: "2024-01-22",
        half_day: true,
      },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockAbsences });

    const request = createMockRequest(
      "/api/time-tracking/absences?from=2024-01-01&to=2024-01-31",
    );
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/time-tracking/absences?from=2024-01-01&to=2024-01-31",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockAbsences>>(response);
    expect(json.data).toEqual(mockAbsences);
  });
});

describe("POST /api/time-tracking/absences", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/time-tracking/absences", {
      method: "POST",
      body: {
        absence_type: "vacation",
        date_start: "2024-01-15",
        date_end: "2024-01-20",
      },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("creates a new absence", async () => {
    const createRequest = {
      absence_type: "vacation",
      date_start: "2024-01-15",
      date_end: "2024-01-20",
      note: "Summer vacation",
    };
    const mockCreatedAbsence = {
      id: 99,
      absence_type: "vacation",
      date_start: "2024-01-15",
      date_end: "2024-01-20",
      note: "Summer vacation",
    };
    mockApiPost.mockResolvedValueOnce({ data: mockCreatedAbsence });

    const request = createMockRequest("/api/time-tracking/absences", {
      method: "POST",
      body: createRequest,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/time-tracking/absences",
      "test-token",
      createRequest,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockCreatedAbsence>>(response);
    expect(json.data).toEqual(mockCreatedAbsence);
  });
});
