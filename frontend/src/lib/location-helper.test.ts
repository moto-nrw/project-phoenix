import { describe, it, expect } from "vitest";
import {
  LOCATION_STATUSES,
  LOCATION_COLORS,
  MOTO_COLOR_PALETTE,
  parseLocation,
  normalizeLocation,
  getLocationColor,
  getLocationBadgeTone,
  getAccessibleTextColor,
  getLocationDisplay,
  getLocationGlowEffect,
  canSeeDetailedLocation,
  isPresentLocation,
  isHomeLocation,
  isSchoolyardLocation,
  isTransitLocation,
  type StudentLocationContext,
} from "./location-helper";

// =============================================================================
// CONSTANTS TESTS
// =============================================================================

describe("LOCATION_STATUSES", () => {
  it("contains all expected status values", () => {
    expect(LOCATION_STATUSES.PRESENT).toBe("Anwesend");
    expect(LOCATION_STATUSES.HOME).toBe("Zuhause");
    expect(LOCATION_STATUSES.SCHOOLYARD).toBe("Schulhof");
    expect(LOCATION_STATUSES.TRANSIT).toBe("Unterwegs");
    expect(LOCATION_STATUSES.UNKNOWN).toBe("Unbekannt");
    expect(LOCATION_STATUSES.SICK).toBe("Krank");
  });

  it("has SICK status for medical indication", () => {
    expect(LOCATION_STATUSES.SICK).toBeDefined();
    expect(typeof LOCATION_STATUSES.SICK).toBe("string");
  });
});

describe("LOCATION_COLORS", () => {
  it("contains all expected color values", () => {
    expect(LOCATION_COLORS.GROUP_ROOM).toBe("#83CD2D");
    expect(LOCATION_COLORS.OTHER_ROOM).toBe("#5080D8");
    expect(LOCATION_COLORS.HOME).toBe("#6B7280");
    expect(LOCATION_COLORS.SCHOOLYARD).toBe("#F78C10");
    expect(LOCATION_COLORS.TRANSIT).toBe("#D946EF");
    expect(LOCATION_COLORS.UNKNOWN).toBe("#78716C");
    expect(LOCATION_COLORS.SICK).toBe("#DC2626");
    expect(LOCATION_COLORS.CLASS_TRIP).toBe("#0891B2");
    expect(LOCATION_COLORS.NOT_ARRIVAL).toBe("#365D83");
    expect(LOCATION_COLORS.DANGER).toBe("#DC2626");
    const statusColors = Object.entries(LOCATION_COLORS)
      .filter(([key]) => key !== "DANGER")
      .map(([, color]) => color);
    expect(new Set(statusColors).size).toBe(statusColors.length);
  });

  it("uses the notification red for medical indication", () => {
    expect(LOCATION_COLORS.SICK).toBeDefined();
    expect(LOCATION_COLORS.SICK).toBe("#DC2626");
  });
});

