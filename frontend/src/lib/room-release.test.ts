import { describe, it, expect } from "vitest";
import type { Room } from "./room-helpers";
import {
  isToiletRoom,
  mapRoomResponse,
  prepareRoomForBackend,
} from "./room-helpers";
import { buildBackendRoom } from "~/test/fixtures/rooms";

// Behaviour of the room release ("offener Raum", #3064) on the client side.
// The load-bearing rule is what an OMITTED value means: the backend leaves a
// standing release untouched when is_open_room is absent, so the mapping must
// never invent one — and must never drop an explicit false either.

describe("isToiletRoom", () => {
  it("recognises both canonical toilet names", () => {
    expect(isToiletRoom({ id: "1", name: "WC" } as Room)).toBe(true);
    expect(isToiletRoom({ id: "2", name: "Toilette" } as Room)).toBe(true);
  });

  it("does not treat the Schulhof or an ordinary room as a toilet", () => {
    expect(isToiletRoom({ id: "3", name: "Schulhof" } as Room)).toBe(false);
    expect(isToiletRoom({ id: "4", name: "Turnhalle" } as Room)).toBe(false);
  });

  it("matches exact-case, mirroring the backend system-room contract", () => {
    expect(isToiletRoom({ id: "5", name: "wc" } as Room)).toBe(false);
    expect(isToiletRoom({ id: "6", name: "toilette" } as Room)).toBe(false);
  });

  it("handles a missing room", () => {
    expect(isToiletRoom(null)).toBe(false);
    expect(isToiletRoom(undefined)).toBe(false);
  });
});

describe("mapRoomResponse with the room release", () => {
  it("carries a released room through", () => {
    const result = mapRoomResponse(
      buildBackendRoom({ id: 1, name: "Turnhalle", is_open_room: true }),
    );
    expect(result.isOpenRoom).toBe(true);
  });

  it("carries an unreleased room through as false", () => {
    const result = mapRoomResponse(
      buildBackendRoom({ id: 2, name: "Werkraum", is_open_room: false }),
    );
    expect(result.isOpenRoom).toBe(false);
  });

  it("leaves the release undefined when the backend did not send it", () => {
    const result = mapRoomResponse(buildBackendRoom({ id: 3, name: "Mensa" }));
    expect(result.isOpenRoom).toBeUndefined();
  });

  it("carries the system-room flag too", () => {
    const result = mapRoomResponse(
      buildBackendRoom({ id: 4, name: "Schulhof", is_system: true }),
    );
    expect(result.isSystem).toBe(true);
  });
});

describe("prepareRoomForBackend with the room release", () => {
  it("sends an explicit release", () => {
    const result = prepareRoomForBackend({
      name: "Turnhalle",
      isOpenRoom: true,
    });
    expect(result.is_open_room).toBe(true);
  });

  it("sends an explicit revocation — false is a real answer, not an absent one", () => {
    const result = prepareRoomForBackend({
      name: "Turnhalle",
      isOpenRoom: false,
    });
    expect(result.is_open_room).toBe(false);
    expect("is_open_room" in result).toBe(true);
  });

  it("omits the field entirely when the form carries no opinion", () => {
    const result = prepareRoomForBackend({ name: "Turnhalle" });
    expect("is_open_room" in result).toBe(false);
  });

  it("omits the field for an explicitly undefined value", () => {
    const result = prepareRoomForBackend({
      name: "Turnhalle",
      isOpenRoom: undefined,
    });
    expect("is_open_room" in result).toBe(false);
  });

  it("does not disturb the other fields it already sent", () => {
    const result = prepareRoomForBackend({
      name: "Turnhalle",
      building: "Sporthalle",
      category: "Sport",
      isOpenRoom: true,
    });
    expect(result.name).toBe("Turnhalle");
    expect(result.building).toBe("Sporthalle");
    expect(result.category).toBe("Sport");
    expect(result.is_open_room).toBe(true);
  });
});
