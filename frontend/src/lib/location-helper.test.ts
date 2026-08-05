import { describe, it, expect } from "vitest";
import {
  LOCATION_STATUSES,
  LOCATION_COLORS,
  parseLocation,
  normalizeLocation,
  getLocationColor,
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
    expect(LOCATION_COLORS.HOME).toBe("#FF3130");
    expect(LOCATION_COLORS.SCHOOLYARD).toBe("#F78C10");
    expect(LOCATION_COLORS.TRANSIT).toBe("#D946EF");
    expect(LOCATION_COLORS.UNKNOWN).toBe("#6B7280");
    expect(LOCATION_COLORS.SICK).toBe("#EAB308");
  });

  it("has SICK color (amber) for medical indication", () => {
    expect(LOCATION_COLORS.SICK).toBeDefined();
    expect(LOCATION_COLORS.SICK).toBe("#EAB308");
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
    expect(result).toContain("rgba(107, 114, 128,"); // Gray fallback
  });

  it("handles SICK color correctly", () => {
    const result = getLocationGlowEffect(LOCATION_COLORS.SICK);
    expect(result).toContain("rgba(234, 179, 8,"); // #EAB308 RGB values
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
