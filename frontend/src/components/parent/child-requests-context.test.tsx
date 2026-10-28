import { act, renderHook, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import deMessages from "~/i18n/messages/de.generated";
import * as parentApi from "~/lib/parent-api";
import type { ChildCareSchedule, ChildFeatures } from "~/lib/parent-api";
import { useChildCare } from "./child-care";
import { useChildMasterData } from "./child-master-data";
import { BookedCareSection } from "./child/booked-care-section";
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
  it.each([
    ["parent-conversation-refresh", { studentId: "42" }, false],
    ["parent-conversation-refresh", { studentId: null }, false],
    ["parent-conversation-refresh", undefined, false],
    ["focus", undefined, false],
    ["parent-conversation-refresh", { studentId: "42" }, true],
  ])(
    "erneuert laufende Abrufe nach %s (%j, Fehler: %s)",
    async (eventName, detail, fails) => {
      const oldFeatures = deferred<ChildFeatures>();
      const getFeatures = vi
        .spyOn(parentApi, "getChildFeatures")
        .mockReturnValueOnce(oldFeatures.promise)
        .mockResolvedValue(features);
      const { result } = renderHook(() => useChildRequests(), {
        wrapper: withProvider,
      });
      const first = result.current.features("42");
      window.dispatchEvent(new CustomEvent(eventName, { detail }));
      // Multiple consumers and notifications still share one follow-up read.
      window.dispatchEvent(new CustomEvent(eventName, { detail }));
      const second = result.current.features("42");
      expect(second).toBe(first);
      if (fails) oldFeatures.reject(new Error("offline"));
      else oldFeatures.resolve({ ...features, sick_note_enabled: false });

      await expect(first).resolves.toBe(features);
      await expect(second).resolves.toBe(features);
      expect(getFeatures).toHaveBeenCalledTimes(2);
      await result.current.features("42");
      expect(getFeatures).toHaveBeenCalledTimes(3);
    },
  );

  it("erneuert keine Abrufe für ein anderes Kind und entfernt den Listener beim Abbau", async () => {
    const pending = deferred<ChildCareSchedule>();
    const getSchedule = vi
      .spyOn(parentApi, "getChildCareSchedule")
      .mockReturnValue(pending.promise);
    const { result, unmount } = renderHook(() => useChildRequests(), {
      wrapper: withProvider,
    });
    const first = result.current.careSchedule("42");
    window.dispatchEvent(
      new CustomEvent("parent-conversation-refresh", {
        detail: { studentId: "43" },
      }),
    );
    unmount();
    window.dispatchEvent(new CustomEvent("parent-conversation-refresh"));
    window.dispatchEvent(new Event("focus"));
    pending.resolve(schedule);

    await expect(first).resolves.toBe(schedule);
    expect(getSchedule).toHaveBeenCalledTimes(1);
  });

  it("erneuert keine Abrufe, wenn nur ein Eingabefeld den Fokus bekommt", async () => {
    const pending = deferred<ChildCareSchedule>();
    const getSchedule = vi
      .spyOn(parentApi, "getChildCareSchedule")
      .mockReturnValue(pending.promise);
    const { result } = renderHook(() => useChildRequests(), {
      wrapper: withProvider,
    });
    const input = document.createElement("input");
    document.body.append(input);
    const first = result.current.careSchedule("42");
    input.focus();
    pending.resolve(schedule);

    await expect(first).resolves.toBe(schedule);
    expect(getSchedule).toHaveBeenCalledTimes(1);
    input.remove();
  });

  it("liefert beiden Betreuungsabschnitten nach einer Änderung frische Daten", async () => {
    const oldSchedule = deferred<ChildCareSchedule>();
    const previousSchedule = {
      ...schedule,
      can_request: false,
      request_capabilities: {
        arrival: false,
        pickup: false,
        departure_mode: false,
      },
    };
    const freshSchedule = {
      ...previousSchedule,
      today_absent: true,
      weekdays: [
        { weekday: 1, status: "scheduled", pickup: "16:30", modes: ["pickup"] },
      ],
    } as ChildCareSchedule;
    const getSchedule = vi
      .spyOn(parentApi, "getChildCareSchedule")
      .mockReturnValueOnce(oldSchedule.promise)
      .mockResolvedValue(freshSchedule);
    vi.spyOn(parentApi, "getChildFeatures").mockResolvedValue(features);
    vi.spyOn(parentApi, "getChildCareOfferings").mockResolvedValue({
      offerings: [],
      can_request: false,
    } as unknown as Awaited<
      ReturnType<typeof parentApi.getChildCareOfferings>
    >);
    vi.spyOn(parentApi, "listSickDays").mockResolvedValue([]);
    vi.spyOn(parentApi, "listExcusedRequests").mockResolvedValue([]);
    vi.spyOn(parentApi, "listCareExceptions").mockResolvedValue([]);
    vi.spyOn(parentApi, "listPickupChangeRequests").mockResolvedValue([]);

    const { result } = renderHook(() => useChildCare("42"), {
      wrapper: ({ children }) => (
        <NextIntlClientProvider locale="de" messages={deMessages}>
          <ChildRequestsProvider>
            {children}
            <BookedCareSection
              studentId="42"
              childFirstName="Mia"
              careEnded={false}
            />
          </ChildRequestsProvider>
        </NextIntlClientProvider>
      ),
    });
    expect(getSchedule).toHaveBeenCalledTimes(1);
    act(() => {
      window.dispatchEvent(
        new CustomEvent("parent-conversation-refresh", {
          detail: { studentId: "42" },
        }),
      );
    });
    await act(async () => {
      oldSchedule.resolve(previousSchedule);
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
      expect(result.current.todayPickup).toEqual({ kind: "absent" });
      expect(screen.getByText("16:30 Uhr")).toBeInTheDocument();
    });
    expect(getSchedule).toHaveBeenCalledTimes(2);
  });

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
