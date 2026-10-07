import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { SWRConfig } from "swr";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { StaffMessagesApi } from "~/lib/staff-messages-api";
import type { TeamChatPortal } from "~/lib/team-chat-portal";
import { catalogText } from "~/test/error-catalog-text";

import { TeamChatInbox } from "./team-chat-inbox";
import { TeamChatThread } from "./team-chat-thread";

// Echtes SWR statt des globalen Stubs: Fehler, Wiederholen und Neuladen
// laufen über den echten Abruf.
vi.mock("swr", async () => await vi.importActual<typeof import("swr")>("swr"));

vi.mock("~/lib/hooks/use-messages-activity", () => ({
  useMessagesActivity: vi.fn(),
}));

vi.mock("~/lib/hooks/use-chat-viewport-lock", () => ({
  useChatViewportLock: () => ({ current: null }),
}));

function stubApi() {
  return {
    fetchInbox: vi.fn<StaffMessagesApi["fetchInbox"]>(),
    fetchUnreadCount: vi.fn<StaffMessagesApi["fetchUnreadCount"]>(),
    fetchRecipients: vi.fn<StaffMessagesApi["fetchRecipients"]>(),
    fetchThread: vi.fn<StaffMessagesApi["fetchThread"]>(),
    openThread: vi.fn<StaffMessagesApi["openThread"]>(),
    postMessage: vi.fn<StaffMessagesApi["postMessage"]>(),
  };
}

function portalFor(api: StaffMessagesApi): TeamChatPortal {
  return {
    kind: "school",
    api,
    cacheScope: "test",
    inboxHref: "/nachrichten",
    threadHref: (id) => `/nachrichten/${id}`,
    navigate: vi.fn(),
    flagSaysEnabled: true,
    title: "Team-Chat",
    emptyDescription: "Hier erscheinen Ihre Unterhaltungen.",
    recipientHint: "Sie erreichen hier Ihr Team.",
  };
}

// Jeder Test mit eigenem SWR-Speicher und ohne SWR-Wiederholungen.
function Fresh({ children }: { readonly children: ReactNode }) {
  return (
    <SWRConfig
      value={{
        provider: () => new Map(),
        dedupingInterval: 0,
        shouldRetryOnError: false,
      }}
    >
      {children}
    </SWRConfig>
  );
}

const thread = {
  thread_id: "t1",
  counterpart_account_id: "9",
  counterpart_name: "Anna Beispiel",
  counterpart_role_kind: "staff" as const,
  messages: [
    {
      id: "m1",
      sender_account_id: "9",
      sender_name: "Anna Beispiel",
      body: "Hallo",
      created_at: "2026-09-09T10:00:00Z",
    },
  ],
};

describe("TeamChatThread (#2517)", () => {
  let api: ReturnType<typeof stubApi>;

  beforeEach(() => {
    api = stubApi();
  });

  it("shows a failed history load with the catalog text and retries", async () => {
    api.fetchThread
      .mockRejectedValueOnce(
        new ApiError("thread kaputt", 500, { code: "general.server" }),
      )
      .mockResolvedValueOnce(thread);
    render(
      <TeamChatThread
        portal={portalFor(api)}
        threadID="t1"
        myAccountId="1"
        backNav={null}
      />,
      { wrapper: Fresh },
    );

    expect(
      await screen.findByText(
        catalogText("general.server", "die Unterhaltung"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/kaputt/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByText("Hallo")).toBeInTheDocument();
  });

  it("keeps a failed send above the composer and resends the draft", async () => {
    api.fetchThread.mockResolvedValue(thread);
    api.postMessage
      .mockRejectedValueOnce(
        new ApiError("send kaputt", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce({
        id: "m2",
        sender_account_id: "1",
        sender_name: "Ich",
        body: "Bin unterwegs",
        created_at: "2026-09-09T10:05:00Z",
      });
    render(
      <TeamChatThread
        portal={portalFor(api)}
        threadID="t1"
        myAccountId="1"
        backNav={null}
      />,
      { wrapper: Fresh },
    );

    await screen.findByText("Hallo");
    fireEvent.change(screen.getByLabelText("Nachricht"), {
      target: { value: "Bin unterwegs" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Senden" }));

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Nachricht"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/kaputt/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() => expect(api.postMessage).toHaveBeenCalledTimes(2));
    expect(api.postMessage).toHaveBeenLastCalledWith("t1", "Bin unterwegs");
  });
});

describe("TeamChatInbox (#2517)", () => {
  it("never shows the empty state for a list that failed to load", async () => {
    const api = stubApi();
    api.fetchInbox
      .mockRejectedValueOnce(
        new ApiError("inbox kaputt", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce([]);
    render(<TeamChatInbox portal={portalFor(api)} />, { wrapper: Fresh });

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Unterhaltungen"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Noch keine Nachrichten"),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(
      await screen.findByText("Noch keine Nachrichten"),
    ).toBeInTheDocument();
  });

  it("reports no zero counts for a list that failed to load", async () => {
    const api = stubApi();
    api.fetchInbox.mockRejectedValueOnce(
      new ApiError("inbox kaputt", 503, { code: "general.unavailable" }),
    );
    const seen: { stats: string | null; count: number | null }[] = [];
    render(
      <TeamChatInbox
        portal={portalFor(api)}
        frame={(parts) => {
          seen.push({ stats: parts.stats, count: parts.count });
          return (
            <>
              <p>{parts.stats}</p>
              {parts.error ? <p>{parts.error.message}</p> : null}
            </>
          );
        }}
      />,
      { wrapper: Fresh },
    );

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Unterhaltungen"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/0 Unterhaltungen/)).not.toBeInTheDocument();
    expect(seen.at(-1)).toEqual({ stats: null, count: null });
  });
});
