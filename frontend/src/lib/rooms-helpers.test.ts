import { describe, it, expect } from "vitest";
import {
  mapRoomResponse,
  mapRoomsResponse,
  createRoomIdToNameMap,
  type BackendRoom,
  type Room,
} from "./rooms-helpers";

describe("rooms-helpers", () => {
  describe("mapRoomResponse", () => {
    it("maps all fields from backend to frontend format", () => {
      const backendRoom: BackendRoom = {
        id: 1,
        name: "Room 101",
        building: "Building A",
        floor: 2,
        capacity: 30,
        category: "classroom",
        color: "#FF5733",
        created_at: "2024-01-15T10:00:00Z",
        updated_at: "2024-01-20T15:30:00Z",
      };

      const result = mapRoomResponse(backendRoom);

      expect(result).toEqual({
        id: "1",
        name: "Room 101",
        building: "Building A",
        floor: 2,
        capacity: 30,
        category: "classroom",
        color: "#FF5733",
        created_at: "2024-01-15T10:00:00Z",
        updated_at: "2024-01-20T15:30:00Z",
      });
    });

    it("converts numeric id to string", () => {
      const backendRoom: BackendRoom = {
        id: 12345,
        name: "Test Room",
        floor: 1,
        capacity: 20,
        category: "office",
        color: "#000000",
      };

      const result = mapRoomResponse(backendRoom);

      expect(result.id).toBe("12345");
      expect(typeof result.id).toBe("string");
    });

    it("handles optional building field when undefined", () => {
      const backendRoom: BackendRoom = {
        id: 1,
        name: "Room without building",
        building: undefined,
        floor: 0,
        capacity: 10,
        category: "storage",
        color: "#CCCCCC",
      };

      const result = mapRoomResponse(backendRoom);

      expect(result.building).toBeUndefined();
    });

    it("handles optional timestamps when undefined", () => {
      const backendRoom: BackendRoom = {
        id: 1,
        name: "Room",
        floor: 1,
        capacity: 5,
        category: "small",
        color: "#FFFFFF",
        created_at: undefined,
        updated_at: undefined,
      };

      const result = mapRoomResponse(backendRoom);

      expect(result.created_at).toBeUndefined();
      expect(result.updated_at).toBeUndefined();
    });

    it("preserves zero values correctly", () => {
      const backendRoom: BackendRoom = {
        id: 0,
        name: "Ground floor room",
        floor: 0,
        capacity: 0,
        category: "unused",
        color: "#000000",
      };

      const result = mapRoomResponse(backendRoom);

      expect(result.id).toBe("0");
      expect(result.floor).toBe(0);
      expect(result.capacity).toBe(0);
    });
  });

  describe("mapRoomsResponse", () => {
    it("maps an array of backend rooms to frontend format", () => {
      const backendRooms: BackendRoom[] = [
        {
          id: 1,
          name: "Room 1",
          floor: 1,
          capacity: 20,
          category: "classroom",
          color: "#FF0000",
        },
        {
          id: 2,
          name: "Room 2",
          building: "Building B",
          floor: 2,
          capacity: 30,
          category: "lab",
          color: "#00FF00",
        },
      ];

      const result = mapRoomsResponse(backendRooms);

      expect(result).toHaveLength(2);
      expect(result[0]!.id).toBe("1");
      expect(result[0]!.name).toBe("Room 1");
      expect(result[1]!.id).toBe("2");
      expect(result[1]!.building).toBe("Building B");
    });

    it("returns empty array for empty input", () => {
      const result = mapRoomsResponse([]);

      expect(result).toEqual([]);
    });

    it("handles single room in array", () => {
      const backendRooms: BackendRoom[] = [
        {
          id: 99,
          name: "Single Room",
          floor: 5,
          capacity: 50,
          category: "auditorium",
          color: "#0000FF",
        },
      ];

      const result = mapRoomsResponse(backendRooms);

      expect(result).toHaveLength(1);
      expect(result[0]!.id).toBe("99");
    });
  });

  describe("createRoomIdToNameMap", () => {
    it("creates a map of room ids to names", () => {
      const rooms: Room[] = [
        {
          id: "1",
          name: "Room Alpha",
          floor: 1,
          capacity: 20,
          category: "classroom",
          color: "#FF0000",
        },
        {
          id: "2",
          name: "Room Beta",
          floor: 2,
          capacity: 25,
          category: "lab",
          color: "#00FF00",
        },
        {
          id: "3",
          name: "Room Gamma",
          floor: 3,
          capacity: 30,
          category: "office",
          color: "#0000FF",
        },
      ];

      const result = createRoomIdToNameMap(rooms);

      expect(result).toEqual({
        "1": "Room Alpha",
        "2": "Room Beta",
        "3": "Room Gamma",
      });
    });

    it("returns empty object for empty array", () => {
      const result = createRoomIdToNameMap([]);

      expect(result).toEqual({});
    });

    it("handles single room", () => {
      const rooms: Room[] = [
        {
          id: "42",
          name: "The Answer",
          floor: 4,
          capacity: 42,
          category: "special",
          color: "#424242",
        },
      ];

      const result = createRoomIdToNameMap(rooms);

      expect(result).toEqual({
        "42": "The Answer",
      });
    });

    it("overwrites duplicate ids with last value", () => {
      const rooms: Room[] = [
        {
          id: "1",
          name: "First Name",
          floor: 1,
          capacity: 10,
          category: "a",
          color: "#111111",
        },
        {
          id: "1",
          name: "Second Name",
          floor: 1,
          capacity: 10,
          category: "a",
          color: "#111111",
        },
      ];

      const result = createRoomIdToNameMap(rooms);

      expect(result["1"]).toBe("Second Name");
      expect(Object.keys(result)).toHaveLength(1);
    });

    it("allows quick lookup of room names by id", () => {
      const rooms: Room[] = [
        {
          id: "100",
          name: "Conference Room",
          floor: 1,
          capacity: 15,
          category: "meeting",
          color: "#AABBCC",
        },
        {
          id: "200",
          name: "Break Room",
          floor: 1,
          capacity: 8,
          category: "common",
          color: "#DDEEFF",
        },
      ];

      const roomMap = createRoomIdToNameMap(rooms);

      // Simulate looking up room name for a student's location
      const studentRoomId = "100";
      const roomName = roomMap[studentRoomId];

      expect(roomName).toBe("Conference Room");
    });
  });
});
