import { describe, it, expect, vi } from "vitest";
import {
  validateStudentFields,
  parseGuardianContact,
  buildBackendStudentRequest,
  handlePrivacyConsentCreation,
  buildStudentResponse,
  handleStudentCreationError,
} from "./student-request-helpers";
import type { Student } from "./student-helpers";
import { LOCATION_STATUSES } from "./location-helper";

describe("validateStudentFields", () => {
  it("validates required fields successfully", () => {
    const body = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "1a",
    };

    const result = validateStudentFields(body);

    expect(result).toEqual({
      firstName: "Max",
      lastName: "Mustermann",
      schoolClass: "1a",
      guardianName: undefined,
      guardianContact: undefined,
    });
  });

  it("trims whitespace from fields", () => {
    const body = {
      first_name: "  Max  ",
      second_name: "  Mustermann  ",
      school_class: "  1a  ",
      name_lg: "  Jane Doe  ",
      contact_lg: "  jane@example.com  ",
    };

    const result = validateStudentFields(body);

    expect(result).toEqual({
      firstName: "Max",
      lastName: "Mustermann",
      schoolClass: "1a",
      guardianName: "Jane Doe",
      guardianContact: "jane@example.com",
    });
  });

  it("throws error when first_name is missing", () => {
    const body = {
      second_name: "Mustermann",
      school_class: "1a",
    };

    expect(() => validateStudentFields(body)).toThrow("First name is required");
  });

  it("throws error when first_name is empty", () => {
    const body = {
      first_name: "   ",
      second_name: "Mustermann",
      school_class: "1a",
    };

    expect(() => validateStudentFields(body)).toThrow("First name is required");
  });

  it("throws error when last_name is missing", () => {
    const body = {
      first_name: "Max",
      school_class: "1a",
    };

    expect(() => validateStudentFields(body)).toThrow("Last name is required");
  });

  it("throws error when last_name is empty", () => {
    const body = {
      first_name: "Max",
      second_name: "   ",
      school_class: "1a",
    };

    expect(() => validateStudentFields(body)).toThrow("Last name is required");
  });

  it("throws error when school_class is missing", () => {
    const body = {
      first_name: "Max",
      second_name: "Mustermann",
    };

    expect(() => validateStudentFields(body)).toThrow(
      "School class is required",
    );
  });

  it("throws error when school_class is empty", () => {
    const body = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "   ",
    };

    expect(() => validateStudentFields(body)).toThrow(
      "School class is required",
    );
  });

  it("includes optional guardian fields", () => {
    const body = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "1a",
      name_lg: "Jane Doe",
      contact_lg: "jane@example.com",
    };

    const result = validateStudentFields(body);

    expect(result.guardianName).toBe("Jane Doe");
    expect(result.guardianContact).toBe("jane@example.com");
  });
});

describe("parseGuardianContact", () => {
  it("prefers explicit guardian email", () => {
    const result = parseGuardianContact(
      "explicit@example.com",
      undefined,
      undefined,
    );

    expect(result).toEqual({
      email: "explicit@example.com",
      phone: undefined,
    });
  });

  it("prefers explicit guardian phone", () => {
    const result = parseGuardianContact(undefined, "123-456-7890", undefined);

    expect(result).toEqual({
      email: undefined,
      phone: "123-456-7890",
    });
  });

  it("uses both explicit email and phone", () => {
    const result = parseGuardianContact(
      "email@example.com",
      "123-456-7890",
      undefined,
    );

    expect(result).toEqual({
      email: "email@example.com",
      phone: "123-456-7890",
    });
  });

  it("parses contactLg as email when it contains @", () => {
    const result = parseGuardianContact(
      undefined,
      undefined,
      "contact@example.com",
    );

    expect(result).toEqual({
      email: "contact@example.com",
      phone: undefined,
    });
  });

  it("parses contactLg as phone when it does not contain @", () => {
    const result = parseGuardianContact(undefined, undefined, "555-1234");

    expect(result).toEqual({
      email: undefined,
      phone: "555-1234",
    });
  });

  it("ignores contactLg when explicit email is provided", () => {
    const result = parseGuardianContact(
      "explicit@example.com",
      undefined,
      "contact@example.com",
    );

    expect(result).toEqual({
      email: "explicit@example.com",
      phone: undefined,
    });
  });

  it("ignores contactLg when explicit phone is provided", () => {
    const result = parseGuardianContact(undefined, "123-456-7890", "555-1234");

    expect(result).toEqual({
      email: undefined,
      phone: "123-456-7890",
    });
  });

  it("returns undefined for both when no contact info provided", () => {
    const result = parseGuardianContact(undefined, undefined, undefined);

    expect(result).toEqual({
      email: undefined,
      phone: undefined,
    });
  });
});

