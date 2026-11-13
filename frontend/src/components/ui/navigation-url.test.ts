import { afterEach, describe, expect, it } from "vitest";

import {
  currentUrl,
  isInternalPath,
  navigationTarget,
  normalizedUrl,
} from "./navigation-url";

describe("navigation-url (#6570)", () => {
  afterEach(() => {
    window.history.replaceState({}, "", "/");
  });

  it.each(["/rooms", "/rooms?tab=plan", "/"])(
    "treats %s as internal",
    (href) => {
      expect(isInternalPath(href)).toBe(true);
    },
  );

  it.each([
    "//example.org/x",
    "/\\example.org/x",
    "https://example.org/x",
    "mailto:info@example.org",
    "rooms",
    "",
  ])("treats %s as foreign", (href) => {
    expect(isInternalPath(href)).toBe(false);
  });

  it("normalizes query encoding the way the router commit does", () => {
    expect(normalizedUrl("/lists", "?q=a%20b")).toBe("/lists?q=a+b");
    expect(normalizedUrl("/lists", "")).toBe("/lists");
  });

  it("resolves a same-origin target and rejects a foreign one", () => {
    window.history.replaceState({}, "", "/dienstplan?woche=2");

    expect(currentUrl()).toBe("/dienstplan?woche=2");
    expect(navigationTarget("rooms?b=1#teil")).toBe("/rooms?b=1");
    expect(navigationTarget("https://example.org/rooms")).toBeNull();
    expect(navigationTarget("//example.org/rooms")).toBeNull();
  });
});
