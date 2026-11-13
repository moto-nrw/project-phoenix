import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { TargetRoomOption } from "~/lib/move-targets";

const mocks = vi.hoisted(() => ({
  batch: vi.fn(),
  single: vi.fn(),
  toGroup: vi.fn(),
  toRoom: vi.fn(),
  mutate: vi.fn(),
  success: vi.fn(),
  show: vi.fn(),
}));

vi.mock("~/lib/student-api", () => ({
  schoolCheckinStudent: mocks.single,
  schoolCheckinStudentsBatch: mocks.batch,
}));
vi.mock("~/lib/active-service", () => ({
  activeService: {
    checkInStudentsToActiveGroup: mocks.toGroup,
    checkInStudentsToOpenRoom: mocks.toRoom,
  },
}));
vi.mock("swr", () => ({ mutate: mocks.mutate }));
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mocks.success, error: vi.fn() }),
  useApiErrorDisplay: () => ({ show: mocks.show }),
}));

import { useSchoolCheckinMode } from "./use-school-checkin-mode";

// The check-in mode with a target (#6953): a check-in puts the children into
// the chosen room in the same request, a checkout stays the school checkout.

const mensa: TargetRoomOption = {
  kind: "session",
  value: "12",
  activeGroupId: "12",
  roomId: "3",
  roomName: "Mensa",
  label: "Mensa",
};
const yard: TargetRoomOption = {
  kind: "openRoom",
  value: "room:9",
  roomId: "9",
  roomName: "Schulhof",
  label: "Schulhof (offener Raum)",
};

describe("useSchoolCheckinMode with a check-in target", () => {
  beforeEach(() => {
    for (const mock of Object.values(mocks)) mock.mockReset();
    mocks.mutate.mockResolvedValue(undefined);
  });

  it("checks a tapped child straight into the released room", async () => {
    mocks.toRoom.mockResolvedValue({ moved: [5], unchanged: [], skipped: [] });
    const { result } = renderHook(() => useSchoolCheckinMode({ target: yard }));

    await act(async () => {
      await result.current.toggle("5", "abwesend");
    });

    expect(mocks.toRoom).toHaveBeenCalledWith(["5"], "9");
    expect(mocks.single).not.toHaveBeenCalled();
  });

  it("checks out without the target", async () => {
    mocks.single.mockResolvedValue({});
    const { result } = renderHook(() =>
      useSchoolCheckinMode({ target: mensa }),
    );

    await act(async () => {
      await result.current.toggle("5", "anwesend");
    });

    expect(mocks.single).toHaveBeenCalledWith("5", "out", undefined);
    expect(mocks.toGroup).not.toHaveBeenCalled();
  });

  it("places only arriving children and leaves present ones where they are", async () => {
    mocks.toGroup.mockResolvedValue({
      moved: [1],
      unchanged: [],
      skipped: [],
      checked_in: [1],
    });
    const { result } = renderHook(() =>
      useSchoolCheckinMode({ target: mensa }),
    );

    let outcome: Awaited<ReturnType<typeof result.current.runBulk>> = null;
    await act(async () => {
      outcome = await result.current.runBulk(
        "in",
        ["1", "2"],
        undefined,
        new Set(["2"]),
      );
    });

    expect(mocks.toGroup).toHaveBeenCalledWith(["1"], "12");
    expect(mocks.batch).not.toHaveBeenCalled();
    expect(outcome).toMatchObject({ succeeded: 2, failed: 0 });
    expect(mocks.success).toHaveBeenCalledWith(
      "1 Kind in Mensa angemeldet. 1 war schon da und bleibt, wo es ist.",
    );
    const matcher = mocks.mutate.mock.calls[0]?.[0] as (
      key: unknown,
    ) => boolean;
    expect(matcher("room-students-3")).toBe(true);
    expect(matcher("search-students-gall--")).toBe(true);
  });

  it("reloads the targets when the backend refuses the target", async () => {
    mocks.toGroup.mockRejectedValue(
      new ApiError("API error (409): ended", 409, { code: "general.conflict" }),
    );
    const onTargetRefused = vi.fn();
    const { result } = renderHook(() =>
      useSchoolCheckinMode({ target: mensa, onTargetRefused }),
    );

    await act(async () => {
      await result.current.runBulk("in", ["1"]);
    });

    expect(onTargetRefused).toHaveBeenCalledOnce();
    expect(mocks.show).toHaveBeenCalledOnce();
  });
});
