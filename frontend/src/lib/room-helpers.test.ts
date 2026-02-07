import { describe, it, expect, vi } from "vitest";
import type { BackendRoom, Room } from "./room-helpers";
import {
  mapRoomResponse,
  mapRoomsResponse,
  mapSingleRoomResponse,
  prepareRoomForBackend,
  formatRoomName,
  formatRoomLocation,
  formatRoomCategory,
  formatRoomCapacity,
  getRoomUtilization,
  getRoomStatusColor,
} from "./room-helpers";
import { suppressConsole } from "~/test/helpers/console";
import { buildBackendRoom } from "~/test/fixtures";

// Sample backend room for testing
const sampleBackendRoom = buildBackendRoom({
  id: 1,
  name: "Room 101",
  building: "Building A",
  floor: 2,
  capacity: 30,
  category: "classroom",
  color: "#FF0000",
  device_id: "device-123",
  is_occupied: true,
  activity_name: "Math Class",
  group_name: "Class 3A",
  supervisor_name: "John Smith",
  student_count: 25,
});

describe("mapRoomResponse", () => {
  it("maps backend room to frontend room structure", () => {
    const result = mapRoomResponse(sampleBackendRoom);

    expect(result.id).toBe("1"); // int64 → string
    expect(result.name).toBe("Room 101");
    expect(result.building).toBe("Building A");
    expect(result.floor).toBe(2);
    expect(result.capacity).toBe(30);
    expect(result.category).toBe("classroom");
    expect(result.color).toBe("#FF0000");
    expect(result.deviceId).toBe("device-123"); // snake_case → camelCase
    expect(result.isOccupied).toBe(true);
    expect(result.activityName).toBe("Math Class");
    expect(result.groupName).toBe("Class 3A");
    expect(result.supervisorName).toBe("John Smith");
    expect(result.studentCount).toBe(25);
    expect(result.createdAt).toBe("2024-01-15T10:00:00Z");
    expect(result.updatedAt).toBe("2024-01-15T12:00:00Z");
  });

  it("converts null values to undefined for optional fields", () => {
    const backendRoom: BackendRoom = {
      ...sampleBackendRoom,
      floor: null,
      capacity: null,
      category: null,
      color: null,
    };

    const result = mapRoomResponse(backendRoom);

    expect(result.floor).toBeUndefined();
    expect(result.capacity).toBeUndefined();
    expect(result.category).toBeUndefined();
    expect(result.color).toBeUndefined();
  });

  it("handles minimal backend room (required fields only)", () => {
    const minimalRoom: BackendRoom = {
      id: 2,
      name: "Minimal Room",
      is_occupied: false,
      created_at: "2024-01-15T10:00:00Z",
      updated_at: "2024-01-15T10:00:00Z",
    };

    const result = mapRoomResponse(minimalRoom);

    expect(result.id).toBe("2");
    expect(result.name).toBe("Minimal Room");
    expect(result.isOccupied).toBe(false);
    expect(result.building).toBeUndefined();
    expect(result.floor).toBeUndefined();
  });
});

describe("mapRoomsResponse", () => {
  const consoleSpies = suppressConsole("warn", "log");

  it("maps array of backend rooms", () => {
    const backendRooms = [sampleBackendRoom, { ...sampleBackendRoom, id: 2 }];

    const result = mapRoomsResponse(backendRooms);

    expect(result).toHaveLength(2);
    expect(result[0]?.id).toBe("1");
    expect(result[1]?.id).toBe("2");
  });

  it("handles nested API response structure { data: [...] }", () => {
    const debugSpy = vi
      .spyOn(console, "debug")
      .mockImplementation(() => undefined);
    const nestedResponse = {
      data: [sampleBackendRoom, { ...sampleBackendRoom, id: 2 }],
    };

    const result = mapRoomsResponse(nestedResponse);

    expect(result).toHaveLength(2);
    expect(debugSpy).toHaveBeenCalledWith(
      "handling nested API response for rooms",
      undefined,
    );
    debugSpy.mockRestore();
  });

  it("returns empty array for null input", () => {
    const result = mapRoomsResponse(null);

    expect(result).toEqual([]);
    expect(consoleSpies.warn).toHaveBeenCalledWith(
      "received invalid response format for rooms",
      { received: "object" },
    );
  });

  it("returns empty array for undefined-like input", () => {
    // TypeScript types prevent passing undefined directly, but runtime might receive it
    const result = mapRoomsResponse(
      undefined as unknown as BackendRoom[] | null,
    );

    expect(result).toEqual([]);
  });

  it("returns empty array for non-array input", () => {
    const result = mapRoomsResponse(
      "invalid" as unknown as BackendRoom[] | null,
    );

    expect(result).toEqual([]);
    expect(consoleSpies.warn).toHaveBeenCalled();
  });

  it("handles empty array", () => {
    const result = mapRoomsResponse([]);

    expect(result).toEqual([]);
  });
});

