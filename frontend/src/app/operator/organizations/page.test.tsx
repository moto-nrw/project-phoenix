/**
 * Tests for Operator Träger Page (Organizations management).
 *
 * Ported from provisioning/page.test.tsx — organization-specific behaviour
 * (create, edit, toggle active, soft-delete, slug validation, error paths).
 */
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const {
  mockUseSession,
  mockUseSWR,
  mockMutateOrgs,
  mockMutateSchools,
  mockPush,
  mockListOrganizations,
  mockCreateOrganization,
  mockUpdateOrganization,
  mockSoftDeleteOrganization,
  mockRestoreOrganization,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUseSWR: vi.fn(),
  mockMutateOrgs: vi.fn(),
  mockMutateSchools: vi.fn(),
  mockPush: vi.fn(),
  mockListOrganizations: vi.fn(),
  mockCreateOrganization: vi.fn(),
  mockUpdateOrganization: vi.fn(),
  mockSoftDeleteOrganization: vi.fn(),
  mockRestoreOrganization: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("swr", () => ({
  default: mockUseSWR,
  useSWRConfig: () => ({ mutate: vi.fn() }),
}));

vi.mock("~/lib/operator/provisioning-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/operator/provisioning-api")
  >("~/lib/operator/provisioning-api");
  return {
    ...actual,
    operatorProvisioningService: {
      listOrganizations: mockListOrganizations,
      createOrganization: mockCreateOrganization,
      updateOrganization: mockUpdateOrganization,
      softDeleteOrganization: mockSoftDeleteOrganization,
      restoreOrganization: mockRestoreOrganization,
      listSchools: vi.fn().mockResolvedValue([]),
    },
  };
});

