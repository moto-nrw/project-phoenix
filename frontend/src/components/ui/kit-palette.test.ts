import { describe, expect, it } from "vitest";

import {
  MOTO_COLOR_PALETTE,
  PILL_TONES,
  getAccessibleTextColor,
  getPillTone,
} from "./kit-palette";

describe("kit palette", () => {
  it("finds every pill tone by its hue's base hex, in any case", () => {
    for (const [hue, tone] of Object.entries(PILL_TONES)) {
      const base = MOTO_COLOR_PALETTE[hue as keyof typeof PILL_TONES].base;
      expect(getPillTone(base), hue).toEqual(tone);
      expect(getPillTone(base.toLowerCase()), hue).toEqual(tone);
    }
  });

  it("gives every pill tone its own background", () => {
    const backgrounds = Object.values(PILL_TONES).map(
      (tone) => tone.backgroundColor,
    );
    expect(new Set(backgrounds).size).toBe(backgrounds.length);
  });

  // #6573: the retired status red #FF3130 had its own text shade although no
  // palette hue carries it. It is a colour outside the palette now and gets
  // the arithmetic shade like any room colour.
  it("treats the retired status red like any colour outside the palette", () => {
    expect(getAccessibleTextColor("#FF3130")).toBe("#dc2a2a");
    expect(getPillTone("#FF3130")).toEqual({
      backgroundColor: "#F9FAFB",
      textColor: "#d62928",
    });
  });
});
