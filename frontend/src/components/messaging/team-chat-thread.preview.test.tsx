import { render, renderHook, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { useSWRAuth } from "~/lib/swr";
import { messageKeys } from "~/lib/swr/portal-keys";
import TeamThreadPage from "~/app/[tenant]/(protected)/team-chat/[threadID]/page";
import SchoolMessageThreadPage from "~/app/school/nachrichten/[threadID]/page";

const state = vi.hoisted(() => ({
  kind: "tenant",
  fetchThread: vi.fn(() => new Promise<never>(() => {})),
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ threadID: "4" }),
  usePathname: () => "/team-chat/4",
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => (state.kind === "tenant" ? "schule" : null),
  useTenantRoutingModeSafe: () => "path",
}));
vi.mock("~/lib/hooks/use-tenant-team-chat-portal", () => ({
  useTenantTeamChatPortal: () => ({
    kind: "tenant",
    api: { fetchThread: state.fetchThread },
    flagSaysEnabled: true,
  }),
}));
vi.mock("~/lib/hooks/use-school-team-chat-portal", () => ({
  SCHOOL_MESSAGES_ROUTE: "/nachrichten",
  useSchoolTeamChatPortal: () => ({
    kind: "school",
    api: { fetchThread: state.fetchThread },
    flagSaysEnabled: true,
  }),
}));
vi.unmock("swr");
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "7", tenantId: 1, scope: state.kind } },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/hooks/use-messages-activity", () => ({
  useMessagesActivity: vi.fn(),
}));
vi.mock("~/lib/hooks/use-chat-viewport-lock", () => ({
  useChatViewportLock: () => ({ current: null }),
}));

beforeEach(() => vi.stubEnv("NEXT_PUBLIC_SCHOOL_HOSTNAME", "schule.localhost"));

it.each(["tenant", "school"] as const)(
  "%s chat shows only inbox identity while detail is pending",
  async (kind) => {
    state.kind = kind;
    state.fetchThread.mockClear();
    const cache = new Map();
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <SWRConfig value={{ provider: () => cache }}>{children}</SWRConfig>
    );
    const list = renderHook(
      () =>
        useSWRAuth(messageKeys.teamChatInbox(false), async () => [
          {
            thread_id: "4",
            counterpart_name: "Anna Beispiel",
            last_message_body: "Private Nachricht",
            unread_count: 99,
          },
        ]),
      { wrapper },
    );
    await waitFor(() => expect(list.result.current.data).toBeDefined());
    list.unmount();
    render(
      kind === "tenant" ? <TeamThreadPage /> : <SchoolMessageThreadPage />,
      { wrapper },
    );
    expect(
      screen.getByRole("heading", { name: "Anna Beispiel" }),
    ).toBeInTheDocument();
    // Anker: die Identität steht bereits im Ladekopf, der Verlauf noch nicht.
    expect(screen.queryByText("Private Nachricht")).not.toBeInTheDocument();
    expect(screen.queryByText(/99 Nachrichten/)).not.toBeInTheDocument();
    expect(state.fetchThread).toHaveBeenCalledWith("4");
  },
);
