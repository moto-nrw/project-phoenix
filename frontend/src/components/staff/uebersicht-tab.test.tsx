import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { StaffAbsenceRow, StaffHistorySession } from "~/lib/staff-api";
import { catalogText } from "~/test/error-catalog-text";

// One file for every UebersichtTab test: each render case installs its own
// SWR responder, so the tab's module graph loads once instead of per scenario.
type SwrResult = {
  data: unknown;
  isLoading: boolean;
  error: unknown;
  mutate?: () => Promise<unknown>;
};
const swr = vi.hoisted(() => ({
  keys: [] as string[],
  respond: (_key: string | null): SwrResult | undefined => undefined,
}));
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    if (key) swr.keys.push(key);
    return (
      swr.respond(key) ?? {
        data: undefined,
        isLoading: false,
        error: undefined,
      }
    );
  },
  useTenantMutateMatching: () => () => Promise.resolve([]),
}));

const getSchedule = vi.fn();
const getScheduleTargetsRange = vi.fn();
vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: { getAbsences: vi.fn() },
  staffBalanceAdjustmentService: { list: vi.fn() },
  staffHistoryService: { getHistory: vi.fn() },
  staffScheduleService: {
    getSchedule: (...args: unknown[]) => getSchedule(...args),
  },
  staffMonthSummaryService: {
    getScheduleTargetsRange: (...args: unknown[]) =>
      getScheduleTargetsRange(...args),
    getMonthSummary: vi.fn(),
  },
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: vi.fn(), error: vi.fn(), info: vi.fn() }),
}));

vi.mock("~/lib/time-tracking-api", () => ({
  timeTrackingService: { getConfig: vi.fn() },
}));

// A fixed Berlin day distinct from the real browser date, so a regression to
// `new Date()` would key the target range against a different day (#1842).
vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => "2025-03-10",
}));

import {
  UebersichtTab,
  countSessionDaysInRange,
  indexSessionNetMinutesByBerlinDate,
} from "./uebersicht-tab";

beforeEach(() => {
  swr.keys.length = 0;
  swr.respond = () => undefined;
});

const retryMutate = vi.fn(() => Promise.resolve(undefined));

function failKey(fragment: string) {
  swr.respond = (key) =>
    typeof key === "string" && key.includes(fragment)
      ? {
          data: undefined,
          isLoading: false,
          error: new ApiError("down", 503, {
            code: "general.unavailable",
            instance: "req-overview",
          }),
          mutate: retryMutate,
        }
      : {
          data: undefined,
          isLoading: false,
          error: undefined,
          mutate: retryMutate,
        };
}

const LOAD_ERROR = catalogText("general.unavailable", "die Auswertung");

// Die Charts dieses Tabs bepreisen Vergangenheit. Sie dürfen ihr Soll NICHT
// aus dem AKTUELLEN Dienstplan ableiten: nach einer Vertragsänderung
// (8h -> 4h) rechneten sie Monate an Historie zu den heutigen Stunden nach und
// widersprachen damit der Stundenkonto-Überschrift direkt darüber, die
// datumsgültig vom Server kommt (#1842). Der Test pinnt die Eingaben: nur die
// datumsgültigen Targets, nie der Plan.
describe("UebersichtTab Soll-Quelle", () => {
  let balanceAdjustments: unknown[] = [];

  beforeEach(() => {
    balanceAdjustments = [];
    swr.respond = (key) =>
      key?.startsWith("staff-balance-adjustments-")
        ? { data: balanceAdjustments, isLoading: false, error: undefined }
        : undefined;
    getSchedule.mockClear();
    getScheduleTargetsRange.mockClear();
  });

  it("lädt datumsgültige Targets für den Kontozeitraum", () => {
    render(<UebersichtTab staffId="1" />);

    expect(
      swr.keys.some((k) => k.startsWith("staff-schedule-targets-account-1-")),
    ).toBe(true);
  });

  it("fragt den aktuellen Dienstplan gar nicht erst ab", () => {
    render(<UebersichtTab staffId="1" />);

    // Kein Plan-Key => keine Möglichkeit, ihn auf historische Tage anzuwenden.
    expect(swr.keys).not.toContain("staff-schedule-1");
  });

  it("leitet 'heute' aus der Berliner Zeit ab, nicht aus der Browser-Zeitzone", () => {
    render(<UebersichtTab staffId="1" />);

    // yearEndKey = toDateKey(berlinToday): der Kontozeitraum-Key endet auf dem
    // Berliner Tag (2025-03-10), nicht auf dem lokalen new Date().
    expect(
      swr.keys.some(
        (k) =>
          k.startsWith("staff-schedule-targets-account-1-") &&
          k.endsWith("-2025-03-10"),
      ),
    ).toBe(true);
  });

  it("lädt auch zukünftige Stundenkonto-Buchungen für die Verwaltung", () => {
    render(<UebersichtTab staffId="1" />);

    expect(
      swr.keys.some(
        (k) =>
          k.startsWith("staff-balance-adjustments-1-") &&
          k.endsWith("-9999-12-31"),
      ),
    ).toBe(true);
  });

  it("zeigt den Saldo-Verlauf für ein Konto mit ausschließlich Buchungen", () => {
    balanceAdjustments = [
      {
        id: "17",
        type: "payout",
        minutesDelta: -120,
        effectiveDate: "2025-02-10",
        note: "Auszahlung",
        decidedBy: "9",
        decidedAt: "2025-02-10T08:00:00Z",
      },
    ];

    render(<UebersichtTab staffId="1" />);

    expect(
      screen.queryByText(
        "Noch keine Daten — der Saldo erscheint, sobald die erste Woche erfasst ist.",
      ),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(
        "Noch keine Daten — sobald die erste Arbeitszeit erfasst ist, erscheint der Vergleich.",
      ),
    ).toBeInTheDocument();
  });
});

