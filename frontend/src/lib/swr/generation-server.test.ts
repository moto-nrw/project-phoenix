import { describe, expect, it } from "vitest";
import {
  _swrStampedKeyCountForTests,
  dataGenerationOf,
  isCurrentSWRValue,
  isFallbackVisible,
  isVisibleSWRValue,
  noteFallbackShown,
  stampSWRValue,
} from "./generation";

// The server renders requests of many sessions in one long process (#5223).
describe("generation lock on the server", () => {
  it("keeps no metadata of the keys it stamps", () => {
    // Anker: dieser Test läuft ohne Browser.
    expect(typeof window).toBe("undefined");
    const stamped: [string, unknown][] = [];
    for (let i = 0; i < 2000; i += 1) {
      const key = `school-${i % 50}:account-${i}:students-list-${i}`;
      const value = { rows: [i] };
      stamped.push([key, value], [`${key}-total`, i]);
      stampSWRValue(key, value, dataGenerationOf(key));
      stampSWRValue(`${key}-total`, i, dataGenerationOf(key));
      noteFallbackShown(key, dataGenerationOf(key));
    }

    expect(_swrStampedKeyCountForTests()).toBe(0);
    // No generation moves on the server: every value it renders counts.
    for (const [key, value] of stamped) {
      expect(isCurrentSWRValue(key, value)).toBeUndefined();
      expect(isVisibleSWRValue(key, value)).toBe(true);
      expect(isFallbackVisible(key)).toBe(true);
    }
  });
});
