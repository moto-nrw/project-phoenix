import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, POST } from "./route";
import { suppressConsole } from "~/test/helpers/console";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", () => ({
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
        : message.includes("(429)")
          ? 429
          : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

vi.mock("~/lib/student-privacy-helpers", () => ({
  shouldCreatePrivacyConsent: vi.fn(() => true),
  updatePrivacyConsent: vi.fn(),
  fetchPrivacyConsent: vi.fn(() =>
    Promise.resolve({
      privacy_consent_accepted: true,
      data_retention_days: 30,
    }),
  ),
}));

vi.mock("~/lib/student-request-helpers", () => ({
  validateStudentFields: vi.fn((body: Record<string, unknown>) => body),
  parseGuardianContact: vi.fn(() => "test@example.com"),
  buildBackendStudentRequest: vi.fn(
    (
      validated: {
        first_name: string;
        last_name: string;
        school_class: string;
      },
      _body: Record<string, unknown>,
      guardianContact: string,
    ) => ({
      first_name: validated.first_name,
      last_name: validated.last_name,
      school_class: validated.school_class,
      guardian_contact: guardianContact,
    }),
  ),
  handlePrivacyConsentCreation: vi.fn(() => Promise.resolve()),
  buildStudentResponse: vi.fn((mapped: Record<string, unknown>) =>
    Promise.resolve(mapped),
  ),
  handleStudentCreationError: vi.fn((error: unknown) => {
    throw error;
  }),
}));

// ============================================================================
// Test Helpers
// ============================================================================

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

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/students", () => {
  const consoleSpies = suppressConsole("warn");

  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches paginated students from backend", async () => {
    const mockResponse = {
      data: [
        {
          id: 1,
          person_id: 10,
          first_name: "Alice",
          last_name: "Smith",
          school_class: "1a",
          guardian_name: "Jane Smith",
          guardian_contact: "jane@example.com",
          bus: false,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
      ],
      pagination: {
        current_page: 1,
        page_size: 1000,
        total_pages: 1,
        total_records: 1,
      },
    };
    mockApiGet.mockResolvedValueOnce(mockResponse);

    const request = createMockRequest("/api/students");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students?page_size=1000",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      data: {
        data: unknown[];
        pagination: unknown;
      };
    }>(response);
    expect(json.data.data).toHaveLength(1);
    expect(json.data.pagination).toEqual(mockResponse.pagination);
  });

  it("keeps slim responses slim after mapping them for the browser", async () => {
    const mockResponse = {
      data: [
        {
          id: 7,
          first_name: "Anna",
          last_name: "Muster",
          school_class: "2a",
          current_location: "Raum 3",
          location_since: "2026-08-03T08:00:00Z",
          current_room_color: "#83CD2D",
          group_id: 9,
          group_name: "Füchse",
          sick: false,
          excused: false,
          class_trip: false,
          pickup_time: "15:30",
          companion_student_ids: [8],
          photo_consent_given: true,
          has_full_access: true,
          // A defensive probe: even an unexpectedly wide backend response
          // must not make the proxy synthesize or forward full-detail fields.
          bus_days: { mon: true },
          guardian_name: "Nicht für die Liste",
        },
      ],
      pagination: {
        current_page: 1,
        page_size: 1000,
        total_pages: 1,
        total_records: 1,
      },
    };
    mockApiGet.mockResolvedValueOnce(mockResponse);

    const request = createMockRequest("/api/students?view=slim");
    const response = await GET(request, createMockContext());
    const json = await parseJsonResponse<{
      data: { data: Array<Record<string, unknown>> };
    }>(response);
    const student = json.data.data[0]!;

    expect(student).toEqual(
      expect.objectContaining({
        id: "7",
        name: "Anna Muster",
        second_name: "Muster",
        group_id: "9",
        companion_student_ids: ["8"],
      }),
    );
    for (const omittedField of [
      "bus_days",
      "pickup_days",
      "departure_days",
      "allowed_departure_modes",
      "guardian_name",
      "address_street",
      "health_info",
      "supervisor_notes",
      "created_at",
      "updated_at",
    ]) {
      expect(student).not.toHaveProperty(omittedField);
    }
  });

  it("handles null response gracefully", async () => {
    mockApiGet.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{ data: { data: unknown[] } }>(
      response,
    );
    expect(json.data.data).toEqual([]);
  });

  it("forwards query parameters to backend", async () => {
    const mockResponse = {
      data: [],
      pagination: {
        current_page: 1,
        page_size: 1000,
        total_pages: 0,
        total_records: 0,
      },
    };
    mockApiGet.mockResolvedValueOnce(mockResponse);

    const request = createMockRequest("/api/students?group_id=5");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students?group_id=5&page_size=1000",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("respects an explicit page_size from the client (room modal sends 200)", async () => {
    // Regression guard for #1374: the proxy used to overwrite every
    // page_size value to 1000, which silently disabled the room-detail
    // modal's 200-card cap and the overflow notice that depends on it.
    const mockResponse = {
      data: [],
      pagination: {
        current_page: 1,
        page_size: 200,
        total_pages: 0,
        total_records: 0,
      },
    };
    mockApiGet.mockResolvedValueOnce(mockResponse);

    const request = createMockRequest("/api/students?room_id=42&page_size=200");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students?room_id=42&page_size=200",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("logs rate-limited fetch failures as warnings", async () => {
    mockApiGet.mockRejectedValueOnce(
      new Error("API error (429): Rate limit exceeded"),
    );

    const request = createMockRequest("/api/students");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(429);
    expect(consoleSpies.warn).toHaveBeenCalledWith("students fetch failed", {
      error: "API error (429): Rate limit exceeded",
      rate_limited: true,
    });
  });
});

describe("POST /api/students", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students", {
      method: "POST",
      body: {
        first_name: "Bob",
        last_name: "Jones",
        school_class: "2b",
        guardian_email: "bob@example.com",
      },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("creates a new student", async () => {
    const mockCreatedStudent = {
      data: {
        id: 42,
        person_id: 100,
        first_name: "Bob",
        last_name: "Jones",
        school_class: "2b",
        guardian_name: "Bob Jones",
        guardian_contact: "bob@example.com",
        bus: false,
        created_at: "2024-01-15T10:00:00Z",
        updated_at: "2024-01-15T10:00:00Z",
      },
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedStudent);

    const request = createMockRequest("/api/students", {
      method: "POST",
      body: {
        first_name: "Bob",
        last_name: "Jones",
        school_class: "2b",
        guardian_email: "bob@example.com",
        privacy_consent_accepted: true,
        data_retention_days: 30,
      },
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/students",
      "test-token",
      expect.objectContaining({
        first_name: "Bob",
        last_name: "Jones",
        school_class: "2b",
      }),
    );
    expect(response.status).toBe(200);
  });
});
