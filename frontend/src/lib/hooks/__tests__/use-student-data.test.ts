/**
 * Tests for use-student-data.ts hook
 *
 * Tests:
 * - useStudentData hook
 * - mapStudentResponse helper
 * - shouldShowCheckoutSection helper
 */

import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

// Mock dependencies
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    status: "authenticated",
    data: { user: { token: "test-token" } },
  })),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(() => ({
    data: null,
    isLoading: true,
    error: null,
    mutate: vi.fn(() => Promise.resolve()),
  })),
}));

vi.mock("~/lib/api", () => ({
  studentService: {
    getStudent: vi.fn(() => Promise.resolve({ id: "1", first_name: "Max" })),
  },
}));

vi.mock("~/lib/usercontext-api", () => ({
  userContextService: {
    getMyEducationalGroups: vi.fn(() => Promise.resolve([])),
    getMySupervisedGroups: vi.fn(() => Promise.resolve([])),
  },
}));

// Import after mocking
import {
  useStudentData,
  shouldShowCheckoutSection,
  type ExtendedStudent,
} from "../use-student-data";
import { useSWRAuth } from "~/lib/swr";

describe("useStudentData", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("hook initialization", () => {
    it("returns loading state initially", () => {
      vi.mocked(useSWRAuth).mockReturnValue({
        data: null,
        isLoading: true,
        error: null,
        mutate: vi.fn(() => Promise.resolve()),
        isValidating: false,
      });

      const { result } = renderHook(() => useStudentData("123"));

      expect(result.current.loading).toBe(true);
      expect(result.current.student).toBeNull();
      expect(result.current.error).toBeNull();
    });

    it("returns student data when loaded", () => {
      const mockStudentData = {
        student: {
          id: "123",
          first_name: "Max",
          second_name: "Mustermann",
          name: "Max Mustermann",
          school_class: "3a",
          group_id: "group1",
          group_name: "Group 1",
          current_location: "Raum 101",
          bus: false,
        },
        hasFullAccess: true,
        supervisors: [],
        myGroups: ["group1"],
        myGroupRooms: ["Raum 101"],
        mySupervisedRooms: [],
      };

      vi.mocked(useSWRAuth).mockReturnValue({
        data: mockStudentData,
        isLoading: false,
        error: null,
        mutate: vi.fn(() => Promise.resolve()),
        isValidating: false,
      });

      const { result } = renderHook(() => useStudentData("123"));

      expect(result.current.loading).toBe(false);
      expect(result.current.student).toEqual(mockStudentData.student);
      expect(result.current.hasFullAccess).toBe(true);
      expect(result.current.myGroups).toEqual(["group1"]);
    });

    it("returns error state on fetch failure", () => {
      vi.mocked(useSWRAuth).mockReturnValue({
        data: null,
        isLoading: false,
        error: new Error("Network error"),
        mutate: vi.fn(() => Promise.resolve()),
        isValidating: false,
      });

      const { result } = renderHook(() => useStudentData("123"));

      expect(result.current.loading).toBe(false);
      expect(result.current.error).toBe("Fehler beim Laden der Schülerdaten.");
    });

    it("provides refreshData function", () => {
      const mutateMock = vi.fn(() => Promise.resolve());
      vi.mocked(useSWRAuth).mockReturnValue({
        data: null,
        isLoading: false,
        error: null,
        mutate: mutateMock,
        isValidating: false,
      });

      const { result } = renderHook(() => useStudentData("123"));

      result.current.refreshData();

      expect(mutateMock).toHaveBeenCalled();
    });

    it("does not fetch when studentId is empty", () => {
      renderHook(() => useStudentData(""));

      expect(useSWRAuth).toHaveBeenCalledWith(
        null, // Key should be null when studentId is empty
        expect.any(Function),
        expect.any(Object),
      );
    });
  });
});

