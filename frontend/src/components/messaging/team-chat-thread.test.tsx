vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "account-1", tenantId: 1, scope: "school" },
      expires: "2099-01-01",
    },
    status: "authenticated",
  }),
}));
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import type { ReactNode } from "react";
import { SWRConfig } from "~/lib/swr/testing";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import { ApiError } from "~/lib/api-error";
import type { StaffMessagesApi } from "~/lib/staff-messages-api";
import type { TeamChatPortal } from "~/lib/team-chat-portal";
import { catalogText } from "~/test/error-catalog-text";

import { TeamChatInbox } from "./team-chat-inbox";
import { TeamChatThread } from "./team-chat-thread";

// Echtes SWR statt des globalen Stubs: Fehler, Wiederholen und Neuladen
// laufen über den echten Abruf.
vi.unmock("swr");

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
    createGroup: vi.fn<StaffMessagesApi["createGroup"]>(),
    renameGroup: vi.fn<StaffMessagesApi["renameGroup"]>(),
    addGroupMembers: vi.fn<StaffMessagesApi["addGroupMembers"]>(),
    removeGroupMember: vi.fn<StaffMessagesApi["removeGroupMember"]>(),
    leaveGroup: vi.fn<StaffMessagesApi["leaveGroup"]>(),
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

  // Die Fläche des Schul-Portals trägt den Rückweg über dem Titel, auf jeder
  // Breite und auch, wenn der Verlauf nicht lädt (#4364).
  it("puts the back link above the title in the school portal frame", async () => {
    api.fetchThread.mockResolvedValue(thread);
    render(
      <TeamChatThread
        portal={portalFor(api)}
        threadID="t1"
        myAccountId="1"
        back={{ href: "/nachrichten", label: "Zurück zu Nachrichten" }}
      />,
      { wrapper: Fresh },
    );

    await screen.findByText("Hallo");
    const link = screen.getByRole("link", { name: "Zurück zu Nachrichten" });
    expect(link).toHaveAttribute("href", "/nachrichten");
    expect(
      link.compareDocumentPosition(screen.getByRole("heading", { level: 1 })) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(link.className).not.toMatch(/(^|\s)(sm|md|lg|xl):hidden/);
  });

  it("keeps the back link when the history fails to load", async () => {
    api.fetchThread.mockRejectedValue(
      new ApiError("thread kaputt", 500, { code: "general.server" }),
    );
    render(
      <TeamChatThread
        portal={portalFor(api)}
        threadID="t1"
        myAccountId="1"
        back={{ href: "/nachrichten", label: "Zurück zu Nachrichten" }}
      />,
      { wrapper: Fresh },
    );

    await screen.findByText(catalogText("general.server", "die Unterhaltung"));
    expect(
      screen.getByRole("link", { name: "Zurück zu Nachrichten" }),
    ).toHaveAttribute("href", "/nachrichten");
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
        back={{ href: "/team-chat", label: "Zurück zum Team-Chat" }}
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
        back={{ href: "/team-chat", label: "Zurück zum Team-Chat" }}
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
    const firstKey = vi.mocked(api.postMessage).mock.calls[0]?.[2];
    expect(firstKey).toEqual(expect.any(String));
    expect(firstKey).not.toBe("");
    expect(api.postMessage).toHaveBeenLastCalledWith(
      "t1",
      "Bin unterwegs",
      firstKey,
    );
  });
});

