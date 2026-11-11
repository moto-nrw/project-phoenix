import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import type { TraegerMasterData } from "~/lib/traeger-api";
import { deferred } from "~/test/helpers/form-modal-pending";
import { useTraegerMasterData } from "./use-traeger-master-data";

const { fetchMasterData } = vi.hoisted(() => ({ fetchMasterData: vi.fn() }));
vi.mock("~/lib/traeger-api", () => ({
  fetchTraegerMasterData: fetchMasterData,
}));

const initial: TraegerMasterData = {
  contact: { name: "Petra Lindner", phone: "", email: "alt@example.test" },
  withdrawal: { phone: "", email: "alt@example.test" },
  logo: null,
  bankAccounts: [],
};

describe("useTraegerMasterData", () => {
  it.each([false, true])(
    "ignores an older contact response (failure: %s)",
    async (fails) => {
      fetchMasterData.mockReset().mockResolvedValueOnce(initial);
      const { result } = renderHook(() =>
        useTraegerMasterData("die Kontaktdaten"),
      );
      await waitFor(() => expect(result.current.loaded).toBe(true));
      const older = deferred<TraegerMasterData>();
      const newer = deferred<TraegerMasterData>();
      fetchMasterData
        .mockReturnValueOnce(older.promise)
        .mockReturnValueOnce(newer.promise);
      let first!: Promise<void>;
      let second!: Promise<void>;
      act(() => {
        first = result.current.refresh();
        second = result.current.refresh();
      });
      const latest = {
        ...initial,
        contact: { ...initial.contact, email: "neu@example.test" },
        withdrawal: { ...initial.withdrawal, email: "neu@example.test" },
      };
      await act(async () => {
        newer.resolve(latest);
        await second;
      });
      expect(result.current.masterData).toEqual(latest);
      await act(async () => {
        if (fails) older.reject(new ApiError("down", 503));
        else older.resolve(initial);
        await first;
      });
      expect(result.current.masterData).toEqual(latest);
      expect(result.current.failure).toBeUndefined();
    },
  );
});
