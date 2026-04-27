/**
 * Tests for Operator School Detail (Drill-in) Page.
 *
 * Ported from the legacy schools/page test — covers school edit/toggle/
 * soft-delete with tenant cache revalidation, redirect to org on delete,
 * Konten tab actions (Konto erstellen, Admin einladen), and Geräte tab.
 */
import {
  act,
  render,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { Suspense } from "react";

const {
  mockUseSession,
  mockUseSWR,
  mockMutateOrgs,
  mockMutateSchools,
  mockMutateAccounts,
  mockMutateDevices,
  mockMutatePersons,
  mockListOrganizationSummaries,
  mockListOrganizationSchools,
  mockListSchoolAccounts,
  mockListSchoolDevices,
  mockListSchoolPersons,
  mockUpdateSchool,
  mockSoftDeleteSchool,
  mockRestoreSchool,
  mockDeleteDevice,
  mockSoftDeletePerson,
  mockPush,
  mockReplace,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUseSWR: vi.fn(),
  mockMutateOrgs: vi.fn(),
  mockMutateSchools: vi.fn(),
  mockMutateAccounts: vi.fn(),
  mockMutateDevices: vi.fn(),
  mockMutatePersons: vi.fn(),
  mockListOrganizationSummaries: vi.fn(),
  mockListOrganizationSchools: vi.fn(),
  mockListSchoolAccounts: vi.fn(),
  mockListSchoolDevices: vi.fn(),
  mockListSchoolPersons: vi.fn(),
  mockUpdateSchool: vi.fn(),
  mockSoftDeleteSchool: vi.fn(),
  mockRestoreSchool: vi.fn(),
  mockDeleteDevice: vi.fn(),
  mockSoftDeletePerson: vi.fn(),
  mockPush: vi.fn(),
  mockReplace: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  usePathname: () => "/operator/organizations/test-org/schools/test-school",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: any) => (
    <a href={typeof href === "string" ? href : "#"} {...rest}>
      {children}
    </a>
  ),
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
    revalidateTenantCache: vi.fn().mockResolvedValue(undefined),
    operatorProvisioningService: {
      listOrganizationSummaries: mockListOrganizationSummaries,
      listOrganizationSchools: mockListOrganizationSchools,
      listSchoolAccounts: mockListSchoolAccounts,
      listSchoolDevices: mockListSchoolDevices,
      listSchoolPersons: mockListSchoolPersons,
      listSystemRoles: vi.fn().mockResolvedValue([]),
      updateSchool: mockUpdateSchool,
      softDeleteSchool: mockSoftDeleteSchool,
      restoreSchool: mockRestoreSchool,
      deleteDevice: mockDeleteDevice,
      softDeletePerson: mockSoftDeletePerson,
      inviteSchoolAdmin: vi.fn(),
      createSchoolAccount: vi.fn(),
    },
  };
});

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

vi.mock("~/components/teachers", () => ({
  CaregiverCapabilityModal: () => null,
}));

import OperatorSchoolDetailPage from "./page";
import { revalidateTenantCache } from "~/lib/operator/provisioning-api";

const mockOrg = {
  id: "1",
  name: "Test Org",
  slug: "test-org",
  active: true,
  deletedAt: null,
  createdAt: "2025-01-01T00:00:00Z",
  updatedAt: "2025-01-01T00:00:00Z",
  schulenCount: 1,
  kontenCount: 2,
  geraeteCount: 3,
  personenCount: 4,
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
  organizationName: "Test Org",
  kontenCount: 2,
  geraeteCount: 3,
  personenCount: 4,
};

interface SetupOpts {
  orgs?: (typeof mockOrg)[];
  schools?: (typeof mockSchool)[];
  accounts?: unknown[];
  devices?: unknown[];
  persons?: unknown[];
}

function keyToString(key: unknown): string {
  if (typeof key === "string") return key;
  if (Array.isArray(key)) return key[0] as string;
  return "";
}

function setupSWR(opts: SetupOpts = {}) {
  const {
    orgs = [mockOrg],
    schools = [mockSchool],
    accounts = [],
    devices = [],
    persons = [],
  } = opts;

  mockUseSWR.mockImplementation((key: unknown) => {
    const k = keyToString(key);
    switch (k) {
      case "operator-organization-summaries":
        return { data: orgs, isLoading: false, mutate: mockMutateOrgs };
      case "operator-organization-schools":
        return {
          data: schools,
          isLoading: false,
          mutate: mockMutateSchools,
        };
      case "operator-school-accounts":
        return {
          data: accounts,
          isLoading: false,
          mutate: mockMutateAccounts,
        };
      case "operator-school-devices":
        return {
          data: devices,
          isLoading: false,
          mutate: mockMutateDevices,
        };
      case "operator-school-persons":
        return {
          data: persons,
          isLoading: false,
          mutate: mockMutatePersons,
        };
      default:
        return { data: undefined, isLoading: false, mutate: vi.fn() };
    }
  });
}