// Ein fehlgeschlagener Targets-Fetch darf NICHT auf leere Sollzeiten
// durchfallen: das bepreiste jeden Vertragstag mit 0 Soll und zeichnete eine
// Saldo-Linie, die wie ein riesiger Überschuss aussieht. Stattdessen wird der
// Fehler angezeigt (#1842).
describe("UebersichtTab Targets-Fehler", () => {
  beforeEach(() => failKey("staff-schedule-targets-"));

  it("zeigt einen Fehler statt einer 0-Soll-Auswertung", async () => {
    retryMutate.mockClear();
    render(<UebersichtTab staffId="1" />);

    expect(await screen.findByText(LOAD_ERROR)).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Vorgangskennung kopieren" }),
    ).toHaveTextContent("req-overview");
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(retryMutate).toHaveBeenCalled();
    // Keine Chart-Überschrift => es wird kein irreführender Chart gerendert.
    expect(screen.queryByText(/Tagesvergleich Ist \/ Soll/i)).toBeNull();
  });
});

// Ein fehlgeschlagener Buchungs-Fetch darf NICHT als leeres Buchungsprotokoll
// durchgehen: Auszahlungen und Resets fehlten dann sowohl im Protokoll
// ("Noch keine Buchungen") als auch in der Saldo-Kurve. Stattdessen wird der
// Fehler angezeigt (#1420).
describe("UebersichtTab Buchungs-Fehler", () => {
  beforeEach(() => failKey("staff-balance-adjustments-"));

  it("zeigt einen Fehler statt eines leeren Buchungsprotokolls", async () => {
    render(<UebersichtTab staffId="1" />);

    expect(await screen.findByText(LOAD_ERROR)).toBeVisible();
    // Kein Panel => kein irreführendes leeres Protokoll.
    expect(screen.queryByText(/Noch keine Buchungen/i)).toBeNull();
  });
});

// Fehlen Arbeitszeiten oder Abwesenheiten, wären Verteilung und Kurven
// unvollständig; auch dann steht der Fehler statt der Auswertung (#2514).
describe("UebersichtTab Verlaufs-Fehler", () => {
  it.each(["staff-history-account-", "staff-absences-account-"])(
    "zeigt einen Fehler statt einer Auswertung ohne %s",
    async (fragment) => {
      failKey(fragment);
      render(<UebersichtTab staffId="1" />);

      expect(await screen.findByText(LOAD_ERROR)).toBeVisible();
      expect(screen.queryByText(/Tagesvergleich Ist \/ Soll/i)).toBeNull();
    },
  );
});

describe("UebersichtTab Freizeitausgleich-Verteilung", () => {
  beforeEach(() => {
    const absences: StaffAbsenceRow[] = [
      {
        id: 1,
        staff_id: 1,
        absence_type: "comp_time",
        date_start: "2025-03-05",
        date_end: "2025-03-05",
        half_day: true,
        start_half_day: false,
        end_half_day: false,
        note: "",
        status: "reported",
      },
    ];
    swr.respond = (key) => {
      if (key?.startsWith("staff-absences-account-")) {
        return { data: absences, isLoading: false, error: undefined };
      }
      if (
        key?.startsWith("staff-history-account-") ||
        key?.startsWith("staff-balance-adjustments-")
      ) {
        return { data: [], isLoading: false, error: undefined };
      }
      if (key?.startsWith("staff-schedule-targets-account-")) {
        return { data: new Map(), isLoading: false, error: undefined };
      }
      if (key === "time-tracking-config") {
        return {
          data: { accountStartDate: "2025-01-01" },
          isLoading: false,
          error: undefined,
        };
      }
      return undefined;
    };
  });

  it("zählt einen halben Freizeitausgleichstag als 0,5 Tage", () => {
    render(<UebersichtTab staffId="1" />);

    expect(screen.getByText("0.5 Tage")).toBeVisible();
  });
});

