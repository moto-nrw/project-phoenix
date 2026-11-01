import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { ParentResourceTestProvider } from "~/test/parent-resource-provider";
import * as api from "~/lib/parent-api";
import { useDiscardRefusedEntity } from "~/lib/swr/hooks";
import {
  useParentFeatures,
  useParentToday,
} from "~/lib/hooks/use-parent-resources";
import { ApiError } from "~/lib/api-error";
import {
  clearRateLimitBackoff,
  rateLimitBlockedError,
  recordRateLimit,
} from "~/lib/rate-limit-backoff";
import { releaseFakeTimers } from "~/test/clock";
import { useChildCare } from "./child-care";
import { useChildMasterData } from "./child-master-data";
import {
  ChildRequestsProvider,
  SharedParentFeaturesProvider,
} from "./child-requests-context";
vi.unmock("swr");
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: () => null }));

const children = [{ student_id: "42", tenant_id: "7" }] as api.Child[];
const features = {
  sick_note_enabled: true,
  master_data_edit_enabled: true,
} as api.ChildFeatures;
function wrapper({ children: content }: Readonly<{ children: ReactNode }>) {
  return (
    <ParentResourceTestProvider>
      <SharedParentFeaturesProvider linkedChildren={children}>
        <ChildRequestsProvider>{content}</ChildRequestsProvider>
      </SharedParentFeaturesProvider>
    </ParentResourceTestProvider>
  );
}
beforeEach(() => {
  vi.restoreAllMocks();
  vi.spyOn(api, "getChildFeatures").mockResolvedValue(features);
  vi.spyOn(api, "getChildMasterData").mockResolvedValue(
    {} as api.ChildMasterData,
  );
  vi.spyOn(api, "getChildCareSchedule").mockResolvedValue({
    weekdays: [],
    today_absent: false,
  } as unknown as api.ChildCareSchedule);
  vi.spyOn(api, "listSickDays").mockResolvedValue([]);
  vi.spyOn(api, "listExcusedRequests").mockResolvedValue([]);
  vi.spyOn(api, "listCareExceptions").mockResolvedValue([]);
  vi.spyOn(api, "listPickupChangeRequests").mockResolvedValue([]);
});
function mount() {
  return renderHook(
    () => ({ care: useChildCare("42"), master: useChildMasterData("42") }),
    { wrapper },
  );
}

describe("shared parent relationship features", () => {
  it("shares the confirmed flags between care and master-data sections", async () => {
    const { result } = mount();
    await waitFor(() =>
      expect(result.current.master.features).toEqual(features),
    );
    expect(result.current.care.features).toEqual(features);
    expect(api.getChildFeatures).toHaveBeenCalledTimes(1);
  });
  it("retries shared flags from care and clears the master-data error after recovery", async () => {
    vi.mocked(api.getChildFeatures).mockRejectedValue(
      new ApiError("offline", 503, { code: "general.unavailable" }),
    );
    const { result } = mount();
    await waitFor(() => expect(result.current.master.error).not.toBeNull());
    expect(result.current.care.features.sick_note_enabled).toBe(false);
    vi.mocked(api.getChildFeatures).mockResolvedValue(features);
    await act(async () => {
      result.current.care.refresh();
    });
    await waitFor(() =>
      expect(result.current.master.features).toEqual(features),
    );
    expect(result.current.master.error).toBeNull();
    expect(result.current.care.features).toEqual(features);
  });
  it("keeps confirmed capabilities when background revalidation fails without access refusal", async () => {
    const { result } = mount();
    await waitFor(() =>
      expect(result.current.master.features).toEqual(features),
    );
    expect(result.current.care.features.sick_note_enabled).toBe(true);
    vi.mocked(api.getChildFeatures).mockRejectedValue(
      new ApiError("offline", 503, { code: "general.unavailable" }),
    );
    await act(async () => {
      result.current.care.refresh();
    });
    await waitFor(() => expect(result.current.master.error).not.toBeNull());
    expect(result.current.master.features).toEqual(features);
    expect(result.current.care.features.sick_note_enabled).toBe(true);
  });
  it("keeps another child's confirmed features when one relationship lookup fails", async () => {
    vi.mocked(api.getChildFeatures).mockImplementation(async (id) => {
      if (id === "42")
        throw new ApiError("offline", 503, { code: "general.unavailable" });
      return features;
    });
    const linked = [
      ...children,
      { student_id: "43", tenant_id: "7" } as api.Child,
    ];
    const { result } = renderHook(
      () => ({
        features: useParentFeatures(linked),
        discard: useDiscardRefusedEntity(),
      }),
      { wrapper: ParentResourceTestProvider },
    );
    await waitFor(() =>
      expect(result.current.features.data?.["43"]).toEqual(features),
    );
    expect(result.current.features.data?.["42"]).toBeUndefined();
    expect(result.current.features.error).toMatchObject({
      code: "general.unavailable",
    });
    vi.mocked(api.getChildFeatures).mockReturnValue(
      new Promise(() => undefined),
    );
    act(() => {
      result.current.discard(
        new ApiError("gone", 403, { code: "care.child_not_linked" }),
        {
          codes: ["care.child_not_linked"],
          matches: (key) => key.startsWith("parent-features-"),
        },
      );
    });
    expect(result.current.features.data).toBeUndefined();
  });
});

