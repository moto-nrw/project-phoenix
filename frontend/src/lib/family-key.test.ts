import { describe, expect, it } from "vitest";
import { familyKeyEmail, familyKeyLabel } from "./family-key";

describe("family key", () => {
  it("shows a mail address as it is", () => {
    expect(familyKeyLabel("eva@example.org")).toBe("eva@example.org");
    expect(familyKeyEmail("eva@example.org")).toBe("eva@example.org");
  });

  it("names the contract of a family without a mail address", () => {
    expect(familyKeyLabel("contract:41")).toBe("Vertrag Nr. 41");
    expect(familyKeyEmail("contract:41")).toBe("");
  });
});