describe("MOTO_COLOR_PALETTE", () => {
  it("matches the shared brand color roles", () => {
    expect(MOTO_COLOR_PALETTE).toEqual({
      green: {
        soft: "#EEF9E1",
        muted: "#D7E8C3",
        light: "#92D63C",
        base: "#83CD2D",
        vivid: "#5F9F1B",
        hover: "#74B825",
        active: "#6DB118",
        strong: "#3F6F12",
      },
      blue: {
        soft: "#EDF3FC",
        light: "#6B95E0",
        base: "#5080D8",
        hover: "#3B68C0",
        strong: "#315C9B",
      },
      timeTracking: {
        soft: "#E0F2FE",
        light: "#7DD3FC",
        base: "#0EA5E9",
        strong: "#0369A1",
      },
      orange: {
        soft: "#FFF3E5",
        base: "#F78C10",
        hover: "#E07400",
        strong: "#9B5609",
      },
      red: {
        soft: "#FEF2F2",
        base: "#DC2626",
        hover: "#B91C1C",
        strong: "#B91C1C",
      },
      teal: {
        soft: "#E8F8F5",
        light: "#5CC8BA",
        base: "#159E90",
        strong: "#0F766E",
      },
      amber: {
        soft: "#FEF3C7",
        light: "#FACC15",
        base: "#EAB308",
        strong: "#92400E",
      },
      purple: {
        soft: "#F3E8FF",
        light: "#A78BFA",
        base: "#7C3AED",
        strong: "#6B21A8",
      },
      magenta: {
        soft: "#FAE8FF",
        light: "#E879F9",
        base: "#D946EF",
        strong: "#86198F",
      },
      indigo: {
        soft: "#EEF2FF",
        light: "#818CF8",
        base: "#4F46E5",
        strong: "#3730A3",
      },
      coral: {
        soft: "#FFF0ED",
        light: "#F29A8D",
        base: "#E85D4A",
        strong: "#A83A2E",
      },
      cyan: {
        soft: "#ECFEFF",
        light: "#67E8F9",
        base: "#0891B2",
        strong: "#155E75",
      },
      navy: {
        soft: "#EEF4F8",
        light: "#7FA6C9",
        base: "#365D83",
        strong: "#1E3A5F",
      },
      mint: {
        soft: "#EAF9F3",
        light: "#8AD9BB",
        base: "#3BAF83",
        strong: "#187255",
      },
      wine: {
        soft: "#FBECEF",
        light: "#CF7180",
        base: "#8F2535",
        strong: "#681A27",
      },
      gold: {
        soft: "#FFF7E6",
        light: "#E6B85C",
        base: "#B7791F",
        strong: "#7A4A0B",
      },
      petrol: {
        soft: "#E9F7F6",
        light: "#72BDB8",
        base: "#217A78",
        strong: "#155A59",
      },
      neutral: {
        soft: "#F3F4F6",
        light: "#9CA3AF",
        base: "#6B7280",
        strong: "#374151",
      },
      stone: {
        soft: "#F5F5F4",
        light: "#A8A29E",
        base: "#78716C",
        strong: "#44403C",
      },
    });
  });
});

describe("getLocationBadgeTone", () => {
  it("maps brand colors to soft surfaces and strong text colors", () => {
    expect(getLocationBadgeTone(LOCATION_COLORS.GROUP_ROOM)).toEqual({
      backgroundColor: MOTO_COLOR_PALETTE.green.soft,
      dotColor: MOTO_COLOR_PALETTE.green.base,
      textColor: MOTO_COLOR_PALETTE.green.strong,
    });
    expect(getLocationBadgeTone(LOCATION_COLORS.HOME)).toEqual({
      backgroundColor: MOTO_COLOR_PALETTE.neutral.soft,
      dotColor: MOTO_COLOR_PALETTE.neutral.light,
      textColor: "#4B5563",
    });
  });

  it("keeps a custom room color's hue in the label, not just the dot", () => {
    // Room colors (#1324) can never match a LOCATION_BADGE_TONES row: the
    // backend reserves exactly the status hexes, so an admin-picked color is
    // always something else. A flat neutral label would collapse every custom
    // room onto one pill and leave only the 6px dot to tell them apart, which
    // is the differentiation the feature exists for. #5b7842 is #A3D977
    // darkened to 4.77:1 on the gray-50 pill.
    expect(getLocationBadgeTone("#A3D977")).toEqual({
      backgroundColor: "#F9FAFB",
      dotColor: "#A3D977",
      textColor: "#5b7842",
    });
  });

  it("gives two different room colors two different labels", () => {
    const a = getLocationBadgeTone("#A3D977");
    const b = getLocationBadgeTone("#D97AA3");

    expect(a.textColor).not.toBe(b.textColor);
  });
});

// =============================================================================
// PARSE LOCATION TESTS
// =============================================================================

describe("parseLocation", () => {
  it("parses location with room", () => {
    const result = parseLocation("Anwesend - Raum 101");
    expect(result.status).toBe("Anwesend");
    expect(result.room).toBe("Raum 101");
  });

  it("parses location without room", () => {
    const result = parseLocation("Zuhause");
    expect(result.status).toBe("Zuhause");
    expect(result.room).toBeUndefined();
  });

  it("handles null/undefined location", () => {
    expect(parseLocation(null).status).toBe("Unbekannt");
    expect(parseLocation(undefined).status).toBe("Unbekannt");
  });

  it("handles empty string", () => {
    expect(parseLocation("").status).toBe("Unbekannt");
  });

  it("normalizes legacy status keywords", () => {
    expect(parseLocation("abwesend").status).toBe("Zuhause");
    expect(parseLocation("home").status).toBe("Zuhause");
    expect(parseLocation("anwesend").status).toBe("Anwesend");
  });
});