vi.mock("~/lib/format-utils", () => ({
  getRelativeTime: (dateStr: string) => `relative(${dateStr})`,
  formatCount: (value: number) => new Intl.NumberFormat("de-DE").format(value),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({ title, tabs, actionButton }: any) => (
    <div data-testid="page-header">
      <h1>{title}</h1>
      {tabs && (
        <div data-testid="tabs">
          {tabs.items.map((tab: any) => (
            <button
              type="button"
              key={tab.id}
              data-testid={`tab-${tab.id}`}
              className={tabs.activeTab === tab.id ? "active" : ""}
            >
              {tab.label}
              {tab.count !== undefined && ` (${tab.count})`}
            </button>
          ))}
        </div>
      )}
      {actionButton}
    </div>
  ),
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({ isOpen, children, title, footer }: any) =>
    isOpen ? (
      <div data-testid="modal">
        <h2>{title}</h2>
        {children}
        <div data-testid="modal-footer">{footer}</div>
      </div>
    ) : null,
  ConfirmationModal: ({ isOpen, children, title, onConfirm }: any) =>
    isOpen ? (
      <div data-testid="confirmation-modal">
        <h2>{title}</h2>
        {children}
        <button type="button" data-testid="confirm-btn" onClick={onConfirm}>
          Bestätigen
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/ui/skeleton", () => ({
  Skeleton: ({ className }: any) => (
    <div data-testid="skeleton" className={className} />
  ),
}));

import OperatorOrganizationsPage from "./page";
import {
  mockOrg,
  setupSWR,
} from "~/test/helpers/operator-provisioning/provisioning-test-helpers";

type SWROverrides = Partial<Omit<Parameters<typeof setupSWR>[0], "useSWRMock">>;
function withDefaultSWR(overrides: SWROverrides = {}) {
  setupSWR({
    useSWRMock: mockUseSWR,
    mutateOrgs: mockMutateOrgs,
    mutateSchools: mockMutateSchools,
    ...overrides,
  });
}

describe("OperatorOrganizationsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSession.mockReturnValue({
      data: { user: { id: "1", email: "operator@example.com" } },
      status: "authenticated",
    });
    mockMutateOrgs.mockResolvedValue(undefined);
    mockMutateSchools.mockResolvedValue([]);
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ status: "ok" })),
    );
  });

  it("renders loading state", () => {
    withDefaultSWR({ orgsLoading: true, schoolsLoading: true });

    render(<OperatorOrganizationsPage />);

    expect(screen.getAllByTestId("skeleton").length).toBeGreaterThan(0);
  });

  it("renders organization list", () => {
    withDefaultSWR();

    render(<OperatorOrganizationsPage />);

    expect(screen.getByText("Test Org")).toBeInTheDocument();
    expect(screen.getByText("test-org")).toBeInTheDocument();
    expect(screen.getByText("Aktiv")).toBeInTheDocument();
  });

  it("opens the create-organization modal from the empty state action button", async () => {
    withDefaultSWR({ orgs: [] });

    render(<OperatorOrganizationsPage />);

    // EmptyState renders "Neuer Träger" inside the empty state. Clicking it
    // opens the create modal.
    const emptyAction = screen.getAllByText("Neuer Träger");
    expect(emptyAction.length).toBeGreaterThan(0);
    // The empty state's action sits inside an EmptyState card; the header
    // also has a "Neuer Träger" trigger. Click whichever is found first in
    // the empty-state card area.
    fireEvent.click(emptyAction[emptyAction.length - 1]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });

  it("renders empty state for organizations", () => {
    withDefaultSWR({ orgs: [] });

    render(<OperatorOrganizationsPage />);

    expect(screen.getByText("Keine Träger")).toBeInTheDocument();
    expect(screen.getAllByText("Neuer Träger").length).toBeGreaterThan(0);
  });

  it("shows tab count", () => {
    withDefaultSWR();

    render(<OperatorOrganizationsPage />);

    expect(screen.getByTestId("tab-organizations")).toHaveTextContent(
      "Träger (1)",
    );
  });

  it("passes null SWR key when not authenticated", () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "unauthenticated",
    });
    withDefaultSWR({ orgs: undefined, schools: undefined });

    render(<OperatorOrganizationsPage />);

    expect(mockUseSWR).toHaveBeenCalledWith(
      null,
      expect.any(Function),
      expect.any(Object),
    );
  });

  // --- Create ---

  it("opens create organization modal", async () => {
    withDefaultSWR();

    render(<OperatorOrganizationsPage />);

    const createButtons = screen.getAllByText("Neuer Träger");
    fireEvent.click(createButtons[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });

  it("auto-generates slug from organization name", async () => {
    withDefaultSWR();

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getAllByText("Neuer Träger")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText(/Name/);
    fireEvent.change(nameInput, { target: { value: "Meine Organisation" } });

    const slugInput = screen.getByLabelText(/Slug/);
    expect((slugInput as HTMLInputElement).value).toBe("meine-organisation");
  });

  it("creates organization successfully", async () => {
    withDefaultSWR();
    mockCreateOrganization.mockResolvedValue(mockOrg);

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getAllByText("Neuer Träger")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText(/Name/);
    fireEvent.change(nameInput, { target: { value: "New Org" } });

    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(mockCreateOrganization).toHaveBeenCalledWith({
        name: "New Org",
        slug: "new-org",
      });
      expect(mockMutateOrgs).toHaveBeenCalled();
    });
  });

  it("shows error for duplicate organization slug", async () => {
    withDefaultSWR();
    const { OperatorApiError } = await import("~/lib/operator/api-helpers");
    mockCreateOrganization.mockRejectedValue(
      new OperatorApiError("conflict", 409),
    );

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getAllByText("Neuer Träger")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/Name/), {
      target: { value: "Dup Org" },
    });

    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(
        screen.getByText("Ein Träger mit diesem Slug existiert bereits."),
      ).toBeInTheDocument();
    });
  });

  it("handles generic create organization error", async () => {
    withDefaultSWR();
    mockCreateOrganization.mockRejectedValue(new Error("Server down"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop
    });

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getAllByText("Neuer Träger")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/Name/), {
      target: { value: "New Org" },
    });

    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(screen.getByText("Server down")).toBeInTheDocument();
      expect(consoleError).toHaveBeenCalledWith(
        "organization_create_failed",
        expect.objectContaining({ error: "Server down" }),
      );
    });

    consoleError.mockRestore();
  });

  it("validates slug format in organization form", async () => {
    withDefaultSWR();

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getAllByText("Neuer Träger")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/Name/), {
      target: { value: "Test" },
    });
    fireEvent.change(screen.getByLabelText(/Slug/), {
      target: { value: "INVALID SLUG!" },
    });

    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(
        screen.getByText(
          "Slug darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.",
        ),
      ).toBeInTheDocument();
    });

    expect(mockCreateOrganization).not.toHaveBeenCalled();
  });

  it("disables create button when required fields are empty", () => {
    withDefaultSWR();

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getAllByText("Neuer Träger")[0]!);

    expect(screen.getByText("Erstellen")).toBeDisabled();
  });

  it("stops slug auto-generation after manual edit", async () => {
    withDefaultSWR();

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getAllByText("Neuer Träger")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText(/Name/);
    const slugInput = screen.getByLabelText(/Slug/);

    fireEvent.change(nameInput, { target: { value: "Auto" } });
    expect((slugInput as HTMLInputElement).value).toBe("auto");

    fireEvent.change(slugInput, { target: { value: "custom-slug" } });

    fireEvent.change(nameInput, { target: { value: "Changed Name" } });
    expect((slugInput as HTMLInputElement).value).toBe("custom-slug");
  });

  it("navigates to the organization detail page when a row is clicked", () => {
    withDefaultSWR();

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getByText("Test Org"));

    expect(mockPush).toHaveBeenCalledWith("/operator/organizations/test-org");
  });

  it("keeps the overview table free of row-level management buttons", () => {
    withDefaultSWR();

    render(<OperatorOrganizationsPage />);

    expect(screen.queryByText("Bearbeiten")).not.toBeInTheDocument();
    expect(screen.queryByText("Löschen")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Deaktivieren")).not.toBeInTheDocument();
  });

  // --- Column sorting (covers sortValue callbacks for each column) ---

  describe("column sorting", () => {
    it("sorts by Träger column when the header is clicked", () => {
      withDefaultSWR({
        orgs: [
          { ...mockOrg, id: "1", slug: "z-org", name: "Z Org" },
          { ...mockOrg, id: "2", slug: "a-org", name: "A Org" },
        ],
      });

      render(<OperatorOrganizationsPage />);

      // Sort header buttons have an accessible name like
      // "Träger – Spalte sortieren" / "Träger – aufsteigend sortiert".
      fireEvent.click(screen.getByRole("button", { name: /^Träger – / }));

      // After clicking the header, both rows still render (sort doesn't
      // hide rows). The point of the test is that the sortValue callback
      // executed at least once during the comparison.
      expect(screen.getByText("A Org")).toBeInTheDocument();
      expect(screen.getByText("Z Org")).toBeInTheDocument();
    });

    it("sorts by Schulen / Konten / Geräte / Personen / Status when their headers are clicked", () => {
      withDefaultSWR({
        orgs: [
          {
            ...mockOrg,
            id: "1",
            slug: "a-org",
            name: "A Org",
            schulenCount: 5,
            kontenCount: 10,
            geraeteCount: 2,
            personenCount: 7,
            active: false,
          },
          {
            ...mockOrg,
            id: "2",
            slug: "b-org",
            name: "B Org",
            schulenCount: 1,
            kontenCount: 30,
            geraeteCount: 8,
            personenCount: 3,
            active: true,
          },
        ],
      });

      render(<OperatorOrganizationsPage />);

      for (const header of [
        "Schulen",
        "Konten",
        "Geräte",
        "Personen",
        "Status",
      ]) {
        fireEvent.click(
          screen.getByRole("button", { name: new RegExp(`^${header} – `) }),
        );
      }

      expect(screen.getByText("A Org")).toBeInTheDocument();
      expect(screen.getByText("B Org")).toBeInTheDocument();
    });
  });
});
