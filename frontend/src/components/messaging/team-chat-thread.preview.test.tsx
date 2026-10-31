import { render, renderHook, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { useSWRAuth } from "~/lib/swr";
import { messageKeys } from "~/lib/swr/portal-keys";
import { TeamChatThread } from "./team-chat-thread";
import { renderThreadFrame } from "~/app/[tenant]/(protected)/team-chat/[threadID]/page";
import type { TeamChatPortal } from "~/lib/team-chat-portal";

vi.unmock("swr");
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "7", tenantId: 1, scope: "tenant" } },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/hooks/use-messages-activity", () => ({
  useMessagesActivity: vi.fn(),
}));
vi.mock("~/lib/hooks/use-chat-viewport-lock", () => ({
  useChatViewportLock: () => ({ current: null }),
}));

it.each(["tenant", "school"] as const)(
  "%s chat shows only inbox identity while detail is pending",
  async (kind) => {
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
    const fetchThread = vi.fn(() => new Promise<never>(() => {}));
    const portal = {
      kind,
      api: { fetchThread },
      flagSaysEnabled: true,
    } as unknown as TeamChatPortal;
    render(
      <TeamChatThread
        portal={portal}
        threadID="4"
        myAccountId="7"
        back={{ href: "/team-chat", label: "Zurück" }}
        frame={kind === "tenant" ? renderThreadFrame : undefined}
      />,
      { wrapper },
    );
    expect(
      screen.getByRole("heading", { name: "Anna Beispiel" }),
    ).toBeInTheDocument();
    // Anker: die Identität steht bereits im Ladekopf, der Verlauf noch nicht.
    expect(screen.queryByText("Private Nachricht")).not.toBeInTheDocument();
    expect(screen.queryByText(/99 Nachrichten/)).not.toBeInTheDocument();
    expect(fetchThread).toHaveBeenCalledWith("4");
  },
);
