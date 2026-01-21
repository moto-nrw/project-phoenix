import { describe, it, expect } from "vitest";
import {
  mapGuardianResponse,
  mapGuardianWithRelationshipResponse,
  mapGuardianFormDataToBackend,
  mapStudentGuardianLinkToBackend,
  mapGuardianSearchResultResponse,
  getGuardianFullName,
  getGuardianPrimaryContact,
  getRelationshipTypeLabel,
  RELATIONSHIP_TYPES,
  CONTACT_METHODS,
  LANGUAGE_PREFERENCES,
  type BackendGuardianProfile,
  type BackendGuardianWithRelationship,
  type BackendGuardianSearchResult,
  type GuardianFormData,
  type StudentGuardianLinkRequest,
  type Guardian,
} from "./guardian-helpers";

describe("guardian-helpers", () => {
  describe("mapGuardianResponse", () => {
    it("maps all fields from backend to frontend format", () => {
      const backendData: BackendGuardianProfile = {
        id: 123,
        first_name: "John",
        last_name: "Doe",
        email: "john.doe@example.com",
        phone: "030-12345678",
        mobile_phone: "0170-12345678",
        address_street: "Hauptstraße 1",
        address_city: "Berlin",
        address_postal_code: "10115",
        preferred_contact_method: "email",
        language_preference: "de",
        occupation: "Engineer",
        employer: "Tech Corp",
        notes: "Some notes",
        has_account: true,
        account_id: 456,
      };

      const result = mapGuardianResponse(backendData);

      expect(result).toEqual({
        id: "123",
        firstName: "John",
        lastName: "Doe",
        email: "john.doe@example.com",
        phone: "030-12345678",
        mobilePhone: "0170-12345678",
        addressStreet: "Hauptstraße 1",
        addressCity: "Berlin",
        addressPostalCode: "10115",
        preferredContactMethod: "email",
        languagePreference: "de",
        occupation: "Engineer",
        employer: "Tech Corp",
        notes: "Some notes",
        hasAccount: true,
        accountId: "456",
      });
    });

    it("converts numeric id to string", () => {
      const backendData: BackendGuardianProfile = {
        id: 999,
        first_name: "Jane",
        last_name: "Smith",
        preferred_contact_method: "phone",
        language_preference: "en",
        has_account: false,
      };

      const result = mapGuardianResponse(backendData);

      expect(result.id).toBe("999");
      expect(typeof result.id).toBe("string");
    });

    it("handles optional fields when undefined", () => {
      const backendData: BackendGuardianProfile = {
        id: 1,
        first_name: "Test",
        last_name: "User",
        preferred_contact_method: "email",
        language_preference: "de",
        has_account: false,
        email: undefined,
        phone: undefined,
        mobile_phone: undefined,
        address_street: undefined,
        address_city: undefined,
        address_postal_code: undefined,
        occupation: undefined,
        employer: undefined,
        notes: undefined,
        account_id: undefined,
      };

      const result = mapGuardianResponse(backendData);

      expect(result.email).toBeUndefined();
      expect(result.phone).toBeUndefined();
      expect(result.mobilePhone).toBeUndefined();
      expect(result.addressStreet).toBeUndefined();
      expect(result.addressCity).toBeUndefined();
      expect(result.addressPostalCode).toBeUndefined();
      expect(result.occupation).toBeUndefined();
      expect(result.employer).toBeUndefined();
      expect(result.notes).toBeUndefined();
      expect(result.accountId).toBeUndefined();
    });

    it("handles guardian without account", () => {
      const backendData: BackendGuardianProfile = {
        id: 1,
        first_name: "Test",
        last_name: "User",
        preferred_contact_method: "mobile",
        language_preference: "tr",
        has_account: false,
      };

      const result = mapGuardianResponse(backendData);

      expect(result.hasAccount).toBe(false);
      expect(result.accountId).toBeUndefined();
    });
  });

  describe("mapGuardianWithRelationshipResponse", () => {
    it("maps guardian with relationship data", () => {
      const backendData: BackendGuardianWithRelationship = {
        guardian: {
          id: 100,
          first_name: "Parent",
          last_name: "One",
          email: "parent@example.com",
          preferred_contact_method: "email",
          language_preference: "de",
          has_account: true,
          account_id: 200,
        },
        relationship_id: 50,
        relationship_type: "parent",
        is_primary: true,
        is_emergency_contact: true,
        can_pickup: true,
        pickup_notes: "Usually picks up at 15:00",
        emergency_priority: 1,
      };

      const result = mapGuardianWithRelationshipResponse(backendData);

      expect(result).toEqual({
        id: "100",
        firstName: "Parent",
        lastName: "One",
        email: "parent@example.com",
        phone: undefined,
        mobilePhone: undefined,
        addressStreet: undefined,
        addressCity: undefined,
        addressPostalCode: undefined,
        preferredContactMethod: "email",
        languagePreference: "de",
        occupation: undefined,
        employer: undefined,
        notes: undefined,
        hasAccount: true,
        accountId: "200",
        relationshipId: "50",
        relationshipType: "parent",
        isPrimary: true,
        isEmergencyContact: true,
        canPickup: true,
        pickupNotes: "Usually picks up at 15:00",
        emergencyPriority: 1,
      });
    });

    it("handles non-primary, non-emergency guardian", () => {
      const backendData: BackendGuardianWithRelationship = {
        guardian: {
          id: 1,
          first_name: "Relative",
          last_name: "Person",
          preferred_contact_method: "phone",
          language_preference: "en",
          has_account: false,
        },
        relationship_id: 10,
        relationship_type: "relative",
        is_primary: false,
        is_emergency_contact: false,
        can_pickup: false,
        emergency_priority: 3,
      };

      const result = mapGuardianWithRelationshipResponse(backendData);

      expect(result.isPrimary).toBe(false);
      expect(result.isEmergencyContact).toBe(false);
      expect(result.canPickup).toBe(false);
      expect(result.pickupNotes).toBeUndefined();
      expect(result.emergencyPriority).toBe(3);
    });
  });

  describe("mapGuardianSearchResultResponse", () => {
    it("maps guardian search result with students", () => {
      const backendData: BackendGuardianSearchResult = {
        id: 123,
        first_name: "Anna",
        last_name: "Müller",
        email: "anna.mueller@example.com",
        phone: "030-12345678",
        students: [
          {
            student_id: 1,
            first_name: "Max",
            last_name: "Müller",
            school_class: "1a",
          },
          {
            student_id: 2,
            first_name: "Lisa",
            last_name: "Müller",
            school_class: "3b",
          },
        ],
      };

      const result = mapGuardianSearchResultResponse(backendData);

      expect(result).toEqual({
        id: "123",
        firstName: "Anna",
        lastName: "Müller",
        email: "anna.mueller@example.com",
        phone: "030-12345678",
        students: [
          {
            studentId: "1",
            firstName: "Max",
            lastName: "Müller",
            schoolClass: "1a",
          },
          {
            studentId: "2",
            firstName: "Lisa",
            lastName: "Müller",
            schoolClass: "3b",
          },
        ],
      });
    });

    it("handles guardian with no students", () => {
      const backendData: BackendGuardianSearchResult = {
        id: 456,
        first_name: "Peter",
        last_name: "Schmidt",
        students: [],
      };

      const result = mapGuardianSearchResultResponse(backendData);

      expect(result.id).toBe("456");
      expect(result.firstName).toBe("Peter");
      expect(result.lastName).toBe("Schmidt");
      expect(result.students).toEqual([]);
      expect(result.email).toBeUndefined();
      expect(result.phone).toBeUndefined();
    });

    it("converts numeric ids to strings", () => {
      const backendData: BackendGuardianSearchResult = {
        id: 999,
        first_name: "Test",
        last_name: "User",
        students: [
          {
            student_id: 888,
            first_name: "Child",
            last_name: "User",
            school_class: "2c",
          },
        ],
      };

      const result = mapGuardianSearchResultResponse(backendData);

      expect(result.id).toBe("999");
      expect(typeof result.id).toBe("string");
      expect(result.students[0]?.studentId).toBe("888");
      expect(typeof result.students[0]?.studentId).toBe("string");
    });

    it("handles optional email and phone fields", () => {
      const backendData: BackendGuardianSearchResult = {
        id: 1,
        first_name: "Only",
        last_name: "Name",
        email: undefined,
        phone: undefined,
        students: [],
      };

      const result = mapGuardianSearchResultResponse(backendData);

      expect(result.email).toBeUndefined();
      expect(result.phone).toBeUndefined();
    });
  });

  describe("mapGuardianFormDataToBackend", () => {
    it("maps all form fields to backend format", () => {
      const formData: GuardianFormData = {
        firstName: "John",
        lastName: "Doe",
        email: "john@example.com",
        phone: "030-12345678",
        mobilePhone: "0170-12345678",
        addressStreet: "Musterstraße 42",
        addressCity: "München",
        addressPostalCode: "80331",
        preferredContactMethod: "mobile",
        languagePreference: "en",
        occupation: "Doctor",
        employer: "Hospital",
        notes: "Available after 17:00",
      };

      const result = mapGuardianFormDataToBackend(formData);

      expect(result).toEqual({
        first_name: "John",
        last_name: "Doe",
        email: "john@example.com",
        phone: "030-12345678",
        mobile_phone: "0170-12345678",
        address_street: "Musterstraße 42",
        address_city: "München",
        address_postal_code: "80331",
        preferred_contact_method: "mobile",
        language_preference: "en",
        occupation: "Doctor",
        employer: "Hospital",
        notes: "Available after 17:00",
      });
    });

    it("uses default contact method when not provided", () => {
      const formData: GuardianFormData = {
        firstName: "Test",
        lastName: "User",
      };

      const result = mapGuardianFormDataToBackend(formData);

      expect(result.preferred_contact_method).toBe("email");
    });

    it("uses default language preference when not provided", () => {
      const formData: GuardianFormData = {
        firstName: "Test",
        lastName: "User",
      };

      const result = mapGuardianFormDataToBackend(formData);

      expect(result.language_preference).toBe("de");
    });

    it("handles minimal required fields", () => {
      const formData: GuardianFormData = {
        firstName: "Minimal",
        lastName: "User",
      };

      const result = mapGuardianFormDataToBackend(formData);

      expect(result.first_name).toBe("Minimal");
      expect(result.last_name).toBe("User");
      expect(result.email).toBeUndefined();
      expect(result.phone).toBeUndefined();
      expect(result.mobile_phone).toBeUndefined();
    });
  });

  describe("mapStudentGuardianLinkToBackend", () => {
    it("maps link request to backend format", () => {
      const linkRequest: StudentGuardianLinkRequest = {
        guardianProfileId: "123",
        relationshipType: "parent",
        isPrimary: true,
        isEmergencyContact: true,
        canPickup: true,
        pickupNotes: "ID required",
        emergencyPriority: 1,
      };

      const result = mapStudentGuardianLinkToBackend(linkRequest);

      expect(result).toEqual({
        guardian_profile_id: 123,
        relationship_type: "parent",
        is_primary: true,
        is_emergency_contact: true,
        can_pickup: true,
        pickup_notes: "ID required",
        emergency_priority: 1,
      });
    });

    it("converts string guardianProfileId to number", () => {
      const linkRequest: StudentGuardianLinkRequest = {
        guardianProfileId: "999",
        relationshipType: "guardian",
        isPrimary: false,
        isEmergencyContact: false,
        canPickup: false,
        emergencyPriority: 2,
      };

      const result = mapStudentGuardianLinkToBackend(linkRequest);

      expect(result.guardian_profile_id).toBe(999);
      expect(typeof result.guardian_profile_id).toBe("number");
    });

    it("handles optional pickup notes when undefined", () => {
      const linkRequest: StudentGuardianLinkRequest = {
        guardianProfileId: "1",
        relationshipType: "other",
        isPrimary: false,
        isEmergencyContact: true,
        canPickup: true,
        emergencyPriority: 3,
      };

      const result = mapStudentGuardianLinkToBackend(linkRequest);

      expect(result.pickup_notes).toBeUndefined();
    });
  });

  describe("getGuardianFullName", () => {
    it("returns full name with first and last name", () => {
      const guardian: Guardian = {
        id: "1",
        firstName: "John",
        lastName: "Doe",
        preferredContactMethod: "email",
        languagePreference: "de",
        hasAccount: false,
      };

      const result = getGuardianFullName(guardian);

      expect(result).toBe("John Doe");
    });

    it("handles names with multiple parts", () => {
      const guardian: Guardian = {
        id: "1",
        firstName: "Anna-Maria",
        lastName: "von der Heide",
        preferredContactMethod: "email",
        languagePreference: "de",
        hasAccount: false,
      };

      const result = getGuardianFullName(guardian);

      expect(result).toBe("Anna-Maria von der Heide");
    });

    it("handles single character names", () => {
      const guardian: Guardian = {
        id: "1",
        firstName: "A",
        lastName: "B",
        preferredContactMethod: "email",
        languagePreference: "de",
        hasAccount: false,
      };

      const result = getGuardianFullName(guardian);

      expect(result).toBe("A B");
    });
  });

  describe("getGuardianPrimaryContact", () => {
    it("returns email when preferred method is email", () => {
      const guardian: Guardian = {
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        phone: "030-123456",
        mobilePhone: "0170-123456",
        preferredContactMethod: "email",
        languagePreference: "de",
        hasAccount: false,
      };

      const result = getGuardianPrimaryContact(guardian);

      expect(result).toBe("test@example.com");
    });

    it("returns mobile phone when preferred method is mobile", () => {
      const guardian: Guardian = {
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        phone: "030-123456",
        mobilePhone: "0170-123456",
        preferredContactMethod: "mobile",
        languagePreference: "de",
        hasAccount: false,
      };

      const result = getGuardianPrimaryContact(guardian);

      expect(result).toBe("0170-123456");
    });

    it("returns phone when preferred method is phone", () => {
      const guardian: Guardian = {
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        phone: "030-123456",
        mobilePhone: "0170-123456",
        preferredContactMethod: "phone",
        languagePreference: "de",
        hasAccount: false,
      };

      const result = getGuardianPrimaryContact(guardian);

      expect(result).toBe("030-123456");
    });

    it("falls back to email when preferred method unavailable", () => {
      const guardian: Guardian = {
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        preferredContactMethod: "mobile", // Preferred mobile but not available
        languagePreference: "de",
        hasAccount: false,
      };

      const result = getGuardianPrimaryContact(guardian);

      expect(result).toBe("test@example.com");
    });

    it("falls back to mobile phone when email unavailable", () => {
      const guardian: Guardian = {
        id: "1",
        firstName: "Test",
        lastName: "User",
        mobilePhone: "0170-123456",
        preferredContactMethod: "email", // Preferred email but not available
        languagePreference: "de",
        hasAccount: false,
      };

      const result = getGuardianPrimaryContact(guardian);

      expect(result).toBe("0170-123456");
    });

    it("falls back to phone when email and mobile unavailable", () => {
      const guardian: Guardian = {
        id: "1",
        firstName: "Test",
        lastName: "User",
        phone: "030-123456",
        preferredContactMethod: "email", // Preferred email but not available
        languagePreference: "de",
        hasAccount: false,
      };

      const result = getGuardianPrimaryContact(guardian);

      expect(result).toBe("030-123456");
    });

    it("returns fallback message when no contact data available", () => {
      const guardian: Guardian = {
        id: "1",
        firstName: "Test",
        lastName: "User",
        preferredContactMethod: "email",
        languagePreference: "de",
        hasAccount: false,
      };

      const result = getGuardianPrimaryContact(guardian);

      expect(result).toBe("Keine Kontaktdaten");
    });
  });

  describe("getRelationshipTypeLabel", () => {
    it("returns label for parent relationship", () => {
      const result = getRelationshipTypeLabel("parent");

      expect(result).toBe("Elternteil");
    });

    it("returns label for guardian relationship", () => {
      const result = getRelationshipTypeLabel("guardian");

      expect(result).toBe("Vormund");
    });

    it("returns label for relative relationship", () => {
      const result = getRelationshipTypeLabel("relative");

      expect(result).toBe("Verwandte/r");
    });

    it("returns label for other relationship", () => {
      const result = getRelationshipTypeLabel("other");

      expect(result).toBe("Sonstige");
    });

    it("returns the type itself when unknown", () => {
      const result = getRelationshipTypeLabel("unknown_type");

      expect(result).toBe("unknown_type");
    });

    it("returns empty string for empty type", () => {
      const result = getRelationshipTypeLabel("");

      expect(result).toBe("");
    });
  });

  describe("constants", () => {
    describe("RELATIONSHIP_TYPES", () => {
      it("contains all expected relationship types", () => {
        expect(RELATIONSHIP_TYPES).toHaveLength(4);
        expect(RELATIONSHIP_TYPES.map((t) => t.value)).toEqual([
          "parent",
          "guardian",
          "relative",
          "other",
        ]);
      });

      it("has German labels", () => {
        expect(RELATIONSHIP_TYPES.find((t) => t.value === "parent")?.label).toBe(
          "Elternteil"
        );
        expect(RELATIONSHIP_TYPES.find((t) => t.value === "guardian")?.label).toBe(
          "Vormund"
        );
      });
    });

    describe("CONTACT_METHODS", () => {
      it("contains all expected contact methods", () => {
        expect(CONTACT_METHODS).toHaveLength(3);
        expect(CONTACT_METHODS.map((m) => m.value)).toEqual([
          "email",
          "phone",
          "mobile",
        ]);
      });

      it("has German labels", () => {
        expect(CONTACT_METHODS.find((m) => m.value === "email")?.label).toBe(
          "E-Mail"
        );
        expect(CONTACT_METHODS.find((m) => m.value === "phone")?.label).toBe(
          "Telefon"
        );
        expect(CONTACT_METHODS.find((m) => m.value === "mobile")?.label).toBe(
          "Mobiltelefon"
        );
      });
    });

    describe("LANGUAGE_PREFERENCES", () => {
      it("contains all expected language preferences", () => {
        expect(LANGUAGE_PREFERENCES).toHaveLength(5);
        expect(LANGUAGE_PREFERENCES.map((l) => l.value)).toEqual([
          "de",
          "en",
          "tr",
          "ar",
          "other",
        ]);
      });

      it("has correct labels for each language", () => {
        expect(LANGUAGE_PREFERENCES.find((l) => l.value === "de")?.label).toBe(
          "Deutsch"
        );
        expect(LANGUAGE_PREFERENCES.find((l) => l.value === "en")?.label).toBe(
          "English"
        );
        expect(LANGUAGE_PREFERENCES.find((l) => l.value === "tr")?.label).toBe(
          "Türkçe"
        );
        expect(LANGUAGE_PREFERENCES.find((l) => l.value === "ar")?.label).toBe(
          "العربية"
        );
      });
    });
  });
});
