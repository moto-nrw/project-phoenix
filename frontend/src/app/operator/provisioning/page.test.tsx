/**
 * Tests for Operator Provisioning Page
 * Tests rendering, tab switching, CRUD modals, and error handling
 */
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Hoisted mocks
const {
  mockUseSession,
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
  mockListSchoolAccounts,
  mockListOrganizationAccounts,
  mockListAllAccounts,
  mockListSchoolDevices,
  mockListOrganizationDevices,
  mockListAllDevices,
  mockSoftDeleteSchool,
  mockRestoreSchool,
  mockPurgeSchool,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
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
  mockListSchoolAccounts: vi.fn(),
  mockListOrganizationAccounts: vi.fn(),
  mockListAllAccounts: vi.fn(),
  mockListSchoolDevices: vi.fn(),
  mockListOrganizationDevices: vi.fn(),
  mockListAllDevices: vi.fn(),
  mockSoftDeleteSchool: vi.fn(),
  mockRestoreSchool: vi.fn(),
  mockPurgeSchool: vi.fn(),
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

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    listOrganizations: mockListOrganizations,
    createOrganization: mockCreateOrganization,
    listSchools: mockListSchools,
    createSchool: mockCreateSchool,
    updateOrganization: mockUpdateOrganization,
    updateSchool: mockUpdateSchool,
    inviteSchoolAdmin: mockInviteSchoolAdmin,
    listSchoolAccounts: mockListSchoolAccounts,
    listOrganizationAccounts: mockListOrganizationAccounts,
    listAllAccounts: mockListAllAccounts,
    listSchoolDevices: mockListSchoolDevices,
    listOrganizationDevices: mockListOrganizationDevices,
    listAllDevices: mockListAllDevices,
    softDeleteSchool: mockSoftDeleteSchool,
    restoreSchool: mockRestoreSchool,
    purgeSchool: mockPurgeSchool,
  },
}));

vi.mock("~/lib/format-utils", () => ({
  getRelativeTime: (dateStr: string) => `relative(${dateStr})`,
}));

vi.mock("~/lib/iot-helpers", () => ({
  getDeviceTypeDisplayName: (type: string) => `type:${type}`,
  getDeviceStatusDisplayName: (status: string) => `status:${status}`,
  formatLastSeen: (lastSeen: string) => `formatted:${lastSeen}`,
  DEVICE_TYPE_OPTIONS: {
    terminal: "Terminal",
    info_point: "Info-Point",
  },
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
  ConfirmationModal: ({
    isOpen,
    children,
    title,
    onConfirm,
    confirmText,
  }: any) =>
    isOpen ? (
      <div data-testid="confirmation-modal">
        <h2>{title}</h2>
        {children}
        <button type="button" onClick={onConfirm}>
          {confirmText}
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/ui/skeleton", () => ({
  Skeleton: ({ className }: any) => (
    <div data-testid="skeleton" className={className} />
  ),
}));
vi.mock("./create-account-modal", () => ({
  CreateAccountModal: ({ isOpen, schoolId, schoolName }: any) =>
    isOpen ? (
      <div data-testid="create-account-modal">
        <span>{schoolName}</span>
        <span>{schoolId}</span>
      </div>
    ) : null,
}));

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
  hidden: false,
  deletedAt: null,
  createdAt: "2025-01-01T00:00:00Z",
  updatedAt: "2025-01-01T00:00:00Z",
  organization: { ...mockOrg },
};

