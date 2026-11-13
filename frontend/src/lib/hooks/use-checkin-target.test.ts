import { describe, expect, it } from "vitest";

import type { TargetRoomOption } from "~/lib/move-targets";
import {
  checkinTargetLabel,
  resolveCheckinTarget,
  type StoredCheckinTarget,
} from "./use-checkin-target";

// Where a check-in in the app puts the children (#6953): the remembered
// choice of a device and how it finds today's target again.

const room1Session: TargetRoomOption = {
  kind: "session",
  value: "41",
  activeGroupId: "41",
  roomId: "7",
  roomName: "OGS-Raum 1",
  activityName: "Hausaufgaben",
  label: "OGS-Raum 1",
};
const yard: TargetRoomOption = {
  kind: "openRoom",
  value: "room:9",
  roomId: "9",
  roomName: "Schulhof",
  label: "Schulhof (offener Raum)",
};

function stored(option: TargetRoomOption): StoredCheckinTarget {
  return {
    value: option.value,
    kind: option.kind,
    roomId: option.roomId,
    roomName: option.roomName,
  };
}

describe("resolveCheckinTarget", () => {
  it("keeps „Anwesend (ohne Raum)“ when nothing was chosen", () => {
    expect(resolveCheckinTarget(null, [room1Session, yard])).toEqual({
      target: null,
      lost: false,
    });
  });

  it("finds the chosen target again", () => {
    expect(resolveCheckinTarget(stored(yard), [room1Session, yard])).toEqual({
      target: yard,
      lost: false,
    });
  });

  it("follows a supervision restarted in the same room", () => {
    const restarted = { ...room1Session, value: "58", activeGroupId: "58" };
    expect(
      resolveCheckinTarget(stored(room1Session), [restarted, yard]).target,
    ).toBe(restarted);
  });

  it("reports a target that is gone instead of picking another room", () => {
    expect(resolveCheckinTarget(stored(room1Session), [yard])).toEqual({
      target: null,
      lost: true,
    });
  });

  it("never guesses between two sessions of the same room", () => {
    const first = { ...room1Session, value: "60", activeGroupId: "60" };
    const second = { ...room1Session, value: "61", activeGroupId: "61" };
    expect(
      resolveCheckinTarget(stored(room1Session), [first, second]).lost,
    ).toBe(true);
  });
});

describe("checkinTargetLabel", () => {
  it("names the activity next to the room", () => {
    expect(checkinTargetLabel(room1Session)).toBe("OGS-Raum 1 · Hausaufgaben");
    expect(
      checkinTargetLabel({ ...room1Session, activityName: undefined }),
    ).toBe("OGS-Raum 1");
  });

  it("marks a released room as such", () => {
    expect(checkinTargetLabel(yard)).toBe("Schulhof (offener Raum)");
  });
});
