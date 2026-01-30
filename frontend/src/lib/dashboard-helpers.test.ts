import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  mapDashboardAnalyticsResponse,
  formatRecentActivityTime,
  getActivityTypeIcon,
  getActivityStatusColor,
  getGroupStatusColor,
  type DashboardAnalyticsResponse,
} from "./dashboard-helpers";

describe("dashboard-helpers", () => {
  describe("mapDashboardAnalyticsResponse", () => {
    it("should map complete backend response to frontend format", () => {
      const backendResponse: DashboardAnalyticsResponse = {
        students_present: 120,
        students_in_transit: 15,
        students_on_playground: 30,
        students_in_rooms: 75,
        active_activities: 8,
        free_rooms: 5,
        total_rooms: 20,
        capacity_utilization: 0.75,
        activity_categories: 4,
        active_ogs_groups: 6,
        students_in_group_rooms: 50,
        supervisors_today: 12,
        students_in_home_room: 25,
        recent_activity: [
          {
            type: "check_in",
            group_name: "Group A",
            room_name: "Room 101",
            count: 5,
            timestamp: "2024-01-15T10:30:00Z",
          },
          {
            type: "group_start",
            group_name: "Group B",
            room_name: "Room 102",
            count: 3,
            timestamp: "2024-01-15T11:00:00Z",
          },
        ],
        current_activities: [
          {
            name: "Soccer",
            category: "Sports",
            participants: 20,
            max_capacity: 25,
            status: "active",
          },
          {
            name: "Art Class",
            category: "Creative",
            participants: 15,
            max_capacity: 15,
            status: "full",
          },
        ],
        active_groups_summary: [
          {
            name: "Group A",
            type: "ogs_group",
            student_count: 25,
            location: "Room 101",
            status: "active",
          },
          {
            name: "Group B",
            type: "activity",
            student_count: 20,
            location: "Gym",
            status: "preparing",
          },
        ],
        last_updated: "2024-01-15T12:00:00Z",
      };

      const result = mapDashboardAnalyticsResponse(backendResponse);

      expect(result.studentsPresent).toBe(120);
      expect(result.studentsInTransit).toBe(15);
      expect(result.studentsOnPlayground).toBe(30);
      expect(result.studentsInRooms).toBe(75);
      expect(result.activeActivities).toBe(8);
      expect(result.freeRooms).toBe(5);
      expect(result.totalRooms).toBe(20);
      expect(result.capacityUtilization).toBe(0.75);
      expect(result.activityCategories).toBe(4);
      expect(result.activeOGSGroups).toBe(6);
      expect(result.studentsInGroupRooms).toBe(50);
      expect(result.supervisorsToday).toBe(12);
      expect(result.studentsInHomeRoom).toBe(25);

      expect(result.recentActivity).toHaveLength(2);
      expect(result.recentActivity[0]).toEqual({
        type: "check_in",
        groupName: "Group A",
        roomName: "Room 101",
        count: 5,
        timestamp: new Date("2024-01-15T10:30:00Z"),
      });

      expect(result.currentActivities).toHaveLength(2);
      expect(result.currentActivities[0]).toEqual({
        name: "Soccer",
        category: "Sports",
        participants: 20,
        maxCapacity: 25,
        status: "active",
      });

      expect(result.activeGroupsSummary).toHaveLength(2);
      expect(result.activeGroupsSummary[0]).toEqual({
        name: "Group A",
        type: "ogs_group",
        studentCount: 25,
        location: "Room 101",
        status: "active",
      });

      expect(result.lastUpdated).toEqual(new Date("2024-01-15T12:00:00Z"));
    });

    it("should handle empty arrays", () => {
      const backendResponse: DashboardAnalyticsResponse = {
        students_present: 0,
        students_in_transit: 0,
        students_on_playground: 0,
        students_in_rooms: 0,
        active_activities: 0,
        free_rooms: 0,
        total_rooms: 0,
        capacity_utilization: 0,
        activity_categories: 0,
        active_ogs_groups: 0,
        students_in_group_rooms: 0,
        supervisors_today: 0,
        students_in_home_room: 0,
        recent_activity: [],
        current_activities: [],
        active_groups_summary: [],
        last_updated: "2024-01-15T12:00:00Z",
      };

      const result = mapDashboardAnalyticsResponse(backendResponse);

      expect(result.recentActivity).toEqual([]);
      expect(result.currentActivities).toEqual([]);
      expect(result.activeGroupsSummary).toEqual([]);
    });
  });

  describe("formatRecentActivityTime", () => {
    beforeEach(() => {
      vi.useFakeTimers();
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it("should return 'gerade eben' for timestamps less than 1 minute ago", () => {
      const now = new Date("2024-01-15T12:00:00Z");
      vi.setSystemTime(now);

      const timestamp = new Date("2024-01-15T11:59:30Z");
      expect(formatRecentActivityTime(timestamp)).toBe("gerade eben");
    });

    it("should return 'vor X min' for timestamps less than 60 minutes ago", () => {
      const now = new Date("2024-01-15T12:00:00Z");
      vi.setSystemTime(now);

      const timestamp = new Date("2024-01-15T11:30:00Z");
      expect(formatRecentActivityTime(timestamp)).toBe("vor 30 min");
    });

    it("should return 'vor 1 min' for 1 minute ago", () => {
      const now = new Date("2024-01-15T12:00:00Z");
      vi.setSystemTime(now);

      const timestamp = new Date("2024-01-15T11:59:00Z");
      expect(formatRecentActivityTime(timestamp)).toBe("vor 1 min");
    });

    it("should return 'vor 59 min' for 59 minutes ago", () => {
      const now = new Date("2024-01-15T12:00:00Z");
      vi.setSystemTime(now);

      const timestamp = new Date("2024-01-15T11:01:00Z");
      expect(formatRecentActivityTime(timestamp)).toBe("vor 59 min");
    });

    it("should return 'vor X Std.' for timestamps less than 24 hours ago", () => {
      const now = new Date("2024-01-15T12:00:00Z");
      vi.setSystemTime(now);

      const timestamp = new Date("2024-01-15T08:00:00Z");
      expect(formatRecentActivityTime(timestamp)).toBe("vor 4 Std.");
    });

    it("should return 'vor 1 Std.' for 1 hour ago", () => {
      const now = new Date("2024-01-15T12:00:00Z");
      vi.setSystemTime(now);

      const timestamp = new Date("2024-01-15T11:00:00Z");
      expect(formatRecentActivityTime(timestamp)).toBe("vor 1 Std.");
    });

    it("should return formatted date for timestamps 24+ hours ago", () => {
      const now = new Date("2024-01-16T12:00:00Z");
      vi.setSystemTime(now);

      const timestamp = new Date("2024-01-14T12:00:00Z");
      const result = formatRecentActivityTime(timestamp);
      // Should match German date format dd.MM.yyyy
      expect(result).toMatch(/\d{2}\.\d{2}\.\d{4}/);
    });

    it("should handle string timestamp input", () => {
      const now = new Date("2024-01-15T12:00:00Z");
      vi.setSystemTime(now);

      const timestamp = "2024-01-15T11:30:00Z";
      expect(formatRecentActivityTime(timestamp)).toBe("vor 30 min");
    });

    it("should return 'Unbekannt' for invalid date", () => {
      expect(formatRecentActivityTime("invalid-date")).toBe("Unbekannt");
    });

    it("should return 'Unbekannt' for invalid Date object", () => {
      expect(formatRecentActivityTime(new Date("invalid"))).toBe("Unbekannt");
    });
  });

  describe("getActivityTypeIcon", () => {
    it("should return arrow icon for check_in", () => {
      expect(getActivityTypeIcon("check_in")).toBe("➡️");
    });

    it("should return arrow icon for check_out", () => {
      expect(getActivityTypeIcon("check_out")).toBe("⬅️");
    });

    it("should return play icon for group_start", () => {
      expect(getActivityTypeIcon("group_start")).toBe("▶️");
    });

    it("should return stop icon for group_end", () => {
      expect(getActivityTypeIcon("group_end")).toBe("⏹️");
    });

    it("should return default icon for unknown type", () => {
      expect(getActivityTypeIcon("unknown" as never)).toBe("📍");
    });
  });

  describe("getActivityStatusColor", () => {
    it("should return green for active status", () => {
      expect(getActivityStatusColor("active")).toBe("bg-green-500");
    });

    it("should return amber for full status", () => {
      expect(getActivityStatusColor("full")).toBe("bg-amber-500");
    });

    it("should return orange for ending_soon status", () => {
      expect(getActivityStatusColor("ending_soon")).toBe("bg-orange-500");
    });

    it("should return gray for unknown status", () => {
      expect(getActivityStatusColor("unknown" as never)).toBe("bg-gray-500");
    });
  });

  describe("getGroupStatusColor", () => {
    it("should return green for active status", () => {
      expect(getGroupStatusColor("active")).toBe("bg-green-500");
    });

    it("should return amber for transitioning status", () => {
      expect(getGroupStatusColor("transitioning")).toBe("bg-amber-500");
    });

    it("should return blue for preparing status", () => {
      expect(getGroupStatusColor("preparing")).toBe("bg-blue-500");
    });

    it("should return gray for unknown status", () => {
      expect(getGroupStatusColor("unknown" as never)).toBe("bg-gray-500");
    });
  });
});
