import { describe, expect, it } from "vitest";

import {
  DEFAULT_LAYOUTS,
  HOME_BLOCKS,
  appendPlacement,
  computeBoardCells,
  homeProfileFor,
  movePlacementBy,
  placePlacement,
  placementWithSpan,
  resolveHomeLayout,
  sortedPlacements,
  sanitizeHomeBlockPlacements,
  sanitizeHomeBlockPolicies,
  sanitizeHomeLayoutOverrides,
  withoutPlacement,
  type HomeBlockAccess,
  type HomeBlockContext,
  type HomeBlockKey,
  type HomeBlockPlacement,
} from "./home-blocks";

/** Jemand, der alles darf — der Betriebsmodus bleibt so die einzige Variable. */
const leadAccess: HomeBlockAccess = {
  isAdminScope: true,
  has: () => true,
  canOpenRequestsPage: true,
  caresForGroups: false,
  hasOwnGroups: false,
};

const careAccess: HomeBlockAccess = {
  isAdminScope: false,
  has: () => true,
  canOpenRequestsPage: true,
  caresForGroups: true,
  hasOwnGroups: true,
};

/** Führt die Einrichtung UND betreut selbst: bekommt die Vereinigung. */
const leadCareAccess: HomeBlockAccess = {
  isAdminScope: true,
  has: () => true,
  canOpenRequestsPage: true,
  caresForGroups: true,
  hasOwnGroups: true,
};

const fullContext: HomeBlockContext = {
  detailed: true,
  openCareGroupMode: false,
  nfcEnabled: true,
  birthdaysEnabled: true,
  timetableEnabled: true,
  remindersEnabled: true,
  messagingEnabled: true,
  staffMessagingEnabled: true,
  access: leadAccess,
};

const keysOf = (placements: readonly { key: HomeBlockKey }[]) =>
  placements.map((placement) => placement.key);

