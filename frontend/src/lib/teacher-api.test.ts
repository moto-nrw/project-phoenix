import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { Teacher } from "./teacher-api";
import { suppressConsole } from "~/test/helpers/console";
import { mockSessionData } from "~/test/mocks/next-auth";

// Mock next-auth/react before importing the module
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(),
}));

// Import after mocks are set up
import { getSession } from "next-auth/react";
import { teacherService } from "./teacher-api";

// Type for mocked functions
const mockedGetSession = vi.mocked(getSession);

// Sample teacher data
const sampleTeacher: Teacher = {
  id: "1",
  name: "Max Mustermann",
  first_name: "Max",
  last_name: "Mustermann",
  email: "max.mustermann@school.local",
  specialization: "Mathematics",
  role: "Teacher",
  qualifications: "M.Ed",
  tag_id: "TAG001",
  staff_notes: "Senior teacher",
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-15T00:00:00Z",
  person_id: "100",
  account_id: 50,
  is_teacher: true,
  staff_id: "1",
  teacher_id: "10",
};

const sampleTeacherMinimal: Teacher = {
  id: "2",
  name: "Anna Schmidt",
  first_name: "Anna",
  last_name: "Schmidt",
};

describe("teacher-api", () => {
  const consoleSpies = suppressConsole("error", "warn");
  let originalFetch: typeof fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn();

    // Default session mock
    mockedGetSession.mockResolvedValue(mockSessionData());
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  describe("teacherService.getTeachers", () => {
    it("fetches all teachers successfully", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve([sampleTeacher]),
      } as Response);

      const result = await teacherService.getTeachers();

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff?teachers_only=true",
        expect.objectContaining({
          credentials: "include",
        }),
      );
      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("1");
      expect(result[0]?.name).toBe("Max Mustermann");
    });

    it("applies search filter", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve([sampleTeacher]),
      } as Response);

      await teacherService.getTeachers({ search: "Max" });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff?teachers_only=true&search=Max",
        expect.any(Object),
      );
    });

    it("handles wrapped response format", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [sampleTeacher] }),
      } as Response);

      const result = await teacherService.getTeachers();

      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("1");
    });

    it("returns empty array for unexpected response format", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ unexpected: "format" }),
      } as Response);

      const result = await teacherService.getTeachers();

      expect(result).toEqual([]);
      expect(consoleSpies.error).toHaveBeenCalledWith(
        "unexpected response format for teachers",
        undefined,
      );
    });

    it("throws error when fetch fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Internal Server Error",
      } as Response);

      await expect(teacherService.getTeachers()).rejects.toThrow(
        "Failed to fetch teachers: Internal Server Error",
      );
    });

    it("throws and logs error on network failure", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      await expect(teacherService.getTeachers()).rejects.toThrow(
        "Network error",
      );
      expect(consoleSpies.error).toHaveBeenCalledWith(
        "error fetching teachers",
        { error: "Error: Network error" },
      );
    });

    it("rejects without auth token", async () => {
      // sessionFetch has always thrown without a token; this test previously
      // asserted the opposite and only stayed green because earlier tests in
      // this file left a session in the module-level getSession cache (10s
      // TTL). Since the cache is reset between tests (#2123), the null mock
      // is actually honored and the real contract shows.
      mockedGetSession.mockResolvedValue(null);

      await expect(teacherService.getTeachers()).rejects.toThrow(
        "No authentication token available",
      );
    });
  });

  describe("teacherService.getTeacher", () => {
    it("fetches a single teacher by ID", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      const result = await teacherService.getTeacher("1");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/1",
        expect.objectContaining({
          credentials: "include",
        }),
      );
      expect(result.id).toBe("1");
      expect(result.name).toBe("Max Mustermann");
    });

    it("handles wrapped response format", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: sampleTeacher }),
      } as Response);

      const result = await teacherService.getTeacher("1");

      expect(result.id).toBe("1");
    });

    it("throws error when fetch fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Not Found",
      } as Response);

      await expect(teacherService.getTeacher("999")).rejects.toThrow(
        "Failed to fetch teacher: Not Found",
      );
    });

    it("logs error on failure", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      await expect(teacherService.getTeacher("1")).rejects.toThrow();
      expect(consoleSpies.error).toHaveBeenCalledWith(
        "error fetching teacher",
        { teacher_id: "1", error: expect.stringContaining("Network error") },
      );
    });
  });

  describe("teacherService.createTeacher", () => {
    it("throws error when password is missing", async () => {
      await expect(
        teacherService.createTeacher({
          first_name: "Test",
          last_name: "Teacher",
          role_id: 1,
        }),
      ).rejects.toThrow("Password is required for creating a teacher");
    });

    it("throws error when role_id is missing", async () => {
      await expect(
        teacherService.createTeacher({
          first_name: "Test",
          last_name: "Teacher",
          password: "SecurePass123!",
        }),
      ).rejects.toThrow("Role ID is required for creating a teacher");
    });

    it("creates teacher with full flow (account+identity -> staff details)", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock account creation — the backend provisions person and staff in the
      // same transaction and reports their ids (#2222)
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            id: 50,
            school_identity: { person_id: "100", staff_id: "7" },
          }),
      } as Response);

      // Mock staff details
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      const result = await teacherService.createTeacher({
        first_name: "Test",
        last_name: "Teacher",
        password: "SecurePass123!",
        role_id: 1,
      });

      expect(result.status).toBe("created");
      if (result.status === "created") {
        expect(result.data.first_name).toBe("Test");
        expect(result.data.last_name).toBe("Teacher");
        expect(result.data.name).toBe("Test Teacher");
        expect(result.data.temporaryCredentials).toEqual({
          email: "test.teacher@school.local",
          password: "SecurePass123!",
        });
      }
    });

    it("uses provided email instead of generating one", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock account creation with provisioned identity
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            id: 50,
            school_identity: { person_id: "100", staff_id: "7" },
          }),
      } as Response);

      // Mock staff details
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      const result = await teacherService.createTeacher({
        first_name: "Test",
        last_name: "Teacher",
        email: "custom@example.com",
        password: "SecurePass123!",
        role_id: 1,
      });

      expect(result.status).toBe("created");
      if (result.status === "created") {
        expect(result.data.email).toBe("custom@example.com");
        expect(result.data.temporaryCredentials?.email).toBe(
          "custom@example.com",
        );
      }
    });

    it("throws error when account creation fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Bad Request",
        json: () => Promise.resolve({ error: "Email already exists" }),
      } as Response);

      await expect(
        teacherService.createTeacher({
          first_name: "Test",
          last_name: "Teacher",
          password: "SecurePass123!",
          role_id: 1,
        }),
      ).rejects.toThrow(
        "Konto konnte nicht erstellt werden: Email already exists",
      );
    });

    // Successor of "throws error when person creation fails": the person is no
    // longer created by a second request, so the failure to cover is the
    // account coming back without a provisioned identity (#2222).
    it("throws when the account response carries no school identity", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ id: 50 }),
      } as Response);

      await expect(
        teacherService.createTeacher({
          first_name: "Test",
          last_name: "Teacher",
          password: "SecurePass123!",
          role_id: 1,
        }),
      ).rejects.toThrow("kein Mitarbeiter-Datensatz");
      expect(consoleSpies.error).toHaveBeenCalledWith(
        "account created without school identity",
        undefined,
      );
    });

    it("throws error when staff creation fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock successful account creation with identity
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            id: 50,
            school_identity: { person_id: "100", staff_id: "7" },
          }),
      } as Response);

      // Mock staff creation failure
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Bad Request",
        text: () => Promise.resolve("Invalid staff data"),
      } as Response);

      await expect(
        teacherService.createTeacher({
          first_name: "Test",
          last_name: "Teacher",
          password: "SecurePass123!",
          role_id: 1,
        }),
      ).rejects.toThrow(
        "Failed to create teacher: Bad Request - Invalid staff data",
      );
    });

    it("throws error when account ID is not returned", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({}),
      } as Response);

      await expect(
        teacherService.createTeacher({
          first_name: "Test",
          last_name: "Teacher",
          password: "SecurePass123!",
          role_id: 1,
        }),
      ).rejects.toThrow("Failed to get account ID from response");
      expect(consoleSpies.error).toHaveBeenCalledWith(
        "failed to get account ID from response",
        undefined,
      );
    });

    // The identity fields are what makes the backend provision the person and
    // staff record with the account. Sending them is the contract that replaced
    // the separate person request (#2222).
    it("sends the identity fields with the account creation", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            id: 50,
            school_identity: { person_id: "100", staff_id: "7" },
          }),
      } as Response);
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      await teacherService.createTeacher({
        first_name: "Test",
        last_name: "Teacher",
        tag_id: "TAG-1",
        password: "SecurePass123!",
        role_id: 1,
      });

      const registerCall = mockFetch.mock.calls.find(
        (call) => call[0] === "/api/auth/register",
      );
      expect(registerCall).toBeDefined();
      const registerInit = registerCall![1] as { body: string };
      const body = JSON.parse(registerInit.body) as Record<string, unknown>;
      expect(body.first_name).toBe("Test");
      expect(body.last_name).toBe("Teacher");
      expect(body.tag_id).toBe("TAG-1");
    });

    // The provisioned ids are bigints and travel as strings — all the way into
    // the /api/staff request, which is the only way they survive that route's
    // parse-and-re-serialize hop intact. It must be the id the backend actually
    // reported, not one a re-parse in between rounded off.
    it("passes the provisioned person id through to the staff request", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      const personId = "9007199254740993"; // 2^53 + 1

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            id: 50,
            school_identity: { person_id: personId, staff_id: "7" },
          }),
      } as Response);
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      await teacherService.createTeacher({
        first_name: "Test",
        last_name: "Teacher",
        password: "SecurePass123!",
        role_id: 1,
      });

      const staffCall = mockFetch.mock.calls.find(
        (call) => call[0] === "/api/staff",
      );
      expect(staffCall).toBeDefined();
      const staffInit = staffCall![1] as { body: string };
      const staffBody = JSON.parse(staffInit.body) as Record<string, unknown>;
      expect(staffBody.person_id).toBe(personId);
    });

    // The assertion above survives the parse only because the id is a string. As
    // a JSON number it would not: JSON.parse rounds a bigint to exactly what
    // Number() produces, so a correct and a rounded body compare equal after
    // parsing and the bug hides. This one reads the raw text, which is what the
    // /api/staff route receives — what that route then does with an id too large
    // to represent is its own test.
    it("sends the exact person id digits on the wire", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      const personId = "9007199254740993"; // 2^53 + 1

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            id: 50,
            school_identity: { person_id: personId, staff_id: "7" },
          }),
      } as Response);
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      await teacherService.createTeacher({
        first_name: "Test",
        last_name: "Teacher",
        password: "SecurePass123!",
        role_id: 1,
      });

      const staffCall = mockFetch.mock.calls.find(
        (call) => call[0] === "/api/staff",
      );
      const staffInit = staffCall![1] as { body: string };

      expect(staffInit.body).toContain(`"person_id":"${personId}"`);
      expect(staffInit.body).not.toContain("9007199254740992");
    });
  });

  describe("teacherService.updateTeacher", () => {
    it("updates teacher staff fields without person update", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock getTeacher (for current data)
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      // Mock staff update
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            ...sampleTeacher,
            specialization: "Physics",
          }),
      } as Response);

      const result = await teacherService.updateTeacher("1", {
        specialization: "Physics",
      });

      expect(result.specialization).toBe("Physics");
    });

    it("updates person fields when name/tag changes", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock getTeacher
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      // Mock person GET
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ account_id: 50 }),
      } as Response);

      // Mock person PUT
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({}),
      } as Response);

      // Mock staff update
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            ...sampleTeacher,
            first_name: "Maximilian",
          }),
      } as Response);

      const result = await teacherService.updateTeacher("1", {
        first_name: "Maximilian",
      });

      expect(result.first_name).toBe("Maximilian");
    });

    it("edits the person a bigint person_id names, digit for digit", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      // 2^53 + 1 — a JS number would round this to ...992 and edit the person
      // next door instead of failing (#2222).
      const personID = "9007199254740993";

      // getTeacher
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ ...sampleTeacher, person_id: personID }),
      } as Response);
      // person GET
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ account_id: 50 }),
      } as Response);
      // person PUT
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({}),
      } as Response);
      // staff update
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      await teacherService.updateTeacher("1", { first_name: "Maximilian" });

      const personCalls = mockFetch.mock.calls.filter((call) =>
        String(call[0]).startsWith("/api/users/"),
      );
      expect(personCalls).toHaveLength(2);
      for (const call of personCalls) {
        expect(call[0]).toBe(`/api/users/${personID}`);
      }

      const staffCall = mockFetch.mock.calls.find(
        (call) =>
          String(call[0]) === "/api/staff/1" &&
          (call[1] as RequestInit | undefined)?.method === "PUT",
      );
      expect(staffCall?.[1]?.body).toContain(`"person_id":"${personID}"`);
    });

    it("throws error when person_id is missing for person update", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock getTeacher returning teacher without person_id
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacherMinimal),
      } as Response);

      await expect(
        teacherService.updateTeacher("2", { first_name: "Updated" }),
      ).rejects.toThrow("Cannot update person fields - person_id not found");
    });

    it("throws error when person fetch fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock getTeacher
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      // Mock person GET failure
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Not Found",
      } as Response);

      await expect(
        teacherService.updateTeacher("1", { first_name: "Updated" }),
      ).rejects.toThrow("Failed to fetch person data");
    });

    it("throws error when person update fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock getTeacher
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      // Mock person GET
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ account_id: 50 }),
      } as Response);

      // Mock person PUT failure
      mockFetch.mockResolvedValueOnce({
        ok: false,
        text: () => Promise.resolve("Validation error"),
      } as Response);

      await expect(
        teacherService.updateTeacher("1", { first_name: "Updated" }),
      ).rejects.toThrow("Failed to update person: Validation error");
    });

    it("throws error when staff update fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock getTeacher
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      // Mock staff update failure
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Bad Request",
        text: () => Promise.resolve("Invalid data"),
      } as Response);

      await expect(
        teacherService.updateTeacher("1", { specialization: "New" }),
      ).rejects.toThrow("Failed to update teacher: Bad Request - Invalid data");
    });

    it("trims field values before sending", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock getTeacher
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      // Mock staff update
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      await teacherService.updateTeacher("1", {
        specialization: "  Physics  ",
        staff_notes: "  Notes  ",
      });

      // Verify the PUT call had trimmed values
      expect(mockFetch).toHaveBeenLastCalledWith(
        "/api/staff/1",
        expect.objectContaining({
          body: expect.stringContaining('"specialization":"Physics"'),
        }),
      );
    });
  });

  describe("teacherService.deleteTeacher", () => {
    it("deletes a teacher successfully", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({}),
      } as Response);

      await teacherService.deleteTeacher("1");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/staff/1",
        expect.objectContaining({
          method: "DELETE",
          credentials: "include",
        }),
      );
    });

    it("throws error when delete fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Not Found",
      } as Response);

      await expect(teacherService.deleteTeacher("999")).rejects.toThrow(
        "Failed to delete teacher: Not Found",
      );
    });

    it("logs error on failure", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      await expect(teacherService.deleteTeacher("1")).rejects.toThrow();
      expect(consoleSpies.error).toHaveBeenCalledWith(
        "error deleting teacher",
        { teacher_id: "1", error: expect.stringContaining("Network error") },
      );
    });
  });

  describe("teacherService.getTeacherActivities", () => {
    it("returns empty array and logs warning", async () => {
      const result = await teacherService.getTeacherActivities("1");

      expect(result).toEqual([]);
      expect(consoleSpies.warn).toHaveBeenCalledWith(
        "activities endpoint not implemented for staff/teachers",
        undefined,
      );
    });
  });

  describe("teacherService.createTeacher — account_exists flow", () => {
    it("returns account_exists when email already exists", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock account creation returning the German duplicate-email sentinel
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Conflict",
        json: () =>
          Promise.resolve({
            error:
              "auth error during register: Diese E-Mail-Adresse ist bereits registriert",
          }),
      } as Response);

      const result = await teacherService.createTeacher({
        first_name: "Test",
        last_name: "Teacher",
        password: "SecurePass123!",
        role_id: 1,
      });

      expect(result.status).toBe("account_exists");
      if (result.status === "account_exists") {
        expect(result.email).toBe("test.teacher@school.local");
      }
    });

    it("throws error for username conflict", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Conflict",
        json: () =>
          Promise.resolve({
            error:
              "auth error during register: Dieser Benutzername ist bereits vergeben",
          }),
      } as Response);

      await expect(
        teacherService.createTeacher({
          first_name: "Test",
          last_name: "Teacher",
          password: "SecurePass123!",
          role_id: 1,
        }),
      ).rejects.toThrow(
        "Ein Konto mit diesem Benutzernamen existiert bereits.",
      );
    });

    it("throws generic error for other account creation failures", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Bad Request",
        json: () => Promise.resolve({ error: "weak password" }),
      } as Response);

      await expect(
        teacherService.createTeacher({
          first_name: "Test",
          last_name: "Teacher",
          password: "weak",
          role_id: 1,
        }),
      ).rejects.toThrow("Konto konnte nicht erstellt werden: weak password");
    });
  });

  describe("teacherService.createTeacher — linkExisting flow", () => {
    it("links existing account and creates person + staff", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock link-to-tenant — person and staff come with the link (#2222)
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            id: 99,
            school_identity: { person_id: "200", staff_id: "8" },
          }),
      } as Response);

      // Mock staff details
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(sampleTeacher),
      } as Response);

      const result = await teacherService.createTeacher({
        first_name: "Linked",
        last_name: "User",
        email: "linked@example.com",
        role_id: 1,
        linkExisting: true,
      });

      expect(result.status).toBe("created");
      if (result.status === "created") {
        // No temporary credentials for linked accounts
        expect(result.data.temporaryCredentials).toBeUndefined();
        expect(result.data.email).toBe("linked@example.com");
      }

      // Verify it called link-to-tenant, not register
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/auth/link-to-tenant",
        expect.objectContaining({
          method: "POST",
          body: expect.stringContaining('"email":"linked@example.com"'),
        }),
      );
    });

    it("throws error when link-to-tenant fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Not Found",
        json: () => Promise.resolve({ error: "account not found" }),
      } as Response);

      await expect(
        teacherService.createTeacher({
          first_name: "Missing",
          last_name: "User",
          email: "missing@example.com",
          role_id: 1,
          linkExisting: true,
        }),
      ).rejects.toThrow(
        "Konto konnte nicht verknüpft werden: account not found",
      );
    });

    it("throws error when link response has no school identity", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({}),
      } as Response);

      await expect(
        teacherService.createTeacher({
          first_name: "No",
          last_name: "ID",
          email: "noid@example.com",
          role_id: 1,
          linkExisting: true,
        }),
      ).rejects.toThrow(
        "Der Mitarbeiter-Datensatz konnte nicht aus der Verknüpfungs-Antwort gelesen werden.",
      );
    });
  });
});
