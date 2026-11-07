import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
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
  withTenantAuth: (handler: unknown) => handler,
}));

// The real handleApiError: the consent marker test reads the forwarded envelope.
vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: mockApiDelete,
}));

const studentPath = "/api/students/123";
const consentPath = "/api/students/123/privacy-consent";

/** The backend student row of a PUT answer. */
function updatedRow(overrides: Record<string, unknown> = {}) {
  return {
    data: {
      id: 123,
      person_id: 10,
      first_name: "Alice",
      last_name: "Smith",
      school_class: "1a",
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-15T10:00:00Z",
      ...overrides,
    },
  };
}

/** The detail read: the full row plus the flags the backend always sends. */
function detailRow(overrides: Record<string, unknown> = {}) {
  return {
    data: {
      ...updatedRow().data,
      has_full_access: true,
      has_write_access: true,
      has_absence_write_access: true,
      has_sick_excused_write_access: true,
      attendance_log_enabled: false,
      feedback_enabled: false,
      privacy_consent: { accepted: false, data_retention_days: 30 },
      ...overrides,
    },
  };
}

function putCalls(path: string) {
  return mockApiPut.mock.calls.filter(([url]) => url === path);
}

// ============================================================================
// Test Helpers
// ============================================================================

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/students/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("answers the mapped child with access and feature flags", async () => {
    mockApiGet.mockResolvedValueOnce(
      detailRow({
        has_sick_excused_write_access: false,
        feedback_enabled: true,
        group_supervisors: [],
      }),
    );

    const request = createMockRequest("/api/students/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students/123?include_privacy_consent=true",
      "test-token",
      { signal: expect.any(AbortSignal) },
    );
    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{ data: Record<string, unknown> }>(
      response,
    );
    expect(json.data).toMatchObject({
      id: "123",
      name: "Alice Smith",
      second_name: "Smith",
      has_full_access: true,
      has_absence_write_access: true,
      has_sick_excused_write_access: false,
      feedback_enabled: true,
    });
  });

  it("spends only one backend read for a missing child with users:read", async () => {
    mockAuth.mockResolvedValue({
      ...defaultSession,
      user: { ...defaultSession.user, permissions: ["users:read"] },
    } as ExtendedSession);
    mockApiGet.mockRejectedValue(
      new ApiResponseError(404, '{"code":"general.input"}'),
    );
    const response = await GET(
      createMockRequest("/api/students/123"),
      createMockContext({ id: "123" }),
    );
    expect(response.status).toBe(404);
    expect(mockApiGet).toHaveBeenCalledTimes(1);
  });

  describe("aggregated student and consent read", () => {
    const backendPath = "/api/students/123?include_privacy_consent=true";
    function getStudent(request = createMockRequest("/api/students/123")) {
      return GET(request, createMockContext({ id: "123" }));
    }

    it("maps consent from the same response without another backend call", async () => {
      mockApiGet.mockResolvedValueOnce(
        detailRow({
          privacy_consent: { accepted: true, data_retention_days: 14 },
        }),
      );
      const response = await getStudent();
      expect(response.status).toBe(200);
      const json = await parseJsonResponse<{
        data: {
          id: string;
          privacy_consent_accepted: boolean;
          data_retention_days: number;
        };
      }>(response);
      expect(json.data).toMatchObject({
        id: "123",
        privacy_consent_accepted: true,
        data_retention_days: 14,
      });
      expect(mockApiGet).toHaveBeenCalledTimes(1);
      expect(mockApiGet).toHaveBeenCalledWith(backendPath, "test-token", {
        signal: expect.any(AbortSignal),
      });
    });

    it.each([403, 404, 429, 500, 502, 503])(
      "forwards aggregate failure %i without successful defaults",
      async (status) => {
        const body = JSON.stringify({
          type: "about:blank",
          title: "Error",
          status,
          code: "general.unavailable",
          detail: "backend",
        });
        mockApiGet.mockRejectedValueOnce(
          new ApiResponseError(status, body, {
            contentType: "application/problem+json",
            retryAfter: "60",
          }),
        );
        const response = await getStudent();
        expect(response.status).toBe(status);
        expect(await response.json()).toMatchObject({
          status,
          code: "general.unavailable",
        });
        expect(response.headers.get("Retry-After")).toBe("60");
        expect(mockApiGet).toHaveBeenCalledTimes(1);
      },
    );

    it("cancels the single backend read when the browser disconnects", async () => {
      const browser = new AbortController();
      let backendSignal: AbortSignal | undefined;
      mockApiGet.mockImplementation(
        (_path: string, _token: string, options: { signal: AbortSignal }) => {
          backendSignal = options.signal;
          return new Promise((_resolve, reject) => {
            options.signal.addEventListener("abort", () =>
              reject(new DOMException("aborted", "AbortError")),
            );
          });
        },
      );
      const pending = getStudent(
        new NextRequest("http://localhost:3000/api/students/123", {
          signal: browser.signal,
        }),
      );
      await vi.waitFor(() => expect(mockApiGet).toHaveBeenCalledTimes(1));
      browser.abort();
      await pending;
      expect(backendSignal?.aborted).toBe(true);
    });

    it.each([[[]], [["students:read"]], [["users:read"]]])(
      "preserves redacted/default consent for permissions %j",
      async (permissions: string[]) => {
        mockAuth.mockResolvedValue({
          ...defaultSession,
          user: { ...defaultSession.user, permissions },
        } as ExtendedSession);
        mockApiGet.mockResolvedValueOnce(
          detailRow({
            privacy_consent: { accepted: false, data_retention_days: 30 },
          }),
        );
        const response = await getStudent();
        expect(response.status).toBe(200);
        const json = await parseJsonResponse<{
          data: {
            privacy_consent_accepted: boolean;
            data_retention_days: number;
          };
        }>(response);
        expect(json.data).toMatchObject({
          privacy_consent_accepted: false,
          data_retention_days: 30,
        });
        expect(mockApiGet).toHaveBeenCalledTimes(1);
      },
    );
  });
});

