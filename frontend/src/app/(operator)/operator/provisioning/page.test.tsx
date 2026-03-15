/**
 * Tests for Operator Provisioning Page
 * Tests rendering, tab switching, CRUD modals, and error handling
 */
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Hoisted mocks
const {
  mockUseOperatorAuth,
  mockUseSWR,
  mockMutateOrgs,
  mockMutateSchools,
  mockListOrganizations,
  mockCreateOrganization,
  mockListSchools,
  mockCreateSchool,
  mockUpdateOrganization,
  mockUpdateSchool,
  mockInviteSchoolAdmin,
} = vi.hoisted(() => ({
  mockUseOperatorAuth: vi.fn(),
  mockUseSWR: vi.fn(),
  mockMutateOrgs: vi.fn(),
  mockMutateSchools: vi.fn(),
  mockListOrganizations: vi.fn(),
  mockCreateOrganization: vi.fn(),
  mockListSchools: vi.fn(),
  mockCreateSchool: vi.fn(),
  mockUpdateOrganization: vi.fn(),
  mockUpdateSchool: vi.fn(),
  mockInviteSchoolAdmin: vi.fn(),
}));

vi.mock("~/lib/operator/auth-context", () => ({
  useOperatorAuth: mockUseOperatorAuth,
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("swr", () => ({
  default: mockUseSWR,
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    listOrganizations: mockListOrganizations,
    createOrganization: mockCreateOrganization,
    listSchools: mockListSchools,
    createSchool: mockCreateSchool,
    updateOrganization: mockUpdateOrganization,
    updateSchool: mockUpdateSchool,
    inviteSchoolAdmin: mockInviteSchoolAdmin,
  },
}));

/* eslint-disable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-return */
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
              onClick={() => tabs.onTabChange(tab.id)}
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
}));

vi.mock("~/components/ui/skeleton", () => ({
  Skeleton: ({ className }: any) => (
    <div data-testid="skeleton" className={className} />
  ),
}));
/* eslint-enable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-return */

import OperatorProvisioningPage from "./page";

const mockOrg = {
  id: "1",
  name: "Test Org",
  slug: "test-org",
  active: true,
  createdAt: "2025-01-01T00:00:00Z",
  updatedAt: "2025-01-01T00:00:00Z",
};

const mockSchool = {
  id: "10",
  organizationId: "1",
  name: "Test School",
  slug: "test-school",
  subdomain: "test-school",
  address: "Main St 1",
  city: "Berlin",
  zip: "10115",
  phone: "",
  email: "",
  active: true,
  createdAt: "2025-01-01T00:00:00Z",
  updatedAt: "2025-01-01T00:00:00Z",
  organization: { ...mockOrg },
};

