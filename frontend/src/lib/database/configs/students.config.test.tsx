/**
 * Tests for Students Configuration
 * Tests student config structure and location helpers
 */
import { describe, it, expect, vi } from "vitest";
import { studentsConfig } from "./students.config";

// Mock location-helper
vi.mock("@/lib/location-helper", () => ({
  LOCATION_STATUSES: {
    HOME: "home",
    PRESENT: "present",
    SCHOOLYARD: "schoolyard",
    TRANSIT: "transit",
  },
  isHomeLocation: vi.fn((loc) => loc === "home"),
  isPresentLocation: vi.fn((loc) => loc === "present"),
  isSchoolyardLocation: vi.fn((loc) => loc === "schoolyard"),
  isTransitLocation: vi.fn((loc) => loc === "transit"),
}));

// Mock dynamic import
vi.mock("next/dynamic", () => ({
  default: vi.fn((_loader: unknown, _options: unknown) => {
    // Return a mock component
    return () => null;
  }),
}));

describe("studentsConfig", () => {
  it("exports a valid entity config", () => {
    expect(studentsConfig).toBeDefined();
    expect(studentsConfig.name).toEqual({
      singular: "Kinder",
      plural: "Kinder",
    });
  });

  it("has correct API configuration", () => {
    expect(studentsConfig.api.basePath).toBe("/api/students");
  });

  it("has form sections configured", () => {
    expect(studentsConfig.form.sections.length).toBeGreaterThan(0);
  });

  it("has list configuration", () => {
    expect(studentsConfig.list.title).toBe("Kinder auswählen");
    expect(studentsConfig.list.searchStrategy).toBe("frontend");
  });

  it("has custom labels", () => {
    expect(studentsConfig.labels?.createButton).toBe("Neues Kind erstellen");
  });

  it("does not forward stale departure_days from the legacy database editor", () => {
    const mapped = studentsConfig.service?.mapRequest?.({
      id: "42",
      first_name: "Max",
      second_name: "Mustermann",
      group_id: "12",
      bus: false,
      bus_days: { mon: true, wed: true },
      departure_days: { mon: "bus", fri: "pickup" },
    });

    expect(mapped).toMatchObject({
      id: "42",
      first_name: "Max",
      second_name: "Mustermann",
      group_id: 12,
      bus: false,
      bus_days: {},
    });
    expect(mapped).not.toHaveProperty("departure_days");
  });
});