describe("resolveHomeLayout — Standardansicht", () => {
  it("zeigt ohne gespeicherte Anordnung die Ansicht der Leitung", () => {
    const { placements, customized } = resolveHomeLayout(
      fullContext,
      [],
      {},
      {},
    );

    expect(customized).toBe(false);
    expect(keysOf(placements)).toEqual(keysOf(DEFAULT_LAYOUTS.lead));
  });

  it("zeigt Betreuungskräften ihre eigene Standardansicht", () => {
    const { placements } = resolveHomeLayout(
      { ...fullContext, access: careAccess },
      [],
      {},
      {},
    );

    expect(homeProfileFor(careAccess)).toBe("care");
    expect(keysOf(placements)).toEqual(keysOf(DEFAULT_LAYOUTS.care));
    // Die Startseite ist ein Einstieg, keine Liste von allem: die
    // Standardansicht bleibt kurz, der Rest steht im Hinzufügen-Menü.
    expect(placements.length).toBeLessThanOrEqual(6);
    // Auf die Person zugeschnitten: der eigene Tag und die eigene Gruppe,
    // nicht die schulweite Betreuung.
    expect(keysOf(placements)).toContain("section.my_day");
    expect(keysOf(placements)).toContain("section.my_group");
    expect(keysOf(placements)).not.toContain("section.active_groups");
  });

  // Zwei Bausteine, die dieselben Zeilen zeigen, gehören nicht zusammen in
  // einen Standard: „Mein Tag" ist der eigene Ausschnitt des Ablaufs.
  it("stellt Mein Tag und Ablauf des Tages nicht zusammen auf, wer nur eines ist", () => {
    for (const profile of ["care", "lead"] as const) {
      const keys = keysOf(DEFAULT_LAYOUTS[profile]);
      expect(
        keys.includes("section.my_day") && keys.includes("section.day_flow"),
      ).toBe(false);
    }
  });

  // Das Issue verlangt für Mischrollen die Vereinigung: die Gruppenleitung
  // mit Adminrecht verliert ihren eigenen Tag nicht an die Leitungsansicht.
  it("gibt der Leitung, die selbst betreut, beides", () => {
    const { placements } = resolveHomeLayout(
      { ...fullContext, access: leadCareAccess },
      [],
      {},
      {},
    );

    expect(homeProfileFor(leadCareAccess)).toBe("lead_care");
    const keys = keysOf(placements);
    expect(keys[0]).toBe("section.my_day");
    expect(keys).toContain("section.my_group");
    expect(keys).toContain("section.open_requests");
    expect(keys).toContain("section.day_flow");
    expect(keys).toContain("tile.students_present");
  });

  // Fehlt ein Baustein der Standardansicht, bleibt kein Loch: der nächste
  // rückt in Lesereihenfolge nach. Sonst stünde „Meine Gruppe" allein links
  // und rechts daneben nichts, nur weil die Schule die Erinnerungen aus hat.
  it("packt die Standardansicht lückenlos, wenn ein Baustein fehlt", () => {
    const { placements } = resolveHomeLayout(
      { ...fullContext, access: careAccess, remindersEnabled: false },
      [],
      {},
      {},
    );

    const cell = (key: HomeBlockKey) => {
      const entry = placements.find((p) => p.key === key)!;
      return [entry.col, entry.row];
    };
    expect(keysOf(placements)).not.toContain("section.reminders");
    expect(cell("section.my_day")).toEqual([0, 0]);
    expect(cell("section.my_group")).toEqual([0, 3]);
    // Die Tagesinformationen nehmen den Platz der Erinnerungen.
    expect(cell("section.staff_notices")).toEqual([2, 3]);
    expect(cell("section.birthdays")).toEqual([0, 5]);
  });

  // Ohne eigene Gruppe wäre „Meine Gruppe" eine Karte, die nur sagt, dass sie
  // leer ist.
  it("lässt Meine Gruppe weg, wer heute keine Gruppe hat", () => {
    const { placements, available } = resolveHomeLayout(
      { ...fullContext, access: { ...careAccess, hasOwnGroups: false } },
      [],
      {},
      {},
    );

    expect(keysOf(placements)).not.toContain("section.my_group");
    expect(available.map((block) => block.key)).not.toContain(
      "section.my_group",
    );
  });

  it("bietet die Nachrichten nur an, wo die Schule eine der beiden Arten eingeschaltet hat", () => {
    const off = resolveHomeLayout(
      { ...fullContext, messagingEnabled: false, staffMessagingEnabled: false },
      [],
      {},
      {},
    );
    expect(off.available.map((block) => block.key)).not.toContain(
      "section.messages",
    );

    const teamOnly = resolveHomeLayout(
      { ...fullContext, messagingEnabled: false },
      [],
      {},
      {},
    );
    expect(teamOnly.available.map((block) => block.key)).toContain(
      "section.messages",
    );
  });

  it("lässt jeden Baustein der Standardansicht auch wirklich existieren", () => {
    const known = new Set(HOME_BLOCKS.map((block) => block.key));
    for (const layout of Object.values(DEFAULT_LAYOUTS)) {
      for (const placement of layout) {
        expect(known.has(placement.key)).toBe(true);
      }
    }
  });

  // Eine Karte, die dauerhaft „ist ausgeschaltet" sagt, belegt nur einen Platz.
  it("lässt die Erinnerungen weg, wenn die Schule keine eingeschaltet hat", () => {
    const { placements, available, addable } = resolveHomeLayout(
      { ...fullContext, access: careAccess, remindersEnabled: false },
      [],
      {},
      {},
    );

    expect(keysOf(placements)).not.toContain("section.reminders");
    expect(available.map((block) => block.key)).not.toContain(
      "section.reminders",
    );
    expect(addable.map((block) => block.key)).not.toContain(
      "section.reminders",
    );
  });

  it("lässt Raum- und Aktivitätsbausteine im Modus 'binary' weg", () => {
    const { available } = resolveHomeLayout(
      { ...fullContext, detailed: false },
      [],
      {},
      {},
    );

    const keys = available.map((block) => block.key);
    expect(keys).not.toContain("tile.students_in_rooms");
    expect(keys).not.toContain("section.recent_activity");
    expect(keys).not.toContain("section.day_flow");
    expect(keys).toContain("tile.students_present");
  });
});

