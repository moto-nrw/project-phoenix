import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiResponseError } from "~/lib/api-response-error.server";
import { GET, PUT, DELETE } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet, mockApiPut, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPut: vi.fn(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: mockApiDelete,
  handleApiError: vi.fn(
    (await import("~/test/helpers/handle-api-error")).mockHandleApiError,
  ),
}));

// ============================================================================
// Test Helpers
// ============================================================================

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/staff/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("preserves the former employment period for the detail page", async () => {
    mockApiGet.mockResolvedValueOnce({
      data: {
        id: "42",
        person_id: "70",
        is_teacher: false,
        started_on: "2024-09-02",
        ended_on: "2026-07-31",
      },
    });
    const response = await GET(
      createMockRequest("/api/staff/42"),
      createMockContext({ id: "42" }),
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data: { started_on: string; ended_on: string };
    };
    expect(json.data.started_on).toBe("2024-09-02");
    expect(json.data.ended_on).toBe("2026-07-31");
  });

  it("fetches staff member by ID from backend", async () => {
    const mockStaff = {
      status: "success",
      data: {
        id: 1,
        person_id: 100,
        staff_notes: "Test notes",
        is_teacher: true,
        teacher_id: 50,
        specialization: "Math",
        role: "Teacher",
        qualifications: "PhD",
        person: {
          id: 100,
          first_name: "John",
          last_name: "Doe",
          email: "john.doe@example.com",
          tag_id: "TAG123",
          account_id: 10,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    };
    mockApiGet.mockResolvedValueOnce(mockStaff);

    const request = createMockRequest("/api/staff/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(mockApiGet).toHaveBeenCalledWith("/api/staff/1", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<{ id: string; name: string }>>(
        response,
      );
    expect(json.data.id).toBe("1");
    expect(json.data.name).toBe("John Doe");
  });

  it("keeps a bigint person_id as the exact decimal string", async () => {
    // 2^53 + 1: the edit screen sends this id back to address the person
    // record, so rounding it here would edit a different person.
    const personID = "9007199254740993";
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: {
        id: 1,
        person_id: personID,
        is_teacher: false,
        person: {
          id: 1,
          first_name: "John",
          last_name: "Doe",
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    });

    const request = createMockRequest("/api/staff/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    const json =
      await parseJsonResponse<ApiResponse<{ person_id: string }>>(response);
    expect(json.data.person_id).toBe(personID);
  });

  it("keeps a bigint account_id as the exact decimal string", async () => {
    const accountID = "9007199254740993";
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: {
        id: 1,
        person_id: "1",
        is_teacher: false,
        person: {
          id: 1,
          first_name: "John",
          last_name: "Doe",
          account_id: accountID,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    });

    const request = createMockRequest("/api/staff/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    const json =
      await parseJsonResponse<ApiResponse<{ account_id: string }>>(response);
    expect(json.data.account_id).toBe(accountID);
  });

  it("forwards the stored avatar path for the profile picture", async () => {
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: {
        id: "7",
        person_id: "1",
        is_teacher: false,
        person: {
          id: 1,
          first_name: "Lea",
          last_name: "Bild",
          avatar: "/uploads/avatars/global/lea.png",
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    });

    const response = await GET(
      createMockRequest("/api/staff/7"),
      createMockContext({ id: "7" }),
    );

    const json =
      await parseJsonResponse<ApiResponse<{ avatar: string | null }>>(response);
    expect(json.data.avatar).toBe("/uploads/avatars/global/lea.png");
  });

  it("keeps the external caregiver fields in the detail response", async () => {
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: {
        id: "9007199254740993",
        person_id: "1",
        is_teacher: false,
        is_external: true,
        external_organization: "Musikschule Bergstadt",
        external_account_status: "invitation_pending",
        person: {
          id: 1,
          first_name: "Lea",
          last_name: "Gast",
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    });

    const response = await GET(
      createMockRequest("/api/staff/9007199254740993"),
      createMockContext({ id: "9007199254740993" }),
    );

    const json = await parseJsonResponse<
      ApiResponse<{
        id: string;
        is_external: boolean;
        external_organization: string | null;
        external_account_status: string | null;
      }>
    >(response);
    expect(json.data).toMatchObject({
      id: "9007199254740993",
      is_external: true,
      external_organization: "Musikschule Bergstadt",
      external_account_status: "invitation_pending",
    });
  });

  it("throws error when staff member not found", async () => {
    mockApiGet.mockResolvedValueOnce({ data: null });

    const request = createMockRequest("/api/staff/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
  });

  it("throws error when ID is missing", async () => {
    const request = createMockRequest("/api/staff/undefined");
    const response = await GET(request, createMockContext({}));

    expect(response.status).toBe(500);
  });
});

describe("PUT /api/staff/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("updates staff member via backend", async () => {
    const updateBody = { role: "Senior Teacher", specialization: "Physics" };
    const mockUpdatedStaff = {
      id: 1,
      person_id: 100,
      staff_notes: null,
      is_teacher: true,
      role: "Senior Teacher",
      specialization: "Physics",
      qualifications: null,
      person: {
        id: 100,
        first_name: "John",
        last_name: "Doe",
        email: "john.doe@example.com",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-15T10:00:00Z",
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedStaff);

    const request = createMockRequest("/api/staff/1", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/staff/1",
      "test-token",
      updateBody,
    );
    expect(response.status).toBe(200);
  });

  it("throws error when staff member not found", async () => {
    mockApiPut.mockRejectedValueOnce(
      new ApiResponseError(400, "staff member not found"),
    );

    const request = createMockRequest("/api/staff/1", {
      method: "PUT",
      body: { role: "Teacher" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Bad Request");
    expect(JSON.stringify(json)).not.toContain("staff member not found");
  });
});

describe("DELETE /api/staff/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("deletes staff member via backend and returns 204", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/staff/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(mockApiDelete).toHaveBeenCalledWith("/api/staff/1", "test-token");
    expect(response.status).toBe(204);
  });
});
