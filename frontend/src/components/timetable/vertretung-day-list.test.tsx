import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { computeStaffTriple, VertretungDayList } from "./vertretung-day-list";
import type { EnrichedInstance, GapInstance } from "~/lib/timetable-types";

const DAY = "2026-07-20";

function makeInstance(
  overrides: Partial<EnrichedInstance> & { id: string },
): EnrichedInstance {
  return {
    date: DAY,
    startTime: "09:00",
    endTime: "10:00",
    title: "Termin",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "care",
    roomId: "1",
    roomName: "Raum 1",
    staff: [],
    studentIds: [],
    students: [],
    staffCount: 0,
    absentStaffCount: 0,
    expectedStudentsCount: 0,
    notScheduledStudentsCount: 0,
    presentStudentsCount: 0,
    requiredStaffCount: 0,
    assignedStaffCount: 0,
    conflictWarnings: [],
    ...overrides,
  };
}

function makeGap(
  overrides: Partial<GapInstance> & { instanceId: string },
): GapInstance {
  return {
    date: DAY,
    title: "Termin",
    startTime: "09:00",
    endTime: "10:00",
    roomId: "1",
    status: "planned",
    assignedStaffCount: 0,
    absentStaffCount: 0,
    ...overrides,
  };
}

// Bedingung 1: offene Lücke, unbesetzte
// Position ohne Ersatz. 09:00, damit die Sortierung mit der nächsten Fixture
// bei Gleichstand die Schwere prüft.
const gapInstance = makeInstance({
  id: "10",
  startTime: "09:00",
  endTime: "10:00",
  title: "Frühstück",
  staff: [
    {
      staffId: "1",
      isPrimary: true,
      isAbsent: true,
      isSubstitute: false,
      absenceReason: "krank",
    },
  ],
});

// Bedingung 4 ohne Bedingung 1: Abwesenheit mit gesetztem, deckendem Ersatz —
// die Störung bleibt sichtbar, zählt aber nicht mehr als offene Lücke.
// Gleiche Startzeit wie gapInstance für den Schwere-Test.
const coveredAbsenceInstance = makeInstance({
  id: "20",
  startTime: "09:00",
  endTime: "10:00",
  title: "Mittagessen",
  staff: [
    {
      staffId: "2",
      isPrimary: true,
      isAbsent: true,
      isSubstitute: false,
      absenceReason: "Fortbildung",
    },
    { staffId: "3", isPrimary: false, isAbsent: false, isSubstitute: true },
  ],
});

// Bedingung 2: quittierte (bewusst unbesetzte) Lücke.
const acknowledgedInstance = makeInstance({
  id: "30",
  startTime: "11:00",
  endTime: "12:00",
  title: "Hausaufgabenzeit",
  staff: [],
});

// Bedingung 3: abgesagter Block.
const cancelledInstance = makeInstance({
  id: "40",
  startTime: "13:00",
  endTime: "13:30",
  title: "Fußball",
  status: "cancelled",
  cancelReason: "Platz gesperrt",
  staff: [
    { staffId: "1", isPrimary: true, isAbsent: false, isSubstitute: false },
  ],
});

// Ungestörte Position — erscheint nur im Ganztags-Modus bzw. -Fallback.
const undisturbedInstance = makeInstance({
  id: "50",
  startTime: "14:00",
  endTime: "15:00",
  title: "Lesen",
  staff: [
    { staffId: "1", isPrimary: true, isAbsent: false, isSubstitute: false },
  ],
});

const allInstances = [
  gapInstance,
  coveredAbsenceInstance,
  acknowledgedInstance,
  cancelledInstance,
  undisturbedInstance,
];

const gaps: GapInstance[] = [
  makeGap({
    instanceId: "10",
    title: "Frühstück",
    startTime: "09:00",
    endTime: "10:00",
    assignedStaffCount: 0,
    absentStaffCount: 1,
    presentStaffCount: 0,
    plannedStaffCount: 1,
  }),
];

const acknowledged: GapInstance[] = [
  makeGap({
    instanceId: "30",
    title: "Hausaufgabenzeit",
    startTime: "11:00",
    endTime: "12:00",
    assignedStaffCount: 0,
    absentStaffCount: 1,
    presentStaffCount: 0,
    plannedStaffCount: 1,
    understaffedNote: "Personalengpass",
  }),
];

const staffNames = new Map<string, string>([
  ["1", "Anna Krause"],
  ["2", "Bernd Neu"],
  ["3", "Clara Fischer"],
  ["4", "David Voss"],
]);

describe("computeStaffTriple", () => {
  it("stimmt für eine offene Lücke mit den GapInstance-Zahlen überein", () => {
    expect(computeStaffTriple(gapInstance)).toEqual({
      planned: 1,
      present: 0,
      absent: 1,
    });
    expect(gaps[0]).toMatchObject({
      plannedStaffCount: 1,
      presentStaffCount: 0,
      absentStaffCount: 1,
    });
  });

  it("zählt eine deckende Vertretung als anwesend, die abwesende Person bleibt in 'absent'", () => {
    expect(computeStaffTriple(coveredAbsenceInstance)).toEqual({
      planned: 1,
      present: 1,
      absent: 1,
    });
  });
});

