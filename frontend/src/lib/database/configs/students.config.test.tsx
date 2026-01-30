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
      singular: "Schüler",
      plural: "Schüler",
    });
  });

  it("has correct API configuration", () => {
    expect(studentsConfig.api.basePath).toBe("/api/students");
  });

  it("has form sections configured", () => {
    expect(studentsConfig.form.sections.length).toBeGreaterThan(0);
  });

  it("has list configuration", () => {
    expect(studentsConfig.list.title).toBe("Schüler auswählen");
    expect(studentsConfig.list.searchStrategy).toBe("frontend");
  });

  it("has custom labels", () => {
    expect(studentsConfig.labels?.createButton).toBe("Neuen Schüler erstellen");
  });
});