describe("resolveHomeLayout — eigene Anordnung", () => {
  it("übernimmt Reihenfolge und Breite aus dem Gespeicherten", () => {
    const { placements, customized } = resolveHomeLayout(
      fullContext,
      [
        { key: "section.staff_notices", span: 4, col: 0, row: 0 },
        { key: "tile.students_sick", span: 1, col: 0, row: 0 },
      ],
      {},
      {},
    );

    expect(placements[0]).toEqual({
      key: "section.staff_notices",
      span: 4,
      col: 0,
      row: 0,
    });
    // Beide auf derselben Zelle gespeichert: die Kennzahl rückt unter die
    // Karte, die zwei Zeilen hoch ist.
    expect(placements[1]).toEqual({
      key: "tile.students_sick",
      span: 1,
      col: 0,
      row: 2,
    });
    expect(customized).toBe(true);
  });

  it("hängt Bausteine der Standardansicht hinten an, die noch nie angeordnet wurden", () => {
    // So erreicht ein später ergänzter Baustein auch ein Konto, das seine
    // Startseite längst eingerichtet hat.
    const { placements } = resolveHomeLayout(
      fullContext,
      [{ key: "section.staff_notices", span: 2, col: 0, row: 0 }],
      {},
      {},
    );

    expect(placements[0]?.key).toBe("section.staff_notices");
    expect(keysOf(placements)).toContain("section.open_requests");
  });

  it("lässt entfernte Bausteine entfernt", () => {
    const { placements, addable } = resolveHomeLayout(
      fullContext,
      [{ key: "section.staff_notices", span: 2, col: 0, row: 0 }],
      { "section.open_requests": false },
      {},
    );

    expect(keysOf(placements)).not.toContain("section.open_requests");
    expect(addable.map((block) => block.key)).toContain(
      "section.open_requests",
    );
  });

  it("korrigiert eine Breite, die es für den Baustein nicht gibt", () => {
    const { placements } = resolveHomeLayout(
      fullContext,
      [{ key: "tile.students_sick", span: 4, col: 0, row: 0 }],
      {},
      {},
    );

    expect(placements[0]).toEqual({
      key: "tile.students_sick",
      span: 1,
      col: 0,
      row: 0,
    });
  });

  it("stellt einen Baustein nicht zweimal auf die Fläche", () => {
    const { placements } = resolveHomeLayout(
      fullContext,
      [
        { key: "section.staff_notices", span: 2, col: 0, row: 0 },
        { key: "section.staff_notices", span: 4, col: 0, row: 0 },
      ],
      {},
      {},
    );

    expect(
      keysOf(placements).filter((key) => key === "section.staff_notices"),
    ).toHaveLength(1);
  });
});

describe("resolveHomeLayout — Berechtigung", () => {
  const accessWith = (
    permissions: readonly string[],
    canOpenRequestsPage = false,
  ): HomeBlockAccess => ({
    isAdminScope: false,
    has: (permission: string) => permissions.includes(permission),
    canOpenRequestsPage,
    caresForGroups: true,
    hasOwnGroups: true,
  });

  const resolveFor = (
    permissions: readonly string[],
    canOpenRequestsPage = false,
  ) =>
    resolveHomeLayout(
      { ...fullContext, access: accessWith(permissions, canOpenRequestsPage) },
      [],
      {},
      {},
    );

  it("lässt ohne groups:read jede Kennzahl aus den Betriebszahlen weg", () => {
    const { available, placements } = resolveFor(["users:read"]);

    const keys = available.map((block) => block.key);
    expect(keys).not.toContain("tile.students_present");
    expect(keys).not.toContain("section.active_groups");
    expect(keysOf(placements)).not.toContain("tile.students_present");
    // users:read trägt die Tagesinformationen — die bleiben.
    expect(keysOf(placements)).toContain("section.staff_notices");
  });

  // Dieselbe Quelle wie der Tagesplan, also dasselbe Recht.
  it("zeigt Mein Tag nur mit schedules:read", () => {
    expect(keysOf(resolveFor([]).placements)).not.toContain("section.my_day");
    expect(keysOf(resolveFor(["schedules:read"]).placements)).toContain(
      "section.my_day",
    );
  });

  // In der Betreuungsansicht steht der Ablauf des Tages nicht: er zeigt
  // dieselben Blöcke wie „Mein Tag". Anbieten lässt er sich trotzdem, sobald
  // das Recht reicht.
  it("bietet den Ablauf des Tages nur mit schedules:read an", () => {
    expect(resolveFor([]).addable.map((block) => block.key)).not.toContain(
      "section.day_flow",
    );
    expect(
      resolveFor(["schedules:read"]).addable.map((block) => block.key),
    ).toContain("section.day_flow");
  });

  // In der Betreuungsansicht stehen die offenen Anfragen nicht von Haus aus:
  // sie betreffen nur Gruppenleitungen, die sie sich dazuholen können.
  it("bietet offene Anfragen nur an, wer das Anfragen-Modul öffnen darf", () => {
    expect(resolveFor([]).addable.map((block) => block.key)).not.toContain(
      "section.open_requests",
    );
    expect(resolveFor([], true).addable.map((block) => block.key)).toContain(
      "section.open_requests",
    );
  });

  it("stellt der Leitung die offenen Anfragen von Haus aus auf", () => {
    const { placements } = resolveHomeLayout(
      {
        ...fullContext,
        access: { ...accessWith([], true), isAdminScope: true },
      },
      [],
      {},
      {},
    );

    expect(keysOf(placements)).toContain("section.open_requests");
  });

  it("stellt einen Baustein ohne Recht auch dann nicht auf, wenn er angeordnet ist", () => {
    const { placements, addable } = resolveHomeLayout(
      { ...fullContext, access: accessWith([]) },
      [{ key: "tile.students_present", span: 1, col: 0, row: 0 }],
      {},
      {},
    );

    expect(keysOf(placements)).not.toContain("tile.students_present");
    expect(addable.map((block) => block.key)).not.toContain(
      "tile.students_present",
    );
  });
});

