import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { suppressConsole } from "~/test/helpers/console";
import { createAxiosResponse } from "~/test/helpers/axios";
import { mockSessionData } from "~/test/mocks/next-auth";
import {
  buildBackendStudent,
  buildStudent,
  buildBackendGroup,
} from "~/test/fixtures";

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
  deleteStudent,
  fetchGroups,
  fetchStudentPrivacyConsent,
  updateStudentPrivacyConsent,
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

      it("builds URL with filters", async () => {
        const filters: StudentFilters = {
          search: "Max",
          school_class: "3a",
          group_id: "10",
          location: "Schule",
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
        // eslint-disable-next-line @typescript-eslint/unbound-method
        const mockGet = vi.mocked(api.get);
        mockGet.mockResolvedValueOnce(createAxiosResponse(paginatedResponse));

        const result = await fetchStudents();

        expect(mockGet).toHaveBeenCalledWith("http://localhost:8080/students");
        expect(result.students).toHaveLength(1);
        expect(result.students[0]?.id).toBe("1");
        expect(result.students[0]?.first_name).toBe("Max");
      });

      it("returns empty array when response has no data", async () => {
        // eslint-disable-next-line @typescript-eslint/unbound-method
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

  describe("deleteStudent", () => {
    describe("client-side (browser)", () => {
      beforeEach(() => {
        mockedIsBrowserContext.mockReturnValue(true);
      });

      it("deletes a student", async () => {
        mockedAuthFetch.mockResolvedValueOnce(undefined);

        await deleteStudent("123");

        expect(mockedAuthFetch).toHaveBeenCalledWith("/api/students/123", {
          method: "DELETE",
          token: "test-token",
        });
      });
    });

    describe("server-side (SSR)", () => {
      beforeEach(() => {
        mockedIsBrowserContext.mockReturnValue(false);
      });

      it("deletes a student using axios", async () => {
        // eslint-disable-next-line @typescript-eslint/unbound-method
        const mockDelete = vi.mocked(api.delete);
        mockDelete.mockResolvedValueOnce(createAxiosResponse({}));

        await deleteStudent("123");

        expect(mockDelete).toHaveBeenCalledWith(
          "http://localhost:8080/students/123",
        );
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
            // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
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
        // eslint-disable-next-line @typescript-eslint/unbound-method
        const mockGet = vi.mocked(api.get);
        mockGet.mockResolvedValueOnce(
          createAxiosResponse({ data: [sampleBackendGroup] }),
        );

        const result = await fetchGroups();

        expect(mockGet).toHaveBeenCalledWith("http://localhost:8080/groups");
        expect(result).toHaveLength(1);
      });

      it("returns empty array when data is missing", async () => {
        // eslint-disable-next-line @typescript-eslint/unbound-method
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

    it("returns null on error", async () => {
      (globalThis.fetch as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
        new Error("Network error"),
      );

      const result = await fetchStudentPrivacyConsent("123");

      expect(result).toBeNull();
      expect(consoleSpies.error).toHaveBeenCalledWith(
        "failed to fetch privacy consent",
        {
          student_id: "123",
          // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
          error: expect.any(String),
        },
      );
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
});
