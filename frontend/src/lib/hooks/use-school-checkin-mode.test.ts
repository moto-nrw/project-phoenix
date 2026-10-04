import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";

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

// The shared display path, reduced to what reaches the toast: the catalog
// text for the error and the named object.
const showApiError = vi.hoisted(
  () =>
    async (error: unknown, options: { object: string; retry?: () => void }) => {
      const { presentError } = await import("~/lib/error-presentation");
      mockToastError(presentError(error, options.object).message, {
        retry: options.retry,
      });
    },
);

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: vi.fn(),
    error: mockToastError,
    info: vi.fn(),
    warning: vi.fn(),
    remove: vi.fn(),
  }),
  useApiErrorDisplay: () => ({ show: showApiError }),
}));

import {
  checkoutConfirmationRoom,
  deriveCheckinState,
  useSchoolCheckinMode,
} from "./use-school-checkin-mode";

describe("checkoutConfirmationRoom", () => {
  it("returns the room of a detailed-mode student sitting in one", () => {
    expect(checkoutConfirmationRoom("Anwesend - Raum 101")).toBe("Raum 101");
    expect(checkoutConfirmationRoom("Anwesend - Mensa")).toBe("Mensa");
  });

  it("returns null for present-but-roomless states", () => {
    // Roomless presence ends no visit, so a checkout needs no warning.
    expect(checkoutConfirmationRoom("Anwesend")).toBeNull();
    expect(checkoutConfirmationRoom("Unterwegs")).toBeNull();
  });

  it("returns null for every binary-mode label", () => {
    expect(checkoutConfirmationRoom("Schulhof")).toBeNull();
    expect(checkoutConfirmationRoom("Zuhause")).toBeNull();
    expect(checkoutConfirmationRoom("Abwesend")).toBeNull();
  });

  it("returns null for absent or unknown students", () => {
    expect(checkoutConfirmationRoom("")).toBeNull();
    expect(checkoutConfirmationRoom(null)).toBeNull();
    expect(checkoutConfirmationRoom(undefined)).toBeNull();
  });
});

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

  // #3260: a child still in class has not checked in, so the toggle must
  // check it in, never out.
  it("maps Schule to abwesend", () => {
    expect(deriveCheckinState("Schule")).toBe("abwesend");
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

    expect(mockSchoolCheckinStudent).toHaveBeenCalledWith(
      "42",
      "in",
      undefined,
    );
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

    expect(mockSchoolCheckinStudent).toHaveBeenCalledWith(
      "42",
      "out",
      undefined,
    );
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

    expect(mockSchoolCheckinStudent).toHaveBeenCalledWith(
      "7",
      "out",
      undefined,
    );
  });

  it("passes the early-checkout note on (#3324)", async () => {
    mockSchoolCheckinStudent.mockResolvedValueOnce({
      studentId: 9,
      status: "checked_out",
      location: "Abwesend",
      changed: true,
    });

    const { result } = renderHook(() => useSchoolCheckinMode());
    await act(async () => {
      await result.current.toggle("9", "anwesend", "Arzttermin");
    });

    expect(mockSchoolCheckinStudent).toHaveBeenCalledWith(
      "9",
      "out",
      "Arzttermin",
    );
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
    mockSchoolCheckinStudent.mockRejectedValueOnce(
      new ApiError("boom", 500, { code: "general.server" }),
    );

    const { result } = renderHook(() => useSchoolCheckinMode());
    await act(async () => {
      await result.current.toggle("5", "abwesend");
    });

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        "Die Anmeldung konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
        { retry: expect.any(Function) },
      ),
    );
    expect(result.current.pendingIds.has("5")).toBe(false);
  });

  it("names the permission problem instead of asking for a retry on 403", async () => {
    // A permission 403 (missing users:checkin or web attendance disabled)
    // fails on every retry — "bitte erneut versuchen" would be misleading
    // advice there (#2220).
    mockSchoolCheckinStudent.mockRejectedValueOnce(
      new ApiError("API error (403): Forbidden", 403, {
        code: "general.permission",
      }),
    );

    const { result } = renderHook(() => useSchoolCheckinMode());
    await act(async () => {
      await result.current.toggle("7", "anwesend");
    });

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        "Für die Abmeldung fehlt Ihnen die Berechtigung. Bitte fragen Sie die Schule.",
        expect.anything(),
      ),
    );
    expect(mockToastError).not.toHaveBeenCalledWith(
      expect.stringContaining("erneut"),
      expect.anything(),
    );
  });

  it("keeps the retry wording for non-permission failures", async () => {
    mockSchoolCheckinStudent.mockRejectedValueOnce(
      new ApiError("API error (500): Internal Server Error", 500, {
        code: "general.server",
      }),
    );

    const { result } = renderHook(() => useSchoolCheckinMode());
    await act(async () => {
      await result.current.toggle("8", "anwesend");
    });

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        "Die Abmeldung konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
        { retry: expect.any(Function) },
      ),
    );
  });

  it("toast copy matches the attempted direction (out on present)", async () => {
    mockSchoolCheckinStudent.mockRejectedValueOnce(
      new ApiError("boom", 503, { code: "general.unavailable" }),
    );

    const { result } = renderHook(() => useSchoolCheckinMode());
    await act(async () => {
      await result.current.toggle("5", "anwesend");
    });

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        "Die Abmeldung ist gerade nicht erreichbar. Bitte versuchen Sie es erneut.",
        { retry: expect.any(Function) },
      ),
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
