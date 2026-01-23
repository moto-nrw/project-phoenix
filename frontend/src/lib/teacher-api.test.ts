/* eslint-disable @typescript-eslint/no-empty-function */
/* eslint-disable @typescript-eslint/no-unsafe-call */
/* eslint-disable @typescript-eslint/no-unsafe-member-access */
/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { Teacher } from "./teacher-api";

// Import after mocks are set up
import { teacherService } from "./teacher-api";

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
  person_id: 100,
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
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;
  let consoleWarnSpy: ReturnType<typeof vi.spyOn>;
  let originalFetch: typeof fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn();
  });

  afterEach(() => {
    consoleErrorSpy.mockRestore();
    consoleWarnSpy.mockRestore();
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
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Unexpected response format:",
        expect.anything(),
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
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Error fetching teachers:",
        expect.any(Error),
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
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Error fetching teacher with ID 1:",
        expect.any(Error),
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

    it("creates teacher with full flow (account -> person -> staff)", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock account creation
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ id: 50 }),
      } as Response);

      // Mock person creation
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: { data: { id: 100 } } }),
      } as Response);

      // Mock staff creation
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

      expect(result.first_name).toBe("Test");
      expect(result.last_name).toBe("Teacher");
      expect(result.name).toBe("Test Teacher");
      expect(result.temporaryCredentials).toEqual({
        email: "test.teacher@school.local",
        password: "SecurePass123!",
      });
    });

    it("uses provided email instead of generating one", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock account creation
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ id: 50 }),
      } as Response);

      // Mock person creation
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: { id: 100 } }),
      } as Response);

      // Mock staff creation
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

      expect(result.email).toBe("custom@example.com");
      expect(result.temporaryCredentials?.email).toBe("custom@example.com");
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
      ).rejects.toThrow("Failed to create account: Email already exists");
    });

    it("throws error when person creation fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock successful account creation
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ id: 50 }),
      } as Response);

      // Mock person creation failure
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Bad Request",
        json: () => Promise.resolve({ message: "Invalid person data" }),
      } as Response);

      await expect(
        teacherService.createTeacher({
          first_name: "Test",
          last_name: "Teacher",
          password: "SecurePass123!",
          role_id: 1,
        }),
      ).rejects.toThrow("Failed to create person: Invalid person data");
    });

    it("throws error when staff creation fails", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock successful account creation
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ id: 50 }),
      } as Response);

      // Mock successful person creation
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: { id: 100 } }),
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
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Failed to get account ID from response:",
        expect.anything(),
      );
    });

    it("throws error when person ID is not returned", async () => {
      const mockFetch = globalThis.fetch as ReturnType<typeof vi.fn>;

      // Mock successful account creation
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ id: 50 }),
      } as Response);

      // Mock person creation with unexpected format
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
      ).rejects.toThrow("Failed to get person ID from response");
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
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Error deleting teacher with ID 1:",
        expect.any(Error),
      );
    });
  });

  describe("teacherService.getTeacherActivities", () => {
    it("returns empty array and logs warning", async () => {
      const result = await teacherService.getTeacherActivities("1");

      expect(result).toEqual([]);
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        "Activities endpoint not implemented for staff/teachers",
      );
    });
  });
});
