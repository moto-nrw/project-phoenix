import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  translateApiError,
  errorTranslations,
  fetchStudentGuardians,
  fetchGuardianStudents,
  createGuardian,
  updateGuardian,
  deleteGuardian,
  createStudentGuardians,
  fetchGuardianDeletePreview,
  linkGuardianToStudent,
  updateStudentGuardianRelationship,
  removeGuardianFromStudent,
  searchGuardians,
  fetchGuardianPhoneNumbers,
  addGuardianPhoneNumber,
  updateGuardianPhoneNumber,
  deleteGuardianPhoneNumber,
  setGuardianPrimaryPhone,
  fetchOpenInvitationDeliveriesByGuardian,
  fetchInvitationDelivery,
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

  it("translates the German duplicate-email sentinel to the guardian-context wording", () => {
    expect(
      translateApiError("Diese E-Mail-Adresse ist bereits registriert"),
    ).toBe("Diese E-Mail-Adresse wird bereits verwendet");
  });

  it("surfaces the 'use the search' guidance for a duplicate email on create (#1513)", () => {
    expect(
      translateApiError(
        'E-Mail-Adresse "peter.berger@email.de" ist bereits vergeben – bitte die vorhandene Person über die Suche auswählen',
      ),
    ).toBe(
      "Diese E-Mail-Adresse ist bereits vergeben. Bitte die vorhandene Person über die Suche auswählen.",
    );
  });

  it("translates 'guardian not found' to German", () => {
    expect(translateApiError("guardian not found")).toBe(
      "Erziehungsberechtigte/r nicht gefunden",
    );
  });

  it("translates 'student not found' to German", () => {
    expect(translateApiError("student not found")).toBe("Kind nicht gefunden");
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

  it("extracts specific reason from 'validation failed: <reason>'", () => {
    expect(
      translateApiError("validation failed: invalid phone number format"),
    ).toBe(
      "Ungültiges Telefonnummernformat (nur Ziffern, Leerzeichen, +, -, Klammern)",
    );
    expect(
      translateApiError(
        "validation failed: phone number must contain at least 3 digits",
      ),
    ).toBe("Telefonnummer muss mindestens 3 Ziffern enthalten");
    expect(translateApiError("validation failed: invalid email format")).toBe(
      "Ungültiges E-Mail-Format",
    );
  });

  it("falls back to generic 'validation failed' when reason is unknown", () => {
    expect(translateApiError("validation failed: some unknown reason")).toBe(
      "Validierung fehlgeschlagen",
    );
  });

  it("handles 'validation failed' without colon", () => {
    expect(translateApiError("validation failed")).toBe(
      "Validierung fehlgeschlagen",
    );
    expect(translateApiError("something validation failed something")).toBe(
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
      translateApiError(
        "auth error during register: Diese E-Mail-Adresse ist bereits registriert",
      ),
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
      "bereits registriert",
      "guardian not found",
      "student not found",
      "relationship already exists",
      "validation failed",
      "invalid phone number format",
      "phone number must contain at least 3 digits",
      "phone number is required",
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

  it("has exactly 12 error translations", () => {
    expect(Object.keys(errorTranslations).length).toBe(12);
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
  notes: "Some notes",
};

const mockLinkRequest: StudentGuardianLinkRequest = {
  guardianProfileId: "1",
  relationshipType: "parent",
  guardianRole: "primary_guardian",
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
        notes: "Some notes",
        hasAccount: false,
        accountId: undefined,
        relationshipId: "10",
        relationshipType: "parent",
        guardianRole: "custom",
        isPrimary: true,
        isEmergencyContact: true,
        canPickup: true,
        pickupNotes: "Can pickup anytime",
        emergencyPriority: 1,
        accountStatus: "none",
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
        body: expect.any(String),
      });
    });

    it("throws translated error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Bad Request",
        json: () =>
          Promise.resolve({
            error: "Diese E-Mail-Adresse ist bereits registriert",
          }),
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

    it("appends force=true and expected link IDs to the URL for a full delete", async () => {
      global.fetch = vi.fn().mockResolvedValue({ ok: true, status: 204 });

      await expect(
        deleteGuardian("1", {
          force: true,
          expectedAffectedLinkIds: ["10", "20"],
        }),
      ).resolves.toBeUndefined();
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/1?force=true&expected_link_ids=10%2C20",
        {
          method: "DELETE",
        },
      );
    });

    it("throws a GuardianApiError carrying the HTTP status on conflict", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 409,
        statusText: "Conflict",
        json: () => Promise.resolve({ error: "Noch mit Kindern verknüpft" }),
      });

      await expect(deleteGuardian("1")).rejects.toMatchObject({
        name: "GuardianApiError",
        status: 409,
        message: "Noch mit Kindern verknüpft",
      });
    });
  });

  describe("fetchGuardianDeletePreview", () => {
    it("maps the snake_case preview payload to camelCase", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: {
              linked_count: 2,
              affected_names: ["Anna Müller", "Ben Müller"],
              affected_link_ids: [10, 20],
              warning: "Die Person ist mit 2 Kindern verknüpft …",
            },
          }),
      });

      await expect(fetchGuardianDeletePreview("1")).resolves.toEqual({
        linkedCount: 2,
        affectedNames: ["Anna Müller", "Ben Müller"],
        affectedLinkIds: ["10", "20"],
        warning: "Die Person ist mit 2 Kindern verknüpft …",
      });
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/1/delete-preview",
      );
    });

    it("throws a GuardianApiError carrying the status when the backend forbids it", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 403,
        statusText: "Forbidden",
        json: () => Promise.resolve({ error: "nur Administratoren" }),
      });

      await expect(fetchGuardianDeletePreview("1")).rejects.toMatchObject({
        name: "GuardianApiError",
        status: 403,
        message: "nur Administratoren",
      });
    });
  });

  describe("createStudentGuardians", () => {
    it("posts the batch as snake_case to the /batch endpoint", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 201,
        json: () => Promise.resolve({ status: "success" }),
      });

      await expect(
        createStudentGuardians("123", [
          {
            firstName: "Atomic",
            lastName: "Guardian",
            email: "a@b.de",
            relationshipType: "parent",
            guardianRole: "legal_guardian",
            isPrimary: true,
            isEmergencyContact: false,
            canPickup: true,
            emergencyPriority: 1,
            phoneNumbers: [
              {
                phoneNumber: "+49 1",
                phoneType: "mobile",
                isPrimary: true,
              },
            ],
          },
        ]),
      ).resolves.toBeUndefined();

      const [url, init] = (global.fetch as ReturnType<typeof vi.fn>).mock
        .calls[0] as [string, RequestInit];
      expect(url).toBe("/api/guardians/students/123/guardians/batch");
      expect(init.method).toBe("POST");
      expect(JSON.parse(init.body as string)).toEqual({
        guardians: [
          expect.objectContaining({
            first_name: "Atomic",
            last_name: "Guardian",
            email: "a@b.de",
            relationship_type: "parent",
            guardian_role: "legal_guardian",
            is_primary: true,
            can_pickup: true,
            emergency_priority: 1,
            phone_numbers: [
              expect.objectContaining({
                phone_number: "+49 1",
                phone_type: "mobile",
                is_primary: true,
              }),
            ],
          }),
        ],
      });
    });

    it("translates a duplicate-email 400 into the German guidance message", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 400,
        statusText: "Bad Request",
        json: () =>
          Promise.resolve({
            error:
              'Erziehungsberechtigte/r 1: E-Mail-Adresse "a@b.de" ist bereits vergeben',
          }),
      });

      await expect(
        createStudentGuardians("123", [
          {
            firstName: "A",
            lastName: "B",
            relationshipType: "parent",
            isPrimary: false,
            isEmergencyContact: false,
            canPickup: false,
            guardianRole: "pickup_only",
            emergencyPriority: 1,
          },
        ]),
      ).rejects.toThrow(
        "Diese E-Mail-Adresse ist bereits vergeben. Bitte die vorhandene Person über die Suche auswählen.",
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
      expect(body.guardian_role).toBe(mockLinkRequest.guardianRole);
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
        guardianRole: "co_guardian",
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
        guardian_role: "co_guardian",
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
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/search?q=john&page_size=50",
      );
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
        "/api/guardians/search?q=john%20doe%20%26%20sons&page_size=50",
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

  // Email delivery status (#1937).
  describe("fetchOpenInvitationDeliveriesByGuardian", () => {
    it("maps guardian profile id to the newest open invitation and delivery summary", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: [
              {
                id: 9,
                guardian_profile_id: 5,
                delivery: {
                  invitation_id: 9,
                  attempts: [
                    {
                      outbox_id: 90,
                      dispatch_status: "sent",
                      delivery_status: "delivered",
                      queued_at: "2026-07-25T10:00:00Z",
                      attempts: 1,
                    },
                  ],
                },
              },
              // Older invitation for the same guardian — the list is
              // newest-first, so the first one seen wins.
              {
                id: 3,
                guardian_profile_id: 5,
                delivery: { invitation_id: 3, attempts: [] },
              },
              {
                id: 7,
                guardian_profile_id: 8,
                delivery: { invitation_id: 7, attempts: [] },
              },
            ],
          }),
      });

      const result = await fetchOpenInvitationDeliveriesByGuardian(["5", "8"]);

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/invitations/pending?guardian_profile_ids=5%2C8",
      );
      expect(result.get("5")?.invitationId).toBe("9");
      expect(result.get("5")?.delivery.attempts[0]?.deliveryStatus).toBe(
        "delivered",
      );
      expect(result.get("8")?.invitationId).toBe("7");
      expect(result.size).toBe(2);
    });

    it("throws on a failed request", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Forbidden",
      });

      await expect(
        fetchOpenInvitationDeliveriesByGuardian(["5"]),
      ).rejects.toThrow("Forbidden");
    });

    it("throws when the API reports an error payload", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ status: "error", error: "nope" }),
      });

      await expect(
        fetchOpenInvitationDeliveriesByGuardian(["5"]),
      ).rejects.toThrow("nope");
    });
  });

  describe("fetchInvitationDelivery", () => {
    it("returns the mapped delivery status", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
            data: {
              invitation_id: 42,
              attempts: [
                {
                  outbox_id: 7,
                  dispatch_status: "sent",
                  delivery_status: "deferred",
                  queued_at: "2026-07-25T10:00:00Z",
                  attempts: 1,
                },
              ],
            },
          }),
      });

      const result = await fetchInvitationDelivery("42");

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/invitations/42/delivery",
      );
      expect(result.invitationId).toBe("42");
      expect(result.attempts[0]?.deliveryStatus).toBe("deferred");
    });

    it("throws on a failed request", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Not Found",
      });

      await expect(fetchInvitationDelivery("42")).rejects.toThrow("Not Found");
    });

    it("throws when the API reports an error payload", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ status: "error", error: "boom" }),
      });

      await expect(fetchInvitationDelivery("42")).rejects.toThrow("boom");
    });
  });
});
