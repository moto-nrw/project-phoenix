import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { BELOW_LG } from "~/lib/hooks/use-media-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { setTestClock } from "~/test/clock";
import {
  getParentAppointmentOverview,
  getParentCalendar,
  respondParentCalendar,
} from "~/lib/personal-calendar-api";
import { toISODate } from "~/lib/date-helpers";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { ParentCalendarPage } from "./parent-calendar-page";

const toastSuccess = vi.fn();
const toastError = vi.fn();
let searchParams = new URLSearchParams();

vi.mock("next/navigation", () => ({
  useSearchParams: () => searchParams,
}));

// Der echte Fehlerweg (useApiErrorDisplay) zeigt Fehler im ToastProvider;
// nur die direkten Erfolgs-Toasts der Seite werden abgefangen.
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: toastSuccess, error: toastError }),
}));

vi.mock("~/components/calendar/calendar-subscribe-panel", () => ({
  CalendarSubscribePanel: () => <div data-testid="subscribe-panel" />,
}));

vi.mock("~/lib/personal-calendar-api", () => ({
  getParentAppointmentOverview: vi.fn(),
  getParentCalendar: vi.fn(),
  respondParentCalendar: vi.fn(),
}));

const mockedCalendar = vi.mocked(getParentCalendar);
const mockedOverview = vi.mocked(getParentAppointmentOverview);
const mockedRespond = vi.mocked(respondParentCalendar);
const MONDAY = new Date(2026, 7, 17, 9, 0, 0);

function event(overrides: Record<string, unknown> = {}) {
  return {
    id: "e1",
    source: "appointment",
    appointment_id: "appointment-1",
    title: "Elternabend",
    description: "Wir besprechen das neue Schuljahr.",
    location: "Aula",
    student_name: "Hannah Klein",
    school_name: "Demo School",
    start_date: "2026-08-19",
    end_date: "2026-08-19",
    start_time: "18:00",
    end_time: "20:00",
    all_day: false,
    can_respond: false,
    can_edit: false,
    ...overrides,
  };
}

function calendarResponse(events: ReturnType<typeof event>[]) {
  return {
    from: "2026-08-17",
    to: "2026-11-16",
    events,
  } as unknown as Awaited<ReturnType<typeof getParentCalendar>>;
}

function renderPage() {
  return render(<ParentCalendarPage />, { wrapper: ToastProvider });
}

