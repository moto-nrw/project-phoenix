import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  translateApiError,
  errorTranslations,
  fetchStudentGuardians,
  fetchGuardianStudents,
  createGuardian,
  updateGuardian,
  deleteGuardian,
  linkGuardianToStudent,
  updateStudentGuardianRelationship,
  removeGuardianFromStudent,
  searchGuardians,
  fetchGuardianPhoneNumbers,
  addGuardianPhoneNumber,
  updateGuardianPhoneNumber,
  deleteGuardianPhoneNumber,
  setGuardianPrimaryPhone,
} from "./guardian-api";
import type {
  GuardianFormData,
  StudentGuardianLinkRequest,
  BackendGuardianProfile,
  BackendGuardianWithRelationship,
  PhoneNumberCreateRequest,
  PhoneNumberUpdateRequest,
  BackendPhoneNumber,
} from "./guardian-helpers";

describe("translateApiError", () => {
  it("translates 'invalid email format' to German", () => {
    expect(translateApiError("invalid email format")).toBe(
      "Ungültiges E-Mail-Format",
    );
  });

  it("translates error message case-insensitively", () => {
    expect(translateApiError("Invalid Email Format")).toBe(
      "Ungültiges E-Mail-Format",
    );
    expect(translateApiError("INVALID EMAIL FORMAT")).toBe(
      "Ungültiges E-Mail-Format",
    );
  });

  it("translates 'email already exists' to German", () => {
    expect(translateApiError("email already exists")).toBe(
      "Diese E-Mail-Adresse wird bereits verwendet",
    );
  });

  it("translates 'guardian not found' to German", () => {
    expect(translateApiError("guardian not found")).toBe(
      "Erziehungsberechtigte/r nicht gefunden",
    );
  });

  it("translates 'student not found' to German", () => {
    expect(translateApiError("student not found")).toBe(
      "Schüler/in nicht gefunden",
    );
  });

  it("translates 'relationship already exists' to German", () => {
    expect(translateApiError("relationship already exists")).toBe(
      "Diese Verknüpfung existiert bereits",
    );
  });

  it("translates 'validation failed' to German", () => {
    expect(translateApiError("validation failed")).toBe(
      "Validierung fehlgeschlagen",
    );
  });

  it("translates 'unauthorized' to German", () => {
    expect(translateApiError("unauthorized")).toBe("Keine Berechtigung");
  });

  it("translates 'forbidden' to German", () => {
    expect(translateApiError("forbidden")).toBe("Zugriff verweigert");
  });

  it("handles error patterns contained in longer messages", () => {
    expect(translateApiError("API error: invalid email format detected")).toBe(
      "Ungültiges E-Mail-Format",
    );
    expect(
      translateApiError("Error 400: email already exists in database"),
    ).toBe("Diese E-Mail-Adresse wird bereits verwendet");
  });

  it("returns generic German message for unknown errors", () => {
    expect(translateApiError("some unknown error")).toBe(
      "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
    );
    expect(translateApiError("connection timeout")).toBe(
      "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
    );
  });

  it("returns generic message for empty string", () => {
    expect(translateApiError("")).toBe(
      "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
    );
  });
});

describe("errorTranslations", () => {
  it("contains all expected error patterns", () => {
    const expectedPatterns = [
      "invalid email format",
      "email already exists",
      "guardian not found",
      "student not found",
      "relationship already exists",
      "validation failed",
      "unauthorized",
      "forbidden",
    ];

    for (const pattern of expectedPatterns) {
      expect(errorTranslations).toHaveProperty(pattern);
    }
  });

  it("all translations are non-empty strings", () => {
    for (const translation of Object.values(errorTranslations)) {
      expect(translation).toBeTruthy();
      expect(typeof translation).toBe("string");
      expect(translation.length).toBeGreaterThan(0);
    }
  });

  it("has exactly 8 error translations", () => {
    expect(Object.keys(errorTranslations).length).toBe(8);
  });
});

