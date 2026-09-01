import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  getDailyMealParticipants: vi.fn(),
  downloadDailyMealParticipants: vi.fn(),
  toastError: vi.fn(),
}));

vi.mock("~/lib/meal-plan-api", () => ({
  getDailyMealParticipants: mocks.getDailyMealParticipants,
  downloadDailyMealParticipants: mocks.downloadDailyMealParticipants,
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => "2026-09-07",
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ error: mocks.toastError }),
}));

import { MealParticipantList } from "./meal-participant-list";

describe("MealParticipantList", () => {
  beforeEach(() => {
    vi.clearAllMocks();
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

  it("shows the kitchen cutoff and registered children", async () => {
    render(<MealParticipantList />);

    expect(
      await screen.findByText(
        "Änderungen für diesen Tag sind bis 09:00 Uhr möglich.",
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Muster, Mia").length).toBeGreaterThan(0);
    expect(screen.getAllByText("2a").length).toBeGreaterThan(0);
    expect(mocks.getDailyMealParticipants).toHaveBeenCalledWith("2026-09-07");
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
