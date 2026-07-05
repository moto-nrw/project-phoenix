import { describe, it, expect } from "vitest";
import {
  mapActiveGroupResponse,
  mapVisitResponse,
  mapSupervisorResponse,
  mapSchulhofStatusResponse,
  mapToggleSupervisionResponse,
  prepareActiveGroupForBackend,
  prepareVisitForBackend,
  prepareSupervisorForBackend,
  type BackendActiveGroup,
  type BackendVisit,
  type BackendSupervisor,
  type BackendSchulhofStatus,
  type BackendToggleSupervisionResponse,
  type ActiveGroup,
  type Visit,
  type Supervisor,
} from "./active-helpers";
import {
  buildBackendActiveSession,
  buildBackendVisit,
  buildBackendSupervisor,
} from "~/test/fixtures/sessions";

// Sample backend data matching actual API responses
const sampleBackendActiveGroup = buildBackendActiveSession({
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
});

const sampleBackendVisit = buildBackendVisit({
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
});

const sampleBackendSupervisor = buildBackendSupervisor({
  id: 200,
  staff_id: 30,
  active_group_id: 1,
  start_time: "2024-01-15T08:00:00Z",
  end_time: "2024-01-15T12:00:00Z",
  is_active: true,
  notes: "Primary supervisor",
  staff_name: "Frau Schmidt",
  active_group_name: "Morning Session",
});

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
      expect(result.actualArrivalTime).toBeUndefined();
      expect(result.actualPickupTime).toBeUndefined();
    });

    it("preserves actual_arrival_time and actual_pickup_time as HH:MM strings", () => {
      // The backend sends the wall-clock string already formatted in Berlin
      // time (`timezone.FormatBerlinClock`). The mapper must NOT parse it
      // into a Date — that would re-anchor it to today's local TZ and
      // break diff calculations against `plannedTime` strings.
      const visit = buildBackendVisit({
        actual_arrival_time: "08:30",
        actual_pickup_time: "16:05",
      });

      const result = mapVisitResponse(visit);

      expect(result.actualArrivalTime).toBe("08:30");
      expect(result.actualPickupTime).toBe("16:05");
      expect(typeof result.actualArrivalTime).toBe("string");
    });

    it("threads only actual_arrival_time when student is still checked in", () => {
      const visit = buildBackendVisit({
        actual_arrival_time: "08:00",
        actual_pickup_time: undefined,
        check_out_time: undefined,
        is_active: true,
      });

      const result = mapVisitResponse(visit);

      expect(result.actualArrivalTime).toBe("08:00");
      expect(result.actualPickupTime).toBeUndefined();
      expect(result.checkOutTime).toBeUndefined();
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

  describe("mapSchulhofStatusResponse", () => {
    it("maps all fields correctly with complete data", () => {
      const backendStatus: BackendSchulhofStatus = {
        exists: true,
        room_id: 42,
        room_name: "Schulhof",
        activity_group_id: 10,
        active_group_id: 5,
        is_user_supervising: true,
        supervision_id: 123,
        supervisor_count: 3,
        student_count: 25,
        supervisors: [
          {
            id: 1,
            staff_id: 100,
            name: "Frau Schmidt",
            is_current_user: true,
          },
          {
            id: 2,
            staff_id: 101,
            name: "Herr Müller",
            is_current_user: false,
          },
        ],
      };

      const result = mapSchulhofStatusResponse(backendStatus);

      expect(result.exists).toBe(true);
      expect(result.roomId).toBe("42");
      expect(result.roomName).toBe("Schulhof");
      expect(result.activityGroupId).toBe("10");
      expect(result.activeGroupId).toBe("5");
      expect(result.isUserSupervising).toBe(true);
      expect(result.supervisionId).toBe("123");
      expect(result.supervisorCount).toBe(3);
      expect(result.studentCount).toBe(25);
      expect(result.supervisors).toHaveLength(2);
      expect(result.supervisors[0]).toEqual({
        id: "1",
        staffId: "100",
        name: "Frau Schmidt",
        isCurrentUser: true,
      });
      expect(result.supervisors[1]).toEqual({
        id: "2",
        staffId: "101",
        name: "Herr Müller",
        isCurrentUser: false,
      });
    });

    it("handles non-existent Schulhof with minimal data", () => {
      const backendStatus: BackendSchulhofStatus = {
        exists: false,
        room_name: "",
        is_user_supervising: false,
        supervisor_count: 0,
        student_count: 0,
        supervisors: [],
      };

      const result = mapSchulhofStatusResponse(backendStatus);

      expect(result.exists).toBe(false);
      expect(result.roomId).toBeNull();
      expect(result.roomName).toBe("");
      expect(result.activityGroupId).toBeNull();
      expect(result.activeGroupId).toBeNull();
      expect(result.isUserSupervising).toBe(false);
      expect(result.supervisionId).toBeNull();
      expect(result.supervisorCount).toBe(0);
      expect(result.studentCount).toBe(0);
      expect(result.supervisors).toEqual([]);
    });

    it("handles undefined optional fields as null", () => {
      const backendStatus: BackendSchulhofStatus = {
        exists: true,
        room_name: "Schulhof",
        is_user_supervising: false,
        supervisor_count: 0,
        student_count: 0,
        supervisors: [],
        // room_id, activity_group_id, active_group_id, supervision_id are undefined
      };

      const result = mapSchulhofStatusResponse(backendStatus);

      expect(result.roomId).toBeNull();
      expect(result.activityGroupId).toBeNull();
      expect(result.activeGroupId).toBeNull();
      expect(result.supervisionId).toBeNull();
    });

    it("converts numeric IDs to strings", () => {
      const backendStatus: BackendSchulhofStatus = {
        exists: true,
        room_id: 99,
        room_name: "Test Room",
        activity_group_id: 88,
        active_group_id: 77,
        is_user_supervising: true,
        supervision_id: 66,
        supervisor_count: 1,
        student_count: 10,
        supervisors: [
          {
            id: 999,
            staff_id: 888,
            name: "Test",
            is_current_user: true,
          },
        ],
      };

      const result = mapSchulhofStatusResponse(backendStatus);

      expect(typeof result.roomId).toBe("string");
      expect(typeof result.activityGroupId).toBe("string");
      expect(typeof result.activeGroupId).toBe("string");
      expect(typeof result.supervisionId).toBe("string");
      expect(typeof result.supervisors[0]!.id).toBe("string");
      expect(typeof result.supervisors[0]!.staffId).toBe("string");
    });

    it("maps supervisor array correctly", () => {
      const backendStatus: BackendSchulhofStatus = {
        exists: true,
        room_name: "Schulhof",
        is_user_supervising: false,
        supervisor_count: 2,
        student_count: 0,
        supervisors: [
          {
            id: 1,
            staff_id: 10,
            name: "Supervisor One",
            is_current_user: false,
          },
          {
            id: 2,
            staff_id: 20,
            name: "Supervisor Two",
            is_current_user: true,
          },
        ],
      };

      const result = mapSchulhofStatusResponse(backendStatus);

      expect(result.supervisors).toHaveLength(2);
      expect(result.supervisors[0]?.isCurrentUser).toBe(false);
      expect(result.supervisors[1]?.isCurrentUser).toBe(true);
    });

    it("handles empty supervisors array", () => {
      const backendStatus: BackendSchulhofStatus = {
        exists: true,
        room_name: "Schulhof",
        is_user_supervising: false,
        supervisor_count: 0,
        student_count: 0,
        supervisors: [],
      };

      const result = mapSchulhofStatusResponse(backendStatus);

      expect(result.supervisors).toEqual([]);
      expect(result.supervisorCount).toBe(0);
    });
  });

  describe("mapToggleSupervisionResponse", () => {
    it("maps started action correctly", () => {
      const backendResponse: BackendToggleSupervisionResponse = {
        action: "started",
        supervision_id: 456,
        active_group_id: 789,
      };

      const result = mapToggleSupervisionResponse(backendResponse);

      expect(result.action).toBe("started");
      expect(result.supervisionId).toBe("456");
      expect(result.activeGroupId).toBe("789");
    });

    it("maps stopped action correctly", () => {
      const backendResponse: BackendToggleSupervisionResponse = {
        action: "stopped",
        active_group_id: 123,
      };

      const result = mapToggleSupervisionResponse(backendResponse);

      expect(result.action).toBe("stopped");
      expect(result.supervisionId).toBeNull();
      expect(result.activeGroupId).toBe("123");
    });

    it("handles undefined supervision_id as null", () => {
      const backendResponse: BackendToggleSupervisionResponse = {
        action: "stopped",
        active_group_id: 999,
        // supervision_id is undefined when stopping
      };

      const result = mapToggleSupervisionResponse(backendResponse);

      expect(result.supervisionId).toBeNull();
    });

    it("converts numeric IDs to strings", () => {
      const backendResponse: BackendToggleSupervisionResponse = {
        action: "started",
        supervision_id: 111,
        active_group_id: 222,
      };

      const result = mapToggleSupervisionResponse(backendResponse);

      expect(typeof result.supervisionId).toBe("string");
      expect(typeof result.activeGroupId).toBe("string");
    });

    it("preserves action as literal type", () => {
      const backendResponse: BackendToggleSupervisionResponse = {
        action: "started",
        supervision_id: 1,
        active_group_id: 2,
      };

      const result = mapToggleSupervisionResponse(backendResponse);

      // TypeScript should infer this as "started" | "stopped"
      const action: "started" | "stopped" = result.action;
      expect(action).toBe("started");
    });
  });
});
