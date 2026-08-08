import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ActivityCategory } from "~/lib/activity-helpers";
import type { TimetableTemplate } from "~/lib/timetable-types";
import * as plannerReferenceApi from "~/lib/planner-reference-api";
import { staffService } from "~/lib/staff-api";
import * as formModel from "./form-model";
import { reconcileCategoryId, useEventForm } from "./use-event-form";
import type { UseEventFormParams } from "./use-event-form";

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
  }),
}));

afterEach(() => {
  vi.restoreAllMocks();
});

describe("reconcileCategoryId", () => {
  const categories = [{ id: "1" }, { id: "2" }];

  it("keeps a selection that remains available", () => {
    expect(reconcileCategoryId("2", categories)).toBe("2");
  });

  it("clears a selection removed by archiving", () => {
    expect(reconcileCategoryId("3", categories)).toBe("");
  });

  it("keeps a newly created selection even if the refetch is stale", () => {
    expect(reconcileCategoryId("1", categories, "3")).toBe("3");
  });
});

describe("useEventForm offering source roster stash", () => {
  it("restores the manual roster when the source is cleared again", async () => {
    vi.spyOn(plannerReferenceApi, "fetchPlannerRooms").mockResolvedValue([]);
    vi.spyOn(plannerReferenceApi, "fetchPlannerGroups").mockResolvedValue([]);
    vi.spyOn(
      plannerReferenceApi,
      "fetchPlannerActivityCategories",
    ).mockResolvedValue([]);
    vi.spyOn(formModel, "fetchAllStudentOptions").mockResolvedValue([]);
    vi.spyOn(staffService, "getAllStaff").mockResolvedValue([]);

    const { result } = renderHook(() =>
      useEventForm({
        isOpen: true,
        onClose: vi.fn(),
        onSaved: vi.fn(),
        defaultDate: "2026-08-03",
        calendarPeriods: [],
        defaultCalendarPeriodId: null,
        initialInstance: null,
        initialSeries: null,
        convertInstance: null,
        defaultRepeat: "none",
        variant: "full",
        canCheckShiftCoverage: false,
      }),
    );

    act(() => {
      result.current.update("studentIds", ["11", "12"]);
    });
    act(() => {
      result.current.changeSourceOfferings(["5"]);
    });
    expect(result.current.form.studentIds).toEqual([]);

    // Adding a second offering keeps the stash from before the first one.
    act(() => {
      result.current.changeSourceOfferings(["5", "6"]);
    });
    expect(result.current.form.sourceCareOfferingIds).toEqual(["5", "6"]);
    expect(result.current.form.studentIds).toEqual([]);

    // Removing one of two sources keeps the sourced roster.
    act(() => {
      result.current.changeSourceOfferings(["6"]);
    });
    expect(result.current.form.sourceCareOfferingIds).toEqual(["6"]);
    expect(result.current.form.studentIds).toEqual([]);

    // Clearing the LAST source must restore the manual picks — submitting
    // the emptied array would wipe the shared manual roster on save.
    act(() => {
      result.current.changeSourceOfferings([]);
    });
    expect(result.current.form.sourceCareOfferingIds).toEqual([]);
    expect(result.current.form.studentIds).toEqual(["11", "12"]);
  });

  it("does not leak the stash into the next modal session", () => {
    vi.spyOn(plannerReferenceApi, "fetchPlannerRooms").mockResolvedValue([]);
    vi.spyOn(plannerReferenceApi, "fetchPlannerGroups").mockResolvedValue([]);
    vi.spyOn(
      plannerReferenceApi,
      "fetchPlannerActivityCategories",
    ).mockResolvedValue([]);
    vi.spyOn(formModel, "fetchAllStudentOptions").mockResolvedValue([]);
    vi.spyOn(staffService, "getAllStaff").mockResolvedValue([]);

    // An already sourced series, as the SECOND session opens it.
    const sourcedSeries: TimetableTemplate = {
      id: "7",
      name: "Mittagessen Jg. 2",
      type: "care",
      categoryId: "2",
      categoryName: "Betreuung",
      isOpen: true,
      maxParticipants: 20,
      targetGroupType: "angebot",
      sourceCareOfferingIds: ["9"],
      enrollmentCount: 0,
      supervisorCount: 0,
      requiredStaffCount: 0,
      assignedStaffCount: 0,
      studentIds: [],
      staffIds: [],
      weekdayAssignments: [],
      schedules: [
        {
          id: "9",
          weekday: 1,
          startTime: "12:00",
          endTime: "13:00",
          weekPattern: 0,
          calendarPeriodId: "5",
        },
      ],
    };

    const props = (
      isOpen: boolean,
      initialSeries: TimetableTemplate | null,
    ): UseEventFormParams => ({
      isOpen,
      onClose: vi.fn(),
      onSaved: vi.fn(),
      defaultDate: "2026-08-03",
      calendarPeriods: [],
      defaultCalendarPeriodId: null,
      initialInstance: null,
      initialSeries,
      convertInstance: null,
      defaultRepeat: "none",
      variant: "full",
      canCheckShiftCoverage: false,
    });

    const { result, rerender } = renderHook(
      (hookProps: UseEventFormParams) => useEventForm(hookProps),
      { initialProps: props(true, null) },
    );

    // Session 1: a manual roster gets stashed when a source is selected.
    act(() => {
      result.current.update("studentIds", ["11", "12"]);
    });
    act(() => {
      result.current.changeSourceOfferings(["5"]);
    });
    expect(result.current.form.studentIds).toEqual([]);

    // Session 2 opens an ALREADY sourced template. Clearing its source must
    // not adopt (and later save) session 1's stashed roster.
    rerender(props(false, sourcedSeries));
    rerender(props(true, sourcedSeries));
    expect(result.current.form.sourceCareOfferingIds).toEqual(["9"]);
    act(() => {
      result.current.changeSourceOfferings([]);
    });
    expect(result.current.form.studentIds).toEqual([]);
  });

  it("asks for confirmation before a source flattens per-weekday staffing", () => {
    vi.spyOn(plannerReferenceApi, "fetchPlannerRooms").mockResolvedValue([]);
    vi.spyOn(plannerReferenceApi, "fetchPlannerGroups").mockResolvedValue([]);
    vi.spyOn(
      plannerReferenceApi,
      "fetchPlannerActivityCategories",
    ).mockResolvedValue([]);
    vi.spyOn(formModel, "fetchAllStudentOptions").mockResolvedValue([]);
    vi.spyOn(staffService, "getAllStaff").mockResolvedValue([]);

    const { result } = renderHook(() =>
      useEventForm({
        isOpen: true,
        onClose: vi.fn(),
        onSaved: vi.fn(),
        defaultDate: "2026-08-03",
        calendarPeriods: [],
        defaultCalendarPeriodId: null,
        initialInstance: null,
        initialSeries: null,
        convertInstance: null,
        defaultRepeat: "none",
        variant: "full",
        canCheckShiftCoverage: false,
      }),
    );

    act(() => {
      result.current.update("weekdays", [1, 2]);
    });
    act(() => {
      result.current.setPerWeekdayRoster(true);
    });
    act(() => {
      result.current.setWeekdayRoster(2, {
        staffIds: ["7"],
        primaryStaffId: "",
        studentIds: [],
      });
    });

    // The pick is parked, not applied — saving now would silently replace
    // the deviating weekday staffing with the shared list.
    act(() => {
      result.current.changeSourceOfferings(["5"]);
    });
    expect(result.current.pendingSourceOfferingIds).toEqual(["5"]);
    expect(result.current.form.sourceCareOfferingIds).toEqual([]);

    act(() => {
      result.current.cancelPendingSourceOffering();
    });
    expect(result.current.pendingSourceOfferingIds).toBeNull();
    expect(result.current.form.sourceCareOfferingIds).toEqual([]);

    act(() => {
      result.current.changeSourceOfferings(["5"]);
    });
    act(() => {
      result.current.confirmPendingSourceOffering();
    });
    expect(result.current.pendingSourceOfferingIds).toBeNull();
    expect(result.current.form.sourceCareOfferingIds).toEqual(["5"]);
  });

  it("restores the manual roster when the target group leaves 'angebot'", () => {
    vi.spyOn(plannerReferenceApi, "fetchPlannerRooms").mockResolvedValue([]);
    vi.spyOn(plannerReferenceApi, "fetchPlannerGroups").mockResolvedValue([]);
    vi.spyOn(
      plannerReferenceApi,
      "fetchPlannerActivityCategories",
    ).mockResolvedValue([]);
    vi.spyOn(formModel, "fetchAllStudentOptions").mockResolvedValue([]);
    vi.spyOn(staffService, "getAllStaff").mockResolvedValue([]);

    const { result } = renderHook(() =>
      useEventForm({
        isOpen: true,
        onClose: vi.fn(),
        onSaved: vi.fn(),
        defaultDate: "2026-08-03",
        calendarPeriods: [],
        defaultCalendarPeriodId: null,
        initialInstance: null,
        initialSeries: null,
        convertInstance: null,
        defaultRepeat: "none",
        variant: "full",
        canCheckShiftCoverage: false,
      }),
    );

    act(() => {
      result.current.update("studentIds", ["11", "12"]);
    });
    act(() => {
      result.current.changeTargetGroupType("angebot");
    });
    act(() => {
      result.current.changeSourceOfferings(["5"]);
    });
    expect(result.current.form.studentIds).toEqual([]);

    // Switching the target group away clears the source, so the stashed
    // manual picks must come back with it — otherwise the next save submits
    // the emptied list and wipes the participants.
    act(() => {
      result.current.changeTargetGroupType("klasse");
    });
    expect(result.current.form.sourceCareOfferingIds).toEqual([]);
    expect(result.current.form.studentIds).toEqual(["11", "12"]);
  });

  it("clears the shared staffing when a confirmed source replaces deviating weekday staff", () => {
    vi.spyOn(plannerReferenceApi, "fetchPlannerRooms").mockResolvedValue([]);
    vi.spyOn(plannerReferenceApi, "fetchPlannerGroups").mockResolvedValue([]);
    vi.spyOn(
      plannerReferenceApi,
      "fetchPlannerActivityCategories",
    ).mockResolvedValue([]);
    vi.spyOn(formModel, "fetchAllStudentOptions").mockResolvedValue([]);
    vi.spyOn(staffService, "getAllStaff").mockResolvedValue([]);

    const { result } = renderHook(() =>
      useEventForm({
        isOpen: true,
        onClose: vi.fn(),
        onSaved: vi.fn(),
        defaultDate: "2026-08-03",
        calendarPeriods: [],
        defaultCalendarPeriodId: null,
        initialInstance: null,
        initialSeries: null,
        convertInstance: null,
        defaultRepeat: "none",
        variant: "full",
        canCheckShiftCoverage: false,
      }),
    );

    act(() => {
      result.current.update("staffIds", ["7"]);
    });
    act(() => {
      result.current.update("weekdays", [1, 2]);
    });
    act(() => {
      result.current.setPerWeekdayRoster(true);
    });
    act(() => {
      result.current.setWeekdayRoster(2, {
        staffIds: ["8"],
        primaryStaffId: "8",
        studentIds: [],
      });
    });

    act(() => {
      result.current.changeSourceOfferings(["5"]);
    });
    act(() => {
      result.current.confirmPendingSourceOffering();
    });

    // Nobody chose an all-weekdays union: per-weekday mode ends and the
    // shared Besetzung starts empty until it is picked explicitly.
    expect(result.current.form.sourceCareOfferingIds).toEqual(["5"]);
    expect(result.current.form.perWeekdayRoster).toBe(false);
    expect(result.current.form.staffIds).toEqual([]);
    expect(result.current.form.primaryStaffId).toBe("");
  });

  it("adopts the identical day staffing when a source ends per-weekday mode without deviation", () => {
    vi.spyOn(plannerReferenceApi, "fetchPlannerRooms").mockResolvedValue([]);
    vi.spyOn(plannerReferenceApi, "fetchPlannerGroups").mockResolvedValue([]);
    vi.spyOn(
      plannerReferenceApi,
      "fetchPlannerActivityCategories",
    ).mockResolvedValue([]);
    vi.spyOn(formModel, "fetchAllStudentOptions").mockResolvedValue([]);
    vi.spyOn(staffService, "getAllStaff").mockResolvedValue([]);

    const { result } = renderHook(() =>
      useEventForm({
        isOpen: true,
        onClose: vi.fn(),
        onSaved: vi.fn(),
        defaultDate: "2026-08-03",
        calendarPeriods: [],
        defaultCalendarPeriodId: null,
        initialInstance: null,
        initialSeries: null,
        convertInstance: null,
        defaultRepeat: "none",
        variant: "full",
        canCheckShiftCoverage: false,
      }),
    );

    act(() => {
      result.current.update("staffIds", ["7"]);
    });
    act(() => {
      result.current.update("weekdays", [1, 2]);
    });
    act(() => {
      result.current.setPerWeekdayRoster(true);
    });
    // The shared list goes stale while the concrete day rosters still agree.
    act(() => {
      result.current.update("staffIds", ["9"]);
    });

    act(() => {
      result.current.changeSourceOfferings(["5"]);
    });

    // No deviation between days, so no confirmation — and the collapse takes
    // the concrete day staffing, not the stale shared list.
    expect(result.current.pendingSourceOfferingIds).toBeNull();
    expect(result.current.form.sourceCareOfferingIds).toEqual(["5"]);
    expect(result.current.form.perWeekdayRoster).toBe(false);
    expect(result.current.form.staffIds).toEqual(["7"]);
  });
});

