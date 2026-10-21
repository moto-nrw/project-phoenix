import { describe, it, expect } from "vitest";
import type {
  BackendActivity,
  BackendActivityCategory,
  BackendActivitySchedule,
  BackendStudentEnrollment,
  BackendActivitySupervisor,
  BackendTimeframe,
  Activity,
  ActivitySchedule,
  ActivitySupervisor,
} from "./activity-helpers";
import {
  formatSupervisorList,
  mapActivityResponse,
  mapActivityCategoryResponse,
  mapActivityScheduleResponse,
  mapStudentEnrollmentResponse,
  mapStudentEnrollmentsResponse,
  isActivityCreator,
  mapSupervisorResponse,
  mapTimeframeResponse,
  prepareActivityForBackend,
  prepareActivityScheduleForBackend,
  isSystemActivity,
} from "./activity-helpers";

// Sample data for tests
const sampleBackendSupervisor: BackendActivitySupervisor = {
  id: 1,
  staff_id: 100,
  is_primary: true,
  first_name: "Maria",
  last_name: "Schmidt",
};

const sampleBackendActivity: BackendActivity = {
  id: 1,
  name: "Football Club",
  max_participants: 20,
  is_open: true,
  category_id: 5,
  planned_room_id: 10,
  supervisor_id: 100,
  supervisors: [sampleBackendSupervisor],
  enrollment_count: 15,
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-15T12:00:00Z",
  category: {
    id: 5,
    name: "Sport",
    description: "Sports activities",
    color: "#0000FF",
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
  },
  schedules: [
    {
      id: 1,
      weekday: 1,
      timeframe_id: 2,
      activity_group_id: 1,
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-01T00:00:00Z",
    },
  ],
};

// ===== SUPERVISOR TESTS =====

describe("formatSupervisorList", () => {
  it("formats list of supervisors with full names", () => {
    const supervisors: ActivitySupervisor[] = [
      {
        id: "1",
        staff_id: "100",
        is_primary: true,
        full_name: "Maria Schmidt",
      },
      {
        id: "2",
        staff_id: "101",
        is_primary: false,
        full_name: "Peter Mueller",
      },
    ];

    const result = formatSupervisorList(supervisors);

    expect(result).toBe("Maria Schmidt, Peter Mueller");
  });

  it("returns German message for empty or undefined list", () => {
    expect(formatSupervisorList(undefined)).toBe("Keine Betreuer zugewiesen");
    expect(formatSupervisorList([])).toBe("Keine Betreuer zugewiesen");
  });

  it("constructs name from first/last when full_name is missing", () => {
    const supervisors: ActivitySupervisor[] = [
      {
        id: "1",
        staff_id: "100",
        is_primary: true,
        first_name: "Anna",
        last_name: "Bauer",
      },
    ];

    const result = formatSupervisorList(supervisors);

    expect(result).toBe("Anna Bauer");
  });

  it("falls back to ID when no name info available", () => {
    const supervisors: ActivitySupervisor[] = [
      { id: "999", staff_id: "100", is_primary: true },
    ];

    const result = formatSupervisorList(supervisors);

    expect(result).toBe("Betreuer 999");
  });
});

// ===== ACTIVITY MAPPING TESTS =====

