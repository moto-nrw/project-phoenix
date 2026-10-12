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
import deMessages from "~/i18n/messages/de.json";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const unavailable = () =>
  new ApiError("diag", 503, { code: "general.unavailable" });
const SCHOOLS_ERROR = catalogText(
  "general.unavailable",
  deMessages.parentMealPlan.errorObjectSchools,
);
const WEEK_ERROR = catalogText(
  "general.unavailable",
  deMessages.parentMealPlan.errorObjectWeek,
);

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

  describe("opening week with meal registration", () => {
    const participationDay = (date: string, changeable: boolean) => ({
      date,
      participating: true,
      source: "regular",
      changeable,
    });

    beforeEach(() => {
      mocks.getChildFeatures.mockResolvedValue({
        meal_plan_enabled: true,
        meal_registration_enabled: true,
      });
    });

    it("opens next week when no day of this week can be changed anymore", async () => {
      mocks.today = "2026-08-15";
      mocks.getMealParticipation.mockResolvedValue({
        weekdays: [1, 2, 3, 4],
        effective_from: "2026-08-17",
        cutoff_time: "09:00",
        days: [
          participationDay("2026-08-14", false),
          participationDay("2026-08-17", true),
        ],
      });

      render(<ParentMealPlanPage />);

      const weekStatus = within(
        await screen.findByRole("navigation", {
          name: "Kalenderwoche wechseln",
        }),
      ).getByRole("status");
      await waitFor(() => {
        expect(weekStatus).toHaveTextContent(/KW 34\s*· Nächste Woche/);
      });
      expect(mocks.getChildMealPlan).toHaveBeenLastCalledWith(
        "child-1",
        "2026-08-17",
      );

      // Die Familie kann danach in diese Woche zurück; die Seite springt
      // nicht erneut.
      const previousWeek = screen.getByRole("button", {
        name: "Vorherige Woche",
      });
      // Der Pfeil ist gesperrt, bis die nächste Woche geladen ist.
      await waitFor(() => expect(previousWeek).toBeEnabled());
      fireEvent.click(previousWeek);
      await waitFor(() => {
        expect(weekStatus).toHaveTextContent(/KW 33\s*· Diese Woche/);
      });
    });

    it("stays on this week while a day of it can still be changed", async () => {
      mocks.getMealParticipation.mockResolvedValue({
        weekdays: [1, 2, 3, 4],
        effective_from: "2026-08-13",
        cutoff_time: "09:00",
        days: [
          participationDay("2026-08-12", false),
          participationDay("2026-08-13", true),
        ],
      });

      render(<ParentMealPlanPage />);

      await screen.findByRole("heading", {
        name: "Wann isst Mia Muster mit?",
      });
      await screen.findByText("Montag, Dienstag, Mittwoch und Donnerstag");
      const weekStatus = within(
        screen.getByRole("navigation", { name: "Kalenderwoche wechseln" }),
      ).getByRole("status");
      expect(weekStatus).toHaveTextContent(/KW 33\s*· Diese Woche/);
    });

    it("rechecks the opening week after Berlin midnight", async () => {
      mocks.today = "2026-08-14";
      mocks.getMealParticipation
        .mockResolvedValueOnce({
          weekdays: [1, 2, 3, 4],
          effective_from: "2026-08-14",
          cutoff_time: "09:00",
          days: [
            participationDay("2026-08-14", true),
            participationDay("2026-08-17", true),
          ],
        })
        .mockResolvedValueOnce({
          weekdays: [1, 2, 3, 4],
          effective_from: "2026-08-16",
          cutoff_time: "09:00",
          days: [
            participationDay("2026-08-14", false),
            participationDay("2026-08-17", true),
          ],
        });

      const { rerender } = render(<ParentMealPlanPage />);

      const weekStatus = within(
        await screen.findByRole("navigation", {
          name: "Kalenderwoche wechseln",
        }),
      ).getByRole("status");
      await waitFor(() => {
        expect(weekStatus).toHaveTextContent(/KW 33\s*· Diese Woche/);
      });

      mocks.today = "2026-08-15";
      rerender(<ParentMealPlanPage />);

      await waitFor(() => {
        expect(mocks.getMealParticipation).toHaveBeenLastCalledWith(
          "child-1",
          "2026-08-10",
          "2026-08-21",
        );
        expect(weekStatus).toHaveTextContent(/KW 34\s*· Nächste Woche/);
      });
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
    mocks.listMyChildren.mockRejectedValue(unavailable());

    render(<ParentMealPlanPage />);

    expect(await screen.findByText(SCHOOLS_ERROR)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Wiederholen" })).toBeVisible();
    expect(
      screen.queryByText(
        "Für diese Woche ist noch kein Essensplan eingetragen",
      ),
    ).not.toBeInTheDocument();
  });

  it("shows a load error when the selected week fails", async () => {
    mocks.getChildMealPlan.mockRejectedValue(unavailable());

    render(<ParentMealPlanPage />);

    expect(await screen.findByText(WEEK_ERROR)).toBeInTheDocument();
    expect(
      screen.queryByText(
        "Für diese Woche ist noch kein Essensplan eingetragen",
      ),
    ).not.toBeInTheDocument();
  });

  const SUBTITLE_WITH_REGISTRATION =
    "Hier sehen Sie, wann Ihr Kind mitisst und was es gibt.";
  const SUBTITLE_MEALS_ONLY = "Hier sehen Sie, was es zu essen gibt.";
  const MEAL_PLAN_ENTRY = {
    date: "2026-08-12",
    position: 0,
    dish: "Gemüse-Lasagne",
    note: "mit Salat",
  };

  it("promises only the dishes while the meal registration is switched off", async () => {
    mocks.getChildMealPlan.mockResolvedValue([MEAL_PLAN_ENTRY]);

    render(<ParentMealPlanPage />);

    expect(await screen.findByText(SUBTITLE_MEALS_ONLY)).toBeInTheDocument();
    expect(
      screen.queryByText(SUBTITLE_WITH_REGISTRATION),
    ).not.toBeInTheDocument();
  });

  it("promises the participation days once the meal registration is switched on", async () => {
    mocks.getChildFeatures.mockResolvedValue({
      meal_plan_enabled: true,
      meal_registration_enabled: true,
    });
    mocks.getChildMealPlan.mockResolvedValue([MEAL_PLAN_ENTRY]);
    mocks.getMealParticipation.mockResolvedValue({
      weekdays: [],
      effective_from: "2026-08-10",
      cutoff_time: "09:00",
      days: [],
    });

    render(<ParentMealPlanPage />);

    expect(
      await screen.findByText(SUBTITLE_WITH_REGISTRATION),
    ).toBeInTheDocument();
    expect(screen.queryByText(SUBTITLE_MEALS_ONLY)).not.toBeInTheDocument();
  });

  it("carries no subtitle until the selected week has a plan", async () => {
    mocks.getChildMealPlan.mockReturnValue(new Promise(() => {}));

    const { unmount } = render(<ParentMealPlanPage />);

    // Selected week loading.
    await screen.findByRole("navigation", {
      name: "Kalenderwoche wechseln",
    });
    expect(screen.getByTestId("meal-plan-week-skeleton")).toBeInTheDocument();
    expect(screen.queryByText(SUBTITLE_MEALS_ONLY)).not.toBeInTheDocument();
    unmount();

    // Selected week could not be loaded.
    mocks.getChildMealPlan.mockRejectedValue(unavailable());
    const { unmount: unmountWeekError } = render(
      <ParentMealPlanPage key="week-error" />,
    );
    expect(await screen.findByText(WEEK_ERROR)).toBeInTheDocument();
    expect(screen.queryByText(SUBTITLE_MEALS_ONLY)).not.toBeInTheDocument();
    unmountWeekError();

    // No dishes were entered for the selected week.
    mocks.getChildMealPlan.mockResolvedValue([]);
    render(<ParentMealPlanPage key="empty-week" />);
    expect(
      await screen.findByText(
        "Für diese Woche ist noch kein Essensplan eingetragen",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(SUBTITLE_MEALS_ONLY)).not.toBeInTheDocument();
  });

  it("carries no subtitle while no plan can be shown", async () => {
    mocks.listMyChildren.mockReturnValue(new Promise(() => {}));

    const { rerender, unmount } = render(<ParentMealPlanPage />);

    // Loading.
    expect(
      screen.queryByText(SUBTITLE_WITH_REGISTRATION),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(SUBTITLE_MEALS_ONLY)).not.toBeInTheDocument();

    // No linked child.
    mocks.listMyChildren.mockResolvedValue([]);
    rerender(<ParentMealPlanPage key="no-children" />);
    expect(
      await screen.findByText("Noch kein Kind verknüpft"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(SUBTITLE_WITH_REGISTRATION),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(SUBTITLE_MEALS_ONLY)).not.toBeInTheDocument();

    // Meal plan switched off for the school.
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
      meal_plan_enabled: false,
      meal_registration_enabled: false,
    });
    rerender(<ParentMealPlanPage key="disabled-school" />);
    expect(
      await screen.findByText(
        "Für Ihre Schule ist der Essensplan derzeit nicht freigeschaltet.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(SUBTITLE_MEALS_ONLY)).not.toBeInTheDocument();

    unmount();

    // Resolving the schools failed.
    mocks.listMyChildren.mockRejectedValue(unavailable());
    render(<ParentMealPlanPage key="schools-error" />);
    expect(await screen.findByText(SCHOOLS_ERROR)).toBeInTheDocument();
    expect(screen.queryByText(SUBTITLE_MEALS_ONLY)).not.toBeInTheDocument();
  });

  it("reloads the schools from the load error's Wiederholen", async () => {
    mocks.listMyChildren.mockRejectedValueOnce(unavailable());

    render(<ParentMealPlanPage />);

    fireEvent.click(await screen.findByRole("button", { name: "Wiederholen" }));

    expect(
      await screen.findByText(
        "Für diese Woche ist noch kein Essensplan eingetragen",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(SCHOOLS_ERROR)).not.toBeInTheDocument();
    expect(mocks.listMyChildren).toHaveBeenCalledTimes(2);
  });

  it("reloads the week from the load error's Wiederholen", async () => {
    mocks.getChildMealPlan.mockRejectedValueOnce(unavailable());

    render(<ParentMealPlanPage />);

    fireEvent.click(await screen.findByRole("button", { name: "Wiederholen" }));

    expect(
      await screen.findByText(
        "Für diese Woche ist noch kein Essensplan eingetragen",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(WEEK_ERROR)).not.toBeInTheDocument();
  });

  describe("participation errors", () => {
    const PARTICIPATION = {
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
      ],
    };

    beforeEach(() => {
      mocks.getChildFeatures.mockResolvedValue({
        meal_plan_enabled: true,
        meal_registration_enabled: true,
      });
    });

    it("shows a failed participation load in place with Wiederholen", async () => {
      mocks.getMealParticipation
        .mockRejectedValueOnce(unavailable())
        .mockResolvedValue(PARTICIPATION);

      render(<ParentMealPlanPage />);

      expect(
        await screen.findByText(
          catalogText(
            "general.unavailable",
            deMessages.parentMealPlan.errorObjectParticipation,
          ),
        ),
      ).toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

      expect(
        await screen.findByText("Montag und Mittwoch"),
      ).toBeInTheDocument();
    });

    it("keeps the regular-days draft open and shows the save error in it", async () => {
      mocks.getMealParticipation.mockResolvedValue(PARTICIPATION);
      mocks.replaceMealParticipationSchedule
        .mockRejectedValueOnce(
          new ApiError("diag", 409, { code: "general.business_rejection" }),
        )
        .mockResolvedValue({ effective_from: "2026-08-17" });

      render(<ParentMealPlanPage />);

      fireEvent.click(await screen.findByRole("button", { name: "Ändern" }));
      fireEvent.click(screen.getByRole("checkbox", { name: "Dienstag" }));
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      expect(
        await screen.findByText(
          catalogText(
            "general.business_rejection",
            deMessages.parentMealPlan.errorObjectParticipation,
          ),
        ),
      ).toBeInTheDocument();
      expect(screen.getByRole("checkbox", { name: "Dienstag" })).toBeChecked();

      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
      await waitFor(() =>
        expect(mocks.replaceMealParticipationSchedule).toHaveBeenLastCalledWith(
          "child-1",
          [1, 2, 3],
        ),
      );
      await waitFor(() =>
        expect(
          screen.queryByRole("checkbox", { name: "Dienstag" }),
        ).not.toBeInTheDocument(),
      );
    });

    it("retries a failed day change with the current draft", async () => {
      mocks.getMealParticipation.mockResolvedValue(PARTICIPATION);
      mocks.setMealParticipationDay
        .mockRejectedValueOnce(unavailable())
        .mockResolvedValue(undefined);

      render(<ParentMealPlanPage />);

      fireEvent.click(
        await screen.findByRole("button", {
          name: "Anmeldung ändern: Mittwoch, 12. August",
        }),
      );
      fireEvent.click(screen.getByRole("button", { name: "Anmelden" }));
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      expect(
        await screen.findByText(
          catalogText(
            "general.unavailable",
            deMessages.parentMealPlan.errorObjectParticipation,
          ),
        ),
      ).toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

      await waitFor(() =>
        expect(mocks.setMealParticipationDay).toHaveBeenCalledTimes(2),
      );
      expect(mocks.setMealParticipationDay).toHaveBeenLastCalledWith(
        "child-1",
        "2026-08-12",
        true,
      );
    });
  });
});
