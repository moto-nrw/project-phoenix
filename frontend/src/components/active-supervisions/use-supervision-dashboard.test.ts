import { describe, expect, it } from "vitest";

import { releasedRoomTargetedByUrl } from "./use-supervision-dashboard";

describe("releasedRoomTargetedByUrl", () => {
  const rooms = [
    { id: "fußball", room_id: "sporthalle" },
    { id: "werkraum", room_id: "werkraum" },
  ];
  const openRoomIds = new Set(["sporthalle"]);

  it("keeps a released room pending until its shared selection is active", () => {
    expect(
      releasedRoomTargetedByUrl({
        sessionParam: null,
        roomParam: "sporthalle",
        rooms,
        openRoomIds,
      }),
    ).toBe("sporthalle");
  });

  it("maps a legacy session URL in a released room to the shared room", () => {
    expect(
      releasedRoomTargetedByUrl({
        sessionParam: "fußball",
        roomParam: null,
        rooms,
        openRoomIds,
      }),
    ).toBe("sporthalle");
  });
});
