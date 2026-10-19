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
  setMealParticipationWeek: vi.fn(),
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
  setMealParticipationWeek: mocks.setMealParticipationWeek,
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
      regular_participating: true,
      source: "regular",
      changeable,
      can_register: changeable,
      can_cancel: changeable,
      register_until: `${date}T07:00:00Z`,
      cancel_until: `${date}T07:00:00Z`,
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
        model: "ongoing_single",
        schedule_changeable: true,
        sick_cancels_after_deadline: false,
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
        model: "ongoing_single",
        schedule_changeable: true,
        sick_cancels_after_deadline: false,
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
          model: "ongoing_single",
          schedule_changeable: true,
          sick_cancels_after_deadline: false,
          days: [
            participationDay("2026-08-14", true),
            participationDay("2026-08-17", true),
          ],
        })
        .mockResolvedValueOnce({
          weekdays: [1, 2, 3, 4],
          effective_from: "2026-08-16",
          cutoff_time: "09:00",
          model: "ongoing_single",
          schedule_changeable: true,
          sick_cancels_after_deadline: false,
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
          "2026-09-04",
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
      model: "ongoing_single",
      schedule_changeable: true,
      sick_cancels_after_deadline: false,
      days: [
        {
          date: "2026-08-12",
          participating: false,
          regular_participating: false,
          source: "none",
          changeable: true,
          can_register: true,
          can_cancel: true,
          register_until: "2026-08-12T07:00:00Z",
          cancel_until: "2026-08-12T07:00:00Z",
        },
        {
          date: "2026-08-14",
          participating: true,
          regular_participating: false,
          source: "override",
          changeable: true,
          can_register: true,
          can_cancel: true,
          register_until: "2026-08-12T07:00:00Z",
          cancel_until: "2026-08-12T07:00:00Z",
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
        "Bis wann Sie ändern können, steht bei jedem Tag.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Montag und Mittwoch")).toBeInTheDocument();
    expect(screen.getByText("Gemüse-Lasagne")).toBeInTheDocument();
    expect(screen.getByText("mit Salat")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Eine bestätigte Krankmeldung oder Abwesenheit bis zur Frist eines Tages meldet Ihr Kind vom Essen ab. Danach bleibt das Essen bestellt.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("parentMealPlan.participationSicknessDeadline"),
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

  it("marks each participation state with its own icon next to the text", async () => {
    mocks.getChildFeatures.mockResolvedValue({
      meal_plan_enabled: true,
      meal_registration_enabled: true,
    });
    mocks.getMealParticipation.mockResolvedValue({
      weekdays: [1, 2, 3],
      effective_from: "2026-08-10",
      cutoff_time: "09:00",
      days: [
        {
          date: "2026-08-10",
          participating: true,
          source: "regular",
          changeable: false,
        },
        {
          date: "2026-08-11",
          participating: false,
          source: "sick",
          changeable: false,
        },
        {
          date: "2026-08-14",
          participating: false,
          source: "none",
          changeable: true,
        },
      ],
    });

    render(<ParentMealPlanPage />);

    const badgeFor = async (label: string) => {
      const badge = await screen.findByText(label);
      expect(badge).toHaveTextContent(new RegExp(`^${label}$`));
      const icons = badge.querySelectorAll("svg");
      expect(icons).toHaveLength(1);
      expect(icons[0]).toHaveAttribute("aria-hidden", "true");
      return icons[0];
    };

    expect(await badgeFor("Zum Mittagessen angemeldet")).toHaveClass(
      "lucide-check",
    );
    expect(await badgeFor("Durch die Krankmeldung abgemeldet")).toHaveClass(
      "lucide-ban",
    );
    expect(await badgeFor("Nicht zum Mittagessen angemeldet")).toHaveClass(
      "lucide-minus",
    );
  });

  it("switches the icon with the draft while a day is being changed", async () => {
    mocks.getChildFeatures.mockResolvedValue({
      meal_plan_enabled: true,
      meal_registration_enabled: true,
    });
    mocks.getMealParticipation.mockResolvedValue({
      weekdays: [],
      effective_from: "2026-08-10",
      cutoff_time: "09:00",
      days: [
        {
          date: "2026-08-14",
          participating: false,
          regular_participating: false,
          source: "none",
          changeable: true,
          can_register: true,
          can_cancel: true,
          register_until: "2026-08-14T07:00:00Z",
          cancel_until: "2026-08-14T07:00:00Z",
        },
      ],
    });

    render(<ParentMealPlanPage />);

    const before = await screen.findByText("Nicht zum Mittagessen angemeldet");
    expect(before.querySelector("svg")).toHaveClass("lucide-minus");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Anmeldung ändern: Freitag, 14. August",
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Anmelden" }));

    const after = screen.getByText("Zum Mittagessen angemeldet");
    expect(after.querySelector("svg")).toHaveClass("lucide-check");
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
      model: "ongoing_single",
      schedule_changeable: true,
      sick_cancels_after_deadline: false,
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
      model: "ongoing_single",
      schedule_changeable: true,
      sick_cancels_after_deadline: false,
      days: [
        {
          date: "2026-08-12",
          participating: false,
          regular_participating: false,
          source: "none",
          changeable: true,
          can_register: true,
          can_cancel: true,
          register_until: "2026-08-12T07:00:00Z",
          cancel_until: "2026-08-12T07:00:00Z",
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

    it("shows no state icon while the participation could not be loaded", async () => {
      mocks.getMealParticipation.mockRejectedValue(unavailable());

      render(<ParentMealPlanPage />);

      const badges = await screen.findAllByText("Anmeldung nicht geladen");
      expect(badges).toHaveLength(5);
      for (const badge of badges) {
        expect(badge.querySelector("svg")).toBeNull();
      }
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

  // #3873: deadlines per day, the annual registration and the week change.
  describe("ordering rules", () => {
    const day = (
      date: string,
      overrides: Partial<{
        participating: boolean;
        regular_participating: boolean;
        source: string;
        can_register: boolean;
        can_cancel: boolean;
        register_until: string;
        cancel_until: string;
      }> = {},
    ) => {
      const value = {
        date,
        participating: true,
        regular_participating: true,
        source: "regular",
        can_register: true,
        can_cancel: true,
        // 12:00 Berlin on the Thursday before, 08:00 Berlin on the day.
        register_until: "2026-08-06T10:00:00Z",
        cancel_until: `${date}T06:00:00Z`,
        ...overrides,
      };
      return {
        ...value,
        changeable: value.can_register || value.can_cancel,
      };
    };
    const plan = (
      overrides: Partial<{
        model: string;
        schedule_changeable: boolean;
        sick_cancels_after_deadline: boolean;
        days: ReturnType<typeof day>[];
      }> = {},
    ) => ({
      weekdays: [1, 3],
      effective_from: "2026-08-10",
      cutoff_time: "12:00",
      model: "ongoing_split",
      schedule_changeable: true,
      sick_cancels_after_deadline: false,
      days: [],
      ...overrides,
    });

    beforeEach(() => {
      mocks.getChildFeatures.mockResolvedValue({
        meal_plan_enabled: true,
        meal_registration_enabled: true,
      });
    });

    it("shows each deadline of a day and offers only what is still open", async () => {
      mocks.getMealParticipation.mockResolvedValue(
        plan({
          days: [
            day("2026-08-12", { can_register: false }),
            day("2026-08-13", {
              participating: false,
              regular_participating: false,
              source: "none",
              can_register: false,
            }),
          ],
        }),
      );

      render(<ParentMealPlanPage />);

      expect(await screen.findAllByText(/^Abmelden bis .*08:00/)).toHaveLength(
        2,
      );
      expect(screen.queryByText(/^Anmelden bis/)).not.toBeInTheDocument();

      fireEvent.click(
        screen.getByRole("button", {
          name: "Anmeldung ändern: Mittwoch, 12. August",
        }),
      );
      const editor = screen.getByRole("group", {
        name: "Anmeldung ändern: Mittwoch, 12. August",
      });
      expect(
        within(editor).getByRole("button", { name: "Abmelden" }),
      ).toBeInTheDocument();
      fireEvent.click(
        within(editor).getByRole("button", { name: "Abbrechen" }),
      );

      fireEvent.click(
        screen.getByRole("button", {
          name: "Anmeldung ändern: Donnerstag, 13. August",
        }),
      );
      const closedEditor = screen.getByRole("group", {
        name: "Anmeldung ändern: Donnerstag, 13. August",
      });
      expect(
        within(closedEditor).queryByRole("button", { name: "Anmelden" }),
      ).not.toBeInTheDocument();
    });

    it("locks the regular days once the school year registration closed", async () => {
      mocks.getMealParticipation.mockResolvedValue(
        plan({
          model: "annual",
          schedule_changeable: false,
          days: [day("2026-08-12", { can_register: false })],
        }),
      );

      render(<ParentMealPlanPage />);

      expect(
        await screen.findByText(
          "Die Anmeldung für das Schuljahr ist abgeschlossen. Feste Tage ändert jetzt nur die Schule.",
        ),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Ändern" }),
      ).not.toBeInTheDocument();
    });

    it("tells parents when a late sick note still cancels the meal", async () => {
      mocks.getMealParticipation.mockResolvedValue(
        plan({ sick_cancels_after_deadline: true }),
      );

      render(<ParentMealPlanPage />);

      expect(
        await screen.findByText(
          "Eine bestätigte Krankmeldung oder Abwesenheit bis zum Essenstag meldet Ihr Kind vom Essen ab.",
        ),
      ).toBeInTheDocument();
    });

    it("cancels the remaining days of a week and names the days already ordered", async () => {
      mocks.getMealParticipation.mockResolvedValue(
        plan({
          days: [
            day("2026-08-12", { can_cancel: false, can_register: false }),
            day("2026-08-13"),
          ],
        }),
      );
      mocks.setMealParticipationWeek.mockResolvedValue({
        changed: ["2026-08-13", "2026-08-14"],
        locked: ["2026-08-12"],
      });

      render(<ParentMealPlanPage />);

      fireEvent.click(
        await screen.findByRole("button", { name: "Woche abmelden" }),
      );
      const dialog = await screen.findByRole("dialog");
      expect(
        within(dialog).getByText(
          "Mia Muster wird an allen Tagen dieser Woche abgemeldet, die Sie noch ändern können.",
        ),
      ).toBeInTheDocument();
      fireEvent.click(within(dialog).getByRole("button", { name: "Abmelden" }));

      await waitFor(() =>
        expect(mocks.setMealParticipationWeek).toHaveBeenCalledWith(
          "child-1",
          "2026-08-10",
          false,
        ),
      );
      expect(
        await screen.findByText(
          "Die Woche ist abgemeldet. Diese Tage waren schon bestellt und bleiben angemeldet: Mittwoch.",
        ),
      ).toBeInTheDocument();
      expect(mocks.getMealParticipation).toHaveBeenCalledTimes(2);
    });

    it("names absent and closed days without a change button", async () => {
      mocks.getMealParticipation.mockResolvedValue(
        plan({
          days: [
            day("2026-08-12", { participating: false, source: "absent" }),
            day("2026-08-13", {
              participating: false,
              source: "closed",
              can_register: false,
              can_cancel: false,
            }),
          ],
        }),
      );

      render(<ParentMealPlanPage />);

      expect(
        await screen.findByText("Abgemeldet, weil Ihr Kind fehlt"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("An diesem Tag ist die OGS zu"),
      ).toBeInTheDocument();
      expect(screen.queryByText("Nicht mehr änderbar")).not.toBeInTheDocument();
      expect(
        screen.queryByRole("button", {
          name: "Anmeldung ändern: Mittwoch, 12. August",
        }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("button", {
          name: "Anmeldung ändern: Donnerstag, 13. August",
        }),
      ).not.toBeInTheDocument();
    });

    it("steps back to next week when the other school has no registration", async () => {
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
      mocks.getChildFeatures.mockImplementation(async (studentId: string) => ({
        meal_plan_enabled: true,
        meal_registration_enabled: studentId === "child-1",
      }));
      mocks.getMealParticipation.mockResolvedValue(plan());

      render(<ParentMealPlanPage />);

      const navigation = await screen.findByRole("navigation", {
        name: "Kalenderwoche wechseln",
      });
      const weekStatus = within(navigation).getByRole("status");
      const nextWeek = within(navigation).getByRole("button", {
        name: "Nächste Woche",
      });
      for (const label of [/Nächste Woche/, /In 2 Wochen/]) {
        await waitFor(() => expect(nextWeek).toBeEnabled());
        fireEvent.click(nextWeek);
        await waitFor(() => expect(weekStatus).toHaveTextContent(label));
      }

      fireEvent.click(screen.getByRole("combobox", { name: "Kind" }));
      fireEvent.click(
        await screen.findByRole("option", {
          name: "Noah Beispiel · OGS Am Park",
        }),
      );

      await waitFor(() =>
        expect(
          within(
            screen.getByRole("navigation", { name: "Kalenderwoche wechseln" }),
          ).getByRole("status"),
        ).toHaveTextContent(/Nächste Woche/),
      );
    });

    it("reaches four weeks and loads the meal plan only for the first two", async () => {
      mocks.getMealParticipation.mockResolvedValue(plan());

      render(<ParentMealPlanPage />);

      const navigation = await screen.findByRole("navigation", {
        name: "Kalenderwoche wechseln",
      });
      const nextWeek = within(navigation).getByRole("button", {
        name: "Nächste Woche",
      });
      const weekStatus = within(navigation).getByRole("status");
      for (const label of [/Nächste Woche/, /In 2 Wochen/, /In 3 Wochen/]) {
        await waitFor(() => expect(nextWeek).toBeEnabled());
        fireEvent.click(nextWeek);
        await waitFor(() => expect(weekStatus).toHaveTextContent(label));
      }
      await waitFor(() => expect(nextWeek).toBeDisabled());
      expect(screen.getAllByText("Essensplan folgt")).toHaveLength(5);
      expect(mocks.getChildMealPlan).not.toHaveBeenCalledWith(
        "child-1",
        "2026-08-24",
      );
      expect(mocks.getChildMealPlan).not.toHaveBeenCalledWith(
        "child-1",
        "2026-08-31",
      );
    });
  });
});