describe("shared parent daily status", () => {
  const linked = [
    ...children,
    { student_id: "43", tenant_id: "7" } as api.Child,
  ];
  const present: api.ChildToday = { at_ogs: true, state: "present" };
  const left: api.ChildToday = { at_ogs: false, state: "left" };
  const failure = new ApiError("offline", 503, { code: "general.unavailable" });

  it("keeps successful sibling statuses and exposes cold-load failures", async () => {
    vi.spyOn(api, "getChildToday").mockImplementation(async (id) => {
      if (id === "42") throw failure;
      return present;
    });
    const { result } = renderHook(() => useParentToday(linked), {
      wrapper: ParentResourceTestProvider,
    });
    await waitFor(() => expect(result.current.error).toBe(failure));
    expect(result.current.data).toEqual({
      "42": api.UNKNOWN_CHILD_TODAY,
      "43": present,
    });
  });

  it("keeps confirmed attendance for failed children while updating successful siblings", async () => {
    const request = vi.spyOn(api, "getChildToday").mockResolvedValue(present);
    const { result } = renderHook(() => useParentToday(linked), {
      wrapper: ParentResourceTestProvider,
    });
    await waitFor(() => expect(result.current.data?.["42"]).toEqual(present));
    request.mockImplementation(async (id) => {
      if (id === "42") throw failure;
      return left;
    });
    await act(async () => {
      await result.current.mutate();
    });
    expect(result.current.error).toBe(failure);
    expect(result.current.data).toEqual({
      "42": present,
      "43": left,
    });
    request.mockResolvedValue(left);
    await act(async () => {
      await result.current.mutate();
    });
    expect(result.current.error).toBeUndefined();
    expect(result.current.data).toEqual({ "42": left, "43": left });
  });

  it("handles a sibling access refusal even when another status request also fails", async () => {
    const request = vi.spyOn(api, "getChildToday").mockResolvedValue(present);
    const { result } = renderHook(
      () => ({
        today: useParentToday(linked),
        features: useParentFeatures(linked),
      }),
      { wrapper: ParentResourceTestProvider },
    );
    await waitFor(() =>
      expect(result.current.features.data?.["43"]).toEqual(features),
    );
    await waitFor(() =>
      expect(result.current.today.data?.["43"]).toEqual(present),
    );
    vi.mocked(api.getChildFeatures).mockReturnValue(
      new Promise(() => undefined),
    );
    request.mockImplementation(async (id) => {
      if (id === "42") throw failure;
      throw new ApiError("gone", 403, { code: "care.child_not_linked" });
    });
    await act(async () => {
      await result.current.today.mutate();
    });
    expect(result.current.features.data).toBeUndefined();
    expect(result.current.today.data).toBeUndefined();
  });

  it("withdraws partial statuses when relationship access is refused", async () => {
    vi.spyOn(api, "getChildToday").mockImplementation(async (id) => {
      if (id === "42") throw failure;
      return present;
    });
    const { result } = renderHook(
      () => ({
        today: useParentToday(linked),
        discard: useDiscardRefusedEntity(),
      }),
      { wrapper: ParentResourceTestProvider },
    );
    await waitFor(() =>
      expect(result.current.today.data?.["43"]).toEqual(present),
    );
    vi.mocked(api.getChildToday).mockReturnValue(new Promise(() => undefined));
    act(() => {
      result.current.discard(
        new ApiError("gone", 403, { code: "care.child_not_linked" }),
        {
          codes: ["care.child_not_linked"],
          matches: (key) => key.startsWith("parent-today-"),
        },
      );
    });
    expect(result.current.today.data).toBeUndefined();
  });
});

describe("shared parent rate-limit recovery", () => {
  afterEach(() => {
    clearRateLimitBackoff();
    releaseFakeTimers();
  });

  it("automatically restores features and daily statuses after the read quota recovers", async () => {
    vi.useFakeTimers();
    recordRateLimit("10", "GET");
    vi.mocked(api.getChildFeatures).mockImplementation(async () => {
      const blocked = rateLimitBlockedError("GET");
      if (blocked) throw blocked;
      return features;
    });
    const present: api.ChildToday = { at_ogs: true, state: "present" };
    vi.spyOn(api, "getChildToday").mockImplementation(async () => {
      const blocked = rateLimitBlockedError("GET");
      if (blocked) throw blocked;
      return present;
    });
    const { result } = renderHook(
      () => ({
        features: useParentFeatures(children),
        today: useParentToday(children),
      }),
      { wrapper: ParentResourceTestProvider },
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(result.current.features.error).toMatchObject({ status: 429 });
    expect(result.current.today.error).toMatchObject({ status: 429 });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_050);
    });
    expect(result.current.features.data).toEqual({ "42": features });
    expect(result.current.today.data).toEqual({ "42": present });
    expect(result.current.features.error).toBeUndefined();
    expect(result.current.today.error).toBeUndefined();
  });
});