// Senden zeigt die Nachricht sofort (#5011): markiert, bis die Antwort sie
// bestätigt; ein Fehler markiert sie als nicht gesendet und gibt den Text
// zurück ins Feld.
describe("TeamChatThread optimistic send (#5011)", () => {
  let api: ReturnType<typeof stubApi>;

  beforeEach(() => {
    api = stubApi();
    api.fetchThread.mockResolvedValue(thread);
  });

  // Der Text einer Blase; das Feld trägt nach einem Fehler denselben Text.
  const bubble = (text: string) => screen.getByText(text, { selector: "span" });

  async function typeAndSend(text: string) {
    render(
      <TeamChatThread
        portal={portalFor(api)}
        threadID="t1"
        myAccountId="1"
        back={{ href: "/team-chat", label: "Zurück zum Team-Chat" }}
      />,
      { wrapper: Fresh },
    );
    await screen.findByText("Hallo");
    fireEvent.change(screen.getByLabelText("Nachricht"), {
      target: { value: text },
    });
    const started = performance.now();
    fireEvent.click(screen.getByRole("button", { name: "Senden" }));
    return started;
  }

  it("shows the message within 100 ms, marked as on its way, and clears the field", async () => {
    let answer: (
      value: Awaited<ReturnType<StaffMessagesApi["postMessage"]>>,
    ) => void = () => {};
    const sent = {
      id: "m2",
      sender_account_id: "1",
      sender_name: "Ich",
      body: "Bin unterwegs",
      created_at: "2026-09-09T10:05:00Z",
    };
    api.postMessage.mockReturnValue(
      new Promise((resolve) => {
        answer = resolve;
      }),
    );
    // Der Abgleich nach dem Senden kennt die Nachricht.
    api.fetchThread
      .mockReset()
      .mockResolvedValueOnce(thread)
      .mockResolvedValue({ ...thread, messages: [...thread.messages, sent] });
    const started = await typeAndSend("Bin unterwegs");

    // Kein await: die Blase steht vor der Antwort.
    expect(bubble("Bin unterwegs")).toBeInTheDocument();
    expect(screen.getByText("Wird gesendet")).toBeInTheDocument();
    expect(performance.now() - started).toBeLessThan(100);
    expect(screen.getByLabelText("Nachricht")).toHaveValue("");

    answer(sent);
    await waitFor(() =>
      expect(screen.queryByText("Wird gesendet")).not.toBeInTheDocument(),
    );
    expect(bubble("Bin unterwegs")).toBeInTheDocument();
    await waitFor(() => expect(api.fetchThread).toHaveBeenCalledTimes(2));
    expect(
      screen.getAllByText("Bin unterwegs", { selector: "span" }),
    ).toHaveLength(1);
  });

  it("marks a failed message, puts the text back and shows the error", async () => {
    api.postMessage.mockRejectedValue(
      new ApiError("send kaputt", 503, { code: "general.unavailable" }),
    );
    await typeAndSend("Bin unterwegs");

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Nachricht"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Nicht gesendet")).toBeInTheDocument();
    expect(bubble("Bin unterwegs")).toBeInTheDocument();
    expect(screen.getByLabelText("Nachricht")).toHaveValue("Bin unterwegs");
    expect(screen.queryByText("Wird gesendet")).not.toBeInTheDocument();
  });

  it.each([false, true])(
    "finishes a read-confirmed failed draft without resending it (edited: %s)",
    async (edited) => {
      api.postMessage.mockRejectedValueOnce(
        new ApiError("response lost", 503, { code: "general.unavailable" }),
      );
      await typeAndSend("Danke");
      await screen.findByText("Nicht gesendet");
      const firstKey = api.postMessage.mock.calls[0]?.[2];
      expect(firstKey).toEqual(expect.any(String));
      if (edited) {
        fireEvent.change(screen.getByLabelText("Nachricht"), {
          target: { value: "" },
        });
        fireEvent.change(screen.getByLabelText("Nachricht"), {
          target: { value: "Danke" },
        });
      }
      const saved = {
        id: "m2",
        sender_account_id: "1",
        sender_name: "Ich",
        body: "Danke",
        created_at: "2026-09-09T10:05:00Z",
        client_message_id: firstKey,
      };
      api.fetchThread.mockResolvedValue({
        ...thread,
        messages: [...thread.messages, saved],
      });
      const refresh = vi
        .mocked(useMessagesActivity)
        .mock.calls.at(-1)?.[0].onMatch;
      expect(refresh).toBeTypeOf("function");
      await act(async () => {
        refresh?.();
      });
      await waitFor(() => {
        expect(
          screen.getByText("Danke", { selector: "span" }),
        ).toBeInTheDocument();
        expect(screen.queryByText("Nicht gesendet")).not.toBeInTheDocument();
      });
      expect(screen.getByLabelText("Nachricht")).toHaveValue(
        edited ? "Danke" : "",
      );
      expect(
        screen.getByText("Danke", { selector: "span" }),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Wiederholen" }),
      ).not.toBeInTheDocument();
      if (!edited)
        fireEvent.change(screen.getByLabelText("Nachricht"), {
          target: { value: "Danke" },
        });
      api.postMessage.mockImplementationOnce(async (_thread, body, key) => {
        const next = { ...saved, id: "m3", body, client_message_id: key };
        api.fetchThread.mockResolvedValue({
          ...thread,
          messages: [...thread.messages, saved, next],
        });
        return next;
      });
      fireEvent.click(screen.getByRole("button", { name: "Senden" }));
      await waitFor(() => expect(api.postMessage).toHaveBeenCalledTimes(2));
      expect(api.postMessage.mock.calls[1]?.[2]).toEqual(expect.any(String));
      expect(api.postMessage.mock.calls[1]?.[2]).not.toBe(firstKey);
      await waitFor(() =>
        expect(screen.getAllByText("Danke", { selector: "span" })).toHaveLength(
          2,
        ),
      );
    },
  );

  it("honors a resource refusal even after a read confirmed the send", async () => {
    let rejectSend!: (error: unknown) => void;
    api.postMessage.mockImplementationOnce(
      () =>
        new Promise((_, reject) => {
          rejectSend = reject;
        }),
    );
    await typeAndSend("Danke");
    const key = api.postMessage.mock.calls[0]?.[2];
    api.fetchThread.mockResolvedValue({
      ...thread,
      messages: [
        ...thread.messages,
        {
          id: "m2",
          sender_account_id: "1",
          sender_name: "Ich",
          body: "Danke",
          created_at: "2026-09-09T10:05:00Z",
          client_message_id: key,
        },
      ],
    });
    const refresh = vi
      .mocked(useMessagesActivity)
      .mock.calls.at(-1)?.[0].onMatch;
    expect(refresh).toBeTypeOf("function");
    await act(async () => {
      refresh?.();
    });
    await waitFor(() => {
      expect(
        screen.getByText("Danke", { selector: "span" }),
      ).toBeInTheDocument();
      expect(screen.queryByText("Wird gesendet")).not.toBeInTheDocument();
    });
    await act(async () => {
      rejectSend(
        new ApiError("Forbidden", 403, { code: "general.permission" }),
      );
    });
    expect(
      await screen.findByText(
        catalogText("general.permission", "die Unterhaltung"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Hallo")).not.toBeInTheDocument();
    expect(screen.queryByText("Danke")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Nachricht")).not.toBeInTheDocument();
  });

  it("drops the conversation on a refusal and shows the refusal", async () => {
    api.postMessage.mockRejectedValue(
      new ApiError("Forbidden", 403, { code: "general.permission" }),
    );
    await typeAndSend("Bin unterwegs");

    expect(
      await screen.findByText(
        catalogText("general.permission", "die Unterhaltung"),
      ),
    ).toBeInTheDocument();
    // Verworfen: weder der Verlauf noch die Blase bleiben stehen, und ohne
    // Zutun fragt die Seite nicht erneut.
    expect(screen.queryByText("Hallo")).not.toBeInTheDocument();
    expect(screen.queryByText("Bin unterwegs")).not.toBeInTheDocument();
    expect(api.fetchThread).toHaveBeenCalledTimes(1);
  });

  // The backend answers "not a participant" with 404 on purpose
  // (staffmessages/errors.go): leaving a group chat ends access that way.
  it("drops the conversation when the backend says it is no longer the sender's (404)", async () => {
    api.postMessage.mockRejectedValue(
      new ApiError("Diese Unterhaltung gibt es nicht.", 404, {
        code: "general.input",
      }),
    );
    await typeAndSend("Bin unterwegs");

    expect(
      await screen.findByText(catalogText("general.input", "die Unterhaltung")),
    ).toBeInTheDocument();
    expect(screen.queryByText("Hallo")).not.toBeInTheDocument();
    expect(screen.queryByText("Bin unterwegs")).not.toBeInTheDocument();
    expect(api.fetchThread).toHaveBeenCalledTimes(1);
  });

  it("keeps the conversation and marks the bubble on a failure without a response", async () => {
    api.postMessage.mockRejectedValue(new TypeError("Failed to fetch"));
    await typeAndSend("Bin unterwegs");

    expect(await screen.findByText("Nicht gesendet")).toBeInTheDocument();
    expect(screen.getByText("Hallo")).toBeInTheDocument();
  });

  it("keeps the conversation on a business 403 with its own code", async () => {
    api.postMessage.mockRejectedValue(
      new ApiError("Aus", 403, {
        code: "communication.staff_messaging_disabled",
      }),
    );
    await typeAndSend("Bin unterwegs");

    expect(
      await screen.findByText(
        "Der Team-Chat ist ausgeschaltet. Sie können hier nicht schreiben.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Hallo")).toBeInTheDocument();
  });

  it("shows the composer again once the school switches the chat back on", async () => {
    api.fetchThread.mockResolvedValue(thread);
    api.postMessage.mockRejectedValue(
      new ApiError("Aus", 403, {
        code: "communication.staff_messaging_disabled",
      }),
    );
    const disabledNotice =
      "Der Team-Chat ist ausgeschaltet. Sie können hier nicht schreiben.";
    const view = (flagSaysEnabled: boolean) => (
      <TeamChatThread
        portal={{ ...portalFor(api), flagSaysEnabled }}
        threadID="t1"
        myAccountId="1"
        back={{ href: "/team-chat", label: "Zurück zum Team-Chat" }}
      />
    );
    const { rerender } = render(view(true), { wrapper: Fresh });
    await screen.findByText("Hallo");
    fireEvent.change(screen.getByLabelText("Nachricht"), {
      target: { value: "Bin unterwegs" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Senden" }));
    expect(await screen.findByText(disabledNotice)).toBeInTheDocument();

    // The flag still says "on": the mark stays.
    rerender(view(true));
    // Anker: der Hinweis auf den ausgeschalteten Chat steht noch.
    expect(screen.getByText(disabledNotice)).toBeInTheDocument();
    expect(screen.queryByLabelText("Nachricht")).not.toBeInTheDocument();

    // The school switches the chat off and on again.
    rerender(view(false));
    rerender(view(true));

    expect(await screen.findByLabelText("Nachricht")).toBeInTheDocument();
    expect(screen.getByText("Hallo")).toBeInTheDocument();
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
