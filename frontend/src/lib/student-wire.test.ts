import { describe, it, expect } from "vitest";
import {
  mapStudentDetailResponse,
  mapStudentListRow,
  mapStudentResponse,
  prepareStudentForBackend,
  type BackendSlimStudent,
  type BackendStudent,
  type BackendStudentDetail,
} from "~/lib/student-wire";
import { buildBackendStudent } from "~/test/fixtures/students";

const sampleBackendStudent = buildBackendStudent({
  id: 1,
  person_id: 100,
  first_name: "Max",
  last_name: "Mustermann",
  tag_id: "RFID-12345",
  school_class: "3a",
  current_location: "Anwesend - Raum 101",
  location_since: "2024-01-15T10:00:00Z",
  bus: true,
  bus_days: { mon: true, tue: true, wed: true, thu: true, fri: true },
  sick: false,
  address_street: "Musterstraße 12",
  address_city: "Köln",
  address_postal_code: "50667",
  group_id: 5,
  group_name: "Klasse 3a",
  extra_info: "Allergies: None",
  birthday: "2015-06-15",
  health_info: "No health concerns",
  pickup_status: "parent",
});

function detail(
  overrides: Partial<BackendStudentDetail> = {},
): BackendStudentDetail {
  return {
    ...sampleBackendStudent,
    has_full_access: true,
    has_write_access: true,
    has_absence_write_access: true,
    has_sick_excused_write_access: true,
    attendance_log_enabled: false,
    feedback_enabled: false,
    privacy_consent: { accepted: false, data_retention_days: 30 },
    ...overrides,
  };
}

it("does not expose retired student contact fields", () => {
  const student = mapStudentResponse(buildBackendStudent());
  for (const key of [
    "name_lg",
    "contact_lg",
    "guardian_email",
    "guardian_phone",
  ]) {
    expect(student).not.toHaveProperty(key);
  }
});

describe("mapStudentListRow", () => {
  it("keeps the selected day's departure modes of the slim projection", () => {
    const row: BackendSlimStudent = {
      id: 7,
      first_name: "Mila",
      last_name: "Meyer",
      school_class: "2a",
      current_location: "Zuhause",
      sick: false,
      excused: false,
      class_trip: false,
      day_planning_reason: "unplanned_attendance",
      departure_modes: ["bus", "pickup"],
      departure_label: "Taxi mit Begleitperson",
      has_full_access: true,
    };

    const student = mapStudentListRow(row, "slim");

    expect(student.departure_modes).toEqual(["bus", "pickup"]);
    expect(student.departure_label).toBe("Taxi mit Begleitperson");
    expect(student.day_planning_reason).toBe("unplanned_attendance");
    // The slim row must not grow the omitted full-detail maps.
    expect(student).not.toHaveProperty("bus_days");
  });

  it("keeps only the name of the names projection (#4996)", () => {
    expect(
      mapStudentListRow(
        { id: 9, first_name: "Ida", last_name: "Neu" },
        "names",
      ),
    ).toEqual({
      id: "9",
      name: "Ida Neu",
      first_name: "Ida",
      second_name: "Neu",
      school_class: "",
      current_location: "",
    });
  });
});

describe("mapStudentResponse", () => {
  it("preserves the current consent states for the staff detail", () => {
    const consents = [
      {
        key: "photo" as const,
        state: "withdrawn" as const,
        changed_at: "2026-08-31T15:00:00Z",
      },
    ];

    const result = mapStudentResponse({ ...sampleBackendStudent, consents });

    expect(result.consents).toEqual(consents);
  });

  it("maps backend student to frontend structure", () => {
    const result = mapStudentResponse(sampleBackendStudent);

    expect(result.id).toBe("1");
    expect(result.name).toBe("Max Mustermann");
    expect(result.first_name).toBe("Max");
    expect(result.second_name).toBe("Mustermann");
    expect(result.school_class).toBe("3a");
    expect(result.studentId).toBe("RFID-12345");
    expect(result.group_name).toBe("Klasse 3a");
    expect(result.group_id).toBe("5");
    expect(result.bus).toBe(true);
    expect(result.sick).toBe(false);
    expect(result.address_street).toBe("Musterstraße 12");
    expect(result.address_city).toBe("Köln");
    expect(result.address_postal_code).toBe("50667");
    expect(result.extra_info).toBe("Allergies: None");
    expect(result.birthday).toBe("2015-06-15");
  });

  it("builds the name from whichever part is present", () => {
    expect(
      mapStudentResponse({ ...sampleBackendStudent, first_name: "" }).name,
    ).toBe("Mustermann");
    expect(
      mapStudentResponse({ ...sampleBackendStudent, last_name: "" }).name,
    ).toBe("Max");
  });

  it("normalizes legacy and missing location values", () => {
    expect(
      mapStudentResponse({
        ...sampleBackendStudent,
        current_location: "Abwesend",
      }).current_location,
    ).toBe("Zuhause");
    expect(
      mapStudentResponse({ ...sampleBackendStudent, current_location: null })
        .current_location,
    ).toBe("Unbekannt");
  });

  it("converts undefined group_id to undefined", () => {
    const result = mapStudentResponse({
      ...sampleBackendStudent,
      group_id: undefined,
    });

    expect(result.group_id).toBeUndefined();
  });

  it("derives bus from bus_days and defaults sick to false", () => {
    const student: BackendStudent = {
      ...sampleBackendStudent,
      bus: true,
      bus_days: undefined,
      sick: undefined,
    };

    const result = mapStudentResponse(student);

    // bus_days is the single source of truth (#1582).
    expect(result.bus).toBe(false);
    expect(result.sick).toBe(false);
  });

  it("folds a plan that predates departure_days from the legacy maps (#1610)", () => {
    const result = mapStudentResponse({
      ...sampleBackendStudent,
      bus_days: { mon: true, tue: true },
      pickup_days: { tue: true },
      departure_days: undefined,
      allowed_departure_modes: undefined,
    });

    expect(result.departure_days).toEqual({ mon: "bus", tue: "pickup" });
    expect(result.allowed_departure_modes).toEqual({
      mon: ["bus"],
      tue: ["bus", "pickup"],
    });
  });

  it("threads the actual times independently and never synthesizes them", () => {
    expect(
      mapStudentResponse({
        ...sampleBackendStudent,
        actual_arrival_time: "08:05",
      }),
    ).toMatchObject({ actual_arrival_time: "08:05" });
    const untouched = mapStudentResponse(sampleBackendStudent);
    expect(untouched.actual_arrival_time).toBeUndefined();
    expect(untouched.actual_pickup_time).toBeUndefined();
  });
});

