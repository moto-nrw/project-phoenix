import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import RelatedAccountsPanel from "./related-accounts-panel";
import { ParentApiError, type RelatedAccount } from "~/lib/parent-api";

const mockList = vi.fn();
const mockInvite = vi.fn();
const mockRemove = vi.fn();

vi.mock("~/lib/parent-api", async (importOriginal) => {
  // Keep the real ParentApiError class so instanceof checks in the panel work.
  const actual = await importOriginal<typeof import("~/lib/parent-api")>();
  return {
    ParentApiError: actual.ParentApiError,
    listRelatedAccounts: (studentId: string): unknown => mockList(studentId),
    // Forward the options argument only when present, so the older
    // two-argument assertions keep matching plain invites.
    inviteRelatedAccount: (
      studentId: string,
      email: string,
      options?: unknown,
    ): unknown =>
      options === undefined
        ? mockInvite(studentId, email)
        : mockInvite(studentId, email, options),
    removeRelatedAccount: (
      studentId: string,
      guardianProfileId: string,
    ): unknown => mockRemove(studentId, guardianProfileId),
  };
});

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    onConfirm,
    onClose,
    title,
    children,
  }: {
    isOpen: boolean;
    onConfirm: () => void;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="upgrade-modal">
        <h3>{title}</h3>
        {children}
        <button type="button" onClick={onConfirm} data-testid="confirm-upgrade">
          Zugriff gewähren
        </button>
        <button type="button" onClick={onClose} data-testid="cancel-upgrade">
          Abbrechen
        </button>
      </div>
    ) : null,
}));

const primaryActive: RelatedAccount = {
  guardian_profile_id: "1",
  first_name: "Sabine",
  last_name: "Schneider",
  email: "sabine.schneider@email.de",
  relationship_type: "parent",
  is_primary: true,
  status: "active",
  is_self: false,
};

const secondaryPending: RelatedAccount = {
  guardian_profile_id: "2",
  first_name: "Markus",
  last_name: "Wolf",
  email: "markus.wolf@email.de",
  relationship_type: "guardian",
  is_primary: false,
  status: "pending",
  is_self: false,
};

const staffContactNoAccount: RelatedAccount = {
  guardian_profile_id: "3",
  first_name: "Clara",
  last_name: "Kontakt",
  email: "clara.kontakt@email.de",
  relationship_type: "guardian",
  is_primary: false,
  status: "no_account",
  is_self: false,
};

