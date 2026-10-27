import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { NextRequest } from "next/server";
import { ApiResponseError } from "~/lib/api-response-error.server";
import { GET, PUT, PATCH, DELETE } from "./route";
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

// The real handleApiError: the consent marker test reads the forwarded envelope.
vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: mockApiDelete,
}));

vi.mock("~/lib/student-privacy-helpers", () => ({
  DEFAULT_PRIVACY_CONSENT: {
    privacy_consent_accepted: false,
    data_retention_days: 30,
  },
  fetchPrivacyConsent: vi.fn(() =>
    Promise.resolve({
      privacy_consent_accepted: true,
      data_retention_days: 30,
    }),
  ),
  updatePrivacyConsent: vi.fn(() => Promise.resolve()),
}));

vi.mock("~/lib/student-helpers", () => ({
  mapStudentResponse: vi.fn(
    (data: {
      id?: number;
      first_name?: string;
      last_name?: string;
      school_class?: string;
      guardian_name?: string;
      guardian_contact?: string;
    }) => ({
      id: data.id?.toString() ?? "1",
      first_name: data.first_name ?? "Test",
      last_name: data.last_name ?? "Student",
      school_class: data.school_class ?? "1a",
      guardian_name: data.guardian_name ?? "Guardian",
      guardian_contact: data.guardian_contact ?? "contact@example.com",
    }),
  ),
  prepareStudentForBackend: vi.fn((data: Record<string, unknown>) => data),
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

describe("GET /api/students/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches student by ID with privacy consent", async () => {
    const mockStudent = {
      data: {
        id: 123,
        person_id: 10,
        first_name: "Alice",
        last_name: "Smith",
        school_class: "1a",
        guardian_name: "Jane Smith",
        guardian_contact: "jane@example.com",
        has_full_access: true,
        has_write_access: true,
        has_absence_write_access: true,
        has_sick_excused_write_access: false,
        group_supervisors: [],
      },
    };
    mockApiGet.mockResolvedValueOnce(mockStudent);

    const request = createMockRequest("/api/students/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith("/api/students/123", "test-token", {
      signal: expect.any(AbortSignal),
    });
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      data: {
        id: string;
        has_full_access: boolean;
        has_absence_write_access: boolean;
        has_sick_excused_write_access: boolean;
      };
    }>(response);
    expect(json.data.id).toBe("123");
    expect(json.data.has_full_access).toBe(true);
    expect(json.data.has_absence_write_access).toBe(true);
    expect(json.data.has_sick_excused_write_access).toBe(false);
  });

  it("throws error when student ID is missing", async () => {
    const request = createMockRequest("/api/students/");
    const response = await GET(request, createMockContext({ id: undefined }));

    expect(response.status).toBe(500);
  });

  // The consent read is independent of the student record, so both backend
  // calls start together (#4995). These cases run the real consent helper so
  // its "missing consent → defaults" rule is exercised through the route.
  describe("student and consent fetched together", () => {
    const STUDENT_PATH = "/api/students/123";
    const CONSENT_PATH = "/api/students/123/privacy-consent";
    const studentBody = {
      data: { id: 123, person_id: 10, first_name: "Alice", last_name: "Smith" },
    };

    function deferred<T>() {
      let resolve!: (value: T) => void;
      let reject!: (reason: unknown) => void;
      const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
      });
      return { promise, resolve, reject };
    }

    function problem(status: number, code: string) {
      return new ApiResponseError(
        status,
        JSON.stringify({
          type: "about:blank",
          title: "Error",
          status,
          detail: "backend",
          code,
        }),
        { contentType: "application/problem+json" },
      );
    }

    async function useRealConsentHelper() {
      const actual = await vi.importActual<
        typeof import("~/lib/student-privacy-helpers")
      >("~/lib/student-privacy-helpers");
      const { fetchPrivacyConsent } =
        await import("~/lib/student-privacy-helpers");
      vi.mocked(fetchPrivacyConsent).mockImplementationOnce(
        actual.fetchPrivacyConsent,
      );
    }

    type Respond = (signal?: AbortSignal) => Promise<unknown>;

    function routeApiGet(responses: Record<string, Respond>) {
      mockApiGet.mockImplementation(
        (path: string, _token: string, options?: { signal?: AbortSignal }) => {
          const respond = responses[path];
          if (!respond) throw new Error(`unexpected GET ${path}`);
          return respond(options?.signal);
        },
      );
    }

    // A backend call that never answers on its own, like fetch it only settles
    // when its signal aborts.
    function hangUntilAborted(seen: AbortSignal[]): Respond {
      return (signal) => {
        if (signal) seen.push(signal);
        return new Promise((_resolve, reject) => {
          signal?.addEventListener("abort", () => {
            reject(new DOMException("aborted", "AbortError"));
          });
        });
      };
    }

    function getStudent(request = createMockRequest(STUDENT_PATH)) {
      return GET(request, createMockContext({ id: "123" }));
    }

    function sessionWith(permissions: string[]): ExtendedSession {
      return {
        ...defaultSession,
        user: { ...defaultSession.user, permissions },
      } as ExtendedSession;
    }

    const withSignal = { signal: expect.any(AbortSignal) as unknown };

    beforeEach(async () => {
      mockAuth.mockResolvedValue(sessionWith(["users:read"]));
      await useRealConsentHelper();
    });

    // A case that fails before the route reads the consent would otherwise
    // leave the real helper queued for the next test.
    afterEach(async () => {
      mockApiGet.mockReset();
      const { fetchPrivacyConsent } =
        await import("~/lib/student-privacy-helpers");
      vi.mocked(fetchPrivacyConsent).mockReset();
    });

    it("starts the consent request before the student request has answered", async () => {
      const student = deferred<unknown>();
      const consent = deferred<unknown>();
      routeApiGet({
        [STUDENT_PATH]: () => student.promise,
        [CONSENT_PATH]: () => consent.promise,
      });

      const pending = getStudent();

      await vi.waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(
          CONSENT_PATH,
          "test-token",
          withSignal,
        );
      });
      expect(mockApiGet).toHaveBeenCalledWith(
        STUDENT_PATH,
        "test-token",
        withSignal,
      );

      consent.resolve({ data: { accepted: true, data_retention_days: 14 } });
      student.resolve(studentBody);
      const response = await pending;

      expect(response.status).toBe(200);
      const json = await parseJsonResponse<{
        data: {
          privacy_consent_accepted: boolean;
          data_retention_days: number;
        };
      }>(response);
      expect(json.data.privacy_consent_accepted).toBe(true);
      expect(json.data.data_retention_days).toBe(14);
    });

    it.each([404, 403])(
      "falls back to the default consent when the consent read answers %i",
      async (status) => {
        routeApiGet({
          [STUDENT_PATH]: () => Promise.resolve(studentBody),
          [CONSENT_PATH]: () =>
            Promise.reject(
              problem(
                status,
                status === 403 ? "general.permission" : "general.input",
              ),
            ),
        });

        const response = await getStudent();

        expect(response.status).toBe(200);
        const json = await parseJsonResponse<{
          data: {
            id: string;
            privacy_consent_accepted: boolean;
            data_retention_days: number;
          };
        }>(response);
        expect(json.data.id).toBe("123");
        expect(json.data.privacy_consent_accepted).toBe(false);
        expect(json.data.data_retention_days).toBe(30);
      },
    );

    it("answers the student 404 even when the consent read fails first", async () => {
      const student = deferred<unknown>();
      routeApiGet({
        [STUDENT_PATH]: () => student.promise,
        [CONSENT_PATH]: () => Promise.reject(problem(500, "general.server")),
      });

      const pending = getStudent();
      await vi.waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(
          CONSENT_PATH,
          "test-token",
          withSignal,
        );
      });
      student.reject(problem(404, "general.input"));
      const response = await pending;

      expect(response.status).toBe(404);
      const json = await parseJsonResponse<{ code: string }>(response);
      expect(json.code).toBe("general.input");
    });

    it("answers a student 5xx through the shared error path", async () => {
      routeApiGet({
        [STUDENT_PATH]: () =>
          Promise.reject(problem(503, "general.unavailable")),
        [CONSENT_PATH]: () =>
          Promise.resolve({
            data: { accepted: true, data_retention_days: 14 },
          }),
      });

      const response = await getStudent();

      expect(response.status).toBe(503);
      const json = await parseJsonResponse<{ code: string }>(response);
      expect(json.code).toBe("general.unavailable");
    });

    // A failed student read must not wait for a consent read that hangs: the
    // request answers at once and the consent read is cancelled.
    it("answers the student 404 at once and cancels a hanging consent read", async () => {
      const consentSignals: AbortSignal[] = [];
      routeApiGet({
        [STUDENT_PATH]: () => Promise.reject(problem(404, "general.input")),
        [CONSENT_PATH]: hangUntilAborted(consentSignals),
      });

      const response = await getStudent();

      expect(response.status).toBe(404);
      const json = await parseJsonResponse<{ code: string }>(response);
      expect(json.code).toBe("general.input");
      expect(consentSignals).toHaveLength(1);
      expect(consentSignals[0]?.aborted).toBe(true);
    });

    it("cancels both backend calls when the browser drops the request", async () => {
      const signals: AbortSignal[] = [];
      routeApiGet({
        [STUDENT_PATH]: hangUntilAborted(signals),
        [CONSENT_PATH]: hangUntilAborted(signals),
      });
      const browser = new AbortController();
      const request = new NextRequest(`http://localhost:3000${STUDENT_PATH}`, {
        signal: browser.signal,
      });

      const pending = getStudent(request);
      await vi.waitFor(() => {
        expect(signals).toHaveLength(2);
      });
      browser.abort();
      await pending;

      expect(signals.every((signal) => signal.aborted)).toBe(true);
    });

    // The consent route demands users:read. Without it the backend answers
    // 403 and the route the defaults, so the call is not made at all and does
    // not spend the caller's read budget.
    it.each([[[]], [["students:read"]]])(
      "does not request the consent for a session with %j",
      async (permissions: string[]) => {
        mockAuth.mockResolvedValue(sessionWith(permissions));
        routeApiGet({
          [STUDENT_PATH]: () => Promise.resolve(studentBody),
        });

        const response = await getStudent();

        expect(response.status).toBe(200);
        const json = await parseJsonResponse<{
          data: {
            id: string;
            privacy_consent_accepted: boolean;
            data_retention_days: number;
          };
        }>(response);
        expect(json.data.id).toBe("123");
        expect(json.data.privacy_consent_accepted).toBe(false);
        expect(json.data.data_retention_days).toBe(30);
        expect(mockApiGet).toHaveBeenCalledTimes(1);
        expect(mockApiGet).not.toHaveBeenCalledWith(
          CONSENT_PATH,
          expect.anything(),
          expect.anything(),
        );
      },
    );

    it("does not answer a student 404 with a consent request for a session without users:read", async () => {
      mockAuth.mockResolvedValue(sessionWith(["students:read"]));
      routeApiGet({
        [STUDENT_PATH]: () => Promise.reject(problem(404, "general.input")),
      });

      const response = await getStudent();

      expect(response.status).toBe(404);
      expect(mockApiGet).toHaveBeenCalledTimes(1);
    });

    it("fails the request when the consent read answers 5xx", async () => {
      routeApiGet({
        [STUDENT_PATH]: () => Promise.resolve(studentBody),
        [CONSENT_PATH]: () =>
          Promise.reject(problem(502, "general.unavailable")),
      });

      const response = await getStudent();

      expect(response.status).toBe(502);
      const json = await parseJsonResponse<{ code: string }>(response);
      expect(json.code).toBe("general.unavailable");
    });
  });
});

