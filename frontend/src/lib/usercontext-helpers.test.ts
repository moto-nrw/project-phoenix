import { describe, it, expect } from "vitest";
import {
  mapEducationalGroupResponse,
  mapNavigationEducationalGroupResponse,
  mapActiveGroupResponse,
  mapPersonResponse,
  mapStaffResponse,
  type BackendEducationalGroup,
  type NavigationEducationalGroup,
  type BackendActiveGroup,
  type BackendPerson,
  type BackendStaff,
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
        roomId: "10",
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

      expect(result.roomId).toBeUndefined();
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

      expect(result.roomId).toBe("5");
      expect(result.room).toBeUndefined();
    });
  });

  describe("mapNavigationEducationalGroupResponse", () => {
    it("preserves string IDs and builds room data from the navigation view", () => {
      const navigationGroup: NavigationEducationalGroup = {
        id: "12345",
        name: "Eulen",
        room_id: "77",
        room_name: "Blauer Raum",
        via_substitution: true,
        is_personal: true,
      };

      expect(mapNavigationEducationalGroupResponse(navigationGroup)).toEqual({
        id: "12345",
        name: "Eulen",
        roomId: "77",
        room: { id: "77", name: "Blauer Raum" },
        viaSubstitution: true,
      });
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
});
