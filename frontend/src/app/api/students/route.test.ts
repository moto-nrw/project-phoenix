import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiResponseError } from "~/lib/api-response-error.server";
import { GET, POST } from "./route";
import { suppressConsole } from "~/test/helpers/console";
import { captureException } from "@sentry/nextjs";
import { ERROR_CODES } from "~/lib/error-codes.generated";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet, mockApiPost, mockApiPut } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
  mockApiPut: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@sentry/nextjs", () => ({ captureException: vi.fn() }));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: mockApiGet,
  apiPost: mockApiPost,
  apiPut: mockApiPut,
  apiDelete: vi.fn(),
  handleApiError: vi.fn(
    (await import("~/test/helpers/handle-api-error")).mockHandleApiError,
  ),
}));

// ============================================================================
// Test Helpers
// ============================================================================

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
      new ApiResponseError(429, "Rate limit exceeded"),
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

  it("returns a registered 400 field error without reporting to Sentry when the last name is missing", async () => {
    const response = await POST(
      createMockRequest("/api/students", {
        method: "POST",
        body: { first_name: "Mia", school_class: "2b" },
      }),
      createMockContext(),
    );

    expect(response.status).toBe(400);
    expect(response.headers.get("Content-Type")).toContain(
      "application/problem+json",
    );
    const body = await response.json();
    expect(ERROR_CODES).toContain(body.code);
    expect(body).toMatchObject({
      status: 400,
      code: "general.input",
      errors: [{ field: "last_name", reason: "required" }],
    });
    expect(mockApiPost).not.toHaveBeenCalled();
    expect(captureException).not.toHaveBeenCalled();
  });

  it.each([undefined, "", "   "])(
    "reports every missing required field for %j without forwarding or reporting to Sentry",
    async (value) => {
      const response = await POST(
        createMockRequest("/api/students", {
          method: "POST",
          body: {
            first_name: value,
            second_name: value,
            school_class: value,
          },
        }),
        createMockContext(),
      );

      expect(response.status).toBe(400);
      expect(await response.json()).toMatchObject({
        code: "general.input",
        errors: [
          { field: "first_name", reason: "required" },
          { field: "last_name", reason: "required" },
          { field: "school_class", reason: "required" },
        ],
      });
      expect(mockApiPost).not.toHaveBeenCalled();
      expect(captureException).not.toHaveBeenCalled();
    },
  );

  it("creates a new student with the backend body built by the real request helpers", async () => {
    mockApiPost.mockResolvedValueOnce({
      data: {
        id: 42,
        person_id: 100,
        first_name: "Bob",
        last_name: "Jones",
        school_class: "2b",
        bus: false,
        created_at: "2024-01-15T10:00:00Z",
        updated_at: "2024-01-15T10:00:00Z",
      },
    });
    mockApiPut.mockResolvedValueOnce({});
    mockApiGet.mockResolvedValueOnce({
      data: { accepted: true, data_retention_days: 60 },
    });

    const request = createMockRequest("/api/students", {
      method: "POST",
      body: {
        first_name: " Bob ",
        second_name: "Jones",
        school_class: "2b",
        privacy_consent_accepted: true,
        data_retention_days: 60,
      },
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledExactlyOnceWith(
      "/api/students",
      "test-token",
      {
        first_name: "Bob",
        last_name: "Jones",
        school_class: "2b",
        current_location: "Unbekannt",
      },
    );
    expect(mockApiPut).toHaveBeenCalledExactlyOnceWith(
      "/api/students/42/privacy-consent",
      "test-token",
      { policy_version: "1.0", accepted: true, data_retention_days: 60 },
    );
    expect(mockApiGet).toHaveBeenCalledExactlyOnceWith(
      "/api/students/42/privacy-consent",
      "test-token",
    );
    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{
      data: {
        id: string;
        first_name: string;
        second_name: string;
        privacy_consent_accepted: boolean;
        data_retention_days: number;
      };
    }>(response);
    expect(json.data).toMatchObject({
      id: "42",
      first_name: "Bob",
      second_name: "Jones",
      privacy_consent_accepted: true,
      data_retention_days: 60,
    });
  });
});
