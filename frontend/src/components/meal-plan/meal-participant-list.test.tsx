import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  getDailyMealParticipants: vi.fn(),
  downloadDailyMealParticipants: vi.fn(),
  toastError: vi.fn(),
  today: "2026-09-07",
}));

vi.mock("~/lib/meal-plan-api", () => ({
  getDailyMealParticipants: mocks.getDailyMealParticipants,
  downloadDailyMealParticipants: mocks.downloadDailyMealParticipants,
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => mocks.today,
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ error: mocks.toastError }),
}));

import { MealParticipantList } from "./meal-participant-list";

describe("MealParticipantList", () => {
  beforeEach(() => {
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

    expect(
      screen.getByRole("button", { name: "Samstag, 12. September 2026" }),
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
      screen.getByRole("button", { name: "Mittwoch, 9. September 2026" }),
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
      screen.getByRole("button", { name: "Dienstag, 8. September 2026" }),
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

  it("retries after a temporary load error", async () => {
    mocks.getDailyMealParticipants
      .mockRejectedValueOnce(new Error("temporary failure"))
      .mockResolvedValueOnce({
        date: "2026-09-07",
        cutoffTime: "09:00",
        participants: [],
      });

    render(<MealParticipantList />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Erneut versuchen" }),
    );
    await waitFor(() => {
      expect(mocks.getDailyMealParticipants).toHaveBeenCalledTimes(2);
    });
    expect(
      (await screen.findAllByText("Keine Anmeldungen für diesen Tag")).length,
    ).toBeGreaterThan(0);
  });
});
