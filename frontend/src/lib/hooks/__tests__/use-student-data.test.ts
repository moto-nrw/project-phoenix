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
    mutate: vi.fn(),
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
        mutate: vi.fn(),
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
        mutate: vi.fn(),
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
        mutate: vi.fn(),
        isValidating: false,
      });

      const { result } = renderHook(() => useStudentData("123"));

      expect(result.current.loading).toBe(false);
      expect(result.current.error).toBe("Fehler beim Laden der Schülerdaten.");
    });

    it("provides refreshData function", () => {
      const mutateMock = vi.fn();
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

  it("returns false when student is not in user's group or supervised room", () => {
    const student = createStudent({
      group_id: "other-group",
      current_location: "Anwesend - Other Room",
    });
    const myGroups = ["group1"];
    const mySupervisedRooms = ["Raum 101"];

    expect(
      shouldShowCheckoutSection(student, myGroups, mySupervisedRooms),
    ).toBe(false);
  });

  it("returns false when student has no group_id", () => {
    const student = createStudent({
      group_id: undefined,
      current_location: "Anwesend - Raum 101",
    });
    const myGroups = ["group1"];
    const mySupervisedRooms: string[] = [];

    expect(
      shouldShowCheckoutSection(student, myGroups, mySupervisedRooms),
    ).toBe(false);
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
});