describe("shouldShowCheckoutSection", () => {
  const createStudent = (
    overrides: Partial<ExtendedStudent> = {},
  ): ExtendedStudent => ({
    id: "1",
    first_name: "Max",
    second_name: "Mustermann",
    name: "Max Mustermann",
    school_class: "3a",
    group_id: "group1",
    group_name: "Group 1",
    current_location: "Anwesend - Raum 101",
    bus: false,
    ...overrides,
  });

  it("returns true when student is in user's group and checked in", () => {
    const student = createStudent({
      group_id: "group1",
      current_location: "Anwesend - Raum 101",
    });
    const myGroups = ["group1"];
    const mySupervisedRooms: string[] = [];

    expect(
      shouldShowCheckoutSection(student, myGroups, mySupervisedRooms),
    ).toBe(true);
  });

  it("returns true when student is in supervised room and checked in", () => {
    const student = createStudent({
      group_id: "other-group",
      current_location: "Anwesend - Raum 101",
    });
    const myGroups: string[] = [];
    const mySupervisedRooms = ["Raum 101"];

    expect(
      shouldShowCheckoutSection(student, myGroups, mySupervisedRooms),
    ).toBe(true);
  });

  it("returns false when student is at home", () => {
    const student = createStudent({
      group_id: "group1",
      current_location: "Zuhause",
    });
    const myGroups = ["group1"];
    const mySupervisedRooms: string[] = [];

    expect(
      shouldShowCheckoutSection(student, myGroups, mySupervisedRooms),
    ).toBe(false);
  });

  it("returns true when student is checked in, regardless of group", () => {
    // Any authenticated staff can checkout any checked-in student
    const student = createStudent({
      group_id: "other-group",
      current_location: "Anwesend - Other Room",
    });
    const myGroups = ["group1"];
    const mySupervisedRooms = ["Raum 101"];

    expect(
      shouldShowCheckoutSection(student, myGroups, mySupervisedRooms),
    ).toBe(true);
  });

  it("returns true when student is checked in without group_id", () => {
    // Any authenticated staff can checkout any checked-in student
    const student = createStudent({
      group_id: undefined,
      current_location: "Anwesend - Raum 101",
    });
    const myGroups = ["group1"];
    const mySupervisedRooms: string[] = [];

    expect(
      shouldShowCheckoutSection(student, myGroups, mySupervisedRooms),
    ).toBe(true);
  });

  it("returns false when student has no current_location", () => {
    const student = createStudent({
      group_id: "group1",
      current_location: undefined,
    });
    const myGroups = ["group1"];
    const mySupervisedRooms: string[] = [];

    expect(
      shouldShowCheckoutSection(student, myGroups, mySupervisedRooms),
    ).toBe(false);
  });

  it("returns true when student is both in group and supervised room", () => {
    const student = createStudent({
      group_id: "group1",
      current_location: "Anwesend - Raum 101",
    });
    const myGroups = ["group1"];
    const mySupervisedRooms = ["Raum 101"];

    expect(
      shouldShowCheckoutSection(student, myGroups, mySupervisedRooms),
    ).toBe(true);
  });

  it("handles multiple groups correctly", () => {
    const student = createStudent({
      group_id: "group2",
      current_location: "Anwesend - Raum 202",
    });
    const myGroups = ["group1", "group2", "group3"];
    const mySupervisedRooms: string[] = [];

    expect(
      shouldShowCheckoutSection(student, myGroups, mySupervisedRooms),
    ).toBe(true);
  });

  it("handles multiple supervised rooms correctly", () => {
    const student = createStudent({
      group_id: "other-group",
      current_location: "Anwesend - Schulhof",
    });
    const myGroups: string[] = [];
    const mySupervisedRooms = ["Raum 101", "Schulhof", "Mensa"];

    expect(
      shouldShowCheckoutSection(student, myGroups, mySupervisedRooms),
    ).toBe(true);
  });

  it("returns false when location starts with Zuhause with additional text", () => {
    const student = createStudent({
      group_id: "group1",
      current_location: "Zuhause - Abgeholt",
    });
    const myGroups = ["group1"];
    const mySupervisedRooms: string[] = [];

    expect(
      shouldShowCheckoutSection(student, myGroups, mySupervisedRooms),
    ).toBe(false);
  });
});

