import { render, screen, within, fireEvent } from "@testing-library/react";
import { describe, expect, it, beforeEach, vi } from "vitest";
import type {
  StaffAbsenceRow,
  StaffHistorySession,
  StaffSchedule,
} from "~/lib/staff-api";
import type { StaffShift } from "~/lib/shift-helpers";
import type { DayProjection } from "~/lib/time-tracking-helpers";
import {
  StaffSessionTable,
  isStaleAfterSessionSave,
} from "./staff-session-table";
import { ApiError } from "~/lib/api-error";
import { setTestClock } from "~/test/clock";
import { catalogText } from "~/test/error-catalog-text";

describe("staff-session-table.targets", () => {
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
    dailyProjectionError?: unknown;
    dailyProjectionPending?: boolean;
    accountStartDate?: string | null;
    accountStartDatePending?: boolean;
    accountStartDateError?: unknown;
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
        dailyProjection: dayProjection({
          "2026-01-05": { targetMinutes: 300 },
        }),
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
        dailyProjection: dayProjection({
          "2026-01-05": { targetMinutes: 300 },
        }),
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

    it("zeigt bei fehlgeschlagenem Targets-Fetch kein Soll aus dem aktuellen Plan", async () => {
      const retry = vi.fn();
      render(
        <StaffSessionTable
          staffId="1"
          from={from}
          to={to}
          sessions={[]}
          schedule={schedule}
          dailyProjectionError={
            new ApiError("boom", 500, {
              code: "general.server",
              instance: "req-targets",
            })
          }
          onRetryDailyProjection={retry}
          accountStartDate=""
          accountStartDatePending={false}
          accountStartDateError={false}
          today={today}
          isAdminView
        />,
      );

      // Der aktuelle Plan darf nicht als historisches Soll auftauchen.
      expect(screen.queryByText("8h")).not.toBeInTheDocument();
      // Die Tabelle zeigt nur Mo–Fr.
      expect(screen.getAllByText("?").length).toBe(5);
      // Der Ladefehler steht vor Ort, mit Wiederholen und Vorgangskennung.
      expect(
        await screen.findByText(
          catalogText("general.server", "das Soll für diesen Zeitraum"),
        ),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Vorgangskennung kopieren" }),
      ).toHaveTextContent("req-targets");
      fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
      expect(retry).toHaveBeenCalled();
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
        dailyProjection: dayProjection({
          "2026-01-05": { targetMinutes: 300 },
        }),
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

    it("zeigt Soll-0-Salden mit Warnung, wenn der Kontostart nicht geladen werden konnte", async () => {
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
        accountStartDateError: new ApiError("down", 503, {
          code: "general.unavailable",
        }),
      });

      expect(screen.getByText("+3h")).toBeInTheDocument();
      expect(
        await screen.findByText(
          catalogText(
            "general.unavailable",
            "die Einstellung zum Stundenkonto",
          ),
        ),
      ).toBeInTheDocument();
    });
  });

  describe("StaffSessionTable Abwesenheitsstatus", () => {
    it("zeigt comp_time ausdrücklich als Freizeitausgleich", () => {
      renderTable({
        dailyProjection: dayProjection({
          "2026-01-05": { targetMinutes: 480 },
        }),
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
        dailyProjection: dayProjection({
          "2026-01-05": { targetMinutes: 480 },
        }),
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
      "2026-01-10": {
        targetMinutes: 0,
        actualMinutes: 180,
        balanceMinutes: 180,
      },
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

    // Bei Überschneidungen verbraucht der Server den Tag mit der NIEDRIGSTEN
    // Abwesenheits-ID. Die API liefert nach Startdatum sortiert, der Tooltip
    // nannte deshalb die früher beginnende Abwesenheit — also die falsche Art,
    // sobald sie die höhere ID trägt.
    it("nennt bei überlappenden Abwesenheiten die vom Server angerechnete", () => {
      const cells = mondayCells(
        { targetMinutes: 480, creditMinutes: 480, balanceMinutes: 0 },
        {
          absences: [
            absenceOn("vacation", {
              id: 7,
              date_start: "2026-01-02",
              date_end: monday,
            }),
            absenceOn("sick", { id: 3 }),
          ],
        },
      );

      expect(within(cells[7]!).getByTitle(/Krank/)).toBeInTheDocument();
      expect(within(cells[7]!).queryByTitle(/Urlaub/)).not.toBeInTheDocument();
    });

    // Nicht wirksame Abwesenheiten (requested/declined) schreiben nichts gut und
    // dürfen die Gutschrift einer wirksamen Abwesenheit nicht überschreiben.
    it("übergeht nicht wirksame Abwesenheiten in der Gutschrift-Herkunft", () => {
      const cells = mondayCells(
        { targetMinutes: 480, creditMinutes: 480, balanceMinutes: 0 },
        {
          absences: [
            absenceOn("vacation", { id: 1, status: "requested" }),
            absenceOn("sick", { id: 4 }),
          ],
        },
      );

      expect(within(cells[7]!).getByTitle(/Krank/)).toBeInTheDocument();
    });
  });
});

describe("staff-session-table.blocks", () => {
  // Mehrere Arbeitsblöcke pro Tag (#2402): Homeoffice-Vormittag, OGS-Nachmittag.
  // Die Tageszeile aggregiert, die Block-Zeilen tragen die Details.

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

  // Nur der Montag, komplett in der Vergangenheit.
  const from = new Date(2026, 0, 5);
  const to = new Date(2026, 0, 5);
  const today = new Date(2026, 5, 15);

  const morningHomeOffice: StaffHistorySession = {
    id: "41",
    date: "2026-01-05",
    status: "home_office",
    source: "app",
    net_minutes: 240,
    check_in_time: "2026-01-05T08:00:00+01:00",
    check_out_time: "2026-01-05T12:00:00+01:00",
    break_minutes: 0,
  };

  const afternoonOgs: StaffHistorySession = {
    id: "42",
    date: "2026-01-05",
    status: "present",
    source: "nfc",
    net_minutes: 130,
    check_in_time: "2026-01-05T13:30:00+01:00",
    check_out_time: "2026-01-05T16:00:00+01:00",
    break_minutes: 20,
  };

  interface TableProps {
    sessions?: readonly StaffHistorySession[];
    from?: Date;
    to?: Date;
    today?: Date;
    onEditDay?: (
      date: Date,
      session: StaffHistorySession | null,
      absence: unknown,
    ) => void;
  }

  // Die Tabelle rechnet Gutschrift und Saldo nicht mehr selbst, sie zeigt die
  // servergerechnete Tagesprojektion (#2443). Der Helfer baut sie mit Nullen für
  // alles, was ein Testfall nicht ausdrücklich setzt.
  function dayProjection(
    entries: Record<string, Partial<DayProjection> & { targetMinutes: number }>,
  ): ReadonlyMap<string, DayProjection> {
    return new Map(
      Object.entries(entries).map(([date, values]) => [
        date,
        {
          creditMinutes: 0,
          actualMinutes: 0,
          balanceMinutes: 0,
          ...values,
        },
      ]),
    );
  }

  function tableElement(props?: TableProps) {
    return (
      <StaffSessionTable
        staffId="1"
        from={props?.from ?? from}
        to={props?.to ?? to}
        // Absichtlich verdreht übergeben — die Tabelle sortiert nach Check-in.
        sessions={props?.sessions ?? [afternoonOgs, morningHomeOffice]}
        schedule={schedule}
        dailyProjection={dayProjection({
          "2026-01-05": { targetMinutes: 480 },
          "2026-01-06": { targetMinutes: 480 },
        })}
        accountStartDate=""
        accountStartDatePending={false}
        accountStartDateError={false}
        today={props?.today ?? today}
        isAdminView
        onEditDay={props?.onEditDay}
      />
    );
  }

  function renderTable(props?: TableProps) {
    return render(tableElement(props));
  }

  describe("StaffSessionTable Arbeitsblöcke (#2402)", () => {
    it("aggregiert die Tageszeile über alle Blöcke", () => {
      renderTable();

      // Check-in = erster Block (Tageszeile + Block-1-Zeile), Check-out =
      // letzter Block.
      expect(screen.getAllByText("08:00").length).toBeGreaterThanOrEqual(2);
      expect(screen.getAllByText("16:00").length).toBeGreaterThan(0);
      // Ist = 240 + 130 = 370min = 6h 10min, Pause = 20min (nur Block 2).
      expect(screen.getAllByText("6h 10min").length).toBeGreaterThan(0);
      // Statuszelle der Tageszeile trägt den Block-Zähler.
      expect(screen.getByText("2 Blöcke")).toBeInTheDocument();
    });

    // Eine Blockzeile mit zu wenig Zellen rutscht als Ganzes eine Spalte nach
    // links: das Ist des Blocks landet unter „Gutschrift", sein Arbeitsort unter
    // „Saldo". Genau das passierte, als die Gutschrift-Spalte dazukam (#2443).
    it("hält die Block-Zeilen spaltengleich zur Tageszeile", () => {
      renderTable();

      const headerCount = screen.getAllByRole("columnheader").length;
      const dayRow = screen.getByText("05.01.").closest("tr");
      const blockRow = screen.getByText("Block 1").closest("tr");
      expect(dayRow).not.toBeNull();
      expect(blockRow).not.toBeNull();

      expect(within(dayRow!).getAllByRole("cell")).toHaveLength(headerCount);
      expect(within(blockRow!).getAllByRole("cell")).toHaveLength(headerCount);
    });

    it("paart Status und Quelle nur in den Block-Zeilen, nie auf der Tageszeile", () => {
      renderTable();

      // Die Tageszeile stapelt KEINE Status-/Quelle-Badges nebeneinander —
      // das las sich als "OGS · App", obwohl der OGS-Block per NFC kam.
      // Jeder Arbeitsort und jede Quelle erscheint genau einmal, nämlich in
      // der Block-Zeile, zu der sie gehören.
      expect(screen.getAllByText("OGS")).toHaveLength(1);
      expect(screen.getAllByText("Homeoffice")).toHaveLength(1);
      expect(screen.getAllByText("NFC")).toHaveLength(1);
      expect(screen.getAllByText("App")).toHaveLength(1);

      // Homeoffice-Block kam per App, OGS-Block per NFC: die Badges sitzen in
      // derselben Zeile wie ihr Block.
      const homeOfficeRow = screen.getByText("Homeoffice").closest("tr");
      const ogsRow = screen.getByText("OGS").closest("tr");
      expect(homeOfficeRow).toContainElement(screen.getByText("App"));
      expect(ogsRow).toContainElement(screen.getByText("NFC"));
    });

    it("zeigt die Quelle auf der Tageszeile nur bei einheitlichem Kanal", () => {
      renderTable({
        sessions: [morningHomeOffice, { ...afternoonOgs, source: "app" }],
      });

      // Beide Blöcke per App → die Tageszeile darf das Badge zeigen
      // (Tageszeile + zwei Block-Zeilen = 3).
      expect(screen.getAllByText("App")).toHaveLength(3);
    });

    it("kennzeichnet die Zeiten der Tageszeile als Tagesgrenzen", () => {
      renderTable();

      // 08:00–16:00 auf der Tageszeile ist KEIN durchgehender Zeitraum: die
      // 90 Minuten zwischen den Blöcken sind keine Arbeitszeit. "ab"/"bis"
      // plus Tooltip sagen das an der Zelle, damit die Grenzen nicht als
      // gearbeitete Spanne gelesen werden.
      const from = screen.getByText("ab");
      const until = screen.getByText("bis");
      expect(from.closest("span[title]")).toHaveAttribute(
        "title",
        expect.stringContaining("nicht als Arbeitszeit"),
      );
      expect(until.closest("span[title]")).toHaveAttribute(
        "title",
        expect.stringContaining("nicht als Arbeitszeit"),
      );
    });

    it("belässt Ein-Block-Tage bei der schlichten Zeitangabe", () => {
      renderTable({ sessions: [morningHomeOffice] });

      expect(screen.queryByText("ab")).not.toBeInTheDocument();
      expect(screen.queryByText("bis")).not.toBeInTheDocument();
      expect(screen.getByText("08:00")).toBeInTheDocument();
      expect(screen.getByText("12:00")).toBeInTheDocument();
    });

    it("listet jeden Block mit eigenen Zeiten als Unterzeile", () => {
      renderTable();

      expect(screen.getByText("Block 1")).toBeInTheDocument();
      expect(screen.getByText("Block 2")).toBeInTheDocument();
      // Blockzeiten in Check-in-Reihenfolge, trotz verdrehter Eingabe.
      expect(screen.getByText("12:00")).toBeInTheDocument();
      expect(screen.getByText("13:30")).toBeInTheDocument();
    });

    it("öffnet die Bearbeitung für genau den angeklickten Block", () => {
      const onEditDay = vi.fn();
      renderTable({ onEditDay });

      // Zeilenaktionen liegen im Kebab der Zeile (Bauart 1 Regel 4).
      fireEvent.click(
        screen.getByRole("button", { name: "Aktionen für Block 2" }),
      );
      fireEvent.click(
        screen.getByRole("menuitem", { name: "Block 2 bearbeiten" }),
      );

      expect(onEditDay).toHaveBeenCalledTimes(1);
      const [, session] = onEditDay.mock.calls[0] as [
        Date,
        StaffHistorySession | null,
        unknown,
      ];
      expect(session?.id).toBe("42");
    });

    it("bearbeitet bei einem Nachtblock immer den vollständigen Originalblock", () => {
      const onEditDay = vi.fn();
      const nightBlock: StaffHistorySession = {
        ...morningHomeOffice,
        id: "night",
        date: "2026-01-04",
        check_in_time: "2026-01-04T22:00:00+01:00",
        check_out_time: "2026-01-05T02:00:00+01:00",
        net_minutes: 240,
      };
      renderTable({ sessions: [nightBlock], onEditDay });

      fireEvent.click(screen.getByRole("button", { name: /^Aktionen für \d/ }));
      fireEvent.click(
        screen.getByRole("menuitem", { name: "Eintrag bearbeiten" }),
      );

      const [date, session] = onEditDay.mock.calls[0] as [
        Date,
        StaffHistorySession | null,
        unknown,
      ];
      expect(date).toEqual(new Date(2026, 0, 4));
      expect(session).toMatchObject({
        id: "night",
        date: "2026-01-04",
        check_in_time: "2026-01-04T22:00:00+01:00",
        check_out_time: "2026-01-05T02:00:00+01:00",
      });
    });

    it("bietet auf einem Mehrblock-Tag das Nachtragen eines weiteren Blocks an", () => {
      const onEditDay = vi.fn();
      renderTable({ onEditDay });

      // Das Menü der Tageszeile (nicht der Blockzeilen) bietet das Nachtragen an.
      fireEvent.click(screen.getByRole("button", { name: /^Aktionen für \d/ }));
      fireEvent.click(
        screen.getByRole("menuitem", { name: "Block nachtragen" }),
      );

      expect(onEditDay).toHaveBeenCalledTimes(1);
      const [, session] = onEditDay.mock.calls[0] as [
        Date,
        StaffHistorySession | null,
        unknown,
      ];
      expect(session).toBeNull();
    });

    it("Ein-Block-Tage rendern unverändert ohne Unterzeilen", () => {
      renderTable({ sessions: [morningHomeOffice] });

      expect(screen.queryByText("Block 1")).not.toBeInTheDocument();
      expect(screen.queryByText("2 Blöcke")).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: /^Aktionen für \d/ }));
      expect(
        screen.getByRole("menuitem", { name: "Eintrag bearbeiten" }),
      ).toBeInTheDocument();
    });

    it("markiert den Tag als eingestempelt, wenn ein Block offen ist", () => {
      renderTable({
        sessions: [
          morningHomeOffice,
          { ...afternoonOgs, check_out_time: null },
        ],
      });

      expect(screen.getAllByText("eingestempelt").length).toBeGreaterThan(0);
    });

    // Die Tagessegmente eines Nachtblocks enden dort, wo auch die gezeigten
    // Minuten enden. Vergleicht die Aufteilung stattdessen mit einem rohen
    // Checkout, findet kein Segment seinen letzten Tag und die letzte Zeile
    // erfindet 23:59.
    describe("gekappte Blöcke", () => {
      beforeEach(() => {
        // 06.01.2026, 09:00 Berlin.
        setTestClock(new Date("2026-01-06T08:00:00Z"));
      });

      it("zeigt beim Checkout in der Zukunft das gekappte Ende statt 23:59", () => {
        renderTable({
          from: new Date(2026, 0, 5),
          to: new Date(2026, 0, 6),
          sessions: [
            {
              ...morningHomeOffice,
              id: "capped",
              date: "2026-01-05",
              check_in_time: "2026-01-05T22:00:00+01:00",
              // Fehleingabe: der Checkout liegt Tage in der Zukunft.
              check_out_time: "2026-01-09T12:00:00+01:00",
              net_minutes: 660,
            },
          ],
        });

        // Zweiter Tag des Blocks: 00:00 bis zur Kappung um 09:00.
        const secondDay = screen.getByText("00:00").closest("tr");
        expect(secondDay).toHaveTextContent("09:00");
        expect(secondDay).not.toHaveTextContent("23:59");
      });

      // Ein Nachtblock steht an zwei Tagen, korrigiert wurde er aber einmal.
      // Trügen beide Segmente die Historie, klappten beide Tageszeilen dieselbe
      // Liste auf und eine Korrektur sähe aus wie zwei.
      it("führt die Änderungshistorie eines Nachtblocks nur am Starttag", () => {
        renderTable({
          from: new Date(2026, 0, 5),
          to: new Date(2026, 0, 6),
          sessions: [
            {
              ...morningHomeOffice,
              id: "night",
              date: "2026-01-05",
              check_in_time: "2026-01-05T22:00:00+01:00",
              check_out_time: "2026-01-06T02:00:00+01:00",
              net_minutes: 240,
              audit_count: 1,
            },
          ],
        });

        const expandable = screen.getAllByLabelText(/Änderungshistorie öffnen/);
        expect(expandable).toHaveLength(1);
        expect(expandable[0]).toHaveAccessibleName(/05\.01\./);
      });

      it("zeigt einen Ladefehler der Änderungshistorie an Stelle der Liste", async () => {
        const fetchEdits = vi
          .fn()
          .mockRejectedValueOnce(
            new ApiError("boom", 500, { code: "general.server" }),
          )
          .mockResolvedValueOnce([]);
        render(
          <StaffSessionTable
            staffId="1"
            from={new Date(2026, 0, 5)}
            to={new Date(2026, 0, 5)}
            sessions={[{ ...morningHomeOffice, audit_count: 1 }]}
            schedule={schedule}
            accountStartDate=""
            accountStartDatePending={false}
            accountStartDateError={false}
            today={today}
            isAdminView
            fetchEdits={fetchEdits}
          />,
        );

        fireEvent.click(
          screen.getAllByLabelText(/Änderungshistorie öffnen/)[0]!,
        );

        expect(
          await screen.findByText(
            catalogText("general.server", "die Liste der Änderungen"),
          ),
        ).toBeInTheDocument();
        fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
        await vi.waitFor(() => expect(fetchEdits).toHaveBeenCalledTimes(2));
      });

      it("behält einen Block ohne volle Minute in der Tabelle", () => {
        renderTable({
          from: new Date(2026, 0, 6),
          to: new Date(2026, 0, 6),
          sessions: [
            {
              ...morningHomeOffice,
              id: "fresh",
              date: "2026-01-06",
              check_in_time: "2026-01-06T08:59:40+01:00",
              check_out_time: null,
              net_minutes: 0,
            },
          ],
        });

        expect(screen.getAllByText("eingestempelt").length).toBeGreaterThan(0);
      });
    });
    // Die Segmentierung liest die Uhr. Bleibt die Seite über Mitternacht offen,
    // ohne dass sich die Sessions ändern, muss der laufende Nachtblock trotzdem
    // auf dem neuen Tag ankommen — sonst hängt er für immer am Vortag.
    describe("Tageswechsel bei offener Seite", () => {
      beforeEach(() => {});

      it("zieht einen laufenden Nachtblock nach Mitternacht auf den neuen Tag", () => {
        const runningNight: StaffHistorySession = {
          ...morningHomeOffice,
          id: "running-night",
          date: "2026-01-05",
          check_in_time: "2026-01-05T22:00:00+01:00",
          check_out_time: null,
          net_minutes: 90,
        };
        const props: TableProps = {
          from: new Date(2026, 0, 5),
          to: new Date(2026, 0, 6),
          sessions: [runningNight],
        };

        setTestClock(new Date("2026-01-05T22:30:00Z")); // 23:30 Berlin
        const view = render(
          tableElement({ ...props, today: new Date(2026, 0, 5) }),
        );
        expect(screen.getByText("06.01.").closest("tr")).not.toHaveTextContent(
          "00:00",
        );

        setTestClock(new Date("2026-01-05T23:30:00Z")); // 00:30 Berlin, neuer Tag
        view.rerender(tableElement({ ...props, today: new Date(2026, 0, 6) }));

        expect(screen.getByText("06.01.").closest("tr")).toHaveTextContent(
          "00:00",
        );
      });
    });
  });
});

