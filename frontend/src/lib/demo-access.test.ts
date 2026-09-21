import { describe, expect, it } from "vitest";
import { isDemoEntryPath } from "./demo-access";

describe("isDemoEntryPath", () => {
  it("matches the entry page on a subdomain and in path mode", () => {
    expect(isDemoEntryPath("/demo", "messe-demo")).toBe(true);
    expect(isDemoEntryPath("/demo/", undefined)).toBe(true);
    expect(isDemoEntryPath("/messe-demo/demo", "messe-demo")).toBe(true);
  });

  it("matches nothing else", () => {
    expect(isDemoEntryPath(null, "messe-demo")).toBe(false);
    expect(isDemoEntryPath("/", "messe-demo")).toBe(false);
    expect(isDemoEntryPath("/demo/settings", "messe-demo")).toBe(false);
    expect(isDemoEntryPath("/other/demo", "messe-demo")).toBe(false);
  });
});
