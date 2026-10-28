import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { SWRConfig } from "~/lib/swr";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  StaffInboxThread,
  StaffMessagesApi,
  StaffThreadDetail,
} from "~/lib/staff-messages-api";
import type { TeamChatPortal } from "~/lib/team-chat-portal";

import { TeamChatInbox } from "./team-chat-inbox";
import { TeamChatThread } from "./team-chat-thread";

// Gruppenchats im Team-Chat (#4776).

// Echtes SWR statt des globalen Stubs.
vi.mock("swr", async (importOriginal) => await importOriginal());

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

function portalFor(
  api: StaffMessagesApi,
  kind: "tenant" | "school",
): TeamChatPortal {
  return {
    kind,
    api,
    cacheScope: `test-${kind}`,
    inboxHref: "/team-chat",
    threadHref: (id) => `/team-chat/${id}`,
    navigate: vi.fn(),
    flagSaysEnabled: true,
    title: "Team-Chat",
    emptyDescription: "Hier erscheinen Ihre Unterhaltungen.",
    recipientHint: "Sie erreichen hier Ihr Team.",
  };
}

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

const group: StaffThreadDetail = {
  thread_id: "g1",
  counterpart_account_id: "",
  counterpart_name: "",
  counterpart_role_kind: "staff",
  kind: "group",
  group_name: "Frühbetreuung",
  can_manage: true,
  members: [
    { account_id: "1", name: "Ben Becker", role_kind: "staff" },
    { account_id: "2", name: "Anna Arndt", role_kind: "staff" },
    { account_id: "3", name: "Lena Lang", role_kind: "lehrkraft" },
  ],
  messages: [
    {
      id: "m1",
      sender_account_id: "1",
      sender_name: "Ben Becker",
      body: "Wer übernimmt Montag?",
      created_at: "2026-09-09T08:00:00Z",
      read_by_account_ids: ["3"],
      eligible_reader_account_ids: ["2", "3"],
      reader_count: 2,
    },
    {
      id: "m2",
      sender_account_id: "3",
      sender_name: "Lena Lang",
      body: "Ich kann.",
      created_at: "2026-09-09T08:05:00Z",
      read_by_account_ids: [],
      eligible_reader_account_ids: ["1", "2"],
      reader_count: 2,
    },
  ],
};

function renderThread(api: StaffMessagesApi, kind: "tenant" | "school") {
  const portal = portalFor(api, kind);
  render(
    <TeamChatThread
      portal={portal}
      threadID="g1"
      myAccountId="1"
      back={{ href: "/team-chat", label: "Zurück zum Team-Chat" }}
    />,
    { wrapper: Fresh },
  );
  return portal;
}

