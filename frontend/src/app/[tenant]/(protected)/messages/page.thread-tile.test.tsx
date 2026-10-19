import { render as rtlRender, screen, within } from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";

function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

const { mockInbox } = vi.hoisted(() => ({
  mockInbox: { threads: [] as Array<Record<string, unknown>> },
}));

vi.mock("swr", () => ({
  default: () => ({
    data: mockInbox.threads,
    error: undefined,
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

// #4361: Absender und Kind tragen die Symbole aus dem Konzeptregister, damit
// sie beim Überfliegen auseinanderzuhalten sind (Kind wie in den Anfragen).
describe("Nachrichten: Kachel einer Unterhaltung", () => {
  beforeEach(() => {
    mockInbox.threads = [
      {
        thread_id: "7",
        student_id: "3",
        student_name: "Felix Schneider",
        guardian_name: "Klaus Schneider",
        relationship_type: "parent",
        unread_count: 2,
        last_message_body: "Felix wird heute abgeholt.",
        last_sender_kind: "guardian",
        last_message_at: "2026-09-09T08:00:00Z",
      },
    ];
  });

  it("marks the sender with the parents symbol", async () => {
    render(<MessagesPage />);
    const sender = await screen.findByRole("heading", {
      name: "Klaus Schneider",
    });
    const icon = sender.previousElementSibling;
    expect(icon).toHaveAttribute("data-moto-duotone-tone", "blue");
  });

  it("marks the child with the children symbol", async () => {
    render(<MessagesPage />);
    const child = await screen.findByText("Felix Schneider");
    const icon = child.previousElementSibling;
    expect(icon).toHaveAttribute("data-moto-duotone-tone", "greenVivid");
  });

  it("keeps relationship, unread count and preview", async () => {
    render(<MessagesPage />);
    const tile = (await screen.findByText("Felix Schneider")).closest("li")!;
    expect(within(tile).getByText(/Elternteil von/)).toBeInTheDocument();
    expect(within(tile).getByText("2")).toBeInTheDocument();
    expect(
      within(tile).getByText("Felix wird heute abgeholt."),
    ).toBeInTheDocument();
  });
});
