import { describe, it, expect } from "vitest";
import type { Staff } from "./staff-api";
import {
  getStaffLocationStatus,
  getStaffDisplayType,
  getStaffCardInfo,
  formatStaffNotes,
  sortStaff,
} from "./staff-helpers";

// Sample staff for testing
const createSampleStaff = (overrides: Partial<Staff> = {}): Staff => ({
  id: "1",
  name: "Max Mustermann",
  firstName: "Max",
  lastName: "Mustermann",
  role: undefined,
  specialization: "Mathematics",
  qualifications: "M.Ed",
  staffNotes: "Senior staff member",
  hasRfid: true,
  isTeacher: true,
  isSupervising: false,
  currentLocation: "Abwesend",
  supervisionRole: undefined,
  supervisions: [],
  wasPresentToday: false,
  ...overrides,
});

describe("getStaffLocationStatus", () => {
  it("returns Abwesend status with red styling for absent staff", () => {
    const staff = createSampleStaff({ currentLocation: "Abwesend" });
    const result = getStaffLocationStatus(staff);

    expect(result.label).toBe("Abwesend");
    expect(result.customBgColor).toBe("#FF3130");
    expect(result.customShadow).toContain("255, 49, 48");
  });

  it("returns Anwesend status with green styling for present staff", () => {
    const staff = createSampleStaff({ currentLocation: "Anwesend" });
    const result = getStaffLocationStatus(staff);

    expect(result.label).toBe("Anwesend");
    expect(result.customBgColor).toBe("#83CD2D");
    expect(result.customShadow).toContain("131, 205, 45");
    expect(result.badgeColor).toContain("text-white");
  });

  it("returns Homeoffice status with light blue styling", () => {
    const staff = createSampleStaff({ currentLocation: "Homeoffice" });
    const result = getStaffLocationStatus(staff);

    expect(result.label).toBe("Homeoffice");
    expect(result.customBgColor).toBe("#0EA5E9");
    expect(result.customShadow).toContain("14, 165, 233");
  });

  it("returns Krank status with gray styling", () => {
    const staff = createSampleStaff({ currentLocation: "Krank" });
    const result = getStaffLocationStatus(staff);

    expect(result.label).toBe("Krank");
    expect(result.customBgColor).toBe("#6B7280");
    expect(result.customShadow).toContain("107, 114, 128");
  });

  it("returns Urlaub status with gray styling", () => {
    const staff = createSampleStaff({ currentLocation: "Urlaub" });
    const result = getStaffLocationStatus(staff);

    expect(result.label).toBe("Urlaub");
    expect(result.customBgColor).toBe("#6B7280");
  });

  it("returns Fortbildung status with gray styling", () => {
    const staff = createSampleStaff({ currentLocation: "Fortbildung" });
    const result = getStaffLocationStatus(staff);

    expect(result.label).toBe("Fortbildung");
    expect(result.customBgColor).toBe("#6B7280");
  });

  it("returns Anwesend for staff in a specific room (supervising)", () => {
    const staff = createSampleStaff({ currentLocation: "Werkraum" });
    const result = getStaffLocationStatus(staff);

    // Any room location means they're present → Anwesend with green
    expect(result.label).toBe("Anwesend");
    expect(result.customBgColor).toBe("#83CD2D");
  });

  it("defaults to Abwesend when currentLocation is undefined", () => {
    const staff = createSampleStaff({ currentLocation: undefined });
    const result = getStaffLocationStatus(staff);

    expect(result.label).toBe("Abwesend");
    expect(result.customBgColor).toBe("#FF3130");
  });

  it("handles supervising staff in multiple rooms as Anwesend", () => {
    const staff = createSampleStaff({ currentLocation: "2 Räume" });
    const result = getStaffLocationStatus(staff);

    // Any room/supervision location means they're present
    expect(result.label).toBe("Anwesend");
    expect(result.customBgColor).toBe("#83CD2D");
  });
});

describe("getStaffDisplayType", () => {
  it("returns role when set (Admin)", () => {
    const staff = createSampleStaff({
      role: "Admin",
      isTeacher: true,
      specialization: "Mathematics",
    });
    const result = getStaffDisplayType(staff);

    expect(result).toBe("Admin");
  });

  it("returns role when set (Betreuer)", () => {
    const staff = createSampleStaff({
      role: "Betreuer",
      isTeacher: true,
      specialization: "Mathematics",
    });
    const result = getStaffDisplayType(staff);

    expect(result).toBe("Betreuer");
  });

  it("returns role when set (Extern)", () => {
    const staff = createSampleStaff({
      role: "Extern",
      isTeacher: false,
    });
    const result = getStaffDisplayType(staff);

    expect(result).toBe("Extern");
  });

  it("returns specialization for teachers without role", () => {
    const staff = createSampleStaff({
      role: undefined,
      isTeacher: true,
      specialization: "Mathematics",
    });
    const result = getStaffDisplayType(staff);

    expect(result).toBe("Mathematics");
  });

  it("returns Betreuer for teachers without role or specialization", () => {
    const staff = createSampleStaff({
      role: undefined,
      isTeacher: true,
      specialization: undefined,
    });
    const result = getStaffDisplayType(staff);

    expect(result).toBe("Betreuer");
  });

  it("returns Betreuer for non-teachers without role", () => {
    const staff = createSampleStaff({
      role: undefined,
      isTeacher: false,
      specialization: "Some Specialization",
    });
    const result = getStaffDisplayType(staff);

    expect(result).toBe("Betreuer");
  });
});