describe("UebersichtTab Nachtblöcke", () => {
  it("teilt Nettozeit und Pause an der Berliner Tagesgrenze", () => {
    const session: StaffHistorySession = {
      date: "2026-07-20",
      net_minutes: 210,
      check_in_time: "2026-07-20T20:00:00.000Z", // 22:00 CEST
      check_out_time: "2026-07-21T00:00:00.000Z", // 02:00 CEST
      break_minutes: 30,
      breaks: [
        {
          started_at: "2026-07-20T22:30:00.000Z", // 00:30 CEST
          ended_at: "2026-07-20T23:00:00.000Z", // 01:00 CEST
        },
      ],
    };

    expect(indexSessionNetMinutesByBerlinDate([session])).toEqual(
      new Map([
        ["2026-07-20", 120],
        ["2026-07-21", 90],
      ]),
    );
  });

  it("zieht Legacy-Pausen zuerst vom ersten Tagesanteil ab", () => {
    const session: StaffHistorySession = {
      date: "2026-07-20",
      net_minutes: 210,
      check_in_time: "2026-07-20T20:00:00.000Z",
      check_out_time: "2026-07-21T00:00:00.000Z",
      break_minutes: 30,
    };

    expect(indexSessionNetMinutesByBerlinDate([session])).toEqual(
      new Map([
        ["2026-07-20", 90],
        ["2026-07-21", 120],
      ]),
    );
  });
});

const from = new Date(2026, 6, 20);
const to = new Date(2026, 6, 24);

describe("UebersichtTab Anwesenheitstage", () => {
  it("zählt einen Tag, dessen Block komplett aus Pause besteht", () => {
    // Der Block hat null Nettominuten, die Person war aber da. Ohne den
    // Tageseintrag verschwände der Tag aus "Anwesend" (#2402).
    const session: StaffHistorySession = {
      date: "2026-07-20",
      status: "present",
      net_minutes: 0,
      check_in_time: "2026-07-20T06:00:00.000Z", // 08:00 CEST
      check_out_time: "2026-07-20T10:00:00.000Z", // 12:00 CEST
      break_minutes: 240,
      breaks: [
        {
          started_at: "2026-07-20T06:00:00.000Z",
          ended_at: "2026-07-20T10:00:00.000Z",
        },
      ],
    };

    expect(countSessionDaysInRange([session], from, to)).toEqual({
      present: 1,
      homeOffice: 0,
    });
  });

  it("zählt einen frisch eingestempelten Homeoffice-Block als Homeoffice-Tag", () => {
    const now = new Date();
    const todayKey = [
      now.getFullYear(),
      String(now.getMonth() + 1).padStart(2, "0"),
      String(now.getDate()).padStart(2, "0"),
    ].join("-");
    const session: StaffHistorySession = {
      date: todayKey,
      status: "home_office",
      net_minutes: 0,
      check_in_time: new Date(now.getTime() - 20_000).toISOString(),
      check_out_time: null,
      break_minutes: 0,
    };
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());

    expect(countSessionDaysInRange([session], today, today)).toEqual({
      present: 0,
      homeOffice: 1,
    });
  });

  it("entscheidet bei gemischten Blöcken weiterhin über die Minuten", () => {
    const homeOffice: StaffHistorySession = {
      date: "2026-07-22",
      status: "home_office",
      net_minutes: 60,
      check_in_time: "2026-07-22T06:00:00.000Z",
      check_out_time: "2026-07-22T07:00:00.000Z",
      break_minutes: 0,
    };
    const onSite: StaffHistorySession = {
      date: "2026-07-22",
      status: "present",
      net_minutes: 240,
      check_in_time: "2026-07-22T08:00:00.000Z",
      check_out_time: "2026-07-22T12:00:00.000Z",
      break_minutes: 0,
    };

    expect(countSessionDaysInRange([homeOffice, onSite], from, to)).toEqual({
      present: 1,
      homeOffice: 0,
    });
  });
});