describe("OperatorProvisioningPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseOperatorAuth.mockReturnValue({
      isAuthenticated: true,
      operator: { id: "1", email: "operator@example.com" },
    });
    mockMutateOrgs.mockResolvedValue(undefined);
    mockMutateSchools.mockResolvedValue(undefined);
    // Suppress fetch calls for revalidation endpoint
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ status: "ok" })),
    );
  });

  function setupSWR(
    orgs: unknown[] | undefined = [mockOrg],
    schools: unknown[] | undefined = [mockSchool],
    orgsLoading = false,
    schoolsLoading = false,
  ) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    mockUseSWR.mockImplementation((key: any) => {
      if (key === "operator-organizations") {
        return {
          data: orgsLoading ? undefined : orgs,
          isLoading: orgsLoading,
          mutate: mockMutateOrgs,
        };
      }
      if (key === "operator-schools") {
        return {
          data: schoolsLoading ? undefined : schools,
          isLoading: schoolsLoading,
          mutate: mockMutateSchools,
        };
      }
      return { data: undefined, isLoading: false, mutate: vi.fn() };
    });
  }

  it("renders loading state", () => {
    setupSWR(undefined, undefined, true, true);

    render(<OperatorProvisioningPage />);

    expect(screen.getAllByTestId("skeleton").length).toBeGreaterThan(0);
  });

  it("renders organizations tab by default", () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    expect(screen.getByText("Test Org")).toBeInTheDocument();
    expect(screen.getByText("test-org")).toBeInTheDocument();
    expect(screen.getByText("Aktiv")).toBeInTheDocument();
  });

  it("renders empty state for organizations", () => {
    setupSWR([]);

    render(<OperatorProvisioningPage />);

    expect(screen.getByText("Keine Träger")).toBeInTheDocument();
    expect(screen.getAllByText("Neuer Träger").length).toBeGreaterThan(0);
  });

  it("switches to schools tab", () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));

    expect(screen.getByText("Test School")).toBeInTheDocument();
    expect(screen.getByText("test-school")).toBeInTheDocument();
  });

  it("renders empty state for schools", () => {
    setupSWR([mockOrg], []);

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));

    expect(screen.getByText("Keine Schulen")).toBeInTheDocument();
  });

  it("shows school organization name and address", () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));

    expect(screen.getByText("Test Org")).toBeInTheDocument();
    expect(screen.getByText("Main St 1, 10115, Berlin")).toBeInTheDocument();
  });

  it("opens create organization modal", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    // Click the action button in the page header
    const createButtons = screen.getAllByText("Neuer Träger");
    fireEvent.click(createButtons[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });

  it("auto-generates slug from organization name", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

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
    setupSWR();
    mockCreateOrganization.mockResolvedValue(mockOrg);

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getAllByText("Neuer Träger")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText(/Name/);
    fireEvent.change(nameInput, { target: { value: "New Org" } });

    const createButton = screen.getByText("Erstellen");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(mockCreateOrganization).toHaveBeenCalledWith({
        name: "New Org",
        slug: "new-org",
      });
      expect(mockMutateOrgs).toHaveBeenCalled();
    });
  });

  it("shows error for duplicate organization slug", async () => {
    setupSWR();

    const { OperatorApiError } = await import("~/lib/operator/api-helpers");
    mockCreateOrganization.mockRejectedValue(
      new OperatorApiError("conflict", 409),
    );

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getAllByText("Neuer Träger")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText(/Name/);
    fireEvent.change(nameInput, { target: { value: "Dup Org" } });

    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(
        screen.getByText("Ein Träger mit diesem Slug existiert bereits."),
      ).toBeInTheDocument();
    });
  });

  it("opens create school modal", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));

    const createButtons = screen.getAllByText("Neue Schule");
    fireEvent.click(createButtons[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByLabelText(/Träger/)).toBeInTheDocument();
      expect(screen.getByLabelText(/Subdomain/)).toBeInTheDocument();
    });
  });

  it("opens invite admin modal", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));

    const inviteButton = screen.getByText("Admin einladen");
    fireEvent.click(inviteButton);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(
        screen.getByText(/Admin einladen — Test School/),
      ).toBeInTheDocument();
    });
  });

  it("sends admin invitation and shows success", async () => {
    setupSWR();
    mockInviteSchoolAdmin.mockResolvedValue({
      id: "1",
      email: "admin@school.de",
      deliveryStatus: "sent",
      emailError: null,
    });

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Admin einladen"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const emailInput = screen.getByLabelText(/E-Mail/);
    fireEvent.change(emailInput, { target: { value: "admin@school.de" } });

    fireEvent.click(screen.getByText("Einladung senden"));

    await waitFor(() => {
      expect(mockInviteSchoolAdmin).toHaveBeenCalledWith("10", {
        email: "admin@school.de",
      });
      expect(screen.getByText("Einladung erstellt")).toBeInTheDocument();
      expect(screen.getByText("Gesendet")).toBeInTheDocument();
    });
  });

  it("validates slug format in organization form", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getAllByText("Neuer Träger")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText(/Name/);
    fireEvent.change(nameInput, { target: { value: "Test" } });

    // Manually set invalid slug
    const slugInput = screen.getByLabelText(/Slug/);
    fireEvent.change(slugInput, { target: { value: "INVALID SLUG!" } });

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
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getAllByText("Neuer Träger")[0]!);

    const createButton = screen.getByText("Erstellen");
    expect(createButton).toBeDisabled();
  });

  it("shows tab counts", () => {
    setupSWR([mockOrg], [mockSchool, mockSchool]);

    render(<OperatorProvisioningPage />);

    expect(screen.getByTestId("tab-organizations")).toHaveTextContent(
      "Träger (1)",
    );
    expect(screen.getByTestId("tab-schools")).toHaveTextContent("Schulen (2)");
  });

  it("passes null SWR key when not authenticated", () => {
    mockUseOperatorAuth.mockReturnValue({
      isAuthenticated: false,
      operator: null,
    });
    setupSWR(undefined, undefined);

    render(<OperatorProvisioningPage />);

    expect(mockUseSWR).toHaveBeenCalledWith(
      null,
      expect.any(Function),
      expect.any(Object),
    );
  });

  // --- Edit Organization ---

  it("opens edit organization modal with pre-filled data", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Träger bearbeiten")).toBeInTheDocument();
      expect(screen.getByDisplayValue("Test Org")).toBeInTheDocument();
      expect(screen.getByDisplayValue("test-org")).toBeInTheDocument();
    });
  });

  it("updates organization and mutates", async () => {
    setupSWR();
    mockUpdateOrganization.mockResolvedValue({
      ...mockOrg,
      name: "Updated Org",
    });

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const nameInput = screen.getByDisplayValue("Test Org");
    fireEvent.change(nameInput, { target: { value: "Updated Org" } });

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
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    expect(
      screen.getByText(/Slug-Änderungen können bestehende Verweise/),
    ).toBeInTheDocument();
  });

  it("shows conflict error when updating organization with duplicate slug", async () => {
    setupSWR();

    const { OperatorApiError } = await import("~/lib/operator/api-helpers");
    mockUpdateOrganization.mockRejectedValue(
      new OperatorApiError("conflict", 409),
    );

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(
        screen.getByText("Ein Träger mit diesem Slug existiert bereits."),
      ).toBeInTheDocument();
    });
  });

  // --- Toggle Active ---

  it("toggles organization active status", async () => {
    setupSWR();
    mockUpdateOrganization.mockResolvedValue({
      ...mockOrg,
      active: false,
    });

    render(<OperatorProvisioningPage />);

    const statusBadge = screen.getByText("Aktiv");
    fireEvent.click(statusBadge);

    await waitFor(() => {
      expect(mockUpdateOrganization).toHaveBeenCalledWith("1", {
        name: "Test Org",
        slug: "test-org",
        active: false,
      });
      expect(mockMutateOrgs).toHaveBeenCalled();
    });
  });

  it("toggles school active status", async () => {
    setupSWR();
    mockUpdateSchool.mockResolvedValue({ ...mockSchool, active: false });

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));

    const statusBadge = screen.getByText("Aktiv");
    fireEvent.click(statusBadge);

    await waitFor(() => {
      expect(mockUpdateSchool).toHaveBeenCalledWith("10", {
        organization_id: 1,
        name: "Test School",
        slug: "test-school",
        subdomain: "test-school",
        address: "Main St 1",
        city: "Berlin",
        zip: "10115",
        phone: "",
        email: "",
        active: false,
      });
      expect(mockMutateSchools).toHaveBeenCalled();
    });
  });

  // --- Edit School ---

  it("opens edit school modal with pre-filled data", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Schule bearbeiten")).toBeInTheDocument();
      expect(screen.getByDisplayValue("Test School")).toBeInTheDocument();
      expect(screen.getByDisplayValue("Main St 1")).toBeInTheDocument();
    });
  });

  it("updates school and mutates", async () => {
    setupSWR();
    mockUpdateSchool.mockResolvedValue({
      ...mockSchool,
      name: "Renamed School",
    });

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const nameInput = screen.getByDisplayValue("Test School");
    fireEvent.change(nameInput, { target: { value: "Renamed School" } });

    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(mockUpdateSchool).toHaveBeenCalledWith(
        "10",
        expect.objectContaining({ name: "Renamed School" }),
      );
      expect(mockMutateSchools).toHaveBeenCalled();
    });
  });

  it("shows subdomain warning when changing subdomain", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Both slug and subdomain are "test-school", change the subdomain (second input)
    const subdomainInputs = screen.getAllByDisplayValue("test-school");
    const subdomainInput = subdomainInputs[1]!;
    fireEvent.change(subdomainInput, { target: { value: "new-subdomain" } });

    await waitFor(() => {
      expect(
        screen.getByText(
          /Subdomain-Änderungen erfordern, dass alle Benutzer die neue Adresse verwenden/,
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows slug warning when changing school slug", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Change slug (first input with value "test-school")
    const slugInputs = screen.getAllByDisplayValue("test-school");
    const slugInput = slugInputs[0]!;
    fireEvent.change(slugInput, { target: { value: "new-slug" } });

    await waitFor(() => {
      expect(
        screen.getByText(
          /Slug-Änderungen können bestehende Verweise ungültig machen/,
        ),
      ).toBeInTheDocument();
    });
  });

  it("calls revalidation endpoint when subdomain changes", async () => {
    setupSWR();
    mockUpdateSchool.mockResolvedValue({
      ...mockSchool,
      subdomain: "new-sub",
    });
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(new Response(JSON.stringify({ status: "ok" })));

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Change subdomain
    const subdomainInputs = screen.getAllByDisplayValue("test-school");
    const subdomainInput = subdomainInputs[1]!;
    fireEvent.change(subdomainInput, { target: { value: "new-sub" } });

    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(fetchSpy).toHaveBeenCalledWith(
        "/api/operator/provisioning/revalidate-tenant",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            slugs: ["test-school", "new-sub"],
          }),
        }),
      );
    });

    fetchSpy.mockRestore();
  });

  it("does not call revalidation when subdomain is unchanged", async () => {
    setupSWR();
    mockUpdateSchool.mockResolvedValue(mockSchool);
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(new Response(JSON.stringify({ status: "ok" })));

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Only change name, not subdomain
    const nameInput = screen.getByDisplayValue("Test School");
    fireEvent.change(nameInput, { target: { value: "Renamed" } });

    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(mockUpdateSchool).toHaveBeenCalled();
    });

    // Revalidation should NOT have been called
    expect(fetchSpy).not.toHaveBeenCalledWith(
      "/api/operator/provisioning/revalidate-tenant",
      expect.anything(),
    );

    fetchSpy.mockRestore();
  });

  it("shows subdomain conflict error on school update", async () => {
    setupSWR();

    const { OperatorApiError } = await import("~/lib/operator/api-helpers");
    mockUpdateSchool.mockRejectedValue(
      new OperatorApiError("subdomain already exists", 409),
    );

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(
        screen.getByText("Eine Schule mit dieser Subdomain existiert bereits."),
      ).toBeInTheDocument();
    });
  });

  it("shows slug conflict error on school update", async () => {
    setupSWR();

    const { OperatorApiError } = await import("~/lib/operator/api-helpers");
    mockUpdateSchool.mockRejectedValue(
      new OperatorApiError("slug conflict", 409),
    );

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(
        screen.getByText(
          "Eine Schule mit diesem Slug existiert bereits in dieser Organisation.",
        ),
      ).toBeInTheDocument();
    });
  });

  // --- Error handling ---

  it("handles update organization error gracefully", async () => {
    setupSWR();
    mockUpdateOrganization.mockRejectedValue(new Error("Server error"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop
    });

    render(<OperatorProvisioningPage />);

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

  it("handles update school error gracefully", async () => {
    setupSWR();
    mockUpdateSchool.mockRejectedValue(new Error("Update failed"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop
    });

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(screen.getByText("Update failed")).toBeInTheDocument();
      expect(consoleError).toHaveBeenCalledWith(
        "school_update_failed",
        expect.objectContaining({ error: "Update failed" }),
      );
    });

    consoleError.mockRestore();
  });

  it("handles toggle active error gracefully", async () => {
    setupSWR();
    mockUpdateOrganization.mockRejectedValue(new Error("Toggle failed"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop
    });

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByText("Aktiv"));

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith(
        "organization_toggle_active_failed",
        expect.objectContaining({ error: "Toggle failed" }),
      );
    });

    consoleError.mockRestore();
  });

  it("closes edit organization modal on cancel", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Abbrechen"));

    await waitFor(() => {
      expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
    });
  });

  it("shows org change warning when changing school organization", async () => {
    const secondOrg = {
      ...mockOrg,
      id: "2",
      name: "Other Org",
      slug: "other-org",
    };
    setupSWR([mockOrg, secondOrg]);

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Change org dropdown
    const orgSelect = screen.getByDisplayValue("Test Org");
    fireEvent.change(orgSelect, { target: { value: "2" } });

    await waitFor(() => {
      expect(
        screen.getByText(/Trägerwechsel kann die Slug-Eindeutigkeit/),
      ).toBeInTheDocument();
    });
  });

  // --- Create School ---

  it("creates school successfully with all fields", async () => {
    setupSWR();
    mockCreateSchool.mockResolvedValue(mockSchool);

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getAllByText("Neue Schule")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Fill in required fields
    const orgSelect = screen.getByLabelText(/Träger/);
    fireEvent.change(orgSelect, { target: { value: "1" } });

    const nameInput = screen.getByLabelText(/Name/);
    fireEvent.change(nameInput, { target: { value: "New School" } });

    // Fill optional contact fields
    fireEvent.change(screen.getByLabelText(/Adresse/), {
      target: { value: "Hauptstr. 1" },
    });
    fireEvent.change(screen.getByLabelText(/Stadt/), {
      target: { value: "Berlin" },
    });
    fireEvent.change(screen.getByLabelText(/PLZ/), {
      target: { value: "10115" },
    });
    fireEvent.change(screen.getByLabelText(/Telefon/), {
      target: { value: "030123" },
    });
    fireEvent.change(screen.getByLabelText(/E-Mail/), {
      target: { value: "info@school.de" },
    });

    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(mockCreateSchool).toHaveBeenCalledWith(
        expect.objectContaining({
          organization_id: 1,
          name: "New School",
          slug: "new-school",
          subdomain: "new-school",
          address: "Hauptstr. 1",
          city: "Berlin",
          zip: "10115",
          phone: "030123",
          email: "info@school.de",
        }),
      );
      expect(mockMutateSchools).toHaveBeenCalled();
    });
  });

  it("validates invalid slug when creating school", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getAllByText("Neue Schule")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/Träger/), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText(/Name/), {
      target: { value: "Test" },
    });

    // Set invalid slug manually
    const slugInput = screen.getByLabelText(/Slug/);
    fireEvent.change(slugInput, { target: { value: "INVALID!" } });

    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(
        screen.getByText(
          "Slug darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.",
        ),
      ).toBeInTheDocument();
    });

    expect(mockCreateSchool).not.toHaveBeenCalled();
  });

  it("validates invalid subdomain when creating school", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getAllByText("Neue Schule")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/Träger/), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText(/Name/), {
      target: { value: "Test" },
    });

    // Set valid slug but invalid subdomain
    const subdomainInput = screen.getByLabelText(/Subdomain/);
    fireEvent.change(subdomainInput, { target: { value: "BAD DOMAIN!" } });

    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(
        screen.getByText(
          "Subdomain darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.",
        ),
      ).toBeInTheDocument();
    });

    expect(mockCreateSchool).not.toHaveBeenCalled();
  });

  it("shows subdomain conflict error when creating school", async () => {
    setupSWR();

    const { OperatorApiError } = await import("~/lib/operator/api-helpers");
    mockCreateSchool.mockRejectedValue(
      new OperatorApiError("subdomain already exists", 409),
    );

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getAllByText("Neue Schule")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/Träger/), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText(/Name/), {
      target: { value: "Dup School" },
    });

    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(
        screen.getByText("Eine Schule mit dieser Subdomain existiert bereits."),
      ).toBeInTheDocument();
    });
  });

  it("shows slug conflict error when creating school", async () => {
    setupSWR();

    const { OperatorApiError } = await import("~/lib/operator/api-helpers");
    mockCreateSchool.mockRejectedValue(
      new OperatorApiError("slug conflict", 409),
    );

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getAllByText("Neue Schule")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/Träger/), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText(/Name/), {
      target: { value: "Dup School" },
    });

    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(
        screen.getByText(
          "Eine Schule mit diesem Slug existiert bereits in dieser Organisation.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("handles generic error when creating school", async () => {
    setupSWR();
    mockCreateSchool.mockRejectedValue(new Error("Network failure"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop
    });

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getAllByText("Neue Schule")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/Träger/), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText(/Name/), {
      target: { value: "Fail School" },
    });

    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(screen.getByText("Network failure")).toBeInTheDocument();
      expect(consoleError).toHaveBeenCalledWith(
        "school_create_failed",
        expect.objectContaining({ error: "Network failure" }),
      );
    });

    consoleError.mockRestore();
  });

  it("handles generic create organization error", async () => {
    setupSWR();
    mockCreateOrganization.mockRejectedValue(new Error("Server down"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop
    });

    render(<OperatorProvisioningPage />);

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

  // --- Edit School Validation ---

  it("validates invalid slug when updating school", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Change slug to invalid
    const slugInputs = screen.getAllByDisplayValue("test-school");
    fireEvent.change(slugInputs[0]!, { target: { value: "BAD SLUG!" } });

    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(
        screen.getByText(
          "Slug darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.",
        ),
      ).toBeInTheDocument();
    });

    expect(mockUpdateSchool).not.toHaveBeenCalled();
  });

  it("validates invalid subdomain when updating school", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Change subdomain to invalid
    const subdomainInputs = screen.getAllByDisplayValue("test-school");
    fireEvent.change(subdomainInputs[1]!, {
      target: { value: "BAD DOMAIN!" },
    });

    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(
        screen.getByText(
          "Subdomain darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.",
        ),
      ).toBeInTheDocument();
    });

    expect(mockUpdateSchool).not.toHaveBeenCalled();
  });

  // --- School Toggle Active Error ---

  it("handles toggle school active error gracefully", async () => {
    setupSWR();
    mockUpdateSchool.mockRejectedValue(new Error("Toggle failed"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop
    });

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Aktiv"));

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith(
        "school_toggle_active_failed",
        expect.objectContaining({ error: "Toggle failed" }),
      );
    });

    consoleError.mockRestore();
  });

  // --- Invite Error & Close ---

  it("handles invite error gracefully", async () => {
    setupSWR();
    mockInviteSchoolAdmin.mockRejectedValue(new Error("Invite failed"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop
    });

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Admin einladen"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/E-Mail/), {
      target: { value: "admin@school.de" },
    });
    fireEvent.click(screen.getByText("Einladung senden"));

    await waitFor(() => {
      expect(screen.getByText("Invite failed")).toBeInTheDocument();
      expect(consoleError).toHaveBeenCalledWith(
        "admin_invite_failed",
        expect.objectContaining({ error: "Invite failed" }),
      );
    });

    consoleError.mockRestore();
  });

  it("sends invite with optional fields", async () => {
    setupSWR();
    mockInviteSchoolAdmin.mockResolvedValue({
      id: "1",
      email: "admin@school.de",
      deliveryStatus: "sent",
      emailError: null,
    });

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Admin einladen"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/E-Mail/), {
      target: { value: "admin@school.de" },
    });
    fireEvent.change(screen.getByLabelText(/Vorname/), {
      target: { value: "Ada" },
    });
    fireEvent.change(screen.getByLabelText(/Nachname/), {
      target: { value: "Lovelace" },
    });
    fireEvent.change(screen.getByLabelText(/Position/), {
      target: { value: "Principal" },
    });

    fireEvent.click(screen.getByText("Einladung senden"));

    await waitFor(() => {
      expect(mockInviteSchoolAdmin).toHaveBeenCalledWith("10", {
        email: "admin@school.de",
        first_name: "Ada",
        last_name: "Lovelace",
        position: "Principal",
      });
    });
  });

  it("closes edit school modal on cancel", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Abbrechen"));

    await waitFor(() => {
      expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
    });
  });

  it("auto-generates slug and subdomain from school name", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getAllByText("Neue Schule")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText(/Name/);
    fireEvent.change(nameInput, { target: { value: "Meine Schule" } });

    const slugInput = screen.getByLabelText(/Slug/);
    const subdomainInput = screen.getByLabelText(
      /Subdomain/,
    );
    expect(slugInput.value).toBe("meine-schule");
    expect(subdomainInput.value).toBe("meine-schule");
  });

  it("pre-selects org when only one exists", async () => {
    setupSWR([mockOrg]); // Only one org

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getAllByText("Neue Schule")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const orgSelect = screen.getByLabelText(/Träger/);
    expect(orgSelect.value).toBe("1");
  });

  it("shows invite result with email error", async () => {
    setupSWR();
    mockInviteSchoolAdmin.mockResolvedValue({
      id: "1",
      email: "admin@school.de",
      deliveryStatus: "failed",
      emailError: "SMTP timeout",
    });

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Admin einladen"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/E-Mail/), {
      target: { value: "admin@school.de" },
    });
    fireEvent.click(screen.getByText("Einladung senden"));

    await waitFor(() => {
      expect(screen.getByText("Einladung erstellt")).toBeInTheDocument();
      expect(screen.getByText("SMTP timeout")).toBeInTheDocument();
    });
  });

  it("shows schools loading state on schools tab", () => {
    setupSWR([mockOrg], undefined, false, true);

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));

    expect(screen.getAllByTestId("skeleton").length).toBeGreaterThan(0);
  });

  it("stops slug auto-generation after manual edit", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getAllByText("Neuer Träger")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText(/Name/);
    const slugInput = screen.getByLabelText(/Slug/);

    // Auto-generate first
    fireEvent.change(nameInput, { target: { value: "Auto" } });
    expect(slugInput.value).toBe("auto");

    // Manually edit slug
    fireEvent.change(slugInput, { target: { value: "custom-slug" } });

    // Now changing name should NOT update slug
    fireEvent.change(nameInput, { target: { value: "Changed Name" } });
    expect(slugInput.value).toBe("custom-slug");
  });
});
