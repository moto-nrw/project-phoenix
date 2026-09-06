import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { VertretungWeekList } from "./vertretung-week-list";
import { parseISODate } from "~/lib/date-helpers";
import type { EnrichedInstance, GapInstance } from "~/lib/timetable-types";

// Woche Mo 13.07. – Fr 17.07.2026.
const MON = "2026-07-13";
const TUE = "2026-07-14";
const WED = "2026-07-15";
const THU = "2026-07-16";
const FRI = "2026-07-17";
const WEEK_DAYS = [MON, TUE, WED, THU, FRI].map(parseISODate);

function makeInstance(
  overrides: Partial<EnrichedInstance> & { id: string },
): EnrichedInstance {
  return {
    date: MON,
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
    date: MON,
    title: "Termin",
    startTime: "09:00",
    endTime: "10:00",
    roomId: "1",
    status: "planned",
    assignedStaffCount: 0,
    absentStaffCount: 1,
    presentStaffCount: 0,
    plannedStaffCount: 1,
    ...overrides,
  };
}

// Mo: offene Lücke. Mi: ungestörter Termin. Do: abgesagt.
const monGapInstance = makeInstance({
  id: "10",
  date: MON,
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
const wedUndisturbedInstance = makeInstance({
  id: "20",
  date: WED,
  title: "Lesen",
  staff: [
    { staffId: "1", isPrimary: true, isAbsent: false, isSubstitute: false },
  ],
});
const thuCancelledInstance = makeInstance({
  id: "30",
  date: THU,
  title: "Fußball",
  status: "cancelled",
  cancelReason: "Platz gesperrt",
  staff: [
    { staffId: "1", isPrimary: true, isAbsent: false, isSubstitute: false },
  ],
});

const INSTANCES = [
  monGapInstance,
  wedUndisturbedInstance,
  thuCancelledInstance,
];
const GAPS: GapInstance[] = [
  makeGap({ instanceId: "10", date: MON, title: "Frühstück" }),
];
const staffNames = new Map<string, string>([["1", "Anna Krause"]]);

function renderWeekList(
  overrides: Partial<React.ComponentProps<typeof VertretungWeekList>> = {},
) {
  const props = {
    weekDays: WEEK_DAYS,
    instances: INSTANCES,
    gaps: GAPS,
    acknowledged: [] as GapInstance[],
    gapsAvailableFrom: MON,
    staffNames,
    mode: "stoerungen" as const,
    canManage: true,
    onEdit: vi.fn(),
    onSelectDay: vi.fn(),
    todayISO: WED,
    ...overrides,
  };
  render(<VertretungWeekList {...props} />);
  return props;
}

function daySection(iso: string) {
  return screen.getByTestId(`vertretung-week-list-day-${iso}`);
}

describe("VertretungWeekList", () => {
  it("rendert genau die fünf übergebenen Wochentage mit Datumskopfzeile", () => {
    renderWeekList();

    for (const [iso, label] of [
      [MON, "Mo 13.07."],
      [TUE, "Di 14.07."],
      [WED, "Mi 15.07."],
      [THU, "Do 16.07."],
      [FRI, "Fr 17.07."],
    ] as const) {
      expect(within(daySection(iso)).getByText(label)).toBeInTheDocument();
    }
  });

  it("zeigt Störungen unter dem Tag, an dem sie auftreten", () => {
    renderWeekList();

    expect(
      within(daySection(MON)).getByTestId("vertretung-day-list-row-10"),
    ).toBeInTheDocument();
    expect(
      within(daySection(THU)).getByTestId("vertretung-day-list-row-30"),
    ).toBeInTheDocument();
    // Der ungestörte Mittwochs-Termin bleibt bei "Nur Störungen" aus.
    expect(
      screen.queryByTestId("vertretung-day-list-row-20"),
    ).not.toBeInTheDocument();
    expect(
      within(daySection(WED)).getByText("Keine Störungen an diesem Tag"),
    ).toBeInTheDocument();
    expect(
      within(daySection(TUE)).getByText("Keine Termine an diesem Tag"),
    ).toBeInTheDocument();
  });

  it("zeigt im Modus 'Ganze Woche' auch ungestörte Termine", () => {
    renderWeekList({ mode: "ganzer-tag" });

    expect(
      within(daySection(WED)).getByTestId("vertretung-day-list-row-20"),
    ).toBeInTheDocument();
  });

  it("zählt die Störungen pro Tag in der Kopfzeile", () => {
    renderWeekList();

    expect(within(daySection(MON)).getByText("1 Störung")).toBeInTheDocument();
    expect(
      within(daySection(WED)).getByText("0 Störungen"),
    ).toBeInTheDocument();
  });

  it("zeigt einen Strich statt einer erfundenen 0, solange für einen Tag keine Lückendaten vorliegen", () => {
    // Lückenfenster erst ab Mittwoch (auf heute geklemmt): Mo und Di haben
    // keine geprüfte Störungslage.
    renderWeekList({ gapsAvailableFrom: WED, instances: INSTANCES, gaps: [] });

    expect(within(daySection(MON)).getByText("–")).toBeInTheDocument();
    expect(within(daySection(TUE)).getByText("–")).toBeInTheDocument();
    expect(
      within(daySection(WED)).getByText("0 Störungen"),
    ).toBeInTheDocument();
    expect(
      within(daySection(MON)).getByText(
        "Störungslage konnte nicht vollständig geprüft werden",
      ),
    ).toBeInTheDocument();
  });

  it("zeigt für alle Tage einen Strich, wenn gar keine Lückendaten geladen sind", () => {
    renderWeekList({ gapsAvailableFrom: null, gaps: [] });

    expect(screen.getAllByText("–")).toHaveLength(5);
  });

  it("markiert heute in der Tageskopfzeile", () => {
    renderWeekList();

    expect(within(daySection(WED)).getByText("Heute")).toBeInTheDocument();
    expect(
      within(daySection(MON)).queryByText("Heute"),
    ).not.toBeInTheDocument();
  });

  it("löst onSelectDay mit dem ISO-Datum der geklickten Kopfzeile aus", () => {
    const props = renderWeekList();

    fireEvent.click(
      screen.getByRole("button", { name: "Do 16.07. als Tagesansicht öffnen" }),
    );
    expect(props.onSelectDay).toHaveBeenCalledWith(THU);
  });

  it("löst onEdit mit der Instanz-ID aus", () => {
    const props = renderWeekList();

    fireEvent.click(
      within(daySection(MON)).getByRole("button", { name: "Bearbeiten" }),
    );
    expect(props.onEdit).toHaveBeenCalledWith("10");
  });
});