describe("normalizeLocation", () => {
  it("normalizes location strings", () => {
    expect(normalizeLocation("anwesend - Room A")).toBe("Anwesend - Room A");
    expect(normalizeLocation("zuhause")).toBe("Zuhause");
    expect(normalizeLocation("unterwegs")).toBe("Unterwegs");
  });

  it("handles WC/bathroom status", () => {
    expect(normalizeLocation("wc")).toBe("Anwesend - Toilette");
    expect(normalizeLocation("bathroom")).toBe("Anwesend - Toilette");
    expect(normalizeLocation("toilette")).toBe("Anwesend - Toilette");
  });

  it("normalizes compound WC location from backend", () => {
    expect(normalizeLocation("Anwesend - WC")).toBe("Anwesend - Toilette");
  });
});

// =============================================================================
// LOCATION COLOR TESTS
// =============================================================================

describe("getLocationColor", () => {
  it("returns HOME color for Zuhause", () => {
    expect(getLocationColor("Zuhause")).toBe(LOCATION_COLORS.HOME);
  });

  it("returns SCHOOLYARD color for Schulhof", () => {
    expect(getLocationColor("Schulhof")).toBe(LOCATION_COLORS.SCHOOLYARD);
  });

  it("returns TRANSIT color for Unterwegs", () => {
    expect(getLocationColor("Unterwegs")).toBe(LOCATION_COLORS.TRANSIT);
  });

  it("returns GROUP_ROOM color for Anwesend without room", () => {
    expect(getLocationColor("Anwesend")).toBe(LOCATION_COLORS.GROUP_ROOM);
  });

  it("returns GROUP_ROOM color when room matches group rooms", () => {
    const color = getLocationColor("Anwesend - Raum A", false, ["Raum A"]);
    expect(color).toBe(LOCATION_COLORS.GROUP_ROOM);
  });

  it("returns OTHER_ROOM color when room does not match group rooms", () => {
    const color = getLocationColor("Anwesend - Raum B", false, ["Raum A"]);
    expect(color).toBe(LOCATION_COLORS.OTHER_ROOM);
  });

  it("returns UNKNOWN color for unknown status", () => {
    expect(getLocationColor("SomeRandomStatus")).toBe(LOCATION_COLORS.UNKNOWN);
  });

  // ===========================================================================
  // Per-room color (Issue #1324 — eliminate the all-blue room badges)
  // ===========================================================================

  it("returns the per-room color when set and the room is not a group room", () => {
    // The viewer is not in this group → no green; the room has its own
    // configured hex → that wins over the OTHER_ROOM blue fallback.
    const color = getLocationColor(
      "Anwesend - Bibliothek",
      false,
      ["Raum A"],
      "#A3D977",
    );
    expect(color).toBe("#A3D977");
  });

  it("falls back to OTHER_ROOM blue when no per-room color is set", () => {
    // Same scenario as above without a configured color: kept as today.
    const color = getLocationColor(
      "Anwesend - Bibliothek",
      false,
      ["Raum A"],
      null,
    );
    expect(color).toBe(LOCATION_COLORS.OTHER_ROOM);
  });

  it("treats empty string roomColor like null (clearing the override)", () => {
    // Clearing the color via the picker yields "" or null depending on the
    // wire shape; either must fall back to the blue default.
    const color = getLocationColor(
      "Anwesend - Bibliothek",
      false,
      ["Raum A"],
      "",
    );
    expect(color).toBe(LOCATION_COLORS.OTHER_ROOM);
  });

  it("keeps GROUP_ROOM green when viewer is in the same group, even with a per-room color", () => {
    // Hard constraint: non-blue colors must keep their meaning. If the room
    // is the viewer's own OGS-Gruppenraum, green wins over the configured
    // per-room hex.
    const color = getLocationColor(
      "Anwesend - Raum A",
      false,
      ["Raum A"],
      "#A3D977",
    );
    expect(color).toBe(LOCATION_COLORS.GROUP_ROOM);
  });

  it("keeps status colors unchanged regardless of per-room color", () => {
    // Status badges (Schulhof, Unterwegs, Zuhause) must never be overridden
    // by a roomColor — the parser hits STATUS_COLOR_MAP first.
    expect(getLocationColor("Schulhof", false, [], "#A3D977")).toBe(
      LOCATION_COLORS.SCHOOLYARD,
    );
    expect(getLocationColor("Unterwegs", false, [], "#A3D977")).toBe(
      LOCATION_COLORS.TRANSIT,
    );
    expect(getLocationColor("Zuhause", false, [], "#A3D977")).toBe(
      LOCATION_COLORS.HOME,
    );
  });
});

