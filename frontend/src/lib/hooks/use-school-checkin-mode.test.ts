import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { mockSchoolCheckinStudent, mockGlobalMutate, mockToastError } =
  vi.hoisted(() => ({
    mockSchoolCheckinStudent: vi.fn(),
    mockGlobalMutate: vi.fn(),
    mockToastError: vi.fn(),
  }));

vi.mock("~/lib/student-api", () => ({
  schoolCheckinStudent: mockSchoolCheckinStudent,
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

import {
  deriveCheckinState,
  useSchoolCheckinMode,
} from "./use-school-checkin-mode";

describe("deriveCheckinState", () => {
  it("maps Anwesend variants to anwesend", () => {
    expect(deriveCheckinState("Anwesend")).toBe("anwesend");
    expect(deriveCheckinState("Anwesend - Raum 101")).toBe("anwesend");
  });

  it("maps Schulhof to schulhof", () => {
    expect(deriveCheckinState("Schulhof")).toBe("schulhof");
  });

  it("maps Zuhause / Abwesend / empty to abwesend", () => {
    expect(deriveCheckinState("Zuhause")).toBe("abwesend");
    expect(deriveCheckinState("Abwesend")).toBe("abwesend");
    expect(deriveCheckinState("")).toBe("abwesend");
    expect(deriveCheckinState(null)).toBe("abwesend");
    expect(deriveCheckinState(undefined)).toBe("abwesend");
  });

  it("maps Unterwegs and room names to anwesend (present in building)", () => {
    // "Unterwegs" = between rooms but still checked in. Toggling must
    // fire checkout — this mapping ensures action='out' via actionForState.
    expect(deriveCheckinState("Unterwegs")).toBe("anwesend");
    expect(deriveCheckinState("Raum 101")).toBe("anwesend");
  });
});

describe("useSchoolCheckinMode", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGlobalMutate.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("starts inactive with an empty pendingIds set", () => {
    const { result } = renderHook(() => useSchoolCheckinMode());
    expect(result.current.isActive).toBe(false);
    expect(result.current.pendingIds.size).toBe(0);
  });

  it("toggleActive flips the flag", () => {
    const { result } = renderHook(() => useSchoolCheckinMode());

    act(() => result.current.toggleActive());
    expect(result.current.isActive).toBe(true);

    act(() => result.current.toggleActive());
    expect(result.current.isActive).toBe(false);
  });

  it("applies batched toggles in order and resets on re-entry", async () => {
    mockSchoolCheckinStudent.mockResolvedValueOnce({
      studentId: 42,
      status: "checked_in",
      location: "Anwesend",
      changed: true,
    });
    const { result } = renderHook(() => useSchoolCheckinMode());

    act(() => result.current.toggleActive());
    await act(async () => {
      await result.current.toggle("42", "abwesend");
    });
    expect(result.current.isActive).toBe(true);
    expect(result.current.successCount).toBe(1);

    act(() => {
      result.current.toggleActive();
      result.current.toggleActive();
    });

    expect(result.current.isActive).toBe(true);
    expect(result.current.successCount).toBe(0);
  });

  it("deactivate forces isActive false", () => {
    const { result } = renderHook(() => useSchoolCheckinMode());
    act(() => result.current.toggleActive());
    expect(result.current.isActive).toBe(true);

    act(() => result.current.deactivate());
    expect(result.current.isActive).toBe(false);
  });

  it("toggle of an absent student calls the API with action='in'", async () => {
    mockSchoolCheckinStudent.mockResolvedValueOnce({
      studentId: 42,
      status: "checked_in",
      location: "Anwesend",
      changed: true,
    });

    const { result } = renderHook(() => useSchoolCheckinMode());

    await act(async () => {
      await result.current.toggle("42", "abwesend");
    });

    expect(mockSchoolCheckinStudent).toHaveBeenCalledWith("42", "in");
    expect(mockGlobalMutate).toHaveBeenCalledTimes(1);
  });

  it("toggle of a present student calls the API with action='out'", async () => {
    mockSchoolCheckinStudent.mockResolvedValueOnce({
      studentId: 42,
      status: "checked_out",
      location: "Abwesend",
      changed: true,
    });

    const { result } = renderHook(() => useSchoolCheckinMode());

    await act(async () => {
      await result.current.toggle("42", "anwesend");
    });

    expect(mockSchoolCheckinStudent).toHaveBeenCalledWith("42", "out");
  });

  it("toggle of a schulhof student calls action='out'", async () => {
    mockSchoolCheckinStudent.mockResolvedValueOnce({
      studentId: 7,
      status: "checked_out",
      location: "Abwesend",
      changed: true,
    });

    const { result } = renderHook(() => useSchoolCheckinMode());
    await act(async () => {
      await result.current.toggle("7", "schulhof");
    });

    expect(mockSchoolCheckinStudent).toHaveBeenCalledWith("7", "out");
  });

  it("tracks pendingIds while a toggle is in flight and clears after", async () => {
    let resolve: ((value: unknown) => void) | undefined;
    mockSchoolCheckinStudent.mockReturnValueOnce(
      new Promise((r) => {
        resolve = r;
      }),
    );

    const { result } = renderHook(() => useSchoolCheckinMode());

    let togglePromise: Promise<void> | undefined;
    act(() => {
      togglePromise = result.current.toggle("99", "abwesend");
    });

    await waitFor(() => {
      expect(result.current.pendingIds.has("99")).toBe(true);
    });

    await act(async () => {
      resolve?.({
        studentId: 99,
        status: "checked_in",
        location: "Anwesend",
        changed: true,
      });
      await togglePromise;
    });

    expect(result.current.pendingIds.has("99")).toBe(false);
  });

  it("shows a German toast on API failure and clears pendingIds", async () => {
    mockSchoolCheckinStudent.mockRejectedValueOnce(new Error("boom"));

    const { result } = renderHook(() => useSchoolCheckinMode());
    await act(async () => {
      await result.current.toggle("5", "abwesend");
    });

    expect(mockToastError).toHaveBeenCalledWith(
      expect.stringContaining("Anmelden fehlgeschlagen"),
    );
    expect(result.current.pendingIds.has("5")).toBe(false);
  });

  it("toast copy matches the attempted direction (out on present)", async () => {
    mockSchoolCheckinStudent.mockRejectedValueOnce(new Error("boom"));

    const { result } = renderHook(() => useSchoolCheckinMode());
    await act(async () => {
      await result.current.toggle("5", "anwesend");
    });

    expect(mockToastError).toHaveBeenCalledWith(
      expect.stringContaining("Abmelden fehlgeschlagen"),
    );
  });

  it("is a no-op when the same student is still pending", async () => {
    let resolve: ((value: unknown) => void) | undefined;
    mockSchoolCheckinStudent.mockReturnValueOnce(
      new Promise((r) => {
        resolve = r;
      }),
    );

    const { result } = renderHook(() => useSchoolCheckinMode());

    let firstToggle: Promise<void> | undefined;
    act(() => {
      firstToggle = result.current.toggle("1", "abwesend");
    });

    // Second call while first is still pending — should be ignored.
    await act(async () => {
      await result.current.toggle("1", "abwesend");
    });

    expect(mockSchoolCheckinStudent).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolve?.({
        studentId: 1,
        status: "checked_in",
        location: "Anwesend",
        changed: true,
      });
      await firstToggle;
    });
  });
});
