/**
 * Tests for Rooms Configuration
 * Tests room config structure and validation
 */
import { describe, it, expect, vi } from "vitest";
import { roomsConfig } from "./rooms.config";
import type { Room } from "@/lib/room-helpers";

// Mock room-helpers
vi.mock("@/lib/room-helpers", () => ({
  mapRoomResponse: vi.fn((data: unknown) => data),
  prepareRoomForBackend: vi.fn((data: unknown) => data),
}));

describe("roomsConfig", () => {
  it("exports a valid entity config", () => {
    expect(roomsConfig).toBeDefined();
    expect(roomsConfig.name).toEqual({
      singular: "Raum",
      plural: "Räume",
    });
  });

  it("has correct API configuration", () => {
    expect(roomsConfig.api.basePath).toBe("/api/rooms");
  });

  it("has form sections configured", () => {
    expect(roomsConfig.form.sections).toHaveLength(1);
    expect(roomsConfig.form.sections[0]?.title).toBe("Raumdetails");
  });

  it("has required form fields", () => {
    const fields = roomsConfig.form.sections[0]?.fields ?? [];
    const fieldNames = fields.map((f) => f.name);

    expect(fieldNames).toContain("name");
    expect(fieldNames).toContain("category");
    expect(fieldNames).toContain("building");
    expect(fieldNames).toContain("floor");
  });

  it("has default color value", () => {
    expect(roomsConfig.form.defaultValues?.color).toBe("#4F46E5");
  });

  it("transforms data before submit", () => {
    const data = {
      name: "  Room 101  ",
      floor: "2",
      color: undefined,
    } as unknown as Partial<Room>;

    const transformed = roomsConfig.form.transformBeforeSubmit?.(data);
    expect(transformed?.name).toBe("Room 101");
    expect(transformed?.floor).toBe(2);
    expect(transformed?.color).toBe("#4F46E5");
  });

  it("validates floor field", () => {
    const fields = roomsConfig.form.sections[0]?.fields ?? [];
    const floorField = fields.find((f) => f.name === "floor");

    const validation = floorField?.validation?.("invalid");
    expect(validation).toBe("Bitte geben Sie eine gültige Etage ein");
  });

  it("accepts valid floor value", () => {
    const fields = roomsConfig.form.sections[0]?.fields ?? [];
    const floorField = fields.find((f) => f.name === "floor");

    const validation = floorField?.validation?.("2");
    expect(validation).toBeNull();
  });

  it("has detail header configuration", () => {
    const mockRoom: Room = {
      id: "1",
      name: "Room 101",
      category: "Normaler Raum",
      building: "Building A",
      floor: 2,
      isOccupied: false,
    };

    expect(roomsConfig.detail.header?.title(mockRoom)).toBe("Room 101");
    expect(roomsConfig.detail.header?.subtitle?.(mockRoom)).toBe(
      "Building A, Etage 2",
    );
  });

  it("shows only building when floor not set", () => {
    const mockRoom: Room = {
      id: "1",
      name: "Room 101",
      category: "Normaler Raum",
      building: "Building A",
      isOccupied: false,
    };

    expect(roomsConfig.detail.header?.subtitle?.(mockRoom)).toBe("Building A");
  });

  it("shows only floor when building not set", () => {
    const mockRoom: Room = {
      id: "1",
      name: "Room 101",
      category: "Normaler Raum",
      floor: 2,
      isOccupied: false,
    };

    expect(roomsConfig.detail.header?.subtitle?.(mockRoom)).toBe("Etage 2");
  });

  it("has list configuration", () => {
    expect(roomsConfig.list.title).toBe("Raum auswählen");
    expect(roomsConfig.list.searchStrategy).toBe("frontend");
  });

  it("displays occupied status in list", () => {
    const mockRoom: Room = {
      id: "1",
      name: "Room 101",
      category: "Normaler Raum",
      isOccupied: true,
      groupName: "Group Blue",
    };

    const subtitle = roomsConfig.list.item.subtitle?.(mockRoom);
    expect(subtitle).toContain("Belegt");
    expect(subtitle).toContain("Group Blue");
  });

  it("displays free status in list", () => {
    const mockRoom: Room = {
      id: "1",
      name: "Room 101",
      category: "Normaler Raum",
      isOccupied: false,
    };

    const subtitle = roomsConfig.list.item.subtitle?.(mockRoom);
    expect(subtitle).toBe("Frei");
  });

  it("shows category-specific emoji", () => {
    const mockRoom: Room = {
      id: "1",
      name: "Room 101",
      category: "Sport",
      isOccupied: false,
    };

    const emoji = roomsConfig.list.item.avatar?.text(mockRoom);
    expect(emoji).toBe("🏃");
  });

  it("shows occupied badge when room is occupied", () => {
    const mockRoom: Room = {
      id: "1",
      name: "Room 101",
      category: "Normaler Raum",
      isOccupied: true,
    };

    const badges = roomsConfig.list.item.badges ?? [];
    const occupiedBadge = badges.find((b) => b.label === "Belegt");
    expect(occupiedBadge?.showWhen!(mockRoom)).toBe(true);
  });

  it("shows free badge when room is not occupied", () => {
    const mockRoom: Room = {
      id: "1",
      name: "Room 101",
      category: "Normaler Raum",
      isOccupied: false,
    };

    const badges = roomsConfig.list.item.badges ?? [];
    const freeBadge = badges.find((b) => b.label === "Frei");
    expect(freeBadge?.showWhen!(mockRoom)).toBe(true);
  });

  it("has custom labels", () => {
    expect(roomsConfig.labels?.createButton).toBe("Neuen Raum erstellen");
    expect(roomsConfig.labels?.deleteConfirmation).toContain("löschen");
  });
});
