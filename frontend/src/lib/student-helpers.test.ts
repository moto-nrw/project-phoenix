import { describe, it, expect } from "vitest";
import type {
  BackendStudent,
  BackendStudentDetail,
  BackendPrivacyConsent,
  UpdateStudentRequest,
} from "./student-helpers";
import {
  SCHOOL_YEAR_FILTER_OPTIONS,
  mapStudentResponse,
  mapStudentsResponse,
  mapSingleStudentResponse,
  mapStudentDetailResponse,
  prepareStudentForBackend,
  mapPrivacyConsentResponse,
  mapUpdateRequestToBackend,
  formatStudentName,
  formatStudentStatus,
  extractGuardianContact,
  getStatusColor,
} from "./student-helpers";

// Sample backend student for testing
const sampleBackendStudent: BackendStudent = {
  id: 1,
  person_id: 100,
  first_name: "Max",
  last_name: "Mustermann",
  tag_id: "RFID-12345",
  school_class: "3a",
  current_location: "Anwesend - Raum 101",
  location_since: "2024-01-15T10:00:00Z",
  bus: true,
  sick: false,
  guardian_name: "Hans Mustermann",
  guardian_contact: "+49 123 456789",
  guardian_email: "hans@example.com",
  guardian_phone: "+49 987 654321",
  group_id: 5,
  group_name: "Klasse 3a",
  extra_info: "Allergies: None",
  birthday: "2015-06-15",
  health_info: "No health concerns",
  supervisor_notes: "Good student",
  pickup_status: "parent",
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-15T12:00:00Z",
};

describe("SCHOOL_YEAR_FILTER_OPTIONS", () => {
  it("contains 5 options including 'all'", () => {
    expect(SCHOOL_YEAR_FILTER_OPTIONS).toHaveLength(5);
  });

  it("has 'all' as first option", () => {
    expect(SCHOOL_YEAR_FILTER_OPTIONS[0]).toEqual({
      value: "all",
      label: "Alle",
    });
  });

  it("contains grade options 1-4", () => {
    const values = SCHOOL_YEAR_FILTER_OPTIONS.map((opt) => opt.value);
    expect(values).toContain("1");
    expect(values).toContain("2");
    expect(values).toContain("3");
    expect(values).toContain("4");
  });
});

describe("mapStudentResponse", () => {
  it("maps backend student to frontend structure", () => {
    const result = mapStudentResponse(sampleBackendStudent);

    expect(result.id).toBe("1"); // int64 → string
    expect(result.name).toBe("Max Mustermann");
    expect(result.first_name).toBe("Max");
    expect(result.second_name).toBe("Mustermann"); // last_name → second_name
    expect(result.school_class).toBe("3a");
    expect(result.studentId).toBe("RFID-12345"); // tag_id → studentId
    expect(result.group_name).toBe("Klasse 3a");
    expect(result.group_id).toBe("5"); // int64 → string
    expect(result.bus).toBe(true);
    expect(result.sick).toBe(false);
    expect(result.name_lg).toBe("Hans Mustermann"); // guardian_name → name_lg
    expect(result.contact_lg).toBe("+49 123 456789"); // guardian_contact → contact_lg
    expect(result.guardian_email).toBe("hans@example.com");
    expect(result.guardian_phone).toBe("+49 987 654321");
    expect(result.extra_info).toBe("Allergies: None");
    expect(result.birthday).toBe("2015-06-15");
  });

  it("constructs full name from first and last name", () => {
    const result = mapStudentResponse(sampleBackendStudent);

    expect(result.name).toBe("Max Mustermann");
  });

  it("handles missing first name", () => {
    const student: BackendStudent = {
      ...sampleBackendStudent,
      first_name: "",
      last_name: "Mustermann",
    };

    const result = mapStudentResponse(student);

    expect(result.name).toBe("Mustermann");
  });

  it("handles missing last name", () => {
    const student: BackendStudent = {
      ...sampleBackendStudent,
      first_name: "Max",
      last_name: "",
    };

    const result = mapStudentResponse(student);

    expect(result.name).toBe("Max");
  });

  it("normalizes legacy location values", () => {
    const student: BackendStudent = {
      ...sampleBackendStudent,
      current_location: "Abwesend", // Legacy value
    };

    const result = mapStudentResponse(student);

    // "Abwesend" should be normalized to "Zuhause"
    expect(result.current_location).toBe("Zuhause");
  });

  it("handles null current_location", () => {
    const student: BackendStudent = {
      ...sampleBackendStudent,
      current_location: null,
    };

    const result = mapStudentResponse(student);

    expect(result.current_location).toBe("Unbekannt");
  });

  it("converts undefined group_id to undefined", () => {
    const student: BackendStudent = {
      ...sampleBackendStudent,
      group_id: undefined,
    };

    const result = mapStudentResponse(student);

    expect(result.group_id).toBeUndefined();
  });

  it("defaults bus and sick to false when undefined", () => {
    const student: BackendStudent = {
      ...sampleBackendStudent,
      bus: undefined,
      sick: undefined,
    };

    const result = mapStudentResponse(student);

    expect(result.bus).toBe(false);
    expect(result.sick).toBe(false);
  });

  it("includes scheduled_checkout when present", () => {
    const student: BackendStudent = {
      ...sampleBackendStudent,
      scheduled_checkout: {
        id: 1,
        scheduled_for: "2024-01-15T14:00:00Z",
        reason: "Doctor appointment",
        scheduled_by: "parent",
      },
    };

    const result = mapStudentResponse(student);

    expect(result.scheduled_checkout).toBeDefined();
    expect(result.scheduled_checkout?.reason).toBe("Doctor appointment");
  });
});

