import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider, useToast } from "~/contexts/ToastContext";
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
  it("offers a lasting „Seite neu laden“ that later toasts cannot push out or close", async () => {
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
    let toast!: ReturnType<typeof useToast>;
    function ToastHandle() {
      toast = useToast();
      return null;
    }
    try {
      render(
        <ToastProvider>
          <NotificationBridge />
          <ToastHandle />
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

      // Later toasts do not push it out, and it has no close button.
      act(() => {
        for (const n of [1, 2, 3, 4, 5])
          toast.warning(`Hinweis ${n}`, { duration: 0 });
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(60_000);
      });
      const notice = screen
        .getByText(RELOAD_REQUIRED_MESSAGE)
        .closest('[role="status"], [role="alert"]');
      expect(notice).not.toBeNull();
      // Anker: the notice's own action is there, its close button is not.
      expect(
        within(notice as HTMLElement).getByRole("button", {
          name: "Seite neu laden",
        }),
      ).toBeInTheDocument();
      expect(
        within(notice as HTMLElement).queryByRole("button", {
          name: "Schließen",
        }),
      ).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Seite neu laden" }));
      expect(reload).toHaveBeenCalledOnce();
      // If the reload does not happen either, the notice stays.
      await act(async () => {
        await vi.advanceTimersByTimeAsync(60_000);
      });
      expect(screen.getByText(RELOAD_REQUIRED_MESSAGE)).toBeInTheDocument();
    } finally {
      if (location) Object.defineProperty(window, "location", location);
      releaseFakeTimers();
    }
  });
});
