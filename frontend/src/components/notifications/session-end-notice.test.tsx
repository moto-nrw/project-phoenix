import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { releaseFakeTimers } from "~/test/clock";
import {
  DISCARDED_WRITE_MESSAGE,
  NotificationBridge,
  RELOAD_REQUIRED_MESSAGE,
} from "./notification-bridge";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useParams: () => ({}),
}));

function sessionEnd(ending: boolean, discardedWrites: number) {
  act(() => {
    window.dispatchEvent(
      new CustomEvent("phoenix:session-end", {
        detail: { ending, discardedWrites },
      }),
    );
  });
}

// A logout that leaves the tab open discards the writes it held (#5126).
describe("NotificationBridge session-end notice", () => {
  it("says once that a held change was not saved", () => {
    render(
      <ToastProvider>
        <NotificationBridge />
      </ToastProvider>,
    );
    sessionEnd(true, 0);
    sessionEnd(false, 2);

    expect(screen.getAllByRole("alert")).toHaveLength(1);
    expect(screen.getByRole("alert")).toHaveTextContent(
      DISCARDED_WRITE_MESSAGE,
    );
  });

  // Decision on #5126: a bfcache restore during a logout reloads the page. If
  // the reload does not happen, a lasting message offers it.
  it("offers a lasting „Seite neu laden“ that reloads the page", async () => {
    vi.useFakeTimers();
    const location = Object.getOwnPropertyDescriptor(window, "location");
    const reload = vi.fn();
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        href: window.location.href,
        origin: window.location.origin,
        reload,
      },
    });
    try {
      render(
        <ToastProvider>
          <NotificationBridge />
        </ToastProvider>,
      );
      act(() => {
        window.dispatchEvent(
          new CustomEvent("phoenix:session-end", {
            detail: { ending: true, discardedWrites: 0, reloadRequired: true },
          }),
        );
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(60_000);
      });
      expect(screen.getByText(RELOAD_REQUIRED_MESSAGE)).toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Seite neu laden" }));
      expect(reload).toHaveBeenCalledOnce();
    } finally {
      if (location) Object.defineProperty(window, "location", location);
      releaseFakeTimers();
    }
  });
});
