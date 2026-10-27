import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useSession } from "next-auth/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import StaffDetailContent from "./page";

// Echtes SWR mit dem globalen Cache (#4993): Der Test zählt die Abrufe, die
// die Seite tatsächlich auslöst. Jeder Test nimmt eine eigene Personal-ID,
// damit kein Cache-Eintrag eines anderen Tests mitzählt.
vi.mock("swr", async (importOriginal) => importOriginal<object>());

const route = vi.hoisted(() => ({ id: "0" }));
const server = vi.hoisted(() => ({
  firstName: "Mila",
  notes: null as string | null,
  requests: [] as string[],
}));
const { mockUpdateTeacher } = vi.hoisted(() => ({
  mockUpdateTeacher: vi.fn(
    (_id: string, _data: { first_name?: string; staff_notes?: string }) =>
      Promise.resolve(),
  ),
}));

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useParams: () => ({ id: route.id }),
  useSearchParams: () => new URLSearchParams("tab=konto"),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: vi.fn(), error: vi.fn() }),
}));

vi.mock("~/lib/teacher-api", () => ({
  teacherService: {
    updateTeacher: mockUpdateTeacher,
    deleteTeacher: vi.fn(),
  },
}));

function staffPayload(id: string) {
  return {
    id,
    started_on: "2024-09-02",
    ended_on: null,
    name: `${server.firstName} Muster`,
    firstName: server.firstName,
    lastName: "Muster",
    first_name: server.firstName,
    last_name: "Muster",
    role: "Betreuung",
    account_role: "teacher",
    qualifications: null,
    staff_notes: server.notes,
    email: "mila@example.test",
    tag_id: null,
    account_id: "7",
    created_at: "2024-09-02T08:00:00Z",
    updated_at: "2024-09-02T08:00:00Z",
  };
}

// Beide Wege der Seite zur API (sessionFetch und der CRUD-Dienst der
// Datenverwaltung) landen hier und werden mitgeschrieben.
function respond(input: RequestInfo | URL): Promise<Response> {
  const raw =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  const path = new URL(raw, "http://localhost").pathname;
  server.requests.push(path);
  const detail = /^\/api\/staff\/(\d+)$/.exec(path);
  const profile = /^\/api\/staff\/financial-profile\/(\d+)$/.exec(path);
  const body = detail?.[1]
    ? { data: staffPayload(detail[1]) }
    : profile?.[1]
      ? {
          data: {
            id: Number(profile[1]),
            name: `${server.firstName} Muster`,
            firstName: server.firstName,
            lastName: "Muster",
            started_on: "2024-09-02",
            ended_on: null,
          },
        }
      : { data: [], pagination: null };
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

vi.mock("~/lib/session-cache", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/session-cache")>()),
  getCachedSession: () =>
    Promise.resolve({
      user: { id: "7" },
      expires: "2099-01-01T00:00:00.000Z",
    }),
  sessionFetch: (input: string) => respond(input),
}));

function signIn(permissions: string[]) {
  vi.mocked(useSession).mockReturnValue({
    data: {
      user: { id: "7", roles: ["teacher"], permissions },
      expires: "2099-01-01T00:00:00.000Z",
    },
    status: "authenticated",
    update: vi.fn(),
  });
}

function recordRequests(id: string) {
  return server.requests.filter((path) => path === `/api/staff/${id}`);
}

describe("StaffDetailContent: Personal-Datensatz (#4993)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    server.firstName = "Mila";
    server.notes = null;
    server.requests = [];
    window.scrollTo = vi.fn();
    vi.stubGlobal("fetch", vi.fn(respond));
  });

  it("lädt GET /api/staff/{id} für Kopf und Konto-Reiter nur einmal", async () => {
    route.id = "101";
    signIn(["users:read"]);

    render(<StaffDetailContent />);

    expect(
      await screen.findByRole("heading", { name: "Mila Muster" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("mila@example.test")).toBeInTheDocument();
    expect(recordRequests("101")).toHaveLength(1);
  });

  it("lädt neben dem Finanzprofil den Datensatz nur für den Konto-Reiter", async () => {
    route.id = "102";
    signIn(["staff:financial", "users:read"]);

    render(<StaffDetailContent />);

    expect(await screen.findByText("mila@example.test")).toBeInTheDocument();
    expect(server.requests).toContain("/api/staff/financial-profile/102");
    expect(recordRequests("102")).toHaveLength(1);
  });

  it("aktualisiert Kopf und Konto-Reiter nach dem Speichern", async () => {
    route.id = "103";
    signIn(["staff:manage", "users:update"]);

    render(<StaffDetailContent />);

    fireEvent.click(await screen.findByRole("button", { name: "Bearbeiten" }));
    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Milena" },
    });
    fireEvent.change(screen.getByLabelText("Notizen der Leitung"), {
      target: { value: "Schlüssel für Raum 3" },
    });
    mockUpdateTeacher.mockImplementationOnce((_id, data) => {
      server.firstName = data.first_name ?? server.firstName;
      server.notes = data.staff_notes ?? server.notes;
      return Promise.resolve();
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    // Kopf und Konto-Reiter zeigen, was der neue Abruf liefert.
    expect(
      await screen.findByRole("heading", { name: "Milena Muster" }),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByText("Schlüssel für Raum 3")).toBeInTheDocument(),
    );
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();
    // Ein Abruf beim Öffnen, einer nach dem Speichern.
    expect(recordRequests("103")).toHaveLength(2);
  });
});
