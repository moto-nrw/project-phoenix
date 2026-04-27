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
  mockListOrganizations: vi.fn(),
  mockCreateOrganization: vi.fn(),
  mockUpdateOrganization: vi.fn(),
  mockSoftDeleteOrganization: vi.fn(),
  mockRestoreOrganization: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
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
}));

vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({ title, tabs, actionButton }: any) => (
    <div data-testid="page-header">
      <h1>{title}</h1>
      {tabs && (
        <div data-testid="tabs">
          {tabs.items.map((tab: any) => (
            <button
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
        <button data-testid="confirm-btn" onClick={onConfirm}>
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
import { mockOrg, setupSWR } from "../provisioning/provisioning-test-helpers";

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

  // --- Edit ---

  it("opens edit organization modal with pre-filled data", async () => {
    withDefaultSWR();

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Träger bearbeiten")).toBeInTheDocument();
      expect(screen.getByDisplayValue("Test Org")).toBeInTheDocument();
      expect(screen.getByDisplayValue("test-org")).toBeInTheDocument();
    });
  });

  it("updates organization and mutates", async () => {
    withDefaultSWR();
    mockUpdateOrganization.mockResolvedValue({
      ...mockOrg,
      name: "Updated Org",
    });

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByDisplayValue("Test Org"), {
      target: { value: "Updated Org" },
    });

    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(mockUpdateOrganization).toHaveBeenCalledWith("1", {
        name: "Updated Org",
        slug: "test-org",
        active: true,
      });
      expect(mockMutateOrgs).toHaveBeenCalled();
    });
  });

  it("shows slug warning in edit organization modal", async () => {
    withDefaultSWR();

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    expect(
      screen.getByText(/Slug-Änderungen können bestehende Verweise/),
    ).toBeInTheDocument();
  });

  it("shows conflict error when updating organization with duplicate slug", async () => {
    withDefaultSWR();
    const { OperatorApiError } = await import("~/lib/operator/api-helpers");
    mockUpdateOrganization.mockRejectedValue(
      new OperatorApiError("conflict", 409),
    );

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByDisplayValue("test-org"), {
      target: { value: "other-slug" },
    });

    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(
        screen.getByText("Ein Träger mit diesem Slug existiert bereits."),
      ).toBeInTheDocument();
    });
  });

  it("handles update organization error gracefully", async () => {
    withDefaultSWR();
    mockUpdateOrganization.mockRejectedValue(new Error("Server error"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop
    });

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(screen.getByText("Server error")).toBeInTheDocument();
      expect(consoleError).toHaveBeenCalledWith(
        "organization_update_failed",
        expect.objectContaining({ error: "Server error" }),
      );
    });

    consoleError.mockRestore();
  });

  it("closes edit organization modal on cancel", async () => {
    withDefaultSWR();

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Abbrechen"));

    await waitFor(() => {
      expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
    });
  });

  // --- Toggle active ---

  it("toggles organization active status", async () => {
    withDefaultSWR();
    mockUpdateOrganization.mockResolvedValue({ ...mockOrg, active: false });

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getByLabelText("Deaktivieren"));

    await waitFor(() => {
      expect(mockUpdateOrganization).toHaveBeenCalledWith("1", {
        name: "Test Org",
        slug: "test-org",
        active: false,
      });
      expect(mockMutateOrgs).toHaveBeenCalled();
    });
  });

  it("handles toggle active error gracefully", async () => {
    withDefaultSWR();
    mockUpdateOrganization.mockRejectedValue(new Error("Toggle failed"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop
    });

    render(<OperatorOrganizationsPage />);

    fireEvent.click(screen.getByLabelText("Deaktivieren"));

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Ändern des Status/),
      ).toBeInTheDocument();
      expect(consoleError).toHaveBeenCalledWith(
        "organization_toggle_active_failed",
        expect.objectContaining({ error: "Toggle failed" }),
      );
    });

    consoleError.mockRestore();
  });
});
