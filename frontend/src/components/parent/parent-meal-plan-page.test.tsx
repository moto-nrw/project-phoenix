import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  getChildFeatures: vi.fn(),
  getChildMealPlan: vi.fn(),
  getMealParticipation: vi.fn(),
  replaceMealParticipationSchedule: vi.fn(),
  setMealParticipationDay: vi.fn(),
  clearMealParticipationDay: vi.fn(),
  listMyChildren: vi.fn(),
  today: "2026-08-12",
}));

vi.mock("~/lib/parent-api", () => ({
  getChildFeatures: mocks.getChildFeatures,
  getChildMealPlan: mocks.getChildMealPlan,
  getMealParticipation: mocks.getMealParticipation,
  replaceMealParticipationSchedule: mocks.replaceMealParticipationSchedule,
  setMealParticipationDay: mocks.setMealParticipationDay,
  clearMealParticipationDay: mocks.clearMealParticipationDay,
  listMyChildren: mocks.listMyChildren,
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => mocks.today,
}));

import { ParentMealPlanPage } from "./parent-meal-plan-page";

describe("ParentMealPlanPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.today = "2026-08-12";
    mocks.listMyChildren.mockResolvedValue([
      {
        student_id: "child-1",
        tenant_id: "school-1",
        first_name: "Mia",
        last_name: "Muster",
        school_name: "OGS Am Berg",
      },
    ]);
    mocks.getChildFeatures.mockResolvedValue({
      meal_plan_enabled: true,
      meal_registration_enabled: false,
    });
    mocks.getChildMealPlan.mockResolvedValue([]);
  });

  it("keeps the final page geometry while the meal plan is loading", () => {
    mocks.listMyChildren.mockReturnValue(new Promise(() => {}));

    render(<ParentMealPlanPage />);

    expect(
      screen.getByRole("heading", { name: "Mittagessen", level: 1 }),
    ).toBeInTheDocument();
    const loadingStatus = screen.getByRole("status", {
      name: "Essensplan wird geladen",
    });
    const skeleton = screen.getByTestId("meal-plan-week-skeleton");
    const desktopGrid = skeleton.querySelector(".grid-cols-5");
    const animatedParts = skeleton.querySelectorAll(".animate-pulse");

    expect(loadingStatus).toBeInTheDocument();
    expect(desktopGrid?.children).toHaveLength(5);
    expect(animatedParts.length).toBeGreaterThan(10);
    for (const part of animatedParts) {
      expect(part).toHaveClass("motion-reduce:animate-none");
    }
  });

  it("navigates between calendar weeks directly above the plan", async () => {
    render(<ParentMealPlanPage />);

    expect(
      await screen.findByRole("heading", { name: "Mittagessen", level: 1 }),
    ).toBeInTheDocument();
    expect(screen.getByText("Essen in der OGS")).toBeInTheDocument();

    const previousWeek = screen.getByRole("button", {
      name: "Vorherige Woche",
    });
    const nextWeek = screen.getByRole("button", { name: "Nächste Woche" });
    const weekStatus = within(
      screen.getByRole("navigation", { name: "Kalenderwoche wechseln" }),
    ).getByRole("status");
    expect(previousWeek).toBeDisabled();
    expect(
      await screen.findByText(
        "Für diese Woche ist noch kein Essensplan eingetragen",
      ),
    ).toBeInTheDocument();
    expect(nextWeek).toBeEnabled();
    expect(weekStatus).toHaveTextContent(/KW 33\s*· Diese Woche/);
    expect(weekStatus).toHaveTextContent("10.08. bis 14.08.2026");

    fireEvent.click(nextWeek);

    await waitFor(() => {
      expect(previousWeek).toBeEnabled();
      expect(nextWeek).toBeDisabled();
      expect(mocks.getChildMealPlan).toHaveBeenLastCalledWith(
        "child-1",
        "2026-08-17",
      );
      expect(weekStatus).toHaveTextContent(/KW 34\s*· Nächste Woche/);
      expect(weekStatus).toHaveTextContent("17.08. bis 21.08.2026");
    });
  });

  it("keeps the week container stable while the next week loads", async () => {
    let resolveNextWeek: (entries: []) => void = () => undefined;
    const nextWeekRequest = new Promise<[]>((resolve) => {
      resolveNextWeek = resolve;
    });
    mocks.getChildMealPlan
      .mockResolvedValueOnce([])
      .mockReturnValueOnce(nextWeekRequest);

    render(<ParentMealPlanPage />);

    expect(
      await screen.findByText(
        "Für diese Woche ist noch kein Essensplan eingetragen",
      ),
    ).toBeInTheDocument();
    const navigation = screen.getByRole("navigation", {
      name: "Kalenderwoche wechseln",
    });
    fireEvent.click(
      within(navigation).getByRole("button", { name: "Nächste Woche" }),
    );

    const skeleton = await screen.findByTestId("meal-plan-week-skeleton");
    expect(navigation.closest("section")).toContainElement(skeleton);

    await act(async () => {
      resolveNextWeek([]);
    });
    await waitFor(() => {
      expect(
        screen.queryByTestId("meal-plan-week-skeleton"),
      ).not.toBeInTheDocument();
    });
  });

  it("uses the ISO calendar week at the turn of the year", async () => {
    mocks.today = "2026-12-30";

    render(<ParentMealPlanPage />);

    expect(
      await screen.findByText(
        "Für diese Woche ist noch kein Essensplan eingetragen",
      ),
    ).toBeInTheDocument();
    expect(
      within(
        screen.getByRole("navigation", { name: "Kalenderwoche wechseln" }),
      ).getByRole("status"),
    ).toHaveTextContent("KW 53");
  });

  it("renders the populated week as the primary content", async () => {
    mocks.getChildMealPlan.mockResolvedValue([
      {
        date: "2026-08-12",
        position: 0,
        dish: "Gemüse-Lasagne",
        note: "mit Salat",
      },
    ]);

    render(<ParentMealPlanPage />);

    const dishes = await screen.findAllByText("Gemüse-Lasagne");
    expect(dishes).not.toHaveLength(0);
    const weekStatus = within(
      screen.getByRole("navigation", { name: "Kalenderwoche wechseln" }),
    ).getByRole("status");
    expect(weekStatus.closest("section")).toContainElement(dishes[0]!);
    expect(screen.getAllByText("mit Salat")).not.toHaveLength(0);
    expect(
      screen.queryByText(
        "Für diese Woche ist noch kein Essensplan eingetragen",
      ),
    ).not.toBeInTheDocument();
  });

  it("combines meals and participation while keeping regular days compact", async () => {
    mocks.listMyChildren.mockResolvedValue([
      {
        student_id: "child-1",
        tenant_id: "school-1",
        first_name: "Mia",
        last_name: "Muster",
        school_name: "OGS Am Berg",
      },
      {
        student_id: "child-2",
        tenant_id: "school-2",
        first_name: "Noah",
        last_name: "Beispiel",
        school_name: "OGS Am Park",
      },
    ]);
    mocks.getChildFeatures.mockResolvedValue({
      meal_plan_enabled: true,
      meal_registration_enabled: true,
    });
    mocks.getChildMealPlan.mockResolvedValue([
      {
        date: "2026-08-12",
        position: 0,
        dish: "Gemüse-Lasagne",
        note: "mit Salat",
      },
    ]);
    mocks.getMealParticipation.mockResolvedValue({
      weekdays: [1, 3],
      effective_from: "2026-08-10",
      cutoff_time: "09:00",
      days: [
        {
          date: "2026-08-12",
          participating: false,
          source: "none",
          changeable: true,
        },
        {
          date: "2026-08-14",
          participating: true,
          source: "override",
          changeable: true,
        },
      ],
    });
    mocks.replaceMealParticipationSchedule.mockResolvedValue({
      effective_from: "2026-08-17",
    });
    mocks.setMealParticipationDay.mockResolvedValue(undefined);
    mocks.clearMealParticipationDay.mockResolvedValue(undefined);

    render(<ParentMealPlanPage />);

    expect(
      await screen.findByRole("heading", {
        name: "Wann isst Mia Muster mit?",
      }),
    ).toBeInTheDocument();
    const childSelect = screen.getByRole("combobox", { name: "Kind" });
    expect(
      await screen.findByText(
        "Sie können die Anmeldung am selben Tag bis 09:00 Uhr ändern.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Montag und Mittwoch")).toBeInTheDocument();
    expect(screen.getByText("Gemüse-Lasagne")).toBeInTheDocument();
    expect(screen.getByText("mit Salat")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Eine bestätigte Krankmeldung bis 09:00 Uhr meldet Ihr Kind vom Essen ab. Danach bleibt die Anmeldung bestehen.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("parentMealPlan.participationSickness"),
    ).not.toBeInTheDocument();

    expect(
      screen.queryByRole("checkbox", { name: "Dienstag" }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Ändern" }));
    expect(childSelect).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Nächste Woche" }),
    ).toBeDisabled();
    expect(
      screen.getByText(
        "Speichern oder abbrechen, bevor Sie ein anderes Kind oder eine andere Woche wählen.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox", { name: "Dienstag" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => {
      expect(mocks.replaceMealParticipationSchedule).toHaveBeenCalledWith(
        "child-1",
        [1, 2, 3],
      );
    });

    expect(
      screen.queryByRole("button", { name: "Anmelden" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Anmeldungen ändern" }),
    ).not.toBeInTheDocument();

    const editWednesday = screen.getByRole("button", {
      name: "Anmeldung ändern: Mittwoch, 12. August",
    });
    editWednesday.focus();
    fireEvent.click(editWednesday);
    expect(
      screen.getByRole("group", {
        name: "Anmeldung ändern: Mittwoch, 12. August",
      }),
    ).toHaveFocus();
    expect(childSelect).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Nächste Woche" }),
    ).toBeDisabled();
    expect(
      screen.getByText(
        "Speichern oder abbrechen, bevor Sie ein anderes Kind oder eine andere Woche wählen.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Anmelden" }));
    expect(mocks.setMealParticipationDay).not.toHaveBeenCalled();
    expect(mocks.clearMealParticipationDay).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(
      screen.queryByRole("button", { name: "Anmelden" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Nächste Woche" })).toBeEnabled();
    expect(childSelect).toBeEnabled();
    expect(
      screen.getByRole("button", {
        name: "Anmeldung ändern: Mittwoch, 12. August",
      }),
    ).toHaveFocus();
    expect(mocks.setMealParticipationDay).not.toHaveBeenCalled();
    expect(mocks.clearMealParticipationDay).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Anmeldung ändern: Mittwoch, 12. August",
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Anmelden" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => {
      expect(mocks.setMealParticipationDay).toHaveBeenCalledWith(
        "child-1",
        "2026-08-12",
        true,
      );
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Anmeldung ändern: Freitag, 14. August",
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Feste Anmeldung verwenden" }),
    );
    expect(mocks.clearMealParticipationDay).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => {
      expect(mocks.clearMealParticipationDay).toHaveBeenCalledWith(
        "child-1",
        "2026-08-14",
      );
    });
  });

  it("distinguishes missing children from a disabled school meal plan", async () => {
    mocks.listMyChildren.mockResolvedValue([]);

    const { rerender } = render(<ParentMealPlanPage />);

    expect(
      await screen.findByText("Noch kein Kind verknüpft"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(
        "Für Ihre Schule ist der Essensplan derzeit nicht freigeschaltet.",
      ),
    ).not.toBeInTheDocument();

    mocks.listMyChildren.mockResolvedValue([
      {
        student_id: "child-1",
        tenant_id: "school-1",
        school_name: "OGS Am Berg",
      },
    ]);
    mocks.getChildFeatures.mockResolvedValue({
      meal_plan_enabled: false,
      meal_registration_enabled: false,
    });
    rerender(<ParentMealPlanPage key="disabled-school" />);

    expect(
      await screen.findByText(
        "Für Ihre Schule ist der Essensplan derzeit nicht freigeschaltet.",
      ),
    ).toBeInTheDocument();
  });

  it("shows a load error when resolving the schools fails", async () => {
    mocks.listMyChildren.mockRejectedValue(new Error("network failed"));

    render(<ParentMealPlanPage />);

    expect(
      await screen.findByText("Essensplan konnte nicht geladen werden."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(
        "Für diese Woche ist noch kein Essensplan eingetragen",
      ),
    ).not.toBeInTheDocument();
  });

  it("shows a load error when the selected week fails", async () => {
    mocks.getChildMealPlan.mockRejectedValue(new Error("network failed"));

    render(<ParentMealPlanPage />);

    expect(
      await screen.findByText("Essensplan konnte nicht geladen werden."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(
        "Für diese Woche ist noch kein Essensplan eingetragen",
      ),
    ).not.toBeInTheDocument();
  });
});
