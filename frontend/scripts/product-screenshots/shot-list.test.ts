import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { devicesFor, parseShotList, selectShots } from "./shot-list";

const VALID = `
shots:
  - id: anwesenheit-uebersicht
    titel: Anwesenheit im Überblick
    portal: tenant
    rolle: admin
    pfad: /home
  - id: kinder-zuhause
    titel: Kinder im Blick
    portal: eltern
    rolle: eltern
    pfad: /children
    geraete: [iphone]
    vorbereitung:
      - warten_auf: "main h1"
      - klicken: "text=Filter"
      - scrollen: 400
      - scrollen: "footer"
`;

describe("Shot-Liste", () => {
  it("lädt gültige Shots", () => {
    const shots = parseShotList(VALID);
    expect(shots.map((shot) => shot.id)).toEqual([
      "anwesenheit-uebersicht",
      "kinder-zuhause",
    ]);
    expect(shots[1]?.vorbereitung).toEqual([
      { warten_auf: "main h1" },
      { klicken: "text=Filter" },
      { scrollen: 400 },
      { scrollen: "footer" },
    ]);
  });

  it("nimmt die Geräte des Portals, wenn der Shot keine nennt", () => {
    const [tenant, eltern] = parseShotList(VALID);
    expect(devicesFor(tenant!)).toEqual(["macbook", "ipad"]);
    // Der Shot überschreibt den Eltern-Standard (iPhone + iPad).
    expect(devicesFor(eltern!)).toEqual(["iphone"]);
    const withDefault = parseShotList(
      "shots:\n  - {id: a, titel: A, portal: eltern, rolle: eltern, pfad: /}",
    );
    expect(devicesFor(withDefault[0]!)).toEqual(["iphone", "ipad"]);
  });

  it("weist unbekannte Felder ab", () => {
    expect(() =>
      parseShotList(
        "shots:\n  - {id: a, titel: A, portal: tenant, rolle: admin, pfad: /, geraet: [ipad]}",
      ),
    ).toThrow(/geraet/);
  });

  it("weist unbekannte Felder in Vorbereitungsschritten ab", () => {
    expect(() =>
      parseShotList(
        "shots:\n  - {id: a, titel: A, portal: tenant, rolle: admin, pfad: /, vorbereitung: [{tippen: x}]}",
      ),
    ).toThrow(/Ungültige Shot-Liste/);
  });

  it.each([
    [
      "eine ID mit Umlaut",
      "{id: Übersicht, titel: A, portal: tenant, rolle: admin, pfad: /}",
    ],
    [
      "eine ID mit Unterstrich",
      "{id: kinder_liste, titel: A, portal: tenant, rolle: admin, pfad: /}",
    ],
    [
      "einen Pfad ohne Schrägstrich",
      "{id: a, titel: A, portal: tenant, rolle: admin, pfad: dashboard}",
    ],
    [
      "eine Adresse statt eines Pfads",
      "{id: a, titel: A, portal: tenant, rolle: admin, pfad: //example.com}",
    ],
    [
      "ein unbekanntes Gerät",
      "{id: a, titel: A, portal: tenant, rolle: admin, pfad: /, geraete: [galaxy]}",
    ],
    [
      "eine Rolle, die nicht zum Portal passt",
      "{id: a, titel: A, portal: tenant, rolle: eltern, pfad: /}",
    ],
    [
      "ein doppeltes Gerät",
      "{id: a, titel: A, portal: tenant, rolle: admin, pfad: /, geraete: [ipad, ipad]}",
    ],
    [
      "eine leere Geräteliste",
      "{id: a, titel: A, portal: tenant, rolle: admin, pfad: /, geraete: []}",
    ],
  ])("weist %s ab", (_name, entry) => {
    expect(() => parseShotList(`shots:\n  - ${entry}`)).toThrow(
      /Ungültige Shot-Liste/,
    );
  });

  it("weist doppelte IDs ab", () => {
    const entry = "{id: a, titel: A, portal: tenant, rolle: admin, pfad: /}";
    expect(() => parseShotList(`shots:\n  - ${entry}\n  - ${entry}`)).toThrow(
      /mehrfach/,
    );
  });

  it("weist eine leere Liste und kaputtes YAML ab", () => {
    expect(() => parseShotList("shots: []")).toThrow(/Ungültige Shot-Liste/);
    expect(() => parseShotList("shots: [")).toThrow(/kein gültiges YAML/);
  });

  it("wählt Shots nach ID und bricht bei einer unbekannten ID ab", () => {
    const shots = parseShotList(VALID);
    expect(selectShots(shots, [])).toHaveLength(2);
    expect(selectShots(shots, ["kinder-zuhause"]).map((s) => s.id)).toEqual([
      "kinder-zuhause",
    ]);
    expect(() => selectShots(shots, ["gibt-es-nicht"])).toThrow(
      /Unbekannte Shot-ID: gibt-es-nicht/,
    );
  });

  it("die Shot-Liste im Repo ist gültig", () => {
    const yamlText = readFileSync(
      join(import.meta.dirname, "shots.yaml"),
      "utf8",
    );
    expect(parseShotList(yamlText).length).toBeGreaterThanOrEqual(2);
  });
});
