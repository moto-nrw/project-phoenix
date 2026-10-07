import { render as rtlRender, screen } from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";

// #3886: the thread is readable school-wide, the request detail only within
// the review scope. "Anfrage ansehen" may only appear where the scope reaches
// the child; otherwise the click ends in a 403.

function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

const { mockUseSWR, mockUseSWRAuth, mockSession } = vi.hoisted(() => ({
  mockUseSWR: vi.fn(),
  mockUseSWRAuth: vi.fn(),
  mockSession: {
    current: null as null | {
      user: { id: string; roles: string[]; permissions: string[] };
    },
  },
}));

vi.mock("next/navigation", () => ({
  useParams: () => ({ threadId: "t1" }),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: mockSession.current }),
}));

vi.mock("swr", () => ({
  default: (...args: unknown[]) => mockUseSWR(...args),
  unstable_serialize: (key: unknown) => JSON.stringify(key),
  useSWRConfig: () => ({ cache: new Map() }),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (...args: unknown[]) => mockUseSWRAuth(...args),
}));

const thread = {
  thread_id: "t1",
  student_id: "42",
  student_name: "Felix Schneider",
  guardian_name: "Anna Schneider",
  messages: [
    {
      id: "m1",
      sender_kind: "system",
      sender_name: "moto",
      body: "Abholzeit angefragt: 30.09.2026, 14:30 Uhr",
      created_at: "2026-09-26T08:00:00Z",
      kind: "event",
      event_type: "request_created",
      request_type: "pickup_change",
      ref_table: "schedule.care_schedule_change_requests",
      ref_id: "7",
    },
    {
      id: "m2",
      sender_kind: "system",
      sender_name: "moto",
      body: "Entschuldigung angefragt",
      created_at: "2026-09-26T09:00:00Z",
      kind: "event",
      event_type: "request_created",
      request_type: "excused_absence",
      ref_table: "active.excused_absence_requests",
      ref_id: "8",
    },
  ],
};

vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({ tenant: { messagingEnabled: true } }),
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

vi.mock("~/components/ui/back-button", () => ({
  BackButton: () => null,
}));

vi.mock("~/components/messaging/pickup-request-detail-modal", () => ({
  PickupRequestDetailModal: () => null,
}));

vi.mock("~/components/messaging/message-composer", () => ({
  MessageComposer: () => null,
}));

vi.mock("~/components/ui/tenant-page", () => ({
  TenantPage: ({ children }: { children?: ReactNode }) => <div>{children}</div>,
}));

import MessageThreadPage from "./page";

function staff(permissions: string[]) {
  mockSession.current = {
    user: { id: "5", roles: ["teacher"], permissions },
  };
}

function coverage(data: { requests: boolean; absences: boolean } | undefined) {
  mockUseSWRAuth.mockReturnValue({ data });
}

describe("Anfrage-Aktionen im Nachrichtenverlauf (#3886)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSWR.mockReturnValue({
      data: thread,
      error: undefined,
      isLoading: false,
      isValidating: false,
      mutate: vi.fn(),
    });
  });

  it("zeigt keinen Button, wenn der Prüfbereich das Kind nicht abdeckt", () => {
    staff(["users:read", "users:update"]);
    coverage({ requests: false, absences: false });

    render(<MessageThreadPage />);

    expect(screen.getByText(/Abholzeit angefragt/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Anfrage ansehen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Anfrage bearbeiten" }),
    ).not.toBeInTheDocument();
    expect(mockUseSWRAuth).toHaveBeenCalledWith(
      "request-review-coverage:5:42",
      expect.any(Function),
      expect.any(Object),
    );
  });

  it("zeigt keinen Button, solange die Abdeckung noch lädt", () => {
    staff(["users:read", "users:update"]);
    coverage(undefined);

    render(<MessageThreadPage />);

    expect(
      screen.queryByRole("button", { name: "Anfrage ansehen" }),
    ).not.toBeInTheDocument();
  });

  it("zeigt „Anfrage ansehen“, wenn der Prüfbereich das Kind abdeckt", () => {
    staff(["users:read", "users:update"]);
    coverage({ requests: true, absences: false });

    render(<MessageThreadPage />);

    expect(
      screen.getByRole("button", { name: "Anfrage ansehen" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Anfrage bearbeiten" }),
    ).not.toBeInTheDocument();
  });

  it("folgt bei Entschuldigungen dem eigenen Prüfbereich für Abwesenheiten", () => {
    staff(["users:read", "users:update"]);
    coverage({ requests: false, absences: true });

    render(<MessageThreadPage />);

    expect(
      screen.queryByRole("button", { name: "Anfrage ansehen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Anfrage bearbeiten" }),
    ).toBeInTheDocument();
  });

  it("fragt ohne users:update nichts ab und zeigt keinen Button", () => {
    staff(["users:read"]);
    coverage({ requests: true, absences: true });

    render(<MessageThreadPage />);

    expect(mockUseSWRAuth).toHaveBeenCalledWith(
      null,
      expect.any(Function),
      expect.any(Object),
    );
    expect(
      screen.queryByRole("button", { name: "Anfrage ansehen" }),
    ).not.toBeInTheDocument();
  });

  it("braucht für die Schulleitung keine Abfrage", () => {
    mockSession.current = {
      user: { id: "1", roles: ["admin"], permissions: ["admin:*"] },
    };
    coverage(undefined);

    render(<MessageThreadPage />);

    expect(mockUseSWRAuth).toHaveBeenCalledWith(
      null,
      expect.any(Function),
      expect.any(Object),
    );
    expect(
      screen.getByRole("button", { name: "Anfrage ansehen" }),
    ).toBeInTheDocument();
  });
});