describe("mapActivityResponse", () => {
  it("maps backend activity to frontend structure", () => {
    const result = mapActivityResponse(sampleBackendActivity);

    expect(result.id).toBe("1");
    expect(result.name).toBe("Football Club");
    expect(result.max_participant).toBe(20);
    expect(result.is_open_ags).toBe(true);
    expect(result.ag_category_id).toBe("5");
    expect(result.planned_room_id).toBe("10");
    expect(result.participant_count).toBe(15);
    expect(result.category_name).toBe("Sport");
    expect(result.created_at).toBeInstanceOf(Date);
  });

  it("extracts supervisor info from detailed supervisors array", () => {
    const result = mapActivityResponse(sampleBackendActivity);

    expect(result.supervisor_id).toBe("100"); // primary supervisor's staff_id
    expect(result.supervisor_name).toBe("Maria Schmidt");
    expect(result.supervisors).toHaveLength(1);
  });

  it("falls back to supervisor_id field when no supervisors array", () => {
    const activityWithoutSupervisors: BackendActivity = {
      ...sampleBackendActivity,
      supervisors: undefined,
      supervisor_id: 200,
    };

    const result = mapActivityResponse(activityWithoutSupervisors);

    expect(result.supervisor_id).toBe("200");
    expect(result.supervisors).toBeUndefined();
  });

  it("falls back to supervisor_ids array when no supervisors or supervisor_id", () => {
    const activityWithIds: BackendActivity = {
      ...sampleBackendActivity,
      supervisors: undefined,
      supervisor_id: undefined,
      supervisor_ids: [300, 301],
    };

    const result = mapActivityResponse(activityWithIds);

    expect(result.supervisor_id).toBe("300"); // First in array
  });

  it("maps schedules to times array", () => {
    const result = mapActivityResponse(sampleBackendActivity);

    expect(result.times).toHaveLength(1);
    expect(result.times?.[0]?.weekday).toBe("1");
    expect(result.times?.[0]?.timeframe_id).toBe("2");
  });

  it("defaults participant_count to 0 when undefined", () => {
    const activityWithoutCount: BackendActivity = {
      ...sampleBackendActivity,
      enrollment_count: undefined,
    };

    const result = mapActivityResponse(activityWithoutCount);

    expect(result.participant_count).toBe(0);
  });
});

describe("mapActivityCategoryResponse", () => {
  it("maps backend category to frontend structure", () => {
    const backendCategory: BackendActivityCategory = {
      id: 1,
      name: "Sport",
      description: "Sports activities",
      color: "#0000FF",
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-15T12:00:00Z",
    };

    const result = mapActivityCategoryResponse(backendCategory);

    expect(result.id).toBe("1");
    expect(result.name).toBe("Sport");
    expect(result.description).toBe("Sports activities");
    expect(result.color).toBe("#0000FF");
    expect(result.created_at).toBeInstanceOf(Date);
    expect(result.updated_at).toBeInstanceOf(Date);
    expect(result.shiftTypeId).toBeUndefined();
  });

  it("maps the optional shift_type_id to a string (#1837 follow-up)", () => {
    const result = mapActivityCategoryResponse({
      id: 7,
      name: "Betreuung",
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-15T12:00:00Z",
      shift_type_id: 42,
    });

    expect(result.shiftTypeId).toBe("42");
  });
});

describe("mapActivityScheduleResponse", () => {
  it("maps backend schedule to frontend structure", () => {
    const backendSchedule: BackendActivitySchedule = {
      id: 1,
      weekday: 3,
      timeframe_id: 5,
      activity_group_id: 10,
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-15T12:00:00Z",
    };

    const result = mapActivityScheduleResponse(backendSchedule);

    expect(result.id).toBe("1");
    expect(result.weekday).toBe("3");
    expect(result.timeframe_id).toBe("5");
    expect(result.activity_id).toBe("10"); // activity_group_id → activity_id
    expect(result.created_at).toBeInstanceOf(Date);
  });

  it("handles missing timeframe_id", () => {
    const backendSchedule: BackendActivitySchedule = {
      id: 1,
      weekday: 1,
      activity_group_id: 10,
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-01T00:00:00Z",
    };

    const result = mapActivityScheduleResponse(backendSchedule);

    expect(result.timeframe_id).toBeUndefined();
  });
});

// ===== STUDENT MAPPING TESTS =====

describe("mapStudentEnrollmentResponse", () => {
  it("maps enrollment with direct person fields", () => {
    const enrollment: BackendStudentEnrollment = {
      id: 100,
      first_name: "Anna",
      last_name: "Schmidt",
      school_class: "3b",
      current_location: "Anwesend",
      activity_group_id: 5,
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-15T12:00:00Z",
    };

    const result = mapStudentEnrollmentResponse(enrollment);

    expect(result.id).toBe("100");
    expect(result.student_id).toBe("100"); // Backend returns id as student ID
    expect(result.name).toBe("Anna Schmidt");
    expect(result.school_class).toBe("3b");
    expect(result.activity_id).toBe("5");
    expect(result.current_location).toBe("Anwesend");
  });

  it("falls back to flattened fields when direct fields missing", () => {
    const enrollment: BackendStudentEnrollment = {
      id: 100,
      person__first_name: "Peter",
      person__last_name: "Mueller",
      student__school_class: "4a",
      student__current_location: "Schulhof",
    };

    const result = mapStudentEnrollmentResponse(enrollment);

    expect(result.name).toBe("Peter Mueller");
    expect(result.school_class).toBe("4a");
    expect(result.current_location).toBe("Schulhof");
  });

  it("returns 'Unnamed Student' when no name info available", () => {
    const enrollment: BackendStudentEnrollment = {
      id: 100,
    };

    const result = mapStudentEnrollmentResponse(enrollment);

    expect(result.name).toBe("Unnamed Student");
  });

  it("generates dates when not provided", () => {
    const enrollment: BackendStudentEnrollment = {
      id: 100,
    };

    const result = mapStudentEnrollmentResponse(enrollment);

    expect(result.created_at).toBeInstanceOf(Date);
    expect(result.updated_at).toBeInstanceOf(Date);
  });
});

