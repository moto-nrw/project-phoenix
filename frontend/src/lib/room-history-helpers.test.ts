import { describe, it, expect } from "vitest";
import {
  mapRoomHistoryEntryResponse,
  mapRoomHistoryEntriesResponse,
  formatDate,
  formatDuration,
  type BackendRoomHistoryEntry,
} from "./room-history-helpers";

describe("room-history-helpers", () => {
  describe("mapRoomHistoryEntryResponse", () => {
    it("should map backend entry to frontend format", () => {
      const backendEntry: BackendRoomHistoryEntry = {
        id: 123,
        room_id: 456,
        date: "2024-01-15T10:30:00Z",
        group_name: "Group A",
        activity_name: "Soccer",
        supervisor_name: "John Doe",
        student_count: 20,
        duration: 90,
      };

      const result = mapRoomHistoryEntryResponse(backendEntry);

      expect(result).toEqual({
        id: "123",
        roomId: "456",
        date: "2024-01-15T10:30:00Z",
        groupName: "Group A",
        activityName: "Soccer",
        supervisorName: "John Doe",
        studentCount: 20,
        duration: 90,
      });
    });

    it("should handle entry without optional fields", () => {
      const backendEntry: BackendRoomHistoryEntry = {
        id: 789,
        room_id: 101,
        date: "2024-01-15T14:00:00Z",
        group_name: "Group B",
        student_count: 15,
        duration: 60,
      };

      const result = mapRoomHistoryEntryResponse(backendEntry);

      expect(result).toEqual({
        id: "789",
        roomId: "101",
        date: "2024-01-15T14:00:00Z",
        groupName: "Group B",
        activityName: undefined,
        supervisorName: undefined,
        studentCount: 15,
        duration: 60,
      });
    });
  });

  describe("mapRoomHistoryEntriesResponse", () => {
    it("should map array of entries", () => {
      const backendEntries: BackendRoomHistoryEntry[] = [
        {
          id: 1,
          room_id: 100,
          date: "2024-01-15T10:00:00Z",
          group_name: "Group A",
          student_count: 10,
          duration: 30,
        },
        {
          id: 2,
          room_id: 200,
          date: "2024-01-15T11:00:00Z",
          group_name: "Group B",
          activity_name: "Art",
          student_count: 15,
          duration: 45,
        },
      ];

      const result = mapRoomHistoryEntriesResponse(backendEntries);

      expect(result).toHaveLength(2);
      expect(result[0]?.id).toBe("1");
      expect(result[1]?.id).toBe("2");
    });

    it("should handle empty array", () => {
      const result = mapRoomHistoryEntriesResponse([]);
      expect(result).toEqual([]);
    });
  });

  describe("formatDate", () => {
    it("should format date to German locale with time", () => {
      const result = formatDate("2024-01-15T10:30:00Z");
      // Format: weekday, dd.MM.yyyy, HH:mm
      expect(result).toMatch(/\w{2,3}\., \d{2}\.\d{2}\.\d{4}, \d{2}:\d{2}/);
    });

    it("should handle different date strings", () => {
      const result = formatDate("2024-12-31T23:59:59Z");
      expect(result).toMatch(/\w{2,3}\., \d{2}\.\d{2}\.\d{4}, \d{2}:\d{2}/);
    });
  });

  describe("formatDuration", () => {
    it("should format minutes only when less than 60", () => {
      expect(formatDuration(0)).toBe("0 Minuten");
      expect(formatDuration(1)).toBe("1 Minuten");
      expect(formatDuration(30)).toBe("30 Minuten");
      expect(formatDuration(59)).toBe("59 Minuten");
    });

    it("should format as '1 Stunde' for exactly 60 minutes", () => {
      expect(formatDuration(60)).toBe("1 Stunde");
    });

    it("should format as 'X Stunden' for multiples of 60", () => {
      expect(formatDuration(120)).toBe("2 Stunden");
      expect(formatDuration(180)).toBe("3 Stunden");
      expect(formatDuration(240)).toBe("4 Stunden");
    });

    it("should format mixed hours and minutes", () => {
      expect(formatDuration(61)).toBe("1 Std. 1 Min.");
      expect(formatDuration(90)).toBe("1 Std. 30 Min.");
      expect(formatDuration(125)).toBe("2 Std. 5 Min.");
      expect(formatDuration(195)).toBe("3 Std. 15 Min.");
    });
  });
});