describe("PUT /api/students/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
    mockApiGet.mockResolvedValue({
      data: { accepted: true, data_retention_days: 25 },
    });
  });

  it("updates the child and leaves an unsent consent alone", async () => {
    mockApiPut.mockResolvedValueOnce(updatedRow({ first_name: "Updated" }));

    const response = await PUT(
      createMockRequest(studentPath, {
        method: "PUT",
        body: { first_name: "Updated", sick: true },
      }),
      createMockContext({ id: "123" }),
    );

    expect(response.status).toBe(200);
    expect(putCalls(consentPath)).toEqual([]);
    expect(putCalls(studentPath)).toEqual([
      [
        studentPath,
        "test-token",
        expect.objectContaining({
          first_name: "Updated",
          sick: true,
        }),
      ],
    ]);
    expect(
      (await parseJsonResponse<{ data: unknown }>(response)).data,
    ).toMatchObject({ id: "123", first_name: "Updated", second_name: "Smith" });
  });

  it("writes a sent consent pair before the child", async () => {
    mockApiPut.mockImplementation((url: string) =>
      Promise.resolve(url === studentPath ? updatedRow() : {}),
    );

    const response = await PUT(
      createMockRequest(studentPath, {
        method: "PUT",
        body: {
          first_name: "Alice",
          privacy_consent_accepted: true,
          data_retention_days: 25,
        },
      }),
      createMockContext({ id: "123" }),
    );

    expect(response.status).toBe(200);
    expect(mockApiPut.mock.calls.map(([url]) => url)).toEqual([
      consentPath,
      studentPath,
    ]);
    expect(putCalls(consentPath)[0]?.[2]).toEqual({
      policy_version: "1.0",
      accepted: true,
      data_retention_days: 25,
    });
  });

  it("refuses a lone consent field instead of resetting the other", async () => {
    const response = await PUT(
      createMockRequest(studentPath, {
        method: "PUT",
        body: { privacy_consent_accepted: true },
      }),
      createMockContext({ id: "123" }),
    );

    expect(response.status).toBe(400);
    expect(mockApiPut).not.toHaveBeenCalled();
  });

  it("rejects retired contact input before saving privacy consent", async () => {
    const response = await PUT(
      createMockRequest(studentPath, {
        method: "PUT",
        body: {
          guardian_email: "",
          privacy_consent_accepted: true,
          data_retention_days: 25,
        },
      }),
      createMockContext({ id: "123" }),
    );
    expect(response.status).toBe(400);
    expect(mockApiPut).not.toHaveBeenCalled();
  });

  // The student PUT carries the companion fingerprint (#1694): once it commits,
  // resending it is refused as `students.companions_changed`. So a failing
  // consent write must happen while the student is still untouched.
  it("answers the backend's consent refusal and does not touch the child", async () => {
    mockApiPut.mockRejectedValueOnce(
      new ApiResponseError(
        400,
        JSON.stringify({
          status: 400,
          detail: "data retention days must be between 1 and 31",
          code: "general.input",
        }),
        { contentType: "application/problem+json" },
      ),
    );

    const response = await PUT(
      createMockRequest(studentPath, {
        method: "PUT",
        body: {
          first_name: "Alice",
          privacy_consent_accepted: true,
          data_retention_days: 99,
          companions: [],
        },
      }),
      createMockContext({ id: "123" }),
    );

    expect(response.status).toBe(400);
    expect((await response.json()).code).toBe("general.input");
    expect(putCalls(studentPath)).toEqual([]);
  });

  // The two writes cannot share a transaction, so a student PUT that fails on
  // top of a committed consent write is a partial success; the marker is what
  // the form turns into "die Datenschutzeinstellungen wurden bereits
  // gespeichert".
  it("marks the failure when the consent write already committed", async () => {
    mockApiPut.mockImplementation((url: string) =>
      url === studentPath
        ? Promise.reject(
            new ApiResponseError(
              409,
              JSON.stringify({
                type: "about:blank",
                title: "Conflict",
                status: 409,
                detail: "companions changed since the form was loaded",
                code: "students.companions_changed",
                conflicts: [{ companion_student_id: "7" }],
              }),
            ),
          )
        : Promise.resolve({}),
    );

    const response = await PUT(
      createMockRequest(studentPath, {
        method: "PUT",
        body: {
          first_name: "Alice",
          privacy_consent_accepted: true,
          data_retention_days: 25,
        },
      }),
      createMockContext({ id: "123" }),
    );

    expect(response.status).toBe(409);
    const payload = await parseJsonResponse<{
      status: number;
      detail: string;
      code?: string;
      conflicts?: unknown[];
      details?: Record<string, unknown>;
    }>(response);
    expect(payload.details?.privacy_consent_saved).toBe(true);
    // Everything the browser branches on has to survive the re-shaping.
    expect(payload.code).toBe("students.companions_changed");
    expect(payload.conflicts).toHaveLength(1);
    expect(payload.status).toBe(409);
    expect(payload.detail).toBe("companions changed since the form was loaded");
  });

  it("leaves the failure untouched when no consent was written", async () => {
    mockApiPut.mockRejectedValueOnce(
      new ApiResponseError(
        400,
        JSON.stringify({
          status: 400,
          detail: "invalid",
          code: "general.input",
        }),
      ),
    );

    const response = await PUT(
      createMockRequest(studentPath, {
        method: "PUT",
        body: { first_name: "Alice" },
      }),
      createMockContext({ id: "123" }),
    );

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{
      code: string;
      details?: Record<string, unknown>;
    }>(response);
    expect(json.code).toBe("general.input");
    expect(json.details?.privacy_consent_saved).toBeUndefined();
  });

  // Everything is committed by the time the read-back runs — failing the
  // request here would send the client down its generic save-failure path for
  // a save that succeeded.
  it("still succeeds when the consent read-back fails after the update", async () => {
    mockApiGet.mockRejectedValueOnce(new ApiResponseError(500, "{}"));
    mockApiPut.mockImplementation((url: string) =>
      Promise.resolve(url === studentPath ? updatedRow() : {}),
    );

    const response = await PUT(
      createMockRequest(studentPath, {
        method: "PUT",
        body: {
          first_name: "Alice",
          privacy_consent_accepted: true,
          data_retention_days: 25,
        },
      }),
      createMockContext({ id: "123" }),
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{
      data: {
        id: string;
        privacy_consent_accepted: boolean;
        data_retention_days: number;
      };
    }>(response);
    expect(json.data.id).toBe("123");
    // Falls back to what was just written instead of dropping the fields.
    expect(json.data.privacy_consent_accepted).toBe(true);
    expect(json.data.data_retention_days).toBe(25);
  });

  it("forwards the write's verdict on the companion links", async () => {
    mockApiPut.mockResolvedValueOnce(updatedRow({ companions_changed: false }));

    const response = await PUT(
      createMockRequest(studentPath, {
        method: "PUT",
        body: { first_name: "Alice" },
      }),
      createMockContext({ id: "123" }),
    );

    expect(
      (await parseJsonResponse<{ data: Record<string, unknown> }>(response))
        .data.companions_changed,
    ).toBe(false);
  });

  // Without a verdict the payload decides: a submitted list or a departure
  // map can change the links (the backend trims days a new plan no longer
  // allows); a name edit cannot.
  it.each([
    [{ departure_days: { mon: "accompanied" } }, true],
    [{ companions: [] }, true],
    [{ first_name: "Alice" }, false],
  ])(
    "derives the verdict from the payload %j when the backend sends none",
    async (body, changed) => {
      mockApiPut.mockResolvedValueOnce(updatedRow());

      const response = await PUT(
        createMockRequest(studentPath, { method: "PUT", body }),
        createMockContext({ id: "123" }),
      );

      expect(
        (await parseJsonResponse<{ data: Record<string, unknown> }>(response))
          .data.companions_changed,
      ).toBe(changed);
    },
  );
});

describe("DELETE /api/students/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("deletes student successfully", async () => {
    mockApiDelete.mockResolvedValueOnce({
      message: "Student deleted successfully",
    });

    const request = createMockRequest("/api/students/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/students/123",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      message: string;
    }>(response);
    expect(json.success).toBe(true);
  });

  it("forwards the guarded deletion confirmation body", async () => {
    mockApiDelete.mockResolvedValueOnce({
      message: "Student and linked data deleted successfully",
    });
    const body = {
      expected_fingerprint: "abc123",
      confirmation_name: "Mia Muster",
      reason: "test_data",
      acknowledged: true,
    };

    const request = createMockRequest("/api/students/123", {
      method: "DELETE",
      body,
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/students/123",
      "test-token",
      body,
    );
    expect(response.status).toBe(200);
  });
});