beforeEach(() => {
  vi.clearAllMocks();
  searchParams = new URLSearchParams();
  setTestClock(MONDAY);
  mockedCalendar.mockResolvedValue(calendarResponse([]));
  mockedRespond.mockResolvedValue(undefined as never);
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("ParentCalendarPage", () => {
  it("reserviert beim Laden Platz für Terminliste und Abo", () => {
    mockedCalendar.mockReturnValue(new Promise(() => {}));

    renderPage();

    expect(
      screen.getByRole("heading", { name: "Kalender" }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("parent-calendar-skeleton")).toBeInTheDocument();
    expect(screen.getByTestId("parent-page-section-skeleton")).toBeVisible();
  });

  it("lädt anstehende Termine für das erlaubte 92-Tage-Fenster", async () => {
    renderPage();

    await waitFor(() => expect(mockedCalendar).toHaveBeenCalledTimes(1));
    const [from, to] = mockedCalendar.mock.calls[0]!;
    expect(toISODate(from)).toBe("2026-08-17");
    expect(toISODate(to)).toBe("2026-11-16");
  });

  it("gruppiert Termine chronologisch in diese Woche, nächste Woche und später", async () => {
    mockedCalendar.mockResolvedValue(
      calendarResponse([
        event({
          id: "later",
          title: "Herbstfest",
          start_date: "2026-09-15",
          end_date: "2026-09-15",
        }),
        event({
          id: "next",
          title: "Ausflug",
          start_date: "2026-08-24",
          end_date: "2026-08-24",
          start_time: "08:30",
          end_time: "14:00",
        }),
        event(),
      ]),
    );

    renderPage();

    const thisWeek = await screen.findByRole("region", {
      name: "Diese Woche",
    });
    const nextWeek = screen.getByRole("region", { name: "Nächste Woche" });
    const later = screen.getByRole("region", { name: "Später" });
    expect(within(thisWeek).getByText("Elternabend")).toBeInTheDocument();
    expect(within(nextWeek).getByText("Ausflug")).toBeInTheDocument();
    expect(within(later).getByText("Herbstfest")).toBeInTheDocument();
    expect(
      screen.getByTestId("parent-calendar-month-grid").parentElement,
    ).toHaveClass("hidden", "lg:block");
  });

  it("zeigt das Monatsraster nur als Desktop-Zusatz", async () => {
    mockedCalendar.mockResolvedValue(calendarResponse([event()]));

    renderPage();

    expect(
      await screen.findByRole("heading", { name: "Diese Woche" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "August 2026" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "August 2026" })).toHaveClass(
      "text-center",
    );
    expect(
      screen.queryByRole("button", { name: "Heute" }),
    ).not.toBeInTheDocument();
    const monthGrid = screen.getByTestId("parent-calendar-month-grid");
    expect(monthGrid).toHaveClass("h-full");
    expect(monthGrid.parentElement).toHaveClass("hidden", "h-full", "lg:block");
    expect(monthGrid.parentElement?.parentElement).toHaveClass(
      "items-stretch",
      "lg:grid-cols-[minmax(20rem,26rem)_minmax(0,1fr)]",
    );
    expect(
      screen.getByRole("region", { name: "Diese Woche" }).parentElement,
    ).toHaveClass("h-full");
    expect(
      screen.queryByText("An diesem Tag steht nichts an."),
    ).not.toBeInTheDocument();
  });

  // Die Grenze ist der Monat, in den das Ladefenster endet — hier November
  // (Fenster 17.08. bis 16.11.). Der Teilmonat gehoert dazu, weil seine
  // Termine links in der Liste stehen; welche Tage darin nicht geladen sind,
  // zeigt das Raster pro Tag.
  it("begrenzt das Monatsraster auf das geladene Fenster", async () => {
    mockedCalendar.mockResolvedValue(calendarResponse([event()]));
    renderPage();

    await screen.findByRole("heading", { name: "August 2026" });
    expect(
      screen.getByRole("button", { name: "Vorheriger Monat" }),
    ).toBeDisabled();
    expect(
      screen.queryByRole("button", { name: "16.08.2026" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Nächster Monat" }));
    fireEvent.click(screen.getByRole("button", { name: "Nächster Monat" }));

    expect(
      screen.getByRole("heading", { name: "Oktober 2026" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Nächster Monat" }),
    ).not.toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Nächster Monat" }));

    expect(
      screen.getByRole("heading", { name: "November 2026" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Nächster Monat" }),
    ).toBeDisabled();
  });

  // Ein Termin im angebrochenen Schlussmonat ist auch per Deep-Link
  // erreichbar: er liegt im Ladefenster, also darf die Seite dorthin springen.
  it("öffnet einen URL-Fokus im angebrochenen Schlussmonat", async () => {
    searchParams = new URLSearchParams("date=2026-11-05");
    mockedCalendar.mockResolvedValue(calendarResponse([event()]));

    renderPage();

    expect(
      await screen.findByRole("heading", { name: "November 2026" }),
    ).toBeInTheDocument();
  });

  it("fällt bei einem URL-Fokus außerhalb des Ladefensters auf heute zurück", async () => {
    searchParams = new URLSearchParams("date=2027-08-01");

    renderPage();

    expect(
      await screen.findByRole("heading", { name: "August 2026" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("August 2027")).not.toBeInTheDocument();
  });

  it("öffnet dieselben verankerten Details aus Liste und Monatsraster", async () => {
    mockedCalendar.mockResolvedValue(calendarResponse([event()]));
    renderPage();

    const row = await screen.findByRole("article", { name: "Elternabend" });
    const listTrigger = within(row).getByRole("button", {
      name: /Elternabend, 18:00/,
    });
    fireEvent.click(listTrigger);

    const dialog = screen.getByRole("dialog", { name: "Elternabend" });
    expect(dialog).toHaveStyle({ position: "fixed", width: "380px" });
    expect(within(dialog).getByText("Termin")).toBeInTheDocument();
    expect(
      within(dialog).getByRole("heading", { name: "Elternabend" }),
    ).toBeInTheDocument();
    const childRow = within(dialog).getByText("Für Hannah Klein");
    expect(
      childRow.parentElement?.querySelector(
        'svg[data-moto-duotone-tone="greenVivid"]',
      ),
    ).toBeInTheDocument();
    expect(within(dialog).getByText("Aula")).toBeInTheDocument();

    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(listTrigger).toHaveFocus();

    const monthTrigger = within(
      screen.getByTestId("parent-calendar-month-grid"),
    ).getByRole("button", { name: /Elternabend, 18:00/ });
    fireEvent.click(monthTrigger);
    expect(
      screen.getByRole("dialog", { name: "Elternabend" }),
    ).toBeInTheDocument();
  });

  it("öffnet Termindetails unterhalb von lg als Bottom Drawer", async () => {
    vi.spyOn(window, "matchMedia").mockImplementation(
      (query) =>
        ({
          matches: query === BELOW_LG,
          media: query,
          onchange: null,
          addEventListener: vi.fn(),
          removeEventListener: vi.fn(),
          addListener: vi.fn(),
          removeListener: vi.fn(),
          dispatchEvent: vi.fn(),
        }) as MediaQueryList,
    );
    mockedCalendar.mockResolvedValue(calendarResponse([event()]));
    renderPage();

    const row = await screen.findByRole("article", { name: "Elternabend" });
    await waitFor(() => expect(window.matchMedia).toHaveBeenCalled());
    fireEvent.click(
      within(row).getByRole("button", { name: /Elternabend, 18:00/ }),
    );

    expect(
      await screen.findByTestId("mobile-calendar-drawer"),
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("heading", { name: "Elternabend" }).length,
    ).toBeGreaterThan(0);
  });

  it("nimmt eine offene Einladung direkt im Monats-Popover an", async () => {
    mockedCalendar.mockResolvedValue(
      calendarResponse([
        event({
          can_respond: true,
          response_status: "pending",
          recipient_id: "77",
        }),
      ]),
    );
    renderPage();

    const monthGrid = await screen.findByTestId("parent-calendar-month-grid");
    fireEvent.click(
      within(monthGrid).getByRole("button", {
        name: /Elternabend, 18:00 - 20:00, Antwort erforderlich/,
      }),
    );
    const dialog = screen.getByRole("dialog", { name: "Elternabend" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Zusagen" }));

    await waitFor(() =>
      expect(mockedRespond).toHaveBeenCalledWith("77", "accepted", undefined),
    );
    expect(within(dialog).getByText("Zugesagt")).toBeInTheDocument();
    expect(
      within(dialog).queryByRole("button", { name: "Zusagen" }),
    ).not.toBeInTheDocument();
    // #6987: die Antwort bleibt änderbar.
    expect(
      within(dialog).getByRole("button", { name: "Antwort ändern" }),
    ).toBeInTheDocument();
    expect(toastSuccess).toHaveBeenCalledWith("Termin zugesagt.");
  });

  // #6846: Eltern sehen nur Zahlen, keine Namen anderer Eltern; bei einer
  // Reihe steht, dass die Antwort für alle Termine gilt.
  it("zeigt die Rückmeldungen als Zahlen und sagt, dass die Antwort für die Reihe gilt", async () => {
    mockedCalendar.mockResolvedValue(
      calendarResponse([
        event({
          can_respond: true,
          can_view_overview: true,
          delivery_mode: "rsvp_required",
          recurring: true,
          occurrence_date: "2026-08-19",
          response_status: "pending",
          series_response_status: "pending",
          recipient_id: "77",
        }),
      ]),
    );
    mockedOverview.mockResolvedValue({
      appointment_id: "appointment-1",
      delivery_mode: "rsvp_required",
      overview_visibility: "all",
      counts: { accepted: 12, declined: 3, pending: 5 },
      attendees: [],
    });
    renderPage();

    const row = await screen.findByRole("article", { name: "Elternabend" });
    expect(
      within(row).getByText("Antwort für alle Termine der Reihe"),
    ).toBeInTheDocument();

    const monthGrid = screen.getByTestId("parent-calendar-month-grid");
    fireEvent.click(
      within(monthGrid).getByRole("button", { name: /Elternabend, 18:00/ }),
    );
    const dialog = screen.getByRole("dialog", { name: "Elternabend" });
    const responses = await within(dialog).findByRole("region", {
      name: "Rückmeldungen",
    });
    expect(await within(responses).findByText("12")).toBeInTheDocument();
    expect(within(responses).getByText("3")).toBeInTheDocument();
    expect(within(responses).getByText("5")).toBeInTheDocument();
    expect(
      within(responses).getByText("Namen anderer Eltern sehen Sie nicht."),
    ).toBeInTheDocument();
    // #6987: die Zahlen gelten für den geöffneten Termin der Reihe.
    expect(mockedOverview).toHaveBeenCalledWith("appointment-1", "2026-08-19");
    expect(
      within(dialog).getByRole("button", { name: "Alle Termine" }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(
      within(dialog).getByText(
        "Ihre Antwort gilt für alle Termine dieser Reihe.",
      ),
    ).toBeInTheDocument();

    // Die eigene Antwort ändert die Zahlen, also laden sie neu.
    fireEvent.click(within(dialog).getByRole("button", { name: "Zusagen" }));
    await waitFor(() => expect(mockedOverview).toHaveBeenCalledTimes(2));
  });

  // #6846: eine Antwort gilt für die ganze Reihe, also zeigen alle Termine
  // derselben Einladung sie sofort.
  it("übernimmt die Antwort für alle Termine derselben Reihe", async () => {
    const series = {
      can_respond: true,
      recurring: true,
      response_status: "pending",
      recipient_id: "77",
    };
    mockedCalendar.mockResolvedValue(
      calendarResponse([
        event({ ...series, id: "appointment:1:2026-08-19" }),
        event({
          ...series,
          id: "appointment:1:2026-08-26",
          start_date: "2026-08-26",
          end_date: "2026-08-26",
        }),
      ]),
    );
    renderPage();

    const rows = await screen.findAllByRole("article", { name: "Elternabend" });
    expect(rows).toHaveLength(2);
    fireEvent.click(within(rows[0]!).getByRole("button", { name: "Zusagen" }));

    await waitFor(() =>
      expect(mockedRespond).toHaveBeenCalledWith("77", "accepted", undefined),
    );
    for (const row of screen.getAllByRole("article", { name: "Elternabend" })) {
      expect(await within(row).findByText("Zugesagt")).toBeInTheDocument();
      expect(
        within(row).queryByRole("button", { name: "Zusagen" }),
      ).not.toBeInTheDocument();
    }
  });

  // #6987: Eltern sagen der Reihe zu und einem einzelnen Termin ab.
  it("sagt einen einzelnen Termin einer zugesagten Reihe ab", async () => {
    const series = {
      can_respond: true,
      recurring: true,
      response_status: "accepted",
      series_response_status: "accepted",
      recipient_id: "77",
    };
    mockedCalendar.mockResolvedValue(
      calendarResponse([
        event({
          ...series,
          id: "appointment:1:2026-08-19",
          occurrence_date: "2026-08-19",
        }),
        event({
          ...series,
          id: "appointment:1:2026-08-26",
          occurrence_date: "2026-08-26",
          start_date: "2026-08-26",
          end_date: "2026-08-26",
        }),
      ]),
    );
    renderPage();

    const rows = await screen.findAllByRole("article", { name: "Elternabend" });
    fireEvent.click(
      within(rows[1]!).getByRole("button", { name: /Elternabend, 18:00/ }),
    );
    const dialog = screen.getByRole("dialog", { name: "Elternabend" });
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Antwort ändern" }),
    );
    expect(
      within(dialog).getByRole("button", { name: /^Nur am / }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(
      within(dialog).getByText(
        "Für die anderen Termine gilt weiter: Zugesagt.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Absagen" }));

    await waitFor(() =>
      expect(mockedRespond).toHaveBeenCalledWith(
        "77",
        "declined",
        "2026-08-26",
      ),
    );
    expect(toastSuccess).toHaveBeenCalledWith(
      expect.stringMatching(/^Für .*26.* abgesagt\.$/),
    );
    const [first, second] = screen.getAllByRole("article", {
      name: "Elternabend",
    });
    expect(within(first!).getByText("Zugesagt")).toBeInTheDocument();
    expect(within(second!).getByText("Abgesagt")).toBeInTheDocument();
    expect(within(second!).getByText(/Nur dieser Termin/)).toBeInTheDocument();
  });

  it("speichert eine Rückmeldung direkt in der Terminzeile", async () => {
    mockedCalendar.mockResolvedValue(
      calendarResponse([
        event({
          can_respond: true,
          response_status: "pending",
          recipient_id: "77",
        }),
      ]),
    );
    renderPage();

    const row = await screen.findByRole("article", { name: "Elternabend" });
    expect(row).toHaveClass("rounded-l-none", "rounded-r-lg", "border-dashed");
    expect(row.style.backgroundImage).toContain("repeating-linear-gradient");
    const acceptButton = within(row).getByRole("button", { name: "Zusagen" });
    const declineButton = within(row).getByRole("button", { name: "Absagen" });
    expect(acceptButton).toHaveClass("rounded-lg", "px-4", "py-2", "text-sm");
    expect(acceptButton.parentElement).toHaveClass("ms-auto", "flex");
    expect(declineButton).toHaveClass("rounded-lg", "px-4", "py-2", "text-sm");
    const monthEvent = screen.getByRole("button", {
      name: /Elternabend, 18:00 - 20:00, Antwort erforderlich/,
    });
    expect(monthEvent).toHaveClass(
      "!rounded-l-none",
      "rounded-r-md",
      "border-dashed",
    );
    expect(monthEvent.style.backgroundImage).toContain(
      "repeating-linear-gradient",
    );
    fireEvent.click(declineButton);

    await waitFor(() =>
      expect(mockedRespond).toHaveBeenCalledWith("77", "declined", undefined),
    );
    expect(within(row).getByText("Abgesagt")).toBeInTheDocument();
    expect(toastSuccess).toHaveBeenCalledWith("Termin abgesagt.");
  });

  it("zeigt eine vorhandene Antwort ohne Aktionsknöpfe", async () => {
    mockedCalendar.mockResolvedValue(
      calendarResponse([
        event({
          can_respond: true,
          response_status: "accepted",
          recipient_id: "77",
        }),
      ]),
    );
    renderPage();

    const row = await screen.findByRole("article", { name: "Elternabend" });
    expect(within(row).getByText("Zugesagt")).toBeInTheDocument();
    expect(
      within(row).queryByRole("button", { name: "Zusagen" }),
    ).not.toBeInTheDocument();
    expect(
      within(row).queryByRole("button", { name: "Absagen" }),
    ).not.toBeInTheDocument();
  });

  it("zeigt bei leerer Liste einen Leerzustand und danach das Abo", async () => {
    renderPage();

    const empty = await screen.findByText(
      "In den nächsten 3 Monaten stehen keine Termine an.",
    );
    const subscribe = screen.getByTestId("subscribe-panel");
    expect(empty).toBeInTheDocument();
    expect(
      empty.compareDocumentPosition(subscribe) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  // #2518: Ladefehler mit Katalogtext und Wiederholen an der Stelle der
  // Termine, ohne Leerzustand und ohne leeres Monatsraster daneben.
  it("meldet einen Ladefehler in Alltagssprache und lädt erneut", async () => {
    mockedCalendar
      .mockRejectedValueOnce(
        new ApiError("offline", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce(calendarResponse([event()]));
    renderPage();
    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Termine"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/offline/)).not.toBeInTheDocument();
    expect(
      screen.queryByText("In den nächsten 3 Monaten stehen keine Termine an."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("parent-calendar-month-grid"),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(
      await screen.findByRole("article", { name: "Elternabend" }),
    ).toBeInTheDocument();
    expect(mockedCalendar).toHaveBeenCalledTimes(2);
  });

  it("zeigt einen Fehler der Rückmeldung in der Terminzeile als Toast", async () => {
    mockedCalendar.mockResolvedValue(
      calendarResponse([
        event({
          can_respond: true,
          response_status: "pending",
          recipient_id: "77",
        }),
      ]),
    );
    mockedRespond.mockRejectedValueOnce(
      new ApiError("kaputt", 500, { code: "general.server" }),
    );
    renderPage();

    const row = await screen.findByRole("article", { name: "Elternabend" });
    fireEvent.click(within(row).getByRole("button", { name: "Zusagen" }));

    expect(
      await screen.findByText(catalogText("general.server", "die Antwort")),
    ).toBeInTheDocument();
    expect(screen.queryByText(/kaputt/)).not.toBeInTheDocument();
    expect(toastSuccess).not.toHaveBeenCalled();
    // Nothing was stored, so the buttons stay.
    expect(
      within(row).getByRole("button", { name: "Zusagen" }),
    ).toBeInTheDocument();
  });

  it("zeigt einen Fehler der Rückmeldung im offenen Popover und wiederholt", async () => {
    mockedCalendar.mockResolvedValue(
      calendarResponse([
        event({
          can_respond: true,
          response_status: "pending",
          recipient_id: "77",
        }),
      ]),
    );
    mockedRespond.mockRejectedValueOnce(
      new ApiError("weg", 503, { code: "general.unavailable" }),
    );
    renderPage();

    const monthGrid = await screen.findByTestId("parent-calendar-month-grid");
    fireEvent.click(
      within(monthGrid).getByRole("button", {
        name: /Elternabend, 18:00 - 20:00, Antwort erforderlich/,
      }),
    );
    const dialog = screen.getByRole("dialog", { name: "Elternabend" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Zusagen" }));

    expect(
      await within(dialog).findByText(
        catalogText("general.unavailable", "die Antwort"),
      ),
    ).toBeInTheDocument();

    fireEvent.click(
      within(dialog).getByRole("button", { name: "Wiederholen" }),
    );

    await waitFor(() => expect(mockedRespond).toHaveBeenCalledTimes(2));
    expect(await within(dialog).findByText("Zugesagt")).toBeInTheDocument();
  });
});