describe("mapStudentsResponse", () => {
  it("maps array of backend students", () => {
    const students = [
      sampleBackendStudent,
      { ...sampleBackendStudent, id: 2, first_name: "Anna" },
    ];

    const result = mapStudentsResponse(students);

    expect(result).toHaveLength(2);
    expect(result[0]?.id).toBe("1");
    expect(result[1]?.id).toBe("2");
    expect(result[1]?.first_name).toBe("Anna");
  });

  it("handles empty array", () => {
    const result = mapStudentsResponse([]);

    expect(result).toEqual([]);
  });
});

describe("mapSingleStudentResponse", () => {
  it("extracts and maps student from { data: student } wrapper", () => {
    const response = { data: sampleBackendStudent };

    const result = mapSingleStudentResponse(response);

    expect(result.id).toBe("1");
    expect(result.name).toBe("Max Mustermann");
  });
});

describe("mapStudentDetailResponse", () => {
  it("maps detail response with access control info", () => {
    const detailStudent: BackendStudentDetail = {
      ...sampleBackendStudent,
      has_full_access: true,
      group_supervisors: [
        {
          id: 1,
          first_name: "John",
          last_name: "Teacher",
          email: "john@school.de",
          phone: "+49 111 222333",
          role: "teacher",
        },
      ],
    };

    const result = mapStudentDetailResponse(detailStudent);

    expect(result.has_full_access).toBe(true);
    expect(result.group_supervisors).toHaveLength(1);
    expect(result.group_supervisors?.[0]?.first_name).toBe("John");
  });

  it("handles missing group_supervisors", () => {
    const detailStudent: BackendStudentDetail = {
      ...sampleBackendStudent,
      has_full_access: false,
    };

    const result = mapStudentDetailResponse(detailStudent);

    expect(result.has_full_access).toBe(false);
    expect(result.group_supervisors).toBeUndefined();
  });
});

describe("prepareStudentForBackend", () => {
  it("converts frontend student to backend format", () => {
    const frontendStudent = {
      id: "1",
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "3a",
      current_location: "Anwesend - Raum 101",
      bus: true,
      group_id: "5",
      tag_id: "RFID-12345",
      guardian_email: "test@example.com",
      birthday: "2015-06-15",
    };

    const result = prepareStudentForBackend(frontendStudent);

    expect(result.id).toBe(1); // string → number
    expect(result.first_name).toBe("Max");
    expect(result.last_name).toBe("Mustermann"); // second_name → last_name
    expect(result.school_class).toBe("3a");
    expect(result.bus).toBe(true);
    expect(result.group_id).toBe(5); // string → number
    expect(result.tag_id).toBe("RFID-12345");
    expect(result.guardian_email).toBe("test@example.com");
    expect(result.birthday).toBe("2015-06-15");
  });

  it("converts empty birthday string to undefined", () => {
    const frontendStudent = {
      id: "1",
      birthday: "",
    };

    const result = prepareStudentForBackend(frontendStudent);

    expect(result.birthday).toBeUndefined();
  });

  it("converts whitespace-only birthday to undefined", () => {
    const frontendStudent = {
      id: "1",
      birthday: "   ",
    };

    const result = prepareStudentForBackend(frontendStudent);

    expect(result.birthday).toBeUndefined();
  });

  it("defaults bus to false when undefined", () => {
    const result = prepareStudentForBackend({ id: "1" });

    expect(result.bus).toBe(false);
  });

  it("handles missing id for creation", () => {
    const result = prepareStudentForBackend({
      first_name: "New",
      second_name: "Student",
    });

    expect(result.id).toBeUndefined();
  });
});