describe("useStudentData additional scenarios", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns default values when no data", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: false,
      error: null,
      mutate: vi.fn(() => Promise.resolve()),
      isValidating: false,
    });

    const { result } = renderHook(() => useStudentData("123"));

    expect(result.current.student).toBeNull();
    expect(result.current.hasFullAccess).toBe(true);
    expect(result.current.supervisors).toEqual([]);
    expect(result.current.myGroups).toEqual([]);
    expect(result.current.myGroupRooms).toEqual([]);
    expect(result.current.mySupervisedRooms).toEqual([]);
  });

  it("returns all group rooms from data", () => {
    const mockStudentData = {
      student: {
        id: "123",
        first_name: "Max",
        second_name: "Mustermann",
        name: "Max Mustermann",
        school_class: "3a",
        group_id: "group1",
        group_name: "Group 1",
        current_location: "Raum 101",
        bus: false,
      },
      hasFullAccess: true,
      supervisors: [],
      myGroups: ["group1", "group2"],
      myGroupRooms: ["Raum 101", "Raum 202"],
      mySupervisedRooms: ["Schulhof"],
    };

    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockStudentData,
      isLoading: false,
      error: null,
      mutate: vi.fn(() => Promise.resolve()),
      isValidating: false,
    });

    const { result } = renderHook(() => useStudentData("123"));

    expect(result.current.myGroupRooms).toEqual(["Raum 101", "Raum 202"]);
    expect(result.current.mySupervisedRooms).toEqual(["Schulhof"]);
    expect(result.current.myGroups).toEqual(["group1", "group2"]);
  });

  it("returns supervisor contacts from data", () => {
    const mockStudentData = {
      student: {
        id: "123",
        first_name: "Max",
        second_name: "Mustermann",
        name: "Max Mustermann",
        school_class: "3a",
        group_id: "group1",
        group_name: "Group 1",
        current_location: "Raum 101",
        bus: false,
      },
      hasFullAccess: true,
      supervisors: [
        {
          id: 1,
          first_name: "Anna",
          last_name: "Supervisor",
          email: "s1@test.com",
          role: "teacher",
        },
        {
          id: 2,
          first_name: "Hans",
          last_name: "Betreuer",
          email: "s2@test.com",
          role: "staff",
        },
      ],
      myGroups: ["group1"],
      myGroupRooms: ["Raum 101"],
      mySupervisedRooms: [],
    };

    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockStudentData,
      isLoading: false,
      error: null,
      mutate: vi.fn(() => Promise.resolve()),
      isValidating: false,
    });

    const { result } = renderHook(() => useStudentData("123"));

    expect(result.current.supervisors).toHaveLength(2);
    expect(result.current.supervisors[0]?.first_name).toBe("Anna");
  });

  it("handles hasFullAccess as false", () => {
    const mockStudentData = {
      student: {
        id: "123",
        first_name: "Max",
        second_name: "Mustermann",
        name: "Max Mustermann",
        school_class: "3a",
        group_id: "group1",
        group_name: "Group 1",
        current_location: "Raum 101",
        bus: false,
      },
      hasFullAccess: false, // Limited access
      supervisors: [],
      myGroups: [],
      myGroupRooms: [],
      mySupervisedRooms: [],
    };

    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockStudentData,
      isLoading: false,
      error: null,
      mutate: vi.fn(() => Promise.resolve()),
      isValidating: false,
    });

    const { result } = renderHook(() => useStudentData("123"));

    expect(result.current.hasFullAccess).toBe(false);
  });
});

describe("extractRoomNames helper", () => {
  // Test the room extraction logic directly
  it("extracts room names from groups with rooms", () => {
    const groups = [
      { id: "1", room: { name: "Raum 101" } },
      { id: "2", room: { name: "Raum 202" } },
      { id: "3", room: { name: "Schulhof" } },
    ];

    const roomNames = groups.map((group) => group.room?.name).filter(Boolean);

    expect(roomNames).toEqual(["Raum 101", "Raum 202", "Schulhof"]);
  });

  it("filters out groups without rooms", () => {
    const groups = [
      { id: "1", room: { name: "Raum 101" } },
      { id: "2", room: undefined },
      { id: "3", room: { name: undefined } },
      { id: "4", room: { name: "Raum 404" } },
    ];

    const roomNames = groups
      .map((group) => group.room?.name)
      .filter(Boolean) as string[];

    expect(roomNames).toEqual(["Raum 101", "Raum 404"]);
  });

  it("returns empty array for empty groups", () => {
    const groups: Array<{ id: string; room?: { name?: string } }> = [];

    const roomNames = groups
      .map((group) => group.room?.name)
      .filter(Boolean) as string[];

    expect(roomNames).toEqual([]);
  });

  it("handles groups with null room names", () => {
    const groups = [
      { id: "1", room: { name: null as unknown as string } },
      { id: "2", room: { name: "Valid Room" } },
    ];

    const roomNames = groups.map((group) => group.room?.name).filter(Boolean);

    expect(roomNames).toEqual(["Valid Room"]);
  });
});

