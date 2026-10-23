import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useSession } from "next-auth/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useIsMobile } from "~/components/ui/hooks/useIsMobile";

import TeachersPage from "./page";

// #4573: externe Personen in der Personalverwaltung. Die echte Liste
// (StaffList) und die echten Dialoge laufen; nur Daten, Sitzung und das
// Seitengerüst sind ersetzt.

const mocks = vi.hoisted(() => ({
  inviteExternal: vi.fn(),
  createExternal: vi.fn(),
  refresh: vi.fn(),
  toastSuccess: vi.fn(),
}));

vi.mock("next-auth/react", () => ({ useSession: vi.fn() }));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: vi.fn() })),
  usePathname: vi.fn(() => "/tenant/database/personal"),
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(() => ({
    data: [
      {
        id: "1",
        name: "Anna Müller",
        first_name: "Anna",
        last_name: "Müller",
        account_role: "teacher",
      },
      {
        id: "7",
        name: "Elli Alt",
        first_name: "Elli",
        last_name: "Alt",
        account_role: null,
        is_external: true,
        external_organization: "Sportverein",
        external_account_status: "none",
      },
      {
        id: "8",
        name: "Jonas Neu",
        first_name: "Jonas",
        last_name: "Neu",
        account_role: null,
        is_external: true,
        external_organization: null,
        external_account_status: "invitation_pending",
      },
    ],
    isLoading: false,
    error: null,
    mutate: mocks.refresh,
  })),
  mutate: vi.fn(),
  useTenantMutate: vi.fn(() => vi.fn()),
}));

vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: vi.fn(() => ({ getList: vi.fn() })),
}));

vi.mock("~/lib/staff-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/staff-api")>()),
  staffService: {
    inviteExternal: mocks.inviteExternal,
    createExternal: mocks.createExternal,
  },
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: mocks.toastSuccess }),
}));

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