describe("resolveHomeLayout — Vorgabe der Einrichtung", () => {
  it("stellt einen verpflichtenden Baustein auf, auch wenn er entfernt wurde", () => {
    const { placements, addable } = resolveHomeLayout(
      fullContext,
      [{ key: "section.staff_notices", span: 2, col: 0, row: 0 }],
      { "section.birthdays": false },
      { "section.birthdays": "required" },
    );

    expect(keysOf(placements)).toContain("section.birthdays");
    expect(addable.map((block) => block.key)).not.toContain(
      "section.birthdays",
    );
  });

  it("lässt einen abgeschalteten Baustein trotz Anordnung weg", () => {
    const { placements, addable } = resolveHomeLayout(
      fullContext,
      [{ key: "tile.students_sick", span: 1, col: 0, row: 0 }],
      {},
      { "tile.students_sick": "disabled" },
    );

    expect(keysOf(placements)).not.toContain("tile.students_sick");
    expect(addable.map((block) => block.key)).not.toContain(
      "tile.students_sick",
    );
  });

  it("behandelt 'optional' wie gar keine Vorgabe", () => {
    const { placements } = resolveHomeLayout(
      fullContext,
      [],
      {},
      { "section.staff_notices": "optional" },
    );

    expect(keysOf(placements)).toContain("section.staff_notices");
  });
});

describe("sanitize", () => {
  it("verwirft unbekannte Schlüssel und falsche Werttypen", () => {
    expect(
      sanitizeHomeLayoutOverrides({
        "section.birthdays": false,
        "tile.does_not_exist": true,
        "tile.students_sick": "ja",
      }),
    ).toEqual({ "section.birthdays": false });
  });

  it("verwirft unbekannte Vorgabe-Werte", () => {
    expect(
      sanitizeHomeBlockPolicies({
        "section.birthdays": "disabled",
        "tile.students_sick": "mandatory",
      }),
    ).toEqual({ "section.birthdays": "disabled" });
  });

  it("räumt eine gespeicherte Anordnung auf", () => {
    expect(
      sanitizeHomeBlockPlacements([
        { key: "section.staff_notices", span: 4, col: 0, row: 0 },
        { key: "section.staff_notices", span: 2, col: 0, row: 0 },
        { key: "tile.does_not_exist", span: 1, col: 0, row: 0 },
        { key: "tile.students_sick", span: 3 },
        "kaputt",
        null,
      ]),
    ).toEqual([
      { key: "section.staff_notices", span: 4, col: 0, row: 0 },
      // Eine Breite, die es für eine Kennzahl nicht gibt, wird korrigiert;
      // ohne Zelle landet sie oben links und rückt unter die Karte dort.
      { key: "tile.students_sick", span: 1, col: 0, row: 2 },
    ]);
  });

  // Anordnungen von vor dem freien Raster kennen keine Spalte: sie werden in
  // ihrer Reihenfolge von oben links her gepackt, wie das Raster sie damals
  // zeichnete — auch in die Lücke unter zwei Kennzahlen.
  it("packt eine Anordnung ohne Zellen in Reihenfolge", () => {
    const packed = sanitizeHomeBlockPlacements([
      { key: "tile.students_present", span: 1 },
      { key: "tile.students_sick", span: 1 },
      { key: "section.open_requests", span: 2 },
      { key: "section.staff_today", span: 2 },
      { key: "section.birthdays", span: 4 },
    ]);

    expect(packed.map((entry) => [entry.col, entry.row])).toEqual([
      [0, 0],
      [1, 0],
      [2, 0],
      [0, 1],
      [0, 3],
    ]);
  });

  // Gespeicherte Zellen sind nur die Reihenfolge: gepackt wird lückenlos,
  // auch wenn eine ältere Anordnung noch Löcher hatte.
  it("nimmt die Reihenfolge der gespeicherten Zellen und packt lückenlos", () => {
    const kept = sanitizeHomeBlockPlacements([
      { key: "tile.students_present", span: 1, col: 3, row: 2 },
      { key: "section.open_requests", span: 2, col: 0, row: 5 },
      { key: "section.staff_today", span: 2, col: 2, row: 5 },
    ]);

    expect(kept.map((entry) => [entry.col, entry.row])).toEqual([
      [0, 0],
      [1, 0],
      [0, 2],
    ]);
  });

  it("holt eine Kachel zurück ins Raster, die darüber hinausragt", () => {
    expect(
      sanitizeHomeBlockPlacements([
        { key: "section.open_requests", span: 2, col: 3, row: 0 },
      ]),
    ).toEqual([{ key: "section.open_requests", span: 2, col: 0, row: 0 }]);
  });

  it("verträgt null und Nicht-Objekte", () => {
    expect(sanitizeHomeLayoutOverrides(null)).toEqual({});
    expect(sanitizeHomeLayoutOverrides("nope")).toEqual({});
    expect(sanitizeHomeBlockPolicies(undefined)).toEqual({});
    expect(sanitizeHomeBlockPlacements("nope")).toEqual([]);
  });
});

