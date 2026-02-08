import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  fetchUnreadAnnouncements,
  markAnnouncementSeen,
  dismissAnnouncement,
} from "./platform-api";

// Use vi.hoisted for mock function referenced in vi.mock
const mockAuthFetch = vi.hoisted(() => vi.fn());

vi.mock("./api-helpers", () => ({
  authFetch: mockAuthFetch,
}));

describe("platform-api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("fetchUnreadAnnouncements", () => {
    it("fetches and maps announcements successfully", async () => {
      const backendData = [
        {
          id: 1,
          title: "System Update",
          content: "New features available",
          type: "release" as const,
          severity: "info" as const,
          version: "1.2.0",
          published_at: "2024-01-15T10:00:00Z",
        },
        {
          id: 2,
          title: "Maintenance",
          content: "Scheduled downtime",
          type: "maintenance" as const,
          severity: "warning" as const,
          version: null,
          published_at: "2024-01-14T09:00:00Z",
        },
      ];

      mockAuthFetch.mockResolvedValueOnce({ data: backendData });

      const result = await fetchUnreadAnnouncements();

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/platform/announcements/unread",
      );
      expect(result).toHaveLength(2);
      expect(result[0]).toEqual({
        id: "1",
        title: "System Update",
        content: "New features available",
        type: "release",
        severity: "info",
        version: "1.2.0",
        publishedAt: "2024-01-15T10:00:00Z",
      });
      expect(result[1]).toEqual({
        id: "2",
        title: "Maintenance",
        content: "Scheduled downtime",
        type: "maintenance",
        severity: "warning",
        version: null,
        publishedAt: "2024-01-14T09:00:00Z",
      });
    });

    it("handles empty announcements array", async () => {
      mockAuthFetch.mockResolvedValueOnce({ data: [] });

      const result = await fetchUnreadAnnouncements();

      expect(result).toEqual([]);
    });

    it("converts backend int64 id to string", async () => {
      mockAuthFetch.mockResolvedValueOnce({
        data: [
          {
            id: 123456789,
            title: "Test",
            content: "Content",
            type: "announcement" as const,
            severity: "info" as const,
            published_at: "2024-01-15T10:00:00Z",
          },
        ],
      });

      const result = await fetchUnreadAnnouncements();

      expect(result[0]?.id).toBe("123456789");
      expect(typeof result[0]?.id).toBe("string");
    });

    it("handles missing version field", async () => {
      mockAuthFetch.mockResolvedValueOnce({
        data: [
          {
            id: 1,
            title: "Announcement",
            content: "Test",
            type: "announcement" as const,
            severity: "info" as const,
            published_at: "2024-01-15T10:00:00Z",
          },
        ],
      });

      const result = await fetchUnreadAnnouncements();

      expect(result[0]?.version).toBeNull();
    });

    it("converts undefined version to null", async () => {
      mockAuthFetch.mockResolvedValueOnce({
        data: [
          {
            id: 1,
            title: "Test",
            content: "Test",
            type: "announcement" as const,
            severity: "info" as const,
            version: undefined,
            published_at: "2024-01-15T10:00:00Z",
          },
        ],
      });

      const result = await fetchUnreadAnnouncements();

      expect(result[0]?.version).toBeNull();
    });

    it("handles all announcement types", async () => {
      mockAuthFetch.mockResolvedValueOnce({
        data: [
          {
            id: 1,
            title: "T1",
            content: "C1",
            type: "announcement" as const,
            severity: "info" as const,
            published_at: "2024-01-15T10:00:00Z",
          },
          {
            id: 2,
            title: "T2",
            content: "C2",
            type: "release" as const,
            severity: "warning" as const,
            published_at: "2024-01-15T10:00:00Z",
          },
          {
            id: 3,
            title: "T3",
            content: "C3",
            type: "maintenance" as const,
            severity: "critical" as const,
            published_at: "2024-01-15T10:00:00Z",
          },
        ],
      });

      const result = await fetchUnreadAnnouncements();

      expect(result[0]?.type).toBe("announcement");
      expect(result[1]?.type).toBe("release");
      expect(result[2]?.type).toBe("maintenance");
    });
  });

  describe("markAnnouncementSeen", () => {
    it("sends POST request with correct ID", async () => {
      mockAuthFetch.mockResolvedValueOnce({});

      await markAnnouncementSeen("123");

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/platform/announcements/123/seen",
        { method: "POST" },
      );
    });

    it("handles string IDs", async () => {
      mockAuthFetch.mockResolvedValueOnce({});

      await markAnnouncementSeen("abc-def-ghi");

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/platform/announcements/abc-def-ghi/seen",
        { method: "POST" },
      );
    });

    it("does not return a value", async () => {
      mockAuthFetch.mockResolvedValueOnce({});

      const result = await markAnnouncementSeen("1");

      expect(result).toBeUndefined();
    });
  });

  describe("dismissAnnouncement", () => {
    it("sends POST request with correct ID", async () => {
      mockAuthFetch.mockResolvedValueOnce({});

      await dismissAnnouncement("456");

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/platform/announcements/456/dismiss",
        { method: "POST" },
      );
    });

    it("handles string IDs", async () => {
      mockAuthFetch.mockResolvedValueOnce({});

      await dismissAnnouncement("xyz-123");

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "/api/platform/announcements/xyz-123/dismiss",
        { method: "POST" },
      );
    });

    it("does not return a value", async () => {
      mockAuthFetch.mockResolvedValueOnce({});

      const result = await dismissAnnouncement("1");

      expect(result).toBeUndefined();
    });
  });
});