describe("staff-session-table.edit", () => {
  // Mo–Fr 8h nach dem heute gültigen Plan — nur damit die Zeilen sichtbar sind.
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

  const mondaySickAbsence: StaffAbsenceRow = {
    id: 7,
    staff_id: 1,
    absence_type: "sick",
    date_start: "2026-01-05",
    date_end: "2026-01-05",
    half_day: true,
    note: "",
    status: "approved",
  };

  const mondayFullDayAbsence: StaffAbsenceRow = {
    ...mondaySickAbsence,
    half_day: false,
  };

  function renderTable(props: {
    sessions?: readonly StaffHistorySession[];
    absences?: readonly StaffAbsenceRow[];
    absencesUnresolved?: boolean;
  }) {
    return render(
      <StaffSessionTable
        staffId="1"
        from={from}
        to={to}
        sessions={props.sessions ?? []}
        absences={props.absences}
        absencesUnresolved={props.absencesUnresolved}
        schedule={schedule}
        accountStartDate=""
        accountStartDatePending={false}
        accountStartDateError={false}
        today={today}
        isAdminView
      />,
    );
  }

  // Zeilenaktionen liegen im Kebab der Zeile (Bauart 1 Regel 4): das Menü der
  // Tageszeile öffnen und den Eintrag liefern; ohne Bearbeitungsrecht gibt es
  // kein Menü in der Zeile.
  function dayMenuItem(row: HTMLElement, name: string) {
    fireEvent.click(within(row).getByRole("button", { name: /^Aktionen für/ }));
    return screen.getByRole("menuitem", { name });
  }
  function queryDayMenu(row: HTMLElement) {
    return within(row).queryByRole("button", { name: /^Aktionen für/ });
  }

  // Der Stift darf auf Tagen mit Abwesenheit nicht fehlen (#2361): eine halbe
  // Krankmeldung schließt Arbeitszeit am selben Tag nicht aus, und Nachtragen
  // muss auch dort erreichbar sein.
  describe("StaffSessionTable Stift-Verfügbarkeit", () => {
    it("zeigt den Nachtragen-Stift auch auf einem Tag mit Abwesenheit ohne Buchung", () => {
      renderTable({ absences: [mondaySickAbsence] });

      const row = screen.getByText("05.01.").closest("tr");
      expect(row).not.toBeNull();
      expect(dayMenuItem(row!, "Eintrag nachtragen")).toBeInTheDocument();
    });

    it("zeigt den Bearbeiten-Stift auf einem Tag mit Buchung und Abwesenheit", () => {
      renderTable({
        sessions: [mondaySession],
        absences: [mondaySickAbsence],
      });

      const row = screen.getByText("05.01.").closest("tr");
      expect(row).not.toBeNull();
      expect(dayMenuItem(row!, "Eintrag bearbeiten")).toBeInTheDocument();
    });

    it.each([
      ["sick", "reported"],
      ["sick", "approved"],
      ["vacation", "reported"],
      ["vacation", "approved"],
      ["training", "reported"],
      ["training", "approved"],
    ])(
      "zeigt keinen Nachtragen-Stift bei ganztägiger %s-Abwesenheit mit Status %s",
      (absenceType, status) => {
        renderTable({
          absences: [
            { ...mondayFullDayAbsence, absence_type: absenceType, status },
          ],
        });

        const row = screen.getByText("05.01.").closest("tr");
        expect(row).not.toBeNull();
        expect(queryDayMenu(row!)).not.toBeInTheDocument();
      },
    );

    it("zeigt keinen Nachtragen-Stift, solange Abwesenheitsdaten nicht aufgelöst sind", () => {
      renderTable({ absencesUnresolved: true });

      const row = screen.getByText("05.01.").closest("tr");
      expect(row).not.toBeNull();
      expect(queryDayMenu(row!)).not.toBeInTheDocument();
    });

    it.each(["requested", "question", "declined", "canceled"])(
      "erlaubt Nachtragen bei ganztägiger nicht wirksamer Abwesenheit mit Status %s",
      (status) => {
        renderTable({
          absences: [{ ...mondayFullDayAbsence, status }],
        });

        const row = screen.getByText("05.01.").closest("tr");
        expect(row).not.toBeNull();
        expect(dayMenuItem(row!, "Eintrag nachtragen")).toBeInTheDocument();
      },
    );

    it("lässt eine bestehende Buchung während des Abwesenheiten-Ladens bearbeiten", () => {
      renderTable({ sessions: [mondaySession], absencesUnresolved: true });

      const row = screen.getByText("05.01.").closest("tr");
      expect(row).not.toBeNull();
      expect(dayMenuItem(row!, "Eintrag bearbeiten")).toBeInTheDocument();
    });
  });
});

