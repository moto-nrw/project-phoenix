import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useStudentData, shouldShowCheckoutSection } from "./use-student-data";
import type { ExtendedStudent } from "./use-student-data";

// Mock dependencies
vi.mock("next-auth/react");
vi.mock("~/lib/swr");
vi.mock("~/lib/api");
vi.mock("~/lib/usercontext-api");

// Import mocked modules
import { useSession } from "next-auth/react";
import { useSWRAuth } from "~/lib/swr";
import { studentService } from "~/lib/api";
import { userContextService } from "~/lib/usercontext-api";

// Type the mocked functions
const mockUseSession = vi.mocked(useSession);
const mockUseSWRAuth = vi.mocked(useSWRAuth);
const mockStudentService = vi.mocked(studentService);
const mockUserContextService = vi.mocked(userContextService);

describe("useStudentData", () => {
  const mockStudent: ExtendedStudent = {
    id: "1",
    first_name: "John",
    second_name: "Doe",
    name: "John Doe",
    school_class: "5a",
    group_id: "100",
    group_name: "Group A",
    current_location: "Room 101",
    location_since: "2024-01-15T10:00:00Z",
    bus: false,
    buskind: false,
    birthday: "2010-05-15",
    extra_info: "Extra info",
    supervisor_notes: "Notes",
    health_info: "Health info",
    pickup_status: "regular",
    sick: false,
  };

  const mockSupervisors = [
    {
      id: 1,
      first_name: "Teacher",
      last_name: "One",
      name: "Teacher One",
      email: "teacher1@school.com",
      phone: "+49123456789",
      role: "teacher",
    },
  ];

  const mockGroups = [
    { id: "100", name: "Group A", room: { id: "10", name: "Room A" } },
    { id: "101", name: "Group B", room: { id: "11", name: "Room B" } },
  ];

  const mockSupervisedGroups = [
    { id: "200", name: "Supervised Group", room: { id: "12", name: "Room C" } },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("loading states", () => {
    it("should show loading when session is loading", () => {
      mockUseSession.mockReturnValue({
        data: null,
        status: "loading",
        update: vi.fn(),
      });

      mockUseSWRAuth.mockReturnValue({
        data: undefined,
        error: undefined,
        isLoading: false,
        isValidating: false,
        mutate: vi.fn(),
      });

      const { result } = renderHook(() => useStudentData("1"));

      expect(result.current.loading).toBe(true);
      expect(result.current.student).toBeNull();
    });

    it("should show loading when SWR is loading", () => {
      mockUseSession.mockReturnValue({
        data: { user: { id: "1", token: "test-token" }, expires: "2099-12-31" },
        status: "authenticated",
        update: vi.fn(),
      });

      mockUseSWRAuth.mockReturnValue({
        data: undefined,
        error: undefined,
        isLoading: true,
        isValidating: false,
        mutate: vi.fn(),
      });

      const { result } = renderHook(() => useStudentData("1"));

      expect(result.current.loading).toBe(true);
      expect(result.current.student).toBeNull();
    });

    it("should not be loading when data is available", () => {
      mockUseSession.mockReturnValue({
        data: { user: { id: "1", token: "test-token" }, expires: "2099-12-31" },
        status: "authenticated",
        update: vi.fn(),
      });

      mockUseSWRAuth.mockReturnValue({
        data: {
          student: mockStudent,
          hasFullAccess: true,
          supervisors: mockSupervisors,
          myGroups: ["100", "101"],
          myGroupRooms: ["Room A", "Room B"],
          mySupervisedRooms: ["Room C"],
        },
        error: undefined,
        isLoading: false,
        isValidating: false,
        mutate: vi.fn(),
      });

      const { result } = renderHook(() => useStudentData("1"));

      expect(result.current.loading).toBe(false);
      expect(result.current.student).toEqual(mockStudent);
    });
  });

  describe("error states", () => {
    it("should return error message when SWR has error", () => {
      mockUseSession.mockReturnValue({
        data: { user: { id: "1", token: "test-token" }, expires: "2099-12-31" },
        status: "authenticated",
        update: vi.fn(),
      });

      mockUseSWRAuth.mockReturnValue({
        data: undefined,
        error: new Error("Network error"),
        isLoading: false,
        isValidating: false,
        mutate: vi.fn(),
      });

      const { result } = renderHook(() => useStudentData("1"));

      expect(result.current.error).toBe("Fehler beim Laden der Schülerdaten.");
      expect(result.current.student).toBeNull();
    });

    it("should return null error when no error", () => {
      mockUseSession.mockReturnValue({
        data: { user: { id: "1", token: "test-token" }, expires: "2099-12-31" },
        status: "authenticated",
        update: vi.fn(),
      });

      mockUseSWRAuth.mockReturnValue({
        data: {
          student: mockStudent,
          hasFullAccess: true,
          supervisors: mockSupervisors,
          myGroups: ["100"],
          myGroupRooms: ["Room A"],
          mySupervisedRooms: ["Room C"],
        },
        error: undefined,
        isLoading: false,
        isValidating: false,
        mutate: vi.fn(),
      });

      const { result } = renderHook(() => useStudentData("1"));

      expect(result.current.error).toBeNull();
    });
  });

  describe("data fetching", () => {
    it("should pass correct SWR key when authenticated", () => {
      mockUseSession.mockReturnValue({
        data: { user: { id: "1", token: "test-token" }, expires: "2099-12-31" },
        status: "authenticated",
        update: vi.fn(),
      });

      mockUseSWRAuth.mockReturnValue({
        data: undefined,
        error: undefined,
        isLoading: true,
        isValidating: false,
        mutate: vi.fn(),
      });

      renderHook(() => useStudentData("123"));

      // Verify SWR was called with correct key
      expect(mockUseSWRAuth).toHaveBeenCalledWith(
        "student-detail-123",
        expect.any(Function),
        expect.objectContaining({
          keepPreviousData: true,
          revalidateOnFocus: false,
        }),
      );
    });

    it("should pass null SWR key when not authenticated", () => {
      mockUseSession.mockReturnValue({
        data: null,
        status: "unauthenticated",
        update: vi.fn(),
      });

      mockUseSWRAuth.mockReturnValue({
        data: undefined,
        error: undefined,
        isLoading: false,
        isValidating: false,
        mutate: vi.fn(),
      });

      renderHook(() => useStudentData("123"));

      // Verify SWR was called with null key (disables fetching)
      expect(mockUseSWRAuth).toHaveBeenCalledWith(
        null,
        expect.any(Function),
        expect.any(Object),
      );
    });

    it("should fetch data in parallel when SWR fetcher is called", async () => {
      const mockMutate = vi.fn();

      mockUseSession.mockReturnValue({
        data: { user: { id: "1", token: "test-token" }, expires: "2099-12-31" },
        status: "authenticated",
        update: vi.fn(),
      });

      // Mock the service responses
      mockStudentService.getStudent.mockResolvedValue({
        id: "1",
        first_name: "John",
        second_name: "Doe",
        name: "John Doe",
        school_class: "5a",
        group_id: "100",
        group_name: "Group A",
        current_location: "Room 101",
        has_full_access: true,
        group_supervisors: mockSupervisors,
      });

      mockUserContextService.getMyEducationalGroups.mockResolvedValue(
        mockGroups,
      );
      mockUserContextService.getMySupervisedGroups.mockResolvedValue(
        mockSupervisedGroups,
      );

      // Capture the fetcher function
      let fetcherFn: (() => Promise<unknown>) | undefined;

      mockUseSWRAuth.mockImplementation((key, fetcher) => {
        if (key !== null && fetcher) {
          fetcherFn = fetcher as () => Promise<unknown>;
        }
        return {
          data: undefined,
          error: undefined,
          isLoading: true,
          isValidating: false,
          mutate: mockMutate,
        };
      });

      renderHook(() => useStudentData("1"));

      // Execute the fetcher
      expect(fetcherFn).toBeDefined();
      if (fetcherFn) {
        await fetcherFn();
      }

      // Verify all services were called
      expect(mockStudentService.getStudent).toHaveBeenCalledWith("1");
      expect(mockUserContextService.getMyEducationalGroups).toHaveBeenCalled();
      expect(mockUserContextService.getMySupervisedGroups).toHaveBeenCalled();
    });

    it("should handle service errors gracefully", async () => {
      mockUseSession.mockReturnValue({
        data: { user: { id: "1", token: "test-token" }, expires: "2099-12-31" },
        status: "authenticated",
        update: vi.fn(),
      });

      mockStudentService.getStudent.mockResolvedValue({
        id: "1",
        first_name: "John",
        second_name: "Doe",
        name: "John Doe",
        school_class: "5a",
        group_id: "100",
        group_name: "Group A",
        current_location: "Room 101",
        has_full_access: true,
        group_supervisors: mockSupervisors,
      });

      // Simulate errors for user context calls (should not crash)
      mockUserContextService.getMyEducationalGroups.mockRejectedValue(
        new Error("Network error"),
      );
      mockUserContextService.getMySupervisedGroups.mockRejectedValue(
        new Error("Network error"),
      );

      let fetcherFn: (() => Promise<unknown>) | undefined;

      mockUseSWRAuth.mockImplementation((key, fetcher) => {
        if (key !== null && fetcher) {
          fetcherFn = fetcher as () => Promise<unknown>;
        }
        return {
          data: undefined,
          error: undefined,
          isLoading: true,
          isValidating: false,
          mutate: vi.fn(),
        };
      });

      renderHook(() => useStudentData("1"));

      // Execute the fetcher
      if (fetcherFn) {
        const result = await fetcherFn();
        const typedResult = result as {
          myGroups: string[];
          mySupervisedRooms: string[];
        };

        // Should return empty arrays when services fail
        expect(typedResult.myGroups).toEqual([]);
        expect(typedResult.mySupervisedRooms).toEqual([]);
      }
    });
  });

  describe("access control", () => {
    it("should include sensitive fields when hasFullAccess is true", () => {
      mockUseSession.mockReturnValue({
        data: { user: { id: "1", token: "test-token" }, expires: "2099-12-31" },
        status: "authenticated",
        update: vi.fn(),
      });

      mockUseSWRAuth.mockReturnValue({
        data: {
          student: {
            ...mockStudent,
            extra_info: "Sensitive info",
            supervisor_notes: "Private notes",
            sick: true,
            sick_since: "2024-01-10T08:00:00Z",
          },
          hasFullAccess: true,
          supervisors: mockSupervisors,
          myGroups: ["100"],
          myGroupRooms: ["Room A"],
          mySupervisedRooms: [],
        },
        error: undefined,
        isLoading: false,
        isValidating: false,
        mutate: vi.fn(),
      });

      const { result } = renderHook(() => useStudentData("1"));

      expect(result.current.hasFullAccess).toBe(true);
      expect(result.current.student?.extra_info).toBe("Sensitive info");
      expect(result.current.student?.supervisor_notes).toBe("Private notes");
      expect(result.current.student?.sick).toBe(true);
    });

    it("should use default value for hasFullAccess when data is undefined", () => {
      mockUseSession.mockReturnValue({
        data: { user: { id: "1", token: "test-token" }, expires: "2099-12-31" },
        status: "authenticated",
        update: vi.fn(),
      });

      mockUseSWRAuth.mockReturnValue({
        data: undefined,
        error: undefined,
        isLoading: false,
        isValidating: false,
        mutate: vi.fn(),
      });

      const { result } = renderHook(() => useStudentData("1"));

      // Default value should be true according to the implementation
      expect(result.current.hasFullAccess).toBe(true);
    });
  });

  describe("refresh functionality", () => {
    it("should call SWR mutate when refreshData is invoked", async () => {
      const mockMutate = vi.fn(() => Promise.resolve());

      mockUseSession.mockReturnValue({
        data: { user: { id: "1", token: "test-token" }, expires: "2099-12-31" },
        status: "authenticated",
        update: vi.fn(),
      });

      mockUseSWRAuth.mockReturnValue({
        data: {
          student: mockStudent,
          hasFullAccess: true,
          supervisors: mockSupervisors,
          myGroups: ["100"],
          myGroupRooms: ["Room A"],
          mySupervisedRooms: ["Room C"],
        },
        error: undefined,
        isLoading: false,
        isValidating: false,
        mutate: mockMutate,
      });

      const { result } = renderHook(() => useStudentData("1"));

      // Call refresh
      result.current.refreshData();

      await waitFor(() => {
        expect(mockMutate).toHaveBeenCalled();
      });
    });

    it("should provide stable refreshData reference", () => {
      const mockMutate = vi.fn();

      mockUseSession.mockReturnValue({
        data: { user: { id: "1", token: "test-token" }, expires: "2099-12-31" },
        status: "authenticated",
        update: vi.fn(),
      });

      mockUseSWRAuth.mockReturnValue({
        data: {
          student: mockStudent,
          hasFullAccess: true,
          supervisors: mockSupervisors,
          myGroups: ["100"],
          myGroupRooms: ["Room A"],
          mySupervisedRooms: ["Room C"],
        },
        error: undefined,
        isLoading: false,
        isValidating: false,
        mutate: mockMutate,
      });

      const { result, rerender } = renderHook(() => useStudentData("1"));

      const firstRefresh = result.current.refreshData;

      // Trigger re-render
      rerender();

      const secondRefresh = result.current.refreshData;

      // Should be the same function reference (memoized with useCallback)
      expect(firstRefresh).toBe(secondRefresh);
    });
  });

  describe("default values", () => {
    it("should return default empty arrays when data is undefined", () => {
      mockUseSession.mockReturnValue({
        data: { user: { id: "1", token: "test-token" }, expires: "2099-12-31" },
        status: "authenticated",
        update: vi.fn(),
      });

      mockUseSWRAuth.mockReturnValue({
        data: undefined,
        error: undefined,
        isLoading: false,
        isValidating: false,
        mutate: vi.fn(),
      });

      const { result } = renderHook(() => useStudentData("1"));

      expect(result.current.student).toBeNull();
      expect(result.current.supervisors).toEqual([]);
      expect(result.current.myGroups).toEqual([]);
      expect(result.current.myGroupRooms).toEqual([]);
      expect(result.current.mySupervisedRooms).toEqual([]);
    });
  });
});

describe("shouldShowCheckoutSection", () => {
  const baseStudent: ExtendedStudent = {
    id: "1",
    first_name: "John",
    second_name: "Doe",
    name: "John Doe",
    school_class: "5a",
    group_id: "100",
    group_name: "Group A",
    current_location: "Room 101",
    bus: false,
    buskind: false,
  };

  describe("group membership checks", () => {
    it("should return true when student is in user's group and checked in", () => {
      const student = {
        ...baseStudent,
        group_id: "100",
        current_location: "Room 101",
      };

      const result = shouldShowCheckoutSection(student, ["100", "101"], []);

      expect(result).toBe(true);
    });

    it("should return true when student is checked in, regardless of group", () => {
      const student = {
        ...baseStudent,
        group_id: "999",
        current_location: "Room 101",
      };

      const result = shouldShowCheckoutSection(student, ["100", "101"], []);

      // New behavior: any checked-in student can be checked out
      expect(result).toBe(true);
    });

    it("should return true when student is checked in without group_id", () => {
      const student = {
        ...baseStudent,
        group_id: undefined,
        current_location: "Room 101",
      };

      const result = shouldShowCheckoutSection(student, ["100", "101"], []);

      // New behavior: any checked-in student can be checked out
      expect(result).toBe(true);
    });
  });

  describe("supervised room checks", () => {
    it("should return true when student is in supervised room and checked in", () => {
      const student = {
        ...baseStudent,
        group_id: "999", // Not in user's groups
        current_location: "Room C - Activity",
      };

      const result = shouldShowCheckoutSection(student, [], ["Room C"]);

      expect(result).toBe(true);
    });

    it("should return true when student is checked in regardless of supervised room", () => {
      const student = {
        ...baseStudent,
        group_id: "999",
        current_location: "Room X",
      };

      const result = shouldShowCheckoutSection(student, [], ["Room C"]);

      // New behavior: any checked-in student can be checked out
      expect(result).toBe(true);
    });

    it("should match partial room names in supervised rooms", () => {
      const student = {
        ...baseStudent,
        current_location: "Room C - Special Activity",
      };

      const result = shouldShowCheckoutSection(student, [], ["Room C"]);

      expect(result).toBe(true);
    });
  });

  describe("check-in status", () => {
    it("should return false when student is at home (Zuhause)", () => {
      const student = {
        ...baseStudent,
        group_id: "100",
        current_location: "Zuhause",
      };

      const result = shouldShowCheckoutSection(student, ["100"], []);

      expect(result).toBe(false);
    });

    it("should return false when current_location is undefined", () => {
      const student = {
        ...baseStudent,
        group_id: "100",
        current_location: "",
      };

      const result = shouldShowCheckoutSection(student, ["100"], []);

      expect(result).toBe(false);
    });

    it("should handle Zuhause variations in location", () => {
      const student = {
        ...baseStudent,
        group_id: "100",
        current_location: "Zuhause - Abgeholt",
      };

      const result = shouldShowCheckoutSection(student, ["100"], []);

      expect(result).toBe(false);
    });
  });

  describe("combined conditions", () => {
    it("should return true when both group and room conditions are met", () => {
      const student = {
        ...baseStudent,
        group_id: "100",
        current_location: "Room A",
      };

      const result = shouldShowCheckoutSection(student, ["100"], ["Room A"]);

      expect(result).toBe(true);
    });

    it("should return true when checked in regardless of group or room", () => {
      const student = {
        ...baseStudent,
        group_id: "999",
        current_location: "Room X",
      };

      const result = shouldShowCheckoutSection(
        student,
        ["100", "101"],
        ["Room A", "Room B"],
      );

      // New behavior: any checked-in student can be checked out
      expect(result).toBe(true);
    });

    it("should return false when in group but not checked in", () => {
      const student = {
        ...baseStudent,
        group_id: "100",
        current_location: "Zuhause",
      };

      const result = shouldShowCheckoutSection(student, ["100"], []);

      expect(result).toBe(false);
    });
  });

  describe("edge cases", () => {
    it("should return true when checked in with empty arrays", () => {
      const student = {
        ...baseStudent,
        group_id: "100",
        current_location: "Room 101",
      };

      const result = shouldShowCheckoutSection(student, [], []);

      // New behavior: any checked-in student can be checked out
      expect(result).toBe(true);
    });

    it("should handle null/undefined location gracefully", () => {
      const student = {
        ...baseStudent,
        group_id: "100",
        current_location: "",
      };

      const result = shouldShowCheckoutSection(student, ["100"], ["Room A"]);

      expect(result).toBe(false);
    });

    it("should be case-sensitive for Zuhause check", () => {
      const student = {
        ...baseStudent,
        group_id: "100",
        current_location: "zuhause", // lowercase
      };

      const result = shouldShowCheckoutSection(student, ["100"], []);

      // Should still return true because lowercase "zuhause" doesn't match "Zuhause"
      expect(result).toBe(true);
    });
  });
});