describe("RelatedAccountsPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockList.mockResolvedValue([primaryActive, secondaryPending]);
    mockInvite.mockResolvedValue({
      outcome: "invited",
      guardian_profile_id: "3",
    });
    mockRemove.mockResolvedValue(undefined);
  });

  it("lists linked accounts with their status", async () => {
    render(
      <RelatedAccountsPanel
        studentId="1"
        canInvite={false}
        canRemove={false}
      />,
    );
    await waitFor(() =>
      expect(screen.getByText("Sabine Schneider")).toBeInTheDocument(),
    );
    expect(screen.getByText("Markus Wolf")).toBeInTheDocument();
    expect(screen.getByText(/Konto aktiv/)).toBeInTheDocument();
    expect(screen.getByText(/Einladung offen/)).toBeInTheDocument();
  });

  it("renders no-account contacts without a remove affordance", async () => {
    mockList.mockResolvedValue([primaryActive, staffContactNoAccount]);
    render(
      <RelatedAccountsPanel studentId="1" canInvite={false} canRemove={true} />,
    );

    await waitFor(() =>
      expect(screen.getByText("Clara Kontakt")).toBeInTheDocument(),
    );
    expect(screen.getByText(/Kontakt ohne Konto/)).toBeInTheDocument();
    expect(screen.queryByTitle("Zugang entfernen")).not.toBeInTheDocument();
  });

  it("does not offer a remove affordance for the parent's own row", async () => {
    // A non-primary guardian who is the logged-in parent: self-removal is
    // rejected by the backend, so the panel must not offer the action.
    const self: RelatedAccount = {
      ...secondaryPending,
      guardian_profile_id: "9",
      first_name: "Ich",
      last_name: "Selbst",
      is_self: true,
    };
    mockList.mockResolvedValue([primaryActive, self]);
    render(
      <RelatedAccountsPanel studentId="1" canInvite={false} canRemove={true} />,
    );

    await waitFor(() =>
      expect(screen.getByText("Ich Selbst")).toBeInTheDocument(),
    );
    expect(screen.queryByTitle("Zugang entfernen")).not.toBeInTheDocument();
  });

  it("hides the invite control when inviting is disabled", async () => {
    render(
      <RelatedAccountsPanel
        studentId="1"
        canInvite={false}
        canRemove={false}
      />,
    );
    await waitFor(() => screen.getByText("Sabine Schneider"));
    expect(
      screen.queryByRole("button", { name: /Einladen/ }),
    ).not.toBeInTheDocument();
  });

  it("invites a person by email when inviting is enabled", async () => {
    render(
      <RelatedAccountsPanel studentId="1" canInvite={true} canRemove={false} />,
    );
    await waitFor(() => screen.getByText("Sabine Schneider"));

    fireEvent.click(screen.getByRole("button", { name: /Einladen/ }));
    const input = screen.getByPlaceholderText("E-Mail-Adresse");
    fireEvent.change(input, { target: { value: "oma@example.test" } });
    fireEvent.click(screen.getByRole("button", { name: /Senden/ }));

    await waitFor(() =>
      expect(mockInvite).toHaveBeenCalledWith("1", "oma@example.test"),
    );
  });

  it("explains that school-managed social-worker contacts cannot be invited", async () => {
    mockInvite.mockRejectedValue(
      new ParentApiError(
        "a social-worker contact is managed by the school",
        403,
        "guardian_social_worker_managed",
      ),
    );
    render(
      <RelatedAccountsPanel studentId="1" canInvite={true} canRemove={false} />,
    );
    await waitFor(() => screen.getByText("Sabine Schneider"));

    fireEvent.click(screen.getByRole("button", { name: /Einladen/ }));
    fireEvent.change(screen.getByPlaceholderText("E-Mail-Adresse"), {
      target: { value: "sozialdienst@example.test" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Senden/ }));

    await waitFor(() =>
      expect(
        screen.getByText(
          "Dieser Kontakt wird von der Schule verwaltet und kann nicht über die Eltern-App eingeladen werden.",
        ),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByTestId("upgrade-modal")).not.toBeInTheDocument();
  });

  it("removes a non-primary account but never the primary", async () => {
    render(
      <RelatedAccountsPanel studentId="1" canInvite={false} canRemove={true} />,
    );
    await waitFor(() => screen.getByText("Markus Wolf"));

    // Exactly one remove affordance — only the non-primary account has it.
    expect(screen.getAllByTitle("Zugang entfernen")).toHaveLength(1);

    fireEvent.click(screen.getByTitle("Zugang entfernen"));
    // two-step confirm
    fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));

    await waitFor(() => expect(mockRemove).toHaveBeenCalledWith("1", "2"));
  });

  it("labels accounts without portal access honestly", async () => {
    const noAccess: RelatedAccount = {
      guardian_profile_id: "5",
      first_name: "Nils",
      last_name: "Notfall",
      email: "nils.notfall@email.de",
      relationship_type: "other",
      is_primary: false,
      status: "active_no_access",
      is_self: false,
    };
    mockList.mockResolvedValue([primaryActive, noAccess]);
    render(
      <RelatedAccountsPanel
        studentId="1"
        canInvite={false}
        canRemove={false}
      />,
    );

    await waitFor(() =>
      expect(screen.getByText("Nils Notfall")).toBeInTheDocument(),
    );
    expect(
      screen.getByText(/Konto aktiv, kein Portalzugriff/),
    ).toBeInTheDocument();
    // Without invite rights there is no upgrade affordance.
    expect(
      screen.queryByRole("button", { name: "Zugriff gewähren" }),
    ).not.toBeInTheDocument();
  });

  it("grants access to a no-access account via the row action", async () => {
    const noAccess: RelatedAccount = {
      guardian_profile_id: "5",
      first_name: "Nils",
      last_name: "Notfall",
      email: "nils.notfall@email.de",
      relationship_type: "other",
      is_primary: false,
      status: "active_no_access",
      is_self: false,
    };
    mockList.mockResolvedValue([primaryActive, noAccess]);
    mockInvite.mockResolvedValue({
      outcome: "already_linked",
      guardian_profile_id: "5",
    });
    render(
      <RelatedAccountsPanel studentId="1" canInvite={true} canRemove={false} />,
    );

    await waitFor(() => screen.getByText("Nils Notfall"));
    fireEvent.click(screen.getByRole("button", { name: "Zugriff gewähren" }));
    expect(screen.getByTestId("upgrade-modal")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("confirm-upgrade"));
    await waitFor(() =>
      expect(mockInvite).toHaveBeenCalledWith("1", "nils.notfall@email.de", {
        confirmRoleUpgrade: true,
      }),
    );
    await waitFor(() =>
      expect(screen.getByText("Zugriff wurde gewährt.")).toBeInTheDocument(),
    );
  });

  it("asks for confirmation when the invite hits a restricted contact", async () => {
    mockInvite.mockResolvedValueOnce({
      outcome: "existing_contact_restricted",
      guardian_profile_id: "7",
      existing_role: "emergency_contact",
    });
    mockInvite.mockResolvedValueOnce({
      outcome: "invited",
      guardian_profile_id: "7",
    });
    render(
      <RelatedAccountsPanel studentId="1" canInvite={true} canRemove={false} />,
    );
    await waitFor(() => screen.getByText("Sabine Schneider"));

    fireEvent.click(screen.getByRole("button", { name: /Einladen/ }));
    fireEvent.change(screen.getByPlaceholderText("E-Mail-Adresse"), {
      target: { value: "opa@example.test" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Senden/ }));

    // First call is the plain invite; it comes back restricted → modal names
    // the existing role.
    await waitFor(() =>
      expect(screen.getByTestId("upgrade-modal")).toBeInTheDocument(),
    );
    expect(mockInvite).toHaveBeenNthCalledWith(1, "1", "opa@example.test");
    expect(screen.getByText(/Notfallkontakt/)).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("confirm-upgrade"));
    await waitFor(() =>
      expect(mockInvite).toHaveBeenNthCalledWith(2, "1", "opa@example.test", {
        confirmRoleUpgrade: true,
      }),
    );
  });

  it("cancelling the upgrade confirmation performs no second call", async () => {
    mockInvite.mockResolvedValueOnce({
      outcome: "existing_contact_restricted",
      guardian_profile_id: "7",
      existing_role: "pickup_only",
    });
    render(
      <RelatedAccountsPanel studentId="1" canInvite={true} canRemove={false} />,
    );
    await waitFor(() => screen.getByText("Sabine Schneider"));

    fireEvent.click(screen.getByRole("button", { name: /Einladen/ }));
    fireEvent.change(screen.getByPlaceholderText("E-Mail-Adresse"), {
      target: { value: "opa@example.test" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Senden/ }));
    await waitFor(() =>
      expect(screen.getByTestId("upgrade-modal")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByTestId("cancel-upgrade"));
    expect(screen.queryByTestId("upgrade-modal")).not.toBeInTheDocument();
    expect(mockInvite).toHaveBeenCalledTimes(1);
  });
});
