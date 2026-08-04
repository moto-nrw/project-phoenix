import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ActivityCategory } from "~/lib/activity-helpers";
import * as plannerReferenceApi from "~/lib/planner-reference-api";
import { staffService } from "~/lib/staff-api";
import * as formModel from "./form-model";
import { reconcileCategoryId, useEventForm } from "./use-event-form";

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
      result.current.changeSourceOffering("5");
    });
    expect(result.current.form.studentIds).toEqual([]);

    // Switching between offerings keeps the stash from before the first one.
    act(() => {
      result.current.changeSourceOffering("6");
    });
    expect(result.current.form.studentIds).toEqual([]);

    // Clearing the source must restore the manual picks — submitting the
    // emptied array would wipe the shared manual roster on save.
    act(() => {
      result.current.changeSourceOffering("");
    });
    expect(result.current.form.studentIds).toEqual(["11", "12"]);
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
      result.current.changeSourceOffering("5");
    });
    expect(result.current.pendingSourceOfferingId).toBe("5");
    expect(result.current.form.sourceCareOfferingId).toBe("");

    act(() => {
      result.current.cancelPendingSourceOffering();
    });
    expect(result.current.pendingSourceOfferingId).toBeNull();
    expect(result.current.form.sourceCareOfferingId).toBe("");

    act(() => {
      result.current.changeSourceOffering("5");
    });
    act(() => {
      result.current.confirmPendingSourceOffering();
    });
    expect(result.current.pendingSourceOfferingId).toBeNull();
    expect(result.current.form.sourceCareOfferingId).toBe("5");
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
