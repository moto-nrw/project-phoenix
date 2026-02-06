import { describe, it, expect, vi, beforeEach } from "vitest";

const { mockOperatorFetch } = vi.hoisted(() => ({
  mockOperatorFetch: vi.fn(),
}));

vi.mock("./api-helpers", () => ({
  operatorFetch: mockOperatorFetch,
}));

import { operatorAnnouncementsService } from "./announcements-api";
import type {
  BackendAnnouncement,
  CreateAnnouncementRequest,
  UpdateAnnouncementRequest,
  AnnouncementStats,
  BackendAnnouncementViewDetail,
} from "./announcements-helpers";

describe("OperatorAnnouncementsService", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("fetchAll", () => {
    it("calls correct endpoint with includeInactive true", async () => {
      const mockData: BackendAnnouncement[] = [];
      mockOperatorFetch.mockResolvedValue(mockData);

      await operatorAnnouncementsService.fetchAll(true);

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/announcements?include_inactive=true",
      );
    });

    it("calls correct endpoint with includeInactive false", async () => {
      const mockData: BackendAnnouncement[] = [];
      mockOperatorFetch.mockResolvedValue(mockData);

      await operatorAnnouncementsService.fetchAll(false);

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/announcements?include_inactive=false",
      );
    });

    it("defaults to includeInactive true", async () => {
      const mockData: BackendAnnouncement[] = [];
      mockOperatorFetch.mockResolvedValue(mockData);

      await operatorAnnouncementsService.fetchAll();

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/announcements?include_inactive=true",
      );
    });

    it("maps response data correctly", async () => {
      const mockData: BackendAnnouncement[] = [
        {
          id: 1,
          title: "Test",
          content: "Content",
          type: "announcement",
          severity: "info",
          version: null,
          active: true,
          published_at: null,
          expires_at: null,
          target_roles: [],
          created_by: 1,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
          status: "draft",
        },
      ];
      mockOperatorFetch.mockResolvedValue(mockData);

      const result = await operatorAnnouncementsService.fetchAll();

      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("1");
      expect(result[0]?.title).toBe("Test");
    });
  });

  describe("fetchById", () => {
    it("calls correct endpoint with ID", async () => {
      const mockData: BackendAnnouncement = {
        id: 42,
        title: "Test Announcement",
        content: "Test content",
        type: "release",
        severity: "warning",
        version: "2.0.0",
        active: true,
        published_at: "2024-02-01T00:00:00Z",
        expires_at: null,
        target_roles: ["admin"],
        created_by: 5,
        created_at: "2024-01-30T00:00:00Z",
        updated_at: "2024-02-01T00:00:00Z",
        status: "published",
      };
      mockOperatorFetch.mockResolvedValue(mockData);

      await operatorAnnouncementsService.fetchById("42");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/announcements/42",
      );
    });

    it("maps single announcement correctly", async () => {
      const mockData: BackendAnnouncement = {
        id: 99,
        title: "Maintenance",
        content: "System down",
        type: "maintenance",
        severity: "critical",
        version: null,
        active: false,
        published_at: null,
        expires_at: null,
        target_roles: ["user"],
        created_by: 1,
        created_at: "2024-03-01T00:00:00Z",
        updated_at: "2024-03-01T00:00:00Z",
        status: "draft",
      };
      mockOperatorFetch.mockResolvedValue(mockData);

      const result = await operatorAnnouncementsService.fetchById("99");

      expect(result.id).toBe("99");
      expect(result.title).toBe("Maintenance");
      expect(result.type).toBe("maintenance");
    });
  });

  describe("create", () => {
    it("calls POST endpoint with create data", async () => {
      const createData: CreateAnnouncementRequest = {
        title: "New Announcement",
        content: "Hello world",
        type: "announcement",
        severity: "info",
        target_roles: ["admin", "user"],
      };

      const mockResponse: BackendAnnouncement = {
        id: 100,
        title: "New Announcement",
        content: "Hello world",
        type: "announcement",
        severity: "info",
        version: null,
        active: false,
        published_at: null,
        expires_at: null,
        target_roles: ["admin", "user"],
        created_by: 1,
        created_at: "2024-04-01T00:00:00Z",
        updated_at: "2024-04-01T00:00:00Z",
        status: "draft",
      };
      mockOperatorFetch.mockResolvedValue(mockResponse);

      const result = await operatorAnnouncementsService.create(createData);

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/announcements",
        { method: "POST", body: createData },
      );
      expect(result.id).toBe("100");
      expect(result.title).toBe("New Announcement");
    });
  });

  describe("update", () => {
    it("calls PUT endpoint with update data", async () => {
      const updateData: UpdateAnnouncementRequest = {
        title: "Updated Title",
        content: "Updated content",
        type: "release",
        severity: "warning",
        version: "3.0.0",
        expires_at: "2024-12-31T23:59:59Z",
        target_roles: ["admin"],
      };

      const mockResponse: BackendAnnouncement = {
        id: 42,
        title: "Updated Title",
        content: "Updated content",
        type: "release",
        severity: "warning",
        version: "3.0.0",
        active: true,
        published_at: "2024-01-01T00:00:00Z",
        expires_at: "2024-12-31T23:59:59Z",
        target_roles: ["admin"],
        created_by: 1,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-05-01T00:00:00Z",
        status: "published",
      };
      mockOperatorFetch.mockResolvedValue(mockResponse);

      const result = await operatorAnnouncementsService.update(
        "42",
        updateData,
      );

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/announcements/42",
        { method: "PUT", body: updateData },
      );
      expect(result.id).toBe("42");
      expect(result.title).toBe("Updated Title");
    });
  });

  describe("delete", () => {
    it("calls DELETE endpoint with ID", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorAnnouncementsService.delete("42");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/announcements/42",
        { method: "DELETE" },
      );
    });
  });

  describe("publish", () => {
    it("calls POST endpoint to publish", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorAnnouncementsService.publish("42");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/announcements/42/publish",
        { method: "POST" },
      );
    });
  });

  describe("fetchStats", () => {
    it("calls correct endpoint and returns stats", async () => {
      const mockStats: AnnouncementStats = {
        announcement_id: 42,
        target_count: 100,
        seen_count: 80,
        dismissed_count: 50,
      };
      mockOperatorFetch.mockResolvedValue(mockStats);

      const result = await operatorAnnouncementsService.fetchStats("42");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/announcements/42/stats",
      );
      expect(result.announcement_id).toBe(42);
      expect(result.seen_count).toBe(80);
      expect(result.dismissed_count).toBe(50);
    });
  });

  describe("fetchViewDetails", () => {
    it("calls correct endpoint and maps view details", async () => {
      const mockDetails: BackendAnnouncementViewDetail[] = [
        {
          user_id: 1,
          user_name: "Alice",
          seen_at: "2024-01-01T10:00:00Z",
          dismissed: true,
        },
        {
          user_id: 2,
          user_name: "Bob",
          seen_at: "2024-01-01T11:00:00Z",
          dismissed: false,
        },
      ];
      mockOperatorFetch.mockResolvedValue(mockDetails);

      const result = await operatorAnnouncementsService.fetchViewDetails("42");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/announcements/42/views",
      );
      expect(result).toHaveLength(2);
      expect(result[0]?.userId).toBe("1");
      expect(result[0]?.userName).toBe("Alice");
      expect(result[1]?.userId).toBe("2");
      expect(result[1]?.dismissed).toBe(false);
    });

    it("returns empty array when no views", async () => {
      mockOperatorFetch.mockResolvedValue([]);

      const result = await operatorAnnouncementsService.fetchViewDetails("42");

      expect(result).toEqual([]);
    });
  });
});