// =============================================================================
// LOCATION DISPLAY TESTS
// =============================================================================

describe("getLocationDisplay", () => {
  const baseStudent: StudentLocationContext = {
    current_location: "Anwesend - Raum 101",
    location_since: "2024-01-15T10:00:00Z",
    group_id: "1",
    group_name: "Gruppe A",
  };

  it("returns group name for groupName display mode", () => {
    const result = getLocationDisplay(baseStudent, "groupName");
    expect(result).toBe("Gruppe A");
  });

  it("returns room for roomName display mode", () => {
    const result = getLocationDisplay(baseStudent, "roomName");
    expect(result).toBe("Raum 101");
  });

  it("returns status for roomName mode without room", () => {
    const studentWithoutRoom = { ...baseStudent, current_location: "Zuhause" };
    const result = getLocationDisplay(studentWithoutRoom, "roomName");
    expect(result).toBe("Zuhause");
  });

  it("contextAware mode shows details for own group students", () => {
    const result = getLocationDisplay(baseStudent, "contextAware", ["1"]);
    expect(result).toBe("Raum 101");
  });

  it("contextAware mode hides details for other group students", () => {
    const result = getLocationDisplay(baseStudent, "contextAware", ["2"]);
    expect(result).toBe("Anwesend"); // Only status, no room
  });
});

// =============================================================================
// ACCESS CONTROL TESTS
// =============================================================================

describe("canSeeDetailedLocation", () => {
  const student: StudentLocationContext = {
    current_location: "Anwesend - Raum 101",
    group_id: "1",
    group_name: "Gruppe A",
  };

  it("returns true when student is in users group", () => {
    expect(canSeeDetailedLocation(student, ["1"])).toBe(true);
  });

  it("returns false when student is not in users group", () => {
    expect(canSeeDetailedLocation(student, ["2", "3"])).toBe(false);
  });

  it("returns false when userGroups is empty", () => {
    expect(canSeeDetailedLocation(student, [])).toBe(false);
  });

  it("returns false when userGroups is undefined", () => {
    expect(canSeeDetailedLocation(student, undefined)).toBe(false);
  });
});

// =============================================================================
// LOCATION CHECK HELPERS
// =============================================================================

describe("location check helpers", () => {
  describe("isPresentLocation", () => {
    it("returns true for Anwesend", () => {
      expect(isPresentLocation("Anwesend")).toBe(true);
      expect(isPresentLocation("Anwesend - Raum 101")).toBe(true);
    });

    it("returns false for other statuses", () => {
      expect(isPresentLocation("Zuhause")).toBe(false);
      expect(isPresentLocation("Unterwegs")).toBe(false);
    });
  });

  describe("isHomeLocation", () => {
    it("returns true for Zuhause", () => {
      expect(isHomeLocation("Zuhause")).toBe(true);
    });

    it("returns true for legacy home status", () => {
      expect(isHomeLocation("abwesend")).toBe(true);
      expect(isHomeLocation("home")).toBe(true);
    });

    it("returns false for other statuses", () => {
      expect(isHomeLocation("Anwesend")).toBe(false);
      expect(isHomeLocation("Unterwegs")).toBe(false);
    });
  });

  describe("isSchoolyardLocation", () => {
    it("returns true for Schulhof", () => {
      expect(isSchoolyardLocation("Schulhof")).toBe(true);
      expect(isSchoolyardLocation("schulhof")).toBe(true);
    });

    it("returns false for other statuses", () => {
      expect(isSchoolyardLocation("Anwesend")).toBe(false);
    });
  });

  describe("isTransitLocation", () => {
    it("returns true for Unterwegs", () => {
      expect(isTransitLocation("Unterwegs")).toBe(true);
      expect(isTransitLocation("unterwegs")).toBe(true);
    });

    it("returns false for other statuses", () => {
      expect(isTransitLocation("Anwesend")).toBe(false);
    });
  });
});

// =============================================================================
// GLOW EFFECT TESTS
// =============================================================================

describe("getLocationGlowEffect", () => {
  it("returns valid box-shadow for valid hex color", () => {
    const result = getLocationGlowEffect("#FF0000");
    expect(result).toContain("rgba(255, 0, 0,");
    expect(result).toContain("0 8px 25px");
  });

  it("returns gray glow for invalid color", () => {
    const result = getLocationGlowEffect("invalid");
    expect(result).toContain("rgba(120, 113, 108,"); // Stone-gray fallback
  });

  it("handles SICK color correctly", () => {
    const result = getLocationGlowEffect(LOCATION_COLORS.SICK);
    expect(result).toContain("rgba(220, 38, 38,");
  });
});

