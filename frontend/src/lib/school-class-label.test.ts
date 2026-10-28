import { describe, expect, it } from "vitest";

import {
  schoolClassesLabel,
  schoolClassLabel,
  schoolClassName,
} from "./school-class-label";

describe("schoolClassLabel", () => {
  it("ergänzt das Präfix bei bloßen Klassennamen", () => {
    expect(schoolClassLabel("1a")).toBe("Klasse 1a");
  });

  it("verdoppelt ein vorhandenes Präfix nicht", () => {
    expect(schoolClassLabel("Klasse 1a")).toBe("Klasse 1a");
    // Seit #5101 schreibt die Beschriftung das Wort einheitlich groß, wie
    // das Backend (collation.SchoolClassLabel).
    expect(schoolClassLabel("klasse 1a")).toBe("Klasse 1a");
  });

  it("liefert bei leerem Namen nichts", () => {
    expect(schoolClassLabel("  ")).toBe("");
  });

  // #5101: die vier Schreibweisen aus dem Issue.
  it.each([
    ["1b", "Klasse 1b"],
    ["Klasse 1b", "Klasse 1b"],
    ["klasse 1b", "Klasse 1b"],
    [" 1b ", "Klasse 1b"],
  ])("beschriftet %j als %j", (stored, label) => {
    expect(schoolClassLabel(stored)).toBe(label);
  });

  it("erkennt das Wort auch ohne Leerzeichen oder am Ende", () => {
    expect(schoolClassLabel("Klasse1b")).toBe("Klasse 1b");
    expect(schoolClassLabel(" kLaSsE 3a ")).toBe("Klasse 3a");
    expect(schoolClassLabel("Klasse")).toBe("Klasse");
    expect(schoolClassLabel("1. Klasse")).toBe("1. Klasse");
    expect(schoolClassLabel(" Klasse 1b ")).toBe("Klasse 1b");
  });

  it("hält Namen, die nur mit „Klasse“ beginnen, für bloße Namen", () => {
    expect(schoolClassLabel("Klassenfahrt")).toBe("Klasse Klassenfahrt");
    expect(schoolClassLabel("Eulen")).toBe("Klasse Eulen");
  });
});

describe("schoolClassName", () => {
  it.each([
    ["1b", "1b"],
    ["Klasse 1b", "1b"],
    ["klasse 1b", "1b"],
    [" 1b ", "1b"],
    ["Klasse 3b", "3b"],
    ["1. Klasse", "1. Klasse"],
    ["Klassenfahrt", "Klassenfahrt"],
  ])("macht aus %j den Namen %j", (stored, name) => {
    expect(schoolClassName(stored)).toBe(name);
  });
});

describe("schoolClassesLabel", () => {
  it("beschriftet eine Klasse wie schoolClassLabel", () => {
    expect(schoolClassesLabel(["Klasse 1b"])).toBe("Klasse 1b");
    expect(schoolClassesLabel([" 1b "])).toBe("Klasse 1b");
  });

  it("setzt bei mehreren Klassen die Mehrzahl einmal davor", () => {
    expect(schoolClassesLabel(["1a", "Klasse 1b", "klasse 2c"])).toBe(
      "Klassen 1a, 1b, 2c",
    );
  });

  it("erhält einen Klassennamen, der nur aus dem Wort Klasse besteht", () => {
    expect(schoolClassesLabel(["Klasse", "1a"])).toBe("Klassen Klasse, 1a");
  });

  it("nimmt eine eigene Verbindung der Namen an", () => {
    expect(
      schoolClassesLabel(["Klasse 1a", "1b"], (names) => names.join(" und ")),
    ).toBe("Klassen 1a und 1b");
  });

  it("liefert ohne Klassen nichts", () => {
    expect(schoolClassesLabel([])).toBe("");
    expect(schoolClassesLabel(["  "])).toBe("");
  });
});
