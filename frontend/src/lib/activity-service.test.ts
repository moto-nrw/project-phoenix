import { describe, it, expect, vi, beforeEach } from "vitest";
import { activityService } from "./activity-service";
import type {
  Activity,
  ActivityFilter,
  ActivitySchedule,
  Timeframe,
} from "./activity-helpers";

// Mock all activity-api imports
vi.mock("./activity-api", () => ({
  fetchActivities: vi.fn(),
  getActivitySchedules: vi.fn(),
  getActivitySchedule: vi.fn(),
  getAvailableTimeSlots: vi.fn(),
  createActivitySchedule: vi.fn(),
  updateActivitySchedule: vi.fn(),
  deleteActivitySchedule: vi.fn(),
  getTimeframes: vi.fn(),
}));

describe("ActivityService", () => {
  let mockFetchActivities: ReturnType<typeof vi.fn>;
  let mockGetActivitySchedules: ReturnType<typeof vi.fn>;
  let mockGetActivitySchedule: ReturnType<typeof vi.fn>;
  let mockGetAvailableTimeSlots: ReturnType<typeof vi.fn>;
  let mockCreateActivitySchedule: ReturnType<typeof vi.fn>;
  let mockUpdateActivitySchedule: ReturnType<typeof vi.fn>;
  let mockDeleteActivitySchedule: ReturnType<typeof vi.fn>;
  let mockGetTimeframes: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    // Import after mocks are set up
    const activityApiModule = await import("./activity-api");

    mockFetchActivities =
      activityApiModule.fetchActivities as typeof mockFetchActivities;
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
});
