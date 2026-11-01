import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import { ApiError } from "~/lib/api-error";
import { SWRConfig } from "~/lib/swr/testing";
import { catalogText } from "~/test/error-catalog-text";

// Echtes SWR: Senden, Verwerfen und Zurücknehmen laufen über den Cache.
vi.unmock("swr");

const { mockFetchGroupThread, mockPostGroupMessage, tenantFlags } = vi.hoisted(
  () => ({
    mockFetchGroupThread: vi.fn(),
    mockPostGroupMessage: vi.fn(),
    tenantFlags: { messagingEnabled: true },
  }),
);

vi.mock("next/navigation", () => ({
  useParams: () => ({ groupThreadId: "4" }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/schule/messages/groups/4",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "7", tenantId: 1 }, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({
    tenant: { messagingEnabled: tenantFlags.messagingEnabled },
  }),
  useTenantSlugSafe: () => "schule",
  useTenantRoutingModeSafe: () => "subdomain",
}));

vi.mock("~/lib/hooks/use-messages-activity", () => ({
  useMessagesActivity: vi.fn(),
}));

// The scaffold by its contract: the page error where the thread is missing,
// otherwise the content.
vi.mock("~/components/ui/tenant-page", () => ({
  TenantPage: ({
    error,
    children,
  }: {
    error?: string | { message: string } | null;
    children?: ReactNode;
  }) => (
    <div>
      {error ? (
        <p>{typeof error === "string" ? error : error.message}</p>
      ) : null}
      {children}
    </div>
  ),
}));

vi.mock("~/lib/parent-group-threads-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-group-threads-api")>()),
  fetchGroupThread: mockFetchGroupThread,
  postGroupMessage: mockPostGroupMessage,
}));

import GroupMessagePage from "./page";

