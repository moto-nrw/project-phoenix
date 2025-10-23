import { describe, expect, it } from "vitest";
import {
  getStudentLocationBadge,
  type StudentLocationStatus,
} from "../student-location-helpers";
import { STUDENT_LOCATION_BADGE_TOKENS } from "~/theme/student-location-status-tokens";

describe("getStudentLocationBadge", () => {
  it("returns home tokens when status is null", () => {
    const badge = getStudentLocationBadge(null);
    expect(badge.label).toBe("Zuhause");
    expect(badge.colorToken).toBe(
      STUDENT_LOCATION_BADGE_TOKENS.home.colorToken,
    );
  });

  it("uses group room styling for PRESENT_IN_ROOM in group room", () => {
    const status: StudentLocationStatus = {
      state: "PRESENT_IN_ROOM",
      room: {
        id: 1,
        name: "Gruppenraum 1",
        isGroupRoom: true,
        ownerType: "GROUP",
      },
    };

    const badge = getStudentLocationBadge(status);

    expect(badge.label).toBe("Gruppenraum 1");
    expect(badge.colorToken).toBe(
      STUDENT_LOCATION_BADGE_TOKENS.groupRoom.colorToken,
    );
  });

  it("uses other room styling for PRESENT_IN_ROOM outside group room", () => {
    const status: StudentLocationStatus = {
      state: "PRESENT_IN_ROOM",
      room: {
        id: 2,
        name: "Werkraum",
        isGroupRoom: false,
        ownerType: "ACTIVITY",
      },
    };

    const badge = getStudentLocationBadge(status);

    expect(badge.label).toBe("Werkraum");
    expect(badge.colorToken).toBe(
      STUDENT_LOCATION_BADGE_TOKENS.otherRoom.colorToken,
    );
  });

  it("returns transit styling for TRANSIT state", () => {
    const status: StudentLocationStatus = {
      state: "TRANSIT",
    };

    const badge = getStudentLocationBadge(status);

    expect(badge.label).toBe("Unterwegs");
    expect(badge.colorToken).toBe(
      STUDENT_LOCATION_BADGE_TOKENS.transit.colorToken,
    );
  });
});
