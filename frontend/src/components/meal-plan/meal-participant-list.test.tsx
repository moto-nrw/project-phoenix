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

const mocks = vi.hoisted(() => ({
  getDailyMealParticipants: vi.fn(),
  downloadDailyMealParticipants: vi.fn(),
  getChildMealSchedule: vi.fn(),
  setChildMealDay: vi.fn(),
  replaceChildMealSchedule: vi.fn(),
  today: "2026-09-07",
}));

vi.mock("~/lib/meal-plan-api", () => ({
  getDailyMealParticipants: mocks.getDailyMealParticipants,
  downloadDailyMealParticipants: mocks.downloadDailyMealParticipants,
  getChildMealSchedule: mocks.getChildMealSchedule,
  setChildMealDay: mocks.setChildMealDay,
  replaceChildMealSchedule: mocks.replaceChildMealSchedule,
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => mocks.today,
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { MealParticipantList } from "./meal-participant-list";

function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
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
      deadline: "2026-09-04T07:00:00Z",
      closed: false,
      participants: [
        {
          studentId: "42",
          firstName: "Mia",
          lastName: "Muster",
          schoolClass: "2a",
        },
      ],
      others: [
        {
          studentId: "43",
          firstName: "Ben",
          lastName: "Bauer",
          schoolClass: "3b",
        },
      ],
    });
    mocks.getChildMealSchedule.mockResolvedValue({
      weekdays: [1, 3],
      effectiveFrom: "2026-09-01",
    });
    mocks.setChildMealDay.mockResolvedValue(undefined);
    mocks.replaceChildMealSchedule.mockResolvedValue(undefined);
  });

  it("starts on Monday and disables weekends when opened on a weekend", async () => {
    mocks.today = "2026-09-05";

    render(<MealParticipantList />);

    await waitFor(() => {
      expect(mocks.getDailyMealParticipants).toHaveBeenCalledWith(
        "2026-09-07",
        true,
      );
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
      expect(mocks.getDailyMealParticipants).toHaveBeenCalledWith(
        "2026-09-07",
        true,
      );
    });

    mocks.today = "2026-09-08";
    rerender(<MealParticipantList />);

    await waitFor(() => {
      expect(mocks.getDailyMealParticipants).toHaveBeenCalledWith(
        "2026-09-08",
        true,
      );
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
      expect(mocks.getDailyMealParticipants).toHaveBeenCalledWith(
        "2026-09-09",
        true,
      );
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
      // #3873: the deadline of the day, here the Friday before.
      await screen.findByText(
        "Bestellfrist für diesen Tag: Fr., 04.09., 09:00.",
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Muster, Mia").length).toBeGreaterThan(0);
    expect(screen.getAllByText("2a").length).toBeGreaterThan(0);
    expect(screen.getByText("Mittagessen am 07.09.2026")).toBeInTheDocument();
    expect(
      screen.queryByText("Mittagessen am 2026-09-07"),
    ).not.toBeInTheDocument();
    expect(mocks.getDailyMealParticipants).toHaveBeenCalledWith(
      "2026-09-07",
      true,
    );
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
      expect(mocks.getDailyMealParticipants).toHaveBeenCalledWith(
        "2026-09-08",
        true,
      );
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
        deadline: "2026-09-07T07:00:00Z",
        closed: false,
        participants: [],
        others: [],
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

  // #3873: the school changes a registration, also after the deadline.
  describe("Anmeldung ändern", () => {
    it("registers a child for the day after the deadline", async () => {
      render(<MealParticipantList />);

      fireEvent.click(
        await screen.findByRole("button", { name: "Anmeldung ändern" }),
      );
      const dialog = await screen.findByRole("dialog");
      fireEvent.click(within(dialog).getByRole("combobox", { name: "Kind" }));
      fireEvent.click(
        await screen.findByRole("option", {
          name: "Bauer, Ben · 3b · Isst nicht mit",
        }),
      );
      await waitFor(() =>
        expect(mocks.getChildMealSchedule).toHaveBeenCalledWith(
          "43",
          "2026-09-07",
        ),
      );
      expect(
        within(dialog).getByRole("button", { name: "Isst mit" }),
      ).toHaveAttribute("aria-pressed", "true");
      fireEvent.click(
        within(dialog).getByRole("button", { name: "Speichern" }),
      );

      await waitFor(() =>
        expect(mocks.setChildMealDay).toHaveBeenCalledWith(
          "43",
          "2026-09-07",
          true,
        ),
      );
      await waitFor(() =>
        expect(mocks.getDailyMealParticipants).toHaveBeenCalledTimes(2),
      );
    });

    it("sets a child's regular days from a date on", async () => {
      render(<MealParticipantList />);

      fireEvent.click(
        await screen.findByRole("button", { name: "Anmeldung ändern" }),
      );
      const dialog = await screen.findByRole("dialog");
      fireEvent.click(within(dialog).getByRole("combobox", { name: "Kind" }));
      fireEvent.click(
        await screen.findByRole("option", {
          name: "Muster, Mia · 2a · Isst mit",
        }),
      );
      fireEvent.click(
        within(dialog).getByRole("button", { name: "Feste Tage" }),
      );
      await waitFor(() =>
        expect(
          within(dialog).getByRole("checkbox", { name: "Montag" }),
        ).toBeChecked(),
      );
      fireEvent.click(
        within(dialog).getByRole("checkbox", { name: "Freitag" }),
      );
      fireEvent.click(
        within(dialog).getByRole("button", { name: "Speichern" }),
      );

      await waitFor(() =>
        expect(mocks.replaceChildMealSchedule).toHaveBeenCalledWith(
          "42",
          [1, 3, 5],
          "2026-09-07",
        ),
      );
    });

    it("offers no change on a closed day and says why", async () => {
      mocks.getDailyMealParticipants.mockResolvedValue({
        date: "2026-09-07",
        cutoffTime: "09:00",
        deadline: "2026-09-07T07:00:00Z",
        closed: true,
        participants: [],
        others: [],
      });

      render(<MealParticipantList />);

      expect(
        await screen.findByText(
          "An diesem Tag ist die OGS geschlossen. Es gibt kein Essen.",
        ),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Anmeldung ändern" }),
      ).toBeDisabled();
    });
  });
});
