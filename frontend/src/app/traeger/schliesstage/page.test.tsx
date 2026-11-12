import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import {
  fireEvent,
  render as rtlRender,
  screen,
  within,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  DESKTOP_WIDTH,
  PHONE_WIDTH,
  setViewportWidth,
} from "~/test/helpers/viewport";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import type {
  HolidayClosureImpact,
  PendingHolidayClosures,
  TraegerCalendar,
  TraegerHolidayClosureNotice,
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
    notifyTraegerHolidayClosureNotice: vi.fn(),
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
      reopenedDays: [],
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

const noPending: PendingHolidayClosures = { ...noImpact, notices: [] };

function schoolNotice(
  overrides: Partial<TraegerHolidayClosureNotice> = {},
): TraegerHolidayClosureNotice {
  return {
    id: "41",
    schoolId: "1",
    schoolName: "",
    closingDayId: "5",
    reason: "Pädagogischer Tag",
    previous: null,
    current: { startDate: "2026-11-04", endDate: "2026-11-04" },
    revision: 3,
    token: "impact-preview",
    status: "suggested",
    changed: false,
    canNotify: true,
    families: 3,
    confirmedAt: null,
    updatedAt: "2026-11-01T10:00:00Z",
    weeks: summerImpact.weeks,
    children: 5,
    ...overrides,
  };
}

function pendingWith(
  ...notices: TraegerHolidayClosureNotice[]
): PendingHolidayClosures {
  return { ...summerImpact, notices };
}

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
    api.fetchPendingHolidayClosures.mockReset().mockResolvedValue(noPending);
    api.notifyPendingHolidayClosures.mockReset().mockResolvedValue(3);
    api.notifyTraegerHolidayClosureNotice.mockReset();
    api.deleteTraegerClosingDay.mockReset();
    api.createTraegerClosingDay.mockReset();
    session.permissions = ["traeger_overview:read"];
  });

  it("shows unknown standalone mail without offering a blind retry", async () => {
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    api.fetchPendingHolidayClosures.mockResolvedValue({
      ...noPending,
      unknownMails: 1,
    });
    render(<TraegerClosingDaysPage />);
    expect(
      await screen.findByText(/Bei 1 E-Mail ist das Ergebnis unbekannt/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Erneut senden" }),
    ).not.toBeInTheDocument();
  });

  it("offers mail recovery on a phone without requiring a school notice", async () => {
    setViewportWidth(PHONE_WIDTH);
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    api.fetchPendingHolidayClosures.mockResolvedValue({
      ...noPending,
      token: "mail-recovery",
      retryableMails: 2,
      unknownMails: 1,
    });
    api.notifyPendingHolidayClosures.mockResolvedValue(0);
    render(<TraegerClosingDaysPage />);
    const retry = await screen.findByRole("button", { name: "Erneut senden" });
    expect(
      screen.getByText(/^2 E-Mails sind nicht angekommen\./),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Bei 1 E-Mail ist das Ergebnis unbekannt/),
    ).toBeInTheDocument();
    api.fetchPendingHolidayClosures.mockResolvedValue(noPending);
    fireEvent.click(retry);
    await waitFor(() =>
      expect(api.notifyPendingHolidayClosures).toHaveBeenCalledWith(
        "mail-recovery",
      ),
    );
    expect(
      await screen.findByText("Erneuter Versand ist beauftragt."),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: "Erneut senden" }),
      ).not.toBeInTheDocument(),
    );
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
      within(dialog).getByText("Der Beitrag bleibt gleich."),
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

  it("lists each school's notice with its weeks and tells the parents with one click (ADR 0196)", async () => {
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    const notice = schoolNotice();
    api.fetchPendingHolidayClosures.mockResolvedValue(pendingWith(notice));
    api.notifyTraegerHolidayClosureNotice.mockResolvedValue({
      ...notice,
      status: "sending",
      canNotify: false,
    });
    render(<TraegerClosingDaysPage />);

    const card = (
      await screen.findByText("Eltern noch nicht informiert")
    ).closest("section")!;
    expect(
      within(card).getByText("OGS Pforzheim: „Pädagogischer Tag“"),
    ).toBeInTheDocument();
    expect(
      within(card).getByText(
        "3 Familien haben an diesen Tagen Ferienbetreuung gebucht.",
      ),
    ).toBeInTheDocument();
    expect(
      within(card).getByText("Die Eltern wurden noch nicht informiert."),
    ).toBeInTheDocument();
    expect(within(card).getAllByText("5 Kinder")).toHaveLength(2);
    expect(within(card).getByText("Sommerferien")).toBeInTheDocument();
    expect(
      within(card).getByText("Geschlossen: Mi., 04.11.2026 (Betriebsausflug)"),
    ).toBeInTheDocument();
    // Ein Vorschlag allein braucht keinen zweiten Knopf für alle.
    expect(
      within(card).queryByRole("button", { name: /Alle Eltern informieren/ }),
    ).not.toBeInTheDocument();

    api.fetchPendingHolidayClosures.mockResolvedValue(
      pendingWith({ ...notice, status: "sending", canNotify: false }),
    );
    fireEvent.click(
      within(card).getByRole("button", {
        name: "Eltern informieren: OGS Pforzheim: „Pädagogischer Tag“",
      }),
    );
    // Kein zweiter Dialog: Der Klick beauftragt den Versand.
    expect(await within(card).findByText("Wird gesendet")).toBeInTheDocument();
    expect(within(card).getByText("Bitte warten Sie.")).toBeInTheDocument();
    expect(api.notifyTraegerHolidayClosureNotice).toHaveBeenCalledWith(
      "41",
      3,
      "impact-preview",
    );
    expect(api.notifyTraegerHolidayClosureNotice).toHaveBeenCalledTimes(1);
    await screen.findByText("Die E-Mails an 3 Familien werden gesendet.");
    expect(
      within(card).queryByRole("button", { name: /Eltern informieren/ }),
    ).not.toBeInTheDocument();
  });

  it("shows a changed notice through the error path and loads the new state (ADR 0196)", async () => {
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    const notice = schoolNotice();
    api.fetchPendingHolidayClosures.mockResolvedValue(pendingWith(notice));
    api.notifyTraegerHolidayClosureNotice.mockRejectedValue(
      new ApiError("conflict", 409, {
        code: "timetable.closing_day_notice_changed",
      }),
    );
    render(<TraegerClosingDaysPage />);

    const button = await screen.findByRole("button", {
      name: "Eltern informieren: OGS Pforzheim: „Pädagogischer Tag“",
    });
    api.fetchPendingHolidayClosures.mockResolvedValue(
      pendingWith({ ...notice, revision: 4, changed: true }),
    );
    fireEvent.click(button);
    expect(
      await screen.findByText(
        catalogText(
          "timetable.closing_day_notice_changed",
          "die E-Mail an die Eltern",
        ),
      ),
    ).toBeInTheDocument();
    expect(
      await screen.findByText(
        "Der Schließtag wurde geändert. Bitte prüfen Sie den Hinweis.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", {
        name: "Eltern informieren: OGS Pforzheim: „Pädagogischer Tag“",
      }),
    );
    await waitFor(() =>
      expect(api.notifyTraegerHolidayClosureNotice).toHaveBeenLastCalledWith(
        "41",
        4,
        "impact-preview",
      ),
    );
  });

  it("keeps failed and unknown sendings visible, with the button only where a retry is safe (ADR 0196)", async () => {
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    api.fetchPendingHolidayClosures.mockResolvedValue(
      pendingWith(
        schoolNotice({
          id: "51",
          reason: "Teamtag",
          status: "failed",
          canNotify: true,
          families: 0,
          weeks: [],
          children: 0,
        }),
        schoolNotice({
          id: "52",
          schoolId: "2",
          reason: "Umzug",
          status: "unknown",
          canNotify: false,
          families: 0,
          weeks: [],
          children: 0,
        }),
        schoolNotice({
          id: "53",
          reason: "Brückentag",
          status: "sent",
          canNotify: false,
          families: 0,
          weeks: [],
          children: 0,
        }),
      ),
    );
    render(<TraegerClosingDaysPage />);

    const card = (
      await screen.findByText("Eltern noch nicht informiert")
    ).closest("section")!;
    expect(within(card).getByText("Nicht angekommen")).toBeInTheDocument();
    expect(
      within(card).getByText("Bitte versuchen Sie es erneut."),
    ).toBeInTheDocument();
    expect(
      within(card).getByRole("button", {
        name: "Eltern informieren: OGS Pforzheim: „Teamtag“",
      }),
    ).toBeInTheDocument();
    expect(within(card).getByText("Ergebnis unbekannt")).toBeInTheDocument();
    expect(
      within(card).getByText(
        "Bitte lassen Sie den Versand vom moto-Team prüfen.",
      ),
    ).toBeInTheDocument();
    expect(
      within(card).queryByRole("button", {
        name: "Eltern informieren: OGS Iserlohn: „Umzug“",
      }),
    ).not.toBeInTheDocument();
    // Gesendet ist erledigt: zugeklappt darunter, ohne erneuten Versand.
    expect(
      screen.getByRole("button", { name: /Erledigte Hinweise \(1\)/ }),
    ).toBeInTheDocument();
    expect(
      within(card).queryByText("OGS Pforzheim: „Brückentag“"),
    ).not.toBeInTheDocument();
  });

  it("tells the parents of every proposal with one click on the head button (#6434)", async () => {
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    api.fetchPendingHolidayClosures.mockResolvedValue(
      pendingWith(
        schoolNotice(),
        schoolNotice({ id: "42", schoolId: "2", reason: "Teamtag" }),
      ),
    );
    render(<TraegerClosingDaysPage />);

    expect(await screen.findByText("2 offen")).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Alle Eltern informieren" }),
    );
    await screen.findByText("3 Familien bekommen eine E-Mail.");
    expect(api.notifyPendingHolidayClosures).toHaveBeenCalledWith("abc123");
  });

  it("shows the days that open again when a closing day is changed (#6457)", async () => {
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    api.previewTraegerClosingDayHolidayBookings.mockResolvedValue({
      ...summerImpact,
      weeks: [
        {
          ...summerImpact.weeks[0]!,
          closedDays: [],
          reopenedDays: [{ date: "2026-11-02", reason: "Brückentag" }],
        },
      ],
    });
    render(<TraegerClosingDaysPage />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Aktionen für Brückentag" }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Ändern" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Schließtag bearbeiten",
    });
    expect(
      await within(dialog).findByText(
        "Wieder geöffnet: Mo., 02.11.2026 (Brückentag)",
      ),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(
        "5 Kinder haben an diesen Tagen Ferienbetreuung gebucht. An den wieder geöffneten Tagen gibt es doch Betreuung.",
      ),
    ).toBeInTheDocument();
    expect(api.previewTraegerClosingDayHolidayBookings).toHaveBeenCalledWith(
      expect.objectContaining({ startDate: "2026-11-02" }),
      { closingDay: calendar.closingDays[0], delete: false },
    );
  });

  it("shows the reopened days before deleting and confirms them with the deletion (#6457)", async () => {
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    api.previewTraegerClosingDayHolidayBookings.mockResolvedValue({
      ...summerImpact,
      token: "del456",
      weeks: [
        {
          ...summerImpact.weeks[0]!,
          closedDays: [],
          reopenedDays: [{ date: "2026-11-02", reason: "Brückentag" }],
        },
      ],
    });
    api.deleteTraegerClosingDay.mockResolvedValue(3);
    render(<TraegerClosingDaysPage />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Aktionen für Brückentag" }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Löschen" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Schließtag löschen",
    });
    expect(
      await within(dialog).findByText(
        "3 Familien bekommen nach dem Löschen eine E-Mail",
      ),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText("Wieder geöffnet: Mo., 02.11.2026 (Brückentag)"),
    ).toBeInTheDocument();
    expect(api.previewTraegerClosingDayHolidayBookings).toHaveBeenCalledWith(
      expect.objectContaining({ startDate: "2026-11-02" }),
      { closingDay: calendar.closingDays[0], delete: true },
    );

    fireEvent.click(
      within(dialog).getByRole("button", { name: "Ja, löschen" }),
    );
    fireEvent.click(
      await within(dialog).findByRole("button", { name: "Endgültig löschen" }),
    );
    await screen.findByText(
      "Der Schließtag „Brückentag“ ist gelöscht. 3 Familien bekommen eine E-Mail.",
    );
    expect(api.deleteTraegerClosingDay).toHaveBeenCalledWith("9", "del456");
  });

  it("refreshes pending notices when their closing day is deleted", async () => {
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    api.fetchPendingHolidayClosures.mockResolvedValue(
      pendingWith(schoolNotice()),
    );
    api.deleteTraegerClosingDay.mockResolvedValue(0);
    render(<TraegerClosingDaysPage />);
    await screen.findByText("Eltern noch nicht informiert");
    fireEvent.click(
      await screen.findByRole("button", { name: "Aktionen für Brückentag" }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Löschen" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Schließtag löschen",
    });
    // Das Löschen wartet auf die Vorschau der Ferienbuchungen (#6457).
    expect(
      await within(dialog).findByText(
        "Keine gebuchte Ferienbetreuung betroffen.",
      ),
    ).toBeInTheDocument();
    api.fetchPendingHolidayClosures.mockResolvedValue(noPending);
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Ja, löschen" }),
    );
    fireEvent.click(
      await within(dialog).findByRole("button", { name: "Endgültig löschen" }),
    );
    await waitFor(() =>
      expect(api.deleteTraegerClosingDay).toHaveBeenCalledWith("9", ""),
    );
    await waitFor(() =>
      expect(
        screen.queryByText("Eltern noch nicht informiert"),
      ).not.toBeInTheDocument(),
    );
  });
});