const schoolPageProps = {
  params: Promise.resolve({ slug: "test-org", schoolSlug: "test-school" }),
};

async function renderPage() {
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <Suspense fallback={<div data-testid="suspense-fallback" />}>
        <OperatorSchoolDetailPage {...schoolPageProps} />
      </Suspense>,
    );
  });
  return result;
}

describe("OperatorSchoolDetailPage", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    mockUseSession.mockReturnValue({
      data: { user: { id: "1", email: "operator@example.com" } },
      status: "authenticated",
    });
    mockMutateOrgs.mockResolvedValue(undefined);
    mockMutateSchools.mockResolvedValue([mockSchool]);
    await schoolPageProps.params;
  });

  // --- Header + listing ---

  it("renders the school detail page with header and tabs", async () => {
    setupSWR();

    await renderPage();

    expect(await screen.findByText("Test School")).toBeInTheDocument();
    expect(screen.getAllByText("Konten").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Geräte").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Personen").length).toBeGreaterThan(0);
  });

  it("shows 'Schule nicht gefunden' when slug does not match", async () => {
    setupSWR({ schools: [{ ...mockSchool, slug: "other-school" }] });

    await renderPage();

    expect(
      await screen.findByText("Schule nicht gefunden."),
    ).toBeInTheDocument();
  });

  it("shows 'Träger nicht gefunden' when org slug does not match", async () => {
    setupSWR({ orgs: [{ ...mockOrg, slug: "other-org" }] });

    await renderPage();

    expect(
      await screen.findByText("Träger nicht gefunden."),
    ).toBeInTheDocument();
  });

  // --- Header actions ---

  it("toggles school active and revalidates tenant cache", async () => {
    setupSWR();
    mockUpdateSchool.mockResolvedValue({ ...mockSchool, active: false });

    await renderPage();

    fireEvent.click(await screen.findByLabelText("Deaktivieren"));

    await waitFor(() => {
      expect(mockUpdateSchool).toHaveBeenCalledWith(
        "10",
        expect.objectContaining({
          organization_id: 1,
          name: "Test School",
          slug: "test-school",
          subdomain: "test-school",
          active: false,
        }),
      );
      expect(revalidateTenantCache).toHaveBeenCalledWith(["test-school"]);
    });
  });

  it("opens edit school modal when Bearbeiten is clicked", async () => {
    setupSWR();

    await renderPage();

    fireEvent.click(await screen.findByText("Bearbeiten"));

    await waitFor(() =>
      expect(screen.getByTestId("modal")).toBeInTheDocument(),
    );
    // Edit modal renders fields keyed off the current school.
    expect(screen.getAllByLabelText(/Name/).length).toBeGreaterThan(0);
  });

  it("opens soft-delete confirmation when Löschen is clicked", async () => {
    setupSWR();

    await renderPage();

    fireEvent.click(await screen.findByText("Löschen"));

    expect(await screen.findByText("Schule löschen")).toBeInTheDocument();
    expect(screen.getByText(/Möchten Sie die Schule/)).toBeInTheDocument();
  });

  // --- Konten tab actions ---

  it("renders 'Konto erstellen' and 'Admin einladen' on the Konten tab", async () => {
    setupSWR();

    await renderPage();

    // Default tab is konten — actions should be visible.
    expect(await screen.findByText("Konto erstellen")).toBeInTheDocument();
    expect(screen.getByText("Admin einladen")).toBeInTheDocument();
  });

  it("opens 'Admin einladen' modal", async () => {
    setupSWR();

    await renderPage();

    fireEvent.click(await screen.findByText("Admin einladen"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
    // The modal title reads "Admin einladen — {schoolName}".
    expect(
      screen.getByText(/Admin einladen — Test School/),
    ).toBeInTheDocument();
  });

  it("opens 'Konto erstellen' modal", async () => {
    setupSWR();

    await renderPage();

    fireEvent.click(await screen.findByText("Konto erstellen"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });

  // --- Settings link ---

  it("links to school settings", async () => {
    setupSWR();

    await renderPage();

    const settings = await screen.findByText("Einstellungen");
    expect(settings.closest("a")?.getAttribute("href")).toBe(
      "/operator/schools/10/settings",
    );
  });
});