describe("VertretungDayList", () => {
  it("zeigt bei 'Nur Störungen' genau die vier gestörten Positionen", () => {
    render(
      <VertretungDayList
        instances={allInstances}
        gaps={gaps}
        acknowledged={acknowledged}
        gapsAvailable
        staffNames={staffNames}
        mode="stoerungen"
        canManage={false}
        onEdit={vi.fn()}
      />,
    );

    expect(screen.getByText("Frühstück")).toBeInTheDocument();
    expect(screen.getByText("Mittagessen")).toBeInTheDocument();
    expect(screen.getByText("Hausaufgabenzeit")).toBeInTheDocument();
    expect(screen.getByText("Fußball")).toBeInTheDocument();
    expect(screen.queryByText("Lesen")).not.toBeInTheDocument();
  });

  it("sortiert nach Startzeit, bei Gleichstand nach Schwere", () => {
    render(
      <VertretungDayList
        instances={allInstances}
        gaps={gaps}
        acknowledged={acknowledged}
        gapsAvailable
        staffNames={staffNames}
        mode="stoerungen"
        canManage={false}
        onEdit={vi.fn()}
      />,
    );

    const rows = screen.getAllByTestId(/vertretung-day-list-row-/);
    const ids = rows.map((row) =>
      row.dataset.testid?.replace("vertretung-day-list-row-", ""),
    );
    expect(ids).toEqual(["10", "20", "30", "40"]);
  });

  it("fällt bei einem störungsfreien Tag automatisch auf die Ganztagsdarstellung zurück", () => {
    render(
      <VertretungDayList
        instances={[undisturbedInstance]}
        gaps={[]}
        acknowledged={[]}
        gapsAvailable
        staffNames={staffNames}
        mode="stoerungen"
        canManage={false}
        onEdit={vi.fn()}
      />,
    );

    expect(
      screen.getByText("Keine Störungen an diesem Tag"),
    ).toBeInTheDocument();
    expect(screen.getByText("Lesen")).toBeInTheDocument();
  });

  it("zeigt einen schlichten Empty-State ohne Instanzen", () => {
    render(
      <VertretungDayList
        instances={[]}
        gaps={[]}
        acknowledged={[]}
        gapsAvailable
        staffNames={staffNames}
        mode="stoerungen"
        canManage={false}
        onEdit={vi.fn()}
      />,
    );

    expect(screen.getByText("Keine Termine an diesem Tag")).toBeInTheDocument();
  });

  it("zeigt Ersatzkräfte blockweit statt sie abwesenden Personen zuzuordnen", () => {
    const multipleAbsences = makeInstance({
      id: "60",
      title: "Ausflug",
      staff: [
        { staffId: "1", isPrimary: true, isAbsent: true, isSubstitute: false },
        { staffId: "2", isPrimary: false, isAbsent: true, isSubstitute: false },
        { staffId: "3", isPrimary: false, isAbsent: false, isSubstitute: true },
      ],
    });
    render(
      <VertretungDayList
        instances={[gapInstance, multipleAbsences]}
        gaps={[]}
        acknowledged={[]}
        gapsAvailable
        staffNames={staffNames}
        mode="ganzer-tag"
        canManage={false}
        onEdit={vi.fn()}
      />,
    );

    const uncoveredRow = screen.getByTestId("vertretung-day-list-row-10");
    expect(
      within(uncoveredRow).getByText("Ersatzkräfte: keine"),
    ).toBeInTheDocument();

    const coveredRow = screen.getByTestId("vertretung-day-list-row-60");
    expect(
      within(coveredRow).getByText("Ersatzkräfte: Clara Fischer"),
    ).toBeInTheDocument();
    expect(within(coveredRow).queryByText(/Ersatz:/)).not.toBeInTheDocument();
  });

  it("entwarnt und fällt ohne verfügbare Lückendaten nicht auf den ganzen Tag zurück", () => {
    render(
      <VertretungDayList
        instances={[undisturbedInstance]}
        gaps={[]}
        acknowledged={[]}
        gapsAvailable={false}
        staffNames={staffNames}
        mode="stoerungen"
        canManage={false}
        onEdit={vi.fn()}
      />,
    );

    expect(
      screen.getByText("Störungslage konnte nicht vollständig geprüft werden"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Keine Störungen an diesem Tag"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Lesen")).not.toBeInTheDocument();
  });

  it("löst onEdit mit der Instanz-ID aus und zeigt ohne canManage 'Details' statt 'Bearbeiten'", () => {
    const onEdit = vi.fn();
    const { rerender } = render(
      <VertretungDayList
        instances={allInstances}
        gaps={gaps}
        acknowledged={acknowledged}
        gapsAvailable
        staffNames={staffNames}
        mode="stoerungen"
        canManage
        onEdit={onEdit}
      />,
    );

    const row = screen.getByTestId("vertretung-day-list-row-10");
    fireEvent.click(within(row).getByRole("button", { name: "Bearbeiten" }));
    expect(onEdit).toHaveBeenCalledWith("10");

    rerender(
      <VertretungDayList
        instances={allInstances}
        gaps={gaps}
        acknowledged={acknowledged}
        gapsAvailable
        staffNames={staffNames}
        mode="stoerungen"
        canManage={false}
        onEdit={onEdit}
      />,
    );

    // Lesenutzer behalten ein Klickziel (unterhalb lg gibt es keine
    // Kalenderspalte; der Verlauf muss ohne schedules:manage lesbar bleiben) —
    // aber nicht unter dem irreführenden Label "Bearbeiten".
    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();
    const readOnlyRow = screen.getByTestId("vertretung-day-list-row-10");
    fireEvent.click(
      within(readOnlyRow).getByRole("button", { name: "Details" }),
    );
    expect(onEdit).toHaveBeenLastCalledWith("10");
  });
});
