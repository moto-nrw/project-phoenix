import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  fetchStudentGuardians,
  updateGuardian,
  deleteGuardian,
  createStudentGuardians,
  fetchGuardianDeletePreview,
  linkGuardianToStudent,
  updateStudentGuardianRelationship,
  removeGuardianFromStudent,
  searchGuardians,
  addGuardianPhoneNumber,
  updateGuardianPhoneNumber,
  deleteGuardianPhoneNumber,
  setGuardianPrimaryPhone,
} from "./guardian-api";
import type {
  StudentGuardianLinkRequest,
  BackendGuardianProfile,
  BackendGuardianWithRelationship,
  PhoneNumberCreateRequest,
  PhoneNumberUpdateRequest,
  BackendPhoneNumber,
} from "./guardian-helpers";

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
        // A payload without the payment mark (#2608) maps to false.
        isPayer: false,
        pickupNotes: "Can pickup anytime",
        emergencyPriority: 1,
        accountStatus: "none",
      });
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/guardians/students/123/guardians",
        undefined,
      );
    });

    it("throws error on non-ok response with JSON error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Not Found",
        json: () => Promise.resolve({ detail: "Student not found" }),
      });

      await expect(fetchStudentGuardians("999")).rejects.toThrow(
        "Failed to fetch guardians",
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

    it("throws the status class code on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 404,
        statusText: "Not Found",
        json: () => Promise.resolve({ detail: "guardian not found" }),
      });

      await expect(
        updateGuardian("999", { firstName: "Johnny" }),
      ).rejects.toMatchObject({
        name: "ApiError",
        status: 404,
        code: "general.input",
        message: "Failed to update guardian",
        detail: "guardian not found",
      });
    });

    it("rejects a success answer without data", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
          }),
      });

      await expect(
        updateGuardian("1", { firstName: "Johnny" }),
      ).rejects.toMatchObject({
        status: 500,
        code: "general.server",
      });
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
        json: () => Promise.resolve({ detail: "Guardian not found" }),
      });

      await expect(deleteGuardian("999")).rejects.toThrow(
        "Failed to delete guardian",
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
        json: () => Promise.resolve({ detail: "Noch mit Kindern verknüpft" }),
      });

      await expect(deleteGuardian("1")).rejects.toMatchObject({
        name: "ApiError",
        status: 409,
        message: "Failed to delete guardian",
        detail: "Noch mit Kindern verknüpft",
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
        undefined,
      );
    });

    it("throws a GuardianApiError carrying the status when the backend forbids it", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 403,
        statusText: "Forbidden",
        json: () => Promise.resolve({ detail: "nur Administratoren" }),
      });

      await expect(fetchGuardianDeletePreview("1")).rejects.toMatchObject({
        name: "ApiError",
        status: 403,
        message: "Failed to load delete preview",
        detail: "nur Administratoren",
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

    it("keeps the wire code of a duplicate-email 400", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 400,
        statusText: "Bad Request",
        json: () =>
          Promise.resolve({
            detail:
              'Erziehungsberechtigte/r 1: E-Mail-Adresse "a@b.de" ist bereits vergeben',
            code: "general.input",
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
      ).rejects.toMatchObject({
        name: "ApiError",
        status: 400,
        code: "general.input",
      });
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
        json: () => Promise.resolve({ detail: "Relationship already exists" }),
      });

      await expect(
        linkGuardianToStudent("123", mockLinkRequest),
      ).rejects.toThrow("Failed to link guardian");
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
        json: () => Promise.resolve({ detail: "Relationship not found" }),
      });

      await expect(
        updateStudentGuardianRelationship("999", { isPrimary: true }),
      ).rejects.toThrow("Failed to update relationship");
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
        json: () => Promise.resolve({ detail: "Relationship not found" }),
      });

      await expect(removeGuardianFromStudent("123", "999")).rejects.toThrow(
        "Failed to remove guardian",
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
        undefined,
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
        undefined,
      );
    });

    it("throws error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Unauthorized",
        json: () => Promise.resolve({ detail: "Not authenticated" }),
      });

      await expect(searchGuardians("john")).rejects.toThrow(
        "Failed to search guardians",
      );
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

    it("throws the backend message as diagnostic on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Bad Request",
        json: () => Promise.resolve({ detail: "validation failed" }),
      });

      await expect(
        addGuardianPhoneNumber("123", mockCreateRequest),
      ).rejects.toThrow("Failed to add phone number");
    });

    it("rejects a success answer without data", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            status: "success",
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

    it("throws the fallback diagnostic when JSON parse fails", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 400,
        statusText: "Bad Request",
        json: () => Promise.reject(new Error("Parse error")),
      });

      await expect(
        addGuardianPhoneNumber("123", mockCreateRequest),
      ).rejects.toMatchObject({
        message: "Failed to add phone number",
        code: "general.input",
      });
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

    it("throws the backend message as diagnostic on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Not Found",
        json: () => Promise.resolve({ detail: "guardian not found" }),
      });

      await expect(
        updateGuardianPhoneNumber("999", "456", mockUpdateRequest),
      ).rejects.toThrow("Failed to update phone number");
    });

    it("throws the fallback diagnostic when JSON parse fails", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Bad Request",
        json: () => Promise.reject(new Error("Parse error")),
      });

      await expect(
        updateGuardianPhoneNumber("123", "456", mockUpdateRequest),
      ).rejects.toThrow("Failed to update phone number");
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
        json: () => Promise.resolve({ detail: "Phone number not found" }),
      });

      await expect(deleteGuardianPhoneNumber("123", "999")).rejects.toThrow(
        "Failed to delete phone number",
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
        json: () => Promise.resolve({ detail: "Phone number not found" }),
      });

      await expect(setGuardianPrimaryPhone("123", "999")).rejects.toThrow(
        "Failed to set primary phone",
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
        json: () => Promise.resolve({ detail: "Guardian not found" }),
      });

      await expect(setGuardianPrimaryPhone("999", "456")).rejects.toThrow(
        "Failed to set primary phone",
      );
    });

    it("handles unauthorized error", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        statusText: "Unauthorized",
        json: () => Promise.resolve({ detail: "Unauthorized" }),
      });

      await expect(setGuardianPrimaryPhone("123", "456")).rejects.toThrow(
        "Failed to set primary phone",
      );
    });
  });
});

describe("guardian-api transport failures", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  // #2517: a request that never reaches the API is general.unavailable, not
  // a raw "Failed to fetch".
  it("turns a network failure into general.unavailable", async () => {
    global.fetch = vi.fn().mockRejectedValue(new TypeError("Failed to fetch"));

    await expect(fetchStudentGuardians("1")).rejects.toMatchObject({
      name: "TypeError",
      code: "general.unavailable",
    });
  });
});