describe("Nahtloses Raster (#2180)", () => {
  // Zwei Kennzahlen, daneben die Anfragen, darunter das Personal, dann die
  // Geburtstage über die volle Breite — so, wie das Raster diese Reihenfolge
  // packt. Kein Loch.
  const grid: HomeBlockPlacement[] = [
    { key: "tile.students_present", span: 1, col: 0, row: 0 },
    { key: "tile.students_sick", span: 1, col: 1, row: 0 },
    { key: "section.open_requests", span: 2, col: 2, row: 0 },
    { key: "section.staff_today", span: 2, col: 0, row: 1 },
    { key: "section.birthdays", span: 4, col: 0, row: 3 },
  ];
  const cellOf = (
    placements: readonly HomeBlockPlacement[],
    key: HomeBlockKey,
  ) => {
    const entry = placements.find((placement) => placement.key === key)!;
    return [entry.col, entry.row];
  };

  it("legt einen Baustein vor die Kachel, über der er losgelassen wird", () => {
    const next = placePlacement(grid, "section.birthdays", { col: 0, row: 0 });

    expect(cellOf(next, "section.birthdays")).toEqual([0, 0]);
    // Alles, was vorher davor lag, rutscht dahinter — lückenlos.
    expect(cellOf(next, "tile.students_present")).toEqual([0, 2]);
    expect(cellOf(next, "tile.students_sick")).toEqual([1, 2]);
    expect(cellOf(next, "section.open_requests")).toEqual([2, 2]);
    expect(cellOf(next, "section.staff_today")).toEqual([0, 3]);
  });

  // Genau das Bild aus der Rückmeldung: keine Kachel hängt allein irgendwo,
  // keine leere Zelle vor der letzten Kachel.
  it("lässt keine Lücke, wenn ein Baustein ans Ende zieht", () => {
    const next = placePlacement(grid, "tile.students_present", {
      col: 0,
      row: 10,
    });

    expect(cellOf(next, "tile.students_sick")).toEqual([0, 0]);
    expect(cellOf(next, "section.open_requests")).toEqual([1, 0]);
    // Die Kennzahl steht in der Reihenfolge zuletzt und füllt die Zelle,
    // die dabei frei wird — statt allein in einer Zeile unter allem.
    expect(cellOf(next, "tile.students_present")).toEqual([3, 0]);
    expect(cellOf(next, "section.staff_today")).toEqual([0, 2]);
    expect(cellOf(next, "section.birthdays")).toEqual([0, 4]);
  });

  it("gibt dieselbe Anordnung zurück, wenn sich nichts ändert", () => {
    expect(placePlacement(grid, "tile.students_sick", { col: 1, row: 0 })).toBe(
      grid,
    );
    expect(placementWithSpan(grid, "section.staff_today", 2)).toBe(grid);
    expect(movePlacementBy(grid, "tile.students_present", "left")).toBe(grid);
  });

  it("packt nach einer Breitenänderung in derselben Reihenfolge neu", () => {
    const next = placementWithSpan(grid, "section.open_requests", 4);

    expect(next.find((entry) => entry.key === "section.open_requests")).toEqual(
      { key: "section.open_requests", span: 4, col: 0, row: 1 },
    );
    expect(cellOf(next, "section.staff_today")).toEqual([0, 3]);
    expect(cellOf(next, "section.birthdays")).toEqual([0, 5]);
  });

  it("rückt nach, wenn ein Baustein entfernt wird", () => {
    const next = withoutPlacement(grid, "tile.students_present");

    expect(next).toHaveLength(4);
    expect(cellOf(next, "tile.students_sick")).toEqual([0, 0]);
    expect(cellOf(next, "section.open_requests")).toEqual([1, 0]);
    expect(cellOf(next, "section.staff_today")).toEqual([0, 2]);
    expect(cellOf(next, "section.birthdays")).toEqual([0, 4]);
  });

  it("hängt einen Baustein dort an, wo er als Nächstes passt", () => {
    expect(appendPlacement(grid, "section.messages", 2).at(-1)).toEqual({
      key: "section.messages",
      span: 2,
      col: 0,
      row: 5,
    });
    expect(
      appendPlacement(grid.slice(0, 2), "section.messages", 2).at(-1),
    ).toEqual({ key: "section.messages", span: 2, col: 2, row: 0 });
  });

  it("rückt eine Kachel ohne Maus in der Reihenfolge", () => {
    const left = movePlacementBy(grid, "tile.students_sick", "left");
    expect(cellOf(left, "tile.students_sick")).toEqual([0, 0]);
    expect(cellOf(left, "tile.students_present")).toEqual([1, 0]);

    const right = movePlacementBy(grid, "tile.students_sick", "right");
    expect(cellOf(right, "section.open_requests")).toEqual([1, 0]);
    expect(cellOf(right, "tile.students_sick")).toEqual([3, 0]);

    // Oben: vor die Kachel, die in derselben Spalte darüber steht.
    const up = movePlacementBy(grid, "section.staff_today", "up");
    expect(cellOf(up, "section.staff_today")).toEqual([0, 0]);
    expect(cellOf(up, "tile.students_present")).toEqual([2, 0]);

    // Unten: hinter die Kachel, die in derselben Spalte darunter steht.
    const down = movePlacementBy(grid, "tile.students_present", "down");
    expect(cellOf(down, "tile.students_sick")).toEqual([0, 0]);
    expect(cellOf(down, "tile.students_present")).toEqual([3, 0]);
  });

  it("speichert in Lesereihenfolge", () => {
    const shuffled = [grid[4]!, grid[2]!, grid[0]!, grid[3]!, grid[1]!];

    expect(sortedPlacements(shuffled).map((entry) => entry.key)).toEqual(
      grid.map((entry) => entry.key),
    );
  });

  it("rechnet die Anordnung in Rasterzellen um", () => {
    const cells = computeBoardCells(grid, 4);

    expect(cells.get("tile.students_sick")).toEqual({
      columnStart: 2,
      columnSpan: 1,
      rowStart: 1,
      rowSpan: 1,
    });
    expect(cells.get("section.staff_today")).toEqual({
      columnStart: 1,
      columnSpan: 2,
      rowStart: 2,
      rowSpan: 2,
    });
    expect(cells.get("section.birthdays")).toEqual({
      columnStart: 1,
      columnSpan: 4,
      rowStart: 4,
      rowSpan: 2,
    });
  });

  // Zwei Spalten: dieselbe Reihenfolge, schmaler gepackt.
  it("packt auf einem schmalen Raster in derselben Reihenfolge", () => {
    const cells = computeBoardCells(grid, 2);

    expect(cells.get("section.open_requests")).toEqual({
      columnStart: 1,
      columnSpan: 2,
      rowStart: 2,
      rowSpan: 2,
    });
    expect(cells.get("section.staff_today")).toEqual({
      columnStart: 1,
      columnSpan: 2,
      rowStart: 4,
      rowSpan: 2,
    });
    expect(cells.get("section.birthdays")).toEqual({
      columnStart: 1,
      columnSpan: 2,
      rowStart: 6,
      rowSpan: 2,
    });
  });
});