// Mock data helpers
const mockBackendGuardian: BackendGuardianProfile = {
  id: 1,
  first_name: "John",
  last_name: "Doe",
  email: "john@example.com",
  phone_numbers: [
    {
      id: 1,
      phone_number: "123-456-7890",
      phone_type: "home",
      is_primary: true,
      priority: 1,
    },
    {
      id: 2,
      phone_number: "098-765-4321",
      phone_type: "mobile",
      is_primary: false,
      priority: 2,
    },
  ],
  address_street: "123 Main St",
  address_city: "Anytown",
  address_postal_code: "12345",
  preferred_contact_method: "email",
  language_preference: "de",
  occupation: "Engineer",
  employer: "Tech Corp",
  notes: "Some notes",
  has_account: false,
  account_id: undefined,
};

const mockBackendGuardianWithRelationship: BackendGuardianWithRelationship = {
  guardian: mockBackendGuardian,
  relationship_id: 10,
  relationship_type: "parent",
  is_primary: true,
  is_emergency_contact: true,
  can_pickup: true,
  pickup_notes: "Can pickup anytime",
  emergency_priority: 1,
};

const mockGuardianFormData: GuardianFormData = {
  firstName: "John",
  lastName: "Doe",
  email: "john@example.com",
  addressStreet: "123 Main St",
  addressCity: "Anytown",
  addressPostalCode: "12345",
  preferredContactMethod: "email",
  languagePreference: "de",
  occupation: "Engineer",
  employer: "Tech Corp",
  notes: "Some notes",
};

const mockLinkRequest: StudentGuardianLinkRequest = {
  guardianProfileId: "1",
  relationshipType: "parent",
  isPrimary: true,
  isEmergencyContact: true,
  canPickup: true,
  pickupNotes: "Can pickup anytime",
  emergencyPriority: 1,
};

