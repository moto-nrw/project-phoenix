import { describe, it, expect } from "vitest";
import type {
  BackendActivity,
  BackendActivityCategory,
  BackendActivitySchedule,
  BackendActivityStudent,
  BackendStudentEnrollment,
  BackendActivitySupervisor,
  BackendTimeframe,
  Activity,
  ActivitySchedule,
  ActivityStudent,
  ActivitySupervisor,
} from "./activity-helpers";
import {
  mapActivitySupervisorsResponse,
  prepareSupervisorAssignmentForBackend,
  formatSupervisorList,
  getPrimarySupervisor,
  mapActivityResponse,
  mapActivityCategoryResponse,
  mapActivityScheduleResponse,
  mapActivityStudentResponse,
  mapActivityStudentsResponse,
  mapStudentEnrollmentResponse,
  mapStudentEnrollmentsResponse,
  formatStudentList,
  groupStudentsByClass,
  prepareBatchEnrollmentForBackend,
  filterStudentsBySearchTerm,
  isActivityCreator,
  mapSupervisorResponse,
  mapTimeframeResponse,
  prepareActivityForBackend,
  prepareActivityScheduleForBackend,
  formatActivityTimes,
  formatWeekday,
  getWeekdayFullName,
  getWeekdayOrder,
  sortSchedulesByWeekday,
  formatScheduleTime,
  formatParticipantStatus,
  isTimeSlotAvailable,
  isSupervisorAvailable,
  getActivityCategoryColor,
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

describe("mapActivitySupervisorsResponse", () => {
  it("maps array of backend supervisors", () => {
    const supervisors: BackendActivitySupervisor[] = [
      sampleBackendSupervisor,
      {
        id: 2,
        staff_id: 101,
        is_primary: false,
        first_name: "Peter",
        last_name: "Mueller",
      },
    ];

    const result = mapActivitySupervisorsResponse(supervisors);

    expect(result).toHaveLength(2);
    expect(result[0]?.id).toBe("1");
    expect(result[0]?.staff_id).toBe("100");
    expect(result[0]?.is_primary).toBe(true);
    expect(result[0]?.full_name).toBe("Maria Schmidt");
    expect(result[1]?.is_primary).toBe(false);
  });

  it("handles supervisors without names", () => {
    const supervisors: BackendActivitySupervisor[] = [
      { id: 3, staff_id: 102, is_primary: true },
    ];

    const result = mapActivitySupervisorsResponse(supervisors);

    expect(result[0]?.full_name).toBeUndefined();
    expect(result[0]?.first_name).toBeUndefined();
  });
});

describe("prepareSupervisorAssignmentForBackend", () => {
  it("converts staff_id from string to number", () => {
    const result = prepareSupervisorAssignmentForBackend({
      staff_id: "123",
      is_primary: true,
    });

    expect(result.staff_id).toBe(123);
    expect(result.is_primary).toBe(true);
  });

  it("handles undefined is_primary", () => {
    const result = prepareSupervisorAssignmentForBackend({ staff_id: "456" });

    expect(result.staff_id).toBe(456);
    expect(result.is_primary).toBeUndefined();
  });
});

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

describe("getPrimarySupervisor", () => {
  it("returns primary supervisor when present", () => {
    const supervisors: ActivitySupervisor[] = [
      { id: "1", staff_id: "100", is_primary: false },
      { id: "2", staff_id: "101", is_primary: true },
    ];

    const result = getPrimarySupervisor(supervisors);

    expect(result?.id).toBe("2");
    expect(result?.is_primary).toBe(true);
  });

  it("returns undefined for empty or undefined list", () => {
    expect(getPrimarySupervisor(undefined)).toBeUndefined();
    expect(getPrimarySupervisor([])).toBeUndefined();
  });

  it("returns undefined when no primary supervisor exists", () => {
    const supervisors: ActivitySupervisor[] = [
      { id: "1", staff_id: "100", is_primary: false },
    ];

    const result = getPrimarySupervisor(supervisors);

    expect(result).toBeUndefined();
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

describe("mapActivityStudentResponse", () => {
  it("maps backend student to frontend structure", () => {
    const backendStudent: BackendActivityStudent = {
      id: 1,
      student_id: 100,
      activity_id: 5,
      name: "Max Mustermann",
      school_class: "3a",
      current_location: "Anwesend - Raum 101",
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-15T12:00:00Z",
    };

    const result = mapActivityStudentResponse(backendStudent);

    expect(result.id).toBe("1");
    expect(result.student_id).toBe("100");
    expect(result.activity_id).toBe("5");
    expect(result.name).toBe("Max Mustermann");
    expect(result.school_class).toBe("3a");
    expect(result.current_location).toBe("Anwesend - Raum 101");
  });

  it("normalizes null location to 'Unbekannt'", () => {
    const backendStudent: BackendActivityStudent = {
      id: 1,
      student_id: 100,
      activity_id: 5,
      current_location: null,
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-01T00:00:00Z",
    };

    const result = mapActivityStudentResponse(backendStudent);

    expect(result.current_location).toBe("Unbekannt");
  });
});

describe("mapActivityStudentsResponse", () => {
  it("maps array of backend students", () => {
    const students: BackendActivityStudent[] = [
      {
        id: 1,
        student_id: 100,
        activity_id: 5,
        created_at: "",
        updated_at: "",
      },
      {
        id: 2,
        student_id: 101,
        activity_id: 5,
        created_at: "",
        updated_at: "",
      },
    ];

    const result = mapActivityStudentsResponse(students);

    expect(result).toHaveLength(2);
    expect(result[0]?.id).toBe("1");
    expect(result[1]?.id).toBe("2");
  });
});

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

describe("formatStudentList", () => {
  it("formats list of students by name", () => {
    const students: ActivityStudent[] = [
      {
        id: "1",
        activity_id: "5",
        student_id: "100",
        name: "Max Mustermann",
        current_location: "",
        created_at: new Date(),
        updated_at: new Date(),
      },
      {
        id: "2",
        activity_id: "5",
        student_id: "101",
        name: "Anna Schmidt",
        current_location: "",
        created_at: new Date(),
        updated_at: new Date(),
      },
    ];

    const result = formatStudentList(students);

    expect(result).toBe("Max Mustermann, Anna Schmidt");
  });

  it("returns German message for empty or undefined list", () => {
    expect(formatStudentList(undefined)).toBe(
      "Keine Teilnehmer eingeschrieben",
    );
    expect(formatStudentList([])).toBe("Keine Teilnehmer eingeschrieben");
  });

  it("falls back to student_id when name is missing", () => {
    const students: ActivityStudent[] = [
      {
        id: "1",
        activity_id: "5",
        student_id: "100",
        current_location: "",
        created_at: new Date(),
        updated_at: new Date(),
      },
    ];

    const result = formatStudentList(students);

    expect(result).toBe("Student 100");
  });
});

describe("groupStudentsByClass", () => {
  it("groups students by school class", () => {
    const students: ActivityStudent[] = [
      {
        id: "1",
        activity_id: "5",
        student_id: "100",
        school_class: "3a",
        current_location: "",
        created_at: new Date(),
        updated_at: new Date(),
      },
      {
        id: "2",
        activity_id: "5",
        student_id: "101",
        school_class: "3a",
        current_location: "",
        created_at: new Date(),
        updated_at: new Date(),
      },
      {
        id: "3",
        activity_id: "5",
        student_id: "102",
        school_class: "3b",
        current_location: "",
        created_at: new Date(),
        updated_at: new Date(),
      },
    ];

    const result = groupStudentsByClass(students);

    expect(Object.keys(result)).toHaveLength(2);
    expect(result["3a"]).toHaveLength(2);
    expect(result["3b"]).toHaveLength(1);
  });

  it("uses 'Keine Klasse' for students without school_class", () => {
    const students: ActivityStudent[] = [
      {
        id: "1",
        activity_id: "5",
        student_id: "100",
        current_location: "",
        created_at: new Date(),
        updated_at: new Date(),
      },
    ];

    const result = groupStudentsByClass(students);

    expect(result["Keine Klasse"]).toHaveLength(1);
  });
});

describe("prepareBatchEnrollmentForBackend", () => {
  it("converts student IDs from strings to numbers", () => {
    const result = prepareBatchEnrollmentForBackend(["1", "2", "3"]);

    expect(result.student_ids).toEqual([1, 2, 3]);
  });
});

describe("filterStudentsBySearchTerm", () => {
  const students: ActivityStudent[] = [
    {
      id: "1",
      activity_id: "5",
      student_id: "100",
      name: "Max Mustermann",
      school_class: "3a",
      current_location: "",
      created_at: new Date(),
      updated_at: new Date(),
    },
    {
      id: "2",
      activity_id: "5",
      student_id: "200",
      name: "Anna Schmidt",
      school_class: "4b",
      current_location: "",
      created_at: new Date(),
      updated_at: new Date(),
    },
  ];

  it("returns all students when search term is empty", () => {
    expect(filterStudentsBySearchTerm(students, "")).toHaveLength(2);
    expect(filterStudentsBySearchTerm(students, "")).toEqual(students);
  });

  it("filters by name (case-insensitive)", () => {
    const result = filterStudentsBySearchTerm(students, "max");

    expect(result).toHaveLength(1);
    expect(result[0]?.name).toBe("Max Mustermann");
  });

  it("filters by school class", () => {
    const result = filterStudentsBySearchTerm(students, "4b");

    expect(result).toHaveLength(1);
    expect(result[0]?.name).toBe("Anna Schmidt");
  });

  it("filters by student_id", () => {
    const result = filterStudentsBySearchTerm(students, "200");

    expect(result).toHaveLength(1);
    expect(result[0]?.student_id).toBe("200");
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

describe("formatWeekday", () => {
  it("formats weekday numbers to German abbreviations", () => {
    expect(formatWeekday("1")).toBe("Mo");
    expect(formatWeekday("2")).toBe("Di");
    expect(formatWeekday("3")).toBe("Mi");
    expect(formatWeekday("4")).toBe("Do");
    expect(formatWeekday("5")).toBe("Fr");
    expect(formatWeekday("6")).toBe("Sa");
    expect(formatWeekday("7")).toBe("So");
  });

  it("returns original value for unknown weekday", () => {
    expect(formatWeekday("8")).toBe("8");
    expect(formatWeekday("unknown")).toBe("unknown");
  });
});

describe("getWeekdayFullName", () => {
  it("returns full German weekday names", () => {
    expect(getWeekdayFullName("1")).toBe("Montag");
    expect(getWeekdayFullName("2")).toBe("Dienstag");
    expect(getWeekdayFullName("3")).toBe("Mittwoch");
    expect(getWeekdayFullName("4")).toBe("Donnerstag");
    expect(getWeekdayFullName("5")).toBe("Freitag");
    expect(getWeekdayFullName("6")).toBe("Samstag");
    expect(getWeekdayFullName("7")).toBe("Sonntag");
  });
});

describe("getWeekdayOrder", () => {
  it("returns correct order for weekdays", () => {
    expect(getWeekdayOrder("1")).toBe(1);
    expect(getWeekdayOrder("7")).toBe(7);
  });

  it("returns 99 for unknown weekday", () => {
    expect(getWeekdayOrder("unknown")).toBe(99);
  });
});

describe("sortSchedulesByWeekday", () => {
  it("sorts schedules by weekday order", () => {
    const schedules: ActivitySchedule[] = [
      {
        id: "1",
        activity_id: "5",
        weekday: "5",
        created_at: new Date(),
        updated_at: new Date(),
      }, // Friday
      {
        id: "2",
        activity_id: "5",
        weekday: "1",
        created_at: new Date(),
        updated_at: new Date(),
      }, // Monday
      {
        id: "3",
        activity_id: "5",
        weekday: "3",
        created_at: new Date(),
        updated_at: new Date(),
      }, // Wednesday
    ];

    const result = sortSchedulesByWeekday(schedules);

    expect(result[0]?.weekday).toBe("1"); // Monday first
    expect(result[1]?.weekday).toBe("3"); // Wednesday
    expect(result[2]?.weekday).toBe("5"); // Friday last
  });

  it("does not mutate original array", () => {
    const schedules: ActivitySchedule[] = [
      {
        id: "1",
        activity_id: "5",
        weekday: "5",
        created_at: new Date(),
        updated_at: new Date(),
      },
    ];

    const result = sortSchedulesByWeekday(schedules);

    expect(result).not.toBe(schedules);
  });
});

// ===== TIME FORMATTING =====

describe("formatActivityTimes", () => {
  it("formats activity with times array", () => {
    const activity: Activity = {
      id: "1",
      name: "Test",
      max_participant: 10,
      is_open_ags: true,
      supervisor_id: "100",
      ag_category_id: "1",
      created_at: new Date(),
      updated_at: new Date(),
      times: [
        {
          id: "1",
          activity_id: "1",
          weekday: "1",
          created_at: new Date(),
          updated_at: new Date(),
        },
        {
          id: "2",
          activity_id: "1",
          weekday: "3",
          created_at: new Date(),
          updated_at: new Date(),
        },
      ],
    };

    const result = formatActivityTimes(activity);

    expect(result).toBe("Mo, Mi");
  });

  it("formats array of schedules directly", () => {
    const schedules: ActivitySchedule[] = [
      {
        id: "1",
        activity_id: "1",
        weekday: "2",
        created_at: new Date(),
        updated_at: new Date(),
      },
    ];

    const result = formatActivityTimes(schedules);

    expect(result).toBe("Di");
  });

  it("returns German message for empty times", () => {
    const activity: Activity = {
      id: "1",
      name: "Test",
      max_participant: 10,
      is_open_ags: true,
      supervisor_id: "100",
      ag_category_id: "1",
      created_at: new Date(),
      updated_at: new Date(),
      times: [],
    };

    expect(formatActivityTimes(activity)).toBe("Keine Zeiten festgelegt");
    expect(formatActivityTimes([])).toBe("Keine Zeiten festgelegt");
  });
});

describe("formatScheduleTime", () => {
  it("includes timeframe when available", () => {
    const schedule: ActivitySchedule = {
      id: "1",
      activity_id: "5",
      weekday: "1",
      timeframe_id: "10",
      created_at: new Date(),
      updated_at: new Date(),
    };
    const timeframes = [{ id: "10", start_time: "14:00", end_time: "16:00" }];

    const result = formatScheduleTime(schedule, timeframes);

    expect(result).toBe("Mo 14:00-16:00");
  });

  it("returns weekday only when no timeframe", () => {
    const schedule: ActivitySchedule = {
      id: "1",
      activity_id: "5",
      weekday: "3",
      created_at: new Date(),
      updated_at: new Date(),
    };

    expect(formatScheduleTime(schedule)).toBe("Mi");
    expect(formatScheduleTime(schedule, [])).toBe("Mi");
  });
});

// ===== PARTICIPANT STATUS =====

describe("formatParticipantStatus", () => {
  it("formats from activity object", () => {
    const activity: Activity = {
      id: "1",
      name: "Test",
      max_participant: 20,
      is_open_ags: true,
      supervisor_id: "100",
      ag_category_id: "1",
      created_at: new Date(),
      updated_at: new Date(),
      participant_count: 15,
    };

    const result = formatParticipantStatus(activity);

    expect(result).toBe("15 / 20 Teilnehmer");
  });

  it("formats from number parameters", () => {
    const result = formatParticipantStatus(10, 25);

    expect(result).toBe("10 / 25 Teilnehmer");
  });

  it("returns 'Unbekannt' for undefined values", () => {
    const activity: Activity = {
      id: "1",
      name: "Test",
      max_participant: 20,
      is_open_ags: true,
      supervisor_id: "100",
      ag_category_id: "1",
      created_at: new Date(),
      updated_at: new Date(),
    };

    expect(formatParticipantStatus(activity)).toBe("Unbekannt");
  });
});

// ===== AVAILABILITY CHECKS =====

describe("isTimeSlotAvailable", () => {
  const existingSchedules: ActivitySchedule[] = [
    {
      id: "1",
      activity_id: "5",
      weekday: "1",
      timeframe_id: "10",
      created_at: new Date(),
      updated_at: new Date(),
    },
    {
      id: "2",
      activity_id: "5",
      weekday: "3",
      timeframe_id: "10",
      created_at: new Date(),
      updated_at: new Date(),
    },
  ];

  it("returns true for available slot", () => {
    expect(isTimeSlotAvailable("2", "10", existingSchedules)).toBe(true); // Tuesday
  });

  it("returns false for taken slot", () => {
    expect(isTimeSlotAvailable("1", "10", existingSchedules)).toBe(false); // Monday
  });

  it("excludes specified schedule when checking", () => {
    // Slot would be taken, but we're editing that schedule
    expect(isTimeSlotAvailable("1", "10", existingSchedules, "1")).toBe(true);
  });
});

describe("isSupervisorAvailable", () => {
  const existingSchedules = [
    { activity_id: "1", weekday: "1", timeframe_id: "10" },
    { activity_id: "2", weekday: "3", timeframe_id: "15" },
  ];

  it("returns true when supervisor has no conflict", () => {
    expect(isSupervisorAvailable("100", "2", "10", existingSchedules)).toBe(
      true,
    );
  });

  it("returns false when supervisor has conflict", () => {
    expect(isSupervisorAvailable("100", "1", "10", existingSchedules)).toBe(
      false,
    );
  });
});

// ===== CATEGORY COLORS =====

describe("getActivityCategoryColor", () => {
  it("returns specific colors for known categories", () => {
    expect(getActivityCategoryColor("Sport")).toBe(
      "from-blue-500 to-indigo-600",
    );
    expect(getActivityCategoryColor("Kreativ")).toBe(
      "from-purple-500 to-pink-600",
    );
    expect(getActivityCategoryColor("Musik")).toBe("from-pink-500 to-rose-600");
    expect(getActivityCategoryColor("Spiele")).toBe(
      "from-green-500 to-emerald-600",
    );
    expect(getActivityCategoryColor("Lernen")).toBe(
      "from-yellow-500 to-orange-600",
    );
    expect(getActivityCategoryColor("Hausaufgaben")).toBe(
      "from-red-500 to-pink-600",
    );
    expect(getActivityCategoryColor("Draußen")).toBe(
      "from-green-600 to-teal-600",
    );
    expect(getActivityCategoryColor("Gruppenraum")).toBe(
      "from-slate-500 to-gray-600",
    );
    expect(getActivityCategoryColor("Mensa")).toBe(
      "from-orange-500 to-amber-600",
    );
  });

  it("returns gray for null/undefined category", () => {
    expect(getActivityCategoryColor(null)).toBe("from-gray-500 to-gray-600");
    expect(getActivityCategoryColor(undefined)).toBe(
      "from-gray-500 to-gray-600",
    );
  });

  it("returns gray for unknown category", () => {
    expect(getActivityCategoryColor("Unknown Category")).toBe(
      "from-gray-500 to-gray-600",
    );
  });
});
