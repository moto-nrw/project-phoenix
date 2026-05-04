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
    // Color picker is part of the form (Issue #1324) so admins can override
    // the default OTHER_ROOM blue per room.
    expect(fieldNames).toContain("color");
  });

  it("does not force a default color (Issue #1324)", () => {
    // The previous "#4F46E5" default forced every saved room into the same
    // hue, defeating the whole point of the color picker. New rooms now
    // start with no color and the badge falls back to OTHER_ROOM blue until
    // an admin picks one.
    expect(roomsConfig.form.defaultValues?.color).toBeUndefined();
  });

  it("preserves color through transformBeforeSubmit", () => {
    // Whatever the picker emits — a hex, null, or undefined — must reach
    // the backend untouched. Forcing a default here would clobber the
    // user's "Zurücksetzen" action and re-introduce the all-blue bug.
    const explicit = roomsConfig.form.transformBeforeSubmit?.({
      name: "Room 101",
      color: "#A3D977",
    } as Partial<Room>);
    expect(explicit?.color).toBe("#A3D977");

    const cleared = roomsConfig.form.transformBeforeSubmit?.({
      name: "Room 101",
      color: null,
    } as unknown as Partial<Room>);
    expect(cleared?.color).toBeNull();

    const omitted = roomsConfig.form.transformBeforeSubmit?.({
      name: "Room 101",
      color: undefined,
    } as unknown as Partial<Room>);
    expect(omitted?.color).toBeUndefined();
  });

  it("transforms name and floor before submit", () => {
    const data = {
      name: "  Room 101  ",
      floor: "2",
    } as unknown as Partial<Room>;

    const transformed = roomsConfig.form.transformBeforeSubmit?.(data);
    expect(transformed?.name).toBe("Room 101");
    expect(transformed?.floor).toBe(2);
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