describe("getStaffCardInfo", () => {
  it("returns empty array when no qualifications or supervision", () => {
    const staff = createSampleStaff({
      qualifications: undefined,
      isSupervising: false,
    });
    const result = getStaffCardInfo(staff);

    expect(result).toEqual([]);
  });

  it("includes qualifications when available", () => {
    const staff = createSampleStaff({ qualifications: "M.Ed" });
    const result = getStaffCardInfo(staff);

    expect(result).toContain("M.Ed");
  });

  it("includes Hauptbetreuer for primary supervision role", () => {
    const staff = createSampleStaff({
      isSupervising: true,
      supervisionRole: "primary",
    });
    const result = getStaffCardInfo(staff);

    expect(result).toContain("Hauptbetreuer");
  });

  it("includes Assistenz for assistant supervision role", () => {
    const staff = createSampleStaff({
      isSupervising: true,
      supervisionRole: "assistant",
    });
    const result = getStaffCardInfo(staff);

    expect(result).toContain("Assistenz");
  });

  it("does not include supervision role when not supervising", () => {
    const staff = createSampleStaff({
      isSupervising: false,
      supervisionRole: "primary",
    });
    const result = getStaffCardInfo(staff);

    expect(result).not.toContain("Hauptbetreuer");
  });

  it("combines qualifications and supervision role", () => {
    const staff = createSampleStaff({
      qualifications: "M.Ed",
      isSupervising: true,
      supervisionRole: "primary",
    });
    const result = getStaffCardInfo(staff);

    expect(result).toContain("M.Ed");
    expect(result).toContain("Hauptbetreuer");
    expect(result).toHaveLength(2);
  });
});

describe("formatStaffNotes", () => {
  it("returns undefined for empty notes", () => {
    expect(formatStaffNotes("")).toBeUndefined();
    expect(formatStaffNotes("   ")).toBeUndefined();
    expect(formatStaffNotes(undefined)).toBeUndefined();
  });

  it("returns trimmed notes under max length", () => {
    const notes = "  Short note  ";
    const result = formatStaffNotes(notes);

    expect(result).toBe("Short note");
  });

  it("truncates notes exceeding max length", () => {
    const longNotes =
      "This is a very long note that exceeds the default maximum length limit";
    const result = formatStaffNotes(longNotes, 30);

    expect(result).toBe("This is a very long note th...");
    expect(result?.length).toBe(30);
  });

  it("respects custom max length", () => {
    const notes = "Short note here";
    const result = formatStaffNotes(notes, 10);

    expect(result).toBe("Short n...");
  });

  it("does not truncate notes exactly at max length", () => {
    const notes = "Exactly10!";
    const result = formatStaffNotes(notes, 10);

    expect(result).toBe("Exactly10!");
  });
});

describe("sortStaff", () => {
  it("sorts supervising staff before non-supervising", () => {
    const staff: Staff[] = [
      createSampleStaff({ id: "1", lastName: "Alpha", isSupervising: false }),
      createSampleStaff({ id: "2", lastName: "Beta", isSupervising: true }),
      createSampleStaff({ id: "3", lastName: "Gamma", isSupervising: false }),
    ];

    const result = sortStaff(staff);

    expect(result[0]?.id).toBe("2"); // Beta is supervising
    expect(result[0]?.isSupervising).toBe(true);
  });

  it("sorts alphabetically by last name within same supervision status", () => {
    const staff: Staff[] = [
      createSampleStaff({ id: "1", lastName: "Zeta", isSupervising: false }),
      createSampleStaff({ id: "2", lastName: "Alpha", isSupervising: false }),
      createSampleStaff({ id: "3", lastName: "Beta", isSupervising: false }),
    ];

    const result = sortStaff(staff);

    expect(result[0]?.lastName).toBe("Alpha");
    expect(result[1]?.lastName).toBe("Beta");
    expect(result[2]?.lastName).toBe("Zeta");
  });

  it("handles German umlauts correctly in sorting", () => {
    const staff: Staff[] = [
      createSampleStaff({ id: "1", lastName: "Müller", isSupervising: false }),
      createSampleStaff({ id: "2", lastName: "Abel", isSupervising: false }),
      createSampleStaff({ id: "3", lastName: "Ziegler", isSupervising: false }),
    ];

    const result = sortStaff(staff);

    // German locale sorts alphabetically (Abel < Müller < Ziegler)
    expect(result.map((s) => s.lastName)).toEqual([
      "Abel",
      "Müller",
      "Ziegler",
    ]);
  });

  it("does not mutate original array", () => {
    const staff: Staff[] = [
      createSampleStaff({ id: "1", lastName: "Zeta" }),
      createSampleStaff({ id: "2", lastName: "Alpha" }),
    ];

    const result = sortStaff(staff);

    expect(staff[0]?.lastName).toBe("Zeta"); // Original unchanged
    expect(result[0]?.lastName).toBe("Alpha"); // Sorted copy
  });
});
