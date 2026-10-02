import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";

import {
  BlockStaffNames,
  blockDetailLines,
  blockPlaceLine,
  blockStaffEntries,
  shortStaffName,
  staffNamesFromOverview,
} from "./block-staff-names";
import type { InstanceStaffSummary } from "~/lib/timetable-types";

function row(
  staffId: string,
  flags: Partial<Omit<InstanceStaffSummary, "staffId">> = {},
): InstanceStaffSummary {
  return {
    staffId,
    isPrimary: false,
    isAbsent: false,
    isSubstitute: false,
    ...flags,
  };
}

const NAMES = new Map([
  ["1", "Anna Kowalski"],
  ["2", "Ben Müller"],
  ["3", "Clara Schmidt"],
  ["4", "Dana Weber"],
  ["5", "Emil Yilmaz"],
]);

describe("shortStaffName", () => {
  it("kürzt den Nachnamen auf den Anfangsbuchstaben", () => {
    expect(shortStaffName("Anna Kowalski")).toBe("Anna K.");
    expect(shortStaffName("  Anna   von Berg ")).toBe("Anna B.");
  });

  it("lässt ein einzelnes Wort stehen", () => {
    expect(shortStaffName("Hausmeister")).toBe("Hausmeister");
  });
});

describe("blockStaffEntries", () => {
  it("ordnet Abwesende vor Ersatz vor Betreuenden, Hauptkraft zuerst", () => {
    const entries = blockStaffEntries(
      [
        row("1", { isAbsent: true }),
        row("2", { isSubstitute: true }),
        row("3"),
        row("4", { isPrimary: true }),
      ],
      NAMES,
    );
    expect(entries.map((entry) => entry.label)).toEqual([
      "Anna K.",
      "Ben M.",
      "Dana W.",
      "Clara S.",
    ]);
  });

  it("lässt eine wieder entfernte Ersatzkraft und unbekannte Namen weg", () => {
    const entries = blockStaffEntries(
      [row("2", { isSubstitute: true, isAbsent: true }), row("99"), row("3")],
      NAMES,
    );
    expect(entries.map((entry) => entry.staffId)).toEqual(["3"]);
  });

  it("zeigt den vollen Namen, wenn zwei Kurzformen gleich wären", () => {
    const entries = blockStaffEntries(
      [row("1"), row("7")],
      new Map([
        ["1", "Anna Kowalski"],
        ["7", "Anna Krüger"],
      ]),
    );
    expect(entries.map((entry) => entry.label)).toEqual([
      "Anna Kowalski",
      "Anna Krüger",
    ]);
  });
});

describe("blockPlaceLine und blockDetailLines", () => {
  it("verbindet Raum und Gruppe und lässt Fehlendes weg", () => {
    expect(blockPlaceLine({ roomName: "Raum 104", groupName: "Sonne" })).toBe(
      "Raum 104 · Sonne",
    );
    expect(blockPlaceLine({ roomName: "Raum 104" })).toBe("Raum 104");
    expect(blockPlaceLine({ roomName: "" })).toBe("");
  });

  it("nennt im Tooltip alle Fachkräfte mit vollem Namen und Rolle", () => {
    const entries = blockStaffEntries(
      [row("1", { isAbsent: true }), row("2", { isSubstitute: true })],
      NAMES,
    );
    expect(
      blockDetailLines({ roomName: "Raum 104", groupName: "Sonne" }, entries),
    ).toEqual([
      "Raum: Raum 104",
      "Gruppe: Sonne",
      "Fachkräfte: Anna Kowalski (abwesend), Ben Müller (Ersatz)",
    ]);
  });
});

describe("staffNamesFromOverview", () => {
  it("sammelt Namen aus den Terminen und den möglichen Ersatzkräften", () => {
    const names = staffNamesFromOverview({
      appointments: [
        {
          id: "10",
          date: "2026-09-09",
          startTime: "13:00",
          endTime: "14:00",
          title: "Mensa",
          status: "planned",
          staff: [
            {
              assignmentId: "1",
              id: "1",
              name: "Anna Kowalski",
              isAbsent: false,
              isSubstitute: false,
              canEnd: false,
            },
          ],
        },
      ],
      staff: [{ id: "2", name: "Ben Müller" }],
    });
    expect(names.get("1")).toBe("Anna Kowalski");
    expect(names.get("2")).toBe("Ben Müller");
    expect(staffNamesFromOverview(undefined).size).toBe(0);
  });
});

describe("BlockStaffNames", () => {
  it("markiert die Ersatzkraft grün mit „(Ersatz)“", () => {
    const entries = blockStaffEntries(
      [row("2", { isSubstitute: true }), row("3")],
      NAMES,
    );
    render(<BlockStaffNames entries={entries} />);

    expect(screen.getByText(/Clara S\./)).toBeInTheDocument();
    expect(screen.getByText("Ben M. (Ersatz)")).toHaveClass(
      "text-moto-green-strong",
    );
    expect(screen.queryByText(/^\+/)).not.toBeInTheDocument();
  });

  it("fasst ab der dritten Person zu „+N“ zusammen, Abweichungen zuerst", () => {
    const entries = blockStaffEntries(
      [
        row("1", { isAbsent: true }),
        row("2", { isSubstitute: true }),
        row("3"),
        row("4"),
        row("5"),
      ],
      NAMES,
    );
    render(<BlockStaffNames entries={entries} />);

    // Die Abwesende und die Ersatzkraft bleiben sichtbar; wer regulär
    // betreut, fällt unter „+3“ und steht im Tooltip.
    expect(screen.getByText(/Anna K\./)).toHaveClass("line-through");
    expect(screen.getByText("Ben M. (Ersatz)")).toBeInTheDocument();
    expect(screen.queryByText(/Clara S\./)).not.toBeInTheDocument();
    expect(screen.getByText("+3")).toBeInTheDocument();
  });

  it("streicht Abwesende durch und sagt es dem Screenreader", () => {
    const entries = blockStaffEntries([row("1", { isAbsent: true })], NAMES);
    render(<BlockStaffNames entries={entries} />);

    const absent = screen.getByText("Anna K.", { exact: false });
    expect(absent).toHaveClass("line-through", "text-moto-red-strong");
    expect(absent).toHaveTextContent("Anna K. (abwesend)");
  });

  it("rendert ohne Namen nichts", () => {
    const { container } = render(<BlockStaffNames entries={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});