describe("mapPrivacyConsentResponse", () => {
  it("maps backend privacy consent to frontend structure", () => {
    const backendConsent: BackendPrivacyConsent = {
      id: 1,
      student_id: 5,
      policy_version: "1.0",
      accepted: true,
      accepted_at: "2024-01-15T10:00:00Z",
      expires_at: "2025-01-15T10:00:00Z",
      duration_days: 365,
      renewal_required: false,
      data_retention_days: 30,
      details: { consent_type: "full" },
      created_at: "2024-01-15T10:00:00Z",
      updated_at: "2024-01-15T12:00:00Z",
    };

    const result = mapPrivacyConsentResponse(backendConsent);

    expect(result.id).toBe("1");
    expect(result.studentId).toBe("5");
    expect(result.policyVersion).toBe("1.0");
    expect(result.accepted).toBe(true);
    expect(result.acceptedAt).toBeInstanceOf(Date);
    expect(result.expiresAt).toBeInstanceOf(Date);
    expect(result.durationDays).toBe(365);
    expect(result.renewalRequired).toBe(false);
    expect(result.dataRetentionDays).toBe(30);
    expect(result.details).toEqual({ consent_type: "full" });
    expect(result.createdAt).toBeInstanceOf(Date);
    expect(result.updatedAt).toBeInstanceOf(Date);
  });

  it("handles missing optional date fields", () => {
    const backendConsent: BackendPrivacyConsent = {
      id: 1,
      student_id: 5,
      policy_version: "1.0",
      accepted: false,
      renewal_required: true,
      data_retention_days: 30,
      created_at: "2024-01-15T10:00:00Z",
      updated_at: "2024-01-15T12:00:00Z",
    };

    const result = mapPrivacyConsentResponse(backendConsent);

    expect(result.acceptedAt).toBeUndefined();
    expect(result.expiresAt).toBeUndefined();
    expect(result.durationDays).toBeUndefined();
  });
});

describe("mapUpdateRequestToBackend", () => {
  it("maps all fields from frontend update request to backend format", () => {
    const request: UpdateStudentRequest = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "3a",
      group_id: "5",
      name_lg: "Hans Mustermann",
      contact_lg: "+49 123 456789",
      tag_id: "RFID-12345",
      guardian_email: "hans@example.com",
      guardian_phone: "+49 987 654321",
      extra_info: "Notes",
      birthday: "2015-06-15",
      health_info: "None",
      supervisor_notes: "Good",
      pickup_status: "parent",
      bus: true,
      sick: false,
    };

    const result = mapUpdateRequestToBackend(request);

    expect(result.first_name).toBe("Max");
    expect(result.last_name).toBe("Mustermann"); // second_name → last_name
    expect(result.school_class).toBe("3a");
    expect(result.group_id).toBe(5); // string → number
    expect(result.guardian_name).toBe("Hans Mustermann"); // name_lg → guardian_name
    expect(result.guardian_contact).toBe("+49 123 456789"); // contact_lg → guardian_contact
    expect(result.tag_id).toBe("RFID-12345");
    expect(result.guardian_email).toBe("hans@example.com");
    expect(result.guardian_phone).toBe("+49 987 654321");
    expect(result.extra_info).toBe("Notes");
    expect(result.birthday).toBe("2015-06-15");
    expect(result.health_info).toBe("None");
    expect(result.supervisor_notes).toBe("Good");
    expect(result.pickup_status).toBe("parent");
    expect(result.bus).toBe(true);
    expect(result.sick).toBe(false);
  });

  it("omits undefined fields", () => {
    const request: UpdateStudentRequest = {
      first_name: "Max",
    };

    const result = mapUpdateRequestToBackend(request);

    expect(result.first_name).toBe("Max");
    expect(result.last_name).toBeUndefined();
    expect(result.group_id).toBeUndefined();
  });

  it("handles group_id string to number conversion", () => {
    const request: UpdateStudentRequest = {
      group_id: "123",
    };

    const result = mapUpdateRequestToBackend(request);

    expect(result.group_id).toBe(123);
    expect(typeof result.group_id).toBe("number");
  });
});