describe("Team-Chat groups (#4776)", () => {
  let api: ReturnType<typeof stubApi>;

  beforeEach(() => {
    api = stubApi();
  });

  it("titles the chat with the group name and names the readers on click", async () => {
    api.fetchThread.mockResolvedValue(group);
    renderThread(api, "school");

    expect(
      await screen.findByRole("heading", { name: "Frühbetreuung" }),
    ).toBeInTheDocument();
    // Only the viewer's own message carries the count.
    const receipt = screen.getByRole("button", { name: "1 von 2 gelesen" });
    expect(screen.getAllByRole("button", { name: /gelesen/ })).toHaveLength(1);

    fireEvent.click(receipt);
    expect(await screen.findByText("Lena Lang")).toBeInTheDocument();
    expect(screen.getByText("Noch nicht gelesen")).toBeInTheDocument();
    expect(screen.getByText("Anna Arndt")).toBeInTheDocument();
  });

  it("does not list people who joined after a message as unread", async () => {
    api.fetchThread.mockResolvedValue({
      ...group,
      members: [
        ...(group.members ?? []),
        { account_id: "4", name: "Mira Müller", role_kind: "staff" },
      ],
    });
    renderThread(api, "tenant");

    fireEvent.click(
      await screen.findByRole("button", { name: "1 von 2 gelesen" }),
    );
    expect(await screen.findByText("Anna Arndt")).toBeInTheDocument();
    expect(screen.queryByText("Mira Müller")).not.toBeInTheDocument();
  });

  it("lets a manager remove a member after confirming", async () => {
    api.fetchThread.mockResolvedValue(group);
    api.removeGroupMember.mockResolvedValue({
      ...group,
      members: group.members?.filter((m) => m.account_id !== "2"),
    });
    renderThread(api, "tenant");

    fireEvent.click(
      await screen.findByRole("button", { name: "Mitglieder (3)" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Anna Arndt" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", {
        name: "Aus dem Gruppenchat entfernen",
      }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Entfernen" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Ja, entfernen" }),
    );

    await waitFor(() =>
      expect(api.removeGroupMember).toHaveBeenCalledWith("g1", "2"),
    );
    expect(
      await screen.findByRole("button", { name: "Zum Verlauf" }),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByText("Anna Arndt")).not.toBeInTheDocument(),
    );
    expect(screen.getByText("Lena Lang")).toBeInTheDocument();
  });

  it("offers a Lehrkraft only leaving, then returns to the inbox", async () => {
    api.fetchThread.mockResolvedValue({ ...group, can_manage: false });
    api.leaveGroup.mockResolvedValue(undefined);
    const portal = renderThread(api, "school");

    fireEvent.click(
      await screen.findByRole("button", { name: "Mitglieder (3)" }),
    );
    expect(screen.getByText("Lena Lang")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Personen hinzufügen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Aktionen für/ }),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Gruppenchat verlassen" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Verlassen" }));

    await waitFor(() => expect(api.leaveGroup).toHaveBeenCalledWith("g1"));
    await waitFor(() =>
      expect(portal.navigate).toHaveBeenCalledWith("/team-chat"),
    );
  });

  it("explains who may change a group", async () => {
    api.fetchThread.mockResolvedValue({ ...group, can_manage: false });
    renderThread(api, "tenant");

    fireEvent.click(
      await screen.findByRole("button", { name: "Mitglieder (3)" }),
    );
    expect(
      screen.getByText(
        (_, element) =>
          element?.tagName === "P" &&
          element.textContent ===
            "Nur Mitglieder dürfen den Namen und die Mitglieder ändern. Das dürfen die Person, die den Gruppenchat angelegt hat, und die OGS-Leitung.",
      ),
    ).toBeInTheDocument();
  });
});

describe("Team-Chat inbox with groups (#4776)", () => {
  const groupRow: StaffInboxThread = {
    thread_id: "g1",
    counterpart_account_id: "",
    counterpart_name: "",
    counterpart_role_kind: "staff",
    kind: "group",
    group_name: "Frühbetreuung",
    member_count: 4,
    last_message_body: "Ich kann.",
    last_message_at: "2026-09-09T08:05:00Z",
    last_message_mine: false,
    last_sender_name: "Lena Lang",
    unread_count: 1,
  };

  it("lists a group with its name, size and last sender", async () => {
    const api = stubApi();
    api.fetchInbox.mockResolvedValue([groupRow]);
    render(<TeamChatInbox portal={portalFor(api, "school")} />, {
      wrapper: Fresh,
    });

    expect(await screen.findByText("Frühbetreuung")).toBeInTheDocument();
    expect(screen.getByText("Gruppenchat · 4 Mitglieder")).toBeInTheDocument();
    expect(screen.getByText("Lena:")).toBeInTheDocument();
  });

  it("offers groups only in the OGS portal", async () => {
    const tenantApi = stubApi();
    tenantApi.fetchInbox.mockResolvedValue([groupRow]);
    const { unmount } = render(
      <TeamChatInbox portal={portalFor(tenantApi, "tenant")} />,
      { wrapper: Fresh },
    );
    expect(
      await screen.findByRole("button", { name: "Neuer Gruppenchat" }),
    ).toBeInTheDocument();
    unmount();

    const schoolApi = stubApi();
    schoolApi.fetchInbox.mockResolvedValue([groupRow]);
    render(<TeamChatInbox portal={portalFor(schoolApi, "school")} />, {
      wrapper: Fresh,
    });
    expect(
      await screen.findByRole("button", { name: "Neue Nachricht" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Neuer Gruppenchat" }),
    ).not.toBeInTheDocument();
  });
});