vi.mock("~/components/ui/navigation-link", () => ({
  default: ({ href, children }: { href: string; children: ReactNode }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock("~/components/database/database-list-layout", () => ({
  DatabaseListLayout: ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  ),
}));

vi.mock("~/components/database/database-page-layout", () => ({
  DatabasePageLayout: ({
    children,
    intro,
    overlays,
  }: {
    children: ReactNode;
    intro?: { title: string; actions?: ReactNode };
    overlays?: ReactNode;
  }) => (
    <div>
      <h1>{intro?.title}</h1>
      {intro?.actions}
      {children}
      {overlays}
    </div>
  ),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: () => null,
}));

vi.mock("~/components/admin/invitation-form", () => ({
  InvitationForm: () => null,
}));

vi.mock("~/components/admin/pending-invitations-list", () => ({
  PendingInvitationsList: () => null,
}));

function signInWith(permissions: string[]) {
  vi.mocked(useSession).mockReturnValue({
    data: {
      user: { id: "1", token: "test-token", permissions },
      expires: "2099-01-01",
    },
    status: "authenticated",
    update: vi.fn(),
  });
}

// „+ Personal“ klappt für die Leitung auf (#4573): einladen oder extern.
// jsdom rendert Desktop-Knopf und schwebenden Knopf; [0] ist der Desktop-Knopf.
function chooseCreate(entry: "Personal einladen" | "Externe Person anlegen") {
  fireEvent.click(
    screen.getAllByRole("button", { name: "Personal hinzufügen" })[0]!,
  );
  fireEvent.click(screen.getByRole("menuitem", { name: entry }));
}

describe("Personalverwaltung: externe Personen", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useIsMobile).mockReturnValue(false);
  });

  it("marks an external person without account and invites them from the row menu", async () => {
    signInWith(["users:read", "users:create"]);
    mocks.inviteExternal.mockResolvedValue({
      id: "7",
      name: "Elli Alt",
      firstName: "Elli",
      lastName: "Alt",
      isExternal: true,
      externalAccountStatus: "invitation_pending",
    });
    render(<TeachersPage />);

    const row = screen.getByRole("link", { name: /Elli Alt/ });
    expect(row).toHaveTextContent("Extern · Sportverein");
    expect(row).toHaveTextContent("Kein Konto");

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Elli Alt" }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Konto einladen" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Konto einladen",
    });
    expect(dialog).toHaveTextContent("Elli Alt bekommt eine Einladung");
    // Ohne users:manage gibt es nur „Eigene Termine betreuen“ (ADR 0091).
    expect(dialog).toHaveTextContent(
      "Eigene Termine betreuenStartet und beendet eigene Termine.",
    );
    expect(screen.queryByRole("radio")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "elli@example.test" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Einladen" }));

    await waitFor(() =>
      expect(mocks.inviteExternal).toHaveBeenCalledWith(
        "7",
        "elli@example.test",
        "supervise",
      ),
    );
    await waitFor(() =>
      expect(mocks.toastSuccess).toHaveBeenCalledWith(
        "Die Einladung an Elli Alt ist unterwegs.",
      ),
    );
    expect(mocks.refresh).toHaveBeenCalled();
  });

  it("asks for an address before inviting", async () => {
    signInWith(["users:read", "users:create"]);
    render(<TeachersPage />);

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Elli Alt" }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Konto einladen" }));
    await screen.findByRole("dialog", { name: "Konto einladen" });
    fireEvent.click(screen.getByRole("button", { name: "Einladen" }));

    expect(screen.getByLabelText("E-Mail-Adresse")).toHaveAccessibleDescription(
      "Bitte E-Mail-Adresse eingeben.",
    );
    expect(mocks.inviteExternal).not.toHaveBeenCalled();
  });

  it("creates a new external person from the header action", async () => {
    signInWith(["users:read", "users:create", "users:manage"]);
    mocks.createExternal.mockResolvedValue({
      id: "9",
      name: "Lea Gast",
      firstName: "Lea",
      lastName: "Gast",
      isExternal: true,
      externalAccountStatus: "invitation_pending",
    });
    render(<TeachersPage />);

    chooseCreate("Externe Person anlegen");
    await screen.findByRole("dialog", { name: "Externe Person anlegen" });
    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Lea" },
    });
    fireEvent.change(screen.getByLabelText("Nachname"), {
      target: { value: "Gast" },
    });
    fireEvent.click(
      screen.getByRole("radio", { name: /Eigene Termine betreuen/ }),
    );
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "lea@example.test" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    await waitFor(() =>
      expect(mocks.toastSuccess).toHaveBeenCalledWith(
        "Lea Gast ist angelegt. Die Einladung ist per E-Mail unterwegs.",
      ),
    );
    expect(mocks.createExternal).toHaveBeenCalledWith({
      firstName: "Lea",
      lastName: "Gast",
      email: "lea@example.test",
      organization: "",
      access: "supervise",
    });
  });

  it("sends a new invitation to a pending person and says the old one ends", async () => {
    signInWith(["users:read", "users:create"]);
    mocks.inviteExternal.mockResolvedValue({
      id: "8",
      name: "Jonas Neu",
      firstName: "Jonas",
      lastName: "Neu",
      isExternal: true,
      externalAccountStatus: "invitation_pending",
    });
    render(<TeachersPage />);

    expect(screen.getByRole("link", { name: /Jonas Neu/ })).toHaveTextContent(
      "Einladung offen",
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Jonas Neu" }),
    );
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Einladung erneut senden" }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Einladung erneut senden",
    });
    expect(dialog).toHaveTextContent(
      "Jonas Neu bekommt eine neue Einladung per E-Mail.",
    );
    expect(dialog).toHaveTextContent(
      "Die alte Einladung gilt dann nicht mehr.",
    );
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "jonas@example.test" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Erneut senden" }));

    await waitFor(() =>
      expect(mocks.toastSuccess).toHaveBeenCalledWith(
        "Die neue Einladung an Jonas Neu ist unterwegs.",
      ),
    );
    expect(mocks.inviteExternal).toHaveBeenCalledWith(
      "8",
      "jonas@example.test",
      "supervise",
    );
  });

  // #4573 decision: the header action always records a new person. A person
  // without account who shares the name may be someone else; inviting them
  // goes only through „Konto einladen“ in their row.
  it("records a namesake of an external without account as a new person", async () => {
    signInWith(["users:read", "users:create", "users:manage"]);
    mocks.createExternal.mockResolvedValue({
      id: "9",
      name: "Elli Alt",
      firstName: "Elli",
      lastName: "Alt",
      isExternal: true,
      externalAccountStatus: "invitation_pending",
    });
    render(<TeachersPage />);

    chooseCreate("Externe Person anlegen");
    await screen.findByRole("dialog", { name: "Externe Person anlegen" });
    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Elli" },
    });
    fireEvent.change(screen.getByLabelText("Nachname"), {
      target: { value: "Alt" },
    });
    fireEvent.click(
      screen.getByRole("radio", { name: /Eigene Termine betreuen/ }),
    );
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "elli@example.test" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    await waitFor(() =>
      expect(mocks.createExternal).toHaveBeenCalledWith({
        firstName: "Elli",
        lastName: "Alt",
        email: "elli@example.test",
        organization: "",
        access: "supervise",
      }),
    );
    expect(mocks.inviteExternal).not.toHaveBeenCalled();
  });

  it("offers both ways to create from the floating button on a phone", async () => {
    vi.mocked(useIsMobile).mockReturnValue(true);
    signInWith(["users:read", "users:create", "users:manage"]);
    render(<TeachersPage />);

    // [1] ist der schwebende Knopf unten rechts.
    fireEvent.click(
      screen.getAllByRole("button", { name: "Personal hinzufügen" })[1]!,
    );
    expect(
      screen.getByRole("menuitem", { name: "Personal einladen" }),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Externe Person anlegen" }),
    );

    expect(
      await screen.findByRole("dialog", { name: "Externe Person anlegen" }),
    ).toBeInTheDocument();
  });

  // ADR 0091: „Kein Zugang“ records a placeholder without address and
  // without invitation; the toast says only that the person is recorded.
  it("creates an external person without access and without invitation", async () => {
    signInWith(["users:read", "users:create", "users:manage"]);
    mocks.createExternal.mockResolvedValue({
      id: "10",
      name: "Tom Platz",
      firstName: "Tom",
      lastName: "Platz",
      isExternal: true,
      externalAccountStatus: "none",
    });
    render(<TeachersPage />);

    chooseCreate("Externe Person anlegen");
    await screen.findByRole("dialog", { name: "Externe Person anlegen" });
    expect(screen.getByRole("radio", { name: /Kein Zugang/ })).toBeChecked();
    expect(screen.queryByLabelText("E-Mail-Adresse")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Tom" },
    });
    fireEvent.change(screen.getByLabelText("Nachname"), {
      target: { value: "Platz" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    await waitFor(() =>
      expect(mocks.toastSuccess).toHaveBeenCalledWith(
        "Tom Platz ist angelegt.",
      ),
    );
    expect(mocks.createExternal).toHaveBeenCalledWith({
      firstName: "Tom",
      lastName: "Platz",
      organization: "",
      access: "none",
    });
  });

  // „Nur ansehen“ gives the role „Gast“ and needs users:manage.
  it("offers „Nur ansehen“ in the invitation with users:manage", async () => {
    signInWith(["users:read", "users:create", "users:manage"]);
    mocks.inviteExternal.mockResolvedValue({
      id: "7",
      name: "Elli Alt",
      firstName: "Elli",
      lastName: "Alt",
      isExternal: true,
      externalAccountStatus: "invitation_pending",
    });
    render(<TeachersPage />);

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Elli Alt" }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Konto einladen" }));
    await screen.findByRole("dialog", { name: "Konto einladen" });
    expect(
      screen.getByRole("radio", { name: /Eigene Termine betreuen/ }),
    ).toBeChecked();
    fireEvent.click(screen.getByRole("radio", { name: /Nur ansehen/ }));
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "elli@example.test" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Einladen" }));

    await waitFor(() =>
      expect(mocks.inviteExternal).toHaveBeenCalledWith(
        "7",
        "elli@example.test",
        "view",
      ),
    );
  });

  it("hides „Nur ansehen“ when creating without users:manage", async () => {
    signInWith(["users:read", "users:create"]);
    render(<TeachersPage />);

    fireEvent.click(
      screen.getAllByRole("button", { name: "Externe Person anlegen" })[0]!,
    );
    await screen.findByRole("dialog", { name: "Externe Person anlegen" });

    // Anker: die Zugangswahl steht da.
    expect(
      screen.getByRole("radio", { name: /Eigene Termine betreuen/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("radio", { name: /Nur ansehen/ }),
    ).not.toBeInTheDocument();
  });

  it("opens „Externe Person anlegen“ directly without the right to invite staff", async () => {
    signInWith(["users:read", "users:create"]);
    render(<TeachersPage />);

    fireEvent.click(
      screen.getAllByRole("button", { name: "Externe Person anlegen" })[0]!,
    );

    expect(
      await screen.findByRole("dialog", { name: "Externe Person anlegen" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Personal einladen" }),
    ).not.toBeInTheDocument();
  });

  it("offers neither action without permission to create users", () => {
    signInWith(["users:read"]);
    render(<TeachersPage />);

    // Anker: die externe Zeile ist geladen und trägt ihren Kontostand.
    expect(screen.getByRole("link", { name: /Elli Alt/ })).toHaveTextContent(
      "Kein Konto",
    );
    expect(
      screen.queryByRole("button", { name: "Aktionen für Elli Alt" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Externe Person anlegen" }),
    ).not.toBeInTheDocument();
  });
});
