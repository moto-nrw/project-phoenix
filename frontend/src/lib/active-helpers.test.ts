import { describe, it, expect } from "vitest";
import {
  mapActiveGroupResponse,
  mapVisitResponse,
  mapSupervisorResponse,
  mapCombinedGroupResponse,
  mapGroupMappingResponse,
  mapAnalyticsResponse,
  prepareActiveGroupForBackend,
  prepareVisitForBackend,
  prepareSupervisorForBackend,
  prepareCombinedGroupForBackend,
  prepareGroupMappingForBackend,
  type BackendActiveGroup,
  type BackendVisit,
  type BackendSupervisor,
  type BackendCombinedGroup,
  type BackendGroupMapping,
  type BackendAnalytics,
  type ActiveGroup,
  type Visit,
  type Supervisor,
  type CombinedGroup,
} from "./active-helpers";

// Sample backend data matching actual API responses
const sampleBackendActiveGroup: BackendActiveGroup = {
  id: 1,
  group_id: 10,
  room_id: 5,
  start_time: "2024-01-15T08:00:00Z",
  end_time: "2024-01-15T12:00:00Z",
  is_active: true,
  notes: "Morning session",
  visit_count: 25,
  supervisor_count: 2,
  room: { id: 5, name: "Room A", category: "classroom" },
  actual_group: { id: 10, name: "Class 3A" },
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-15T08:00:00Z",
};

const sampleBackendVisit: BackendVisit = {
  id: 100,
  student_id: 50,
  active_group_id: 1,
  check_in_time: "2024-01-15T08:30:00Z",
  check_out_time: "2024-01-15T11:45:00Z",
  is_active: false,
  notes: "Early checkout",
  student_name: "Max Mustermann",
  school_class: "3a",
  group_name: "OGS Group A",
  active_group_name: "Morning Session",
  created_at: "2024-01-15T08:30:00Z",
  updated_at: "2024-01-15T11:45:00Z",
};

const sampleBackendSupervisor: BackendSupervisor = {
  id: 200,
  staff_id: 30,
  active_group_id: 1,
  start_time: "2024-01-15T08:00:00Z",
  end_time: "2024-01-15T12:00:00Z",
  is_active: true,
  notes: "Primary supervisor",
  staff_name: "Frau Schmidt",
  active_group_name: "Morning Session",
  created_at: "2024-01-15T08:00:00Z",
  updated_at: "2024-01-15T08:00:00Z",
};

const sampleBackendCombinedGroup: BackendCombinedGroup = {
  id: 300,
  name: "Combined Morning",
  description: "Combined session for multiple groups",
  room_id: 5,
  start_time: "2024-01-15T08:00:00Z",
  end_time: "2024-01-15T12:00:00Z",
  is_active: true,
  notes: "Special combined session",
  group_count: 3,
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-15T08:00:00Z",
};

const sampleBackendGroupMapping: BackendGroupMapping = {
  id: 400,
  active_group_id: 1,
  combined_group_id: 300,
  group_name: "Class 3A",
  combined_name: "Combined Morning",
};

const sampleBackendAnalytics: BackendAnalytics = {
  active_groups_count: 5,
  total_visits_count: 150,
  active_visits_count: 45,
  room_utilization: 0.75,
  attendance_rate: 0.92,
};

