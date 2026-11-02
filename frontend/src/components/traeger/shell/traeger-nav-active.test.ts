import { describe, expect, it } from "vitest";
import { isTraegerNavActive } from "./traeger-nav-active";

describe("isTraegerNavActive", () => {
  it("marks Schulen on the start page of both spellings only", () => {
    expect(isTraegerNavActive("/traeger", "/")).toBe(true);
    expect(isTraegerNavActive("/traeger", "/traeger")).toBe(true);
    expect(isTraegerNavActive("/traeger", "/ferien")).toBe(false);
    expect(isTraegerNavActive("/traeger", "/traeger/ferien")).toBe(false);
  });

  it("marks a page on the Träger host and below /traeger", () => {
    expect(isTraegerNavActive("/traeger/ferien", "/ferien")).toBe(true);
    expect(isTraegerNavActive("/traeger/ferien", "/traeger/ferien")).toBe(true);
  });

  it("keeps the area marked on its sub-pages", () => {
    expect(
      isTraegerNavActive("/traeger/ferien", "/ferien/5/anmeldungen/9"),
    ).toBe(true);
    expect(
      isTraegerNavActive("/traeger/abrechnung", "/traeger/abrechnung/3"),
    ).toBe(true);
  });

  it("does not mark a page whose path only starts the same", () => {
    expect(isTraegerNavActive("/traeger/accounts", "/access-log")).toBe(false);
    expect(isTraegerNavActive("/traeger/ferien", "/ferienwochen")).toBe(false);
  });
});