describe("useEventForm category loading", () => {
  it("does not let the initial request overwrite a newer refresh", async () => {
    let resolveInitialCategories: (value: ActivityCategory[]) => void = () =>
      undefined;
    const initialCategories = new Promise<ActivityCategory[]>((resolve) => {
      resolveInitialCategories = resolve;
    });
    const timestamp = new Date("2026-08-03T10:00:00Z");
    const refreshedCategories: ActivityCategory[] = [
      { id: "2", name: "Neu", created_at: timestamp, updated_at: timestamp },
    ];

    vi.spyOn(plannerReferenceApi, "fetchPlannerRooms").mockResolvedValue([]);
    vi.spyOn(plannerReferenceApi, "fetchPlannerGroups").mockResolvedValue([]);
    vi.spyOn(plannerReferenceApi, "fetchPlannerActivityCategories")
      .mockImplementationOnce(() => initialCategories)
      .mockResolvedValueOnce(refreshedCategories);
    vi.spyOn(formModel, "fetchAllStudentOptions").mockResolvedValue([]);
    vi.spyOn(staffService, "getAllStaff").mockResolvedValue([]);

    const { result } = renderHook(() =>
      useEventForm({
        isOpen: true,
        onClose: vi.fn(),
        onSaved: vi.fn(),
        defaultDate: "2026-08-03",
        calendarPeriods: [],
        defaultCalendarPeriodId: null,
        initialInstance: null,
        initialSeries: null,
        convertInstance: null,
        defaultRepeat: "none",
        variant: "full",
        canCheckShiftCoverage: false,
      }),
    );

    await waitFor(() => {
      expect(
        plannerReferenceApi.fetchPlannerActivityCategories,
      ).toHaveBeenCalledTimes(1);
    });
    await act(async () => {
      await result.current.refreshCategories("2");
    });
    expect(result.current.categories).toEqual(refreshedCategories);
    expect(result.current.form.categoryId).toBe("2");

    await act(async () => {
      resolveInitialCategories([
        { id: "1", name: "Alt", created_at: timestamp, updated_at: timestamp },
      ]);
      await initialCategories;
    });

    expect(result.current.categories).toEqual(refreshedCategories);
    expect(result.current.form.categoryId).toBe("2");
  });
});
