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

describe("isSchoolNavActive auf dem Schul-Host", () => {
  it("markiert die Aufsichten auch ohne /school-Präfix (#2527)", () => {
    expect(isSchoolNavActive("/school/aufsichten", "/aufsichten")).toBe(true);
    expect(isSchoolNavActive("/school/aufsichten", "/school/aufsichten")).toBe(
      true,
    );
  });

  it("markiert die Klassenansicht nicht, wenn die Aufsichten offen sind", () => {
    expect(isSchoolNavActive("/school", "/aufsichten")).toBe(false);
  });

  it("verwechselt keine Pfade mit gleichem Anfang", () => {
    expect(isSchoolNavActive("/school/aufsichten", "/aufsichten-alt")).toBe(
      false,
    );
  });

  it("markiert die Klassenansicht auf einer Klassenseite (#2294)", () => {
    // Die Klassenseite liegt unter der Klassenansicht; ohne diesen Fall
    // stünde man auf einer Unterseite und die Navigation zeigte nirgends hin.
    // Der Klassenname steht in der Query, der Pfad endet auf /klasse.
    expect(isSchoolNavActive("/school", "/klasse")).toBe(true);
    expect(isSchoolNavActive("/school", "/school/klasse")).toBe(true);
  });

  it("verwechselt die Klassenseite nicht mit einem Pfad gleichen Anfangs", () => {
    expect(isSchoolNavActive("/school", "/klassenbuch")).toBe(false);
  });
});
