import { describe, it, expect } from "vitest";
import {
  mapSubstitutionResponse,
  mapTeacherAvailabilityResponse,
  mapSubstitutionsResponse,
  mapTeacherAvailabilityResponses,
  prepareSubstitutionForBackend,
  formatDateForBackend,
  formatTeacherName,
  getTeacherStatus,
  getSubstitutionCounts,
  type BackendSubstitution,
  type BackendStaffWithSubstitutionStatus,
  type TeacherAvailability,
} from "./substitution-helpers";

describe("substitution-helpers", () => {
  describe("mapSubstitutionResponse", () => {
    it("should map backend substitution with full data", () => {
      const backend: BackendSubstitution = {
        id: 123,
        group_id: 456,
        group: {
          id: 456,
          name: "Group A",
        },
        regular_staff_id: 789,
        regular_staff: {
          id: 789,
          person_id: 100,
          person: {
            id: 100,
            first_name: "Regular",
            last_name: "Teacher",
            full_name: "Regular Teacher",
          },
        },
        substitute_staff_id: 999,
        substitute_staff: {
          id: 999,
          person_id: 200,
          person: {
            id: 200,
            first_name: "Substitute",
            last_name: "Teacher",
            full_name: "Substitute Teacher",
          },
        },
        start_date: "2024-01-15",
        end_date: "2024-01-20",
        reason: "Vacation",
        notes: "Test notes",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T00:00:00Z",
      };

      const result = mapSubstitutionResponse(backend);

      expect(result).toEqual({
        id: "123",
        groupId: "456",
        groupName: "Group A",
        substituteStaffId: "999",
        substituteStaffName: "Substitute Teacher",
        startDate: new Date("2024-01-15"),
        endDate: new Date("2024-01-20"),
        reason: "Vacation",
        notes: "Test notes",
        isTransfer: false,
      });
    });

    it("should fallback to first+last name when full_name is missing", () => {
      const backend: BackendSubstitution = {
        id: 1,
        group_id: 2,
        substitute_staff_id: 3,
        substitute_staff: {
          id: 3,
          person_id: 4,
          person: {
            id: 4,
            first_name: "John",
            last_name: "Doe",
          },
        },
        start_date: "2024-01-15",
        end_date: "2024-01-15",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T00:00:00Z",
      };

      const result = mapSubstitutionResponse(backend);

      expect(result.substituteStaffName).toBe("John Doe");
    });

    it("should handle missing substitute person", () => {
      const backend: BackendSubstitution = {
        id: 1,
        group_id: 2,
        substitute_staff_id: 3,
        start_date: "2024-01-15",
        end_date: "2024-01-15",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T00:00:00Z",
      };

      const result = mapSubstitutionResponse(backend);

      expect(result.substituteStaffName).toBeUndefined();
    });

    it("should set isTransfer to true when start and end dates are the same", () => {
      const backend: BackendSubstitution = {
        id: 1,
        group_id: 2,
        substitute_staff_id: 3,
        start_date: "2024-01-15",
        end_date: "2024-01-15",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T00:00:00Z",
      };

      const result = mapSubstitutionResponse(backend);

      expect(result.isTransfer).toBe(true);
    });

    it("should set isTransfer to false when start and end dates differ", () => {
      const backend: BackendSubstitution = {
        id: 1,
        group_id: 2,
        substitute_staff_id: 3,
        start_date: "2024-01-15",
        end_date: "2024-01-20",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T00:00:00Z",
      };

      const result = mapSubstitutionResponse(backend);

      expect(result.isTransfer).toBe(false);
    });
  });

  describe("mapTeacherAvailabilityResponse", () => {
    it("should map staff with substitutions array", () => {
      const backend: BackendStaffWithSubstitutionStatus = {
        id: 123,
        person_id: 456,
        person: {
          id: 456,
          first_name: "Jane",
          last_name: "Doe",
        },
        is_substituting: true,
        substitution_count: 2,
        substitutions: [
          {
            id: 1,
            group_id: 100,
            group_name: "Group A",
            is_transfer: true,
            start_date: "2024-01-15",
            end_date: "2024-01-15",
          },
          {
            id: 2,
            group_id: 200,
            group: {
              id: 200,
              name: "Group B",
            },
            is_transfer: false,
            start_date: "2024-01-15",
            end_date: "2024-01-20",
          },
        ],
        regular_group: {
          id: 300,
          name: "Home Group",
        },
        current_group: {
          id: 100,
          name: "Current Group",
        },
        teacher_id: 789,
        role: "Lead Teacher",
        specialization: "Mathematics",
      };

      const result = mapTeacherAvailabilityResponse(backend);

      expect(result).toEqual({
        id: "123",
        firstName: "Jane",
        lastName: "Doe",
        regularGroup: "Home Group",
        role: "Lead Teacher",
        inSubstitution: true,
        substitutionCount: 2,
        substitutions: [
          {
            id: "1",
            groupId: "100",
            groupName: "Group A",
            isTransfer: true,
            startDate: new Date("2024-01-15"),
            endDate: new Date("2024-01-15"),
          },
          {
            id: "2",
            groupId: "200",
            groupName: "Group B",
            isTransfer: false,
            startDate: new Date("2024-01-15"),
            endDate: new Date("2024-01-20"),
          },
        ],
        currentGroup: "Current Group",
        teacherId: "789",
        specialization: "Mathematics",
      });
    });

    it("should handle staff without substitutions", () => {
      const backend: BackendStaffWithSubstitutionStatus = {
        id: 1,
        person_id: 2,
        person: {
          id: 2,
          first_name: "John",
          last_name: "Smith",
        },
        is_substituting: false,
        substitution_count: 0,
      };

      const result = mapTeacherAvailabilityResponse(backend);

      expect(result.substitutions).toEqual([]);
      expect(result.substitutionCount).toBe(0);
    });

    it("should handle staff without person", () => {
      const backend: BackendStaffWithSubstitutionStatus = {
        id: 1,
        person_id: 2,
        is_substituting: false,
        substitution_count: 0,
      };

      const result = mapTeacherAvailabilityResponse(backend);

      expect(result.firstName).toBe("");
      expect(result.lastName).toBe("");
    });

    it("should handle staff with teacher_id", () => {
      const backend: BackendStaffWithSubstitutionStatus = {
        id: 1,
        person_id: 2,
        person: {
          id: 2,
          first_name: "Test",
          last_name: "Teacher",
        },
        is_substituting: false,
        substitution_count: 0,
        teacher_id: 999,
      };

      const result = mapTeacherAvailabilityResponse(backend);

      expect(result.teacherId).toBe("999");
    });

    it("should handle staff without teacher_id", () => {
      const backend: BackendStaffWithSubstitutionStatus = {
        id: 1,
        person_id: 2,
        person: {
          id: 2,
          first_name: "Test",
          last_name: "Staff",
        },
        is_substituting: false,
        substitution_count: 0,
      };

      const result = mapTeacherAvailabilityResponse(backend);

      expect(result.teacherId).toBeUndefined();
    });

    it("should prefer group_name over group.name in substitutions", () => {
      const backend: BackendStaffWithSubstitutionStatus = {
        id: 1,
        person_id: 2,
        person: {
          id: 2,
          first_name: "Test",
          last_name: "Teacher",
        },
        is_substituting: true,
        substitution_count: 1,
        substitutions: [
          {
            id: 1,
            group_id: 100,
            group_name: "Direct Name",
            group: {
              id: 100,
              name: "Nested Name",
            },
            is_transfer: false,
            start_date: "2024-01-15",
            end_date: "2024-01-20",
          },
        ],
      };

      const result = mapTeacherAvailabilityResponse(backend);

      expect(result.substitutions[0]?.groupName).toBe("Direct Name");
    });

    it("should use 'Unbekannt' when no group name is available", () => {
      const backend: BackendStaffWithSubstitutionStatus = {
        id: 1,
        person_id: 2,
        person: {
          id: 2,
          first_name: "Test",
          last_name: "Teacher",
        },
        is_substituting: true,
        substitution_count: 1,
        substitutions: [
          {
            id: 1,
            group_id: 100,
            is_transfer: false,
            start_date: "2024-01-15",
            end_date: "2024-01-20",
          },
        ],
      };

      const result = mapTeacherAvailabilityResponse(backend);

      expect(result.substitutions[0]?.groupName).toBe("Unbekannt");
    });
  });

  describe("mapSubstitutionsResponse", () => {
    it("should map array of substitutions", () => {
      const backend: BackendSubstitution[] = [
        {
          id: 1,
          group_id: 2,
          substitute_staff_id: 3,
          start_date: "2024-01-15",
          end_date: "2024-01-15",
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-15T00:00:00Z",
        },
        {
          id: 4,
          group_id: 5,
          substitute_staff_id: 6,
          start_date: "2024-01-20",
          end_date: "2024-01-25",
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-15T00:00:00Z",
        },
      ];

      const result = mapSubstitutionsResponse(backend);

      expect(result).toHaveLength(2);
      expect(result[0]?.id).toBe("1");
      expect(result[1]?.id).toBe("4");
    });

    it("should return empty array for non-array input", () => {
      const result = mapSubstitutionsResponse(
        "not an array" as unknown as BackendSubstitution[],
      );
      expect(result).toEqual([]);
    });
  });

  describe("mapTeacherAvailabilityResponses", () => {
    it("should map array of staff", () => {
      const backend: BackendStaffWithSubstitutionStatus[] = [
        {
          id: 1,
          person_id: 2,
          person: {
            id: 2,
            first_name: "Teacher",
            last_name: "One",
          },
          is_substituting: false,
          substitution_count: 0,
        },
        {
          id: 3,
          person_id: 4,
          person: {
            id: 4,
            first_name: "Teacher",
            last_name: "Two",
          },
          is_substituting: true,
          substitution_count: 1,
        },
      ];

      const result = mapTeacherAvailabilityResponses(backend);

      expect(result).toHaveLength(2);
      expect(result[0]?.id).toBe("1");
      expect(result[1]?.id).toBe("3");
    });

    it("should return empty array for non-array input", () => {
      const result = mapTeacherAvailabilityResponses(
        {} as unknown as BackendStaffWithSubstitutionStatus[],
      );
      expect(result).toEqual([]);
    });
  });

  describe("prepareSubstitutionForBackend", () => {
    it("should prepare substitution with all fields", () => {
      const startDate = new Date("2024-01-15T00:00:00Z");
      const endDate = new Date("2024-01-20T00:00:00Z");

      const result = prepareSubstitutionForBackend(
        "100",
        "200",
        "300",
        startDate,
        endDate,
        "Test reason",
        "Test notes",
      );

      expect(result).toEqual({
        group_id: 100,
        regular_staff_id: 200,
        substitute_staff_id: 300,
        start_date: "2024-01-15",
        end_date: "2024-01-20",
        reason: "Test reason",
        notes: "Test notes",
      });
    });

    it("should handle null regularStaffId", () => {
      const startDate = new Date("2024-01-15T00:00:00Z");
      const endDate = new Date("2024-01-20T00:00:00Z");

      const result = prepareSubstitutionForBackend(
        "100",
        null,
        "300",
        startDate,
        endDate,
      );

      expect(result).toEqual({
        group_id: 100,
        regular_staff_id: undefined,
        substitute_staff_id: 300,
        start_date: "2024-01-15",
        end_date: "2024-01-20",
        reason: undefined,
        notes: undefined,
      });
    });
  });

  describe("formatDateForBackend", () => {
    it("should format date to YYYY-MM-DD", () => {
      const date = new Date("2024-01-15T10:30:00Z");
      expect(formatDateForBackend(date)).toBe("2024-01-15");
    });

    it("should handle different dates", () => {
      const date = new Date("2024-12-31T23:59:59Z");
      expect(formatDateForBackend(date)).toBe("2024-12-31");
    });
  });

  describe("formatTeacherName", () => {
    it("should format teacher name", () => {
      const teacher = {
        id: "1",
        firstName: "John",
        lastName: "Doe",
        inSubstitution: false,
        substitutionCount: 0,
        substitutions: [],
      };

      expect(formatTeacherName(teacher)).toBe("John Doe");
    });

    it("should format with spaces between names", () => {
      const teacher = {
        id: "1",
        firstName: "Jane",
        lastName: "Smith",
        inSubstitution: false,
        substitutionCount: 0,
        substitutions: [],
      };

      expect(formatTeacherName(teacher)).toBe("Jane Smith");
    });
  });

  describe("getTeacherStatus", () => {
    it("should return 'Verfügbar' when not substituting", () => {
      const teacher = {
        id: "1",
        firstName: "John",
        lastName: "Doe",
        inSubstitution: false,
        substitutionCount: 0,
        substitutions: [],
      };

      expect(getTeacherStatus(teacher)).toBe("Verfügbar");
    });

    it("should return transfer status for single transfer", () => {
      const teacher = {
        id: "1",
        firstName: "John",
        lastName: "Doe",
        inSubstitution: true,
        substitutionCount: 1,
        substitutions: [
          {
            id: "1",
            groupId: "100",
            groupName: "Group A",
            isTransfer: true,
            startDate: new Date("2024-01-15"),
            endDate: new Date("2024-01-15"),
          },
        ],
      };

      expect(getTeacherStatus(teacher)).toBe("Übergabe: Group A");
    });

    it("should return substitution status for single non-transfer", () => {
      const teacher = {
        id: "1",
        firstName: "John",
        lastName: "Doe",
        inSubstitution: true,
        substitutionCount: 1,
        substitutions: [
          {
            id: "1",
            groupId: "100",
            groupName: "Group B",
            isTransfer: false,
            startDate: new Date("2024-01-15"),
            endDate: new Date("2024-01-20"),
          },
        ],
      };

      expect(getTeacherStatus(teacher)).toBe("Vertretung: Group B");
    });

    it("should use currentGroup when substitution groupName is empty", () => {
      const teacher: TeacherAvailability = {
        id: "1",
        firstName: "John",
        lastName: "Doe",
        inSubstitution: true,
        substitutionCount: 1,
        currentGroup: "Current Group",
        substitutions: [
          {
            id: "1",
            groupId: "100",
            groupName: "Current Group",
            isTransfer: false,
            startDate: new Date("2024-01-15"),
            endDate: new Date("2024-01-20"),
          },
        ],
      };

      expect(getTeacherStatus(teacher)).toBe("Vertretung: Current Group");
    });

    it("should use group name from substitution", () => {
      const teacher: TeacherAvailability = {
        id: "1",
        firstName: "John",
        lastName: "Doe",
        inSubstitution: true,
        substitutionCount: 1,
        substitutions: [
          {
            id: "1",
            groupId: "100",
            groupName: "Group X",
            isTransfer: false,
            startDate: new Date("2024-01-15"),
            endDate: new Date("2024-01-20"),
          },
        ],
      };

      expect(getTeacherStatus(teacher)).toBe("Vertretung: Group X");
    });

    it("should return multiple assignments status", () => {
      const teacher = {
        id: "1",
        firstName: "John",
        lastName: "Doe",
        inSubstitution: true,
        substitutionCount: 3,
        substitutions: [
          {
            id: "1",
            groupId: "100",
            groupName: "Group A",
            isTransfer: true,
            startDate: new Date("2024-01-15"),
            endDate: new Date("2024-01-15"),
          },
          {
            id: "2",
            groupId: "200",
            groupName: "Group B",
            isTransfer: false,
            startDate: new Date("2024-01-15"),
            endDate: new Date("2024-01-20"),
          },
          {
            id: "3",
            groupId: "300",
            groupName: "Group C",
            isTransfer: false,
            startDate: new Date("2024-01-16"),
            endDate: new Date("2024-01-18"),
          },
        ],
      };

      expect(getTeacherStatus(teacher)).toBe("3 Zuweisungen aktiv");
    });
  });

  describe("getSubstitutionCounts", () => {
    it("should count transfers and substitutions correctly", () => {
      const teacher = {
        id: "1",
        firstName: "John",
        lastName: "Doe",
        inSubstitution: true,
        substitutionCount: 4,
        substitutions: [
          {
            id: "1",
            groupId: "100",
            groupName: "Group A",
            isTransfer: true,
            startDate: new Date("2024-01-15"),
            endDate: new Date("2024-01-15"),
          },
          {
            id: "2",
            groupId: "200",
            groupName: "Group B",
            isTransfer: false,
            startDate: new Date("2024-01-15"),
            endDate: new Date("2024-01-20"),
          },
          {
            id: "3",
            groupId: "300",
            groupName: "Group C",
            isTransfer: true,
            startDate: new Date("2024-01-16"),
            endDate: new Date("2024-01-16"),
          },
          {
            id: "4",
            groupId: "400",
            groupName: "Group D",
            isTransfer: false,
            startDate: new Date("2024-01-17"),
            endDate: new Date("2024-01-19"),
          },
        ],
      };

      const result = getSubstitutionCounts(teacher);

      expect(result).toEqual({
        transfers: 2,
        substitutions: 2,
      });
    });

    it("should handle all transfers", () => {
      const teacher = {
        id: "1",
        firstName: "John",
        lastName: "Doe",
        inSubstitution: true,
        substitutionCount: 2,
        substitutions: [
          {
            id: "1",
            groupId: "100",
            groupName: "Group A",
            isTransfer: true,
            startDate: new Date("2024-01-15"),
            endDate: new Date("2024-01-15"),
          },
          {
            id: "2",
            groupId: "200",
            groupName: "Group B",
            isTransfer: true,
            startDate: new Date("2024-01-16"),
            endDate: new Date("2024-01-16"),
          },
        ],
      };

      const result = getSubstitutionCounts(teacher);

      expect(result).toEqual({
        transfers: 2,
        substitutions: 0,
      });
    });

    it("should handle no substitutions", () => {
      const teacher = {
        id: "1",
        firstName: "John",
        lastName: "Doe",
        inSubstitution: false,
        substitutionCount: 0,
        substitutions: [],
      };

      const result = getSubstitutionCounts(teacher);

      expect(result).toEqual({
        transfers: 0,
        substitutions: 0,
      });
    });
  });
});