// =============================================================================
// STUDENT LOCATION CONTEXT WITH SICK FIELDS
// =============================================================================

describe("StudentLocationContext with sick fields", () => {
  it("supports sick boolean field", () => {
    const student: StudentLocationContext = {
      current_location: "Zuhause",
      sick: true,
    };
    expect(student.sick).toBe(true);
  });

  it("supports sick_since timestamp field", () => {
    const student: StudentLocationContext = {
      current_location: "Zuhause",
      sick: true,
      sick_since: "2024-01-15T08:00:00Z",
    };
    expect(student.sick_since).toBe("2024-01-15T08:00:00Z");
  });

  it("sick fields are optional", () => {
    const student: StudentLocationContext = {
      current_location: "Anwesend",
    };
    expect(student.sick).toBeUndefined();
    expect(student.sick_since).toBeUndefined();
  });
});

// =============================================================================
// ACCESSIBLE TEXT COLOR TESTS
// =============================================================================

describe("getAccessibleTextColor", () => {
  // Independent WCAG implementation: the test must not agree with the helper
  // by sharing its arithmetic.
  function luminance(hex: string): number {
    const value = hex.replace("#", "");
    const channels = [0, 2, 4].map((offset) => {
      const srgb = Number.parseInt(value.slice(offset, offset + 2), 16) / 255;
      return srgb <= 0.03928
        ? srgb / 12.92
        : Math.pow((srgb + 0.055) / 1.055, 2.4);
    });
    return (
      0.2126 * channels[0]! + 0.7152 * channels[1]! + 0.0722 * channels[2]!
    );
  }

  function contrast(hex: string, background: string): number {
    const a = luminance(hex);
    const b = luminance(background);
    return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
  }

  const WHITE = "#FFFFFF";
  // Die Pillenfläche der StatusDotBadge — die dunklere der beiden Flächen und
  // damit der Fall, an dem sich der Kontrast entscheidet.
  const GRAY_50 = "#F9FAFB";

  it("clears 4.5:1 for every palette color on both surfaces", () => {
    for (const color of Object.values(LOCATION_COLORS)) {
      for (const surface of [WHITE, GRAY_50]) {
        const text = getAccessibleTextColor(color, surface);
        expect(contrast(text, surface)).toBeGreaterThanOrEqual(4.5);
      }
    }
  });

  it("darkens a color outside the palette instead of passing it through", () => {
    // Homeoffice-Blau aus staff-helpers und eine helle Raumfarbe.
    for (const color of ["#0EA5E9", "#FFD166"]) {
      for (const surface of [WHITE, GRAY_50]) {
        const text = getAccessibleTextColor(color, surface);
        expect(text).not.toBe(color);
        expect(contrast(text, surface)).toBeGreaterThanOrEqual(4.5);
      }
    }
  });

  it("darkens further on gray-50 than on white where the surface decides", () => {
    // Marken-Orange: die auf Weiß gerechnete Schattierung landet auf gray-50
    // bei 4,48:1 und reicht dort nicht.
    const onWhite = getAccessibleTextColor(LOCATION_COLORS.SCHOOLYARD, WHITE);
    const onGray = getAccessibleTextColor(LOCATION_COLORS.SCHOOLYARD, GRAY_50);
    expect(contrast(onWhite, GRAY_50)).toBeGreaterThanOrEqual(4.5);
    expect(contrast(onGray, GRAY_50)).toBeGreaterThanOrEqual(4.5);
  });

  it("defaults to white when no surface is given", () => {
    expect(getAccessibleTextColor(LOCATION_COLORS.HOME)).toBe(
      getAccessibleTextColor(LOCATION_COLORS.HOME, WHITE),
    );
  });

  it("leaves a color that already passes untouched", () => {
    expect(getAccessibleTextColor(LOCATION_COLORS.EXCUSED)).toBe(
      LOCATION_COLORS.EXCUSED,
    );
  });

  it("is case insensitive", () => {
    expect(getAccessibleTextColor("#83cd2d")).toBe(
      getAccessibleTextColor("#83CD2D"),
    );
  });

  it("falls back to neutral gray for a missing or unusable color", () => {
    for (const color of [undefined, null, "", "  ", "currentColor"]) {
      expect(getAccessibleTextColor(color)).toBe("#374151");
    }
  });
});
