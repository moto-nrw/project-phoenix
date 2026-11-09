import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import {
  fireEvent,
  render as rtlRender,
  screen,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { TraegerCalendar } from "~/lib/traeger-closing-days";

const { api, session } = vi.hoisted(() => ({
  api: {
    fetchTraegerCalendar: vi.fn(),
    createTraegerClosingDay: vi.fn(),
    updateTraegerClosingDay: vi.fn(),
    deleteTraegerClosingDay: vi.fn(),
  },
  session: { permissions: ["traeger_overview:read"] as string[] },
}));

vi.mock("~/lib/traeger-closing-days", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...api,
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: {
        id: "A",
        scope: "org",
        orgId: 7,
        permissions: session.permissions,
      },
    },
    status: "authenticated",
  }),
}));

import TraegerClosingDaysPage from "./page";

const calendar: TraegerCalendar = {
  schools: [
    {
      id: "1",
      name: "OGS Pforzheim",
      federalStateCode: "DE-BW",
      federalStateName: "Baden-Württemberg",
    },
    {
      id: "2",
      name: "OGS Iserlohn",
      federalStateCode: "DE-NW",
      federalStateName: "Nordrhein-Westfalen",
    },
  ],
  closingDays: [
    {
      id: "9",
      startDate: "2026-11-02",
      endDate: "2026-11-02",
      reason: "Brückentag",
      schoolIds: ["1", "2"],
    },
  ],
  regions: [
    {
      code: "DE-BW",
      name: "Baden-Württemberg",
      holidays: [{ date: "2026-12-25", name: "1. Weihnachtstag" }],
    },
    {
      code: "DE-NW",
      name: "Nordrhein-Westfalen",
      holidays: [
        { date: "2026-11-01", name: "Allerheiligen" },
        { date: "2026-12-25", name: "1. Weihnachtstag" },
      ],
    },
  ],
};

function render(ui: React.ReactElement) {
  return rtlRender(
    <SWRConfig value={{ provider: () => new Map() }}>
      <ToastProvider>{ui}</ToastProvider>
    </SWRConfig>,
  );
}

describe("TraegerClosingDaysPage (#4702)", () => {
  beforeEach(() => {
    api.fetchTraegerCalendar.mockReset().mockResolvedValue(calendar);
    session.permissions = ["traeger_overview:read"];
  });

  it("shows the closing days, the year overview and each school's federal state read only", async () => {
    render(<TraegerClosingDaysPage />);

    const table = await screen.findByRole("table");
    expect(within(table).getByText("Brückentag")).toBeInTheDocument();
    expect(within(table).getByText("Alle Schulen")).toBeInTheDocument();
    expect(api.fetchTraegerCalendar).toHaveBeenCalledWith(
      "2026-01-01",
      "2026-12-31",
    );
    expect(
      screen.getByRole("img", { name: /2\. November: Schließtag: Brückentag/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("img", {
        name: /1\. November: Feiertag in Nordrhein-Westfalen: Allerheiligen/,
      }),
    ).toBeInTheDocument();
    const nrw = screen.getByRole("region", { name: "Nordrhein-Westfalen" });
    expect(within(nrw).getByText("Gilt für: OGS Iserlohn")).toBeInTheDocument();
    // Ohne das Recht „Einstellungen“ plant niemand Schließtage.
    expect(
      screen.queryByRole("button", { name: "Schließtag anlegen" }),
    ).not.toBeInTheDocument();
    expect(
      within(table).queryByRole("button", { name: /Aktionen für/ }),
    ).not.toBeInTheDocument();
  });

  it("lets the settings right plan a closing day and switches the year", async () => {
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    render(<TraegerClosingDaysPage />);

    fireEvent.click(await screen.findByRole("button", { name: "2027" }));
    expect(api.fetchTraegerCalendar).toHaveBeenLastCalledWith(
      "2027-01-01",
      "2027-12-31",
    );

    fireEvent.click(screen.getByRole("button", { name: "Schließtag anlegen" }));
    expect(
      await screen.findByRole("dialog", { name: "Schließtag anlegen" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("checkbox", { name: "Alle Schulen" }),
    ).toBeChecked();
  });
});
