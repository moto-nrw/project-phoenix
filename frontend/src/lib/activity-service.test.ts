import { describe, it, expect, vi, beforeEach } from "vitest";
import { activityService } from "./activity-service";
import type {
  Activity,
  CreateActivityRequest,
  UpdateActivityRequest,
  ActivityFilter,
  ActivityCategory,
  ActivityStudent,
  ActivitySchedule,
  Timeframe,
} from "./activity-helpers";

// Mock all activity-api imports
vi.mock("./activity-api", () => ({
  fetchActivities: vi.fn(),
  createActivity: vi.fn(),
  updateActivity: vi.fn(),
  deleteActivity: vi.fn(),
  getActivity: vi.fn(),
  getCategories: vi.fn(),
  getSupervisors: vi.fn(),
  getEnrolledStudents: vi.fn(),
  enrollStudent: vi.fn(),
  unenrollStudent: vi.fn(),
  getActivitySchedules: vi.fn(),
  getActivitySchedule: vi.fn(),
  getAvailableTimeSlots: vi.fn(),
  createActivitySchedule: vi.fn(),
  updateActivitySchedule: vi.fn(),
  deleteActivitySchedule: vi.fn(),
  getActivitySupervisors: vi.fn(),
  getAvailableSupervisors: vi.fn(),
  assignSupervisor: vi.fn(),
  updateSupervisorRole: vi.fn(),
  removeSupervisor: vi.fn(),
  getAvailableStudents: vi.fn(),
  updateGroupEnrollments: vi.fn(),
  getTimeframes: vi.fn(),
}));

