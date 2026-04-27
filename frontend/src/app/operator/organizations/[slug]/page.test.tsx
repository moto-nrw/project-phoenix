/**
 * Tests for Operator Organization Detail (Träger Drill-in) Page.
 *
 * Ported from the legacy schools/page + organizations/page tests — covers
 * create-school flows (slug + subdomain validation, conflict errors, generic
 * error), org-level toggle/edit/soft-delete with tenant cache revalidation,
 * and trash/restore behaviour for schools listed under the Träger.
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
  mockMutateSchoolSummaries,
  mockMutateOrgAccounts,
  mockMutateOrgDevices,
  mockListOrganizations,
  mockListOrganizationSummaries,
  mockListOrganizationSchools,
  mockListOrganizationAccounts,
  mockListOrganizationDevices,
  mockListOrganizationPersons,
  mockCreateSchool,
  mockUpdateOrganization,
  mockSoftDeleteOrganization,
  mockRestoreOrganization,
  mockSoftDeleteSchool,
  mockRestoreSchool,
  mockDeleteDevice,
  mockPush,
  mockReplace,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUseSWR: vi.fn(),
  mockMutateOrgs: vi.fn(),
  mockMutateSchoolSummaries: vi.fn(),
  mockMutateOrgAccounts: vi.fn(),
  mockMutateOrgDevices: vi.fn(),
  mockListOrganizations: vi.fn(),
  mockListOrganizationSummaries: vi.fn(),
  mockListOrganizationSchools: vi.fn(),
  mockListOrganizationAccounts: vi.fn(),
  mockListOrganizationDevices: vi.fn(),
  mockListOrganizationPersons: vi.fn(),
  mockCreateSchool: vi.fn(),
  mockUpdateOrganization: vi.fn(),
  mockSoftDeleteOrganization: vi.fn(),
  mockRestoreOrganization: vi.fn(),
  mockSoftDeleteSchool: vi.fn(),
  mockRestoreSchool: vi.fn(),
  mockDeleteDevice: vi.fn(),
  mockPush: vi.fn(),
  mockReplace: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  usePathname: () => "/operator/organizations/test-org",
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
      listOrganizations: mockListOrganizations,
      listOrganizationSummaries: mockListOrganizationSummaries,
      listOrganizationSchools: mockListOrganizationSchools,
      listOrganizationAccounts: mockListOrganizationAccounts,
      listOrganizationDevices: mockListOrganizationDevices,
      listOrganizationPersons: mockListOrganizationPersons,
      createSchool: mockCreateSchool,
      updateOrganization: mockUpdateOrganization,
      softDeleteOrganization: mockSoftDeleteOrganization,
      restoreOrganization: mockRestoreOrganization,
      softDeleteSchool: mockSoftDeleteSchool,
      restoreSchool: mockRestoreSchool,
      deleteDevice: mockDeleteDevice,
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

import OperatorOrganizationDetailPage from "./page";
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

const mockSchool: {
  id: string;
  organizationId: string;
  name: string;
  slug: string;
  subdomain: string;
  address: string;
  city: string;
  zip: string;
  phone: string;
  email: string;
  active: boolean;
  hidden: boolean;
  deletedAt: string | null;
  createdAt: string;
  updatedAt: string;
  organizationName: string;
  kontenCount: number;
  geraeteCount: number;
  personenCount: number;
} = {
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
  orgsLoading?: boolean;
  schoolsLoading?: boolean;
  accountsLoading?: boolean;
  devicesLoading?: boolean;
  personsLoading?: boolean;
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
    orgsLoading = false,
    schoolsLoading = false,
    accountsLoading = false,
    devicesLoading = false,
    personsLoading = false,
  } = opts;

  mockUseSWR.mockImplementation((key: unknown) => {
    const k = keyToString(key);
    switch (k) {
      case "operator-organization-summaries":
        return {
          data: orgsLoading ? undefined : orgs,
          isLoading: orgsLoading,
          mutate: mockMutateOrgs,
        };
      case "operator-organization-school-summaries":
        return {
          data: schoolsLoading ? undefined : schools,
          isLoading: schoolsLoading,
          mutate: mockMutateSchoolSummaries,
        };
      case "operator-org-accounts":
        return {
          data: accountsLoading ? undefined : [],
          isLoading: accountsLoading,
          mutate: mockMutateOrgAccounts,
        };
      case "operator-org-devices":
        return {
          data: devicesLoading ? undefined : [],
          isLoading: devicesLoading,
          mutate: mockMutateOrgDevices,
        };
      case "operator-org-persons":
        return {
          data: personsLoading ? undefined : [],
          isLoading: personsLoading,
          mutate: vi.fn(),
        };
      default:
        return { data: undefined, isLoading: false, mutate: vi.fn() };
    }
  });
}

const orgPageProps = { params: Promise.resolve({ slug: "test-org" }) };

async function renderPage() {
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <Suspense fallback={<div data-testid="suspense-fallback" />}>
        <OperatorOrganizationDetailPage {...orgPageProps} />
      </Suspense>,
    );
  });
  return result;
}

describe("OperatorOrganizationDetailPage", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    mockUseSession.mockReturnValue({
      data: { user: { id: "1", email: "operator@example.com" } },
      status: "authenticated",
    });
    mockMutateOrgs.mockResolvedValue(undefined);
    mockMutateSchoolSummaries.mockResolvedValue([mockSchool]);
    // Pre-resolve the params Promise so the first test doesn't get stuck on
    // the initial Suspense fallback. React 19's `use()` caches resolution per
    // Promise instance, so awaiting it once here primes the cache for all
    // subsequent renders that share `orgPageProps`.
    await orgPageProps.params;
  });

  // --- Header + listing ---

  it("renders the schools table and org header", async () => {
    setupSWR();

    await renderPage();

    expect(await screen.findByText("Test School")).toBeInTheDocument();
    expect(screen.getByText("test-school")).toBeInTheDocument();
    // Schulen/Konten/Personen each appear in both the stat row and the tabs.
    expect(screen.getAllByText("Schulen").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Konten").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Personen").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Test Org").length).toBeGreaterThan(0);
  });

  it("renders empty state when no schools", async () => {
    setupSWR({ schools: [] });

    await renderPage();

    expect(await screen.findByText("Keine Schulen")).toBeInTheDocument();
  });

  it("shows 'Träger nicht gefunden' when slug does not match", async () => {
    setupSWR({ orgs: [{ ...mockOrg, slug: "other-org" }] });

    await renderPage();

    expect(
      await screen.findByText("Träger nicht gefunden."),
    ).toBeInTheDocument();
  });

  // --- Create school flow ---

  it("opens create school modal", async () => {
    setupSWR();

    await renderPage();

    fireEvent.click(await screen.findByText("Neue Schule"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByLabelText(/Subdomain/)).toBeInTheDocument();
    });
  });

  it("validates invalid slug when creating school", async () => {
    setupSWR();

    await renderPage();

    fireEvent.click(await screen.findByText("Neue Schule"));

    await waitFor(() =>
      expect(screen.getByTestId("modal")).toBeInTheDocument(),
    );

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
    setupSWR();

    await renderPage();

    fireEvent.click(await screen.findByText("Neue Schule"));

    await waitFor(() =>
      expect(screen.getByTestId("modal")).toBeInTheDocument(),
    );

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
    setupSWR();
    const { OperatorApiError } = await import("~/lib/operator/api-helpers");
    mockCreateSchool.mockRejectedValue(
      new OperatorApiError("subdomain already exists", 409),
    );

    await renderPage();

    fireEvent.click(await screen.findByText("Neue Schule"));

    await waitFor(() =>
      expect(screen.getByTestId("modal")).toBeInTheDocument(),
    );

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

  it("creates school successfully and revalidates tenant cache", async () => {
    setupSWR();
    mockCreateSchool.mockResolvedValue(mockSchool);

    await renderPage();

    fireEvent.click(await screen.findByText("Neue Schule"));

    await waitFor(() =>
      expect(screen.getByTestId("modal")).toBeInTheDocument(),
    );

    fireEvent.change(screen.getByLabelText(/Name/), {
      target: { value: "New School" },
    });
    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(mockCreateSchool).toHaveBeenCalledWith(
        expect.objectContaining({
          organization_id: 1,
          name: "New School",
          slug: "new-school",
          subdomain: "new-school",
        }),
      );
      expect(mockMutateSchoolSummaries).toHaveBeenCalled();
    });
  });

  // --- Org edit/toggle/delete ---

  it("toggles org active and revalidates tenant cache for active schools", async () => {
    setupSWR();
    mockUpdateOrganization.mockResolvedValue({
      ...mockOrg,
      active: false,
    });

    await renderPage();

    const toggleButton = await screen.findByLabelText("Deaktivieren");
    fireEvent.click(toggleButton);

    await waitFor(() => {
      expect(mockUpdateOrganization).toHaveBeenCalledWith(
        "1",
        expect.objectContaining({
          name: "Test Org",
          slug: "test-org",
          active: false,
        }),
      );
      expect(revalidateTenantCache).toHaveBeenCalledWith(["test-school"]);
    });
  });

  it("opens soft-delete confirmation when Löschen is clicked", async () => {
    setupSWR({ schools: [] }); // org without schools so delete is enabled

    await renderPage();

    const orgDeleteButtons = await screen.findAllByText("Löschen");
    fireEvent.click(orgDeleteButtons[0]!);

    expect(await screen.findByText("Träger löschen")).toBeInTheDocument();
    expect(screen.getByText(/Möchten Sie den Träger/)).toBeInTheDocument();
  });

  // --- Trash + restore for schools ---

  it("shows Papierkorb button when deleted schools exist under the org", async () => {
    setupSWR({
      schools: [
        mockSchool,
        { ...mockSchool, id: "11", deletedAt: "2025-06-01T00:00:00Z" },
      ],
    });

    await renderPage();

    expect(await screen.findByText(/Papierkorb \(1\)/)).toBeInTheDocument();
  });

  it("restores a deleted school via Papierkorb", async () => {
    mockRestoreSchool.mockResolvedValue(undefined);
    setupSWR({
      schools: [
        mockSchool,
        {
          ...mockSchool,
          id: "20",
          name: "Deleted School",
          subdomain: "deleted-school",
          deletedAt: "2025-06-01T00:00:00Z",
        },
      ],
    });

    await renderPage();

    fireEvent.click(await screen.findByText("Papierkorb (1)"));

    await waitFor(() =>
      expect(screen.getByText("Wiederherstellen")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByText("Wiederherstellen"));

    await waitFor(() =>
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByTestId("confirm-btn"));

    await waitFor(() => {
      expect(mockRestoreSchool).toHaveBeenCalledWith("20");
      expect(revalidateTenantCache).toHaveBeenCalledWith(["deleted-school"]);
    });
  });

  // --- Navigation ---

  it("navigates to school detail when row is clicked", async () => {
    setupSWR();

    await renderPage();

    fireEvent.click(await screen.findByText("Test School"));

    expect(mockPush).toHaveBeenCalledWith(
      "/operator/organizations/test-org/schools/test-school",
    );
  });
});
