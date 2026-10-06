import { render as rtlRender, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

const { mockInbox } = vi.hoisted(() => ({
  mockInbox: {
    threads: undefined as Array<Record<string, unknown>> | undefined,
    error: undefined as unknown,
  },
}));

vi.mock("swr", () => ({
  default: () => ({
    data: mockInbox.threads,
    error: mockInbox.error,
    isLoading: false,
    mutate: vi.fn(() => Promise.resolve(undefined)),
  }),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/schule/messages",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({ tenant: { messagingEnabled: true } }),
  useTenantSlugSafe: () => "schule",
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn() }),
}));

vi.mock("~/lib/hooks/use-messages-activity", () => ({
  useMessagesActivity: vi.fn(),
}));

vi.mock("~/lib/hooks/use-messages-unread", () => ({
  useMessagesUnread: () => ({
    unreadCount: 0,
    isLoading: false,
    refresh: vi.fn(),
  }),
}));

vi.mock("~/components/messaging/new-message-modal", () => ({
  NewMessageModal: () => null,
}));

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: () => false,
}));

import MessagesPage from "./page";

describe("Nachrichten: Statuszeile bei Ladefehler (#2517)", () => {
  beforeEach(() => {
    mockInbox.threads = undefined;
    mockInbox.error = undefined;
  });

  it("shows the catalog load error and no zero count", async () => {
    mockInbox.error = new ApiError("inbox exploded", 503, {
      code: "general.unavailable",
    });

    render(<MessagesPage />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Nachrichten"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/0 Unterhaltungen/)).not.toBeInTheDocument();
  });

  it("still counts a loaded, empty inbox", async () => {
    mockInbox.threads = [];
    render(<MessagesPage />);

    expect(
      await screen.findByText(/0 Unterhaltungen · 0 ungelesen/),
    ).toBeInTheDocument();
  });
});
