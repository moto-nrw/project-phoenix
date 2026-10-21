import { describe, it, expect } from "vitest";
import {
  mapGuardianResponse,
  mapGuardianWithRelationshipResponse,
  mapStudentGuardianLinkToBackend,
  mapPhoneNumberResponse,
  mapPhoneNumberCreateToBackend,
  mapPhoneNumberUpdateToBackend,
  getGuardianFullName,
  getRelationshipTypeLabel,
  getLanguageLabel,
  RELATIONSHIP_TYPES,
  CONTACT_METHODS,
  LANGUAGE_PREFERENCES,
  PHONE_TYPE_LABELS,
  type BackendGuardianProfile,
  type BackendGuardianWithRelationship,
  type StudentGuardianLinkRequest,
  type Guardian,
  type BackendPhoneNumber,
  type PhoneNumberCreateRequest,
  type PhoneNumberUpdateRequest,
} from "./guardian-helpers";

describe("guardian-helpers", () => {
  describe("mapGuardianResponse", () => {
    it("maps all fields from backend to frontend format", () => {
      const backendData: BackendGuardianProfile = {
        id: 123,
        first_name: "John",
        last_name: "Doe",
        email: "john.doe@example.com",
        phone_numbers: [
          {
            id: 1,
            phone_number: "030-12345678",
            phone_type: "home",
            is_primary: true,
            priority: 1,
          },
          {
            id: 2,
            phone_number: "0170-12345678",
            phone_type: "mobile",
            is_primary: false,
            priority: 2,
          },
        ],
        address_street: "Hauptstraße 1",
        address_city: "Berlin",
        address_postal_code: "10115",
        preferred_contact_method: "email",
        language_preference: "de",
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
        phoneNumbers: [
          {
            id: "1",
            phoneNumber: "030-12345678",
            phoneType: "home",
            isPrimary: true,
            priority: 1,
            label: undefined,
          },
          {
            id: "2",
            phoneNumber: "0170-12345678",
            phoneType: "mobile",
            isPrimary: false,
            priority: 2,
            label: undefined,
          },
        ],
        addressStreet: "Hauptstraße 1",
        addressCity: "Berlin",
        addressPostalCode: "10115",
        preferredContactMethod: "email",
        languagePreference: "de",
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
        phone_numbers: undefined,
        address_street: undefined,
        address_city: undefined,
        address_postal_code: undefined,
        notes: undefined,
        account_id: undefined,
      };

      const result = mapGuardianResponse(backendData);

      expect(result.email).toBeUndefined();
      expect(result.phoneNumbers).toEqual([]);
      expect(result.addressStreet).toBeUndefined();
      expect(result.addressCity).toBeUndefined();
      expect(result.addressPostalCode).toBeUndefined();
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
        phoneNumbers: [],
        addressStreet: undefined,
        addressCity: undefined,
        addressPostalCode: undefined,
        preferredContactMethod: "email",
        languagePreference: "de",
        notes: undefined,
        hasAccount: true,
        accountId: "200",
        relationshipId: "50",
        relationshipType: "parent",
        guardianRole: "custom",
        isPrimary: true,
        isEmergencyContact: true,
        canPickup: true,
        // Payloads that predate the payment mark (#2608) map to false, so an
        // older backend never reads as "this guardian pays".
        isPayer: false,
        pickupNotes: "Usually picks up at 15:00",
        emergencyPriority: 1,
        accountStatus: "active",
      });
    });

    it("carries the payment mark through", () => {
      const backendData: BackendGuardianWithRelationship = {
        guardian: {
          id: 101,
          first_name: "Payer",
          last_name: "One",
          preferred_contact_method: "email",
          language_preference: "de",
          has_account: false,
        },
        relationship_id: 51,
        relationship_type: "parent",
        is_primary: true,
        is_emergency_contact: false,
        can_pickup: true,
        is_payer: true,
        emergency_priority: 1,
      };

      expect(mapGuardianWithRelationshipResponse(backendData).isPayer).toBe(
        true,
      );
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

  describe("mapStudentGuardianLinkToBackend", () => {
    it("maps link request to backend format", () => {
      const linkRequest: StudentGuardianLinkRequest = {
        guardianProfileId: "123",
        relationshipType: "parent",
        guardianRole: "primary_guardian",
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
        guardian_role: "primary_guardian",
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
        phoneNumbers: [],
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
        phoneNumbers: [],
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
        phoneNumbers: [],
      };

      const result = getGuardianFullName(guardian);

      expect(result).toBe("A B");
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

  describe("getLanguageLabel", () => {
    it("returns label for known language code", () => {
      expect(getLanguageLabel("de")).toBe("Deutsch");
      expect(getLanguageLabel("en")).toBe("English");
      expect(getLanguageLabel("tr")).toBe("Türkisch");
      expect(getLanguageLabel("ar")).toBe("Arabisch");
      expect(getLanguageLabel("ru")).toBe("Russisch");
    });

    it("returns uppercased code for unknown language", () => {
      expect(getLanguageLabel("xx")).toBe("XX");
    });

    it("returns label for 'other'", () => {
      expect(getLanguageLabel("other")).toBe("Sonstige");
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
        expect(
          RELATIONSHIP_TYPES.find((t) => t.value === "parent")?.label,
        ).toBe("Elternteil");
        expect(
          RELATIONSHIP_TYPES.find((t) => t.value === "guardian")?.label,
        ).toBe("Vormund");
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
          "E-Mail",
        );
        expect(CONTACT_METHODS.find((m) => m.value === "phone")?.label).toBe(
          "Telefon",
        );
        expect(CONTACT_METHODS.find((m) => m.value === "mobile")?.label).toBe(
          "Mobiltelefon",
        );
      });
    });

    describe("LANGUAGE_PREFERENCES", () => {
      it("contains all expected language preferences", () => {
        expect(LANGUAGE_PREFERENCES).toHaveLength(10);
        expect(LANGUAGE_PREFERENCES.map((l) => l.value)).toEqual([
          "de",
          "en",
          "tr",
          "ar",
          "ru",
          "pl",
          "uk",
          "fa",
          "ro",
          "other",
        ]);
      });

      it("has correct labels for each language", () => {
        expect(LANGUAGE_PREFERENCES.find((l) => l.value === "de")?.label).toBe(
          "Deutsch",
        );
        expect(LANGUAGE_PREFERENCES.find((l) => l.value === "en")?.label).toBe(
          "English",
        );
        expect(LANGUAGE_PREFERENCES.find((l) => l.value === "tr")?.label).toBe(
          "Türkisch",
        );
        expect(LANGUAGE_PREFERENCES.find((l) => l.value === "ar")?.label).toBe(
          "Arabisch",
        );
        expect(LANGUAGE_PREFERENCES.find((l) => l.value === "ru")?.label).toBe(
          "Russisch",
        );
        expect(LANGUAGE_PREFERENCES.find((l) => l.value === "pl")?.label).toBe(
          "Polnisch",
        );
        expect(LANGUAGE_PREFERENCES.find((l) => l.value === "uk")?.label).toBe(
          "Ukrainisch",
        );
        expect(LANGUAGE_PREFERENCES.find((l) => l.value === "fa")?.label).toBe(
          "Persisch",
        );
        expect(LANGUAGE_PREFERENCES.find((l) => l.value === "ro")?.label).toBe(
          "Rumänisch",
        );
      });
    });

    describe("PHONE_TYPE_LABELS", () => {
      it("contains all expected phone types", () => {
        expect(Object.keys(PHONE_TYPE_LABELS)).toHaveLength(4);
        expect(Object.keys(PHONE_TYPE_LABELS)).toEqual([
          "mobile",
          "home",
          "work",
          "other",
        ]);
      });

      it("has German labels", () => {
        expect(PHONE_TYPE_LABELS.mobile).toBe("Mobil");
        expect(PHONE_TYPE_LABELS.home).toBe("Telefon");
        expect(PHONE_TYPE_LABELS.work).toBe("Dienstlich");
        expect(PHONE_TYPE_LABELS.other).toBe("Sonstige");
      });
    });
  });

  describe("mapPhoneNumberResponse", () => {
    it("maps all fields from backend to frontend format", () => {
      const backendData: BackendPhoneNumber = {
        id: 42,
        phone_number: "+49 30 123456",
        phone_type: "work",
        label: "Büro",
        is_primary: true,
        priority: 1,
      };

      const result = mapPhoneNumberResponse(backendData);

      expect(result).toEqual({
        id: "42",
        phoneNumber: "+49 30 123456",
        phoneType: "work",
        label: "Büro",
        isPrimary: true,
        priority: 1,
      });
    });

    it("converts numeric id to string", () => {
      const backendData: BackendPhoneNumber = {
        id: 999,
        phone_number: "030-123456",
        phone_type: "home",
        is_primary: false,
        priority: 2,
      };

      const result = mapPhoneNumberResponse(backendData);

      expect(result.id).toBe("999");
      expect(typeof result.id).toBe("string");
    });

    it("handles undefined label", () => {
      const backendData: BackendPhoneNumber = {
        id: 1,
        phone_number: "0170-123456",
        phone_type: "mobile",
        is_primary: true,
        priority: 1,
      };

      const result = mapPhoneNumberResponse(backendData);

      expect(result.label).toBeUndefined();
    });
  });

  describe("mapPhoneNumberCreateToBackend", () => {
    it("maps all fields to backend format", () => {
      const request: PhoneNumberCreateRequest = {
        phoneNumber: "+49 170 1234567",
        phoneType: "mobile",
        label: "Privat",
        isPrimary: true,
      };

      const result = mapPhoneNumberCreateToBackend(request);

      expect(result).toEqual({
        phone_number: "+49 170 1234567",
        phone_type: "mobile",
        label: "Privat",
        is_primary: true,
      });
    });

    it("handles minimal required fields", () => {
      const request: PhoneNumberCreateRequest = {
        phoneNumber: "030-123456",
        phoneType: "home",
      };

      const result = mapPhoneNumberCreateToBackend(request);

      expect(result.phone_number).toBe("030-123456");
      expect(result.phone_type).toBe("home");
      expect(result.label).toBeUndefined();
      expect(result.is_primary).toBeUndefined();
    });
  });

  describe("mapPhoneNumberUpdateToBackend", () => {
    it("maps all fields to backend format", () => {
      const request: PhoneNumberUpdateRequest = {
        phoneNumber: "+49 30 9876543",
        phoneType: "work",
        label: "Dienstlich",
      };

      const result = mapPhoneNumberUpdateToBackend(request);

      expect(result).toEqual({
        phone_number: "+49 30 9876543",
        phone_type: "work",
        label: "Dienstlich",
      });
    });

    it("only includes defined fields", () => {
      const request: PhoneNumberUpdateRequest = {
        phoneNumber: "030-999999",
      };

      const result = mapPhoneNumberUpdateToBackend(request);

      expect(result).toEqual({
        phone_number: "030-999999",
      });
      expect(result.phone_type).toBeUndefined();
      expect(result.label).toBeUndefined();
    });

    it("returns empty object when no fields provided", () => {
      const request: PhoneNumberUpdateRequest = {};

      const result = mapPhoneNumberUpdateToBackend(request);

      expect(result).toEqual({});
    });
  });
});
