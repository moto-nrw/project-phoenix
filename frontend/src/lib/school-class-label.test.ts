import { describe, expect, it } from "vitest";

import { schoolClassLabel } from "./school-class-label";

describe("schoolClassLabel", () => {
  it("ergänzt das Präfix bei bloßen Klassennamen", () => {
    expect(schoolClassLabel("1a")).toBe("Klasse 1a");
  });

  it("verdoppelt ein vorhandenes Präfix nicht", () => {
    expect(schoolClassLabel("Klasse 1a")).toBe("Klasse 1a");
    expect(schoolClassLabel("klasse 1a")).toBe("klasse 1a");
  });

  it("liefert bei leerem Namen nichts", () => {
    expect(schoolClassLabel("  ")).toBe("");
  });
});
