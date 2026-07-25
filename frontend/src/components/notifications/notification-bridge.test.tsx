import { describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import { NotificationBridge } from "./notification-bridge";
import type { PhoenixNotificationDetail } from "./notification-bridge";

const toast = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
  warning: vi.fn(),
  remove: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => toast,
}));

function dispatch(detail: Partial<PhoenixNotificationDetail>) {
  window.dispatchEvent(new CustomEvent("phoenix:notification", { detail }));
}

describe("NotificationBridge", () => {
  it("renders a notification event as an info toast", () => {
    render(<NotificationBridge />);
    dispatch({ title: "Testbenachrichtigung", body: "Alles eingerichtet." });

    expect(toast.info).toHaveBeenCalledWith(
      "Testbenachrichtigung: Alles eingerichtet.",
      expect.objectContaining({ duration: expect.any(Number) as number }),
    );
  });

  it("renders high priority as a warning toast", () => {
    render(<NotificationBridge />);
    dispatch({ title: "Wichtig", body: "Bitte beachten.", priority: "high" });

    expect(toast.warning).toHaveBeenCalledWith(
      "Wichtig: Bitte beachten.",
      expect.anything(),
    );
  });

  it("ignores events without a title", () => {
    toast.info.mockClear();
    render(<NotificationBridge />);
    dispatch({ body: "kein Titel" });

    expect(toast.info).not.toHaveBeenCalled();
  });

  it("stops listening after unmount", () => {
    toast.info.mockClear();
    const { unmount } = render(<NotificationBridge />);
    unmount();
    dispatch({ title: "Nach Unmount" });

    expect(toast.info).not.toHaveBeenCalled();
  });
});
