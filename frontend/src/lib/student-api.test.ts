import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { suppressConsole } from "~/test/helpers/console";
import { createAxiosResponse } from "~/test/helpers/axios";
import { mockSessionData } from "~/test/mocks/next-auth";
import { buildBackendStudent, buildStudent } from "~/test/fixtures/students";
import { buildBackendGroup } from "~/test/fixtures/groups";

// Mock dependencies before importing the module
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(),
}));

vi.mock("./api-helpers", () => ({
  handleDomainApiError: vi.fn(() => {
    throw new Error("Mocked error");
  }),
  isBrowserContext: vi.fn(),
  authFetch: vi.fn(),
}));

vi.mock("./api", () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

// Import after mocks are set up
import { getSession } from "next-auth/react";
import {
  isBrowserContext,
  authFetch,
  handleDomainApiError,
} from "./api-helpers";
import api from "./api";
import {
  fetchStudents,
  fetchStudent,
  createStudent,
  updateStudent,
  fetchGroups,
  fetchStudentPrivacyConsent,
  fetchStudentEnrollmentExtraFields,
  updateStudentPrivacyConsent,
  uploadStudentPhoto,
  deleteStudentPhoto,
  fetchStudentDeletionImpact,
  deleteStudentWithData,
  StudentDeletionApiError,
  type StudentFilters,
} from "./student-api";

// Sample data matching actual type definitions
const sampleBackendStudent = buildBackendStudent({
  id: 1,
  person_id: 100,
  first_name: "Max",
  last_name: "Mustermann",
  school_class: "3a",
  current_location: "Schule",
  group_id: 10,
  tag_id: "TAG001",
});

// Sample mapped student (frontend type)
const sampleStudent = buildStudent({
  id: "1",
  name: "Max Mustermann",
  first_name: "Max",
  second_name: "Mustermann",
  school_class: "3a",
  current_location: "Schule",
  group_id: "10",
});

const sampleBackendGroup = buildBackendGroup({
  id: 1,
  name: "Class 3A",
  room_id: 10,
});

// Type for mocked functions
const mockedIsBrowserContext = vi.mocked(isBrowserContext);
const mockedAuthFetch = vi.mocked(authFetch);
const mockedGetSession = vi.mocked(getSession);
const mockedHandleDomainApiError = vi.mocked(handleDomainApiError);

describe("student-api", () => {
  const consoleSpies = suppressConsole("error");

  beforeEach(() => {
    vi.clearAllMocks();
    mockedGetSession.mockResolvedValue(mockSessionData());
    mockedHandleDomainApiError.mockImplementation(() => {
      throw new Error("Mocked error");
    });
  });

  describe("fetchStudents", () => {
    describe("client-side (browser)", () => {
      beforeEach(() => {
        mockedIsBrowserContext.mockReturnValue(true);
      });

      it("fetches students with pagination when in browser", async () => {
        const paginatedResponse = {
          status: "success",
          data: [sampleStudent],
          pagination: {
            current_page: 1,
            page_size: 10,
            total_pages: 1,
            total_records: 1,
          },
        };
        mockedAuthFetch.mockResolvedValueOnce(paginatedResponse);

        const result = await fetchStudents();

        expect(mockedAuthFetch).toHaveBeenCalledWith("/api/students", {
          token: "test-token",
        });
        expect(result.students).toEqual([sampleStudent]);
        expect(result.pagination).toEqual(paginatedResponse.pagination);
      });

      it("handles non-paginated response (array)", async () => {
        mockedAuthFetch.mockResolvedValueOnce([sampleStudent]);

        const result = await fetchStudents();

        expect(result.students).toEqual([sampleStudent]);
        expect(result.pagination).toBeUndefined();
      });

      it("unwraps Next.js route-wrapper paginated responses", async () => {
        const pagination = {
          current_page: 1,
          page_size: 1000,
          total_pages: 1,
          total_records: 1,
        };
        mockedAuthFetch.mockResolvedValueOnce({
          status: "success",
          data: {
            data: [sampleStudent],
            pagination,
          },
          message: "Success",
        });

        const result = await fetchStudents();

        expect(result.students).toEqual([sampleStudent]);
        expect(result.pagination).toEqual(pagination);
      });

      it("builds URL with filters", async () => {
        const filters: StudentFilters = {
          search: "Max",
          school_class: "3a",
          group_id: "10",
          location: "Schule",
          location_state: "transit",
          page: 1,
          page_size: 20,
        };
        mockedAuthFetch.mockResolvedValueOnce([]);

        await fetchStudents(filters);

        expect(mockedAuthFetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/students?"),
          expect.anything(),
        );
        const calledUrl = mockedAuthFetch.mock.calls[0]![0];
        expect(calledUrl).toContain("search=Max");
        expect(calledUrl).toContain("school_class=3a");
        expect(calledUrl).toContain("group_id=10");
        expect(calledUrl).toContain("location_state=transit");
        expect(calledUrl).toContain("page=1");
        expect(calledUrl).toContain("page_size=20");
      });
    });

    describe("server-side (SSR)", () => {
      beforeEach(() => {
        mockedIsBrowserContext.mockReturnValue(false);
      });

      it("fetches students using axios when on server", async () => {
        const paginatedResponse = {
          status: "success",
          data: [sampleBackendStudent],
          pagination: {
            current_page: 1,
            page_size: 10,
            total_pages: 1,
            total_records: 1,
          },
        };
        const mockGet = vi.mocked(api.get);
        mockGet.mockResolvedValueOnce(createAxiosResponse(paginatedResponse));

        const result = await fetchStudents();

        expect(mockGet).toHaveBeenCalledWith("http://server:8080/students");
        expect(result.students).toHaveLength(1);
        expect(result.students[0]?.id).toBe("1");
        expect(result.students[0]?.first_name).toBe("Max");
      });

      it("returns empty array when response has no data", async () => {
        const mockGet = vi.mocked(api.get);
        mockGet.mockResolvedValueOnce(createAxiosResponse({}));

        const result = await fetchStudents();

        expect(result.students).toEqual([]);
      });
    });
  });

  describe("fetchStudent", () => {
    describe("client-side (browser)", () => {
      beforeEach(() => {
        mockedIsBrowserContext.mockReturnValue(true);
      });

      it("fetches a single student by ID", async () => {
        mockedAuthFetch.mockResolvedValueOnce({ data: sampleStudent });

        const result = await fetchStudent("123");

        expect(mockedAuthFetch).toHaveBeenCalledWith("/api/students/123", {
          token: "test-token",
        });
        expect(result).toEqual(sampleStudent);
      });

      it("extracts data from wrapped response", async () => {
        mockedAuthFetch.mockResolvedValueOnce({ data: sampleStudent });

        const result = await fetchStudent("123");

        expect(result).toEqual(sampleStudent);
      });

      it("handles unwrapped response", async () => {
        mockedAuthFetch.mockResolvedValueOnce(sampleStudent);

        const result = await fetchStudent("123");

        expect(result).toEqual(sampleStudent);
      });
    });
  });

  describe("createStudent", () => {
    describe("client-side (browser)", () => {
      beforeEach(() => {
        mockedIsBrowserContext.mockReturnValue(true);
      });

      it("creates a new student", async () => {
        mockedAuthFetch.mockResolvedValueOnce({ data: sampleBackendStudent });

        const studentData = {
          first_name: "Max",
          last_name: "Mustermann",
          school_class: "3a",
          guardian_name: "Hans Mustermann",
          guardian_contact: "hans@example.com",
        };

        const result = await createStudent(studentData);

        expect(mockedAuthFetch).toHaveBeenCalledWith("/api/students", {
          method: "POST",
          body: studentData,
          token: "test-token",
        });
        expect(result.first_name).toBe("Max");
      });
    });
  });

  describe("updateStudent", () => {
    describe("client-side (browser)", () => {
      beforeEach(() => {
        mockedIsBrowserContext.mockReturnValue(true);
      });

      it("updates a student", async () => {
        mockedAuthFetch.mockResolvedValueOnce({ data: sampleBackendStudent });

        const updateData = { first_name: "Updated" };
        const result = await updateStudent("123", updateData);

        expect(mockedAuthFetch).toHaveBeenCalledWith("/api/students/123", {
          method: "PUT",
          body: updateData,
          token: "test-token",
        });
        expect(result.id).toBe("1");
      });
    });
  });

  describe("guarded student deletion", () => {
    const originalFetch = global.fetch;
    const fetchMock = vi.fn();

    beforeEach(() => {
      global.fetch = fetchMock as unknown as typeof fetch;
      fetchMock.mockReset();
    });

    afterEach(() => {
      global.fetch = originalFetch;
    });

    it("loads and unwraps the deletion impact", async () => {
      const impact = {
        confirmation_name: "Mia Muster",
        fingerprint: "abc",
        total: 0,
        counts: {},
        preserved: {},
      };
      fetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ data: impact }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      await expect(fetchStudentDeletionImpact("42/unsafe")).resolves.toEqual(
        impact,
      );
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/students/42%2Funsafe/delete-impact",
        { cache: "no-store" },
      );
    });

    it("uses the completion-scoped deletion endpoints", async () => {
      fetchMock
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ data: { fingerprint: "abc" } }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        )
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ success: true }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        );
      const input = {
        expected_fingerprint: "abc",
        confirmation_name: "Mia Muster",
        reason: "privacy_request" as const,
        acknowledged: true as const,
      };

      await fetchStudentDeletionImpact("42", "73");
      await deleteStudentWithData("42", input, "73");

      expect(fetchMock).toHaveBeenNthCalledWith(
        1,
        "/api/students/care-withdrawals/73/deletion-impact",
        { cache: "no-store" },
      );
      expect(fetchMock).toHaveBeenNthCalledWith(
        2,
        "/api/students/care-withdrawals/73",
        expect.objectContaining({
          method: "DELETE",
          body: JSON.stringify(input),
        }),
      );
    });

    it("sends every confirmation field in the DELETE body", async () => {
      fetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ success: true }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
      const input = {
        expected_fingerprint: "abc",
        confirmation_name: "Mia Muster",
        reason: "test_data" as const,
        acknowledged: true as const,
      };

      await deleteStudentWithData("42", input);

      expect(fetchMock).toHaveBeenCalledWith("/api/students/42", {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(input),
      });
    });

    it("preserves the status and original backend message on conflicts", async () => {
      fetchMock.mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            error: "Vorschau veraltet",
            code: "students.deletion_preview_changed",
          }),
          {
            status: 409,
            headers: { "Content-Type": "application/json" },
          },
        ),
      );

      const error = await deleteStudentWithData("42", {
        expected_fingerprint: "abc",
        confirmation_name: "Mia Muster",
        reason: "test_data",
        acknowledged: true,
      }).catch((caught: unknown) => caught);

      expect(error).toBeInstanceOf(StudentDeletionApiError);
      expect(error).toMatchObject({
        status: 409,
        message: "Vorschau veraltet",
        code: "students.deletion_preview_changed",
      });
    });

    it("extracts conflict codes embedded by the deletion proxy", async () => {
      fetchMock.mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            error: JSON.stringify({
              error: "Vorschau veraltet",
              code: "students.deletion_preview_changed",
            }),
          }),
          { status: 409, headers: { "Content-Type": "application/json" } },
        ),
      );

      const error = await deleteStudentWithData("42", {
        expected_fingerprint: "abc",
        confirmation_name: "Mia Muster",
        reason: "test_data",
        acknowledged: true,
      }).catch((caught: unknown) => caught);

      expect(error).toMatchObject({
        status: 409,
        message: "Vorschau veraltet",
        code: "students.deletion_preview_changed",
      });
    });
  });

  describe("fetchGroups", () => {
    describe("client-side (browser)", () => {
      beforeEach(() => {
        mockedIsBrowserContext.mockReturnValue(true);
      });

      it("fetches groups and maps them", async () => {
        mockedAuthFetch.mockResolvedValueOnce([sampleBackendGroup]);

        const result = await fetchGroups();

        expect(mockedAuthFetch).toHaveBeenCalledWith("/api/groups", {
          token: "test-token",
        });
        expect(result).toHaveLength(1);
        expect(result[0]?.id).toBe("1");
        expect(result[0]?.name).toBe("Class 3A");
      });

      it("returns empty array when response is not an array", async () => {
        mockedAuthFetch.mockResolvedValueOnce({ invalid: "response" });

        const result = await fetchGroups();

        expect(result).toEqual([]);
      });

      it("returns empty array on error", async () => {
        mockedAuthFetch.mockRejectedValueOnce(new Error("Network error"));

        const result = await fetchGroups();

        expect(result).toEqual([]);
        expect(consoleSpies.error).toHaveBeenCalledWith(
          "failed to fetch groups",
          {
            error: expect.any(String),
          },
        );
      });
    });

    describe("server-side (SSR)", () => {
      beforeEach(() => {
        mockedIsBrowserContext.mockReturnValue(false);
      });

      it("fetches groups using axios", async () => {
        const mockGet = vi.mocked(api.get);
        mockGet.mockResolvedValueOnce(
          createAxiosResponse({ data: [sampleBackendGroup] }),
        );

        const result = await fetchGroups();

        expect(mockGet).toHaveBeenCalledWith("http://server:8080/groups");
        expect(result).toHaveLength(1);
      });

      it("returns empty array when data is missing", async () => {
        const mockGet = vi.mocked(api.get);
        mockGet.mockResolvedValueOnce(createAxiosResponse({}));

        const result = await fetchGroups();

        expect(result).toEqual([]);
      });
    });
  });

  describe("fetchStudentPrivacyConsent", () => {
    let originalFetch: typeof fetch;

    beforeEach(() => {
      mockedIsBrowserContext.mockReturnValue(true);
      originalFetch = globalThis.fetch;
      globalThis.fetch = vi.fn();
    });

    afterEach(() => {
      globalThis.fetch = originalFetch;
    });

    it("returns null when consent not found (404)", async () => {
      (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
        ok: false,
        status: 404,
      } as Response);

      const result = await fetchStudentPrivacyConsent("123");

      expect(result).toBeNull();
    });

    it("fetches and maps privacy consent", async () => {
      const backendConsent = {
        id: 1,
        student_id: 123,
        policy_version: "1.0",
        accepted: true,
        data_retention_days: 30,
        renewal_required: false,
        accepted_at: "2024-01-15T10:00:00Z",
        created_at: "2024-01-15T10:00:00Z",
        updated_at: "2024-01-15T10:00:00Z",
      };
      (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: backendConsent }),
      } as Response);

      const result = await fetchStudentPrivacyConsent("123");

      expect(result).not.toBeNull();
      expect(result?.studentId).toBe("123");
      expect(result?.policyVersion).toBe("1.0");
      expect(result?.accepted).toBe(true);
    });

    it("throws on network error", async () => {
      (globalThis.fetch as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
        new Error("Network error"),
      );

      await expect(fetchStudentPrivacyConsent("123")).rejects.toThrow(
        "Network error",
      );

      expect(consoleSpies.error).toHaveBeenCalledWith(
        "failed to fetch privacy consent",
        {
          student_id: "123",
          error: expect.any(String),
        },
      );
    });

    it("throws on non-404 API error", async () => {
      (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
        ok: false,
        status: 500,
        statusText: "Internal Server Error",
      } as Response);

      await expect(fetchStudentPrivacyConsent("123")).rejects.toThrow(
        "API error (500): Internal Server Error",
      );
    });

    it("returns null for server-side axios 404 responses", async () => {
      mockedIsBrowserContext.mockReturnValue(false);
      const mockGet = vi.mocked(api.get);
      mockGet.mockRejectedValueOnce({
        isAxiosError: true,
        response: { status: 404 },
      });

      await expect(fetchStudentPrivacyConsent("123")).resolves.toBeNull();
    });
  });

  describe("updateStudentPrivacyConsent", () => {
    describe("client-side (browser)", () => {
      beforeEach(() => {
        mockedIsBrowserContext.mockReturnValue(true);
      });

      it("updates privacy consent", async () => {
        const backendConsent = {
          id: 1,
          student_id: 123,
          policy_version: "1.0",
          accepted: true,
          data_retention_days: 30,
          renewal_required: false,
          accepted_at: "2024-01-15T10:00:00Z",
          created_at: "2024-01-15T10:00:00Z",
          updated_at: "2024-01-15T10:00:00Z",
        };
        mockedAuthFetch.mockResolvedValueOnce({ data: backendConsent });

        const consentData = {
          policy_version: "1.0",
          accepted: true,
          data_retention_days: 30,
        };

        const result = await updateStudentPrivacyConsent("123", consentData);

        expect(mockedAuthFetch).toHaveBeenCalledWith(
          "/api/students/123/privacy-consent",
          {
            method: "PUT",
            body: consentData,
            token: "test-token",
          },
        );
        expect(result.accepted).toBe(true);
      });
    });

    describe("server-side (SSR)", () => {
      beforeEach(() => {
        mockedIsBrowserContext.mockReturnValue(false);
      });

      it("updates privacy consent with the bearer token", async () => {
        const backendConsent = {
          id: 1,
          student_id: 123,
          policy_version: "1.0",
          accepted: true,
          data_retention_days: 30,
          renewal_required: false,
          accepted_at: "2024-01-15T10:00:00Z",
          created_at: "2024-01-15T10:00:00Z",
          updated_at: "2024-01-15T10:00:00Z",
        };
        const mockPut = vi.mocked(api.put);
        mockPut.mockResolvedValueOnce(
          createAxiosResponse({ data: backendConsent }),
        );

        const consentData = {
          policy_version: "1.0",
          accepted: true,
          data_retention_days: 30,
        };

        const result = await updateStudentPrivacyConsent("123", consentData);

        expect(mockPut).toHaveBeenCalledWith(
          expect.stringContaining("/students/123/privacy-consent"),
          consentData,
          { headers: { Authorization: "Bearer test-token" } },
        );
        expect(result.accepted).toBe(true);
      });

      it("uses the shared student error handler when server update fails", async () => {
        const mockPut = vi.mocked(api.put);
        mockPut.mockRejectedValueOnce(new Error("Network error"));

        await expect(
          updateStudentPrivacyConsent("123", {
            policy_version: "1.0",
            accepted: false,
            data_retention_days: 30,
          }),
        ).rejects.toThrow("Mocked error");
        expect(mockedHandleDomainApiError).toHaveBeenCalledWith(
          expect.any(Error),
          "update privacy consent",
          "STUDENT",
        );
      });
    });
  });

  describe("fetchStudentEnrollmentExtraFields", () => {
    const enrollmentExtraGroups = [
      {
        request_id: "77",
        phase_name: "Anmeldung 2026",
        submitted_at: "2026-06-01T12:00:00Z",
        fields: [
          {
            key: "swimming_level",
            label: "Kann Schwimmen?",
            type: "select",
            options: [{ label: "Ja", value: "yes" }],
            value: "yes",
          },
        ],
      },
    ];

    it("fetches enrollment extra fields through the browser proxy", async () => {
      mockedIsBrowserContext.mockReturnValue(true);
      mockedAuthFetch.mockResolvedValueOnce({ data: enrollmentExtraGroups });

      const result = await fetchStudentEnrollmentExtraFields("123");

      expect(mockedAuthFetch).toHaveBeenCalledWith(
        "/api/students/123/enrollment-extra-fields",
        { token: "test-token" },
      );
      expect(result).toEqual(enrollmentExtraGroups);
    });

    it("accepts an unwrapped browser response array", async () => {
      mockedIsBrowserContext.mockReturnValue(true);
      mockedAuthFetch.mockResolvedValueOnce(enrollmentExtraGroups);

      await expect(fetchStudentEnrollmentExtraFields("123")).resolves.toEqual(
        enrollmentExtraGroups,
      );
    });

    it("returns an empty array for malformed browser responses", async () => {
      mockedIsBrowserContext.mockReturnValue(true);
      mockedAuthFetch.mockResolvedValueOnce({ data: { unexpected: true } });

      await expect(fetchStudentEnrollmentExtraFields("123")).resolves.toEqual(
        [],
      );
    });

    it("fetches enrollment extra fields from the backend in server context", async () => {
      mockedIsBrowserContext.mockReturnValue(false);
      const mockGet = vi.mocked(api.get);
      mockGet.mockResolvedValueOnce(
        createAxiosResponse({ data: enrollmentExtraGroups }),
      );

      const result = await fetchStudentEnrollmentExtraFields("123");

      expect(mockGet).toHaveBeenCalledWith(
        "http://server:8080/api/students/123/enrollment-extra-fields",
      );
      expect(result).toEqual(enrollmentExtraGroups);
    });

    it("returns an empty array when the server response has no data", async () => {
      mockedIsBrowserContext.mockReturnValue(false);
      const mockGet = vi.mocked(api.get);
      mockGet.mockResolvedValueOnce(createAxiosResponse({}));

      await expect(fetchStudentEnrollmentExtraFields("123")).resolves.toEqual(
        [],
      );
    });

    it("uses the shared student error handler on failure", async () => {
      mockedIsBrowserContext.mockReturnValue(true);
      mockedAuthFetch.mockRejectedValueOnce(new Error("Network error"));

      await expect(fetchStudentEnrollmentExtraFields("123")).rejects.toThrow(
        "Mocked error",
      );
      expect(mockedHandleDomainApiError).toHaveBeenCalledWith(
        expect.any(Error),
        "fetch student enrollment extra fields",
        "STUDENT",
      );
    });
  });

  describe("error handling", () => {
    beforeEach(() => {
      mockedIsBrowserContext.mockReturnValue(true);
      mockedHandleDomainApiError.mockImplementation(() => {
        throw new Error(
          JSON.stringify({
            status: 500,
            code: "STUDENT_API_ERROR",
            message: "Test error",
          }),
        );
      });
    });

    it("calls handleDomainApiError when fetchStudents fails", async () => {
      mockedAuthFetch.mockRejectedValueOnce(new Error("Network error"));

      await expect(fetchStudents()).rejects.toThrow();

      expect(mockedHandleDomainApiError).toHaveBeenCalledWith(
        expect.any(Error),
        "fetch students",
        "STUDENT",
      );
    });

    it("calls handleDomainApiError when fetchStudent fails", async () => {
      mockedAuthFetch.mockRejectedValueOnce(new Error("Not found"));

      await expect(fetchStudent("123")).rejects.toThrow();

      expect(mockedHandleDomainApiError).toHaveBeenCalledWith(
        expect.any(Error),
        "fetch student",
        "STUDENT",
      );
    });
  });

  // ─── uploadStudentPhoto ────────────────────────────────────────────────
  // Uses raw fetch (multipart/form-data) rather than authFetch which only
  // speaks JSON. Mock global.fetch and ensure the bearer token + form
  // fields end up shaped correctly.
  describe("uploadStudentPhoto", () => {
    const originalFetch = global.fetch;
    const fetchMock = vi.fn();

    beforeEach(() => {
      global.fetch = fetchMock as unknown as typeof fetch;
      fetchMock.mockReset();
    });

    afterEach(() => {
      global.fetch = originalFetch;
    });

    it("posts multipart form data with bearer token and returns photoUrl", async () => {
      fetchMock.mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            data: { photo_url: "/uploads/student-photos/42_abc.jpg" },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      );

      const file = new Blob(["binary"], { type: "image/jpeg" });
      const result = await uploadStudentPhoto("42", file);

      expect(result).toEqual({
        photoUrl: "/uploads/student-photos/42_abc.jpg",
      });
      expect(fetchMock).toHaveBeenCalledTimes(1);
      const [calledUrl, calledOptions] = fetchMock.mock.calls[0] as [
        string,
        RequestInit,
      ];
      expect(calledUrl).toBe("/api/students/42/photo");
      expect(calledOptions.method).toBe("POST");
      const headers = calledOptions.headers as Record<string, string>;
      // Default mockSessionData() returns user.token = "test-token".
      expect(headers.Authorization).toMatch(/^Bearer /);
      // Body must be FormData with the "photo" field set.
      const body = calledOptions.body as FormData;
      expect(body).toBeInstanceOf(FormData);
      expect(body.get("photo")).toBeInstanceOf(Blob);
      expect((body.get("photo") as File).name).toBe("student-photo.jpg");
      // No consent_acknowledged when option not passed.
      expect(body.get("consent_acknowledged")).toBeNull();
    });

    it("preserves File names when caller passes a File", async () => {
      fetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ data: { photo_url: "/u/file.jpg" } }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      const file = new File(["binary"], "student.jpg", { type: "image/jpeg" });
      await uploadStudentPhoto("42", file);

      const [, calledOptions] = fetchMock.mock.calls[0] as [
        string,
        RequestInit,
      ];
      const body = calledOptions.body as FormData;
      expect((body.get("photo") as File).name).toBe("student.jpg");
    });

    it("appends consent_acknowledged=true when caller opts in", async () => {
      fetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ data: { photo_url: "/u/x.jpg" } }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      const file = new Blob(["binary"], { type: "image/jpeg" });
      await uploadStudentPhoto("42", file, { consentAcknowledged: true });

      const [, calledOptions] = fetchMock.mock.calls[0] as [
        string,
        RequestInit,
      ];
      const body = calledOptions.body as FormData;
      expect(body.get("consent_acknowledged")).toBe("true");
    });

    it("supports unwrapped { photo_url } response shape", async () => {
      fetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ photo_url: "/uploads/raw.png" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      const file = new Blob(["binary"], { type: "image/png" });
      const result = await uploadStudentPhoto("99", file);
      expect(result).toEqual({ photoUrl: "/uploads/raw.png" });
    });

    it("throws Authentifizierung erforderlich when no session token", async () => {
      // session-cache memoizes results across tests; reset before
      // changing the underlying mock so the fresh null result wins.
      const { clearSessionCache } = await import("./session-cache");
      clearSessionCache();
      mockedGetSession.mockResolvedValueOnce(null);
      const file = new Blob(["binary"], { type: "image/jpeg" });

      await expect(uploadStudentPhoto("42", file)).rejects.toThrow(
        "Authentifizierung erforderlich",
      );
      // Must not even attempt to call fetch when auth is missing.
      expect(fetchMock).not.toHaveBeenCalled();
      // Restore default mock for subsequent tests.
      clearSessionCache();
    });

    it("throws backend error text on non-OK response", async () => {
      fetchMock.mockResolvedValueOnce(
        new Response("photos feature disabled", {
          status: 403,
        }),
      );

      const file = new Blob(["binary"], { type: "image/jpeg" });
      await expect(uploadStudentPhoto("42", file)).rejects.toThrow(
        "photos feature disabled",
      );
    });

    it("falls back to generic message when error body is empty", async () => {
      fetchMock.mockResolvedValueOnce(new Response("", { status: 500 }));

      const file = new Blob(["binary"], { type: "image/jpeg" });
      await expect(uploadStudentPhoto("42", file)).rejects.toThrow(
        /Upload fehlgeschlagen \(HTTP 500\)/,
      );
    });
  });

  // ─── deleteStudentPhoto ────────────────────────────────────────────────
  describe("deleteStudentPhoto", () => {
    it("calls authFetch with DELETE method and bearer token", async () => {
      mockedAuthFetch.mockResolvedValueOnce(undefined);

      await deleteStudentPhoto("42");

      expect(mockedAuthFetch).toHaveBeenCalledWith(
        "/api/students/42/photo",
        expect.objectContaining({
          method: "DELETE",
          token: expect.any(String),
        }),
      );
    });

    it("propagates authFetch errors so the caller can render them", async () => {
      mockedAuthFetch.mockRejectedValueOnce(new Error("API error (403): nope"));

      await expect(deleteStudentPhoto("42")).rejects.toThrow(
        "API error (403): nope",
      );
    });
  });
});