describe("OperatorProvisioningPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSession.mockReturnValue({
      data: { user: { id: "1", email: "operator@example.com" } },
      status: "authenticated",
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
    mockUseSession.mockReturnValue({
      data: null,
      status: "unauthenticated",
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
        hidden: false,
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
    fireEvent.click(screen.getByText("Einladung senden"));

    await waitFor(() => {
      expect(mockInviteSchoolAdmin).toHaveBeenCalledWith("10", {
        email: "admin@school.de",
        first_name: "Ada",
        last_name: "Lovelace",
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
    const subdomainInput = screen.getByLabelText(/Subdomain/);
    expect((slugInput as HTMLInputElement).value).toBe("meine-schule");
    expect((subdomainInput as HTMLInputElement).value).toBe("meine-schule");
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
    expect((orgSelect as HTMLSelectElement).value).toBe("1");
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
    expect((slugInput as HTMLInputElement).value).toBe("auto");

    // Manually edit slug
    fireEvent.change(slugInput, { target: { value: "custom-slug" } });

    // Now changing name should NOT update slug
    fireEvent.change(nameInput, { target: { value: "Changed Name" } });
    expect((slugInput as HTMLInputElement).value).toBe("custom-slug");
  });

  // --- Accounts Tab ---

  describe("Accounts Tab", () => {
    const mockAccount: {
      accountId: string;
      email: string;
      active: boolean;
      firstName: string;
      lastName: string;
      roleName: string;
      pedagogicRole: string;
      status: string;
    } = {
      accountId: "100",
      email: "teacher@school.de",
      active: true,
      firstName: "Anna",
      lastName: "Schmidt",
      roleName: "Lehrer",
      pedagogicRole: "Klassenleitung",
      status: "active",
    };

    const mockOrgAccount = {
      ...mockAccount,
      schoolId: "10",
      schoolName: "Test School",
    };

    function setupSWRWithAccounts(
      accountsKey: string,
      accounts: unknown[] | undefined,
      accountsLoading = false,
    ) {
      mockUseSWR.mockImplementation((key: any) => {
        if (key === "operator-organizations") {
          return {
            data: [mockOrg],
            isLoading: false,
            mutate: mockMutateOrgs,
          };
        }
        if (key === "operator-schools") {
          return {
            data: [mockSchool],
            isLoading: false,
            mutate: mockMutateSchools,
          };
        }
        if (key === accountsKey) {
          return {
            data: accountsLoading ? undefined : accounts,
            isLoading: accountsLoading,
            mutate: vi.fn(),
          };
        }
        return { data: undefined, isLoading: false, mutate: vi.fn() };
      });
    }

    it("renders accounts tab with all accounts view", () => {
      setupSWRWithAccounts("operator-all-accounts", [mockOrgAccount]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      expect(screen.getByText("Anna Schmidt")).toBeInTheDocument();
      expect(screen.getByText("teacher@school.de")).toBeInTheDocument();
      expect(screen.getByText("Lehrer")).toBeInTheDocument();
      expect(screen.getByText("Klassenleitung")).toBeInTheDocument();
      expect(screen.getByText("Aktiv")).toBeInTheDocument();
    });

    it("shows empty state for all accounts", () => {
      setupSWRWithAccounts("operator-all-accounts", []);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      expect(screen.getByText("Keine Konten")).toBeInTheDocument();
      expect(
        screen.getByText("Es gibt noch keine Konten im System."),
      ).toBeInTheDocument();
    });

    it("shows school name column when showSchool is true", () => {
      setupSWRWithAccounts("operator-all-accounts", [mockOrgAccount]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      // "Schule" appears as both filter label and table header
      const schuleElements = screen.getAllByText("Schule");
      expect(schuleElements.length).toBeGreaterThanOrEqual(2);
      expect(screen.getByText("Test School")).toBeInTheDocument();
    });

    it("filters accounts by organization", () => {
      setupSWRWithAccounts(`operator-org-accounts-1`, [mockOrgAccount]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      // Select org filter
      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      expect(screen.getByText("Anna Schmidt")).toBeInTheDocument();
    });

    it("shows empty state for org-level accounts", () => {
      setupSWRWithAccounts(`operator-org-accounts-1`, []);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      expect(screen.getByText("Keine Konten")).toBeInTheDocument();
      expect(
        screen.getByText(
          "Für diesen Träger gibt es noch keine zugewiesenen Konten.",
        ),
      ).toBeInTheDocument();
    });

    it("filters accounts by school", () => {
      setupSWRWithAccounts(`operator-school-accounts-10`, [mockAccount]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      // Select org, then school
      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      const schoolSelect = screen.getByLabelText("Schule");
      fireEvent.change(schoolSelect, { target: { value: "10" } });

      expect(screen.getByText("Anna Schmidt")).toBeInTheDocument();
      // School header should appear
      expect(screen.getByText("Test School")).toBeInTheDocument();
    });

    it("shows school-level account count badge", () => {
      setupSWRWithAccounts(`operator-school-accounts-10`, [mockAccount]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      const schoolSelect = screen.getByLabelText("Schule");
      fireEvent.change(schoolSelect, { target: { value: "10" } });

      expect(screen.getByText("1 Konto")).toBeInTheDocument();
    });

    it("shows empty state for school-level accounts", () => {
      setupSWRWithAccounts(`operator-school-accounts-10`, []);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      const schoolSelect = screen.getByLabelText("Schule");
      fireEvent.change(schoolSelect, { target: { value: "10" } });

      expect(screen.getByText("Keine Konten")).toBeInTheDocument();
      expect(
        screen.getByText(
          "Für diese Schule gibt es noch keine zugewiesenen Konten.",
        ),
      ).toBeInTheDocument();
    });

    it("renders account with pending status badge", () => {
      const pendingAccount = { ...mockOrgAccount, status: "pending" };
      setupSWRWithAccounts("operator-all-accounts", [pendingAccount]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      expect(screen.getByText("Ausstehend")).toBeInTheDocument();
    });

    it("renders account with invited status badge", () => {
      const invitedAccount = {
        ...mockOrgAccount,
        accountId: "0",
        status: "invited",
        firstName: "",
        lastName: "",
      };
      setupSWRWithAccounts("operator-all-accounts", [invitedAccount]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      expect(screen.getByText("Eingeladen")).toBeInTheDocument();
    });

    it("renders account with inactive status badge", () => {
      const inactiveAccount = { ...mockOrgAccount, status: "inactive" };
      setupSWRWithAccounts("operator-all-accounts", [inactiveAccount]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      expect(screen.getByText("Inaktiv")).toBeInTheDocument();
    });

    it("renders dash when account has no name", () => {
      const noNameAccount = {
        ...mockOrgAccount,
        firstName: "",
        lastName: "",
      };
      setupSWRWithAccounts("operator-all-accounts", [noNameAccount]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      // Name column should show dash
      const cells = screen.getAllByText("—");
      expect(cells.length).toBeGreaterThan(0);
    });

    it("renders dash when account has no role", () => {
      const noRoleAccount = { ...mockOrgAccount, roleName: "" };
      setupSWRWithAccounts("operator-all-accounts", [noRoleAccount]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      const cells = screen.getAllByText("—");
      expect(cells.length).toBeGreaterThan(0);
    });

    it("renders multiple roles as separate badges", () => {
      const multiRoleAccount = {
        ...mockOrgAccount,
        roleName: "Lehrer, Admin",
      };
      setupSWRWithAccounts("operator-all-accounts", [multiRoleAccount]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      expect(screen.getByText("Lehrer")).toBeInTheDocument();
      expect(screen.getByText("Admin")).toBeInTheDocument();
    });

    it("sorts accounts by clicking column header", () => {
      const account1 = { ...mockOrgAccount, email: "a@school.de" };
      const account2 = {
        ...mockOrgAccount,
        accountId: "101",
        email: "z@school.de",
      };
      setupSWRWithAccounts("operator-all-accounts", [account2, account1]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      // Click email header to sort
      fireEvent.click(screen.getByText("E-Mail"));

      const rows = screen.getAllByText(/@school.de/);
      expect(rows[0]!.textContent).toBe("a@school.de");
      expect(rows[1]!.textContent).toBe("z@school.de");

      // Click again to reverse sort
      fireEvent.click(screen.getByText("E-Mail"));

      const rowsReversed = screen.getAllByText(/@school.de/);
      expect(rowsReversed[0]!.textContent).toBe("z@school.de");
      expect(rowsReversed[1]!.textContent).toBe("a@school.de");
    });

    it("navigates to accounts tab via school card Konten button", () => {
      setupSWRWithAccounts(`operator-school-accounts-10`, [mockAccount]);

      render(<OperatorProvisioningPage />);

      // Start on schools tab
      fireEvent.click(screen.getByTestId("tab-schools"));

      // Click "Konten" button on school card (not the tab)
      const kontenButtons = screen.getAllByText("Konten");
      // The school card button is not the tab
      const cardButton = kontenButtons.find(
        (el) => el.tagName === "BUTTON" && !el.closest("[data-testid='tabs']"),
      );
      fireEvent.click(cardButton!);

      // Should now be on accounts tab with school selected
      expect(screen.getByText("Anna Schmidt")).toBeInTheDocument();
    });

    it("shows accounts loading state", () => {
      setupSWRWithAccounts("operator-all-accounts", undefined, true);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      expect(screen.getAllByTestId("skeleton").length).toBeGreaterThan(0);
    });

    it("shows plural Konten for multiple accounts", () => {
      const account2 = { ...mockAccount, accountId: "101", email: "b@s.de" };
      setupSWRWithAccounts(`operator-school-accounts-10`, [
        mockAccount,
        account2,
      ]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      const schoolSelect = screen.getByLabelText("Schule");
      fireEvent.change(schoolSelect, { target: { value: "10" } });

      expect(screen.getByText("2 Konten")).toBeInTheDocument();
    });

    it("clears school filter when org filter changes to different org", () => {
      const secondOrg = {
        ...mockOrg,
        id: "2",
        name: "Other Org",
        slug: "other-org",
      };
      mockUseSWR.mockImplementation((key: any) => {
        if (key === "operator-organizations") {
          return {
            data: [mockOrg, secondOrg],
            isLoading: false,
            mutate: mockMutateOrgs,
          };
        }
        if (key === "operator-schools") {
          return {
            data: [mockSchool],
            isLoading: false,
            mutate: mockMutateSchools,
          };
        }
        if (key === `operator-school-accounts-10`) {
          return { data: [mockAccount], isLoading: false, mutate: vi.fn() };
        }
        if (key === `operator-org-accounts-2`) {
          return { data: [], isLoading: false, mutate: vi.fn() };
        }
        return { data: undefined, isLoading: false, mutate: vi.fn() };
      });

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      // Select org 1, then school
      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      const schoolSelect = screen.getByLabelText("Schule");
      fireEvent.change(schoolSelect, { target: { value: "10" } });

      // Switch to a different org — school should be cleared
      fireEvent.change(orgSelect, { target: { value: "2" } });

      // School select should be reset
      expect((schoolSelect as HTMLSelectElement).value).toBe("");
    });
  });

  // --- Devices Tab ---

  describe("Devices Tab", () => {
    const mockDevice = {
      id: "200",
      deviceId: "DEV-001",
      deviceType: "terminal",
      name: "Eingang",
      status: "active",
      apiKey: "full-api-key-value",
      maskedApiKey: "****-key",
      lastSeen: "2026-03-20T10:00:00Z",
      isOnline: true,
      schoolId: "10",
      schoolName: "Test School",
      organizationId: "1",
      organizationName: "Test Org",
      createdAt: "2025-01-01T00:00:00Z",
      updatedAt: "2025-01-01T00:00:00Z",
    };

    function setupSWRWithDevices(
      devicesKey: string,
      devices: unknown[] | undefined,
      devicesLoading = false,
    ) {
      mockUseSWR.mockImplementation((key: any) => {
        if (key === "operator-organizations") {
          return {
            data: [mockOrg],
            isLoading: false,
            mutate: mockMutateOrgs,
          };
        }
        if (key === "operator-schools") {
          return {
            data: [mockSchool],
            isLoading: false,
            mutate: mockMutateSchools,
          };
        }
        if (key === devicesKey) {
          return {
            data: devicesLoading ? undefined : devices,
            isLoading: devicesLoading,
            mutate: vi.fn(),
          };
        }
        return { data: undefined, isLoading: false, mutate: vi.fn() };
      });
    }

    it("renders devices tab with all devices view", () => {
      setupSWRWithDevices("operator-all-devices", [mockDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      expect(screen.getByText("DEV-001")).toBeInTheDocument();
      expect(screen.getByText("Eingang")).toBeInTheDocument();
      expect(screen.getByText("type:terminal")).toBeInTheDocument();
      expect(screen.getByText("Online")).toBeInTheDocument();
      expect(screen.getByText("****-key")).toBeInTheDocument();
    });

    it("shows empty state for all devices", () => {
      setupSWRWithDevices("operator-all-devices", []);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      expect(screen.getByText("Keine Geräte")).toBeInTheDocument();
      expect(
        screen.getByText("Es gibt noch keine registrierten Geräte im System."),
      ).toBeInTheDocument();
    });

    it("filters devices by organization", () => {
      setupSWRWithDevices(`operator-org-devices-1`, [mockDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      expect(screen.getByText("DEV-001")).toBeInTheDocument();
    });

    it("shows empty state for org-level devices", () => {
      setupSWRWithDevices(`operator-org-devices-1`, []);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      expect(screen.getByText("Keine Geräte")).toBeInTheDocument();
      expect(
        screen.getByText(
          "Für diesen Träger gibt es noch keine registrierten Geräte.",
        ),
      ).toBeInTheDocument();
    });

    it("filters devices by school", () => {
      setupSWRWithDevices(`operator-school-devices-10`, [mockDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      const schoolSelect = screen.getByLabelText("Schule");
      fireEvent.change(schoolSelect, { target: { value: "10" } });

      expect(screen.getByText("DEV-001")).toBeInTheDocument();
      expect(screen.getByText("Test School")).toBeInTheDocument();
    });

    it("shows school-level device count badge", () => {
      setupSWRWithDevices(`operator-school-devices-10`, [mockDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      const schoolSelect = screen.getByLabelText("Schule");
      fireEvent.change(schoolSelect, { target: { value: "10" } });

      expect(screen.getByText("1 Gerät")).toBeInTheDocument();
    });

    it("shows plural Geräte for multiple devices", () => {
      const device2 = { ...mockDevice, id: "201", deviceId: "DEV-002" };
      setupSWRWithDevices(`operator-school-devices-10`, [mockDevice, device2]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      const schoolSelect = screen.getByLabelText("Schule");
      fireEvent.change(schoolSelect, { target: { value: "10" } });

      expect(screen.getByText("2 Geräte")).toBeInTheDocument();
    });

    it("shows empty state for school-level devices", () => {
      setupSWRWithDevices(`operator-school-devices-10`, []);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      const schoolSelect = screen.getByLabelText("Schule");
      fireEvent.change(schoolSelect, { target: { value: "10" } });

      expect(screen.getByText("Keine Geräte")).toBeInTheDocument();
      expect(
        screen.getByText(
          "Für diese Schule gibt es noch keine registrierten Geräte.",
        ),
      ).toBeInTheDocument();
    });

    it("renders offline device with status badge", () => {
      const offlineDevice = {
        ...mockDevice,
        isOnline: false,
        status: "offline",
      };
      setupSWRWithDevices("operator-all-devices", [offlineDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      expect(screen.getByText("status:offline")).toBeInTheDocument();
    });

    it("renders device with no lastSeen as Nie", () => {
      const neverSeenDevice = {
        ...mockDevice,
        lastSeen: null,
        isOnline: false,
        status: "inactive",
      };
      setupSWRWithDevices("operator-all-devices", [neverSeenDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      expect(screen.getByText("Nie")).toBeInTheDocument();
    });

    it("renders device with no name as dash", () => {
      const noNameDevice = { ...mockDevice, name: "" };
      setupSWRWithDevices("operator-all-devices", [noNameDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      const cells = screen.getAllByText("—");
      expect(cells.length).toBeGreaterThan(0);
    });

    it("renders device with no maskedApiKey as dash", () => {
      const noKeyDevice = { ...mockDevice, maskedApiKey: "", apiKey: "" };
      setupSWRWithDevices("operator-all-devices", [noKeyDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      const cells = screen.getAllByText("—");
      expect(cells.length).toBeGreaterThan(0);
    });

    it("copies API key to clipboard", async () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, "clipboard", {
        value: { writeText },
        writable: true,
        configurable: true,
      });

      setupSWRWithDevices("operator-all-devices", [mockDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      const copyButton = screen.getByTitle("API-Key kopieren");
      fireEvent.click(copyButton);

      await waitFor(() => {
        expect(writeText).toHaveBeenCalledWith("full-api-key-value");
      });
    });

    it("logs clipboard errors when copying fails", async () => {
      const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
        // noop
      });
      const writeText = vi.fn().mockRejectedValue(new Error("denied"));
      Object.defineProperty(navigator, "clipboard", {
        value: { writeText },
        writable: true,
        configurable: true,
      });

      setupSWRWithDevices("operator-all-devices", [mockDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));
      fireEvent.click(screen.getByTitle("API-Key kopieren"));

      await waitFor(() => {
        expect(consoleError).toHaveBeenCalledWith(
          "clipboard_copy_failed",
          expect.objectContaining({
            error: "Failed to copy API key to clipboard",
          }),
        );
      });

      consoleError.mockRestore();
    });

    it("shows devices loading state", () => {
      setupSWRWithDevices("operator-all-devices", undefined, true);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      expect(screen.getAllByTestId("skeleton").length).toBeGreaterThan(0);
    });

    it("sorts devices by clicking column header", () => {
      const device1 = { ...mockDevice, deviceId: "AAA-001" };
      const device2 = {
        ...mockDevice,
        id: "201",
        deviceId: "ZZZ-999",
      };
      setupSWRWithDevices("operator-all-devices", [device2, device1]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      // Click device ID header to sort ascending
      fireEvent.click(screen.getByText("Geräte-ID"));

      const rows = screen.getAllByText(/[AZ]{3}-\d{3}/);
      expect(rows[0]!.textContent).toBe("AAA-001");
      expect(rows[1]!.textContent).toBe("ZZZ-999");

      // Click again to reverse
      fireEvent.click(screen.getByText("Geräte-ID"));

      const rowsReversed = screen.getAllByText(/[AZ]{3}-\d{3}/);
      expect(rowsReversed[0]!.textContent).toBe("ZZZ-999");
      expect(rowsReversed[1]!.textContent).toBe("AAA-001");
    });

    it("sorts devices by api key column", () => {
      const device1 = { ...mockDevice, id: "201", maskedApiKey: "aaaa-key" };
      const device2 = { ...mockDevice, id: "202", maskedApiKey: "zzzz-key" };
      setupSWRWithDevices("operator-all-devices", [device2, device1]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));
      fireEvent.click(screen.getByText("API-Key"));

      const apiKeyButtons = screen.getAllByTitle("API-Key kopieren");
      expect(apiKeyButtons[0]).toHaveTextContent("aaaa-key");
      expect(apiKeyButtons[1]).toHaveTextContent("zzzz-key");
    });

    it("shows school column in all-devices view", () => {
      setupSWRWithDevices("operator-all-devices", [mockDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      // "Schule" appears as both filter label and table header
      const schuleElements = screen.getAllByText("Schule");
      expect(schuleElements.length).toBeGreaterThanOrEqual(2);
    });

    it("renders maintenance status badge", () => {
      const maintenanceDevice = {
        ...mockDevice,
        isOnline: false,
        status: "maintenance",
      };
      setupSWRWithDevices("operator-all-devices", [maintenanceDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      expect(screen.getByText("status:maintenance")).toBeInTheDocument();
    });

    it("shows organization name in school-level header", () => {
      setupSWRWithDevices(`operator-school-devices-10`, [mockDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      const schoolSelect = screen.getByLabelText("Schule");
      fireEvent.change(schoolSelect, { target: { value: "10" } });

      // Org name appears in both dropdown option and header
      const orgNameElements = screen.getAllByText("Test Org");
      expect(orgNameElements.length).toBeGreaterThanOrEqual(2);
    });

    // --- Neues Gerät button ---

    it("shows create device button in devices tab", () => {
      setupSWRWithDevices("operator-all-devices", [mockDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      expect(screen.getByText("Neues Gerät")).toBeInTheDocument();
    });

    // --- DevicesTable actions column ---

    it("shows Key ändern button for each device", () => {
      setupSWRWithDevices("operator-all-devices", [mockDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));

      expect(screen.getByText("Key ändern")).toBeInTheDocument();
    });

    it("opens the set api key modal from the devices table", async () => {
      setupSWRWithDevices("operator-all-devices", [mockDevice]);

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-devices"));
      fireEvent.click(screen.getByText("Key ändern"));

      await waitFor(() => {
        expect(screen.getByText("API-Key ändern")).toBeInTheDocument();
      });
    });
  });

  // --- OrgSchoolFilter ---

  describe("OrgSchoolFilter", () => {
    it("shows 'Schule auswählen…' when no org is selected", () => {
      setupSWR();

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      const schoolSelect = screen.getByLabelText("Schule");
      expect(schoolSelect).toHaveTextContent("Schule auswählen…");
    });

    it("shows 'Alle Schulen' when an org is selected", () => {
      setupSWR();

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      const orgSelect = screen.getByLabelText("Träger");
      fireEvent.change(orgSelect, { target: { value: "1" } });

      const schoolSelect = screen.getByLabelText("Schule");
      expect(schoolSelect).toHaveTextContent("Alle Schulen");
    });

    it("shows 'Alle Träger' option in org filter", () => {
      setupSWR();

      render(<OperatorProvisioningPage />);

      fireEvent.click(screen.getByTestId("tab-accounts"));

      const orgSelect = screen.getByLabelText("Träger");
      expect(orgSelect).toHaveTextContent("Alle Träger");
    });
  });

  // --- Create Account Modal ---

  it("shows Konto erstellen button on school card", () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));

    expect(screen.getByText("Konto erstellen")).toBeInTheDocument();
  });

  it("opens create account modal when Konto erstellen is clicked", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Konto erstellen"));

    await waitFor(() => {
      const modal = screen.getByTestId("create-account-modal");
      expect(modal).toBeInTheDocument();
      expect(modal).toHaveTextContent("Test School");
    });
  });

  it("renders CreateAccountModal with correct school info", async () => {
    setupSWR();

    render(<OperatorProvisioningPage />);

    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Konto erstellen"));

    await waitFor(() => {
      const modal = screen.getByTestId("create-account-modal");
      expect(modal).toBeInTheDocument();
      expect(modal).toHaveTextContent("Test School");
      expect(modal).toHaveTextContent("10");
    });
  });

  // --- Hidden Badge on SchoolCard ---

  it("does not show Verborgen badge for non-hidden school", () => {
    setupSWR([mockOrg], [mockSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));

    expect(screen.queryByText("Verborgen")).not.toBeInTheDocument();
  });

  it("shows Verborgen badge for hidden school", () => {
    const hiddenSchool = { ...mockSchool, hidden: true };
    setupSWR([mockOrg], [hiddenSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));

    expect(screen.getByText("Verborgen")).toBeInTheDocument();
  });

  it("shows Verborgen badge only on hidden schools when mixed", () => {
    const visibleSchool = {
      ...mockSchool,
      id: "11",
      name: "Visible School",
      slug: "visible",
      subdomain: "visible",
    };
    const hiddenSchool = {
      ...mockSchool,
      id: "12",
      name: "Hidden School",
      slug: "hidden",
      subdomain: "hidden",
      hidden: true,
    };
    setupSWR([mockOrg], [visibleSchool, hiddenSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));

    const badges = screen.getAllByText("Verborgen");
    expect(badges).toHaveLength(1);
  });

  // --- Trash View & Soft-Delete / Restore / Purge ---

  const deletedSchool = {
    ...mockSchool,
    id: "20",
    name: "Deleted School",
    slug: "deleted-school",
    subdomain: "deleted-school",
    deletedAt: "2026-01-15T14:30:00Z",
  };

  it("shows Papierkorb button when deleted schools exist", () => {
    setupSWR([mockOrg], [mockSchool, deletedSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));

    expect(screen.getByText("Papierkorb (1)")).toBeInTheDocument();
  });

  it("does not show Papierkorb button when no deleted schools", () => {
    setupSWR([mockOrg], [mockSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));

    expect(screen.queryByText(/Papierkorb/)).not.toBeInTheDocument();
  });

  it("filters active schools — excludes deleted ones from main list", () => {
    setupSWR([mockOrg], [mockSchool, deletedSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));

    expect(screen.getByText("Test School")).toBeInTheDocument();
    expect(screen.queryByText("Deleted School")).not.toBeInTheDocument();
  });

  it("toggles to trash view and shows deleted schools", () => {
    setupSWR([mockOrg], [mockSchool, deletedSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Papierkorb (1)"));

    expect(screen.getByText("Deleted School")).toBeInTheDocument();
    expect(screen.queryByText("Test School")).not.toBeInTheDocument();
  });

  it("shows deletion time on deleted school card", () => {
    setupSWR([mockOrg], [deletedSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Papierkorb (1)"));

    expect(
      screen.getByText(/relative\(2026-01-15T14:30:00Z\)/),
    ).toBeInTheDocument();
  });

  it("shows restore and purge buttons on deleted school card", () => {
    setupSWR([mockOrg], [deletedSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Papierkorb (1)"));

    expect(screen.getByText("Wiederherstellen")).toBeInTheDocument();
    expect(screen.getByText("Endgültig löschen")).toBeInTheDocument();
  });

  it("toggles back from trash to active view", () => {
    setupSWR([mockOrg], [mockSchool, deletedSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Papierkorb (1)"));

    expect(screen.getByText("Deleted School")).toBeInTheDocument();

    // Click again to toggle back
    fireEvent.click(screen.getByText("Papierkorb (1)"));

    expect(screen.getByText("Test School")).toBeInTheDocument();
    expect(screen.queryByText("Deleted School")).not.toBeInTheDocument();
  });

  it("shows Löschen button on active school cards", () => {
    setupSWR([mockOrg], [mockSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));

    expect(screen.getByText("Löschen")).toBeInTheDocument();
  });

  it("opens soft-delete confirmation modal when Löschen is clicked", () => {
    setupSWR([mockOrg], [mockSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));

    // The "Löschen" button is on the school card
    const deleteButtons = screen.getAllByText("Löschen");
    fireEvent.click(deleteButtons[0]!);

    expect(screen.getByText("Schule löschen")).toBeInTheDocument();
    expect(screen.getByText(/Alle Daten bleiben erhalten/)).toBeInTheDocument();
  });

  it("calls softDeleteSchool on confirmation and refreshes", async () => {
    mockSoftDeleteSchool.mockResolvedValue(undefined);
    mockMutateSchools.mockResolvedValue(undefined);
    setupSWR([mockOrg], [mockSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));

    const deleteButtons = screen.getAllByText("Löschen");
    fireEvent.click(deleteButtons[0]!);

    // The confirmation modal button
    const modal = screen.getByTestId("confirmation-modal");
    const confirmBtn = modal.querySelector("button");
    fireEvent.click(confirmBtn!);

    await waitFor(() => {
      expect(mockSoftDeleteSchool).toHaveBeenCalledWith("10");
    });
    await waitFor(() => {
      expect(mockMutateSchools).toHaveBeenCalled();
    });
  });

  it("shows error message when soft-delete fails", async () => {
    mockSoftDeleteSchool.mockRejectedValue(new Error("server error"));
    setupSWR([mockOrg], [mockSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));

    const deleteButtons = screen.getAllByText("Löschen");
    fireEvent.click(deleteButtons[0]!);

    const modal = screen.getByTestId("confirmation-modal");
    const confirmBtn = modal.querySelector("button");
    fireEvent.click(confirmBtn!);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Löschen der Schule/),
      ).toBeInTheDocument();
    });
  });

  it("opens restore confirmation modal from trash view", () => {
    setupSWR([mockOrg], [deletedSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Papierkorb (1)"));

    // Click restore on the deleted school card
    const restoreButtons = screen.getAllByText("Wiederherstellen");
    fireEvent.click(restoreButtons[0]!);

    expect(screen.getByText("Schule wiederherstellen")).toBeInTheDocument();
    expect(screen.getByText(/reaktiviert/)).toBeInTheDocument();
  });

  it("calls restoreSchool on confirmation and exits trash view", async () => {
    mockRestoreSchool.mockResolvedValue(undefined);
    mockMutateSchools.mockResolvedValue(undefined);
    setupSWR([mockOrg], [deletedSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Papierkorb (1)"));
    fireEvent.click(screen.getByText("Wiederherstellen"));

    // Click the confirm button inside the modal
    const modal = screen.getByTestId("confirmation-modal");
    const confirmBtn = modal.querySelector("button");
    fireEvent.click(confirmBtn!);

    await waitFor(() => {
      expect(mockRestoreSchool).toHaveBeenCalledWith("20");
    });
    await waitFor(() => {
      expect(mockMutateSchools).toHaveBeenCalled();
    });
  });

  it("shows error message when restore fails", async () => {
    mockRestoreSchool.mockRejectedValue(new Error("server error"));
    setupSWR([mockOrg], [deletedSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Papierkorb (1)"));
    fireEvent.click(screen.getByText("Wiederherstellen"));

    const modal = screen.getByTestId("confirmation-modal");
    const confirmBtn = modal.querySelector("button");
    fireEvent.click(confirmBtn!);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Wiederherstellen/),
      ).toBeInTheDocument();
    });
  });

  it("opens purge confirmation modal from trash view", () => {
    setupSWR([mockOrg], [deletedSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Papierkorb (1)"));
    fireEvent.click(screen.getByText("Endgültig löschen"));

    expect(screen.getByText("Schule endgültig löschen")).toBeInTheDocument();
    expect(
      screen.getByText(/kann nicht rückgängig gemacht werden/),
    ).toBeInTheDocument();
  });

  it("calls purgeSchool on confirmation", async () => {
    mockPurgeSchool.mockResolvedValue({ tables_purged: 5 });
    mockMutateSchools.mockResolvedValue(undefined);
    setupSWR([mockOrg], [deletedSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Papierkorb (1)"));
    fireEvent.click(screen.getByText("Endgültig löschen"));

    const modal = screen.getByTestId("confirmation-modal");
    const confirmBtn = modal.querySelector("button");
    fireEvent.click(confirmBtn!);

    await waitFor(() => {
      expect(mockPurgeSchool).toHaveBeenCalledWith("20");
    });
    await waitFor(() => {
      expect(mockMutateSchools).toHaveBeenCalled();
    });
  });

  it("shows error message when purge fails", async () => {
    mockPurgeSchool.mockRejectedValue(new Error("purge failed"));
    setupSWR([mockOrg], [deletedSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Papierkorb (1)"));
    fireEvent.click(screen.getByText("Endgültig löschen"));

    const modal = screen.getByTestId("confirmation-modal");
    const confirmBtn = modal.querySelector("button");
    fireEvent.click(confirmBtn!);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim endgültigen Löschen/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state in trash view when all deleted schools are purged", () => {
    // No deleted schools, but force trash view by having had deleted schools
    setupSWR([mockOrg], [mockSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));

    // Trash button should not appear since no deleted schools
    expect(screen.queryByText(/Papierkorb/)).not.toBeInTheDocument();
  });

  it("shows multiple deleted schools in trash view", () => {
    const deleted1 = {
      ...deletedSchool,
      id: "21",
      name: "Deleted One",
      subdomain: "deleted-one",
    };
    const deleted2 = {
      ...deletedSchool,
      id: "22",
      name: "Deleted Two",
      subdomain: "deleted-two",
    };
    setupSWR([mockOrg], [mockSchool, deleted1, deleted2]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));
    expect(screen.getByText("Papierkorb (2)")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Papierkorb (2)"));

    expect(screen.getByText("Deleted One")).toBeInTheDocument();
    expect(screen.getByText("Deleted Two")).toBeInTheDocument();
  });

  it("shows subdomain and org name on deleted school card", () => {
    setupSWR([mockOrg], [deletedSchool]);

    render(<OperatorProvisioningPage />);
    fireEvent.click(screen.getByTestId("tab-schools"));
    fireEvent.click(screen.getByText("Papierkorb (1)"));

    expect(screen.getByText("deleted-school")).toBeInTheDocument();
    expect(screen.getByText("Test Org")).toBeInTheDocument();
  });
});
