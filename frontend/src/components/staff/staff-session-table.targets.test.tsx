import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type {
  StaffAbsenceRow,
  StaffHistorySession,
  StaffSchedule,
} from "~/lib/staff-api";
import type { StaffShift } from "~/lib/shift-helpers";
import type { DayProjection } from "~/lib/time-tracking-helpers";

import { StaffSessionTable } from "./staff-session-table";

// Mo–Fr 8h nach dem HEUTE gültigen Plan. Genau dieser Plan darf bei einem
// fehlgeschlagenen Targets-Fetch NICHT auf vergangene Tage angewendet werden
// (#1842): wer seine Stunden geändert hat, bekäme sonst ein erfundenes
// historisches Soll, das der Monatskarte widerspricht.
const schedule: StaffSchedule = {
  mode: "custom",
  model: null,
  rotationLength: 1,
  rotationAnchorDate: "2026-01-05",
  entries: [0, 1, 2, 3, 4].map((dayOfWeek) => ({
    weekIndex: 0,
    dayOfWeek,
    targetMinutes: 480,
  })),
  weeklyTotals: [2400],
  validFrom: "2026-01-05",
};

// Eine Woche Mo–So, komplett in der Vergangenheit.
const from = new Date(2026, 0, 5);
const to = new Date(2026, 0, 11);
const today = new Date(2026, 5, 15);

const mondaySession: StaffHistorySession = {
  id: "41",
  date: "2026-01-05",
  net_minutes: 180,
  check_in_time: "2026-01-05T09:00:00Z",
  check_out_time: "2026-01-05T12:00:00Z",
  break_minutes: 0,
};

// Soll, Gutschrift und Saldo einer Zeile kommen seit #2443 als
// servergerechnete Tagesprojektion herein — die Tabelle leitet nichts mehr
// selbst ab. Der Helfer füllt alles, was ein Testfall nicht setzt, mit 0.
function dayProjection(
  entries: Record<string, Partial<DayProjection> & { targetMinutes: number }>,
): ReadonlyMap<string, DayProjection> {
  return new Map(
    Object.entries(entries).map(([date, values]) => [
      date,
      { creditMinutes: 0, actualMinutes: 0, balanceMinutes: 0, ...values },
    ]),
  );
}

function renderTable(props: {
  dailyProjection?: ReadonlyMap<string, DayProjection>;
  dailyProjectionError?: boolean;
  dailyProjectionPending?: boolean;
  accountStartDate?: string | null;
  accountStartDatePending?: boolean;
  accountStartDateError?: boolean;
  sessions?: readonly StaffHistorySession[];
  absences?: readonly StaffAbsenceRow[];
}) {
  return render(
    <StaffSessionTable
      staffId="1"
      from={from}
      to={to}
      sessions={props.sessions ?? []}
      absences={props.absences}
      schedule={schedule}
      dailyProjection={props.dailyProjection}
      dailyProjectionError={props.dailyProjectionError}
      dailyProjectionPending={props.dailyProjectionPending}
      accountStartDate={
        props.accountStartDate === undefined ? "" : props.accountStartDate
      }
      accountStartDatePending={props.accountStartDatePending ?? false}
      accountStartDateError={props.accountStartDateError ?? false}
      today={today}
      isAdminView
    />,
  );
}

