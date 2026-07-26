/**
 * Tests for Operator Schulen Page (Schools management).
 *
 * Ported from provisioning/page.test.tsx — now focused on the directory
 * behaviour of the schools overview plus create/trash flows.
 */
import {
  render,
  screen,
  fireEvent,
  waitFor,
  within,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const {
  mockUseSession,
  mockUseSWR,
  mockMutateOrgs,
  mockMutateSchools,
  mockListOrganizations,
  mockListSchools,
  mockCreateSchool,
  mockUpdateSchool,
  mockInviteSchoolAdmin,
  mockSoftDeleteSchool,
  mockRestoreSchool,
  mockPush,
  mockReplace,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUseSWR: vi.fn(),
  mockMutateOrgs: vi.fn(),
  mockMutateSchools: vi.fn(),
  mockListOrganizations: vi.fn(),
  mockListSchools: vi.fn(),
  mockCreateSchool: vi.fn(),
  mockUpdateSchool: vi.fn(),
  mockInviteSchoolAdmin: vi.fn(),
  mockSoftDeleteSchool: vi.fn(),
  mockRestoreSchool: vi.fn(),
  mockPush: vi.fn(),
  mockReplace: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
}));

vi.mock("~/lib/operator-url", () => ({
  isOperatorSubdomain: () => false,
  operatorPath: (path: string) =>
    path.startsWith("/operator") ? path : `/operator${path}`,
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
      listSchools: mockListSchools,
      createSchool: mockCreateSchool,
      updateSchool: mockUpdateSchool,
      inviteSchoolAdmin: mockInviteSchoolAdmin,
      softDeleteSchool: mockSoftDeleteSchool,
      restoreSchool: mockRestoreSchool,
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

vi.mock("../provisioning/create-account-modal", () => ({
  CreateAccountModal: ({ isOpen, schoolId, schoolName }: any) =>
    isOpen ? (
      <div data-testid="create-account-modal">
        <span>{schoolName}</span>
        <span>{schoolId}</span>
      </div>
    ) : null,
}));

import OperatorSchoolsPage from "./page";
import {
  mockOrg,
  mockSchool,
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

describe("OperatorSchoolsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSession.mockReturnValue({
      data: { user: { id: "1", email: "operator@example.com" } },
      status: "authenticated",
    });
    mockMutateOrgs.mockResolvedValue(undefined);
    mockMutateSchools.mockResolvedValue([mockSchool]);
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ status: "ok" })),
    );
  });

  it("renders loading state", () => {
    withDefaultSWR({ schoolsLoading: true });

    render(<OperatorSchoolsPage />);

    expect(screen.getAllByTestId("skeleton").length).toBeGreaterThan(0);
  });

  it("renders school list", () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    expect(screen.getByText("Test School")).toBeInTheDocument();
    expect(screen.getByText("test-school")).toBeInTheDocument();
  });

  it("renders empty state for schools", () => {
    withDefaultSWR({ schools: [] });

    render(<OperatorSchoolsPage />);

    expect(screen.getByText("Keine Schulen")).toBeInTheDocument();
  });

  it("shows school organization name and address", () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    expect(screen.getByText("Test Org")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("shows Verborgen badge for hidden schools", () => {
    withDefaultSWR({ schools: [{ ...mockSchool, hidden: true }] });

    render(<OperatorSchoolsPage />);

    expect(screen.getByText("Verborgen")).toBeInTheDocument();
  });

  it("does not show Verborgen badge for visible schools", () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    expect(screen.queryByText("Verborgen")).not.toBeInTheDocument();
  });

  // --- Create ---

  it("opens create school modal", async () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getAllByText("Neue Schule")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByLabelText(/Träger/)).toBeInTheDocument();
      expect(screen.getByLabelText(/Subdomain/)).toBeInTheDocument();
    });
  });

  it("creates school successfully with all fields", async () => {
    withDefaultSWR();
    mockCreateSchool.mockResolvedValue(mockSchool);

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getAllByText("Neue Schule")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/Träger/), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText(/Name/), {
      target: { value: "New School" },
    });
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
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

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

    fireEvent.change(screen.getByLabelText(/Slug/), {
      target: { value: "INVALID!" },
    });

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
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

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

    fireEvent.change(screen.getByLabelText(/Subdomain/), {
      target: { value: "BAD DOMAIN!" },
    });

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
    withDefaultSWR();
    const { OperatorApiError } = await import("~/lib/operator/api-helpers");
    mockCreateSchool.mockRejectedValue(
      new OperatorApiError("subdomain already exists", 409),
    );

    render(<OperatorSchoolsPage />);

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
    withDefaultSWR();
    const { OperatorApiError } = await import("~/lib/operator/api-helpers");
    mockCreateSchool.mockRejectedValue(
      new OperatorApiError("slug conflict", 409),
    );

    render(<OperatorSchoolsPage />);

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
    withDefaultSWR();
    mockCreateSchool.mockRejectedValue(new Error("Network failure"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop
    });

    render(<OperatorSchoolsPage />);

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

  it("auto-generates slug and subdomain from school name", async () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getAllByText("Neue Schule")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/Name/), {
      target: { value: "Meine Schule" },
    });

    const slugInput = screen.getByLabelText(/Slug/);
    const subdomainInput = screen.getByLabelText(/Subdomain/);
    expect((slugInput as HTMLInputElement).value).toBe("meine-schule");
    expect((subdomainInput as HTMLInputElement).value).toBe("meine-schule");
  });

  it("pre-selects org when only one exists", async () => {
    withDefaultSWR({ orgs: [mockOrg] });

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getAllByText("Neue Schule")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const orgSelect = screen.getByLabelText(/Träger/);
    expect(orgSelect).toHaveTextContent("Test Org");
  });

  it("navigates to the school detail page when a row is clicked", () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Test School"));

    expect(mockPush).toHaveBeenCalledWith(
      "/operator/organizations/test-org/schools/test-school",
    );
  });

  it("keeps row-level management buttons out of the directory table", () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    const table = screen.getByRole("table");
    expect(
      within(table).queryByRole("button", { name: "Konten" }),
    ).not.toBeInTheDocument();
    expect(
      within(table).queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();
    expect(
      within(table).queryByRole("button", { name: "Konto erstellen" }),
    ).not.toBeInTheDocument();
    expect(
      within(table).queryByRole("button", { name: "Admin einladen" }),
    ).not.toBeInTheDocument();
    expect(
      within(table).queryByRole("link", { name: "Einstellungen" }),
    ).not.toBeInTheDocument();
    expect(
      within(table).queryByRole("button", { name: "Löschen" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Deaktivieren")).not.toBeInTheDocument();
  });

  // --- Soft-delete flow ---

  it("shows Papierkorb toggle when deleted schools exist", () => {
    withDefaultSWR({
      schools: [
        mockSchool,
        { ...mockSchool, id: "11", deletedAt: "2025-06-01T00:00:00Z" },
      ],
    });

    render(<OperatorSchoolsPage />);

    expect(screen.getByText(/Papierkorb \(1\)/)).toBeInTheDocument();
  });

  it("shows trash view with deleted schools when Papierkorb clicked", async () => {
    const deletedSchool = {
      ...mockSchool,
      id: "20",
      name: "Deleted School",
      subdomain: "deleted-school",
      deletedAt: "2025-06-01T00:00:00Z",
    };
    withDefaultSWR({ schools: [mockSchool, deletedSchool] });

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Papierkorb (1)"));

    await waitFor(() => {
      expect(screen.getByText("Deleted School")).toBeInTheDocument();
      expect(screen.getByText("Wiederherstellen")).toBeInTheDocument();
      expect(screen.getByText("Gelöscht")).toBeInTheDocument();
    });
  });

  it("calls restoreSchool and mutates on confirm", async () => {
    mockRestoreSchool.mockResolvedValue(undefined);
    const deletedSchool = {
      ...mockSchool,
      id: "20",
      name: "Deleted School",
      subdomain: "deleted-school",
      deletedAt: "2025-06-01T00:00:00Z",
    };
    withDefaultSWR({ schools: [mockSchool, deletedSchool] });

    render(<OperatorSchoolsPage />);

    await waitFor(() => {
      expect(screen.getByText("Papierkorb (1)")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Papierkorb (1)"));

    await waitFor(() => {
      expect(screen.getByText("Wiederherstellen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Wiederherstellen"));

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId("confirm-btn"));

    await waitFor(() => {
      expect(mockRestoreSchool).toHaveBeenCalledWith("20");
      expect(mockMutateSchools).toHaveBeenCalled();
    });
  });

  // --- Column sorting (covers each column's sortValue callback) ---

  describe("column sorting", () => {
    it("sorts by Schule / Träger / Konten / Geräte / Personen / Status when headers are clicked", () => {
      withDefaultSWR({
        schools: [
          {
            ...mockSchool,
            id: "1",
            slug: "z-school",
            name: "Z School",
            subdomain: "z-school",
            organizationName: "Org B",
            kontenCount: 10,
            geraeteCount: 2,
            personenCount: 7,
            active: false,
          },
          {
            ...mockSchool,
            id: "2",
            slug: "a-school",
            name: "A School",
            subdomain: "a-school",
            organizationName: "Org A",
            kontenCount: 30,
            geraeteCount: 8,
            personenCount: 3,
            active: true,
          },
        ],
      });

      render(<OperatorSchoolsPage />);

      for (const header of [
        "Schule",
        "Träger",
        "Konten",
        "Geräte",
        "Personen",
        "Status",
      ]) {
        fireEvent.click(
          screen.getByRole("button", { name: new RegExp(`^${header} – `) }),
        );
      }

      expect(screen.getByText("A School")).toBeInTheDocument();
      expect(screen.getByText("Z School")).toBeInTheDocument();
    });
  });
});