describe("formatStudentName", () => {
  it("returns name property when available", () => {
    const student = {
      id: "1",
      name: "Max Mustermann",
      school_class: "3a",
      current_location: "Anwesend",
    };

    const result = formatStudentName(student);

    expect(result).toBe("Max Mustermann");
  });

  it("constructs name from first_name and second_name as fallback", () => {
    const student = {
      id: "1",
      name: "",
      first_name: "Anna",
      second_name: "Schmidt",
      school_class: "3a",
      current_location: "Anwesend",
    };

    const result = formatStudentName(student);

    expect(result).toBe("Anna Schmidt");
  });

  it("returns 'Unnamed Student' when no name info available", () => {
    const student = {
      id: "1",
      name: "",
      school_class: "3a",
      current_location: "Anwesend",
    };

    const result = formatStudentName(student);

    expect(result).toBe("Unnamed Student");
  });

  it("handles only first_name available", () => {
    const student = {
      id: "1",
      name: "",
      first_name: "Anna",
      school_class: "3a",
      current_location: "Anwesend",
    };

    const result = formatStudentName(student);

    expect(result).toBe("Anna");
  });
});

describe("formatStudentStatus", () => {
  it("returns room when location includes room", () => {
    const student = {
      id: "1",
      name: "Test",
      school_class: "3a",
      current_location: "Anwesend - Raum 101",
    };

    const result = formatStudentStatus(student);

    expect(result).toBe("Raum 101");
  });

  it("returns status when no room specified", () => {
    const student = {
      id: "1",
      name: "Test",
      school_class: "3a",
      current_location: "Zuhause",
    };

    const result = formatStudentStatus(student);

    expect(result).toBe("Zuhause");
  });

  it("returns 'Unbekannt' for empty location", () => {
    const student = {
      id: "1",
      name: "Test",
      school_class: "3a",
      current_location: "",
    };

    const result = formatStudentStatus(student);

    expect(result).toBe("Unbekannt");
  });
});

describe("extractGuardianContact", () => {
  it("prioritizes guardian_email when available", () => {
    const result = extractGuardianContact({
      guardian_email: "email@example.com",
      contact_lg: "+49 123 456789",
    });

    expect(result).toBe("email@example.com");
  });

  it("falls back to contact_lg when guardian_email is missing", () => {
    const result = extractGuardianContact({
      contact_lg: "+49 123 456789",
    });

    expect(result).toBe("+49 123 456789");
  });

  it("returns empty string when no contact info available", () => {
    const result = extractGuardianContact({});

    expect(result).toBe("");
  });

  it("returns empty string when both fields are empty strings", () => {
    const result = extractGuardianContact({
      guardian_email: "",
      contact_lg: "",
    });

    // Empty strings are falsy, so returns ""
    expect(result).toBe("");
  });

  it("uses guardian_email even when contact_lg has value", () => {
    const result = extractGuardianContact({
      guardian_email: "test@test.com",
      contact_lg: "phone number",
    });

    expect(result).toBe("test@test.com");
  });
});

describe("getStatusColor", () => {
  it("returns 'green' for present location", () => {
    const student = {
      id: "1",
      name: "Test",
      school_class: "3a",
      current_location: "Anwesend - Raum 101",
    };

    const result = getStatusColor(student);

    expect(result).toBe("green");
  });

  it("returns 'yellow' for schoolyard location", () => {
    const student = {
      id: "1",
      name: "Test",
      school_class: "3a",
      current_location: "Schulhof",
    };

    const result = getStatusColor(student);

    expect(result).toBe("yellow");
  });

  it("returns 'purple' for transit location", () => {
    const student = {
      id: "1",
      name: "Test",
      school_class: "3a",
      current_location: "Unterwegs",
    };

    const result = getStatusColor(student);

    expect(result).toBe("purple");
  });

  it("returns 'red' for home location", () => {
    const student = {
      id: "1",
      name: "Test",
      school_class: "3a",
      current_location: "Zuhause",
    };

    const result = getStatusColor(student);

    expect(result).toBe("red");
  });

  it("returns 'gray' for unknown location", () => {
    const student = {
      id: "1",
      name: "Test",
      school_class: "3a",
      current_location: "Unbekannt",
    };

    const result = getStatusColor(student);

    expect(result).toBe("gray");
  });

  it("returns 'gray' for empty location", () => {
    const student = {
      id: "1",
      name: "Test",
      school_class: "3a",
      current_location: "",
    };

    const result = getStatusColor(student);

    expect(result).toBe("gray");
  });
});