describe("PUT /api/students/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("updates student successfully", async () => {
    const mockUpdatedStudent = {
      data: {
        id: 123,
        person_id: 10,
        first_name: "Updated",
        last_name: "Smith",
        school_class: "1a",
        guardian_name: "Jane Smith",
        guardian_contact: "jane@example.com",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:00:00Z",
      },
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedStudent);

    const request = createMockRequest("/api/students/123", {
      method: "PUT",
      body: { first_name: "Updated" },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/students/123",
      "test-token",
      expect.any(Object),
    );
    expect(response.status).toBe(200);
  });

  it("updates privacy consent when provided", async () => {
    const { updatePrivacyConsent } =
      await import("~/lib/student-privacy-helpers");

    const mockUpdatedStudent = {
      data: {
        id: 123,
        person_id: 10,
        first_name: "Alice",
        last_name: "Smith",
        school_class: "1a",
        guardian_name: "Jane Smith",
        guardian_contact: "jane@example.com",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:00:00Z",
      },
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedStudent);

    const request = createMockRequest("/api/students/123", {
      method: "PUT",
      body: {
        first_name: "Alice",
        privacy_consent_accepted: true,
        data_retention_days: 25,
      },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(updatePrivacyConsent).toHaveBeenCalled();
    expect(response.status).toBe(200);
  });

  // The student PUT carries the companion fingerprint (#1694): once it commits,
  // resending it is refused as `students.companions_changed`. So a failing consent write
  // must happen while the student is still untouched — otherwise the client
  // reports a failure for links that are already stored and its retry conflicts.
  it("rejects retired contact input before saving privacy consent", async () => {
    const { updatePrivacyConsent } =
      await import("~/lib/student-privacy-helpers");
    const response = await PUT(
      createMockRequest("/api/students/123", {
        method: "PUT",
        body: { guardian_email: "", privacy_consent_accepted: true },
      }),
      createMockContext({ id: "123" }),
    );
    expect(response.status).toBe(400);
    expect(updatePrivacyConsent).not.toHaveBeenCalled();
    expect(mockApiPut).not.toHaveBeenCalled();
  });

  it("does not touch the student when the consent update fails", async () => {
    const { updatePrivacyConsent } =
      await import("~/lib/student-privacy-helpers");
    vi.mocked(updatePrivacyConsent).mockRejectedValueOnce(
      new Error("consent backend down"),
    );

    const request = createMockRequest("/api/students/123", {
      method: "PUT",
      body: {
        first_name: "Alice",
        privacy_consent_accepted: true,
        companions: [],
      },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(500);
    expect(mockApiPut).not.toHaveBeenCalled();
  });

  // The two writes cannot share a transaction, so a student PUT that fails on
  // top of a committed consent write is a partial success. Reporting it as a
  // plain failure is what lets the user cancel the retry believing the consent
  // change never landed — the marker is what the form turns into "die
  // Datenschutzeinstellungen wurden bereits gespeichert".
  it("marks the failure when the consent write already committed", async () => {
    // updatePrivacyConsent is mocked at module level and resolves, so the
    // consent half of this request commits; the student PUT is what fails.
    mockApiPut.mockRejectedValueOnce(
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
    );

    const request = createMockRequest("/api/students/123", {
      method: "PUT",
      body: {
        first_name: "Alice",
        privacy_consent_accepted: true,
        data_retention_days: 25,
      },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

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

    const request = createMockRequest("/api/students/123", {
      method: "PUT",
      body: { first_name: "Alice" },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{
      code: string;
      details?: Record<string, unknown>;
    }>(response);
    expect(json.code).toBe("general.input");
    expect(json.details?.privacy_consent_saved).toBeUndefined();
  });

  // Everything is committed by the time the read-back runs — failing the
  // request here would send the client down its generic save-failure path for a
  // save that succeeded.
  it("still succeeds when the consent read-back fails after the update", async () => {
    const { fetchPrivacyConsent } =
      await import("~/lib/student-privacy-helpers");
    vi.mocked(fetchPrivacyConsent).mockRejectedValueOnce(
      new Error("consent read failed"),
    );

    mockApiPut.mockResolvedValueOnce({
      data: {
        id: 123,
        person_id: 10,
        first_name: "Alice",
        last_name: "Smith",
        school_class: "1a",
        guardian_name: "Jane Smith",
        guardian_contact: "jane@example.com",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:00:00Z",
      },
    });

    const request = createMockRequest("/api/students/123", {
      method: "PUT",
      body: {
        first_name: "Alice",
        privacy_consent_accepted: true,
        data_retention_days: 25,
      },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

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
});

describe("PATCH /api/students/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("is an alias for PUT handler", async () => {
    expect(PATCH).toBe(PUT);
  });
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
