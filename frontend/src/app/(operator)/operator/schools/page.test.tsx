import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

/* eslint-disable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-call */

const {
  mockUseOperatorAuth,
  mockUseSWR,
  mockMutateOrgs,
  mockMutateSchools,
  mockFetchOrganizations,
  mockFetchSchools,
} = vi.hoisted(() => ({
  mockUseOperatorAuth: vi.fn(),
  mockUseSWR: vi.fn(),
  mockMutateOrgs: vi.fn(),
  mockMutateSchools: vi.fn(),
  mockFetchOrganizations: vi.fn(),
  mockFetchSchools: vi.fn(),
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
    fetchOrganizations: mockFetchOrganizations,
    fetchSchools: mockFetchSchools,
  },
}));

vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({ title, badge, actionButton }: any) => (
    <div data-testid="page-header">
      <h1>{title}</h1>
      {badge && <span data-testid="badge">{badge.count}</span>}
      {actionButton}
    </div>
  ),
}));

vi.mock("~/components/ui/skeleton", () => ({
  Skeleton: ({ className }: any) => (
    <div data-testid="skeleton" className={className} />
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

vi.mock("lucide-react", () => ({
  MoreVertical: () => <span>MoreVertical</span>,
}));

// Mock child modals to simplify page-level testing
const {
  MockCreateOrganizationModal,
  MockCreateSchoolModal,
  MockInviteAdminModal,
} = vi.hoisted(() => ({
  MockCreateOrganizationModal: vi.fn(({ isOpen, onClose, onCreated }: any) =>
    isOpen ? (
      <div data-testid="org-modal">
        <button
          onClick={() => {
            onCreated({ id: "1" });
            onClose();
          }}
        >
          Create Org
        </button>
      </div>
    ) : null,
  ),
  MockCreateSchoolModal: vi.fn(
    ({ isOpen, onClose, onCreated, onCreateOrganization }: any) =>
      isOpen ? (
        <div data-testid="school-modal">
          <button
            onClick={() => {
              onCreated({ id: "1" });
              onClose();
            }}
          >
            Create School
          </button>
          <button onClick={onCreateOrganization}>Need Org</button>
        </div>
      ) : null,
  ),
  MockInviteAdminModal: vi.fn(({ isOpen, onClose }: any) =>
    isOpen ? (
      <div data-testid="invite-modal">
        <button onClick={onClose}>Close Invite</button>
      </div>
    ) : null,
  ),
}));

vi.mock("~/components/operator/create-organization-modal", () => ({
  CreateOrganizationModal: MockCreateOrganizationModal,
}));

vi.mock("~/components/operator/create-school-modal", () => ({
  CreateSchoolModal: MockCreateSchoolModal,
}));

vi.mock("~/components/operator/invite-admin-modal", () => ({
  InviteAdminModal: MockInviteAdminModal,
}));

import OperatorSchoolsPage from "./page";
import type { Organization, School } from "~/lib/operator/provisioning-helpers";

const mockOrg: Organization = {
  id: "1",
  createdAt: "2026-01-01",
  updatedAt: "2026-01-01",
  name: "Test Org",
  slug: "test-org",
  active: true,
};

const mockSchool: School = {
  id: "5",
  createdAt: "2026-01-01",
  updatedAt: "2026-01-01",
  organizationId: "1",
  name: "GGS Musterberg",
  slug: "ggs-musterberg",
  subdomain: "ggs-musterberg",
  active: true,
  address: "Musterstraße 1",
  city: "Köln",
  zip: "50667",
  phone: "0221 12345",
  email: "info@schule.de",
};

describe("OperatorSchoolsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseOperatorAuth.mockReturnValue({
      isAuthenticated: true,
      operator: { id: "1", email: "op@test.de" },
    });
  });

  function setupSWR(opts: {
    organizations?: Organization[] | undefined;
    schools?: School[] | undefined;
    isLoadingOrgs?: boolean;
    isLoadingSchools?: boolean;
  }) {
    const {
      organizations,
      schools,
      isLoadingOrgs = false,
      isLoadingSchools = false,
    } = opts;

    mockUseSWR.mockImplementation((key: string | null) => {
      if (key === "operator-organizations") {
        return {
          data: organizations,
          isLoading: isLoadingOrgs,
          mutate: mockMutateOrgs,
        };
      }
      if (key === "operator-schools") {
        return {
          data: schools,
          isLoading: isLoadingSchools,
          mutate: mockMutateSchools,
        };
      }
      return { data: undefined, isLoading: false, mutate: vi.fn() };
    });
  }

  it("renders loading skeletons", () => {
    setupSWR({ isLoadingOrgs: true, isLoadingSchools: true });
    render(<OperatorSchoolsPage />);
    expect(screen.getAllByTestId("skeleton").length).toBeGreaterThan(0);
  });

  it("renders empty state when no schools", () => {
    setupSWR({ organizations: [mockOrg], schools: [] });
    render(<OperatorSchoolsPage />);
    expect(screen.getByText("Keine Schulen")).toBeInTheDocument();
    expect(
      screen.getByText("Erstellen Sie eine neue Schule, um loszulegen."),
    ).toBeInTheDocument();
  });

  it("renders empty state when schools is undefined", () => {
    setupSWR({ organizations: [mockOrg], schools: undefined });
    render(<OperatorSchoolsPage />);
    expect(screen.getByText("Keine Schulen")).toBeInTheDocument();
  });

  it("renders schools grouped by organization", () => {
    setupSWR({ organizations: [mockOrg], schools: [mockSchool] });
    render(<OperatorSchoolsPage />);
    expect(screen.getByText("Test Org")).toBeInTheDocument();
    expect(screen.getByText("GGS Musterberg")).toBeInTheDocument();
    expect(screen.getByText("ggs-musterberg.moto-app.de")).toBeInTheDocument();
    expect(screen.getByText("ggs-musterberg")).toBeInTheDocument();
  });

  it("renders school address", () => {
    setupSWR({ organizations: [mockOrg], schools: [mockSchool] });
    render(<OperatorSchoolsPage />);
    expect(screen.getByText(/Musterstraße 1/)).toBeInTheDocument();
    expect(screen.getByText(/50667 Köln/)).toBeInTheDocument();
  });

  it("renders school contact info", () => {
    setupSWR({ organizations: [mockOrg], schools: [mockSchool] });
    render(<OperatorSchoolsPage />);
    expect(screen.getByText("0221 12345")).toBeInTheDocument();
    expect(screen.getByText("info@schule.de")).toBeInTheDocument();
  });

  it("renders school without optional fields", () => {
    const minimalSchool: School = {
      ...mockSchool,
      address: undefined,
      city: undefined,
      zip: undefined,
      phone: undefined,
      email: undefined,
    };
    setupSWR({ organizations: [mockOrg], schools: [minimalSchool] });
    render(<OperatorSchoolsPage />);
    expect(screen.getByText("GGS Musterberg")).toBeInTheDocument();
    expect(screen.queryByText("0221 12345")).not.toBeInTheDocument();
  });

  it("shows badge with school count", () => {
    setupSWR({ organizations: [mockOrg], schools: [mockSchool] });
    render(<OperatorSchoolsPage />);
    expect(screen.getByTestId("badge")).toHaveTextContent("1");
  });

  it("does not show badge when schools not loaded", () => {
    setupSWR({ organizations: undefined, schools: undefined });
    render(<OperatorSchoolsPage />);
    expect(screen.queryByTestId("badge")).not.toBeInTheDocument();
  });

  it("shows empty org with hint when it has no schools", () => {
    const emptyOrg: Organization = {
      ...mockOrg,
      id: "99",
      name: "Empty Org",
    };
    setupSWR({
      organizations: [mockOrg, emptyOrg],
      schools: [mockSchool],
    });
    render(<OperatorSchoolsPage />);
    expect(screen.getByText("Test Org")).toBeInTheDocument();
    expect(screen.getByText("Empty Org")).toBeInTheDocument();
    expect(screen.getByText("Schule erstellen")).toBeInTheDocument();
  });

  it("shows org school count badge", () => {
    setupSWR({ organizations: [mockOrg], schools: [mockSchool] });
    render(<OperatorSchoolsPage />);
    // The org group shows a count badge
    const orgSection = screen.getByText("Test Org").parentElement;
    expect(orgSection?.textContent).toContain("1");
  });

  it("opens school modal when 'Neue Schule' button clicked", async () => {
    setupSWR({ organizations: [mockOrg], schools: [mockSchool] });
    render(<OperatorSchoolsPage />);

    const createButton = screen.getByText("Neue Schule");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("school-modal")).toBeInTheDocument();
    });
  });

  it("opens school modal from empty state button", async () => {
    setupSWR({ organizations: [mockOrg], schools: [] });
    render(<OperatorSchoolsPage />);

    // There are multiple "Neue Schule" buttons in empty state
    const buttons = screen.getAllByText("Neue Schule");
    fireEvent.click(buttons[buttons.length - 1]!);

    await waitFor(() => {
      expect(screen.getByTestId("school-modal")).toBeInTheDocument();
    });
  });

  it("mutates schools after school creation", async () => {
    setupSWR({ organizations: [mockOrg], schools: [mockSchool] });
    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Neue Schule"));

    await waitFor(() => {
      expect(screen.getByTestId("school-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Create School"));

    await waitFor(() => {
      expect(mockMutateSchools).toHaveBeenCalled();
    });
  });

  it("mutates orgs after org creation", async () => {
    setupSWR({ organizations: [mockOrg], schools: [mockSchool] });
    render(<OperatorSchoolsPage />);

    // We need to open the org modal. The school modal has a "Need Org" button
    // that calls onCreateOrganization which opens the org modal
    fireEvent.click(screen.getByText("Neue Schule"));

    await waitFor(() => {
      expect(screen.getByTestId("school-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Need Org"));

    await waitFor(() => {
      expect(screen.getByTestId("org-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Create Org"));

    await waitFor(() => {
      expect(mockMutateOrgs).toHaveBeenCalled();
    });
  });

  it("opens kebab menu and invite admin modal", async () => {
    setupSWR({ organizations: [mockOrg], schools: [mockSchool] });
    render(<OperatorSchoolsPage />);

    // Open kebab menu
    const menuButton = screen.getByLabelText("Menü öffnen");
    fireEvent.click(menuButton);

    await waitFor(() => {
      expect(screen.getByText("Admin einladen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Admin einladen"));

    await waitFor(() => {
      expect(screen.getByTestId("invite-modal")).toBeInTheDocument();
    });
  });

  it("closes kebab menu on click outside", async () => {
    setupSWR({ organizations: [mockOrg], schools: [mockSchool] });
    render(<OperatorSchoolsPage />);

    const menuButton = screen.getByLabelText("Menü öffnen");
    fireEvent.click(menuButton);

    await waitFor(() => {
      expect(screen.getByText("Admin einladen")).toBeInTheDocument();
    });

    // Click outside
    fireEvent.mouseDown(document.body);

    await waitFor(() => {
      expect(screen.queryByText("Admin einladen")).not.toBeInTheDocument();
    });
  });

  it("toggles kebab menu", async () => {
    setupSWR({ organizations: [mockOrg], schools: [mockSchool] });
    render(<OperatorSchoolsPage />);

    const menuButton = screen.getByLabelText("Menü öffnen");

    // Open
    fireEvent.click(menuButton);
    await waitFor(() => {
      expect(screen.getByText("Admin einladen")).toBeInTheDocument();
    });

    // Close by clicking again
    fireEvent.click(menuButton);
    await waitFor(() => {
      expect(screen.queryByText("Admin einladen")).not.toBeInTheDocument();
    });
  });

  it("passes null SWR key when not authenticated", () => {
    mockUseOperatorAuth.mockReturnValue({
      isAuthenticated: false,
      operator: null,
    });
    setupSWR({ organizations: undefined, schools: undefined });

    render(<OperatorSchoolsPage />);

    // SWR should be called with null key
    expect(mockUseSWR).toHaveBeenCalledWith(
      null,
      expect.any(Function),
      expect.any(Object),
    );
  });

  it("passes organizations to school modal", async () => {
    setupSWR({ organizations: [mockOrg], schools: [mockSchool] });
    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Neue Schule"));

    await waitFor(() => {
      const calls = MockCreateSchoolModal.mock.calls;
      const lastCall = calls[calls.length - 1];
      expect(lastCall?.[0]?.organizations).toEqual([mockOrg]);
    });
  });

  it("passes empty array when organizations not loaded", async () => {
    setupSWR({ organizations: undefined, schools: undefined });
    render(<OperatorSchoolsPage />);

    // The modal is always rendered (just not open), so check props
    const calls = MockCreateSchoolModal.mock.calls;
    const lastCall = calls[calls.length - 1];
    expect(lastCall?.[0]?.organizations).toEqual([]);
  });

  it("handles schools for organization with no data", () => {
    setupSWR({
      organizations: undefined,
      schools: undefined,
    });
    render(<OperatorSchoolsPage />);
    // Should not crash, just show loading or empty
    expect(screen.queryByText("Test Org")).not.toBeInTheDocument();
  });
});
