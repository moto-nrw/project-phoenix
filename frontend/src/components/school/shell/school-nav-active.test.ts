import { describe, expect, it } from "vitest";

import { isSchoolNavActive } from "./school-nav-active";

describe("isSchoolNavActive", () => {
  it("markiert die Klassenansicht auf dem Schul-Host, wo der Pfad nur / ist", () => {
    expect(isSchoolNavActive("/school", "/")).toBe(true);
  });

  it("markiert die Klassenansicht auch beim internen /school-Pfad", () => {
    expect(isSchoolNavActive("/school", "/school")).toBe(true);
  });

  it("markiert die Klassenansicht nicht auf der Hilfe", () => {
    expect(isSchoolNavActive("/school", "/help")).toBe(false);
  });

  it("markiert die Hilfe samt Unterseiten", () => {
    expect(isSchoolNavActive("/help", "/help")).toBe(true);
    expect(isSchoolNavActive("/help", "/help/setup")).toBe(true);
    expect(isSchoolNavActive("/help", "/helpdesk")).toBe(false);
  });
});
