import {
  act,
  fireEvent,
  render as rtlRender,
  screen,
  within,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";

function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

const { state, push, mutate } = vi.hoisted(() => ({
  state: { threads: [] as Array<Record<string, unknown>>, enabled: true },
  push: vi.fn(),
  mutate: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({
    data: state.threads,
    error: undefined,
    isLoading: false,
    mutate,
  }),
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/schule/messages/groups",
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({ tenant: { messagingEnabled: state.enabled } }),
  useTenantSlugSafe: () => "schule",
}));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push }),
}));
vi.mock("~/components/messaging/new-group-message-modal", () => ({
  NewGroupMessageModal: () => <div>Formular Gruppennachricht</div>,
}));
vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: () => false,
}));

import GroupMessagesPage from "./page";

describe("Gruppennachrichten: Liste", () => {
  beforeEach(() => {
    push.mockReset();
    mutate.mockReset();
    state.enabled = true;
    state.threads = [
      {
        id: "4",
        target_type: "group",
        target_id: "5",
        target_label: "Gruppe Bären",
        title: "Sommerfest",
        created_at: "2026-09-08T08:00:00Z",
        last_message_at: "2026-09-09T08:00:00Z",
        last_message_body: "Wer bringt einen Kuchen mit?",
        message_count: 2,
        family_count: 5,
        read_count: 3,
      },
      {
        id: "3",
        target_type: "class",
        target_id: "1a",
        target_label: "Klasse 1a",
        title: "Elternabend",
        created_at: "2026-09-01T08:00:00Z",
        closed_at: "2026-09-02T08:00:00Z",
        last_message_at: "2026-09-01T08:00:00Z",
        last_message_body: "Der Elternabend ist verschoben.",
        message_count: 1,
        family_count: 1,
        read_count: 1,
      },
    ];
  });

  it("shows target, read count and preview per thread", () => {
    render(<GroupMessagesPage />);
    const tile = screen
      .getByRole("heading", { name: "Sommerfest" })
      .closest("li")!;
    expect(within(tile).getByText(/Gruppe Bären/)).toBeInTheDocument();
    expect(
      within(tile).getByText(/3 von 5 Familien gelesen/),
    ).toBeInTheDocument();
    expect(
      within(tile).getByText("Wer bringt einen Kuchen mit?"),
    ).toBeInTheDocument();
  });

  it("marks a closed thread", () => {
    render(<GroupMessagesPage />);
    const tile = screen
      .getByRole("heading", { name: "Elternabend" })
      .closest("li")!;
    expect(within(tile).getByText("Geschlossen")).toBeInTheDocument();
    expect(
      within(tile).getByText(/1 von 1 Familie gelesen/),
    ).toBeInTheDocument();
  });

  it("counts threads in the status line", () => {
    render(<GroupMessagesPage />);
    expect(
      screen.getByText("2 Gruppennachrichten · 1 offen"),
    ).toBeInTheDocument();
  });

  it("reloads after a group-message activity event", () => {
    render(<GroupMessagesPage />);
    act(() => {
      window.dispatchEvent(new CustomEvent("messages-activity"));
    });
    expect(mutate).toHaveBeenCalledTimes(1);
  });

  it("links both registers of Nachrichten", () => {
    render(<GroupMessagesPage />);
    expect(screen.getByRole("tab", { name: "Unterhaltungen" })).toHaveAttribute(
      "href",
      "/messages",
    );
    expect(
      screen.getByRole("tab", { name: "Gruppennachrichten" }),
    ).toHaveAttribute("aria-selected", "true");
  });

  it("opens a thread and the composer", () => {
    render(<GroupMessagesPage />);
    fireEvent.click(screen.getByRole("heading", { name: "Sommerfest" }));
    expect(push).toHaveBeenCalledWith("/messages/groups/4");
    fireEvent.click(
      screen.getByRole("button", { name: "Neue Gruppennachricht" }),
    );
    expect(screen.getByText("Formular Gruppennachricht")).toBeInTheDocument();
  });

  it("offers no composer when the school switched messaging off", () => {
    state.enabled = false;
    render(<GroupMessagesPage />);
    expect(
      screen.getByRole("heading", { name: "Sommerfest" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Neue Gruppennachricht" }),
    ).not.toBeInTheDocument();
  });
});
