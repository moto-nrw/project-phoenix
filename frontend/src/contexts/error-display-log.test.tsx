import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  ToastProvider,
  useApiErrorDisplay,
  useApiFormError,
  useApiLoadError,
} from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { ERROR_DISPLAYED } from "~/lib/error-log";

function wrapper({ children }: { readonly children: ReactNode }) {
  return <ToastProvider>{children}</ToastProvider>;
}

function displayedEntries(warn: { mock: { calls: unknown[][] } }) {
  return warn.mock.calls
    .filter(([msg]) => msg === ERROR_DISPLAYED)
    .map(([, context]) => context);
}

const rejection = () =>
  new ApiError("Anna Schmidt ist schon im Raum", 409, {
    code: "presence.room_capacity_exceeded",
    detail: "student Anna Schmidt already checked in",
    details: { max_capacity: 20, current_occupancy: 20 },
    instance: "req-9",
  });

afterEach(() => {
  vi.restoreAllMocks();
});

describe("error_displayed entries (#2512)", () => {
  it("logs a failed action shown as a toast with code, class and request ID", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = renderHook(() => useApiErrorDisplay(), { wrapper });

    await act(async () => {
      await result.current.show(rejection(), { object: "das Kind" });
    });

    expect(displayedEntries(warn)).toEqual([
      {
        surface: "toast",
        error_class: "business_rejection",
        error_code: "presence.room_capacity_exceeded",
        status: 409,
        request_id: "req-9",
        error_name: "ApiError",
      },
    ]);
    expect(JSON.stringify(warn.mock.calls)).not.toContain("Anna");
  });

  it("logs the class of the code, not of the status", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = renderHook(() => useApiErrorDisplay(), { wrapper });

    await act(async () => {
      await result.current.show(
        new ApiError("x", 400, { code: "general.request_too_large" }),
        { object: "die Datei" },
      );
    });

    expect(displayedEntries(warn)).toMatchObject([
      { error_class: "input", error_code: "general.request_too_large" },
    ]);
  });

  it("logs a form failure with the names of its invalid fields", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = renderHook(() => useApiFormError(), { wrapper });

    await act(async () => {
      await result.current.show(
        new ApiError("Vorname fehlt", 400, {
          code: "general.input",
          errors: [{ field: "first_name", reason: "Anna Schmidt" }],
        }),
        { object: "das Kind" },
      );
    });

    expect(displayedEntries(warn)).toEqual([
      {
        surface: "form",
        error_class: "input",
        error_code: "general.input",
        status: 400,
        error_name: "ApiError",
        fields: ["first_name"],
      },
    ]);
    expect(JSON.stringify(warn.mock.calls)).not.toContain("Anna");
  });

  it("logs a failed load and an exception as a crash", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = renderHook(() => useApiLoadError(), { wrapper });

    await act(async () => {
      await result.current.show(new TypeError("Anna is undefined"), {
        object: "die Liste",
      });
    });

    expect(displayedEntries(warn)).toEqual([
      { surface: "load", error_class: "crash", error_name: "TypeError" },
    ]);
    expect(JSON.stringify(warn.mock.calls)).not.toContain("Anna");
  });

  it("logs a failure the screen found itself without its text", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const toast = renderHook(() => useApiErrorDisplay(), { wrapper });
    const form = renderHook(() => useApiFormError(), { wrapper });

    act(() => {
      toast.result.current.showMessage("Anna Schmidt.pdf ist zu groß.");
      form.result.current.invalid("Bitte prüfen Sie den Vornamen.", {
        first_name: "Pflichtfeld",
      });
    });

    expect(displayedEntries(warn)).toEqual([
      { surface: "toast", error_class: "local" },
      { surface: "form", error_class: "local", fields: ["first_name"] },
    ]);
    expect(JSON.stringify(warn.mock.calls)).not.toContain("Anna");
  });

  it("logs nothing for an expired session, which opens the login instead", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    const { result } = renderHook(() => useApiErrorDisplay(), { wrapper });

    await act(async () => {
      await result.current.show(new ApiError("expired", 401), {
        object: "die Gruppe",
      });
    });

    expect(assign).toHaveBeenCalledTimes(1);
    expect(displayedEntries(warn)).toEqual([]);
  });
});
