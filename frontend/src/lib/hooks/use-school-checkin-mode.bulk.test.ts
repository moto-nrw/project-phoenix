import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockSchoolCheckinStudentsBatch, mockGlobalMutate, mockToastError } =
  vi.hoisted(() => ({
    mockSchoolCheckinStudentsBatch: vi.fn(),
    mockGlobalMutate: vi.fn(),
    mockToastError: vi.fn(),
  }));

vi.mock("~/lib/student-api", () => ({
  schoolCheckinStudent: vi.fn(),
  schoolCheckinStudentsBatch: mockSchoolCheckinStudentsBatch,
}));

vi.mock("swr", () => ({
  mutate: mockGlobalMutate,
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: vi.fn(),
    error: mockToastError,
    info: vi.fn(),
    warning: vi.fn(),
    remove: vi.fn(),
  }),
}));

import { useSchoolCheckinMode } from "./use-school-checkin-mode";

// Selection sub-mode + bulk execution (#2359).
describe("useSchoolCheckinMode selection sub-mode", () => {
  beforeEach(() => {
    mockSchoolCheckinStudentsBatch.mockReset();
    mockGlobalMutate.mockReset().mockResolvedValue(undefined);
    mockToastError.mockReset();
  });

  it("marks and unmarks students without calling any API", () => {
    const { result } = renderHook(() => useSchoolCheckinMode());

    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));
    act(() => result.current.toggleSelected("1"));
    act(() => result.current.toggleSelected("2"));

    expect(result.current.selectedIds.has("1")).toBe(true);
    expect(result.current.selectedIds.has("2")).toBe(true);
    expect(mockSchoolCheckinStudentsBatch).not.toHaveBeenCalled();

    act(() => result.current.toggleSelected("1"));
    expect(result.current.selectedIds.has("1")).toBe(false);
    expect(result.current.selectedIds.size).toBe(1);
  });

  it("clears the selection when the sub-mode is switched either way", () => {
    const { result } = renderHook(() => useSchoolCheckinMode());

    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));
    act(() => result.current.toggleSelected("1"));
    act(() => result.current.setSelectionActive(false));

    expect(result.current.selectedIds.size).toBe(0);
  });

  it("clears selection state when the mode is left", () => {
    const { result } = renderHook(() => useSchoolCheckinMode());

    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));
    act(() => result.current.toggleSelected("1"));
    act(() => result.current.toggleActive());

    expect(result.current.selectionActive).toBe(false);
    expect(result.current.selectedIds.size).toBe(0);
  });

  it("runBulk sends the whole selection, bundles one invalidation, and drops processed students from the selection", async () => {
    mockSchoolCheckinStudentsBatch.mockResolvedValue({
      action: "out",
      succeeded: 2,
      failed: 0,
      results: [
        { studentId: "1", ok: true, changed: true, location: "Abwesend" },
        { studentId: "2", ok: true, changed: false, location: "Abwesend" },
      ],
    });

    const { result } = renderHook(() => useSchoolCheckinMode());
    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));
    act(() => result.current.toggleSelected("1"));
    act(() => result.current.toggleSelected("2"));

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.runBulk("out");
    });

    expect(mockSchoolCheckinStudentsBatch).toHaveBeenCalledTimes(1);
    expect(mockSchoolCheckinStudentsBatch).toHaveBeenCalledWith(
      expect.arrayContaining(["1", "2"]),
      "out",
    );
    // Exactly ONE bundled revalidation for the whole batch — not per student.
    expect(mockGlobalMutate).toHaveBeenCalledTimes(1);
    expect(outcome).toMatchObject({ succeeded: 2, failed: 0 });
    // Both processed → selection empty; counter reflects the batch.
    expect(result.current.selectedIds.size).toBe(0);
    expect(result.current.successCount).toBe(2);
    expect(result.current.isBulkRunning).toBe(false);
  });

  it("keeps only the failed students selected after a partial failure", async () => {
    mockSchoolCheckinStudentsBatch.mockResolvedValue({
      action: "out",
      succeeded: 1,
      failed: 1,
      results: [
        { studentId: "1", ok: true, changed: true, location: "Abwesend" },
        { studentId: "2", ok: false, changed: false, error: "not_found" },
      ],
    });

    const { result } = renderHook(() => useSchoolCheckinMode());
    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));
    act(() => result.current.toggleSelected("1"));
    act(() => result.current.toggleSelected("2"));

    await act(async () => {
      await result.current.runBulk("out");
    });

    expect(result.current.selectedIds.has("1")).toBe(false);
    expect(result.current.selectedIds.has("2")).toBe(true);
    expect(result.current.successCount).toBe(1);
  });

  it("clearSelection empties the selection", () => {
    const { result } = renderHook(() => useSchoolCheckinMode());

    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));
    act(() => result.current.toggleSelected("1"));
    act(() => result.current.toggleSelected("2"));
    act(() => result.current.clearSelection());

    expect(result.current.selectedIds.size).toBe(0);
  });

  it("runBulk executes an explicit id list exactly, even after a committed scope clear", async () => {
    mockSchoolCheckinStudentsBatch.mockResolvedValue({
      action: "out",
      succeeded: 2,
      failed: 0,
      results: [
        { studentId: "1", ok: true, changed: true, location: "Abwesend" },
        { studentId: "3", ok: true, changed: true, location: "Abwesend" },
      ],
    });

    const { result } = renderHook(() => useSchoolCheckinMode());
    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));
    act(() => result.current.toggleSelected("1"));
    act(() => result.current.toggleSelected("2"));
    // A search/filter change committed between snapshot and execution (e.g.
    // while the failure dialog is open) empties the selection. The explicit
    // list is a dialog's displayed snapshot and must still run exactly as
    // displayed — intersecting with the now-empty selection would make the
    // promised retry a silent no-op (review #2372).
    act(() => result.current.clearSelection());
    expect(result.current.selectedIds.size).toBe(0);

    await act(async () => {
      await result.current.runBulk("out", ["1", "3"]);
    });

    expect(mockSchoolCheckinStudentsBatch).toHaveBeenCalledWith(
      ["1", "3"],
      "out",
    );
    // Processed students are subtracted from the (already empty) selection.
    expect(result.current.selectedIds.size).toBe(0);
  });

  it("keeps a selection made while the bulk request is in flight", async () => {
    let resolveBatch!: (value: unknown) => void;
    mockSchoolCheckinStudentsBatch.mockReturnValue(
      new Promise((resolve) => {
        resolveBatch = resolve;
      }),
    );

    const { result } = renderHook(() => useSchoolCheckinMode());
    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));
    act(() => result.current.toggleSelected("1"));

    let runPromise: Promise<unknown> = Promise.resolve(null);
    act(() => {
      runPromise = result.current.runBulk("out");
    });
    // Mid-flight mark: must survive the response processing (review #2372 —
    // replacing the set with the snapshot's failures would discard it).
    act(() => result.current.toggleSelected("3"));

    await act(async () => {
      resolveBatch({
        action: "out",
        succeeded: 1,
        failed: 0,
        results: [
          { studentId: "1", ok: true, changed: true, location: "Abwesend" },
        ],
      });
      await runPromise;
    });

    expect(result.current.selectedIds.has("3")).toBe(true);
    expect(result.current.selectedIds.has("1")).toBe(false);
  });

  it("ignores mode exit and sub-mode switch while a bulk request is in flight", async () => {
    let resolveBatch!: (value: unknown) => void;
    mockSchoolCheckinStudentsBatch.mockReturnValue(
      new Promise((resolve) => {
        resolveBatch = resolve;
      }),
    );

    const { result } = renderHook(() => useSchoolCheckinMode());
    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));
    act(() => result.current.toggleSelected("1"));
    act(() => result.current.toggleSelected("2"));

    let runPromise: Promise<unknown> = Promise.resolve(null);
    act(() => {
      runPromise = result.current.runBulk("out");
    });

    // "Fertig" (toggleActive/deactivate) and the Sofort|Auswahl switch would
    // all clear selectedIds — mid-flight they must be ignored so the failed
    // students can stay marked for the retry (review #2372).
    act(() => result.current.toggleActive());
    act(() => result.current.deactivate());
    act(() => result.current.setSelectionActive(false));

    expect(result.current.isActive).toBe(true);
    expect(result.current.selectionActive).toBe(true);
    expect(result.current.selectedIds.size).toBe(2);

    await act(async () => {
      resolveBatch({
        action: "out",
        succeeded: 1,
        failed: 1,
        results: [
          { studentId: "1", ok: true, changed: true, location: "Abwesend" },
          { studentId: "2", ok: false, changed: false, error: "not_found" },
        ],
      });
      await runPromise;
    });

    // Failed student kept its mark; after the run the exit works again.
    expect(result.current.selectedIds.has("2")).toBe(true);
    act(() => result.current.toggleActive());
    expect(result.current.isActive).toBe(false);
    expect(result.current.selectedIds.size).toBe(0);
  });

  it("defers a clearSelection issued during the run and keeps only the failed students", async () => {
    let resolveBatch!: (value: unknown) => void;
    mockSchoolCheckinStudentsBatch.mockReturnValue(
      new Promise((resolve) => {
        resolveBatch = resolve;
      }),
    );

    const { result } = renderHook(() => useSchoolCheckinMode());
    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));
    act(() => result.current.toggleSelected("1"));
    act(() => result.current.toggleSelected("2"));

    let runPromise: Promise<unknown> = Promise.resolve(null);
    act(() => {
      runPromise = result.current.runBulk("out");
    });

    // The page fires clearSelection on every search/filter scope change.
    // Mid-flight it must not touch the selection the response will be
    // processed against (review #2372)…
    act(() => result.current.clearSelection());
    expect(result.current.selectedIds.size).toBe(2);
    // A mark added after the deferred clear belongs to the replaced view too.
    act(() => result.current.toggleSelected("3"));

    await act(async () => {
      resolveBatch({
        action: "out",
        succeeded: 1,
        failed: 1,
        results: [
          { studentId: "1", ok: true, changed: true, location: "Abwesend" },
          { studentId: "2", ok: false, changed: false, error: "not_found" },
        ],
      });
      await runPromise;
    });

    // …once the run has settled the scope change drops every mark EXCEPT the
    // failed student: the failure dialog promises a one-tap retry for them
    // (review #2372).
    expect(result.current.selectedIds.has("2")).toBe(true);
    expect(result.current.selectedIds.size).toBe(1);
  });

  it("applies a deferred clear fully after a clean run and after a total failure keeps the run", async () => {
    // Clean run: nothing failed, so the deferred scope clear empties all.
    mockSchoolCheckinStudentsBatch.mockReturnValueOnce(
      new Promise((resolve) => {
        setTimeout(
          () =>
            resolve({
              action: "out",
              succeeded: 1,
              failed: 0,
              results: [
                {
                  studentId: "1",
                  ok: true,
                  changed: true,
                  location: "Abwesend",
                },
              ],
            }),
          0,
        );
      }),
    );

    const { result } = renderHook(() => useSchoolCheckinMode());
    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));
    act(() => result.current.toggleSelected("1"));

    let runPromise: Promise<unknown> = Promise.resolve(null);
    act(() => {
      runPromise = result.current.runBulk("out");
    });
    act(() => result.current.clearSelection());
    await act(async () => {
      await runPromise;
    });
    expect(result.current.selectedIds.size).toBe(0);

    // Total failure: the whole run counts as failed — the retry toast
    // promises another attempt, so the deferred clear keeps the run marked.
    mockSchoolCheckinStudentsBatch.mockRejectedValueOnce(
      new Error("API error (500): boom"),
    );
    act(() => result.current.toggleSelected("4"));
    act(() => {
      runPromise = result.current.runBulk("out");
    });
    act(() => result.current.clearSelection());
    await act(async () => {
      await runPromise;
    });
    expect(result.current.selectedIds.has("4")).toBe(true);
    expect(result.current.selectedIds.size).toBe(1);
  });

  it("invalidates exactly the presence-bearing SWR key families, bundled", async () => {
    mockSchoolCheckinStudentsBatch.mockResolvedValue({
      action: "in",
      succeeded: 1,
      failed: 0,
      results: [
        { studentId: "1", ok: true, changed: true, location: "Anwesend" },
      ],
    });

    const { result } = renderHook(() => useSchoolCheckinMode());
    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));
    act(() => result.current.toggleSelected("1"));
    await act(async () => {
      await result.current.runBulk("in");
    });

    const matcher = mockGlobalMutate.mock.calls[0]?.[0] as (
      key: unknown,
    ) => boolean;
    expect(matcher("search-students-gall--")).toBe(true);
    expect(matcher("ogs-students-7")).toBe(true);
    expect(matcher("tracking-supervisions-2026-08-17")).toBe(true);
    expect(matcher("tracking-indicators-2026-08-17")).toBe(true);
    expect(matcher("student-detail-42")).toBe(true);
    expect(matcher("rooms-list")).toBe(false);
    expect(matcher(["not", "a", "string"])).toBe(false);
  });

  it("toasts the generic retry message when the whole request fails without a 403", async () => {
    mockSchoolCheckinStudentsBatch.mockRejectedValue(
      new Error("API error (500): boom"),
    );

    const { result } = renderHook(() => useSchoolCheckinMode());
    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));
    act(() => result.current.toggleSelected("1"));

    await act(async () => {
      await result.current.runBulk("in");
    });

    expect(mockToastError).toHaveBeenCalledWith(
      "Anmelden fehlgeschlagen. Bitte erneut versuchen.",
    );
    expect(result.current.selectedIds.has("1")).toBe(true);
  });

  it("returns null and keeps the selection when the whole request fails", async () => {
    mockSchoolCheckinStudentsBatch.mockRejectedValue(
      new Error("API error (403): Forbidden"),
    );

    const { result } = renderHook(() => useSchoolCheckinMode());
    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));
    act(() => result.current.toggleSelected("1"));

    let outcome: unknown = "sentinel";
    await act(async () => {
      outcome = await result.current.runBulk("out");
    });

    expect(outcome).toBeNull();
    expect(result.current.selectedIds.has("1")).toBe(true);
    expect(mockToastError).toHaveBeenCalledWith(
      "Keine Berechtigung, Kinder abzumelden.",
    );
    expect(result.current.isBulkRunning).toBe(false);
  });

  it("runBulk is a no-op with an empty selection", async () => {
    const { result } = renderHook(() => useSchoolCheckinMode());
    act(() => result.current.toggleActive());
    act(() => result.current.setSelectionActive(true));

    let outcome: unknown = "sentinel";
    await act(async () => {
      outcome = await result.current.runBulk("in");
    });

    expect(outcome).toBeNull();
    expect(mockSchoolCheckinStudentsBatch).not.toHaveBeenCalled();
  });
});
