import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Student } from "~/lib/api";
import type {
  SchoolCheckinAction,
  SchoolCheckinBatchOutcome,
} from "~/lib/student-api";
import { useBulkCheckinActions } from "./use-bulk-checkin-actions";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

const selectedStudent = {
  id: "1",
  name: "Max Mustermann",
  first_name: "Max",
  second_name: "Mustermann",
  current_location: "Zuhause",
} as Student;

describe("useBulkCheckinActions", () => {
  it("keeps the completion callback from the scope where the run started", async () => {
    const outcome = deferred<SchoolCheckinBatchOutcome>();
    const runBulk = vi.fn(() => outcome.promise);
    const firstScopeProcessed = vi.fn();
    const secondScopeProcessed = vi.fn();
    const schoolCheckin = {
      runBulk,
    } as unknown as Parameters<typeof useBulkCheckinActions>[0];
    const { result, rerender } = renderHook(
      ({
        onRunProcessed,
      }: {
        onRunProcessed: (action: SchoolCheckinAction) => void;
      }) =>
        useBulkCheckinActions(schoolCheckin, [selectedStudent], onRunProcessed),
      { initialProps: { onRunProcessed: firstScopeProcessed } },
    );

    act(() => result.current.handleBulkAction("in"));
    rerender({ onRunProcessed: secondScopeProcessed });

    await act(async () => {
      outcome.resolve({ action: "in", results: [], succeeded: 1, failed: 0 });
      await outcome.promise;
    });

    expect(firstScopeProcessed).toHaveBeenCalledWith("in");
    expect(secondScopeProcessed).not.toHaveBeenCalled();
  });
});