const detail = {
  thread: {
    id: "4",
    target_type: "group",
    target_id: "5",
    target_label: "Gruppe Bären",
    title: "Sommerfest",
    created_at: "2026-09-08T08:00:00Z",
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
  children: [],
};

function Fresh({ children }: { readonly children: ReactNode }) {
  return (
    <ToastProvider>
      <SWRConfig
        value={{
          provider: () => new Map(),
          dedupingInterval: 0,
          shouldRetryOnError: false,
        }}
      >
        {children}
      </SWRConfig>
    </ToastProvider>
  );
}

async function typeAndSend(text: string) {
  const view = render(<GroupMessagePage />, { wrapper: Fresh });
  await screen.findByText("Wer bringt einen Kuchen mit?");
  fireEvent.change(screen.getByLabelText("Nachricht"), {
    target: { value: text },
  });
  fireEvent.click(screen.getByRole("button", { name: "Senden" }));
  return view;
}

// Gruppennachricht (#5011): POST /api/messages/group-threads/{id}/messages.
describe("Gruppennachricht senden: Ablehnung je Code", () => {
  beforeEach(() => {
    mockFetchGroupThread.mockReset().mockResolvedValue(detail);
    mockPostGroupMessage.mockReset();
    tenantFlags.messagingEnabled = true;
  });

  it.each([false, true])(
    "reconciles a saved send after a lost response (edited: %s)",
    async (edited) => {
      mockPostGroupMessage.mockRejectedValueOnce(
        new ApiError("response lost", 503, { code: "general.unavailable" }),
      );
      await typeAndSend("Danke");
      await screen.findByText("Nicht gesendet");
      const key = mockPostGroupMessage.mock.calls[0]?.[2];
      expect(key).toEqual(expect.any(String));
      if (edited)
        fireEvent.change(screen.getByLabelText("Nachricht"), {
          target: { value: "Neuer Entwurf" },
        });
      const saved = {
        id: "saved",
        sender_kind: "staff",
        sender_name: "Ich",
        body: "Danke",
        kind: "message",
        created_at: "2026-09-09T10:05:00Z",
        client_message_id: key,
      };
      mockFetchGroupThread.mockResolvedValue({
        ...detail,
        messages: [...detail.messages, saved],
      });
      const refresh = vi
        .mocked(useMessagesActivity)
        .mock.calls.at(-1)?.[0].onMatch;
      expect(refresh).toBeTypeOf("function");
      await act(async () => {
        refresh?.();
      });
      await waitFor(() => {
        expect(screen.getAllByText("Danke", { selector: "span" })).toHaveLength(
          1,
        );
        expect(screen.queryByText("Nicht gesendet")).not.toBeInTheDocument();
      });
      expect(screen.getByLabelText("Nachricht")).toHaveValue(
        edited ? "Neuer Entwurf" : "",
      );
      expect(mockPostGroupMessage).toHaveBeenCalledTimes(1);
    },
  );

  it("behält den Verlauf und sperrt Senden, wenn die Schule Nachrichten ausschaltet", async () => {
    mockPostGroupMessage.mockRejectedValue(
      new ApiError("messaging disabled", 403, {
        code: "communication.parent_messaging_disabled",
      }),
    );
    await typeAndSend("Bitte Kuchen mitbringen");

    expect(
      await screen.findByText(
        /Die Eltern-Nachrichten sind für diese Schule ausgeschaltet/,
      ),
    ).toBeInTheDocument();
    // Anker: der geladene Verlauf und die nicht gesendete Blase bleiben sichtbar.
    expect(
      screen.getByText("Wer bringt einen Kuchen mit?"),
    ).toBeInTheDocument();
    expect(screen.getByText("Nicht gesendet")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Senden" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Nachricht")).not.toBeInTheDocument();
    expect(mockPostGroupMessage).toHaveBeenCalledTimes(1);
  });

  it("zeigt das Schreibfeld wieder, wenn die Schule Nachrichten wieder einschaltet", async () => {
    mockPostGroupMessage.mockRejectedValue(
      new ApiError("messaging disabled", 403, {
        code: "communication.parent_messaging_disabled",
      }),
    );
    const view = await typeAndSend("Bitte Kuchen mitbringen");
    expect(
      await screen.findByText(
        /Die Eltern-Nachrichten sind für diese Schule ausgeschaltet/,
      ),
    ).toBeInTheDocument();

    // Die Einstellung meldet weiter „an“: die Sperre bleibt.
    view.rerender(<GroupMessagePage />);
    // Anker: der Ausschalt-Hinweis steht noch.
    expect(
      screen.getByText(
        /Die Eltern-Nachrichten sind für diese Schule ausgeschaltet/,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Nachricht")).not.toBeInTheDocument();

    // Die Schule schaltet aus und wieder ein.
    tenantFlags.messagingEnabled = false;
    view.rerender(<GroupMessagePage />);
    tenantFlags.messagingEnabled = true;
    view.rerender(<GroupMessagePage />);

    expect(await screen.findByLabelText("Nachricht")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Senden" })).toBeInTheDocument();
    expect(
      screen.getByText("Wer bringt einen Kuchen mit?"),
    ).toBeInTheDocument();
  });

  it("verwirft die Gruppennachricht bei 404, wenn es sie nicht mehr gibt", async () => {
    mockPostGroupMessage.mockRejectedValue(
      new ApiError("group thread not found", 404, { code: "general.input" }),
    );
    await typeAndSend("Bitte Kuchen mitbringen");

    expect(
      await screen.findByText(
        catalogText("general.input", "die Gruppennachricht"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Wer bringt einen Kuchen mit?"),
    ).not.toBeInTheDocument();
    expect(mockFetchGroupThread).toHaveBeenCalledTimes(1);
  });

  it("verwirft die Gruppennachricht bei 403 general.permission", async () => {
    mockPostGroupMessage.mockRejectedValue(
      new ApiError("Forbidden", 403, { code: "general.permission" }),
    );
    await typeAndSend("Bitte Kuchen mitbringen");

    expect(
      await screen.findByText(
        catalogText("general.permission", "die Gruppennachricht"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Wer bringt einen Kuchen mit?"),
    ).not.toBeInTheDocument();
  });

  it("behält die Gruppennachricht, wenn sie inzwischen geschlossen ist (409)", async () => {
    mockPostGroupMessage.mockRejectedValue(
      new ApiError("closed", 409, {
        code: "communication.group_thread_closed",
      }),
    );
    await typeAndSend("Bitte Kuchen mitbringen");

    expect(await screen.findByText("Nicht gesendet")).toBeInTheDocument();
    expect(
      screen.getByText("Wer bringt einen Kuchen mit?"),
    ).toBeInTheDocument();
  });
});