describe("guardian-api functions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("fetchStudentGuardians", () => {
    it("returns mapped guardians on success", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: [mockBackendGuardianWithRelationship],
          }),
      });

      const result = await fetchStudentGuardians("123");

      expect(result).toHaveLength(1);
      expect(result[0]).toEqual({
        id: "1",
        firstName: "John",
        lastName: "Doe",
        email: "john@example.com",
        phoneNumbers: [
          {
            id: "1",
            phoneNumber: "123-456-7890",
            phoneType: "home",
            isPrimary: true,
            priority: 1,
            label: undefined,
          },
          {
            id: "2",
            phoneNumber: "098-765-4321",
            phoneType: "mobile",
            isPrimary: false,
            priority: 2,
            label: undefined,
          },
        ],
        addressStreet: "123 Main St",
        addressCity: "Anytown",
        addressPostalCode: "12345",
        preferredContactMethod: "email",
        languagePreference: "de",
        occupation: "Engineer",
        employer: "Tech Corp",
        notes: "Some notes",
        hasAccount: false,
        accountId: undefined,
        relationshipId: "10",
        relationshipType: "parent",
        isPrimary: true,
        isEmergencyContact: true,
        canPickup: true,
        pickupNotes: "Can pickup anytime",
        emergencyPriority: 1,
      });
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/students/123/guardians",
      );
    });

    it("throws error on non-ok response with JSON error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Not Found",
        json: () => Promise.resolve({ error: "Student not found" }),
      });

      await expect(fetchStudentGuardians("999")).rejects.toThrow(
        "Student not found",
      );
    });

    it("throws error on non-ok response when JSON parse fails", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Internal Server Error",
        json: () => Promise.reject(new Error("Parse error")),
      });

      await expect(fetchStudentGuardians("123")).rejects.toThrow(
        "Failed to fetch guardians",
      );
    });

    it("throws error when status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "error",
            error: "Database error",
          }),
      });

      await expect(fetchStudentGuardians("123")).rejects.toThrow(
        "Database error",
      );
    });

    it("returns empty array when data is undefined", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: undefined,
          }),
      });

      const result = await fetchStudentGuardians("123");
      expect(result).toEqual([]);
    });
  });

  describe("fetchGuardianStudents", () => {
    it("returns students on success", async () => {
      const mockStudents = [
        {
          id: 1,
          first_name: "Jane",
          last_name: "Student",
          date_of_birth: "2015-01-01",
        },
      ];

      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: mockStudents,
          }),
      });

      const result = await fetchGuardianStudents("1");

      expect(result).toEqual(mockStudents);
      expect(global.fetch).toHaveBeenCalledWith("/api/guardians/1/students");
    });

    it("throws error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Not Found",
        json: () => Promise.resolve({ error: "Guardian not found" }),
      });

      await expect(fetchGuardianStudents("999")).rejects.toThrow(
        "Guardian not found",
      );
    });

    it("throws error when status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "error",
            error: "Access denied",
          }),
      });

      await expect(fetchGuardianStudents("1")).rejects.toThrow("Access denied");
    });

    it("returns empty array when data is undefined", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
          }),
      });

      const result = await fetchGuardianStudents("1");
      expect(result).toEqual([]);
    });
  });

  describe("createGuardian", () => {
    it("creates guardian and returns mapped result", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: mockBackendGuardian,
          }),
      });

      const result = await createGuardian(mockGuardianFormData);

      expect(result.id).toBe("1");
      expect(result.firstName).toBe("John");
      expect(result.lastName).toBe("Doe");
      expect(global.fetch).toHaveBeenCalledWith("/api/guardians", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
        body: expect.any(String),
      });
    });

    it("throws translated error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Bad Request",
        json: () => Promise.resolve({ error: "email already exists" }),
      });

      await expect(createGuardian(mockGuardianFormData)).rejects.toThrow(
        "Diese E-Mail-Adresse wird bereits verwendet",
      );
    });

    it("throws error when status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "error",
            error: "validation failed",
          }),
      });

      await expect(createGuardian(mockGuardianFormData)).rejects.toThrow(
        "Validierung fehlgeschlagen",
      );
    });

    it("throws error when data is missing", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: null,
          }),
      });

      await expect(createGuardian(mockGuardianFormData)).rejects.toThrow();
    });
  });

  describe("updateGuardian", () => {
    it("updates guardian and returns mapped result", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: { ...mockBackendGuardian, first_name: "Johnny" },
          }),
      });

      const result = await updateGuardian("1", { firstName: "Johnny" });

      expect(result.firstName).toBe("Johnny");
      expect(global.fetch).toHaveBeenCalledWith("/api/guardians/1", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
        },
        // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
        body: expect.any(String),
      });
    });

    it("only sends defined fields in partial update", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: mockBackendGuardian,
          }),
      });

      // When email is undefined, it should not be included in the request
      await updateGuardian("1", { firstName: "Johnny", email: undefined });

      const callArgs = vi.mocked(global.fetch).mock.calls[0] as [
        string,
        RequestInit,
      ];
      const body = JSON.parse(callArgs[1].body as string) as Record<
        string,
        unknown
      >;
      // mapGuardianFormToBackend only includes email if !== undefined
      expect(body).toEqual({ first_name: "Johnny" });
    });

    it("throws translated error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Not Found",
        json: () => Promise.resolve({ error: "guardian not found" }),
      });

      await expect(
        updateGuardian("999", { firstName: "Johnny" }),
      ).rejects.toThrow("Erziehungsberechtigte/r nicht gefunden");
    });

    it("throws error when status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "error",
            error: "unauthorized",
          }),
      });

      await expect(
        updateGuardian("1", { firstName: "Johnny" }),
      ).rejects.toThrow("Keine Berechtigung");
    });
  });

  describe("deleteGuardian", () => {
    it("succeeds with 204 No Content response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 204,
      });

      await expect(deleteGuardian("1")).resolves.toBeUndefined();
      expect(global.fetch).toHaveBeenCalledWith("/api/guardians/1", {
        method: "DELETE",
      });
    });

    it("succeeds with JSON success response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            status: "success",
          }),
      });

      await expect(deleteGuardian("1")).resolves.toBeUndefined();
    });

    it("throws error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Not Found",
        json: () => Promise.resolve({ error: "Guardian not found" }),
      });

      await expect(deleteGuardian("999")).rejects.toThrow("Guardian not found");
    });

    it("throws error when status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            status: "error",
            error: "Cannot delete guardian with linked students",
          }),
      });

      await expect(deleteGuardian("1")).rejects.toThrow(
        "Cannot delete guardian with linked students",
      );
    });
  });

  describe("linkGuardianToStudent", () => {
    it("links guardian to student successfully", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
          }),
      });

      await expect(
        linkGuardianToStudent("123", mockLinkRequest),
      ).resolves.toBeUndefined();

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/students/123/guardians",
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
          body: expect.any(String),
        },
      );

      const callArgs = vi.mocked(global.fetch).mock.calls[0] as [
        string,
        RequestInit,
      ];
      const body = JSON.parse(callArgs[1].body as string) as Record<
        string,
        unknown
      >;
      expect(body.guardian_profile_id).toBe(1);
      expect(body.relationship_type).toBe("parent");
      expect(body.is_primary).toBe(true);
    });

    it("throws error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Bad Request",
        json: () => Promise.resolve({ error: "Relationship already exists" }),
      });

      await expect(
        linkGuardianToStudent("123", mockLinkRequest),
      ).rejects.toThrow("Relationship already exists");
    });

    it("throws error when status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "error",
            error: "Student not found",
          }),
      });

      await expect(
        linkGuardianToStudent("999", mockLinkRequest),
      ).rejects.toThrow("Student not found");
    });
  });

  describe("updateStudentGuardianRelationship", () => {
    it("updates relationship with all fields", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
          }),
      });

      await updateStudentGuardianRelationship("10", {
        relationshipType: "guardian",
        isPrimary: false,
        isEmergencyContact: true,
        canPickup: false,
        pickupNotes: "New notes",
        emergencyPriority: 2,
      });

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/relationships/10",
        {
          method: "PUT",
          headers: {
            "Content-Type": "application/json",
          },
          // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
          body: expect.any(String),
        },
      );

      const callArgs = vi.mocked(global.fetch).mock.calls[0] as [
        string,
        RequestInit,
      ];
      const body = JSON.parse(callArgs[1].body as string) as Record<
        string,
        unknown
      >;
      expect(body).toEqual({
        relationship_type: "guardian",
        is_primary: false,
        is_emergency_contact: true,
        can_pickup: false,
        pickup_notes: "New notes",
        emergency_priority: 2,
      });
    });

    it("only sends defined fields", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
          }),
      });

      await updateStudentGuardianRelationship("10", {
        isPrimary: true,
      });

      const callArgs = vi.mocked(global.fetch).mock.calls[0] as [
        string,
        RequestInit,
      ];
      const body = JSON.parse(callArgs[1].body as string) as Record<
        string,
        unknown
      >;
      expect(body).toEqual({ is_primary: true });
    });

    it("throws error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Not Found",
        json: () => Promise.resolve({ error: "Relationship not found" }),
      });

      await expect(
        updateStudentGuardianRelationship("999", { isPrimary: true }),
      ).rejects.toThrow("Relationship not found");
    });

    it("throws error when status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "error",
            error: "Permission denied",
          }),
      });

      await expect(
        updateStudentGuardianRelationship("10", { isPrimary: true }),
      ).rejects.toThrow("Permission denied");
    });
  });

  describe("removeGuardianFromStudent", () => {
    it("succeeds with 204 No Content response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 204,
      });

      await expect(
        removeGuardianFromStudent("123", "1"),
      ).resolves.toBeUndefined();

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/students/123/guardians/1",
        {
          method: "DELETE",
        },
      );
    });

    it("succeeds with JSON success response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            status: "success",
          }),
      });

      await expect(
        removeGuardianFromStudent("123", "1"),
      ).resolves.toBeUndefined();
    });

    it("throws error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Not Found",
        json: () => Promise.resolve({ error: "Relationship not found" }),
      });

      await expect(removeGuardianFromStudent("123", "999")).rejects.toThrow(
        "Relationship not found",
      );
    });

    it("throws error when status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            status: "error",
            error: "Cannot remove primary guardian",
          }),
      });

      await expect(removeGuardianFromStudent("123", "1")).rejects.toThrow(
        "Cannot remove primary guardian",
      );
    });
  });

  describe("searchGuardians", () => {
    it("returns mapped guardians on success", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: [mockBackendGuardian],
            pagination: {
              current_page: 1,
              page_size: 10,
              total_pages: 1,
              total_records: 1,
            },
          }),
      });

      const result = await searchGuardians("john");

      expect(result).toHaveLength(1);
      expect(result[0]!.firstName).toBe("John");
      expect(global.fetch).toHaveBeenCalledWith("/api/guardians?search=john");
    });

    it("encodes search query properly", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: [],
          }),
      });

      await searchGuardians("john doe & sons");

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians?search=john%20doe%20%26%20sons",
      );
    });

    it("throws error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Unauthorized",
        json: () => Promise.resolve({ error: "Not authenticated" }),
      });

      await expect(searchGuardians("john")).rejects.toThrow(
        "Not authenticated",
      );
    });

    it("throws error when status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "error",
            error: "Search failed",
          }),
      });

      await expect(searchGuardians("john")).rejects.toThrow("Search failed");
    });

    it("returns empty array when data is undefined", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
          }),
      });

      const result = await searchGuardians("nonexistent");
      expect(result).toEqual([]);
    });

    it("handles fallback error when JSON parse fails", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Service Unavailable",
        json: () => Promise.reject(new Error("Parse error")),
      });

      // The catch block returns a generic error message without statusText
      await expect(searchGuardians("john")).rejects.toThrow(
        "Failed to search guardians",
      );
    });
  });

  // =============================================================================
  // Phone Number API Functions Tests
  // =============================================================================

  describe("fetchGuardianPhoneNumbers", () => {
    it("fetches phone numbers for a guardian", async () => {
      const mockPhoneNumbers: BackendPhoneNumber[] = [
        {
          id: 1,
          phone_number: "555-1234",
          phone_type: "mobile",
          is_primary: true,
          priority: 1,
        },
        {
          id: 2,
          phone_number: "555-5678",
          phone_type: "home",
          label: "Home Office",
          is_primary: false,
          priority: 2,
        },
      ];

      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: mockPhoneNumbers,
          }),
      });

      const result = await fetchGuardianPhoneNumbers("123");

      expect(result).toHaveLength(2);
      expect(result[0]).toEqual({
        id: "1",
        phoneNumber: "555-1234",
        phoneType: "mobile",
        label: undefined,
        isPrimary: true,
        priority: 1,
      });
      expect(result[1]).toEqual({
        id: "2",
        phoneNumber: "555-5678",
        phoneType: "home",
        label: "Home Office",
        isPrimary: false,
        priority: 2,
      });
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/123/phone-numbers",
      );
    });

    it("returns empty array when data is undefined", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: undefined,
          }),
      });

      const result = await fetchGuardianPhoneNumbers("123");
      expect(result).toEqual([]);
    });

    it("throws error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Not Found",
        json: () => Promise.resolve({ error: "Guardian not found" }),
      });

      await expect(fetchGuardianPhoneNumbers("999")).rejects.toThrow(
        "Guardian not found",
      );
    });

    it("throws error when status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "error",
            error: "Database error",
          }),
      });

      await expect(fetchGuardianPhoneNumbers("123")).rejects.toThrow(
        "Database error",
      );
    });

    it("throws fallback error when JSON parse fails", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Internal Server Error",
        json: () => Promise.reject(new Error("Parse error")),
      });

      await expect(fetchGuardianPhoneNumbers("123")).rejects.toThrow(
        "Failed to fetch phone numbers",
      );
    });
  });

  describe("addGuardianPhoneNumber", () => {
    const mockCreateRequest: PhoneNumberCreateRequest = {
      phoneNumber: "555-9999",
      phoneType: "work",
      label: "Office",
      isPrimary: false,
    };

    const mockBackendPhoneNumber: BackendPhoneNumber = {
      id: 3,
      phone_number: "555-9999",
      phone_type: "work",
      label: "Office",
      is_primary: false,
      priority: 3,
    };

    it("adds a phone number to a guardian", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: mockBackendPhoneNumber,
          }),
      });

      const result = await addGuardianPhoneNumber("123", mockCreateRequest);

      expect(result).toEqual({
        id: "3",
        phoneNumber: "555-9999",
        phoneType: "work",
        label: "Office",
        isPrimary: false,
        priority: 3,
      });

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/123/phone-numbers",
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
          body: expect.any(String),
        },
      );

      // Verify the request body was mapped correctly
      const callArgs = vi.mocked(global.fetch).mock.calls[0] as [
        string,
        RequestInit,
      ];
      const body = JSON.parse(callArgs[1].body as string) as Record<
        string,
        unknown
      >;
      expect(body).toEqual({
        phone_number: "555-9999",
        phone_type: "work",
        label: "Office",
        is_primary: false,
      });
    });

    it("adds phone number without optional fields", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: {
              id: 4,
              phone_number: "555-0000",
              phone_type: "mobile",
              is_primary: true,
              priority: 1,
            },
          }),
      });

      const minimalRequest: PhoneNumberCreateRequest = {
        phoneNumber: "555-0000",
        phoneType: "mobile",
      };

      const result = await addGuardianPhoneNumber("123", minimalRequest);

      expect(result.phoneNumber).toBe("555-0000");
      expect(result.phoneType).toBe("mobile");
    });

    it("throws translated error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Bad Request",
        json: () => Promise.resolve({ error: "validation failed" }),
      });

      await expect(
        addGuardianPhoneNumber("123", mockCreateRequest),
      ).rejects.toThrow("Validierung fehlgeschlagen");
    });

    it("throws error when status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "error",
            error: "invalid phone format",
          }),
      });

      await expect(
        addGuardianPhoneNumber("123", mockCreateRequest),
      ).rejects.toThrow();
    });

    it("throws error when data is missing", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: null,
          }),
      });

      await expect(
        addGuardianPhoneNumber("123", mockCreateRequest),
      ).rejects.toThrow();
    });

    it("throws translated fallback error when JSON parse fails", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Bad Request",
        json: () => Promise.reject(new Error("Parse error")),
      });

      await expect(
        addGuardianPhoneNumber("123", mockCreateRequest),
      ).rejects.toThrow(
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
      );
    });
  });

  describe("updateGuardianPhoneNumber", () => {
    const mockUpdateRequest: PhoneNumberUpdateRequest = {
      phoneNumber: "555-1111",
      phoneType: "mobile",
      label: "New Mobile",
    };

    it("updates a guardian phone number", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
          }),
      });

      await expect(
        updateGuardianPhoneNumber("123", "456", mockUpdateRequest),
      ).resolves.toBeUndefined();

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/123/phone-numbers/456",
        {
          method: "PUT",
          headers: {
            "Content-Type": "application/json",
          },
          // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
          body: expect.any(String),
        },
      );

      // Verify the request body was mapped correctly
      const callArgs = vi.mocked(global.fetch).mock.calls[0] as [
        string,
        RequestInit,
      ];
      const body = JSON.parse(callArgs[1].body as string) as Record<
        string,
        unknown
      >;
      expect(body).toEqual({
        phone_number: "555-1111",
        phone_type: "mobile",
        label: "New Mobile",
      });
    });

    it("updates only specified fields", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
          }),
      });

      const partialUpdate: PhoneNumberUpdateRequest = {
        phoneType: "home",
      };

      await updateGuardianPhoneNumber("123", "456", partialUpdate);

      const callArgs = vi.mocked(global.fetch).mock.calls[0] as [
        string,
        RequestInit,
      ];
      const body = JSON.parse(callArgs[1].body as string) as Record<
        string,
        unknown
      >;
      expect(body).toEqual({
        phone_type: "home",
      });
      expect(body).not.toHaveProperty("phone_number");
      expect(body).not.toHaveProperty("label");
    });

    it("throws translated error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Not Found",
        json: () => Promise.resolve({ error: "guardian not found" }),
      });

      await expect(
        updateGuardianPhoneNumber("999", "456", mockUpdateRequest),
      ).rejects.toThrow("Erziehungsberechtigte/r nicht gefunden");
    });

    it("throws error when status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "error",
            error: "unauthorized",
          }),
      });

      await expect(
        updateGuardianPhoneNumber("123", "456", mockUpdateRequest),
      ).rejects.toThrow("Keine Berechtigung");
    });

    it("throws translated fallback error when JSON parse fails", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Bad Request",
        json: () => Promise.reject(new Error("Parse error")),
      });

      await expect(
        updateGuardianPhoneNumber("123", "456", mockUpdateRequest),
      ).rejects.toThrow(
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
      );
    });
  });

  describe("deleteGuardianPhoneNumber", () => {
    it("deletes a phone number with 204 No Content response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 204,
      });

      await expect(
        deleteGuardianPhoneNumber("123", "456"),
      ).resolves.toBeUndefined();

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/123/phone-numbers/456",
        {
          method: "DELETE",
        },
      );
    });

    it("deletes a phone number with JSON success response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            status: "success",
          }),
      });

      await expect(
        deleteGuardianPhoneNumber("123", "456"),
      ).resolves.toBeUndefined();
    });

    it("throws error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Not Found",
        json: () => Promise.resolve({ error: "Phone number not found" }),
      });

      await expect(deleteGuardianPhoneNumber("123", "999")).rejects.toThrow(
        "Phone number not found",
      );
    });

    it("throws error when status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            status: "error",
            error: "Cannot delete primary phone number",
          }),
      });

      await expect(deleteGuardianPhoneNumber("123", "456")).rejects.toThrow(
        "Cannot delete primary phone number",
      );
    });

    it("throws fallback error when JSON parse fails", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Bad Request",
        json: () => Promise.reject(new Error("Parse error")),
      });

      await expect(deleteGuardianPhoneNumber("123", "456")).rejects.toThrow(
        "Failed to delete phone number",
      );
    });
  });

  describe("setGuardianPrimaryPhone", () => {
    it("sets a phone number as primary", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
          }),
      });

      await expect(
        setGuardianPrimaryPhone("123", "456"),
      ).resolves.toBeUndefined();

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/123/phone-numbers/456/set-primary",
        {
          method: "POST",
        },
      );
    });

    it("throws error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Not Found",
        json: () => Promise.resolve({ error: "Phone number not found" }),
      });

      await expect(setGuardianPrimaryPhone("123", "999")).rejects.toThrow(
        "Phone number not found",
      );
    });

    it("throws error when status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "error",
            error: "Failed to update primary phone",
          }),
      });

      await expect(setGuardianPrimaryPhone("123", "456")).rejects.toThrow(
        "Failed to update primary phone",
      );
    });

    it("throws fallback error when JSON parse fails", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Internal Server Error",
        json: () => Promise.reject(new Error("Parse error")),
      });

      await expect(setGuardianPrimaryPhone("123", "456")).rejects.toThrow(
        "Failed to set primary phone",
      );
    });

    it("handles guardian not found error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Not Found",
        json: () => Promise.resolve({ error: "Guardian not found" }),
      });

      await expect(setGuardianPrimaryPhone("999", "456")).rejects.toThrow(
        "Guardian not found",
      );
    });

    it("handles unauthorized error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Unauthorized",
        json: () => Promise.resolve({ error: "Unauthorized" }),
      });

      await expect(setGuardianPrimaryPhone("123", "456")).rejects.toThrow(
        "Unauthorized",
      );
    });
  });
});