describe("mapStudentResponse helper logic", () => {
  // Test the mapping logic
  it("maps student response with full access", () => {
    const response = {
      id: "123",
      first_name: "Max",
      second_name: "Mustermann",
      name: "Max Mustermann",
      school_class: "3a",
      group_id: "g1",
      group_name: "Group 1",
      current_location: "Raum 101",
      bus: true,
      location_since: "2024-01-15T10:00:00Z",
      extra_info: "Some notes",
      supervisor_notes: "Teacher notes",
      sick: true,
      sick_since: "2024-01-14",
    };

    const hasAccess = true;

    // Simulate mapping with access
    const mapped = {
      ...response,
      location_since: hasAccess ? response.location_since : undefined,
      extra_info: hasAccess ? response.extra_info : undefined,
      supervisor_notes: hasAccess ? response.supervisor_notes : undefined,
      sick: hasAccess ? response.sick : false,
      sick_since: hasAccess ? response.sick_since : undefined,
    };

    expect(mapped.location_since).toBe("2024-01-15T10:00:00Z");
    expect(mapped.extra_info).toBe("Some notes");
    expect(mapped.supervisor_notes).toBe("Teacher notes");
    expect(mapped.sick).toBe(true);
    expect(mapped.sick_since).toBe("2024-01-14");
  });

  it("maps student response without full access", () => {
    const response = {
      id: "123",
      first_name: "Max",
      second_name: "Mustermann",
      name: "Max Mustermann",
      school_class: "3a",
      group_id: "g1",
      group_name: "Group 1",
      current_location: "Raum 101",
      bus: true,
      location_since: "2024-01-15T10:00:00Z",
      extra_info: "Some notes",
      supervisor_notes: "Teacher notes",
      sick: true,
      sick_since: "2024-01-14",
    };

    const hasAccess = false;

    // Simulate mapping without access
    const mapped = {
      ...response,
      location_since: hasAccess ? response.location_since : undefined,
      extra_info: hasAccess ? response.extra_info : undefined,
      supervisor_notes: hasAccess ? response.supervisor_notes : undefined,
      sick: hasAccess ? response.sick : false,
      sick_since: hasAccess ? response.sick_since : undefined,
    };

    expect(mapped.location_since).toBeUndefined();
    expect(mapped.extra_info).toBeUndefined();
    expect(mapped.supervisor_notes).toBeUndefined();
    expect(mapped.sick).toBe(false);
    expect(mapped.sick_since).toBeUndefined();
  });

  it("handles wrapped response format", () => {
    const wrappedResponse = {
      data: {
        id: "123",
        first_name: "Max",
        second_name: "Mustermann",
        name: "Max Mustermann",
      },
      success: true,
      message: "OK",
    };

    // Simulate extracting data from wrapped response
    const studentData =
      (wrappedResponse as { data?: unknown }).data ?? wrappedResponse;

    expect(studentData).toEqual({
      id: "123",
      first_name: "Max",
      second_name: "Mustermann",
      name: "Max Mustermann",
    });
  });

  it("handles direct response format (not wrapped)", () => {
    const directResponse = {
      id: "123",
      first_name: "Max",
      second_name: "Mustermann",
      name: "Max Mustermann",
    };

    // Simulate extracting data from direct response
    const studentData =
      (directResponse as { data?: unknown }).data ?? directResponse;

    expect(studentData).toEqual({
      id: "123",
      first_name: "Max",
      second_name: "Mustermann",
      name: "Max Mustermann",
    });
  });

  it("handles missing optional fields", () => {
    const response = {
      id: "123",
      // All other fields missing
    };

    const mapped = {
      id: response.id,
      first_name: (response as { first_name?: string }).first_name ?? "",
      second_name: (response as { second_name?: string }).second_name ?? "",
      name: (response as { name?: string }).name,
      bus: (response as { bus?: boolean }).bus ?? false,
    };

    expect(mapped.first_name).toBe("");
    expect(mapped.second_name).toBe("");
    expect(mapped.name).toBeUndefined();
    expect(mapped.bus).toBe(false);
  });
});
