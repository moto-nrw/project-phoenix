import { act, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import {
  DISCARDED_WRITE_MESSAGE,
  NotificationBridge,
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
});
