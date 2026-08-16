import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  getParentCalendar,
  respondParentCalendar,
} from "~/lib/personal-calendar-api";
import { toISODate } from "~/lib/date-helpers";
import { ParentCalendarPage } from "./parent-calendar-page";

const toastSuccess = vi.fn();
const toastError = vi.fn();
let searchParams = new URLSearchParams();

vi.mock("next/navigation", () => ({
  useSearchParams: () => searchParams,
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: toastSuccess, error: toastError }),
}));

vi.mock("~/components/calendar/calendar-subscribe-panel", () => ({
  CalendarSubscribePanel: () => <div data-testid="subscribe-panel" />,
}));

vi.mock("~/lib/personal-calendar-api", () => ({
  getParentCalendar: vi.fn(),
  respondParentCalendar: vi.fn(),
}));

const mockedCalendar = vi.mocked(getParentCalendar);
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
  return render(<ParentCalendarPage />);
}

beforeEach(() => {
  vi.clearAllMocks();
  searchParams = new URLSearchParams();
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(MONDAY);
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
    expect(
      screen.getAllByTestId("parent-page-section-row-skeleton"),
    ).toHaveLength(2);
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

  it("begrenzt das Monatsraster auf vollständig geladene Monate", async () => {
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
    ).toBeDisabled();
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
    expect(within(dialog).getByText("Für Hannah Klein")).toBeInTheDocument();
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
          matches: query === "(max-width: 1023px)",
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
      expect(mockedRespond).toHaveBeenCalledWith("77", "accepted"),
    );
    expect(within(dialog).getByText("Zugesagt")).toBeInTheDocument();
    expect(
      within(dialog).queryByRole("button", { name: "Zusagen" }),
    ).not.toBeInTheDocument();
    expect(toastSuccess).toHaveBeenCalledWith("Termin zugesagt.");
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
      expect(mockedRespond).toHaveBeenCalledWith("77", "declined"),
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

  it("meldet einen Ladefehler in Alltagssprache", async () => {
    mockedCalendar.mockRejectedValue(new Error("offline"));
    renderPage();
    expect(
      await screen.findByText(
        /Die Termine konnten gerade nicht geladen werden/,
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("In den nächsten 3 Monaten stehen keine Termine an."),
    ).not.toBeInTheDocument();
  });
});
