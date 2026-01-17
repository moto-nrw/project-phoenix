// activity-api.test.ts
// Comprehensive tests for activity API service

/* eslint-disable @typescript-eslint/unbound-method */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type {
  BackendActivity,
  BackendActivityCategory,
  BackendActivitySchedule,
  BackendTimeframe,
  BackendSupervisor,
  BackendStudentEnrollment,
  Activity,
  CreateActivityRequest,
  UpdateActivityRequest,
} from "./activity-helpers";

// Mock dependencies
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(),
}));

vi.mock("./api", () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

vi.mock("./auth-api", () => ({
  handleAuthFailure: vi.fn(),
}));

vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

// Import after mocks
import { getSession } from "next-auth/react";
import api from "./api";
import { handleAuthFailure } from "./auth-api";
import * as activityApi from "./activity-api";

const mockedGetSession = vi.mocked(getSession);
const mockedApiGet = vi.mocked(api.get);
const mockedApiPost = vi.mocked(api.post);
const mockedApiPut = vi.mocked(api.put);
const mockedApiDelete = vi.mocked(api.delete);
const mockedHandleAuthFailure = vi.mocked(handleAuthFailure);

// Sample test data
const sampleBackendActivity: BackendActivity = {
  id: 1,
  name: "Basketball AG",
  max_participants: 20,
  is_open: true,
  category_id: 1,
  supervisor_id: 10,
  enrollment_count: 15,
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
};

const sampleBackendCategory: BackendActivityCategory = {
  id: 1,
  name: "Sport",
  description: "Sports activities",
  color: "#3b82f6",
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
};

const sampleBackendSchedule: BackendActivitySchedule = {
  id: 1,
  weekday: 1,
  timeframe_id: 1,
  activity_group_id: 1,
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
};

const sampleBackendTimeframe: BackendTimeframe = {
  id: 1,
  name: "Morning",
  start_time: "08:00",
  end_time: "12:00",
  description: "Morning slot",
};

const sampleBackendSupervisor: BackendSupervisor = {
  id: 1,
  first_name: "John",
  last_name: "Doe",
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
};

const sampleBackendEnrollment: BackendStudentEnrollment = {
  id: 1,
  first_name: "Jane",
  last_name: "Smith",
  school_class: "5a",
  current_location: "room_101",
};

describe("activity-api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset window for browser detection
    vi.stubGlobal("window", undefined);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  describe("fetchActivities", () => {
    it("fetches activities in server context using axios", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: [sampleBackendActivity] },
      });

      const result = await activityApi.fetchActivities();

      expect(mockedApiGet).toHaveBeenCalledWith(
        "http://localhost:8080/api/activities",
      );
      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("1");
      expect(result[0]?.name).toBe("Basketball AG");
    });

    it("fetches activities with filters in server context", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: [sampleBackendActivity] },
      });

      await activityApi.fetchActivities({
        search: "basketball",
        category_id: "1",
        is_open_ags: true,
      });

      expect(mockedApiGet).toHaveBeenCalledWith(
        expect.stringContaining("search=basketball"),
      );
      expect(mockedApiGet).toHaveBeenCalledWith(
        expect.stringContaining("category_id=1"),
      );
      expect(mockedApiGet).toHaveBeenCalledWith(
        expect.stringContaining("is_open_ags=true"),
      );
    });

    it("fetches activities in browser context using fetch", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const mockResponse = {
        ok: true,
        json: async () => ({ data: [sampleBackendActivity] }),
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.fetchActivities();

      expect(result).toHaveLength(1);
      expect(result[0]?.name).toBe("Basketball AG");
    });

    it("handles nested data.data structure in browser context", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const mockResponse = {
        ok: true,
        json: async () => ({ data: { data: [sampleBackendActivity] } }),
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.fetchActivities();

      expect(result).toHaveLength(1);
    });

    it("handles direct array response in browser context", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const frontendActivity: Activity = {
        id: "1",
        name: "Test Activity",
        max_participant: 20,
        is_open_ags: true,
        supervisor_id: "1",
        ag_category_id: "1",
        created_at: new Date(),
        updated_at: new Date(),
      };

      const mockResponse = {
        ok: true,
        json: async () => [frontendActivity],
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.fetchActivities();

      expect(result).toHaveLength(1);
    });

    it("returns empty array for empty response", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: [] },
      });

      const result = await activityApi.fetchActivities();

      expect(result).toEqual([]);
    });

    it("throws error on API failure in browser context", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce(null);

      const mockResponse = {
        ok: false,
        status: 401,
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      await expect(activityApi.fetchActivities()).rejects.toThrow(
        "API error: 401",
      );
    });
  });

  describe("getActivity / fetchActivity", () => {
    it("fetches a single activity in server context", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: sampleBackendActivity },
      });

      const result = await activityApi.getActivity("1");

      expect(mockedApiGet).toHaveBeenCalledWith(
        "http://localhost:8080/api/activities/1",
      );
      expect(result.id).toBe("1");
      expect(result.name).toBe("Basketball AG");
    });

    it("fetchActivity is an alias for getActivity", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: sampleBackendActivity },
      });

      const result = await activityApi.fetchActivity("1");

      expect(result.id).toBe("1");
    });

    it("fetches activity in browser context with wrapped response", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const frontendActivity: Activity = {
        id: "1",
        name: "Test Activity",
        max_participant: 20,
        is_open_ags: true,
        supervisor_id: "1",
        ag_category_id: "1",
        created_at: new Date(),
        updated_at: new Date(),
      };

      const mockResponse = {
        ok: true,
        json: async () => ({ data: frontendActivity }),
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.getActivity("1");

      expect(result.id).toBe("1");
    });

    it("fetches activity in browser context with direct response", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const frontendActivity: Activity = {
        id: "1",
        name: "Test Activity",
        max_participant: 20,
        is_open_ags: true,
        supervisor_id: "1",
        ag_category_id: "1",
        created_at: new Date(),
        updated_at: new Date(),
      };

      const mockResponse = {
        ok: true,
        json: async () => frontendActivity,
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.getActivity("1");

      expect(result.name).toBe("Test Activity");
    });
  });

  describe("createActivity", () => {
    const createRequest: CreateActivityRequest = {
      name: "New Activity",
      max_participants: 15,
      is_open: true,
      category_id: 1,
      supervisor_ids: [10],
    };

    it("creates activity in server context", async () => {
      mockedApiPost.mockResolvedValueOnce({
        data: { data: sampleBackendActivity },
      });

      const result = await activityApi.createActivity(createRequest);

      expect(mockedApiPost).toHaveBeenCalledWith(
        "http://localhost:8080/api/activities",
        createRequest,
      );
      expect(result.id).toBe("1");
    });

    it("creates activity in browser context", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const mockResponse = {
        ok: true,
        json: async () => ({ data: sampleBackendActivity }),
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.createActivity(createRequest);

      expect(result).toBeDefined();
    });

    it("returns fallback activity on parsing failure", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const mockResponse = {
        ok: true,
        json: async () => {
          throw new Error("Parse error");
        },
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.createActivity(createRequest);

      expect(result.name).toBe("New Activity");
      expect(result.id).toBe("0");
    });

    it("extracts ID from response with partial data", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const mockResponse = {
        ok: true,
        json: async () => ({ data: { id: 42 } }),
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.createActivity(createRequest);

      expect(result.id).toBe("42");
    });
  });

  describe("updateActivity", () => {
    const updateRequest: UpdateActivityRequest = {
      name: "Updated Activity",
      max_participants: 25,
      is_open: false,
      category_id: 2,
      supervisor_ids: [11],
    };

    it("updates activity in server context", async () => {
      mockedApiPut.mockResolvedValueOnce({
        data: { data: { ...sampleBackendActivity, name: "Updated Activity" } },
      });

      const result = await activityApi.updateActivity("1", updateRequest);

      expect(mockedApiPut).toHaveBeenCalled();
      expect(result.name).toBe("Updated Activity");
    });

    it("updates activity in browser context", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const updatedActivity: Activity = {
        id: "1",
        name: "Updated Activity",
        max_participant: 25,
        is_open_ags: false,
        supervisor_id: "11",
        ag_category_id: "2",
        created_at: new Date(),
        updated_at: new Date(),
      };

      const mockResponse = {
        ok: true,
        json: async () => ({ data: updatedActivity }),
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.updateActivity("1", updateRequest);

      expect(result.name).toBe("Updated Activity");
    });
  });

  describe("deleteActivity", () => {
    it("deletes activity in server context", async () => {
      mockedApiDelete.mockResolvedValueOnce({});

      await activityApi.deleteActivity("1");

      expect(mockedApiDelete).toHaveBeenCalledWith(
        "http://localhost:8080/api/activities/1",
      );
    });

    it("deletes activity in browser context", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const mockResponse = { ok: true };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      await activityApi.deleteActivity("1");

      expect(globalThis.fetch).toHaveBeenCalledWith(
        "/api/activities/1",
        expect.objectContaining({ method: "DELETE" }),
      );
    });
  });

  describe("getCategories", () => {
    it("fetches categories in server context", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: [sampleBackendCategory] },
      });

      const result = await activityApi.getCategories();

      expect(result).toHaveLength(1);
      expect(result[0]?.name).toBe("Sport");
    });

    it("fetches categories in browser context with wrapped response", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const frontendCategory = {
        id: "1",
        name: "Sport",
        description: "Sports",
        color: "#3b82f6",
        created_at: new Date(),
        updated_at: new Date(),
      };

      const mockResponse = {
        ok: true,
        json: async () => ({ data: [frontendCategory] }),
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.getCategories();

      expect(result).toHaveLength(1);
    });

    it("fetches categories in browser context with direct response", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const frontendCategory = {
        id: "1",
        name: "Sport",
        description: "Sports",
        color: "#3b82f6",
        created_at: new Date(),
        updated_at: new Date(),
      };

      const mockResponse = {
        ok: true,
        json: async () => [frontendCategory],
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.getCategories();

      expect(result).toHaveLength(1);
    });

    it("returns empty array for non-array response", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: null },
      });

      const result = await activityApi.getCategories();

      expect(result).toEqual([]);
    });
  });

  describe("getSupervisors", () => {
    it("fetches supervisors in server context", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: [sampleBackendSupervisor] },
      });

      const result = await activityApi.getSupervisors();

      expect(result).toHaveLength(1);
      expect(result[0]?.name).toBe("John Doe");
    });

    it("returns empty array on error", async () => {
      mockedApiGet.mockRejectedValueOnce(new Error("Network error"));

      const result = await activityApi.getSupervisors();

      expect(result).toEqual([]);
    });
  });

  describe("getEnrolledStudents", () => {
    it("fetches enrolled students in server context", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: [sampleBackendEnrollment] },
      });

      const result = await activityApi.getEnrolledStudents("1");

      expect(result).toHaveLength(1);
      expect(result[0]?.name).toBe("Jane Smith");
    });

    it("fetches enrolled students in browser context", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const mockResponse = {
        ok: true,
        json: async () => ({ data: [sampleBackendEnrollment] }),
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.getEnrolledStudents("1");

      expect(result).toHaveLength(1);
    });

    it("handles direct array response", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const mockResponse = {
        ok: true,
        json: async () => [sampleBackendEnrollment],
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.getEnrolledStudents("1");

      expect(result).toHaveLength(1);
    });
  });

  describe("enrollStudent", () => {
    it("enrolls student in server context", async () => {
      mockedApiPost.mockResolvedValueOnce({});

      const result = await activityApi.enrollStudent("1", { studentId: "42" });

      expect(mockedApiPost).toHaveBeenCalledWith(
        "http://localhost:8080/api/activities/1/enroll/42",
        {},
      );
      expect(result.success).toBe(true);
    });

    it("enrolls student in browser context", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const mockResponse = { ok: true };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.enrollStudent("1", { studentId: "42" });

      expect(result.success).toBe(true);
    });
  });

  describe("unenrollStudent", () => {
    it("unenrolls student in server context", async () => {
      mockedApiDelete.mockResolvedValueOnce({});

      await activityApi.unenrollStudent("1", "42");

      expect(mockedApiDelete).toHaveBeenCalledWith(
        "http://localhost:8080/api/activities/1/students/42",
      );
    });

    it("unenrolls student in browser context", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const mockResponse = { ok: true };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      await activityApi.unenrollStudent("1", "42");

      expect(globalThis.fetch).toHaveBeenCalledWith(
        "/api/activities/1/students/42",
        expect.objectContaining({ method: "DELETE" }),
      );
    });
  });

  describe("getActivitySchedules", () => {
    it("fetches schedules in server context", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: [sampleBackendSchedule] },
      });

      const result = await activityApi.getActivitySchedules("1");

      expect(result).toHaveLength(1);
      expect(result[0]?.weekday).toBe("1");
    });

    it("returns empty array on error", async () => {
      mockedApiGet.mockRejectedValueOnce(new Error("Network error"));

      const result = await activityApi.getActivitySchedules("1");

      expect(result).toEqual([]);
    });

    it("handles direct array response", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: [sampleBackendSchedule],
      });

      const result = await activityApi.getActivitySchedules("1");

      expect(result).toHaveLength(1);
    });
  });

  describe("getActivitySchedule", () => {
    it("fetches single schedule in server context", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: sampleBackendSchedule },
      });

      const result = await activityApi.getActivitySchedule("1", "1");

      expect(result?.weekday).toBe("1");
    });

    it("returns null on error", async () => {
      mockedApiGet.mockRejectedValueOnce(new Error("Not found"));

      const result = await activityApi.getActivitySchedule("1", "999");

      expect(result).toBeNull();
    });
  });

  describe("createActivitySchedule", () => {
    it("creates schedule in server context", async () => {
      mockedApiPost.mockResolvedValueOnce({
        data: { data: sampleBackendSchedule },
      });

      const result = await activityApi.createActivitySchedule("1", {
        weekday: "1",
        timeframe_id: "1",
      });

      expect(result.weekday).toBe("1");
    });
  });

  describe("updateActivitySchedule", () => {
    it("updates schedule in server context", async () => {
      mockedApiPut.mockResolvedValueOnce({
        data: { data: sampleBackendSchedule },
      });

      const result = await activityApi.updateActivitySchedule("1", "1", {
        weekday: "2",
      });

      expect(result?.weekday).toBe("1");
    });

    it("returns null on error", async () => {
      mockedApiPut.mockRejectedValueOnce(new Error("Update failed"));

      const result = await activityApi.updateActivitySchedule("1", "1", {
        weekday: "2",
      });

      expect(result).toBeNull();
    });
  });

  describe("deleteActivitySchedule", () => {
    it("deletes schedule in server context", async () => {
      mockedApiDelete.mockResolvedValueOnce({});

      const result = await activityApi.deleteActivitySchedule("1", "1");

      expect(result).toBe(true);
    });

    it("returns false on error", async () => {
      mockedApiDelete.mockRejectedValueOnce(new Error("Delete failed"));

      const result = await activityApi.deleteActivitySchedule("1", "1");

      expect(result).toBe(false);
    });
  });

  describe("getTimeframes / getAvailableTimeframes", () => {
    it("fetches timeframes in server context", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: [sampleBackendTimeframe] },
      });

      const result = await activityApi.getTimeframes();

      expect(result).toHaveLength(1);
      expect(result[0]?.name).toBe("Morning");
    });

    it("getAvailableTimeframes is an alias for getTimeframes", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: [sampleBackendTimeframe] },
      });

      const result = await activityApi.getAvailableTimeframes();

      expect(result).toHaveLength(1);
    });

    it("handles direct array response with backend format", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: [sampleBackendTimeframe],
      });

      const result = await activityApi.getTimeframes();

      expect(result).toHaveLength(1);
    });

    it("handles frontend format timeframes", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const frontendTimeframe = {
        id: "1",
        name: "Morning",
        start_time: "08:00",
        end_time: "12:00",
      };

      const mockResponse = {
        ok: true,
        json: async () => ({ data: [frontendTimeframe] }),
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.getTimeframes();

      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("1");
    });

    it("returns empty array for invalid response", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: null,
      });

      const result = await activityApi.getTimeframes();

      expect(result).toEqual([]);
    });

    it("returns empty array for empty array", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: [] },
      });

      const result = await activityApi.getTimeframes();

      expect(result).toEqual([]);
    });
  });

  describe("getAvailableTimeSlots", () => {
    it("fetches available time slots in server context", async () => {
      const slots = [{ weekday: "1", timeframe_id: "1" }];
      mockedApiGet.mockResolvedValueOnce({
        data: { data: slots },
      });

      const result = await activityApi.getAvailableTimeSlots("1");

      expect(result).toHaveLength(1);
    });

    it("fetches with date parameter", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: [] },
      });

      await activityApi.getAvailableTimeSlots("1", "2024-01-15");

      expect(mockedApiGet).toHaveBeenCalledWith(
        expect.stringContaining("date=2024-01-15"),
      );
    });

    it("returns empty array on error", async () => {
      mockedApiGet.mockRejectedValueOnce(new Error("Network error"));

      const result = await activityApi.getAvailableTimeSlots("1");

      expect(result).toEqual([]);
    });
  });

  describe("Supervisor management", () => {
    describe("getActivitySupervisors", () => {
      it("fetches activity supervisors in server context", async () => {
        const backendSupervisors = [
          {
            id: 1,
            staff_id: 10,
            is_primary: true,
            first_name: "John",
            last_name: "Doe",
          },
        ];
        mockedApiGet.mockResolvedValueOnce({
          data: { data: backendSupervisors },
        });

        const result = await activityApi.getActivitySupervisors("1");

        expect(result).toHaveLength(1);
        expect(result[0]?.name).toBe("John Doe");
      });

      it("returns empty array on error", async () => {
        mockedApiGet.mockRejectedValueOnce(new Error("Network error"));

        const result = await activityApi.getActivitySupervisors("1");

        expect(result).toEqual([]);
      });
    });

    describe("getAvailableSupervisors", () => {
      it("fetches available supervisors in server context", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [sampleBackendSupervisor] },
        });

        const result = await activityApi.getAvailableSupervisors("1");

        expect(result).toHaveLength(1);
      });

      it("returns empty array on error", async () => {
        mockedApiGet.mockRejectedValueOnce(new Error("Network error"));

        const result = await activityApi.getAvailableSupervisors("1");

        expect(result).toEqual([]);
      });
    });

    describe("assignSupervisor", () => {
      it("assigns supervisor in server context", async () => {
        mockedApiPost.mockResolvedValueOnce({});

        const result = await activityApi.assignSupervisor("1", {
          staff_id: "10",
          is_primary: true,
        });

        expect(mockedApiPost).toHaveBeenCalledWith(
          "http://localhost:8080/api/activities/1/supervisors",
          { staff_id: 10, is_primary: true },
        );
        expect(result).toBe(true);
      });

      it("returns false on error", async () => {
        mockedApiPost.mockRejectedValueOnce(new Error("Assignment failed"));

        const result = await activityApi.assignSupervisor("1", {
          staff_id: "10",
        });

        expect(result).toBe(false);
      });
    });

    describe("updateSupervisorRole", () => {
      it("updates supervisor role in server context", async () => {
        mockedApiPut.mockResolvedValueOnce({});

        const result = await activityApi.updateSupervisorRole("1", "10", {
          is_primary: true,
        });

        expect(result).toBe(true);
      });

      it("returns false on error", async () => {
        mockedApiPut.mockRejectedValueOnce(new Error("Update failed"));

        const result = await activityApi.updateSupervisorRole("1", "10", {
          is_primary: false,
        });

        expect(result).toBe(false);
      });
    });

    describe("removeSupervisor", () => {
      it("removes supervisor in server context", async () => {
        mockedApiDelete.mockResolvedValueOnce({});

        const result = await activityApi.removeSupervisor("1", "10");

        expect(result).toBe(true);
      });

      it("returns false on error", async () => {
        mockedApiDelete.mockRejectedValueOnce(new Error("Remove failed"));

        const result = await activityApi.removeSupervisor("1", "10");

        expect(result).toBe(false);
      });
    });
  });

  describe("getAvailableStudents", () => {
    it("fetches available students in server context", async () => {
      const backendStudents = [
        { id: 1, name: "Jane Smith", school_class: "5a" },
      ];
      mockedApiGet.mockResolvedValueOnce({
        data: { data: backendStudents },
      });

      const result = await activityApi.getAvailableStudents("1");

      expect(result).toHaveLength(1);
      expect(result[0]?.name).toBe("Jane Smith");
    });

    it("fetches with filters", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: [] },
      });

      await activityApi.getAvailableStudents("1", {
        search: "Jane",
        group_id: "5",
      });

      expect(mockedApiGet).toHaveBeenCalledWith(
        expect.stringContaining("search=Jane"),
      );
      expect(mockedApiGet).toHaveBeenCalledWith(
        expect.stringContaining("group_id=5"),
      );
    });

    it("returns empty array on error", async () => {
      mockedApiGet.mockRejectedValueOnce(new Error("Network error"));

      const result = await activityApi.getAvailableStudents("1");

      expect(result).toEqual([]);
    });
  });

  describe("updateGroupEnrollments", () => {
    it("updates enrollments in server context", async () => {
      mockedApiPut.mockResolvedValueOnce({});

      const result = await activityApi.updateGroupEnrollments("1", {
        student_ids: ["1", "2", "3"],
      });

      expect(mockedApiPut).toHaveBeenCalledWith(
        "http://localhost:8080/api/activities/1/students",
        { student_ids: [1, 2, 3] },
      );
      expect(result).toBe(true);
    });

    it("updates enrollments in browser context", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const mockResponse = { ok: true };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await activityApi.updateGroupEnrollments("1", {
        student_ids: ["1", "2", "3"],
      });

      expect(result).toBe(true);
    });

    it("throws error when no token available", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce(null);

      await expect(
        activityApi.updateGroupEnrollments("1", { student_ids: ["1"] }),
      ).rejects.toThrow("No authentication token available");
    });

    it("handles 401 with token refresh", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession
        .mockResolvedValueOnce({
          user: { id: "1", token: "old-token" },
          expires: "2099-01-01",
        })
        .mockResolvedValueOnce({
          user: { id: "1", token: "new-token" },
          expires: "2099-01-01",
        });

      mockedHandleAuthFailure.mockResolvedValueOnce(true);

      const fetchMock = vi
        .fn()
        .mockResolvedValueOnce({ ok: false, status: 401 })
        .mockResolvedValueOnce({ ok: true });
      vi.stubGlobal("fetch", fetchMock);

      const result = await activityApi.updateGroupEnrollments("1", {
        student_ids: ["1"],
      });

      expect(mockedHandleAuthFailure).toHaveBeenCalled();
      expect(result).toBe(true);
    });

    it("throws error on 403", async () => {
      vi.stubGlobal("window", {});
      mockedGetSession.mockResolvedValueOnce({
        user: { id: "1", token: "test-token" },
        expires: "2099-01-01",
      });

      const mockResponse = { ok: false, status: 403 };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      await expect(
        activityApi.updateGroupEnrollments("1", { student_ids: ["1"] }),
      ).rejects.toThrow("permission");
    });
  });
});
