import { describe, it, expect } from "vitest";
import {
  LOCATION_STATUSES,
  LOCATION_COLORS,
  ROOM_COLOR_PALETTE,
  parseLocation,
  normalizeLocation,
  getLocationColor,
  getLocationDisplay,
  getLocationGlowEffect,
  canSeeDetailedLocation,
  isPresentLocation,
  isHomeLocation,
  isSchoolyardLocation,
  isTransitLocation,
  pickRoomColor,
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

  // Cross-stack parity: GROUP_ROOM is also hardcoded in
  // backend/api/common/badge.go as `BadgeStammraumGreen`. The Go-side pin
  // lives in `TestBadgeStammraumGreenLiteral`. If you change either side,
  // update the other in the same commit or stammraum badges will desync
  // between API responses and frontend rendering.
  it("pins GROUP_ROOM to the value the backend resolves stammraum to", () => {
    expect(LOCATION_COLORS.GROUP_ROOM).toBe("#83CD2D");
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
// ROOM COLOR PALETTE + PICKER LOGIC
// =============================================================================

describe("ROOM_COLOR_PALETTE", () => {
  it("contains the 9 curated palette colors", () => {
    expect(ROOM_COLOR_PALETTE).toHaveLength(9);
  });

  it("does not include reserved status / brand colors", () => {
    const reserved = [
      LOCATION_COLORS.GROUP_ROOM,
      LOCATION_COLORS.OTHER_ROOM,
      LOCATION_COLORS.HOME,
      LOCATION_COLORS.SCHOOLYARD,
      LOCATION_COLORS.TRANSIT,
      LOCATION_COLORS.SICK,
      LOCATION_COLORS.EXCUSED,
    ].map((c) => c.toLowerCase());
    for (const palette of ROOM_COLOR_PALETTE) {
      expect(reserved).not.toContain(palette.toLowerCase());
    }
  });
});

describe("pickRoomColor", () => {
  it("returns a palette color when nothing is used", () => {
    const picked = pickRoomColor([]);
    expect(ROOM_COLOR_PALETTE).toContain(picked);
  });

  it("prefers an unused palette color when some are used", () => {
    const used = ROOM_COLOR_PALETTE.slice(0, 3);
    const picked = pickRoomColor(used);
    expect(ROOM_COLOR_PALETTE).toContain(picked);
    expect(used).not.toContain(picked);
  });

  it("falls back to allowing duplicates when palette is exhausted", () => {
    const picked = pickRoomColor(ROOM_COLOR_PALETTE);
    expect(ROOM_COLOR_PALETTE).toContain(picked);
  });

  it("compares case-insensitively when filtering used colors", () => {
    const used = ROOM_COLOR_PALETTE.slice(0, 8).map((c) => c.toLowerCase());
    const picked = pickRoomColor(used);
    // Only one palette color should be left as truly "unused"
    expect(picked.toLowerCase()).toBe(ROOM_COLOR_PALETTE[8]!.toLowerCase());
  });
});

// =============================================================================
// getLocationColor with roomColor parameter (per-room custom colors)
// =============================================================================

describe("getLocationColor with roomColor", () => {
  it("returns the room's custom color for non-group rooms", () => {
    const color = getLocationColor(
      "Anwesend - Mathe-Raum",
      false,
      [],
      "#06B6D4",
    );
    expect(color).toBe("#06B6D4");
  });

  it("falls back to OTHER_ROOM blue when roomColor is not provided", () => {
    const color = getLocationColor("Anwesend - Mathe-Raum", false, []);
    expect(color).toBe(LOCATION_COLORS.OTHER_ROOM);
  });

  it("falls back to OTHER_ROOM blue when roomColor is empty string", () => {
    const color = getLocationColor("Anwesend - Mathe-Raum", false, [], "");
    expect(color).toBe(LOCATION_COLORS.OTHER_ROOM);
  });

  it("falls back to OTHER_ROOM blue when roomColor is null", () => {
    const color = getLocationColor("Anwesend - Mathe-Raum", false, [], null);
    expect(color).toBe(LOCATION_COLORS.OTHER_ROOM);
  });

  it("Stammraum-Grün via groupRooms wins over roomColor", () => {
    const color = getLocationColor(
      "Anwesend - Mathe-Raum",
      false,
      ["Mathe-Raum"],
      "#06B6D4",
    );
    expect(color).toBe(LOCATION_COLORS.GROUP_ROOM);
  });

  it("Stammraum-Grün via isGroupRoom flag wins over roomColor", () => {
    const color = getLocationColor(
      "Anwesend - Mathe-Raum",
      true,
      undefined,
      "#06B6D4",
    );
    expect(color).toBe(LOCATION_COLORS.GROUP_ROOM);
  });

  it("status colors win over roomColor (Schulhof stays orange)", () => {
    const color = getLocationColor("Schulhof", false, [], "#06B6D4");
    expect(color).toBe(LOCATION_COLORS.SCHOOLYARD);
  });

  it("returns roomColor when room is not in groupRooms list", () => {
    const color = getLocationColor(
      "Anwesend - Bastelraum",
      false,
      ["Mathe-Raum"],
      "#DB2777",
    );
    expect(color).toBe("#DB2777");
  });
});
