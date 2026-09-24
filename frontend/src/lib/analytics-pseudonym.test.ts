import { describe, expect, it } from "vitest";
import { analyticsPseudonym, isPseudonym } from "./analytics-pseudonym";

describe("analyticsPseudonym", () => {
  // The backend pins the same vector (backend/analytics/pseudonym_test.go); a
  // mismatch splits one person into two profiles.
  it("hashes school and account like the backend", async () => {
    await expect(analyticsPseudonym("42", "7")).resolves.toBe(
      "pseudo_4b9630678fc1afce76ce690721aaf949",
    );
  });

  it("gives the same account another ID at another school", async () => {
    const [a, b] = await Promise.all([
      analyticsPseudonym("42", "7"),
      analyticsPseudonym("43", "7"),
    ]);
    expect(a).not.toBe(b);
  });

  it("refuses IDs that are not numeric", async () => {
    await expect(analyticsPseudonym("42", "kim@example.org")).resolves.toBe(
      null,
    );
    await expect(analyticsPseudonym("", "7")).resolves.toBe(null);
  });

  it("recognizes its own shape only", async () => {
    expect(isPseudonym(await analyticsPseudonym("42", "7"))).toBe(true);
    expect(isPseudonym("pseudo_kim")).toBe(false);
    expect(isPseudonym("0199aa2b-anon")).toBe(false);
  });
});
