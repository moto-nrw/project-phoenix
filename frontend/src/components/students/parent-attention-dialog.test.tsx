import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ParentAttentionDialog } from "./parent-attention-dialog";
import type {
  InboxThread,
  Message,
  ThreadDetail,
} from "~/lib/parent-messages-api";

const mocks = vi.hoisted(() => ({
  threads: vi.fn<() => Promise<InboxThread[]>>(),
  detail: vi.fn<(id: string) => Promise<ThreadDetail>>(),
  confirm: vi.fn<(id: string, boundary: string) => Promise<void>>(),
  loadError:
    vi.fn<
      (
        error: unknown,
        options: { object: string; retry: () => Promise<void> },
      ) => Promise<void>
    >(),
  actionError: vi.fn(),
  clear: vi.fn(),
  push: vi.fn(),
}));
vi.mock("~/lib/parent-messages-api", async (original) => ({
  ...(await original<typeof import("~/lib/parent-messages-api")>()),
  fetchStudentThreads: mocks.threads,
  fetchThread: mocks.detail,
  confirmThreadForTeam: mocks.confirm,
}));
vi.mock("~/components/students/aggregated-request-list", () => ({
  AggregatedRequestList: () => null,
}));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mocks.push }),
}));
vi.mock("~/contexts/ToastContext", () => ({
  useApiLoadError: () => ({
    error: null,
    show: mocks.loadError,
    clear: mocks.clear,
  }),
  useApiErrorDisplay: () => ({ show: mocks.actionError }),
}));

function thread(id: string): InboxThread {
  return {
    thread_id: id,
    student_id: "7",
    student_name: "Mia",
    guardian_name: `Eltern ${id}`,
    unread_count: 1,
  };
}
function message(
  id: string,
  body: string,
  extra: Partial<Message> = {},
): Message {
  return {
    id,
    body,
    sender_kind: "guardian",
    sender_name: "Eltern",
    created_at: "2026-09-09T08:00:00Z",
    ...extra,
  };
}
function detail(id: string, messages: Message[]): ThreadDetail {
  return {
    thread_id: id,
    student_id: "7",
    student_name: "Mia",
    guardian_name: `Eltern ${id}`,
    messages,
  };
}
function openDialog() {
  render(
    <ParentAttentionDialog
      studentId="7"
      studentName="Mia"
      attention={{ messages: 2, requests: 0, total: 2 }}
      isOpen
      onClose={vi.fn()}
    />,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.threads.mockResolvedValue([thread("a")]);
  mocks.confirm.mockResolvedValue(undefined);
  mocks.loadError.mockResolvedValue(undefined);
});

describe("parent attention conversation snapshots", () => {
  it("shows every parent message in the detail snapshot, including arrivals after the older count", async () => {
    mocks.detail.mockResolvedValue(
      detail("a", [
        message("old", "Bisherige Nachricht"),
        message("first", "Erste neue Nachricht"),
        message("second", "Inzwischen eingetroffen"),
        message("staff-event", "Vom Team bestätigt", {
          sender_kind: "system",
          event_type: "request_status",
          event_actor_kind: "staff",
        }),
      ]),
    );
    openDialog();
    expect(await screen.findByText("Erste neue Nachricht")).toBeInTheDocument();
    expect(screen.getByText("Inzwischen eingetroffen")).toBeInTheDocument();
    expect(screen.getByText("Bisherige Nachricht")).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Als gelesen bestätigen" }),
    );
    await waitFor(() =>
      expect(mocks.confirm).toHaveBeenCalledWith("a", "second"),
    );
  });

  it("makes attachments accessible through the existing viewer before confirmation", async () => {
    mocks.detail.mockResolvedValue(
      detail("a", [
        message("image", "", {
          attachments: [
            {
              id: "55",
              filename: "Plan.png",
              content_type: "image/png",
              size_bytes: 120,
            },
            {
              id: "56",
              filename: "Brief.pdf",
              content_type: "application/pdf",
              size_bytes: 240,
            },
          ],
        }),
      ]),
    );
    openDialog();
    expect(
      await screen.findByRole("button", { name: "Bild öffnen: Plan.png" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Datei öffnen: Brief.pdf" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("link", { name: "Herunterladen: Brief.pdf" }),
    ).toHaveAttribute("href", "/api/messages/attachments/56/download");
  });

  it("keeps successful details visible and retries only the failed conversations", async () => {
    mocks.threads.mockResolvedValue([thread("a"), thread("b")]);
    mocks.detail.mockImplementation(async (id) => {
      if (id === "b") throw new Error("unavailable");
      return detail(id, [message("a-1", "Erfolgreich geladen")]);
    });
    openDialog();
    expect(await screen.findByText("Erfolgreich geladen")).toBeInTheDocument();
    await waitFor(() => expect(mocks.loadError).toHaveBeenCalledOnce());
    mocks.detail.mockResolvedValue(
      detail("b", [message("b-1", "Nachgeladen")]),
    );
    const retry = mocks.loadError.mock.calls[0]?.[1].retry;
    expect(retry).toBeDefined();
    await act(async () => {
      await retry?.();
    });
    expect(await screen.findByText("Nachgeladen")).toBeInTheDocument();
    expect(screen.getByText("Erfolgreich geladen")).toBeInTheDocument();
    expect(mocks.threads).toHaveBeenCalledOnce();
    expect(mocks.detail.mock.calls.map(([id]) => id)).toEqual(["a", "b", "b"]);
  });
});