describe("mapSingleRoomResponse", () => {
  it("extracts and maps room from { data: room } wrapper", () => {
    const response = { data: sampleBackendRoom };

    const result = mapSingleRoomResponse(response);

    expect(result.id).toBe("1");
    expect(result.name).toBe("Room 101");
  });
});

describe("prepareRoomForBackend", () => {
  it("converts frontend room to backend format", () => {
    const frontendRoom: Partial<Room> = {
      id: "1",
      name: "Room 101",
      building: "Building A",
      floor: 2,
      category: "classroom",
      color: "#FF0000",
      deviceId: "device-123",
      isOccupied: true,
    };

    const result = prepareRoomForBackend(frontendRoom);

    expect(result.id).toBe(1); // string → number
    expect(result.name).toBe("Room 101");
    expect(result.building).toBe("Building A");
    expect(result.floor).toBe(2);
    expect(result.category).toBe("classroom");
    expect(result.color).toBe("#FF0000");
    expect(result.device_id).toBe("device-123"); // camelCase → snake_case
    expect(result.is_occupied).toBe(true);
  });

  it("returns empty object when name is empty string", () => {
    const result = prepareRoomForBackend({ name: "" });

    expect(result).toEqual({});
  });

  it("omits optional fields that are undefined", () => {
    const result = prepareRoomForBackend({
      name: "Test Room",
    });

    expect(result.name).toBe("Test Room");
    expect(result.is_occupied).toBe(false); // defaults to false
    expect(result.building).toBeUndefined();
    expect(result.floor).toBeUndefined();
    expect(result.category).toBeUndefined();
  });

  it("omits optional fields that are empty strings", () => {
    const result = prepareRoomForBackend({
      name: "Test Room",
      building: "",
      category: "",
      color: "",
      deviceId: "",
    });

    expect(result.name).toBe("Test Room");
    expect(result.building).toBeUndefined();
    expect(result.category).toBeUndefined();
    expect(result.color).toBeUndefined();
    expect(result.device_id).toBeUndefined();
  });

  it("includes floor when explicitly set to 0", () => {
    const result = prepareRoomForBackend({
      name: "Ground Floor Room",
      floor: 0,
    });

    expect(result.floor).toBe(0);
  });

  it("handles room without id (for creation)", () => {
    const result = prepareRoomForBackend({
      name: "New Room",
    });

    expect(result.id).toBeUndefined();
  });
});

describe("formatRoomName", () => {
  it("returns room name when no building", () => {
    const room: Room = {
      id: "1",
      name: "Room 101",
      isOccupied: false,
    };

    const result = formatRoomName(room);

    expect(result).toBe("Room 101");
  });

  it("prepends building name when available", () => {
    const room: Room = {
      id: "1",
      name: "Room 101",
      building: "Building A",
      isOccupied: false,
    };

    const result = formatRoomName(room);

    expect(result).toBe("Building A - Room 101");
  });
});

describe("formatRoomLocation", () => {
  it("returns floor information when available", () => {
    const room: Room = {
      id: "1",
      name: "Room 101",
      floor: 2,
      isOccupied: false,
    };

    const result = formatRoomLocation(room);

    expect(result).toBe("Etage 2");
  });

  it("returns 'Etage nicht angegeben' when floor is undefined", () => {
    const room: Room = {
      id: "1",
      name: "Room 101",
      isOccupied: false,
    };

    const result = formatRoomLocation(room);

    expect(result).toBe("Etage nicht angegeben");
  });

  it("handles floor 0 (ground floor)", () => {
    const room: Room = {
      id: "1",
      name: "Room 101",
      floor: 0,
      isOccupied: false,
    };

    const result = formatRoomLocation(room);

    expect(result).toBe("Erdgeschoss");
  });
});

