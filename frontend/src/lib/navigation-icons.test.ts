import { describe, it, expect } from "vitest";
import { navigationIcons } from "./navigation-icons";

describe("navigationIcons", () => {
  it("exposes a non-empty SVG path for every icon key", () => {
    for (const [key, path] of Object.entries(navigationIcons)) {
      expect(typeof path, `${key} should be a string`).toBe("string");
      expect(path.length, `${key} should be a non-empty path`).toBeGreaterThan(
        0,
      );
    }
  });

  it("includes the `book` icon used by the public Hilfe guide link", () => {
    // The desktop sidebar and the mobile bottom-nav both reference this single
    // source for the help icon — keep it present so the path can't drift apart
    // between the two nav surfaces.
    expect(navigationIcons.book).toBeDefined();
    expect(navigationIcons.book.startsWith("M12 6.042")).toBe(true);
  });
});