describe("mapStudentEnrollmentsResponse", () => {
  it("maps array of enrollments", () => {
    const enrollments: BackendStudentEnrollment[] = [
      { id: 1, first_name: "Max", last_name: "Test" },
      { id: 2, first_name: "Anna", last_name: "Schmidt" },
    ];

    const result = mapStudentEnrollmentsResponse(enrollments);

    expect(result).toHaveLength(2);
    expect(result[0]?.name).toBe("Max Test");
    expect(result[1]?.name).toBe("Anna Schmidt");
  });
});

// ===== CREATOR CHECK =====

describe("isActivityCreator", () => {
  it("returns true when staff is the primary supervisor", () => {
    const activity: Activity = {
      id: "1",
      name: "Test",
      max_participant: 10,
      is_open_ags: true,
      supervisor_id: "100",
      ag_category_id: "1",
      created_at: new Date(),
      updated_at: new Date(),
      supervisors: [{ id: "1", staff_id: "100", is_primary: true }],
    };

    expect(isActivityCreator(activity, "100")).toBe(true);
  });

  it("returns false for non-primary supervisors", () => {
    const activity: Activity = {
      id: "1",
      name: "Test",
      max_participant: 10,
      is_open_ags: true,
      supervisor_id: "100",
      ag_category_id: "1",
      created_at: new Date(),
      updated_at: new Date(),
      supervisors: [
        { id: "1", staff_id: "100", is_primary: true },
        { id: "2", staff_id: "200", is_primary: false },
      ],
    };

    expect(isActivityCreator(activity, "200")).toBe(false);
  });

  it("uses first supervisor when no primary is marked", () => {
    const activity: Activity = {
      id: "1",
      name: "Test",
      max_participant: 10,
      is_open_ags: true,
      supervisor_id: "100",
      ag_category_id: "1",
      created_at: new Date(),
      updated_at: new Date(),
      supervisors: [{ id: "1", staff_id: "100", is_primary: false }],
    };

    expect(isActivityCreator(activity, "100")).toBe(true);
  });

  it("returns false for null/undefined staffId", () => {
    const activity: Activity = {
      id: "1",
      name: "Test",
      max_participant: 10,
      is_open_ags: true,
      supervisor_id: "100",
      ag_category_id: "1",
      created_at: new Date(),
      updated_at: new Date(),
      supervisors: [{ id: "1", staff_id: "100", is_primary: true }],
    };

    expect(isActivityCreator(activity, null)).toBe(false);
    expect(isActivityCreator(activity, undefined)).toBe(false);
  });

  it("returns false when no supervisors", () => {
    const activity: Activity = {
      id: "1",
      name: "Test",
      max_participant: 10,
      is_open_ags: true,
      supervisor_id: "100",
      ag_category_id: "1",
      created_at: new Date(),
      updated_at: new Date(),
    };

    expect(isActivityCreator(activity, "100")).toBe(false);
  });
});

// ===== GENERIC SUPERVISOR RESPONSE =====

