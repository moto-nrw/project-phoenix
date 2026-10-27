import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockUseUserContext } = vi.hoisted(() => ({
  mockUseUserContext: vi.fn(),
}));

vi.mock("~/lib/hooks/use-user-context", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/hooks/use-user-context")>()),
  useUserContext: mockUseUserContext,
}));

import { IncompleteUserContextError } from "./use-user-context";

import {
  useSchoolDayTimes,
  useSchoolParentInviteMode,
  useSchoolParentNewsEnabled,
} from "./use-school-switches";

const switches = {
  parentNewsEnabled: true,
  parentInviteMode: "direct",
  timetableDayStartTime: "07:30",
  timetableDayEndTime: "16:15",
} as const;

function context(value: {
  schoolSwitches?: typeof switches | null;
  isLoading?: boolean;
  error?: Error;
}) {
  mockUseUserContext.mockReturnValue({
    userContext:
      value.schoolSwitches === undefined
        ? undefined
        : { schoolSwitches: value.schoolSwitches },
    isLoading: value.isLoading ?? false,
    error: value.error,
  });
}

describe("useSchoolSwitches", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("reads the switches of the signed-in navigation context", () => {
    context({ schoolSwitches: switches });

    expect(renderHook(() => useSchoolParentNewsEnabled()).result.current).toBe(
      true,
    );
    expect(renderHook(() => useSchoolDayTimes()).result.current).toEqual({
      start: "07:30",
      end: "16:15",
    });
    expect(
      renderHook(() => useSchoolParentInviteMode()).result.current,
    ).toEqual({ mode: "direct", isLoading: false, error: undefined });
  });

  it("keeps parent news hidden while the context loads or failed", () => {
    context({ isLoading: true });
    expect(renderHook(() => useSchoolParentNewsEnabled()).result.current).toBe(
      false,
    );

    const failure = new Error("navigation failed");
    context({ error: failure });
    expect(renderHook(() => useSchoolParentNewsEnabled()).result.current).toBe(
      false,
    );
    expect(
      renderHook(() => useSchoolParentInviteMode()).result.current.error,
    ).toBe(failure);
  });

  it("preserves a refresh failure even with cached switches", () => {
    const failure = new Error("navigation refresh failed");
    context({ schoolSwitches: switches, error: failure });

    expect(
      renderHook(() => useSchoolParentInviteMode()).result.current,
    ).toEqual({ mode: "direct", isLoading: false, error: failure });
  });

  it("ignores optional-section failures only when their response has switches", () => {
    const failure = new IncompleteUserContextError({
      educationalGroups: [],
      supervisedGroups: [],
      currentStaff: null,
      educationalGroupIds: [],
      educationalGroupRoomNames: [],
      supervisedRoomNames: [],
      incomplete: true,
      unavailableSections: ["supervised_groups"],
      schoolSwitches: switches,
    });
    context({ schoolSwitches: switches, error: failure });
    expect(
      renderHook(() => useSchoolParentInviteMode()).result.current,
    ).toEqual({ mode: "direct", isLoading: false, error: undefined });

    const withoutSwitches = new IncompleteUserContextError({
      ...failure.userContext,
      schoolSwitches: null,
    });
    context({ schoolSwitches: switches, error: withoutSwitches });
    expect(
      renderHook(() => useSchoolParentInviteMode()).result.current.error,
    ).toBe(withoutSwitches);
  });

  it("skips the request when disabled", () => {
    context({ schoolSwitches: null });

    renderHook(() => useSchoolParentNewsEnabled(false));

    expect(mockUseUserContext).toHaveBeenCalledWith(false);
  });
});