describe("mapStudentDetailResponse", () => {
  it("preserves a sick/excused denial independently of other write rights", () => {
    const result = mapStudentDetailResponse(
      detail({ has_sick_excused_write_access: false }),
    );
    expect(result.has_absence_write_access).toBe(true);
    expect(result.has_sick_excused_write_access).toBe(false);
  });

  it("carries access, feature flags, supervisors and the consent pair", () => {
    const result = mapStudentDetailResponse(
      detail({
        has_write_access: false,
        attendance_log_enabled: true,
        feedback_enabled: true,
        group_supervisors: [
          {
            id: 1,
            first_name: "John",
            last_name: "Teacher",
            role: "teacher",
          },
        ],
        privacy_consent: { accepted: true, data_retention_days: 14 },
      }),
    );

    expect(result).toMatchObject({
      name: "Max Mustermann",
      has_full_access: true,
      has_write_access: false,
      attendance_log_enabled: true,
      feedback_enabled: true,
      privacy_consent_accepted: true,
      data_retention_days: 14,
    });
    expect(result.group_supervisors?.[0]?.first_name).toBe("John");
  });

  it("answers an empty supervisor list when the backend sends none", () => {
    expect(mapStudentDetailResponse(detail()).group_supervisors).toEqual([]);
  });
});

describe("prepareStudentForBackend", () => {
  it("converts frontend student to backend format", () => {
    const result = prepareStudentForBackend({
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "3a",
      bus: true,
      bus_days: { mon: true, fri: true },
      group_id: "5",
      address_street: "Musterstraße 12",
      birthday: "2015-06-15",
    });

    expect(result).toMatchObject({
      first_name: "Max",
      last_name: "Mustermann",
      school_class: "3a",
      bus_days: { mon: true, fri: true },
      group_id: 5,
      address_street: "Musterstraße 12",
      birthday: "2015-06-15",
    });
    // bus_days is the single source of truth (#1582).
    expect(result.bus).toBeUndefined();
  });

  // The submitted list REPLACES the stored one, so the backend needs to know
  // WHICH stored list it is replacing (#1694).
  it("sends the companion fingerprint with a submitted list", () => {
    const result = prepareStudentForBackend({
      companions: [{ companion_student_id: "7", weekdays: ["mon"] }],
      companions_fingerprint: "7:mon",
    });

    expect(result.companions).toEqual([
      { companion_student_id: "7", weekdays: ["mon"] },
    ]);
    expect(result.companions_fingerprint).toBe("7:mon");
  });

  // A write without a list removes links too: the departure plan that rides
  // along on every save trims every link the plan no longer allows.
  it("sends the companion fingerprint when no list travels", () => {
    const result = prepareStudentForBackend({
      first_name: "Max",
      companions_fingerprint: "7:mon",
    });

    expect(result.companions).toBeUndefined();
    expect(result.companions_fingerprint).toBe("7:mon");
  });

  it("drops an empty or blank birthday", () => {
    expect(prepareStudentForBackend({ birthday: "" }).birthday).toBeUndefined();
    expect(
      prepareStudentForBackend({ birthday: "   " }).birthday,
    ).toBeUndefined();
  });

  it("leaves unsent plan maps out so partial updates keep the stored plan", () => {
    const result = prepareStudentForBackend({ sick: true });

    expect(result.bus_days).toBeUndefined();
    expect(result.pickup_days).toBeUndefined();
    expect(result.departure_days).toBeUndefined();
    expect(result.allowed_departure_modes).toBeUndefined();
  });

  it("normalizes pickup_days when present", () => {
    const result = prepareStudentForBackend({
      pickup_days: { mon: true, tue: false, wed: true },
    });

    expect(result.pickup_days).toEqual({ mon: true, wed: true });
  });
});
