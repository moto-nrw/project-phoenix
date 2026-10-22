import { describe, it, expect, vi } from "vitest";
import type { BackendRoom, Room } from "./room-helpers";
import {
  mapRoomResponse,
  mapRoomsResponse,
  mapSingleRoomResponse,
  prepareRoomForBackend,
  isColorLockedRoom,
  isSystemRoom,
  formatFloor,
} from "./room-helpers";
import { suppressConsole } from "~/test/helpers/console";
import { buildBackendRoom } from "~/test/fixtures/rooms";

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

  it("treats a legacy zero capacity as no limit", () => {
    const result = mapRoomResponse({
      ...sampleBackendRoom,
      capacity: 0,
    });

    expect(result.capacity).toBeUndefined();
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
      capacity: 43,
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
    expect(result.capacity).toBe(43);
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
    expect(result.capacity).toBeUndefined();
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

  it("maps an empty capacity to null so an existing limit is cleared", () => {
    const result = prepareRoomForBackend({
      name: "Room without limit",
      capacity: "" as unknown as number,
    });

    expect(result.capacity).toBeNull();
  });

  it("handles room without id (for creation)", () => {
    const result = prepareRoomForBackend({
      name: "New Room",
    });

    expect(result.id).toBeUndefined();
  });
});

describe("formatFloor", () => {
  it.each([
    [2, "Etage 2"],
    [0, "Erdgeschoss"],
    [undefined, "Etage nicht angegeben"],
  ])("formats floor %j as %j", (floor, expected) => {
    expect(formatFloor(floor)).toBe(expected);
  });
});

describe("isColorLockedRoom", () => {
  // Issue #2405 — the Schulhof left the colour-locked set: schools colour-code
  // rooms and tablets and need the yard in that scheme. Only the toilet rooms
  // keep the frozen colour (they have no badge of their own).
  it("locks the WC", () => {
    expect(isColorLockedRoom({ id: "1", name: "WC" } as never)).toBe(true);
  });

  it("locks the Toilette alias", () => {
    expect(isColorLockedRoom({ id: "1", name: "Toilette" } as never)).toBe(
      true,
    );
  });

  it("does NOT lock the Schulhof", () => {
    expect(isColorLockedRoom({ id: "1", name: "Schulhof" } as never)).toBe(
      false,
    );
  });

  it("does not lock a regular room", () => {
    expect(isColorLockedRoom({ id: "1", name: "Werkraum" } as never)).toBe(
      false,
    );
  });

  it("returns false for null/undefined", () => {
    expect(isColorLockedRoom(null)).toBe(false);
    expect(isColorLockedRoom(undefined)).toBe(false);
  });
});

describe("isSystemRoom", () => {
  it("returns true for Schulhof", () => {
    const room: Room = { id: "1", name: "Schulhof", isOccupied: false };
    expect(isSystemRoom(room)).toBe(true);
  });

  it("returns true for WC", () => {
    const room: Room = { id: "2", name: "WC", isOccupied: false };
    expect(isSystemRoom(room)).toBe(true);
  });

  it("returns true for Toilette", () => {
    const room: Room = { id: "3", name: "Toilette", isOccupied: false };
    expect(isSystemRoom(room)).toBe(true);
  });

  it("returns false for regular room", () => {
    const room: Room = { id: "4", name: "Gruppenraum 1", isOccupied: false };
    expect(isSystemRoom(room)).toBe(false);
  });

  it("returns false for null", () => {
    expect(isSystemRoom(null)).toBe(false);
  });

  it("returns false for undefined", () => {
    expect(isSystemRoom(undefined)).toBe(false);
  });
});
