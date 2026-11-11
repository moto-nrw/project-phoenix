import { act, renderHook, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import { ToastProvider, useApiErrorDisplay } from "./ToastContext";

vi.mock("@sentry/nextjs", () => ({
  captureException: vi.fn(),
  addBreadcrumb: vi.fn(),
}));

import * as Sentry from "@sentry/nextjs";

vi.mock("~/lib/error-presentation", () => {
  throw new Error("Presenter chunk unavailable");
});

afterEach(() => {
  document.documentElement.lang = "de";
});

describe("useApiErrorDisplay when the presenter chunk fails", () => {
  it("never offers retry or request ID without reliable classification", async () => {
    const retry = vi.fn();
    const { result } = renderHook(() => useApiErrorDisplay(), {
      wrapper: ToastProvider,
    });
    await act(() =>
      result.current.show(
        new ApiError("backend", undefined, {
          code: "general.input",
          instance: "req-20",
        }),
        { object: "Die Gruppe", retry },
      ),
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Die Gruppe konnte nicht bearbeitet werden. Bitte versuchen Sie es später.",
    );
    expect(screen.queryByRole("button", { name: "Wiederholen" })).toBeNull();
    expect(screen.queryByText("req-20")).toBeNull();
    expect(retry).not.toHaveBeenCalled();
  });

  it("uses the active locale for emergency text", async () => {
    document.documentElement.lang = "en";
    const { result } = renderHook(() => useApiErrorDisplay(), {
      wrapper: ToastProvider,
    });
    await act(() =>
      result.current.show(new ApiError("backend", 500), {
        object: "The group",
      }),
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "The group could not be processed. Please try later.",
    );
    expect(screen.queryByText(/konnte nicht bearbeitet/)).toBeNull();
  });

  it("still reports a crash to Sentry, and no API failure", async () => {
    const { result } = renderHook(() => useApiErrorDisplay(), {
      wrapper: ToastProvider,
    });
    const crash = new TypeError("undefined is not an object");
    await act(async () => {
      await result.current.show(new ApiError("backend", 500), {
        object: "Die Gruppe",
      });
      await result.current.show(crash, { object: "Die Gruppe" });
    });
    expect(vi.mocked(Sentry.captureException).mock.calls).toEqual([
      [crash, { tags: { surface: "toast" } }],
    ]);
  });
});
