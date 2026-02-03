import { describe, it, expect } from "vitest";
import type { Staff } from "./staff-api";
import {
  getStaffLocationStatus,
  getStaffDisplayType,
  getStaffCardInfo,
  formatStaffNotes,
  sortStaff,
  getStaffSupervisionBadges,
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
  currentLocation: "Zuhause",
  supervisionRole: undefined,
  supervisions: [],
  wasPresentToday: false,
  ...overrides,
});

describe("getStaffLocationStatus", () => {
  it("returns Zuhause status with red styling for staff at home", () => {
    const staff = createSampleStaff({ currentLocation: "Zuhause" });
    const result = getStaffLocationStatus(staff);

    expect(result.label).toBe("Zuhause");
    expect(result.customBgColor).toBe("#FF3130");
    expect(result.customShadow).toContain("255, 49, 48");
  });

  it("returns Anwesend status with green styling matching student badge", () => {
    const staff = createSampleStaff({ currentLocation: "Anwesend" });
    const result = getStaffLocationStatus(staff);

    expect(result.label).toBe("Anwesend");
    // Should match LOCATION_COLORS.GROUP_ROOM from location-helper.ts
    expect(result.customBgColor).toBe("#83CD2D");
    expect(result.customShadow).toContain("131, 205, 45");
    expect(result.badgeColor).toContain("text-white");
  });

  it("returns Schulhof status with orange styling", () => {
    const staff = createSampleStaff({ currentLocation: "Schulhof" });
    const result = getStaffLocationStatus(staff);

    expect(result.label).toBe("Schulhof");
    expect(result.customBgColor).toBe("#F78C10");
    expect(result.customShadow).toContain("247, 140, 16");
  });

  it("returns Unterwegs status with purple styling", () => {
    const staff = createSampleStaff({ currentLocation: "Unterwegs" });
    const result = getStaffLocationStatus(staff);

    expect(result.label).toBe("Unterwegs");
    expect(result.customBgColor).toBe("#D946EF");
    expect(result.customShadow).toContain("217, 70, 239");
  });

  it("returns room name with blue styling for specific rooms", () => {
    const staff = createSampleStaff({ currentLocation: "Werkraum" });
    const result = getStaffLocationStatus(staff);

    expect(result.label).toBe("Werkraum");
    expect(result.customBgColor).toBe("#5080D8");
    expect(result.customShadow).toContain("80, 128, 216");
  });

  it("defaults to Zuhause when currentLocation is undefined", () => {
    const staff = createSampleStaff({ currentLocation: undefined });
    const result = getStaffLocationStatus(staff);

    expect(result.label).toBe("Zuhause");
    expect(result.customBgColor).toBe("#FF3130");
  });

  it("handles multiple room display", () => {
    const staff = createSampleStaff({ currentLocation: "2 Räume" });
    const result = getStaffLocationStatus(staff);

    // Multiple rooms should use blue styling
    expect(result.label).toBe("2 Räume");
    expect(result.customBgColor).toBe("#5080D8");
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

describe("getStaffSupervisionBadges", () => {
  it("returns single Zuhause badge for non-supervising staff", () => {
    const staff = createSampleStaff({
      isSupervising: false,
      currentLocation: "Zuhause",
      supervisions: [],
    });
    const result = getStaffSupervisionBadges(staff);

    expect(result).toHaveLength(1);
    expect(result[0]?.label).toBe("Zuhause");
    expect(result[0]?.key).toBe("status");
    expect(result[0]?.locationStatus.customBgColor).toBe("#FF3130");
  });

  it("returns single Anwesend badge for present non-supervising staff", () => {
    const staff = createSampleStaff({
      isSupervising: false,
      currentLocation: "Anwesend",
      supervisions: [],
    });
    const result = getStaffSupervisionBadges(staff);

    expect(result).toHaveLength(1);
    expect(result[0]?.label).toBe("Anwesend");
    expect(result[0]?.locationStatus.customBgColor).toBe("#83CD2D");
  });

  it("returns one badge per room for supervising staff", () => {
    const staff = createSampleStaff({
      isSupervising: true,
      currentLocation: "2 Räume",
      supervisions: [
        { roomId: "1", roomName: "Gymnasium", activeGroupId: "101" },
        { roomId: "2", roomName: "Bibliothek", activeGroupId: "102" },
      ],
    });
    const result = getStaffSupervisionBadges(staff);

    expect(result).toHaveLength(2);
    expect(result[0]?.label).toBe("Gymnasium");
    expect(result[0]?.key).toBe("room-1-101");
    expect(result[1]?.label).toBe("Bibliothek");
    expect(result[1]?.key).toBe("room-2-102");
  });

  it("uses blue color for regular room badges", () => {
    const staff = createSampleStaff({
      isSupervising: true,
      currentLocation: "Gymnasium",
      supervisions: [
        { roomId: "1", roomName: "Gymnasium", activeGroupId: "101" },
      ],
    });
    const result = getStaffSupervisionBadges(staff);

    expect(result).toHaveLength(1);
    expect(result[0]?.locationStatus.customBgColor).toBe("#5080D8");
  });

  it("uses orange color for Schulhof badge", () => {
    const staff = createSampleStaff({
      isSupervising: true,
      currentLocation: "Schulhof",
      supervisions: [
        { roomId: "1", roomName: "Schulhof", activeGroupId: "101" },
      ],
    });
    const result = getStaffSupervisionBadges(staff);

    expect(result).toHaveLength(1);
    expect(result[0]?.label).toBe("Schulhof");
    expect(result[0]?.locationStatus.customBgColor).toBe("#F78C10");
  });

  it("mixes colors for multiple rooms including Schulhof", () => {
    const staff = createSampleStaff({
      isSupervising: true,
      currentLocation: "3 Räume",
      supervisions: [
        { roomId: "1", roomName: "Gymnasium", activeGroupId: "101" },
        { roomId: "2", roomName: "Schulhof", activeGroupId: "102" },
        { roomId: "3", roomName: "Bibliothek", activeGroupId: "103" },
      ],
    });
    const result = getStaffSupervisionBadges(staff);

    expect(result).toHaveLength(3);
    expect(result[0]?.locationStatus.customBgColor).toBe("#5080D8"); // Gymnasium - blue
    expect(result[1]?.locationStatus.customBgColor).toBe("#F78C10"); // Schulhof - orange
    expect(result[2]?.locationStatus.customBgColor).toBe("#5080D8"); // Bibliothek - blue
  });

  it("falls back to status badge when isSupervising but empty supervisions", () => {
    const staff = createSampleStaff({
      isSupervising: true,
      currentLocation: "Unterwegs",
      supervisions: [],
    });
    const result = getStaffSupervisionBadges(staff);

    expect(result).toHaveLength(1);
    expect(result[0]?.label).toBe("Unterwegs");
    expect(result[0]?.key).toBe("status");
  });
});
