import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  getParentCalendar,
  respondParentCalendar,
} from "~/lib/personal-calendar-api";
import { ParentCalendarPage } from "./parent-calendar-page";

const toastSuccess = vi.fn();
const toastError = vi.fn();

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(),
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

// Montag der Fixtur-Woche. "Diese Woche" ist Mo 17.08. bis So 23.08.2026.
const MONDAY = new Date(2026, 7, 17, 9, 0, 0);

function event(overrides: Record<string, unknown>) {
  return {
    id: "e1",
    source: "appointment",
    title: "Elternabend",
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

function renderPage() {
  return render(<ParentCalendarPage />);
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(MONDAY);
  mockedCalendar.mockResolvedValue({
    from: "",
    to: "",
    events: [],
  } as unknown as Awaited<ReturnType<typeof getParentCalendar>>);
  mockedRespond.mockResolvedValue(undefined as never);
});

afterEach(() => {
  vi.useRealTimers();
});

describe("ParentCalendarPage", () => {
  it("gruppiert die Termine nach Diese Woche, Naechste Woche und Spaeter", async () => {
    mockedCalendar.mockResolvedValue({
      from: "",
      to: "",
      events: [
        event({ id: "a", start_date: "2026-08-19", title: "Elternabend" }),
        event({ id: "b", start_date: "2026-08-26", title: "Ausflug" }),
        event({ id: "c", start_date: "2026-10-01", title: "Herbstfest" }),
      ],
    } as unknown as Awaited<ReturnType<typeof getParentCalendar>>);

    renderPage();

    expect(await screen.findByText("Diese Woche")).toBeInTheDocument();
    expect(screen.getByText("Nächste Woche")).toBeInTheDocument();
    expect(screen.getByText("Später")).toBeInTheDocument();
    expect(screen.getByText("Elternabend")).toBeInTheDocument();
    expect(screen.getByText("Ausflug")).toBeInTheDocument();
    expect(screen.getByText("Herbstfest")).toBeInTheDocument();
  });

  it("bietet Zusagen und Absagen in der Zeile und speichert die Antwort", async () => {
    mockedCalendar.mockResolvedValue({
      from: "",
      to: "",
      events: [
        event({
          can_respond: true,
          response_status: "pending",
          recipient_id: "77",
        }),
      ],
    } as unknown as Awaited<ReturnType<typeof getParentCalendar>>);

    renderPage();

    fireEvent.click(await screen.findByRole("button", { name: "Absagen" }));
    await waitFor(() =>
      expect(mockedRespond).toHaveBeenCalledWith("77", "declined"),
    );
    expect(toastSuccess).toHaveBeenCalledWith("Termin abgesagt.");
  });

  it("zeigt eine bereits gegebene Antwort als Zeile statt als Schaltflaechen", async () => {
    mockedCalendar.mockResolvedValue({
      from: "",
      to: "",
      events: [
        event({
          can_respond: true,
          response_status: "accepted",
          recipient_id: "77",
        }),
      ],
    } as unknown as Awaited<ReturnType<typeof getParentCalendar>>);

    renderPage();

    expect(await screen.findByText("Zugesagt")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Zusagen" }),
    ).not.toBeInTheDocument();
  });

  it("gibt Terminen ohne Rueckmeldebedarf keine Schaltflaechen", async () => {
    mockedCalendar.mockResolvedValue({
      from: "",
      to: "",
      events: [event({ source: "timetable", title: "Betreuung Gruppe A" })],
    } as unknown as Awaited<ReturnType<typeof getParentCalendar>>);

    renderPage();

    expect(await screen.findByText("Betreuung Gruppe A")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Zusagen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Absagen" }),
    ).not.toBeInTheDocument();
  });

  it("sagt es, wenn nichts ansteht, und haelt das Abo am Ende bereit", async () => {
    renderPage();
    expect(
      await screen.findByText(
        "In den nächsten drei Monaten steht kein Termin an.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByTestId("subscribe-panel")).toBeInTheDocument();
  });

  it("meldet einen Ladefehler in Alltagssprache", async () => {
    mockedCalendar.mockRejectedValue(new Error("offline"));
    renderPage();
    expect(
      await screen.findByText(
        /Die Termine konnten gerade nicht geladen werden/,
      ),
    ).toBeInTheDocument();
  });
});