describe("staff-session-table.backfill", () => {
  // „Abwesenheit nachtragen" (#3258): ein vergangener Tag ohne Eintrag bekommt
  // statt Arbeitszeit eine Abwesenheit.
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

  const mondayHalfSick: StaffAbsenceRow = {
    id: 7,
    staff_id: 1,
    absence_type: "sick",
    date_start: "2026-01-05",
    date_end: "2026-01-05",
    half_day: true,
    note: "",
    status: "reported",
  };

  function renderTable(props: {
    sessions?: readonly StaffHistorySession[];
    absences?: readonly StaffAbsenceRow[];
    onBackfillAbsence?: (date: Date) => void;
    from?: Date;
    to?: Date;
    today?: Date;
  }) {
    return render(
      <StaffSessionTable
        staffId="1"
        from={props.from ?? from}
        to={props.to ?? to}
        sessions={props.sessions ?? []}
        absences={props.absences ?? []}
        schedule={schedule}
        accountStartDate=""
        accountStartDatePending={false}
        accountStartDateError={false}
        today={props.today ?? today}
        isAdminView
        onBackfillAbsence={props.onBackfillAbsence}
      />,
    );
  }

  function openDayMenu(day: string) {
    const row = screen.getByText(day).closest("tr");
    expect(row).not.toBeNull();
    fireEvent.click(
      within(row!).getByRole("button", { name: /^Aktionen für/ }),
    );
  }

  describe("StaffSessionTable Abwesenheit nachtragen", () => {
    it("bietet auf einem Tag ohne Eintrag die Abwesenheit an", () => {
      const onBackfillAbsence = vi.fn();
      renderTable({ onBackfillAbsence });

      openDayMenu("05.01.");
      fireEvent.click(
        screen.getByRole("menuitem", { name: "Abwesenheit nachtragen" }),
      );

      expect(onBackfillAbsence).toHaveBeenCalledTimes(1);
      const [day, kind] = onBackfillAbsence.mock.calls[0]! as [Date, string];
      expect([day.getFullYear(), day.getMonth(), day.getDate()]).toEqual([
        2026, 0, 5,
      ]);
      expect(kind).toBe("absence");
    });

    it("bietet auf einem Tag ohne Eintrag die Krankmeldung an", () => {
      const onBackfillAbsence = vi.fn();
      renderTable({ onBackfillAbsence });

      openDayMenu("05.01.");
      fireEvent.click(
        screen.getByRole("menuitem", { name: "Krankmeldung nachtragen" }),
      );

      expect(onBackfillAbsence).toHaveBeenCalledWith(expect.any(Date), "sick");
    });

    it("bietet sie nicht an, wenn der Tag schon Arbeitszeit hat", () => {
      renderTable({ sessions: [mondaySession], onBackfillAbsence: vi.fn() });

      openDayMenu("05.01.");
      expect(
        screen.queryByRole("menuitem", { name: "Abwesenheit nachtragen" }),
      ).not.toBeInTheDocument();
    });

    it("bietet sie nicht an, wenn der Tag schon eine Abwesenheit hat", () => {
      renderTable({ absences: [mondayHalfSick], onBackfillAbsence: vi.fn() });

      openDayMenu("05.01.");
      expect(
        screen.queryByRole("menuitem", { name: "Abwesenheit nachtragen" }),
      ).not.toBeInTheDocument();
    });

    it("bietet sie ohne Berechtigung zum Eintragen nicht an", () => {
      renderTable({});

      openDayMenu("05.01.");
      expect(
        screen.queryByRole("menuitem", { name: "Abwesenheit nachtragen" }),
      ).not.toBeInTheDocument();
      expect(
        screen.getByRole("menuitem", { name: "Eintrag nachtragen" }),
      ).toBeInTheDocument();
    });

    it("bietet sie nicht für den aktuellen oder einen zukünftigen Tag an", () => {
      const sameDay = new Date(2026, 0, 5);
      renderTable({
        from: sameDay,
        to: new Date(2026, 0, 6),
        today: sameDay,
        onBackfillAbsence: vi.fn(),
      });

      openDayMenu("05.01.");
      expect(
        screen.queryByRole("menuitem", { name: "Abwesenheit nachtragen" }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("menuitem", { name: "Krankmeldung nachtragen" }),
      ).not.toBeInTheDocument();

      const futureRow = screen.getByText("06.01.").closest("tr");
      expect(futureRow).not.toBeNull();
      expect(
        within(futureRow!).queryByRole("button", { name: /^Aktionen für/ }),
      ).not.toBeInTheDocument();
    });
  });
});

describe("staff-session-table.override", () => {
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

  function projected(targetMinutes: number, isOverride = false): DayProjection {
    return {
      targetMinutes,
      creditMinutes: 0,
      actualMinutes: 0,
      balanceMinutes: -targetMinutes,
      ...(isOverride ? { isOverride: true } : {}),
    };
  }

  // Mo–Fr 05.–09.01.2026 are closing days. Monday carries a Sonderarbeitszeit of
  // 8,5 h, Tuesday one of 0 h; Wednesday is a plain closing day.
  function renderClosureWeek() {
    const closing = new Map(
      [
        "2026-01-05",
        "2026-01-06",
        "2026-01-07",
        "2026-01-08",
        "2026-01-09",
      ].map((date) => [date, "Herbstferien"] as const),
    );
    return render(
      <StaffSessionTable
        staffId="1"
        from={new Date(2026, 0, 5)}
        to={new Date(2026, 0, 9)}
        sessions={[]}
        schedule={schedule}
        dailyProjection={
          new Map([
            ["2026-01-05", projected(510, true)],
            ["2026-01-06", projected(0, true)],
            ["2026-01-07", projected(0)],
            ["2026-01-08", projected(0)],
            ["2026-01-09", projected(0)],
          ])
        }
        closingDays={closing}
        accountStartDate=""
        accountStartDatePending={false}
        accountStartDateError={false}
        today={new Date(2026, 5, 15)}
        isAdminView
      />,
    );
  }

  function row(date: string): HTMLElement {
    const cell = screen.getByText(date).closest("tr");
    if (!cell) throw new Error(`row ${date} missing`);
    return cell;
  }

  describe("StaffSessionTable Sonderarbeitszeit (#3259)", () => {
    it("shows the override Soll, names both sources and keeps „Nicht erfasst“", () => {
      renderClosureWeek();

      const monday = row("05.01.");
      expect(within(monday).getByText("8h 30min")).toBeInTheDocument();
      expect(
        within(monday).getByText("Sonderarbeitszeit · Schließtag"),
      ).toBeInTheDocument();
      expect(within(monday).getByText("Nicht erfasst")).toBeInTheDocument();
      expect(within(monday).queryByText("Schließtag")).toBeNull();
    });

    it("shows 0min on a 0-hour Sonderarbeitszeit, the closure badge stays", () => {
      renderClosureWeek();

      const tuesday = row("06.01.");
      expect(within(tuesday).getByText("0min")).toBeInTheDocument();
      expect(within(tuesday).getByText("Schließtag")).toBeInTheDocument();
      expect(within(tuesday).queryByText("Nicht erfasst")).toBeNull();
    });

    it("leaves a plain closing day without a Soll marker", () => {
      renderClosureWeek();

      const wednesday = row("07.01.");
      expect(within(wednesday).getByText("Schließtag")).toBeInTheDocument();
      expect(within(wednesday).queryByText(/Sonderarbeitszeit/)).toBeNull();
      expect(within(wednesday).queryByText("0min")).toBeNull();
    });
  });
});

describe("staff-session-table.invalidation", () => {
  // A correction or backfill in the admin edit modal rewrites the day's Ist —
  // and with it the Gutschrift and the Saldo the server now projects per day
  // (#2443). Every cache showing those numbers has to be invalidated, or the
  // table keeps the pre-save Saldo while the Monatskarte above it has already
  // updated. useSWRAuth prefixes keys with the tenant slug, so the predicate
  // matches with includes.
  describe("isStaleAfterSessionSave", () => {
    it("invalidates the sessions, absences, month and daily-projection caches", () => {
      for (const key of [
        "phoenix:staff-history-42-2026-08-01-2026-08-31",
        "phoenix:staff-absences-42-2026-08-01-2026-08-31",
        "phoenix:staff-month-summary-42-2026-8",
        "phoenix:staff-schedule-targets-42-2026-08-01-2026-08-31",
        // Own-service portal keys (no staff id): a manager correcting their OWN
        // days also has the self-service table and weekly KPI open, which key
        // without an id and read the same projection.
        "phoenix:time-tracking-month-summary-2026-8",
        "phoenix:time-tracking-schedule-targets-2026-08-01-2026-08-31",
      ]) {
        expect(isStaleAfterSessionSave(key, "42")).toBe(true);
      }
    });

    it("leaves the target-only account chart and unrelated caches alone", () => {
      for (const key of [
        // Fetched with target_only=true: pure Soll, which a session edit cannot
        // change. Refetching the whole account range on every correction would
        // be a large request for an unchanged answer.
        "phoenix:staff-schedule-targets-account-42-2026-01-01-2026-12-31",
        "phoenix:staff-schedule-42",
        "phoenix:time-tracking-holidays-2026-08-01-2026-08-31",
        "phoenix:time-tracking-closing-days-2026-08-01-2026-08-31",
        "time-tracking-config",
      ]) {
        expect(isStaleAfterSessionSave(key, "42")).toBe(false);
      }
    });

    it("scopes the daily projection to the edited staff member", () => {
      expect(
        isStaleAfterSessionSave(
          "phoenix:staff-schedule-targets-7-2026-08-01-2026-08-31",
          "42",
        ),
      ).toBe(false);
    });

    it("ignores non-string SWR keys", () => {
      expect(isStaleAfterSessionSave(null, "42")).toBe(false);
      expect(isStaleAfterSessionSave(["staff-history-42"], "42")).toBe(false);
      expect(isStaleAfterSessionSave(undefined, "42")).toBe(false);
    });
  });
});