describe("mapSupervisorResponse", () => {
  it("handles object with person nested structure", () => {
    const result = mapSupervisorResponse({
      id: 1,
      person: { first_name: "Max", last_name: "Test" },
    });

    expect(result.id).toBe("1");
    expect(result.name).toBe("Max Test");
  });

  it("handles object with direct first/last name", () => {
    const result = mapSupervisorResponse({
      id: 2,
      first_name: "Anna",
      last_name: "Schmidt",
    });

    expect(result.id).toBe("2");
    expect(result.name).toBe("Anna Schmidt");
  });

  it("handles object with name property", () => {
    const result = mapSupervisorResponse({
      id: 3,
      name: "Peter Mueller",
    });

    expect(result.id).toBe("3");
    expect(result.name).toBe("Peter Mueller");
  });

  it("returns fallback for null/undefined input", () => {
    expect(mapSupervisorResponse(null)).toEqual({
      id: "0",
      name: "Unknown Supervisor",
    });
    expect(mapSupervisorResponse(undefined)).toEqual({
      id: "0",
      name: "Unknown Supervisor",
    });
  });

  it("returns supervisor ID fallback when no name found", () => {
    const result = mapSupervisorResponse({ id: 999 });

    expect(result.name).toBe("Supervisor 999");
  });
});

// ===== TIMEFRAME =====

describe("mapTimeframeResponse", () => {
  it("maps backend timeframe to frontend structure", () => {
    const backendTimeframe: BackendTimeframe = {
      id: 1,
      name: "Morning",
      start_time: "08:00",
      end_time: "12:00",
      description: "Morning session",
      display_name: "8:00 - 12:00",
    };

    const result = mapTimeframeResponse(backendTimeframe);

    expect(result.id).toBe("1");
    expect(result.name).toBe("Morning");
    expect(result.start_time).toBe("08:00");
    expect(result.end_time).toBe("12:00");
    expect(result.description).toBe("Morning session");
    expect(result.display_name).toBe("8:00 - 12:00");
  });
});

// ===== PREPARE FOR BACKEND =====

describe("prepareActivityForBackend", () => {
  it("converts frontend activity to backend format", () => {
    const activity: Partial<Activity> = {
      id: "1",
      name: "Test Activity",
      max_participant: 20,
      is_open_ags: true,
      ag_category_id: "5",
      planned_room_id: "10",
      supervisor_id: "100",
    };

    const result = prepareActivityForBackend(activity);

    expect(result.id).toBe(1);
    expect(result.name).toBe("Test Activity");
    expect(result.max_participants).toBe(20);
    expect(result.is_open).toBe(true);
    expect(result.category_id).toBe(5);
    expect(result.planned_room_id).toBe(10);
    expect(result.supervisor_ids).toEqual([100]);
  });

  it("handles missing optional fields", () => {
    const activity: Partial<Activity> = {
      name: "New Activity",
    };

    const result = prepareActivityForBackend(activity);

    expect(result.id).toBeUndefined();
    expect(result.name).toBe("New Activity");
    expect(result.planned_room_id).toBeUndefined();
    expect(result.supervisor_ids).toBeUndefined();
  });

  it("preserves an unlimited participant capacity as null", () => {
    const result = prepareActivityForBackend({
      name: "Open Sports Hall",
      max_participant: null,
    });

    expect(result.max_participants).toBeNull();
  });
});

describe("prepareActivityScheduleForBackend", () => {
  it("converts frontend schedule to backend format", () => {
    const schedule: Partial<ActivitySchedule> = {
      id: "1",
      activity_id: "5",
      weekday: "3",
      timeframe_id: "10",
    };

    const result = prepareActivityScheduleForBackend(schedule);

    expect(result.id).toBe(1);
    expect(result.activity_group_id).toBe(5);
    expect(result.weekday).toBe(3);
    expect(result.timeframe_id).toBe(10);
  });
});

// ===== WEEKDAY FORMATTING =====

// ===== TIME FORMATTING =====

// ===== PARTICIPANT STATUS =====

// ===== AVAILABILITY CHECKS =====

describe("isSystemActivity", () => {
  const makeActivity = (name: string): Activity => ({
    id: "1",
    name,
    max_participant: 20,
    is_open_ags: false,
    supervisor_id: "1",
    ag_category_id: "1",
    created_at: new Date(),
    updated_at: new Date(),
  });

  it("returns true for Schulhof Freispiel", () => {
    expect(isSystemActivity(makeActivity("Schulhof Freispiel"))).toBe(true);
  });

  it("returns true for WC", () => {
    expect(isSystemActivity(makeActivity("WC"))).toBe(true);
  });

  it("returns false for regular activity", () => {
    expect(isSystemActivity(makeActivity("Basteln"))).toBe(false);
  });

  it("returns false for null", () => {
    expect(isSystemActivity(null)).toBe(false);
  });

  it("returns false for undefined", () => {
    expect(isSystemActivity(undefined)).toBe(false);
  });
});