describe("buildBackendStudentRequest", () => {
  it("builds basic request with required fields", () => {
    const validated = {
      firstName: "Max",
      lastName: "Mustermann",
      schoolClass: "1a",
    };
    const body = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "1a",
    };
    const guardianContact = { email: undefined, phone: undefined };

    const result = buildBackendStudentRequest(validated, body, guardianContact);

    expect(result).toMatchObject({
      first_name: "Max",
      last_name: "Mustermann",
      school_class: "1a",
      current_location: LOCATION_STATUSES.UNKNOWN,
      notes: undefined,
    });
  });

  it("includes tag_id when provided", () => {
    const validated = {
      firstName: "Max",
      lastName: "Mustermann",
      schoolClass: "1a",
    };
    const body = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "1a",
      tag_id: "TAG123",
    };
    const guardianContact = { email: undefined, phone: undefined };

    const result = buildBackendStudentRequest(validated, body, guardianContact);

    expect(result.tag_id).toBe("TAG123");
  });

  it("includes group_id when provided", () => {
    const validated = {
      firstName: "Max",
      lastName: "Mustermann",
      schoolClass: "1a",
    };
    const body = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "1a",
      group_id: "5",
    };
    const guardianContact = { email: undefined, phone: undefined };

    const result = buildBackendStudentRequest(validated, body, guardianContact);

    expect(result.group_id).toBe(5);
  });

  it("includes all optional fields when provided", () => {
    const validated = {
      firstName: "Max",
      lastName: "Mustermann",
      schoolClass: "1a",
      guardianName: "Jane Doe",
      guardianContact: "jane@example.com",
    };
    const body = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "1a",
      bus: true,
      extra_info: "Special needs",
      birthday: "2015-06-15",
      health_info: "Allergies",
      supervisor_notes: "Extra care",
      pickup_status: "Authorized",
    };
    const guardianContact = {
      email: "guardian@example.com",
      phone: "123-456-7890",
    };

    const result = buildBackendStudentRequest(validated, body, guardianContact);

    expect(result).toMatchObject({
      first_name: "Max",
      last_name: "Mustermann",
      school_class: "1a",
      bus: true,
      extra_info: "Special needs",
      birthday: "2015-06-15",
      health_info: "Allergies",
      supervisor_notes: "Extra care",
      pickup_status: "Authorized",
      guardian_name: "Jane Doe",
      guardian_contact: "jane@example.com",
      guardian_email: "guardian@example.com",
      guardian_phone: "123-456-7890",
    });
  });

  it("uses current_location from body if provided", () => {
    const validated = {
      firstName: "Max",
      lastName: "Mustermann",
      schoolClass: "1a",
    };
    const body = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "1a",
      current_location: "Anwesend",
    };
    const guardianContact = { email: undefined, phone: undefined };

    const result = buildBackendStudentRequest(validated, body, guardianContact);

    expect(result.current_location).toBe("Anwesend");
  });

  it("defaults to UNKNOWN location when not provided", () => {
    const validated = {
      firstName: "Max",
      lastName: "Mustermann",
      schoolClass: "1a",
    };
    const body = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "1a",
    };
    const guardianContact = { email: undefined, phone: undefined };

    const result = buildBackendStudentRequest(validated, body, guardianContact);

    expect(result.current_location).toBe(LOCATION_STATUSES.UNKNOWN);
  });

  it("prefers guardianContact email over body guardian_email", () => {
    const validated = {
      firstName: "Max",
      lastName: "Mustermann",
      schoolClass: "1a",
    };
    const body = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "1a",
      guardian_email: "old@example.com",
    };
    const guardianContact = {
      email: "new@example.com",
      phone: undefined,
    };

    const result = buildBackendStudentRequest(validated, body, guardianContact);

    expect(result.guardian_email).toBe("new@example.com");
  });

  it("uses body guardian_email when guardianContact email is undefined", () => {
    const validated = {
      firstName: "Max",
      lastName: "Mustermann",
      schoolClass: "1a",
    };
    const body = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "1a",
      guardian_email: "body@example.com",
    };
    const guardianContact = {
      email: undefined,
      phone: undefined,
    };

    const result = buildBackendStudentRequest(validated, body, guardianContact);

    expect(result.guardian_email).toBe("body@example.com");
  });
});

