import {
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { setTestClock } from "~/test/clock";
import { preloadDayPicker } from "~/components/ui/lazy-day-picker";
import "@testing-library/jest-dom/vitest";
import { NextIntlClientProvider } from "next-intl";
import deMessages from "~/i18n/messages/de.json";

// Real next-intl: the cutoff texts use ICU plurals the global mock cannot
// render.
vi.unmock("next-intl");

const mocks = vi.hoisted(() => ({
  getDailyMealParticipants: vi.fn(),
  downloadDailyMealParticipants: vi.fn(),
  getWeeklyMealParticipants: vi.fn(),
  downloadWeeklyMealParticipants: vi.fn(),
  today: "2026-09-07",
}));

vi.mock("~/lib/meal-plan-api", async (importOriginal) => ({
  mondayOf: (await importOriginal<typeof import("~/lib/meal-plan-api")>())
    .mondayOf,
  getDailyMealParticipants: mocks.getDailyMealParticipants,
  downloadDailyMealParticipants: mocks.downloadDailyMealParticipants,
  getWeeklyMealParticipants: mocks.getWeeklyMealParticipants,
  downloadWeeklyMealParticipants: mocks.downloadWeeklyMealParticipants,
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => mocks.today,
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { MealParticipantList } from "./meal-participant-list";

function render(ui: ReactElement) {
  return rtlRender(ui, {
    wrapper: ({ children }) => (
      <NextIntlClientProvider
        locale="de"
        messages={deMessages}
        timeZone="Europe/Berlin"
      >
        <ToastProvider>{children}</ToastProvider>
      </NextIntlClientProvider>
    ),
  });
}

// The kit loads the calendar grid lazily; load it once up front so a cold
// import on a busy machine does not race the findBy timeout.
beforeAll(() => preloadDayPicker());

describe("MealParticipantList", () => {
  beforeEach(() => {
    // Keep the calendar's "today" label independent of the actual CI date.
    setTestClock(new Date("2026-09-07T12:00:00+02:00"));
    vi.clearAllMocks();
    mocks.today = "2026-09-07";
    mocks.getDailyMealParticipants.mockResolvedValue({
      date: "2026-09-07",
      cutoffTime: "09:00",
      participants: [
        {
          studentId: "42",
          firstName: "Mia",
          lastName: "Muster",
          schoolClass: "2a",
        },
      ],
    });
  });

  it("starts on Monday and disables weekends when opened on a weekend", async () => {
    mocks.today = "2026-09-05";

    render(<MealParticipantList />);

    await waitFor(() => {
      expect(mocks.getDailyMealParticipants).toHaveBeenCalledWith("2026-09-07");
    });
    const datePicker = screen.getByRole("button", {
      name: "Datum: 07.09.2026",
    });
    fireEvent.click(datePicker);

    // The kit loads the calendar grid lazily.
    expect(
      await screen.findByRole("button", {
        name: "Samstag, 12. September 2026",
      }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Sonntag, 13. September 2026" }),
    ).toBeDisabled();
  });

  it("advances the default list when the Berlin date changes", async () => {
    const { rerender } = render(<MealParticipantList />);

    await waitFor(() => {
      expect(mocks.getDailyMealParticipants).toHaveBeenCalledWith("2026-09-07");
    });

    mocks.today = "2026-09-08";
    rerender(<MealParticipantList />);

    await waitFor(() => {
      expect(mocks.getDailyMealParticipants).toHaveBeenCalledWith("2026-09-08");
    });
    expect(
      screen.getByRole("button", { name: "Datum: 08.09.2026" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Mittagessen am 08.09.2026")).toBeInTheDocument();
  });

  it("keeps a manually selected list across a Berlin date change", async () => {
    const { rerender } = render(<MealParticipantList />);

    await screen.findByText("Mittagessen am 07.09.2026");
    fireEvent.click(screen.getByRole("button", { name: "Datum: 07.09.2026" }));
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Mittwoch, 9. September 2026",
      }),
    );
    await waitFor(() => {
      expect(mocks.getDailyMealParticipants).toHaveBeenCalledWith("2026-09-09");
    });

    mocks.today = "2026-09-08";
    rerender(<MealParticipantList />);

    expect(
      screen.getByRole("button", { name: "Datum: 09.09.2026" }),
    ).toBeInTheDocument();
    expect(mocks.getDailyMealParticipants).not.toHaveBeenCalledWith(
      "2026-09-08",
    );
  });

  it("shows the kitchen cutoff and registered children", async () => {
    render(<MealParticipantList />);

    expect(
      await screen.findByText(
        "Änderungen für diesen Tag sind bis 09:00 Uhr möglich.",
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Muster, Mia").length).toBeGreaterThan(0);
    expect(screen.getAllByText("2a").length).toBeGreaterThan(0);
    expect(screen.getByText("Mittagessen am 07.09.2026")).toBeInTheDocument();
    expect(
      screen.queryByText("Mittagessen am 2026-09-07"),
    ).not.toBeInTheDocument();
    expect(mocks.getDailyMealParticipants).toHaveBeenCalledWith("2026-09-07");
    const datePicker = screen.getByRole("button", {
      name: "Datum: 07.09.2026",
    });
    expect(datePicker).toBeInTheDocument();
    expect(datePicker).toHaveClass("h-10", "min-w-44", "rounded-lg");
    expect(
      document.querySelector('input[type="date"]'),
    ).not.toBeInTheDocument();

    fireEvent.click(datePicker);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Dienstag, 8. September 2026",
      }),
    );
    await waitFor(() => {
      expect(mocks.getDailyMealParticipants).toHaveBeenCalledWith("2026-09-08");
    });
    expect(
      screen.getByRole("button", { name: "Datum: 08.09.2026" }),
    ).toBeInTheDocument();
  });

  it("groups the established PDF and Excel export actions", async () => {
    let finishPdf: () => void = () => undefined;
    mocks.downloadDailyMealParticipants
      .mockReturnValueOnce(
        new Promise<void>((resolve) => {
          finishPdf = resolve;
        }),
      )
      .mockResolvedValueOnce(undefined);
    render(<MealParticipantList />);

    await screen.findByText("Mittagessen am 07.09.2026");
    expect(
      screen.getByRole("group", { name: "Tagesliste herunterladen" }),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", {
        name: "Tagesliste als PDF herunterladen",
      }),
    );
    expect(
      screen.getByRole("button", {
        name: "Tagesliste als PDF herunterladen",
      }),
    ).toHaveAttribute("aria-busy", "true");
    expect(screen.getByRole("status")).toHaveTextContent(
      "PDF wird heruntergeladen.",
    );
    await waitFor(() => {
      expect(mocks.downloadDailyMealParticipants).toHaveBeenCalledWith(
        "2026-09-07",
        "pdf",
      );
    });
    finishPdf();
    await screen.findByRole("button", {
      name: "Tagesliste als PDF herunterladen",
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Tagesliste als Excel-Datei herunterladen",
      }),
    );
    await waitFor(() => {
      expect(mocks.downloadDailyMealParticipants).toHaveBeenCalledWith(
        "2026-09-07",
        "xlsx",
      );
    });
  });

  it("shows the catalog text with retry when an export fails", async () => {
    mocks.downloadDailyMealParticipants
      .mockRejectedValueOnce(
        new ApiError("boom", 500, { code: "general.server" }),
      )
      .mockResolvedValueOnce(undefined);
    render(<MealParticipantList />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Tagesliste als PDF herunterladen",
      }),
    );

    expect(
      await screen.findByText(catalogText("general.server", "die Tagesliste")),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => {
      expect(mocks.downloadDailyMealParticipants).toHaveBeenCalledTimes(2);
    });
    expect(mocks.downloadDailyMealParticipants).toHaveBeenLastCalledWith(
      "2026-09-07",
      "pdf",
    );
  });

  it("retries after a temporary load error", async () => {
    mocks.getDailyMealParticipants
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce({
        date: "2026-09-07",
        cutoffTime: "09:00",
        participants: [],
      });

    render(<MealParticipantList />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Tagesliste"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => {
      expect(mocks.getDailyMealParticipants).toHaveBeenCalledTimes(2);
    });
    expect(
      (await screen.findAllByText("Keine Anmeldungen für diesen Tag")).length,
    ).toBeGreaterThan(0);
  });

  it("names the lead days of the cutoff on the daily list", async () => {
    mocks.getDailyMealParticipants.mockResolvedValue({
      date: "2026-09-07",
      cutoffTime: "09:00",
      cutoffLeadDays: 2,
      participants: [],
    });
    render(<MealParticipantList />);

    expect(
      await screen.findByText(
        "Änderungen für diesen Tag sind bis 09:00 Uhr 2 Werktage vorher möglich.",
      ),
    ).toBeInTheDocument();
  });

  it("shows the week with a mark per day and exports it", async () => {
    mocks.getWeeklyMealParticipants.mockResolvedValue({
      weekStart: "2026-09-07",
      cutoffTime: "09:00",
      cutoffLeadDays: 1,
      days: [
        {
          date: "2026-09-07",
          participants: [
            {
              studentId: "42",
              firstName: "Mia",
              lastName: "Muster",
              schoolClass: "2a",
            },
            {
              studentId: "43",
              firstName: "Ben",
              lastName: "Bauer",
              schoolClass: "1b",
            },
          ],
        },
        { date: "2026-09-08", participants: [] },
        {
          date: "2026-09-09",
          participants: [
            {
              studentId: "42",
              firstName: "Mia",
              lastName: "Muster",
              schoolClass: "2a",
            },
          ],
        },
        { date: "2026-09-10", participants: [] },
        { date: "2026-09-11", participants: [] },
      ],
    });
    mocks.downloadWeeklyMealParticipants.mockResolvedValue(undefined);
    render(<MealParticipantList />);
    await screen.findByText("Mittagessen am 07.09.2026");

    fireEvent.click(screen.getByRole("button", { name: "Woche" }));

    expect(
      await screen.findByText("Mittagessen vom 07.09.2026 bis 11.09.2026"),
    ).toBeInTheDocument();
    expect(mocks.getWeeklyMealParticipants).toHaveBeenCalledWith("2026-09-07");
    expect(
      screen.getByText(
        "Änderungen sind bis 09:00 Uhr am Werktag davor möglich.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Wochenliste für die Küche" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Essen pro Tag:").parentElement).toHaveTextContent(
      "Essen pro Tag: Mo 2 Kinder · Di 0 Kinder · Mi 1 Kind · Do 0 Kinder · Fr 0 Kinder",
    );
    expect(screen.getAllByText("Muster, Mia").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Bauer, Ben").length).toBeGreaterThan(0);
    // Mia on Mon and Wed, Ben on Mon: three marks in all.
    expect(
      within(screen.getByRole("table")).getAllByText("angemeldet"),
    ).toHaveLength(3);

    fireEvent.click(
      screen.getByRole("button", {
        name: "Wochenliste als Excel-Datei herunterladen",
      }),
    );
    await waitFor(() => {
      expect(mocks.downloadWeeklyMealParticipants).toHaveBeenCalledWith(
        "2026-09-07",
        "xlsx",
      );
    });
    expect(mocks.downloadDailyMealParticipants).not.toHaveBeenCalled();
  });
});
