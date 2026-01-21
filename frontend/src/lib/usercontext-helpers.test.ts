import { describe, it, expect } from "vitest";
import {
  mapEducationalGroupResponse,
  mapActivityGroupResponse,
  mapActiveGroupResponse,
  mapUserProfileResponse,
  mapPersonResponse,
  mapStaffResponse,
  mapTeacherResponse,
  type BackendEducationalGroup,
  type BackendActivityGroup,
  type BackendActiveGroup,
  type BackendUserProfile,
  type BackendPerson,
  type BackendStaff,
  type BackendTeacher,
} from "./usercontext-helpers";

describe("usercontext-helpers", () => {
  describe("mapEducationalGroupResponse", () => {
    it("maps all fields from backend to frontend format", () => {
      const backendData: BackendEducationalGroup = {
        id: 1,
        name: "Class 1A",
        room_id: 10,
        room: {
          id: 10,
          name: "Room 101",
        },
        via_substitution: false,
      };

      const result = mapEducationalGroupResponse(backendData);

      expect(result).toEqual({
        id: "1",
        name: "Class 1A",
        room_id: "10",
        room: {
          id: "10",
          name: "Room 101",
        },
        viaSubstitution: false,
      });
    });

    it("converts numeric id to string", () => {
      const backendData: BackendEducationalGroup = {
        id: 12345,
        name: "Test Group",
      };

      const result = mapEducationalGroupResponse(backendData);

      expect(result.id).toBe("12345");
      expect(typeof result.id).toBe("string");
    });

    it("handles group without room", () => {
      const backendData: BackendEducationalGroup = {
        id: 1,
        name: "Outdoor Group",
        room_id: undefined,
        room: undefined,
      };

      const result = mapEducationalGroupResponse(backendData);

      expect(result.room_id).toBeUndefined();
      expect(result.room).toBeUndefined();
    });

    it("sets viaSubstitution to false when undefined", () => {
      const backendData: BackendEducationalGroup = {
        id: 1,
        name: "Group",
        via_substitution: undefined,
      };

      const result = mapEducationalGroupResponse(backendData);

      expect(result.viaSubstitution).toBe(false);
    });

    it("preserves viaSubstitution when true", () => {
      const backendData: BackendEducationalGroup = {
        id: 1,
        name: "Substitute Group",
        via_substitution: true,
      };

      const result = mapEducationalGroupResponse(backendData);

      expect(result.viaSubstitution).toBe(true);
    });

    it("handles room_id without room object", () => {
      const backendData: BackendEducationalGroup = {
        id: 1,
        name: "Group with room_id only",
        room_id: 5,
      };

      const result = mapEducationalGroupResponse(backendData);

      expect(result.room_id).toBe("5");
      expect(result.room).toBeUndefined();
    });
  });

  describe("mapActivityGroupResponse", () => {
    it("maps all fields from backend to frontend format", () => {
      const backendData: BackendActivityGroup = {
        id: 50,
        name: "Chess Club",
        room_id: 20,
        room: {
          id: 20,
          name: "Activity Room A",
        },
      };

      const result = mapActivityGroupResponse(backendData);

      expect(result).toEqual({
        id: "50",
        name: "Chess Club",
        room_id: "20",
        room: {
          id: "20",
          name: "Activity Room A",
        },
      });
    });

    it("converts numeric id to string", () => {
      const backendData: BackendActivityGroup = {
        id: 99999,
        name: "Large ID Group",
      };

      const result = mapActivityGroupResponse(backendData);

      expect(result.id).toBe("99999");
      expect(typeof result.id).toBe("string");
    });

    it("handles activity group without room", () => {
      const backendData: BackendActivityGroup = {
        id: 1,
        name: "Field Trip Group",
      };

      const result = mapActivityGroupResponse(backendData);

      expect(result.room_id).toBeUndefined();
      expect(result.room).toBeUndefined();
    });

    it("handles null room values", () => {
      const backendData = {
        id: 1,
        name: "Test Group",
        room_id: null,
        room: null,
      } as unknown as BackendActivityGroup;

      const result = mapActivityGroupResponse(backendData);

      expect(result.room_id).toBeUndefined();
      expect(result.room).toBeUndefined();
    });
  });

  describe("mapActiveGroupResponse", () => {
    it("maps all fields from backend to frontend format", () => {
      const backendData: BackendActiveGroup = {
        id: 100,
        name: "Morning Session",
        room_id: 30,
        room: {
          id: 30,
          name: "Main Hall",
        },
      };

      const result = mapActiveGroupResponse(backendData);

      expect(result).toEqual({
        id: "100",
        name: "Morning Session",
        room_id: "30",
        room: {
          id: "30",
          name: "Main Hall",
        },
      });
    });

    it("converts numeric id to string", () => {
      const backendData: BackendActiveGroup = {
        id: 42,
        name: "Active Group",
      };

      const result = mapActiveGroupResponse(backendData);

      expect(result.id).toBe("42");
      expect(typeof result.id).toBe("string");
    });

    it("handles active group without room", () => {
      const backendData: BackendActiveGroup = {
        id: 1,
        name: "Outdoor Activity",
      };

      const result = mapActiveGroupResponse(backendData);

      expect(result.room_id).toBeUndefined();
      expect(result.room).toBeUndefined();
    });
  });

  describe("mapUserProfileResponse", () => {
    it("maps all fields from backend to frontend format", () => {
      const backendData: BackendUserProfile = {
        id: 200,
        email: "teacher@school.com",
        username: "teacher1",
        name: "John Teacher",
        active: true,
      };

      const result = mapUserProfileResponse(backendData);

      expect(result).toEqual({
        id: "200",
        email: "teacher@school.com",
        username: "teacher1",
        name: "John Teacher",
        active: true,
      });
    });

    it("converts numeric id to string", () => {
      const backendData: BackendUserProfile = {
        id: 1234567,
        email: "user@test.com",
        username: "testuser",
        name: "Test User",
        active: true,
      };

      const result = mapUserProfileResponse(backendData);

      expect(result.id).toBe("1234567");
      expect(typeof result.id).toBe("string");
    });

    it("handles inactive user", () => {
      const backendData: BackendUserProfile = {
        id: 1,
        email: "inactive@test.com",
        username: "inactive",
        name: "Inactive User",
        active: false,
      };

      const result = mapUserProfileResponse(backendData);

      expect(result.active).toBe(false);
    });

    it("preserves all string fields", () => {
      const backendData: BackendUserProfile = {
        id: 1,
        email: "special+chars@test.com",
        username: "user_with_underscore",
        name: "Name with Ümläüts",
        active: true,
      };

      const result = mapUserProfileResponse(backendData);

      expect(result.email).toBe("special+chars@test.com");
      expect(result.username).toBe("user_with_underscore");
      expect(result.name).toBe("Name with Ümläüts");
    });
  });

  describe("mapPersonResponse", () => {
    it("maps all fields from backend to frontend format", () => {
      const backendData: BackendPerson = {
        id: 300,
        first_name: "Anna",
        last_name: "Schmidt",
        date_of_birth: "1985-03-15",
      };

      const result = mapPersonResponse(backendData);

      expect(result).toEqual({
        id: "300",
        first_name: "Anna",
        last_name: "Schmidt",
        date_of_birth: "1985-03-15",
      });
    });

    it("converts numeric id to string", () => {
      const backendData: BackendPerson = {
        id: 999,
        first_name: "Test",
        last_name: "Person",
      };

      const result = mapPersonResponse(backendData);

      expect(result.id).toBe("999");
      expect(typeof result.id).toBe("string");
    });

    it("handles person without date of birth", () => {
      const backendData: BackendPerson = {
        id: 1,
        first_name: "Unknown",
        last_name: "Birthday",
        date_of_birth: undefined,
      };

      const result = mapPersonResponse(backendData);

      expect(result.date_of_birth).toBeUndefined();
    });

    it("preserves date format", () => {
      const backendData: BackendPerson = {
        id: 1,
        first_name: "Test",
        last_name: "User",
        date_of_birth: "2000-12-31",
      };

      const result = mapPersonResponse(backendData);

      expect(result.date_of_birth).toBe("2000-12-31");
    });
  });

  describe("mapStaffResponse", () => {
    it("maps all fields from backend to frontend format", () => {
      const backendData: BackendStaff = {
        id: 400,
        person_id: 300,
        phone: "030-12345678",
        email: "staff@school.com",
        emergency_contact: "Spouse Name",
        emergency_phone: "0170-87654321",
        person: {
          id: 300,
          first_name: "Staff",
          last_name: "Member",
          date_of_birth: "1980-06-20",
        },
      };

      const result = mapStaffResponse(backendData);

      expect(result).toEqual({
        id: "400",
        person_id: "300",
        phone: "030-12345678",
        email: "staff@school.com",
        emergency_contact: "Spouse Name",
        emergency_phone: "0170-87654321",
        person: {
          id: "300",
          first_name: "Staff",
          last_name: "Member",
          date_of_birth: "1980-06-20",
        },
      });
    });

    it("converts numeric ids to strings", () => {
      const backendData: BackendStaff = {
        id: 123,
        person_id: 456,
      };

      const result = mapStaffResponse(backendData);

      expect(result.id).toBe("123");
      expect(result.person_id).toBe("456");
      expect(typeof result.id).toBe("string");
      expect(typeof result.person_id).toBe("string");
    });

    it("handles staff without person data", () => {
      const backendData: BackendStaff = {
        id: 1,
        person_id: 2,
        person: undefined,
      };

      const result = mapStaffResponse(backendData);

      expect(result.person).toBeUndefined();
    });

    it("handles staff without optional fields", () => {
      const backendData: BackendStaff = {
        id: 1,
        person_id: 2,
        phone: undefined,
        email: undefined,
        emergency_contact: undefined,
        emergency_phone: undefined,
      };

      const result = mapStaffResponse(backendData);

      expect(result.phone).toBeUndefined();
      expect(result.email).toBeUndefined();
      expect(result.emergency_contact).toBeUndefined();
      expect(result.emergency_phone).toBeUndefined();
    });

    it("maps nested person correctly", () => {
      const backendData: BackendStaff = {
        id: 1,
        person_id: 2,
        person: {
          id: 2,
          first_name: "Nested",
          last_name: "Person",
          date_of_birth: "1990-01-01",
        },
      };

      const result = mapStaffResponse(backendData);

      expect(result.person).toBeDefined();
      expect(result.person?.id).toBe("2");
      expect(result.person?.first_name).toBe("Nested");
      expect(result.person?.last_name).toBe("Person");
    });
  });

  describe("mapTeacherResponse", () => {
    it("maps all fields from backend to frontend format", () => {
      const backendData: BackendTeacher = {
        id: 500,
        staff_id: 400,
        staff: {
          id: 400,
          person_id: 300,
          phone: "030-12345678",
          email: "teacher@school.com",
          person: {
            id: 300,
            first_name: "Teacher",
            last_name: "Name",
          },
        },
      };

      const result = mapTeacherResponse(backendData);

      expect(result).toEqual({
        id: "500",
        staff_id: "400",
        staff: {
          id: "400",
          person_id: "300",
          phone: "030-12345678",
          email: "teacher@school.com",
          emergency_contact: undefined,
          emergency_phone: undefined,
          person: {
            id: "300",
            first_name: "Teacher",
            last_name: "Name",
            date_of_birth: undefined,
          },
        },
      });
    });

    it("converts numeric ids to strings", () => {
      const backendData: BackendTeacher = {
        id: 789,
        staff_id: 456,
      };

      const result = mapTeacherResponse(backendData);

      expect(result.id).toBe("789");
      expect(result.staff_id).toBe("456");
      expect(typeof result.id).toBe("string");
      expect(typeof result.staff_id).toBe("string");
    });

    it("handles teacher without staff data", () => {
      const backendData: BackendTeacher = {
        id: 1,
        staff_id: 2,
        staff: undefined,
      };

      const result = mapTeacherResponse(backendData);

      expect(result.staff).toBeUndefined();
    });

    it("maps deeply nested structure correctly", () => {
      const backendData: BackendTeacher = {
        id: 1,
        staff_id: 2,
        staff: {
          id: 2,
          person_id: 3,
          email: "deep@test.com",
          person: {
            id: 3,
            first_name: "Deep",
            last_name: "Nested",
            date_of_birth: "1975-05-05",
          },
        },
      };

      const result = mapTeacherResponse(backendData);

      expect(result.staff).toBeDefined();
      expect(result.staff?.person).toBeDefined();
      expect(result.staff?.person?.id).toBe("3");
      expect(result.staff?.person?.first_name).toBe("Deep");
      expect(result.staff?.person?.date_of_birth).toBe("1975-05-05");
    });

    it("handles staff with person undefined", () => {
      const backendData: BackendTeacher = {
        id: 1,
        staff_id: 2,
        staff: {
          id: 2,
          person_id: 3,
          person: undefined,
        },
      };

      const result = mapTeacherResponse(backendData);

      expect(result.staff).toBeDefined();
      expect(result.staff?.person).toBeUndefined();
    });
  });
});
