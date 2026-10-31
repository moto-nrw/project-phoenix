import { fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { SWRConfig } from "~/lib/swr/testing";
import { catalogText } from "~/test/error-catalog-text";

// Echtes SWR: Senden, Verwerfen und Zurücknehmen laufen über den Cache.
vi.unmock("swr");

const { mockFetchThread, mockPostMessage, tenantFlags } = vi.hoisted(() => ({
  mockFetchThread: vi.fn(),
  mockPostMessage: vi.fn(),
  tenantFlags: { messagingEnabled: true },
}));

vi.mock("next/navigation", () => ({
  useParams: () => ({ threadId: "t1" }),
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
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn() }),
}));

vi.mock("~/lib/hooks/use-messages-activity", () => ({
  useMessagesActivity: vi.fn(),
}));

vi.mock("~/lib/hooks/use-chat-viewport-lock", () => ({
  useChatViewportLock: () => ({ current: null }),
}));

vi.mock("~/components/messaging/pickup-request-detail-modal", () => ({
  PickupRequestDetailModal: () => null,
}));

// The scaffold by its contract: the page error where the thread is missing,
// otherwise the content.
vi.mock("~/components/ui/tenant-page", () => ({
  TenantPage: ({
    error,
    children,
  }: {
    error?: string | { message: string };
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

vi.mock("~/lib/parent-messages-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-messages-api")>()),
  fetchThread: mockFetchThread,
  postMessage: mockPostMessage,
}));

import MessageThreadPage from "./page";

const thread = {
  thread_id: "t1",
  student_id: "42",
  student_name: "Max Muster",
  guardian_name: "Anna Muster",
  messages: [
    {
      id: "m1",
      sender_kind: "guardian",
      sender_name: "Anna Muster",
      body: "Hallo",
      created_at: "2026-09-09T10:00:00Z",
      kind: "message",
    },
  ],
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
  const view = render(<MessageThreadPage />, { wrapper: Fresh });
  await screen.findByText("Hallo");
  fireEvent.change(screen.getByLabelText("Nachricht"), {
    target: { value: text },
  });
  fireEvent.click(screen.getByRole("button", { name: "Senden" }));
  return view;
}

// Eltern-Nachricht im OGS-Portal (#5011): POST /api/messages/threads/{id}.
describe("Eltern-Nachricht senden: Ablehnung je Code", () => {
  beforeEach(() => {
    mockFetchThread.mockReset().mockResolvedValue(thread);
    mockPostMessage.mockReset();
    tenantFlags.messagingEnabled = true;
  });

  it("behält den Verlauf und sperrt Senden, wenn die Schule Nachrichten ausschaltet", async () => {
    mockPostMessage.mockRejectedValue(
      new ApiError("messaging disabled", 403, {
        code: "communication.parent_messaging_disabled",
      }),
    );
    await typeAndSend("Bis morgen");

    expect(
      await screen.findByText(
        /Die Eltern-Nachrichten sind für diese Schule ausgeschaltet/,
      ),
    ).toBeInTheDocument();
    // Anker: der geladene Verlauf und die nicht gesendete Blase bleiben sichtbar.
    expect(screen.getByText("Hallo")).toBeInTheDocument();
    expect(screen.getByText("Nicht gesendet")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Senden" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Nachricht")).not.toBeInTheDocument();
    expect(mockPostMessage).toHaveBeenCalledTimes(1);
  });

  it("zeigt das Schreibfeld wieder, wenn die Schule Nachrichten wieder einschaltet", async () => {
    mockPostMessage.mockRejectedValue(
      new ApiError("messaging disabled", 403, {
        code: "communication.parent_messaging_disabled",
      }),
    );
    const view = await typeAndSend("Bis morgen");
    expect(
      await screen.findByText(
        /Die Eltern-Nachrichten sind für diese Schule ausgeschaltet/,
      ),
    ).toBeInTheDocument();

    // Die Einstellung meldet weiter „an“: die Sperre bleibt.
    view.rerender(<MessageThreadPage />);
    // Anker: der Ausschalt-Hinweis steht noch.
    expect(
      screen.getByText(
        /Die Eltern-Nachrichten sind für diese Schule ausgeschaltet/,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Nachricht")).not.toBeInTheDocument();

    // Die Schule schaltet aus und wieder ein.
    tenantFlags.messagingEnabled = false;
    view.rerender(<MessageThreadPage />);
    tenantFlags.messagingEnabled = true;
    view.rerender(<MessageThreadPage />);

    expect(await screen.findByLabelText("Nachricht")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Senden" })).toBeInTheDocument();
    expect(screen.getByText("Hallo")).toBeInTheDocument();
  });

  it("verwirft die Unterhaltung bei 403 general.permission und zeigt die Ablehnung", async () => {
    mockPostMessage.mockRejectedValue(
      new ApiError("Forbidden", 403, { code: "general.permission" }),
    );
    await typeAndSend("Bis morgen");

    expect(
      await screen.findByText(
        catalogText("general.permission", "die Unterhaltung"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Hallo")).not.toBeInTheDocument();
    expect(screen.queryByText("Bis morgen")).not.toBeInTheDocument();
    expect(mockFetchThread).toHaveBeenCalledTimes(1);
  });

  it("verwirft die Unterhaltung bei 404, wenn es sie nicht mehr gibt", async () => {
    mockPostMessage.mockRejectedValue(
      new ApiError("thread not found", 404, { code: "general.input" }),
    );
    await typeAndSend("Bis morgen");

    expect(
      await screen.findByText(catalogText("general.input", "die Unterhaltung")),
    ).toBeInTheDocument();
    expect(screen.queryByText("Hallo")).not.toBeInTheDocument();
  });

  it("behält die Unterhaltung bei einem fachlichen 403 und markiert die Blase", async () => {
    mockPostMessage.mockRejectedValue(
      new ApiError("Leitung nicht erreichbar", 403, {
        code: "communication.leadership_messages_unavailable",
      }),
    );
    await typeAndSend("Bis morgen");

    expect(await screen.findByText("Nicht gesendet")).toBeInTheDocument();
    expect(screen.getByText("Hallo")).toBeInTheDocument();
    expect(screen.getByLabelText("Nachricht")).toHaveValue("Bis morgen");
  });
});
