import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { StaffHistorySession, StaffSchedule } from "~/lib/staff-api";
import type { DayProjection } from "~/lib/time-tracking-helpers";

import { StaffSessionTable } from "./staff-session-table";

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

    fireEvent.click(screen.getByLabelText("Block 2 bearbeiten"));

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

    fireEvent.click(screen.getByLabelText("Eintrag bearbeiten"));

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

    fireEvent.click(screen.getByLabelText("Block nachtragen"));

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
    expect(screen.getByLabelText("Eintrag bearbeiten")).toBeInTheDocument();
  });

  it("markiert den Tag als eingestempelt, wenn ein Block offen ist", () => {
    renderTable({
      sessions: [morningHomeOffice, { ...afternoonOgs, check_out_time: null }],
    });

    expect(screen.getAllByText("eingestempelt").length).toBeGreaterThan(0);
  });

  // Die Tagessegmente eines Nachtblocks enden dort, wo auch die gezeigten
  // Minuten enden. Vergleicht die Aufteilung stattdessen mit einem rohen
  // Checkout, findet kein Segment seinen letzten Tag und die letzte Zeile
  // erfindet 23:59.
  describe("gekappte Blöcke", () => {
    beforeEach(() => {
      vi.useFakeTimers({ toFake: ["Date"] });
      // 06.01.2026, 09:00 Berlin.
      vi.setSystemTime(new Date("2026-01-06T08:00:00Z"));
    });

    afterEach(() => {
      vi.useRealTimers();
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
    beforeEach(() => {
      vi.useFakeTimers({ toFake: ["Date"] });
    });

    afterEach(() => {
      vi.useRealTimers();
    });

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

      vi.setSystemTime(new Date("2026-01-05T22:30:00Z")); // 23:30 Berlin
      const view = render(
        tableElement({ ...props, today: new Date(2026, 0, 5) }),
      );
      expect(screen.getByText("06.01.").closest("tr")).not.toHaveTextContent(
        "00:00",
      );

      vi.setSystemTime(new Date("2026-01-05T23:30:00Z")); // 00:30 Berlin, neuer Tag
      view.rerender(tableElement({ ...props, today: new Date(2026, 0, 6) }));

      expect(screen.getByText("06.01.").closest("tr")).toHaveTextContent(
        "00:00",
      );
    });
  });
});