describe("handlePrivacyConsentCreation", () => {
  it("does nothing when shouldCreate returns false", async () => {
    const apiPut = vi.fn();
    const shouldCreate = vi.fn().mockReturnValue(false);
    const updateConsent = vi.fn();

    await handlePrivacyConsentCreation(
      1,
      true,
      30,
      apiPut,
      "token",
      shouldCreate,
      updateConsent,
    );

    expect(shouldCreate).toHaveBeenCalledWith(true, 30);
    expect(updateConsent).not.toHaveBeenCalled();
  });

  it("creates consent when shouldCreate returns true", async () => {
    const apiPut = vi.fn();
    const shouldCreate = vi.fn().mockReturnValue(true);
    const updateConsent = vi.fn().mockResolvedValue(undefined);

    await handlePrivacyConsentCreation(
      1,
      true,
      30,
      apiPut,
      "token",
      shouldCreate,
      updateConsent,
    );

    expect(shouldCreate).toHaveBeenCalledWith(true, 30);
    expect(updateConsent).toHaveBeenCalledWith(
      1,
      apiPut,
      "token",
      true,
      30,
      "POST Student",
    );
  });

  it("logs error when consent creation fails", async () => {
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);
    const consoleWarnSpy = vi
      .spyOn(console, "warn")
      .mockImplementation(() => undefined);

    const apiPut = vi.fn();
    const shouldCreate = vi.fn().mockReturnValue(true);
    const updateConsent = vi
      .fn()
      .mockRejectedValue(new Error("Consent creation failed"));

    await handlePrivacyConsentCreation(
      1,
      false,
      15,
      apiPut,
      "token",
      shouldCreate,
      updateConsent,
    );

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "[POST Student] Error creating privacy consent for student 1:",
      expect.any(Error),
    );
    expect(consoleWarnSpy).toHaveBeenCalledWith(
      "[POST Student] Student created but privacy consent failed. Admin can update later.",
    );

    consoleErrorSpy.mockRestore();
    consoleWarnSpy.mockRestore();
  });

  it("does not throw when consent creation fails", async () => {
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);
    const consoleWarnSpy = vi
      .spyOn(console, "warn")
      .mockImplementation(() => undefined);

    const apiPut = vi.fn();
    const shouldCreate = vi.fn().mockReturnValue(true);
    const updateConsent = vi.fn().mockRejectedValue(new Error("Network error"));

    await expect(
      handlePrivacyConsentCreation(
        1,
        true,
        30,
        apiPut,
        "token",
        shouldCreate,
        updateConsent,
      ),
    ).resolves.not.toThrow();

    consoleErrorSpy.mockRestore();
    consoleWarnSpy.mockRestore();
  });
});

