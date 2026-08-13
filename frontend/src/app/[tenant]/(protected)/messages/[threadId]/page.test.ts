import { describe, expect, it } from "vitest";
import { isMessageSnapshotUnavailable } from "./page";

describe("isMessageSnapshotUnavailable", () => {
  it("blocks replies until the seeded thread history has loaded", () => {
    expect(isMessageSnapshotUnavailable(false, true, 0, undefined)).toBe(true);
  });

  it("blocks replies after the history request failed", () => {
    expect(
      isMessageSnapshotUnavailable(false, false, 0, new Error("failed")),
    ).toBe(true);
  });

  it("allows replies after an empty or populated snapshot loaded", () => {
    expect(isMessageSnapshotUnavailable(false, false, 0, undefined)).toBe(
      false,
    );
    expect(isMessageSnapshotUnavailable(false, false, 2, undefined)).toBe(
      false,
    );
  });
});
