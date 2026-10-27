import { renderHook, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import deMessages from "~/i18n/messages/de.json";
import * as parentApi from "~/lib/parent-api";
import type { ChildCareSchedule, ChildFeatures } from "~/lib/parent-api";
import { useChildCare } from "./child-care";
import { useChildMasterData } from "./child-master-data";
import {
  ChildRequestsProvider,
  useChildRequests,
} from "./child-requests-context";

vi.mock("~/lib/parent-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-api")>()),
}));

const features = { sick_note_enabled: true } as unknown as ChildFeatures;
const schedule = {
  weekdays: [],
  today_absent: false,
} as unknown as ChildCareSchedule;

function withProvider({ children }: { children: ReactNode }) {
  return (
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ChildRequestsProvider>{children}</ChildRequestsProvider>
    </NextIntlClientProvider>
  );
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

beforeEach(() => {
  vi.restoreAllMocks();
});

describe("ChildRequestsProvider (#4994)", () => {
  it("teilt laufende Abrufe je Kind und Endpunkt", async () => {
    const getFeatures = vi
      .spyOn(parentApi, "getChildFeatures")
      .mockResolvedValue(features);
    const getSchedule = vi
      .spyOn(parentApi, "getChildCareSchedule")
      .mockResolvedValue(schedule);
    const { result } = renderHook(() => useChildRequests(), {
      wrapper: withProvider,
    });

    const first = result.current.features("42");
    expect(result.current.features("42")).toBe(first);
    void result.current.features("43");
    void result.current.careSchedule("42");
    void result.current.careSchedule("42");
    await first;

    expect(getFeatures).toHaveBeenCalledTimes(2);
    expect(getFeatures).toHaveBeenCalledWith("42");
    expect(getFeatures).toHaveBeenCalledWith("43");
    expect(getSchedule).toHaveBeenCalledTimes(1);
  });

  it("holt nach dem Ende eines Abrufs wieder frisch", async () => {
    const getFeatures = vi
      .spyOn(parentApi, "getChildFeatures")
      .mockResolvedValue(features);
    const { result } = renderHook(() => useChildRequests(), {
      wrapper: withProvider,
    });

    await result.current.features("42");
    await result.current.features("42");

    expect(getFeatures).toHaveBeenCalledTimes(2);
  });

  it("gibt einen Fehler an alle Wartenden und versucht danach neu", async () => {
    const failed = deferred<ChildFeatures>();
    const getFeatures = vi
      .spyOn(parentApi, "getChildFeatures")
      .mockReturnValueOnce(failed.promise)
      .mockResolvedValueOnce(features);
    const { result } = renderHook(() => useChildRequests(), {
      wrapper: withProvider,
    });

    const first = result.current.features("42");
    const second = result.current.features("42");
    failed.reject(new Error("offline"));

    await expect(first).rejects.toThrow("offline");
    await expect(second).rejects.toThrow("offline");
    await expect(result.current.features("42")).resolves.toBe(features);
    expect(getFeatures).toHaveBeenCalledTimes(2);
  });

  it("holt ohne Provider direkt", async () => {
    const getFeatures = vi
      .spyOn(parentApi, "getChildFeatures")
      .mockResolvedValue(features);
    const { result } = renderHook(() => useChildRequests());

    await Promise.all([
      result.current.features("42"),
      result.current.features("42"),
    ]);

    expect(getFeatures).toHaveBeenCalledTimes(2);
  });

  // Betreuung und Angaben brauchen beide die Schulschalter; auf einer
  // Kindseite holt sie nur ein Abruf.
  it("laedt die Schulschalter fuer Betreuung und Angaben einmal", async () => {
    const getFeatures = vi
      .spyOn(parentApi, "getChildFeatures")
      .mockResolvedValue(features);
    const getSchedule = vi
      .spyOn(parentApi, "getChildCareSchedule")
      .mockResolvedValue(schedule);
    vi.spyOn(parentApi, "getChildMasterData").mockResolvedValue(
      {} as Awaited<ReturnType<typeof parentApi.getChildMasterData>>,
    );
    vi.spyOn(parentApi, "listSickDays").mockResolvedValue([]);
    vi.spyOn(parentApi, "listExcusedRequests").mockResolvedValue([]);
    vi.spyOn(parentApi, "listCareExceptions").mockResolvedValue([]);
    vi.spyOn(parentApi, "listPickupChangeRequests").mockResolvedValue([]);

    const { result } = renderHook(
      () => ({
        care: useChildCare("42"),
        masterData: useChildMasterData("42"),
      }),
      { wrapper: withProvider },
    );

    await waitFor(() => {
      expect(result.current.care.loading).toBe(false);
      expect(result.current.masterData.loading).toBe(false);
    });
    expect(result.current.masterData.features).toBe(features);
    expect(result.current.care.features).toBe(features);
    expect(getFeatures).toHaveBeenCalledTimes(1);
    expect(getSchedule).toHaveBeenCalledTimes(1);
  });
});