describe("buildStudentResponse", () => {
  it("fetches and includes consent data when studentId is provided", async () => {
    const mappedStudent = {
      id: "1",
      first_name: "Max",
      second_name: "Mustermann",
    } as Student;
    const apiGet = vi.fn();
    const fetchConsent = vi.fn().mockResolvedValue({
      privacy_consent_accepted: true,
      data_retention_days: 30,
    });

    const result = await buildStudentResponse(
      mappedStudent,
      1,
      apiGet,
      "token",
      fetchConsent,
    );

    expect(fetchConsent).toHaveBeenCalledWith("1", apiGet, "token");
    expect(result).toEqual({
      ...mappedStudent,
      privacy_consent_accepted: true,
      data_retention_days: 30,
    });
  });

  it("returns default consent values when studentId is null", async () => {
    const mappedStudent = {
      id: "0",
      first_name: "Max",
      second_name: "Mustermann",
    } as Student;
    const apiGet = vi.fn();
    const fetchConsent = vi.fn();

    const result = await buildStudentResponse(
      mappedStudent,
      null,
      apiGet,
      "token",
      fetchConsent,
    );

    expect(fetchConsent).not.toHaveBeenCalled();
    expect(result).toEqual({
      ...mappedStudent,
      privacy_consent_accepted: false,
      data_retention_days: 30,
    });
  });

  it("returns default consent values when studentId is undefined", async () => {
    const mappedStudent = {
      id: "0",
      first_name: "Max",
      second_name: "Mustermann",
    } as Student;
    const apiGet = vi.fn();
    const fetchConsent = vi.fn();

    const result = await buildStudentResponse(
      mappedStudent,
      undefined,
      apiGet,
      "token",
      fetchConsent,
    );

    expect(fetchConsent).not.toHaveBeenCalled();
    expect(result).toEqual({
      ...mappedStudent,
      privacy_consent_accepted: false,
      data_retention_days: 30,
    });
  });

  it("returns default consent values when studentId is 0", async () => {
    const mappedStudent = {
      id: "0",
      first_name: "Max",
      second_name: "Mustermann",
    } as Student;
    const apiGet = vi.fn();
    const fetchConsent = vi.fn();

    const result = await buildStudentResponse(
      mappedStudent,
      0,
      apiGet,
      "token",
      fetchConsent,
    );

    expect(fetchConsent).not.toHaveBeenCalled();
    expect(result).toEqual({
      ...mappedStudent,
      privacy_consent_accepted: false,
      data_retention_days: 30,
    });
  });
});

describe("handleStudentCreationError", () => {
  it("throws permission error for 403 status", () => {
    const error = new Error("API error: 403 Forbidden");
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);

    expect(() => handleStudentCreationError(error)).toThrow(
      "Permission denied: You need the 'users:create' permission to create students.",
    );

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "Permission denied when creating student:",
      error,
    );

    consoleErrorSpy.mockRestore();
  });

  it("throws specific error for missing first name validation", () => {
    const error = new Error("400: first name is required");
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);

    expect(() => handleStudentCreationError(error)).toThrow(
      "First name is required",
    );

    consoleErrorSpy.mockRestore();
  });

  it("throws specific error for missing school class validation", () => {
    const error = new Error(
      "400 Bad Request: school class is required for this student",
    );
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);

    expect(() => handleStudentCreationError(error)).toThrow(
      "School class is required",
    );

    consoleErrorSpy.mockRestore();
  });

  it("throws specific error for missing guardian name validation", () => {
    const error = new Error("400: guardian name is required");
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);

    expect(() => handleStudentCreationError(error)).toThrow(
      "Guardian name is required",
    );

    consoleErrorSpy.mockRestore();
  });

  it("throws specific error for missing guardian contact validation", () => {
    const error = new Error("400 - guardian contact is required");
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);

    expect(() => handleStudentCreationError(error)).toThrow(
      "Guardian contact is required",
    );

    consoleErrorSpy.mockRestore();
  });

  it("re-throws non-validation errors as-is", () => {
    const error = new Error("Network timeout");

    expect(() => handleStudentCreationError(error)).toThrow("Network timeout");
  });

  it("re-throws non-Error objects as-is", () => {
    const error = "String error";

    expect(() => handleStudentCreationError(error)).toThrow("String error");
  });

  it("re-throws 400 errors without specific validation messages", () => {
    const error = new Error("400 Bad Request: unknown validation error");
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);

    expect(() => handleStudentCreationError(error)).toThrow(
      "400 Bad Request: unknown validation error",
    );

    consoleErrorSpy.mockRestore();
  });
});