describe("active-helpers", () => {
  describe("mapActiveGroupResponse", () => {
    it("maps all fields correctly from backend to frontend format", () => {
      const result = mapActiveGroupResponse(sampleBackendActiveGroup);

      expect(result.id).toBe("1");
      expect(result.groupId).toBe("10");
      expect(result.roomId).toBe("5");
      expect(result.startTime).toEqual(new Date("2024-01-15T08:00:00Z"));
      expect(result.endTime).toEqual(new Date("2024-01-15T12:00:00Z"));
      expect(result.isActive).toBe(true);
      expect(result.notes).toBe("Morning session");
      expect(result.visitCount).toBe(25);
      expect(result.supervisorCount).toBe(2);
      expect(result.room).toEqual({
        id: 5,
        name: "Room A",
        category: "classroom",
      });
      expect(result.actualGroup).toEqual({ id: 10, name: "Class 3A" });
      expect(result.createdAt).toEqual(new Date("2024-01-01T00:00:00Z"));
      expect(result.updatedAt).toEqual(new Date("2024-01-15T08:00:00Z"));
    });

    it("handles undefined optional fields", () => {
      const minimalGroup: BackendActiveGroup = {
        id: 1,
        group_id: 10,
        room_id: 5,
        start_time: "2024-01-15T08:00:00Z",
        is_active: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T08:00:00Z",
      };

      const result = mapActiveGroupResponse(minimalGroup);

      expect(result.endTime).toBeUndefined();
      expect(result.notes).toBeUndefined();
      expect(result.visitCount).toBeUndefined();
      expect(result.supervisorCount).toBeUndefined();
      expect(result.room).toBeUndefined();
      expect(result.actualGroup).toBeUndefined();
    });

    it("converts numeric IDs to strings", () => {
      const result = mapActiveGroupResponse(sampleBackendActiveGroup);

      expect(typeof result.id).toBe("string");
      expect(typeof result.groupId).toBe("string");
      expect(typeof result.roomId).toBe("string");
    });
  });

  describe("mapVisitResponse", () => {
    it("maps all fields correctly from backend to frontend format", () => {
      const result = mapVisitResponse(sampleBackendVisit);

      expect(result.id).toBe("100");
      expect(result.studentId).toBe("50");
      expect(result.activeGroupId).toBe("1");
      expect(result.checkInTime).toEqual(new Date("2024-01-15T08:30:00Z"));
      expect(result.checkOutTime).toEqual(new Date("2024-01-15T11:45:00Z"));
      expect(result.isActive).toBe(false);
      expect(result.notes).toBe("Early checkout");
      expect(result.studentName).toBe("Max Mustermann");
      expect(result.schoolClass).toBe("3a");
      expect(result.groupName).toBe("OGS Group A");
      expect(result.activeGroupName).toBe("Morning Session");
    });

    it("handles undefined optional fields", () => {
      const minimalVisit: BackendVisit = {
        id: 100,
        student_id: 50,
        active_group_id: 1,
        check_in_time: "2024-01-15T08:30:00Z",
        is_active: true,
        created_at: "2024-01-15T08:30:00Z",
        updated_at: "2024-01-15T08:30:00Z",
      };

      const result = mapVisitResponse(minimalVisit);

      expect(result.checkOutTime).toBeUndefined();
      expect(result.notes).toBeUndefined();
      expect(result.studentName).toBeUndefined();
      expect(result.schoolClass).toBeUndefined();
      expect(result.groupName).toBeUndefined();
      expect(result.activeGroupName).toBeUndefined();
    });
  });

  describe("mapSupervisorResponse", () => {
    it("maps all fields correctly from backend to frontend format", () => {
      const result = mapSupervisorResponse(sampleBackendSupervisor);

      expect(result.id).toBe("200");
      expect(result.staffId).toBe("30");
      expect(result.activeGroupId).toBe("1");
      expect(result.startTime).toEqual(new Date("2024-01-15T08:00:00Z"));
      expect(result.endTime).toEqual(new Date("2024-01-15T12:00:00Z"));
      expect(result.isActive).toBe(true);
      expect(result.notes).toBe("Primary supervisor");
      expect(result.staffName).toBe("Frau Schmidt");
      expect(result.activeGroupName).toBe("Morning Session");
    });

    it("handles undefined optional fields", () => {
      const minimalSupervisor: BackendSupervisor = {
        id: 200,
        staff_id: 30,
        active_group_id: 1,
        start_time: "2024-01-15T08:00:00Z",
        is_active: true,
        created_at: "2024-01-15T08:00:00Z",
        updated_at: "2024-01-15T08:00:00Z",
      };

      const result = mapSupervisorResponse(minimalSupervisor);

      expect(result.endTime).toBeUndefined();
      expect(result.notes).toBeUndefined();
      expect(result.staffName).toBeUndefined();
      expect(result.activeGroupName).toBeUndefined();
    });
  });

  describe("mapCombinedGroupResponse", () => {
    it("maps all fields correctly from backend to frontend format", () => {
      const result = mapCombinedGroupResponse(sampleBackendCombinedGroup);

      expect(result.id).toBe("300");
      expect(result.name).toBe("Combined Morning");
      expect(result.description).toBe("Combined session for multiple groups");
      expect(result.roomId).toBe("5");
      expect(result.startTime).toEqual(new Date("2024-01-15T08:00:00Z"));
      expect(result.endTime).toEqual(new Date("2024-01-15T12:00:00Z"));
      expect(result.isActive).toBe(true);
      expect(result.notes).toBe("Special combined session");
      expect(result.groupCount).toBe(3);
    });

    it("handles undefined optional fields", () => {
      const minimalCombinedGroup: BackendCombinedGroup = {
        id: 300,
        name: "Combined Morning",
        room_id: 5,
        start_time: "2024-01-15T08:00:00Z",
        is_active: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T08:00:00Z",
      };

      const result = mapCombinedGroupResponse(minimalCombinedGroup);

      expect(result.description).toBeUndefined();
      expect(result.endTime).toBeUndefined();
      expect(result.notes).toBeUndefined();
      expect(result.groupCount).toBeUndefined();
    });
  });

  describe("mapGroupMappingResponse", () => {
    it("maps all fields correctly from backend to frontend format", () => {
      const result = mapGroupMappingResponse(sampleBackendGroupMapping);

      expect(result.id).toBe("400");
      expect(result.activeGroupId).toBe("1");
      expect(result.combinedGroupId).toBe("300");
      expect(result.groupName).toBe("Class 3A");
      expect(result.combinedName).toBe("Combined Morning");
    });

    it("handles undefined optional fields", () => {
      const minimalMapping: BackendGroupMapping = {
        id: 400,
        active_group_id: 1,
        combined_group_id: 300,
      };

      const result = mapGroupMappingResponse(minimalMapping);

      expect(result.groupName).toBeUndefined();
      expect(result.combinedName).toBeUndefined();
    });
  });

  describe("mapAnalyticsResponse", () => {
    it("maps all fields correctly from backend to frontend format", () => {
      const result = mapAnalyticsResponse(sampleBackendAnalytics);

      expect(result.activeGroupsCount).toBe(5);
      expect(result.totalVisitsCount).toBe(150);
      expect(result.activeVisitsCount).toBe(45);
      expect(result.roomUtilization).toBe(0.75);
      expect(result.attendanceRate).toBe(0.92);
    });

    it("handles undefined optional fields", () => {
      const emptyAnalytics: BackendAnalytics = {};

      const result = mapAnalyticsResponse(emptyAnalytics);

      expect(result.activeGroupsCount).toBeUndefined();
      expect(result.totalVisitsCount).toBeUndefined();
      expect(result.activeVisitsCount).toBeUndefined();
      expect(result.roomUtilization).toBeUndefined();
      expect(result.attendanceRate).toBeUndefined();
    });

    it("handles partial analytics data", () => {
      const partialAnalytics: BackendAnalytics = {
        active_groups_count: 3,
        room_utilization: 0.5,
      };

      const result = mapAnalyticsResponse(partialAnalytics);

      expect(result.activeGroupsCount).toBe(3);
      expect(result.roomUtilization).toBe(0.5);
      expect(result.totalVisitsCount).toBeUndefined();
      expect(result.activeVisitsCount).toBeUndefined();
      expect(result.attendanceRate).toBeUndefined();
    });
  });

  describe("prepareActiveGroupForBackend", () => {
    it("converts frontend ActiveGroup to backend format", () => {
      const frontendGroup: Partial<ActiveGroup> = {
        groupId: "10",
        roomId: "5",
        startTime: new Date("2024-01-15T08:00:00Z"),
        endTime: new Date("2024-01-15T12:00:00Z"),
        notes: "Test notes",
      };

      const result = prepareActiveGroupForBackend(frontendGroup);

      expect(result.group_id).toBe(10);
      expect(result.room_id).toBe(5);
      expect(result.start_time).toBe("2024-01-15T08:00:00.000Z");
      expect(result.end_time).toBe("2024-01-15T12:00:00.000Z");
      expect(result.notes).toBe("Test notes");
    });

    it("only includes defined fields", () => {
      const partialGroup: Partial<ActiveGroup> = {
        groupId: "10",
        roomId: "5",
      };

      const result = prepareActiveGroupForBackend(partialGroup);

      expect(result.group_id).toBe(10);
      expect(result.room_id).toBe(5);
      expect(result.start_time).toBeUndefined();
      expect(result.end_time).toBeUndefined();
      expect(result.notes).toBeUndefined();
    });

    it("handles empty notes", () => {
      const groupWithEmptyNotes: Partial<ActiveGroup> = {
        groupId: "10",
        notes: "",
      };

      const result = prepareActiveGroupForBackend(groupWithEmptyNotes);

      expect(result.notes).toBe("");
    });
  });

  describe("prepareVisitForBackend", () => {
    it("converts frontend Visit to backend format", () => {
      const frontendVisit: Partial<Visit> = {
        studentId: "50",
        activeGroupId: "1",
        checkInTime: new Date("2024-01-15T08:30:00Z"),
        checkOutTime: new Date("2024-01-15T11:45:00Z"),
        notes: "Test visit",
      };

      const result = prepareVisitForBackend(frontendVisit);

      expect(result.student_id).toBe(50);
      expect(result.active_group_id).toBe(1);
      expect(result.check_in_time).toBe("2024-01-15T08:30:00.000Z");
      expect(result.check_out_time).toBe("2024-01-15T11:45:00.000Z");
      expect(result.notes).toBe("Test visit");
    });

    it("only includes defined fields", () => {
      const partialVisit: Partial<Visit> = {
        studentId: "50",
        activeGroupId: "1",
      };

      const result = prepareVisitForBackend(partialVisit);

      expect(result.student_id).toBe(50);
      expect(result.active_group_id).toBe(1);
      expect(result.check_in_time).toBeUndefined();
      expect(result.check_out_time).toBeUndefined();
      expect(result.notes).toBeUndefined();
    });
  });

  describe("prepareSupervisorForBackend", () => {
    it("converts frontend Supervisor to backend format", () => {
      const frontendSupervisor: Partial<Supervisor> = {
        staffId: "30",
        activeGroupId: "1",
        startTime: new Date("2024-01-15T08:00:00Z"),
        endTime: new Date("2024-01-15T12:00:00Z"),
        notes: "Primary supervisor",
      };

      const result = prepareSupervisorForBackend(frontendSupervisor);

      expect(result.staff_id).toBe(30);
      expect(result.active_group_id).toBe(1);
      expect(result.start_time).toBe("2024-01-15T08:00:00.000Z");
      expect(result.end_time).toBe("2024-01-15T12:00:00.000Z");
      expect(result.notes).toBe("Primary supervisor");
    });

    it("only includes defined fields", () => {
      const partialSupervisor: Partial<Supervisor> = {
        staffId: "30",
        activeGroupId: "1",
      };

      const result = prepareSupervisorForBackend(partialSupervisor);

      expect(result.staff_id).toBe(30);
      expect(result.active_group_id).toBe(1);
      expect(result.start_time).toBeUndefined();
      expect(result.end_time).toBeUndefined();
      expect(result.notes).toBeUndefined();
    });
  });

  describe("prepareCombinedGroupForBackend", () => {
    it("converts frontend CombinedGroup to backend format", () => {
      const frontendCombined: Partial<CombinedGroup> = {
        name: "Combined Morning",
        description: "Combined session",
        roomId: "5",
        startTime: new Date("2024-01-15T08:00:00Z"),
        endTime: new Date("2024-01-15T12:00:00Z"),
        notes: "Test combined",
      };

      const result = prepareCombinedGroupForBackend(frontendCombined);

      expect(result.name).toBe("Combined Morning");
      expect(result.description).toBe("Combined session");
      expect(result.room_id).toBe(5);
      expect(result.start_time).toBe("2024-01-15T08:00:00.000Z");
      expect(result.end_time).toBe("2024-01-15T12:00:00.000Z");
      expect(result.notes).toBe("Test combined");
    });

    it("only includes defined fields", () => {
      const partialCombined: Partial<CombinedGroup> = {
        name: "Test Group",
        roomId: "5",
      };

      const result = prepareCombinedGroupForBackend(partialCombined);

      expect(result.name).toBe("Test Group");
      expect(result.room_id).toBe(5);
      expect(result.description).toBeUndefined();
      expect(result.start_time).toBeUndefined();
      expect(result.end_time).toBeUndefined();
      expect(result.notes).toBeUndefined();
    });

    it("handles empty description", () => {
      const combinedWithEmptyDesc: Partial<CombinedGroup> = {
        name: "Test",
        description: "",
      };

      const result = prepareCombinedGroupForBackend(combinedWithEmptyDesc);

      expect(result.description).toBe("");
    });
  });

  describe("prepareGroupMappingForBackend", () => {
    it("converts frontend group mapping to backend format", () => {
      const mapping = {
        activeGroupId: "1",
        combinedGroupId: "300",
      };

      const result = prepareGroupMappingForBackend(mapping);

      expect(result.active_group_id).toBe(1);
      expect(result.combined_group_id).toBe(300);
    });

    it("parses string IDs to integers", () => {
      const mapping = {
        activeGroupId: "999",
        combinedGroupId: "1000",
      };

      const result = prepareGroupMappingForBackend(mapping);

      expect(typeof result.active_group_id).toBe("number");
      expect(typeof result.combined_group_id).toBe("number");
      expect(result.active_group_id).toBe(999);
      expect(result.combined_group_id).toBe(1000);
    });
  });
});
