import {
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";

function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

const { state, mutate, api } = vi.hoisted(() => ({
  state: { detail: null as Record<string, unknown> | null },
  mutate: vi.fn(() => Promise.resolve(undefined)),
  api: {
    closeGroupThread: vi.fn(),
    postGroupMessage: vi.fn(),
    fetchGroupThread: vi.fn(),
  },
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({
    data: state.detail,
    error: undefined,
    isLoading: false,
    mutate,
  }),
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ groupThreadId: "4" }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/schule/messages/groups/4",
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({ tenant: { messagingEnabled: true } }),
  useTenantSlugSafe: () => "schule",
  useTenantRoutingModeSafe: () => "subdomain",
}));
vi.mock("~/lib/parent-group-threads-api", async (importOriginal) => ({
  ...(await importOriginal()),
  ...api,
}));
vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: () => false,
}));

import GroupMessagePage from "./page";

function detail(closed: boolean) {
  return {
    thread: {
      id: "4",
      target_type: "group",
      target_id: "5",
      target_label: "Gruppe Bären",
      title: "Sommerfest",
      created_at: "2026-09-08T08:00:00Z",
      ...(closed ? { closed_at: "2026-09-09T09:00:00Z" } : {}),
    },
    messages: [
      {
        id: "1",
        sender_name: "Olivia Berg",
        body: "Wer bringt einen Kuchen mit?",
        created_at: "2026-09-08T08:00:00Z",
      },
    ],
    family_count: 2,
    read_count: 1,
    children: [
      {
        student_id: "7",
        first_name: "Felix",
        last_name: "Schneider",
        school_class: "1a",
        reachable: true,
        read: false,
      },
      {
        student_id: "8",
        first_name: "Emma",
        last_name: "Klein",
        reachable: true,
        read: true,
      },
    ],
  };
}

describe("Gruppennachricht: Objektseite", () => {
  beforeEach(() => {
    state.detail = detail(false);
    mutate.mockClear();
    api.closeGroupThread.mockReset();
  });

  it("shows the messages, the status line and who has read", () => {
    render(<GroupMessagePage />);
    expect(
      screen.getByText("Wer bringt einen Kuchen mit?"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Gruppe Bären · 1 von 2 Familien gelesen"),
    ).toBeInTheDocument();
    const list = screen.getByTestId("group-read-list");
    const felix = within(list)
      .getByText(/Felix Schneider/)
      .closest("li")!;
    expect(within(felix).getByText("Noch nicht gelesen")).toBeInTheDocument();
    const emma = within(list)
      .getByText(/Emma Klein/)
      .closest("li")!;
    expect(within(emma).getByText("Gelesen")).toBeInTheDocument();
  });

  it("closes the thread only after confirmation", async () => {
    api.closeGroupThread.mockResolvedValue(detail(true));
    render(<GroupMessagePage />);
    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    fireEvent.click(await screen.findByRole("menuitem", { name: "Schließen" }));
    expect(api.closeGroupThread).not.toHaveBeenCalled();
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(/Danach können Eltern nicht mehr antworten/),
    ).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Schließen" }));
    await waitFor(() => expect(api.closeGroupThread).toHaveBeenCalledWith("4"));
    expect(mutate).toHaveBeenCalledWith(detail(true), { revalidate: false });
  });

  it("offers no composer and no close once closed", () => {
    state.detail = detail(true);
    render(<GroupMessagePage />);
    expect(
      screen.getByText(/Diese Gruppennachricht ist geschlossen/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("textbox", { name: "Nachricht" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText("Gruppe Bären · 1 von 2 Familien gelesen · Geschlossen"),
    ).toBeInTheDocument();
  });

  it("aktualisiert die offene Ansicht nach einer Nachricht aus einem anderen Tab", async () => {
    render(<GroupMessagePage />);
    expect(
      await screen.findByText("Wer bringt einen Kuchen mit?"),
    ).toBeInTheDocument();
    mutate.mockClear();

    window.dispatchEvent(new CustomEvent("messages-activity"));

    await waitFor(() => expect(mutate).toHaveBeenCalledTimes(1));
  });
});