describe("formatRoomCategory", () => {
  it("translates known category names", () => {
    const categories = [
      { input: "standard", expected: "Standard" },
      { input: "classroom", expected: "Classroom" },
      { input: "lab", expected: "Laboratory" },
      { input: "gym", expected: "Gymnasium" },
      { input: "cafeteria", expected: "Cafeteria" },
      { input: "office", expected: "Office" },
      { input: "meeting", expected: "Meeting Room" },
      { input: "bathroom", expected: "Bathroom" },
      { input: "storage", expected: "Storage" },
      { input: "other", expected: "Other" },
    ];

    for (const { input, expected } of categories) {
      const room: Room = {
        id: "1",
        name: "Test Room",
        category: input,
        isOccupied: false,
      };

      expect(formatRoomCategory(room)).toBe(expected);
    }
  });

  it("handles case-insensitive category names", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      category: "CLASSROOM",
      isOccupied: false,
    };

    const result = formatRoomCategory(room);

    expect(result).toBe("Classroom");
  });

  it("returns 'Keine Kategorie' when category is undefined", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      isOccupied: false,
    };

    const result = formatRoomCategory(room);

    expect(result).toBe("Keine Kategorie");
  });

  it("returns original value for unknown categories", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      category: "custom-category",
      isOccupied: false,
    };

    const result = formatRoomCategory(room);

    expect(result).toBe("custom-category");
  });
});

describe("formatRoomCapacity", () => {
  it("formats capacity with student count", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      capacity: 30,
      studentCount: 25,
      isOccupied: true,
    };

    const result = formatRoomCapacity(room);

    expect(result).toBe("25/30 students");
  });

  it("shows 0 students when studentCount is undefined", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      capacity: 30,
      isOccupied: false,
    };

    const result = formatRoomCapacity(room);

    expect(result).toBe("0/30 students");
  });

  it("returns 'Kapazität nicht angegeben' when capacity is undefined", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      isOccupied: false,
    };

    const result = formatRoomCapacity(room);

    expect(result).toBe("Kapazität nicht angegeben");
  });
});

describe("getRoomUtilization", () => {
  it("calculates utilization percentage", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      capacity: 100,
      studentCount: 50,
      isOccupied: true,
    };

    const result = getRoomUtilization(room);

    expect(result).toBe(50);
  });

  it("returns 0 when capacity is undefined", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      studentCount: 10,
      isOccupied: true,
    };

    const result = getRoomUtilization(room);

    expect(result).toBe(0);
  });

  it("returns 0 when capacity is zero (division by zero guard)", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      capacity: 0,
      studentCount: 10,
      isOccupied: true,
    };

    const result = getRoomUtilization(room);

    expect(result).toBe(0);
  });

  it("handles missing student count as 0", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      capacity: 30,
      isOccupied: false,
    };

    const result = getRoomUtilization(room);

    expect(result).toBe(0);
  });

  it("calculates correct percentage for full room", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      capacity: 25,
      studentCount: 25,
      isOccupied: true,
    };

    const result = getRoomUtilization(room);

    expect(result).toBe(100);
  });
});

describe("getRoomStatusColor", () => {
  it("returns 'green' for unoccupied rooms", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      capacity: 30,
      studentCount: 25,
      isOccupied: false,
    };

    const result = getRoomStatusColor(room);

    expect(result).toBe("green");
  });

  it("returns 'green' for occupied rooms with low utilization (<50%)", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      capacity: 100,
      studentCount: 40,
      isOccupied: true,
    };

    const result = getRoomStatusColor(room);

    expect(result).toBe("green");
  });

  it("returns 'yellow' for occupied rooms with medium utilization (50-79%)", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      capacity: 100,
      studentCount: 70,
      isOccupied: true,
    };

    const result = getRoomStatusColor(room);

    expect(result).toBe("yellow");
  });

  it("returns 'red' for occupied rooms with high utilization (≥80%)", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      capacity: 100,
      studentCount: 85,
      isOccupied: true,
    };

    const result = getRoomStatusColor(room);

    expect(result).toBe("red");
  });

  it("returns 'green' for occupied room with no capacity defined", () => {
    const room: Room = {
      id: "1",
      name: "Test Room",
      studentCount: 10,
      isOccupied: true,
    };

    const result = getRoomStatusColor(room);

    // No capacity = 0 utilization = green
    expect(result).toBe("green");
  });
});