describe("StaffSessionTable Soll-Auflösung", () => {
  it("nutzt die servergelieferten Targets, wenn vorhanden", () => {
    renderTable({
      dailyProjection: dayProjection({ "2026-01-05": { targetMinutes: 300 } }),
    });

    // 5h aus den Targets, nicht 8h aus dem aktuellen Plan.
    expect(screen.getAllByText("5h").length).toBeGreaterThan(0);
  });

  it("zeigt während des Ladens kein Soll aus dem aktuellen Plan", () => {
    // Die Fetches laufen mit keepPreviousData: nach einem Zeitraumwechsel hält
    // `dailyProjection` noch die Keys des VORHERIGEN Zeitraums, die neuen Tage
    // fehlen. Der Plan als Lückenfüller zeigte dort kurzzeitig ein falsches
    // historisches Soll, bis die Antwort eintraf (#1842).
    renderTable({ dailyProjectionPending: true });

    expect(screen.queryByText("8h")).not.toBeInTheDocument();
    expect(screen.getAllByText("…").length).toBe(5);
    // Laden ist kein Fehler — kein Warnhinweis.
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("zeigt während des Ladens keinen Saldo für Sessions mit ungelöstem Soll", () => {
    renderTable({
      sessions: [mondaySession],
      dailyProjectionPending: true,
    });

    const row = screen.getByText("05.01.").closest("tr");
    expect(row).not.toBeNull();
    const cells = within(row!).getAllByRole("cell");
    expect(cells[5]).toHaveTextContent("…");
    expect(cells[8]).toHaveTextContent("–");
    expect(screen.queryByText("+3h")).not.toBeInTheDocument();
  });

  it("behält beim Zeitraumwechsel die bereits aufgelösten Tage", () => {
    // Ein überlappender Tag aus dem vorherigen Fetch bleibt gültig: dasselbe
    // Datum liefert für dieselbe Person immer dasselbe Soll. Nur die noch
    // nicht abgedeckten Tage bleiben ungelöst.
    renderTable({
      dailyProjection: dayProjection({ "2026-01-05": { targetMinutes: 300 } }),
      dailyProjectionPending: true,
    });

    expect(screen.getAllByText("5h").length).toBeGreaterThan(0);
    expect(screen.getAllByText("…").length).toBe(4);
    expect(screen.queryByText("8h")).not.toBeInTheDocument();
  });

  it("nutzt ohne Lade- und Fehlersignal den Plan als Fallback", () => {
    // Der unsignalisierte Default (kein Caller im Produktivcode): ohne jede
    // Angabe bleibt der Plan die einzige Quelle.
    renderTable({});

    expect(screen.getAllByText("8h").length).toBe(5);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("zeigt bei fehlgeschlagenem Targets-Fetch kein Soll aus dem aktuellen Plan", () => {
    renderTable({ dailyProjectionError: true });

    // Der aktuelle Plan darf nicht als historisches Soll auftauchen.
    expect(screen.queryByText("8h")).not.toBeInTheDocument();
    // Die Tabelle zeigt nur Mo–Fr.
    expect(screen.getAllByText("?").length).toBe(5);
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("zeigt nach einem Targets-Fehler keinen Saldo für Sessions mit ungelöstem Soll", () => {
    renderTable({
      sessions: [mondaySession],
      dailyProjectionError: true,
    });

    const row = screen.getByText("05.01.").closest("tr");
    expect(row).not.toBeNull();
    const cells = within(row!).getAllByRole("cell");
    expect(cells[5]).toHaveTextContent("?");
    expect(cells[8]).toHaveTextContent("–");
    expect(screen.queryByText("+3h")).not.toBeInTheDocument();
  });

  it("bevorzugt geladene Targets auch dann, wenn ein Fehler gemeldet ist", () => {
    renderTable({
      dailyProjection: dayProjection({ "2026-01-05": { targetMinutes: 300 } }),
      dailyProjectionError: true,
    });

    expect(screen.getAllByText("5h").length).toBeGreaterThan(0);
    // Nur die Tage ohne aufgelöstes Target bleiben ungelöst.
    expect(screen.getAllByText("?").length).toBe(4);
  });

  it("zählt Sessions vor einem untermonatigen Kontostart nicht als Plus", () => {
    renderTable({
      sessions: [mondaySession],
      // Der Server liefert für einen Tag vor dem Kontostart durchgehend 0.
      dailyProjection: dayProjection({ "2026-01-05": { targetMinutes: 0 } }),
      accountStartDate: "2026-01-08",
    });

    const row = screen.getByText("05.01.").closest("tr");
    expect(row).not.toBeNull();
    const cells = within(row!).getAllByRole("cell");
    expect(cells[5]).toHaveTextContent("–");
    expect(cells[8]).toHaveTextContent("–");
    expect(screen.queryByText("+3h")).not.toBeInTheDocument();
  });

  it("lässt einen ungelösten Kontostart bei Soll 0 nicht als Plus aufblitzen", () => {
    renderTable({
      sessions: [mondaySession],
      dailyProjection: dayProjection({
        "2026-01-05": {
          targetMinutes: 0,
          actualMinutes: 180,
          balanceMinutes: 180,
        },
      }),
      accountStartDate: null,
      accountStartDatePending: true,
    });

    const row = screen.getByText("05.01.").closest("tr");
    expect(row).not.toBeNull();
    const cells = within(row!).getAllByRole("cell");
    expect(cells[8]).toHaveTextContent("–");
    expect(screen.queryByText("+3h")).not.toBeInTheDocument();
  });

  it("zeigt Soll-0-Salden mit Warnung, wenn der Kontostart nicht geladen werden konnte", () => {
    renderTable({
      sessions: [mondaySession],
      dailyProjection: dayProjection({
        "2026-01-05": {
          targetMinutes: 0,
          actualMinutes: 180,
          balanceMinutes: 180,
        },
      }),
      accountStartDate: null,
      accountStartDateError: true,
    });

    expect(screen.getByText("+3h")).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Der Stundenkonto-Start konnte nicht geladen werden.",
    );
  });
});

describe("StaffSessionTable Abwesenheitsstatus", () => {
  it("zeigt comp_time ausdrücklich als Freizeitausgleich", () => {
    renderTable({
      dailyProjection: dayProjection({ "2026-01-05": { targetMinutes: 480 } }),
      absences: [
        {
          id: 73,
          staff_id: 1,
          absence_type: "comp_time",
          date_start: "2026-01-05",
          date_end: "2026-01-05",
          half_day: false,
          status: "approved",
          note: "",
        },
      ],
    });

    expect(screen.getByText("Freizeitausgleich")).toBeInTheDocument();
  });

  it("kennzeichnet administrativ angelegten halben Freizeitausgleich", () => {
    renderTable({
      dailyProjection: dayProjection({ "2026-01-05": { targetMinutes: 480 } }),
      absences: [
        {
          id: 74,
          staff_id: 1,
          absence_type: "comp_time",
          date_start: "2026-01-05",
          date_end: "2026-01-05",
          half_day: true,
          start_half_day: false,
          end_half_day: false,
          status: "reported",
          note: "",
        },
      ],
    });

    expect(
      screen.getByText("Freizeitausgleich · Halber Tag"),
    ).toBeInTheDocument();
  });
});

// #1967: Sa/So waren hart aus der Tagesansicht gefiltert, während Monatskarte
// und KPI-Karten sie serverseitig mitzählten. Die Summe der sichtbaren Zeilen
// ergab dann nicht mehr das ausgewiesene Ist.
describe("StaffSessionTable Wochenendtage", () => {
  // Sa, 10.01.2026 — im Zeitraum from/to und in der Vergangenheit.
  const saturday = "2026-01-10";
  const emptySessions: readonly StaffHistorySession[] = [];
  const weekendProjection = dayProjection({
    "2026-01-05": { targetMinutes: 480 },
    "2026-01-06": { targetMinutes: 480 },
    "2026-01-07": { targetMinutes: 480 },
    "2026-01-08": { targetMinutes: 480 },
    "2026-01-09": { targetMinutes: 480 },
    // Der Server liefert Wochenenden mit Soll 0 mit; am Samstag zählt er die
    // 3 Stunden der Wochenend-Session voll als Plus.
    "2026-01-10": { targetMinutes: 0, actualMinutes: 180, balanceMinutes: 180 },
    "2026-01-11": { targetMinutes: 0 },
  });

  const weekendSession: StaffHistorySession = {
    id: "42",
    date: saturday,
    net_minutes: 180,
    check_in_time: `${saturday}T09:00:00Z`,
    check_out_time: `${saturday}T12:00:00Z`,
    break_minutes: 0,
  };

  type WeekendTableProps = {
    sessions?: readonly StaffHistorySession[];
    plannedShifts?: readonly StaffShift[];
    absences?: readonly StaffAbsenceRow[];
    holidays?: ReadonlyMap<string, string>;
    closingDays?: ReadonlyMap<string, string>;
  };

  function WeekendTable(props: WeekendTableProps) {
    return (
      <StaffSessionTable
        staffId="1"
        from={from}
        to={to}
        sessions={props.sessions ?? emptySessions}
        absences={props.absences}
        plannedShifts={props.plannedShifts}
        schedule={schedule}
        accountStartDate="2026-01-08"
        dailyProjection={weekendProjection}
        holidays={props.holidays}
        closingDays={props.closingDays}
        today={today}
        isAdminView
        accountStartDatePending={false}
        accountStartDateError={false}
      />
    );
  }

  function renderWeekend(props: WeekendTableProps) {
    return render(<WeekendTable {...props} />);
  }

  it("blendet leere Wochenenden weiter aus", () => {
    renderWeekend({});

    // 10.01. = Sa, 11.01. = So — beide Datumszellen fehlen.
    expect(screen.queryByText("10.01.")).not.toBeInTheDocument();
    expect(screen.queryByText("11.01.")).not.toBeInTheDocument();
  });

  it("zeigt einen Feiertag am Sonntag, sobald die Feiertage geladen sind", () => {
    const view = renderWeekend({});
    expect(screen.queryByText("11.01.")).not.toBeInTheDocument();

    view.rerender(
      <WeekendTable holidays={new Map([["2026-01-11", "Ostersonntag"]])} />,
    );

    const row = screen.getByText("11.01.").closest("tr");
    expect(row).not.toBeNull();
    expect(within(row!).getByText("Feiertag")).toHaveAttribute(
      "title",
      "Ostersonntag",
    );
    expect(screen.queryByText("10.01.")).not.toBeInTheDocument();
  });

  // #1418 3b: Schließtage verhalten sich wie Feiertage (Soll 0, eigenes
  // Badge), stehen aber bei Überschneidung hinter dem Feiertag zurück.
  it("zeigt einen Schließtag am Wochenende mit Badge und Grund", () => {
    renderWeekend({
      closingDays: new Map([["2026-01-10", "Pädagogischer Tag"]]),
    });

    const row = screen.getByText("10.01.").closest("tr");
    expect(row).not.toBeNull();
    expect(within(row!).getByText("Schließtag")).toHaveAttribute(
      "title",
      "Pädagogischer Tag",
    );
    expect(screen.queryByText("11.01.")).not.toBeInTheDocument();
  });

  it("lässt den Feiertag gewinnen, wenn Feiertag und Schließtag zusammenfallen", () => {
    renderWeekend({
      holidays: new Map([["2026-01-11", "Ostersonntag"]]),
      closingDays: new Map([["2026-01-11", "Ferienschließung"]]),
    });

    const row = screen.getByText("11.01.").closest("tr");
    expect(row).not.toBeNull();
    expect(within(row!).getByText("Feiertag")).toBeInTheDocument();
    expect(within(row!).queryByText("Schließtag")).not.toBeInTheDocument();
  });

  it("zeigt einen Samstag mit Session als Zeile", () => {
    renderWeekend({ sessions: [weekendSession] });

    expect(screen.getByText("10.01.")).toBeInTheDocument();
    // Der Sonntag bleibt leer und damit unsichtbar.
    expect(screen.queryByText("11.01.")).not.toBeInTheDocument();
  });

  it("zählt das Ist eines Wochenendtags voll als Plus", () => {
    // Soll ist 0, die Monatskarte rechnet actual - targetToDate. Ein "–" im
    // Saldo machte die Monatskarte aus der Tabelle unerklärbar.
    renderWeekend({ sessions: [weekendSession] });

    expect(screen.getByText("+3h")).toBeInTheDocument();
  });

  it("zeigt für eine zukünftige Session keinen Saldo", () => {
    render(
      <StaffSessionTable
        staffId="1"
        from={new Date(2026, 0, 10)}
        to={new Date(2026, 0, 10)}
        sessions={[weekendSession]}
        schedule={schedule}
        dailyProjection={dayProjection({ [saturday]: { targetMinutes: 0 } })}
        accountStartDate=""
        accountStartDatePending={false}
        accountStartDateError={false}
        today={new Date(2026, 0, 9)}
        isAdminView
      />,
    );

    const row = screen.getByText("10.01.").closest("tr");
    expect(row).not.toBeNull();
    const cells = within(row!).getAllByRole("cell");
    expect(cells[8]).toHaveTextContent("–");
    expect(screen.queryByText("+3h")).not.toBeInTheDocument();
  });

  it("zeigt einen Wochenendtag mit geplanter Schicht", () => {
    renderWeekend({
      plannedShifts: [
        {
          id: "1",
          staffId: "1",
          date: saturday,
          startTime: "09:00",
          endTime: "12:00",
          breakMinutes: 0,
          shiftTypeId: null,
          shiftTypeName: null,
          shiftTypeColor: null,
          notes: "",
          seriesId: null,
          detached: false,
          cancelled: false,
          changeReason: null,
          originShiftId: null,
        },
      ],
    });

    expect(screen.getByText("10.01.")).toBeInTheDocument();
  });

  // Ein ungelöstes Soll ist kein Beleg für einen leeren Tag: ein vertraglich
  // verplanter Samstag darf beim Laden oder nach einem Fehler nicht aus der
  // Tabelle fallen, sonst ist er weder als ungelöst erkennbar noch
  // korrigierbar.
  const weekendSchedule: StaffSchedule = {
    ...schedule,
    entries: [
      ...schedule.entries,
      { weekIndex: 0, dayOfWeek: 5, targetMinutes: 240 },
    ],
    weeklyTotals: [2640],
  };

  function renderScheduledWeekend(props: {
    dailyProjectionPending?: boolean;
    dailyProjectionError?: boolean;
  }) {
    return render(
      <StaffSessionTable
        staffId="1"
        from={from}
        to={to}
        sessions={[]}
        schedule={weekendSchedule}
        dailyProjectionPending={props.dailyProjectionPending}
        dailyProjectionError={props.dailyProjectionError}
        accountStartDate=""
        accountStartDatePending={false}
        accountStartDateError={false}
        today={today}
        isAdminView
      />,
    );
  }

  it("behält einen verplanten Samstag, während die Targets laden", () => {
    renderScheduledWeekend({ dailyProjectionPending: true });

    const row = screen.getByText("10.01.").closest("tr");
    expect(row).not.toBeNull();
    // Sichtbar, aber ohne erfundenes Soll aus dem aktuellen Plan.
    expect(within(row!).getAllByRole("cell")[5]).toHaveTextContent("…");
    expect(screen.queryByText("11.01.")).not.toBeInTheDocument();
  });

  it("behält einen verplanten Samstag nach einem Targets-Fehler", () => {
    renderScheduledWeekend({ dailyProjectionError: true });

    const row = screen.getByText("10.01.").closest("tr");
    expect(row).not.toBeNull();
    expect(within(row!).getAllByRole("cell")[5]).toHaveTextContent("?");
  });
});

// #2443: Der Tages-Saldo hing an einer vorhandenen WorkSession und wurde als
// „Ist minus Soll" abgeleitet. Ein Abwesenheitstag hatte damit gar keinen
// Saldo — ein Freizeitausgleich sah kostenlos aus, obwohl die Monatskarte
// darüber längst das volle Tagessoll abgezogen hatte. Die Zeile zeigt jetzt
// die servergerechneten Werte, auch an Tagen ohne Session.
describe("StaffSessionTable Tages-Saldo (#2443)", () => {
  const monday = "2026-01-05";

  function absenceOn(
    absenceType: string,
    overrides?: Partial<StaffAbsenceRow>,
  ): StaffAbsenceRow {
    return {
      id: 1,
      staff_id: 1,
      absence_type: absenceType,
      date_start: monday,
      date_end: monday,
      half_day: false,
      status: "approved",
      note: "",
      ...overrides,
    } as StaffAbsenceRow;
  }

  function mondayCells(
    projection: Partial<DayProjection> & { targetMinutes: number },
    props?: { absences?: readonly StaffAbsenceRow[] },
  ) {
    renderTable({
      dailyProjection: dayProjection({ [monday]: projection }),
      absences: props?.absences,
    });
    const row = screen.getByText("05.01.").closest("tr");
    expect(row).not.toBeNull();
    return within(row!).getAllByRole("cell");
  }

  it("zeigt den vollen Abzug eines Freizeitausgleichs ohne Arbeitszeit", () => {
    const cells = mondayCells(
      { targetMinutes: 480, balanceMinutes: -480 },
      { absences: [absenceOn("comp_time")] },
    );

    expect(cells[5]).toHaveTextContent("8h");
    expect(cells[6]).toHaveTextContent("–");
    expect(cells[7]).toHaveTextContent("–");
    expect(cells[8]).toHaveTextContent("−8h");
  });

  it("weist die Gutschrift eines Urlaubstags aus und gleicht den Saldo aus", () => {
    const cells = mondayCells(
      { targetMinutes: 480, creditMinutes: 480, balanceMinutes: 0 },
      { absences: [absenceOn("vacation")] },
    );

    expect(cells[7]).toHaveTextContent("8h");
    expect(cells[8]).toHaveTextContent("0min");
  });

  it("rechnet einen halben Urlaubstag zur Hälfte an", () => {
    const cells = mondayCells(
      { targetMinutes: 480, creditMinutes: 240, balanceMinutes: -240 },
      { absences: [absenceOn("vacation", { half_day: true })] },
    );

    expect(cells[7]).toHaveTextContent("4h");
    expect(cells[8]).toHaveTextContent("−4h");
  });

  it("zeigt auch ohne Abwesenheit und ohne Erfassung das offene Tagessoll", () => {
    const cells = mondayCells({ targetMinutes: 480, balanceMinutes: -480 });

    expect(within(cells[9]!).getByText("Nicht erfasst")).toBeInTheDocument();
    expect(cells[8]).toHaveTextContent("−8h");
  });

  it("behauptet an einem freien Tag ohne alles keinen Saldo", () => {
    const cells = mondayCells({ targetMinutes: 0 });

    expect(cells[5]).toHaveTextContent("–");
    expect(cells[7]).toHaveTextContent("–");
    expect(cells[8]).toHaveTextContent("–");
  });

  it("nennt in der Gutschrift-Spalte die Abwesenheit, aus der sie stammt", () => {
    const cells = mondayCells(
      { targetMinutes: 480, creditMinutes: 480, balanceMinutes: 0 },
      { absences: [absenceOn("sick")] },
    );

    expect(within(cells[7]!).getByTitle(/Krank/)).toBeInTheDocument();
  });
});
