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
  getTimeframes: vi.fn(),
}));

describe("ActivityService", () => {
  let mockFetchActivities: ReturnType<typeof vi.fn>;
  let mockGetActivitySchedules: ReturnType<typeof vi.fn>;
  let mockGetActivitySchedule: ReturnType<typeof vi.fn>;
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

    it("getTimeframes delegates to API", async () => {
      const mockTimeframes: Timeframe[] = [
        { id: "1", name: "Morning" } as Timeframe,
      ];
      mockGetTimeframes.mockResolvedValue(mockTimeframes);

      const result = await activityService.getTimeframes();

      expect(mockGetTimeframes).toHaveBeenCalled();
      expect(result).toBe(mockTimeframes);
    });
  });
});
