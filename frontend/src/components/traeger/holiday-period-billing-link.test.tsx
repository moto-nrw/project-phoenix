import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { HolidayPeriod } from "~/lib/traeger-api";
import type { BillingRun } from "~/lib/traeger-billing-api";

const { state } = vi.hoisted(() => ({
  state: {
    permissions: [] as string[],
    runs: undefined as BillingRun[] | undefined,
    keys: [] as (string | null)[],
  },
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { permissions: state.permissions } },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    state.keys.push(key);
    return { data: key ? state.runs : undefined };
  },
}));
vi.mock("~/lib/traeger-billing-api", () => ({ fetchBillingRuns: vi.fn() }));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));

import {
  billingPathForPeriod,
  HolidayPeriodBillingLink,
} from "./holiday-period-billing-link";

const PERIOD = { id: "5", status: "published" } as HolidayPeriod;

function run(id: string, holidayPeriodId: string): BillingRun {
  return {
    id,
    holidayPeriodId,
    periodName: "Herbstferien 2026",
    serviceFrom: "2026-10-12",
    serviceUntil: "2026-10-23",
    status: "calculated",
    createdAt: "2026-09-01T08:00:00Z",
    releasedAt: null,
    issuedOn: null,
    documentCount: null,
    totalCents: null,
  };
}

describe("billingPathForPeriod", () => {
  it("opens the only run of the period", () => {
    expect(billingPathForPeriod("5", [run("9", "5"), run("3", "7")])).toBe(
      "/traeger/abrechnung/9",
    );
  });

  it("filters the list with several runs, none, or before they load", () => {
    const filtered = "/traeger/abrechnung?ferienzeitraum=5";
    expect(billingPathForPeriod("5", [run("9", "5"), run("10", "5")])).toBe(
      filtered,
    );
    expect(billingPathForPeriod("5", [run("3", "7")])).toBe(filtered);
    expect(billingPathForPeriod("5", undefined)).toBe(filtered);
  });
});

describe("HolidayPeriodBillingLink", () => {
  beforeEach(() => {
    state.permissions = ["traeger_billing:manage"];
    state.runs = [run("9", "5")];
    state.keys = [];
  });

  it("leads to the run of the period with the billing right", () => {
    render(<HolidayPeriodBillingLink period={PERIOD} />);

    expect(
      screen.getByRole("link", { name: "Zur Abrechnung" }),
    ).toHaveAttribute("href", "/abrechnung/9");
  });

  it("leads to the filtered list while the period has several runs", () => {
    state.runs = [run("9", "5"), run("10", "5")];
    render(<HolidayPeriodBillingLink period={PERIOD} />);

    expect(
      screen.getByRole("link", { name: "Zur Abrechnung" }),
    ).toHaveAttribute("href", "/abrechnung?ferienzeitraum=5");
  });

  // Die sichtbare Umgebung, gegen die jede verborgene Zeile prüft.
  const VISIBLE = {
    permissions: ["traeger_billing:manage"],
    period: PERIOD,
  };
  const HIDDEN = [
    {
      name: "without the billing right",
      ...VISIBLE,
      permissions: ["traeger_enrollment:manage"],
    },
    {
      name: "for a draft period, which has no runs",
      ...VISIBLE,
      period: { ...PERIOD, status: "draft" } as HolidayPeriod,
    },
  ];

  it.each(HIDDEN)("stays hidden and loads no runs $name", (row) => {
    state.permissions = row.permissions;
    const hidden = render(<HolidayPeriodBillingLink period={row.period} />);

    expect(hidden.container).toBeEmptyDOMElement();
    expect(state.keys).toEqual([null]);
    hidden.unmount();

    state.permissions = VISIBLE.permissions;
    state.keys = [];
    render(<HolidayPeriodBillingLink period={VISIBLE.period} />);
    // Anker: dieselbe Umgebung mit Recht und veröffentlichtem Zeitraum zeigt die Aktion.
    expect(
      screen.getByRole("link", { name: "Zur Abrechnung" }),
    ).toBeInTheDocument();
  });
});