describe("ActivityService", () => {
  let mockFetchActivities: ReturnType<typeof vi.fn>;
  let mockCreateActivity: ReturnType<typeof vi.fn>;
  let mockUpdateActivity: ReturnType<typeof vi.fn>;
  let mockDeleteActivity: ReturnType<typeof vi.fn>;
  let mockGetActivity: ReturnType<typeof vi.fn>;
  let mockGetCategories: ReturnType<typeof vi.fn>;
  let mockGetSupervisors: ReturnType<typeof vi.fn>;
  let mockGetEnrolledStudents: ReturnType<typeof vi.fn>;
  let mockEnrollStudent: ReturnType<typeof vi.fn>;
  let mockUnenrollStudent: ReturnType<typeof vi.fn>;
  let mockGetActivitySchedules: ReturnType<typeof vi.fn>;
  let mockGetActivitySchedule: ReturnType<typeof vi.fn>;
  let mockGetAvailableTimeSlots: ReturnType<typeof vi.fn>;
  let mockCreateActivitySchedule: ReturnType<typeof vi.fn>;
  let mockUpdateActivitySchedule: ReturnType<typeof vi.fn>;
  let mockDeleteActivitySchedule: ReturnType<typeof vi.fn>;
  let mockGetActivitySupervisors: ReturnType<typeof vi.fn>;
  let mockGetAvailableSupervisors: ReturnType<typeof vi.fn>;
  let mockAssignSupervisor: ReturnType<typeof vi.fn>;
  let mockUpdateSupervisorRole: ReturnType<typeof vi.fn>;
  let mockRemoveSupervisor: ReturnType<typeof vi.fn>;
  let mockGetAvailableStudents: ReturnType<typeof vi.fn>;
  let mockUpdateGroupEnrollments: ReturnType<typeof vi.fn>;
  let mockGetTimeframes: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    // Import after mocks are set up
    const activityApiModule = await import("./activity-api");

    mockFetchActivities =
      activityApiModule.fetchActivities as typeof mockFetchActivities;
    mockCreateActivity =
      activityApiModule.createActivity as typeof mockCreateActivity;
    mockUpdateActivity =
      activityApiModule.updateActivity as typeof mockUpdateActivity;
    mockDeleteActivity =
      activityApiModule.deleteActivity as typeof mockDeleteActivity;
    mockGetActivity = activityApiModule.getActivity as typeof mockGetActivity;
    mockGetCategories =
      activityApiModule.getCategories as typeof mockGetCategories;
    mockGetSupervisors =
      activityApiModule.getSupervisors as typeof mockGetSupervisors;
    mockGetEnrolledStudents =
      activityApiModule.getEnrolledStudents as typeof mockGetEnrolledStudents;
    mockEnrollStudent =
      activityApiModule.enrollStudent as typeof mockEnrollStudent;
    mockUnenrollStudent =
      activityApiModule.unenrollStudent as typeof mockUnenrollStudent;
    mockGetActivitySchedules =
      activityApiModule.getActivitySchedules as typeof mockGetActivitySchedules;
    mockGetActivitySchedule =
      activityApiModule.getActivitySchedule as typeof mockGetActivitySchedule;
    mockGetAvailableTimeSlots =
      activityApiModule.getAvailableTimeSlots as typeof mockGetAvailableTimeSlots;
    mockCreateActivitySchedule =
      activityApiModule.createActivitySchedule as typeof mockCreateActivitySchedule;
    mockUpdateActivitySchedule =
      activityApiModule.updateActivitySchedule as typeof mockUpdateActivitySchedule;
    mockDeleteActivitySchedule =
      activityApiModule.deleteActivitySchedule as typeof mockDeleteActivitySchedule;
    mockGetActivitySupervisors =
      activityApiModule.getActivitySupervisors as typeof mockGetActivitySupervisors;
    mockGetAvailableSupervisors =
      activityApiModule.getAvailableSupervisors as typeof mockGetAvailableSupervisors;
    mockAssignSupervisor =
      activityApiModule.assignSupervisor as typeof mockAssignSupervisor;
    mockUpdateSupervisorRole =
      activityApiModule.updateSupervisorRole as typeof mockUpdateSupervisorRole;
    mockRemoveSupervisor =
      activityApiModule.removeSupervisor as typeof mockRemoveSupervisor;
    mockGetAvailableStudents =
      activityApiModule.getAvailableStudents as typeof mockGetAvailableStudents;
    mockUpdateGroupEnrollments =
      activityApiModule.updateGroupEnrollments as typeof mockUpdateGroupEnrollments;
    mockGetTimeframes =
      activityApiModule.getTimeframes as typeof mockGetTimeframes;

    vi.clearAllMocks();
  });

  describe("getActivities", () => {
    it("delegates to fetchActivities with filters", async () => {
      const mockActivities: Activity[] = [
        { id: "1", name: "Test Activity" } as Activity,
      ];
      mockFetchActivities.mockResolvedValue(mockActivities);

      const filters: ActivityFilter = { search: "active" };
      const result = await activityService.getActivities(filters);

      expect(mockFetchActivities).toHaveBeenCalledWith(filters);
      expect(result).toBe(mockActivities);
    });

    it("delegates to fetchActivities without filters", async () => {
      const mockActivities: Activity[] = [];
      mockFetchActivities.mockResolvedValue(mockActivities);

      const result = await activityService.getActivities();

      expect(mockFetchActivities).toHaveBeenCalledWith(undefined);
      expect(result).toBe(mockActivities);
    });
  });

  describe("getActivity", () => {
    it("delegates to getActivity API", async () => {
      const mockActivity: Activity = { id: "1", name: "Test" } as Activity;
      mockGetActivity.mockResolvedValue(mockActivity);

      const result = await activityService.getActivity("1");

      expect(mockGetActivity).toHaveBeenCalledWith("1");
      expect(result).toBe(mockActivity);
    });
  });

  describe("createActivity", () => {
    it("delegates to createActivity API", async () => {
      const mockData: CreateActivityRequest = {
        name: "New Activity",
      } as CreateActivityRequest;
      const mockActivity: Activity = {
        id: "1",
        name: "New Activity",
      } as Activity;
      mockCreateActivity.mockResolvedValue(mockActivity);

      const result = await activityService.createActivity(mockData);

      expect(mockCreateActivity).toHaveBeenCalledWith(mockData);
      expect(result).toBe(mockActivity);
    });
  });

  describe("updateActivity", () => {
    it("delegates to updateActivity API", async () => {
      const mockData: UpdateActivityRequest = {
        name: "Updated",
      } as UpdateActivityRequest;
      const mockActivity: Activity = { id: "1", name: "Updated" } as Activity;
      mockUpdateActivity.mockResolvedValue(mockActivity);

      const result = await activityService.updateActivity("1", mockData);

      expect(mockUpdateActivity).toHaveBeenCalledWith("1", mockData);
      expect(result).toBe(mockActivity);
    });
  });

  describe("deleteActivity", () => {
    it("delegates to deleteActivity API", async () => {
      mockDeleteActivity.mockResolvedValue(undefined);

      await activityService.deleteActivity("1");

      expect(mockDeleteActivity).toHaveBeenCalledWith("1");
    });
  });

  describe("getCategories", () => {
    it("delegates to getCategories API", async () => {
      const mockCategories: ActivityCategory[] = [
        { id: "1", name: "Sports" } as ActivityCategory,
      ];
      mockGetCategories.mockResolvedValue(mockCategories);

      const result = await activityService.getCategories();

      expect(mockGetCategories).toHaveBeenCalled();
      expect(result).toBe(mockCategories);
    });
  });

  describe("getSupervisors", () => {
    it("delegates to getSupervisors API", async () => {
      const mockSupervisors = [{ id: "1", name: "John Doe" }];
      mockGetSupervisors.mockResolvedValue(mockSupervisors);

      const result = await activityService.getSupervisors();

      expect(mockGetSupervisors).toHaveBeenCalled();
      expect(result).toBe(mockSupervisors);
    });
  });

  describe("student enrollment methods", () => {
    it("getEnrolledStudents delegates to API", async () => {
      const mockStudents: ActivityStudent[] = [
        { id: "1", name: "Student 1" } as ActivityStudent,
      ];
      mockGetEnrolledStudents.mockResolvedValue(mockStudents);

      const result = await activityService.getEnrolledStudents("activity-1");

      expect(mockGetEnrolledStudents).toHaveBeenCalledWith("activity-1");
      expect(result).toBe(mockStudents);
    });

    it("getAvailableStudents delegates to API with filters", async () => {
      const mockStudents = [{ id: "1", name: "Student 1", school_class: "1A" }];
      mockGetAvailableStudents.mockResolvedValue(mockStudents);

      const filters = { search: "john", group_id: "g1" };
      const result = await activityService.getAvailableStudents(
        "activity-1",
        filters,
      );

      expect(mockGetAvailableStudents).toHaveBeenCalledWith(
        "activity-1",
        filters,
      );
      expect(result).toBe(mockStudents);
    });

    it("enrollStudent delegates to API", async () => {
      const studentData = { studentId: "student-1" };
      mockEnrollStudent.mockResolvedValue({ success: true });

      const result = await activityService.enrollStudent(
        "activity-1",
        studentData,
      );

      expect(mockEnrollStudent).toHaveBeenCalledWith("activity-1", studentData);
      expect(result).toEqual({ success: true });
    });

    it("unenrollStudent delegates to API", async () => {
      mockUnenrollStudent.mockResolvedValue(undefined);

      await activityService.unenrollStudent("activity-1", "student-1");

      expect(mockUnenrollStudent).toHaveBeenCalledWith(
        "activity-1",
        "student-1",
      );
    });

    it("updateGroupEnrollments delegates to API", async () => {
      const data = { student_ids: ["s1", "s2"] };
      mockUpdateGroupEnrollments.mockResolvedValue(true);

      const result = await activityService.updateGroupEnrollments(
        "activity-1",
        data,
      );

      expect(mockUpdateGroupEnrollments).toHaveBeenCalledWith(
        "activity-1",
        data,
      );
      expect(result).toBe(true);
    });
  });

  describe("schedule management methods", () => {
    it("getActivitySchedules delegates to API", async () => {
      const mockSchedules: ActivitySchedule[] = [
        { id: "1", weekday: "monday" } as ActivitySchedule,
      ];
      mockGetActivitySchedules.mockResolvedValue(mockSchedules);

      const result = await activityService.getActivitySchedules("activity-1");

      expect(mockGetActivitySchedules).toHaveBeenCalledWith("activity-1");
      expect(result).toBe(mockSchedules);
    });

    it("getActivitySchedule delegates to API", async () => {
      const mockSchedule: ActivitySchedule = {
        id: "1",
        weekday: "monday",
      } as ActivitySchedule;
      mockGetActivitySchedule.mockResolvedValue(mockSchedule);

      const result = await activityService.getActivitySchedule(
        "activity-1",
        "schedule-1",
      );

      expect(mockGetActivitySchedule).toHaveBeenCalledWith(
        "activity-1",
        "schedule-1",
      );
      expect(result).toBe(mockSchedule);
    });

    it("getAvailableTimeSlots delegates to API with date", async () => {
      const mockSlots = [{ weekday: "monday", timeframe_id: "t1" }];
      mockGetAvailableTimeSlots.mockResolvedValue(mockSlots);

      const result = await activityService.getAvailableTimeSlots(
        "activity-1",
        "2024-01-15",
      );

      expect(mockGetAvailableTimeSlots).toHaveBeenCalledWith(
        "activity-1",
        "2024-01-15",
      );
      expect(result).toBe(mockSlots);
    });

    it("getAvailableTimeSlots delegates to API without date", async () => {
      const mockSlots = [{ weekday: "monday" }];
      mockGetAvailableTimeSlots.mockResolvedValue(mockSlots);

      const result = await activityService.getAvailableTimeSlots("activity-1");

      expect(mockGetAvailableTimeSlots).toHaveBeenCalledWith(
        "activity-1",
        undefined,
      );
      expect(result).toBe(mockSlots);
    });

    it("getTimeframes delegates to API", async () => {
      const mockTimeframes: Timeframe[] = [
        { id: "1", name: "Morning" } as Timeframe,
      ];
      mockGetTimeframes.mockResolvedValue(mockTimeframes);

      const result = await activityService.getTimeframes();

      expect(mockGetTimeframes).toHaveBeenCalled();
      expect(result).toBe(mockTimeframes);
    });

    it("createActivitySchedule delegates to API", async () => {
      const scheduleData: Partial<ActivitySchedule> = { weekday: "monday" };
      const mockSchedule: ActivitySchedule = {
        id: "1",
        weekday: "monday",
      } as ActivitySchedule;
      mockCreateActivitySchedule.mockResolvedValue(mockSchedule);

      const result = await activityService.createActivitySchedule(
        "activity-1",
        scheduleData,
      );

      expect(mockCreateActivitySchedule).toHaveBeenCalledWith(
        "activity-1",
        scheduleData,
      );
      expect(result).toBe(mockSchedule);
    });

    it("updateActivitySchedule delegates to API", async () => {
      const scheduleData: Partial<ActivitySchedule> = { weekday: "tuesday" };
      const mockSchedule: ActivitySchedule = {
        id: "1",
        weekday: "tuesday",
      } as ActivitySchedule;
      mockUpdateActivitySchedule.mockResolvedValue(mockSchedule);

      const result = await activityService.updateActivitySchedule(
        "activity-1",
        "schedule-1",
        scheduleData,
      );

      expect(mockUpdateActivitySchedule).toHaveBeenCalledWith(
        "activity-1",
        "schedule-1",
        scheduleData,
      );
      expect(result).toBe(mockSchedule);
    });

    it("deleteActivitySchedule delegates to API", async () => {
      mockDeleteActivitySchedule.mockResolvedValue(true);

      const result = await activityService.deleteActivitySchedule(
        "activity-1",
        "schedule-1",
      );

      expect(mockDeleteActivitySchedule).toHaveBeenCalledWith(
        "activity-1",
        "schedule-1",
      );
      expect(result).toBe(true);
    });
  });

  describe("alias methods for compatibility", () => {
    it("deleteTimeSlot delegates to deleteActivitySchedule", async () => {
      mockDeleteActivitySchedule.mockResolvedValue(true);

      const result = await activityService.deleteTimeSlot(
        "activity-1",
        "time-1",
      );

      expect(mockDeleteActivitySchedule).toHaveBeenCalledWith(
        "activity-1",
        "time-1",
      );
      expect(result).toBe(true);
    });

    it("addTimeSlot formats data and calls createActivitySchedule", async () => {
      const timeData = {
        weekday: "Monday",
        startTime: "09:00",
        endTime: "10:00",
      };
      const mockSchedule: ActivitySchedule = {
        id: "1",
        weekday: "monday",
      } as ActivitySchedule;
      mockCreateActivitySchedule.mockResolvedValue(mockSchedule);

      const result = await activityService.addTimeSlot("activity-1", timeData);

      expect(mockCreateActivitySchedule).toHaveBeenCalledWith("activity-1", {
        activity_id: "activity-1",
        weekday: "monday",
      });
      expect(result).toBe(mockSchedule);
    });
  });

  describe("supervisor assignment methods", () => {
    it("getActivitySupervisors delegates to API", async () => {
      const mockSupervisors = [
        { id: "1", staff_id: "s1", is_primary: true, name: "John Doe" },
      ];
      mockGetActivitySupervisors.mockResolvedValue(mockSupervisors);

      const result = await activityService.getActivitySupervisors("activity-1");

      expect(mockGetActivitySupervisors).toHaveBeenCalledWith("activity-1");
      expect(result).toBe(mockSupervisors);
    });

    it("getAvailableSupervisors delegates to API", async () => {
      const mockSupervisors = [{ id: "1", name: "Jane Doe" }];
      mockGetAvailableSupervisors.mockResolvedValue(mockSupervisors);

      const result =
        await activityService.getAvailableSupervisors("activity-1");

      expect(mockGetAvailableSupervisors).toHaveBeenCalledWith("activity-1");
      expect(result).toBe(mockSupervisors);
    });

    it("assignSupervisor delegates to API", async () => {
      const supervisorData = { staff_id: "s1", is_primary: true };
      mockAssignSupervisor.mockResolvedValue(true);

      const result = await activityService.assignSupervisor(
        "activity-1",
        supervisorData,
      );

      expect(mockAssignSupervisor).toHaveBeenCalledWith(
        "activity-1",
        supervisorData,
      );
      expect(result).toBe(true);
    });

    it("updateSupervisorRole delegates to API", async () => {
      const roleData = { is_primary: false };
      mockUpdateSupervisorRole.mockResolvedValue(true);

      const result = await activityService.updateSupervisorRole(
        "activity-1",
        "supervisor-1",
        roleData,
      );

      expect(mockUpdateSupervisorRole).toHaveBeenCalledWith(
        "activity-1",
        "supervisor-1",
        roleData,
      );
      expect(result).toBe(true);
    });

    it("removeSupervisor delegates to API", async () => {
      mockRemoveSupervisor.mockResolvedValue(true);

      const result = await activityService.removeSupervisor(
        "activity-1",
        "supervisor-1",
      );

      expect(mockRemoveSupervisor).toHaveBeenCalledWith(
        "activity-1",
        "supervisor-1",
      );
      expect(result).toBe(true);
    });
  });
});
