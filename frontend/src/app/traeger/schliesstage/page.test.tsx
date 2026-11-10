import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import {
  fireEvent,
  render as rtlRender,
  screen,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  DESKTOP_WIDTH,
  PHONE_WIDTH,
  setViewportWidth,
} from "~/test/helpers/viewport";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type {
  HolidayClosureImpact,
  TraegerCalendar,
} from "~/lib/traeger-closing-days";

const { api, session } = vi.hoisted(() => ({
  api: {
    fetchTraegerCalendar: vi.fn(),
    createTraegerClosingDay: vi.fn(),
    updateTraegerClosingDay: vi.fn(),
    deleteTraegerClosingDay: vi.fn(),
    previewTraegerClosingDayHolidayBookings: vi.fn(),
    fetchPendingHolidayClosures: vi.fn(),
    notifyPendingHolidayClosures: vi.fn(),
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

const noImpact: HolidayClosureImpact = {
  weeks: [],
  children: 0,
  families: 0,
  token: "",
  keepsContribution: true,
};

const summerImpact: HolidayClosureImpact = {
  weeks: [
    {
      weekId: "31",
      periodName: "Sommerferien",
      startsOn: "2026-11-02",
      endsOn: "2026-11-06",
      hostSchoolId: "1",
      closedDays: [{ date: "2026-11-04", reason: "Betriebsausflug" }],
      children: 5,
      waitlisted: 1,
      families: 3,
    },
  ],
  children: 5,
  families: 3,
  token: "abc123",
  keepsContribution: true,
};

function render(ui: React.ReactElement) {
  return rtlRender(
    <SWRConfig value={{ provider: () => new Map() }}>
      <ToastProvider>{ui}</ToastProvider>
    </SWRConfig>,
  );
}

describe("TraegerClosingDaysPage (#4702)", () => {
  afterEach(() => setViewportWidth(DESKTOP_WIDTH));
  beforeEach(() => {
    api.fetchTraegerCalendar.mockReset().mockResolvedValue(calendar);
    api.previewTraegerClosingDayHolidayBookings
      .mockReset()
      .mockResolvedValue(noImpact);
    api.fetchPendingHolidayClosures.mockReset().mockResolvedValue(noImpact);
    api.notifyPendingHolidayClosures.mockReset().mockResolvedValue(3);
    api.createTraegerClosingDay.mockReset();
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
  it("keeps school scope and edit/delete actions on phones", async () => {
    setViewportWidth(PHONE_WIDTH);
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    api.fetchTraegerCalendar.mockResolvedValue({
      ...calendar,
      closingDays: [{ ...calendar.closingDays[0]!, schoolIds: ["1"] }],
    });
    render(<TraegerClosingDaysPage />);
    const actions = await screen.findByRole("button", {
      name: "Aktionen für Brückentag",
    });
    const row = actions.closest("li")!;
    expect(within(row).getByText("OGS Pforzheim")).toBeInTheDocument();
    fireEvent.click(actions);
    expect(
      await screen.findByRole("menuitem", { name: "Ändern" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Löschen" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("menuitem", { name: "Ändern" }));
    expect(
      await screen.findByRole("dialog", { name: "Schließtag bearbeiten" }),
    ).toBeInTheDocument();
  });

  it("shows the booked holiday weeks before saving and confirms them with the save (#6434)", async () => {
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    api.previewTraegerClosingDayHolidayBookings.mockResolvedValue(summerImpact);
    api.createTraegerClosingDay.mockResolvedValue({
      ...calendar.closingDays[0]!,
      reason: "Betriebsausflug",
      notifiedFamilies: 3,
    });
    render(<TraegerClosingDaysPage />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Schließtag anlegen" }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Schließtag anlegen",
    });
    expect(
      await within(dialog).findByText(
        "3 Familien bekommen nach dem Speichern eine E-Mail",
      ),
    ).toBeInTheDocument();
    expect(within(dialog).getByText("Sommerferien")).toBeInTheDocument();
    expect(within(dialog).getByText("5 Kinder")).toBeInTheDocument();
    expect(
      within(dialog).getByText(
        "Der Beitrag bleibt gleich. Er gilt für die ganze Woche.",
      ),
    ).toBeInTheDocument();

    fireEvent.change(within(dialog).getByLabelText("Grund"), {
      target: { value: "Betriebsausflug" },
    });
    fireEvent.click(
      within(dialog).getByRole("button", {
        name: "Speichern und Eltern informieren",
      }),
    );
    await screen.findByText(
      "Der Schließtag „Betriebsausflug“ ist gespeichert. 3 Familien bekommen eine E-Mail.",
    );
    expect(api.createTraegerClosingDay).toHaveBeenCalledWith(
      expect.objectContaining({ holidayBookingsToken: "abc123" }),
    );
  });

  it("loads the preview again when the bookings changed before the save (#6434)", async () => {
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    api.previewTraegerClosingDayHolidayBookings.mockResolvedValue(summerImpact);
    api.createTraegerClosingDay.mockRejectedValue(
      new ApiError("conflict", 409, {
        code: "traeger.closing_day_bookings_changed",
      }),
    );
    render(<TraegerClosingDaysPage />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Schließtag anlegen" }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Schließtag anlegen",
    });
    fireEvent.change(within(dialog).getByLabelText("Grund"), {
      target: { value: "Betriebsausflug" },
    });
    const save = await within(dialog).findByRole("button", {
      name: "Speichern und Eltern informieren",
    });
    const calls = api.previewTraegerClosingDayHolidayBookings.mock.calls.length;
    fireEvent.click(save);
    await vi.waitFor(() =>
      expect(
        api.previewTraegerClosingDayHolidayBookings.mock.calls.length,
      ).toBeGreaterThan(calls),
    );
    expect(
      screen.getByRole("dialog", { name: "Schließtag anlegen" }),
    ).toBeInTheDocument();
  });

  it("lists closed days nobody told the parents about and tells them on confirmation (#6434)", async () => {
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    api.fetchPendingHolidayClosures.mockResolvedValue(summerImpact);
    render(<TraegerClosingDaysPage />);

    expect(
      await screen.findByText("Eltern noch nicht informiert"),
    ).toBeInTheDocument();
    expect(screen.getByText("Sommerferien")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Eltern informieren" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "E-Mails senden" }),
    );
    await screen.findByText("3 Familien bekommen eine E-Mail.");
    expect(api.notifyPendingHolidayClosures).toHaveBeenCalledWith("abc123");
  });
});
