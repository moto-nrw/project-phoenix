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
  within,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("~/components/operator/transfer-device-modal", () => ({
  TransferDeviceModal: () => null,
}));
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
  mockListSchoolSummaries,
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
  mockListSchoolSummaries: vi.fn(),
  mockPush: vi.fn(),
  mockReplace: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

let currentSearchParams = new URLSearchParams();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  usePathname: () => "/operator/organizations/test-org/schools/test-school",
  useSearchParams: () => currentSearchParams,
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
      listSchoolSummaries: mockListSchoolSummaries,
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
  ConfirmationModal: ({
    isOpen,
    children,
    title,
    onConfirm,
    confirmText = "Bestätigen",
    isConfirmDisabled,
    isConfirmLoading,
  }: any) =>
    isOpen ? (
      <div role="dialog" aria-label={title} data-testid="confirmation-modal">
        <h2>{title}</h2>
        {children}
        <button
          type="button"
          data-testid="confirm-btn"
          onClick={onConfirm}
          disabled={isConfirmDisabled || isConfirmLoading}
        >
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

vi.mock("~/components/teachers/caregiver-capability-modal", () => ({
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
    currentSearchParams = new URLSearchParams();
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

  it("links to school settings with back URL pointing to drill-in", async () => {
    setupSWR();

    await renderPage();

    const settings = await screen.findByText("Einstellungen");
    const expected = `/operator/schools/10/settings?back=${encodeURIComponent(
      "/operator/organizations/test-org/schools/test-school",
    )}`;
    expect(settings.closest("a")?.getAttribute("href")).toBe(expected);
  });

  // --- Tab flows ---

  describe("with the Konten tab active", () => {
    const mockSchoolAccount = {
      accountId: "100",
      personId: "200",
      firstName: "Anna",
      lastName: "Beispiel",
      email: "anna@example.com",
      isStaff: true,
      isStudent: false,
      hasGuardianAccess: false,
      role: "teacher",
      createdAt: "2026-01-01T00:00:00Z",
    };

    it("renders the accounts table when accounts are present", async () => {
      currentSearchParams = new URLSearchParams("tab=konten");
      setupSWR({ accounts: [mockSchoolAccount] });

      await renderPage();

      expect(await screen.findByText("anna@example.com")).toBeInTheDocument();
    });

    it("renders empty state when there are no accounts", async () => {
      currentSearchParams = new URLSearchParams("tab=konten");
      setupSWR({ accounts: [] });

      await renderPage();

      expect(
        await screen.findByText("Keine Konten für diese Schule."),
      ).toBeInTheDocument();
    });
  });

  describe("with the Geräte tab active", () => {
    const mockDevice = {
      id: "1",
      deviceId: "OGS-001",
      deviceType: "ogs",
      name: "Empfang",
      status: "active",
      apiKey: "",
      maskedApiKey: "abc***",
      lastSeen: null,
      isOnline: false,
      schoolId: "10",
      schoolName: "Test School",
      organizationId: "1",
      organizationName: "Test Org",
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    };

    it("renders the devices table when devices are present", async () => {
      currentSearchParams = new URLSearchParams("tab=geraete");
      setupSWR({ devices: [mockDevice] });

      await renderPage();

      expect(await screen.findByText("OGS-001")).toBeInTheDocument();
    });

    it("renders empty state when there are no devices", async () => {
      currentSearchParams = new URLSearchParams("tab=geraete");
      setupSWR({ devices: [] });

      await renderPage();

      expect(
        await screen.findByText("Keine Geräte für diese Schule."),
      ).toBeInTheDocument();
    });

    it("renders the 'Neues Gerät' action button", async () => {
      currentSearchParams = new URLSearchParams("tab=geraete");
      setupSWR({ devices: [] });

      await renderPage();

      expect(await screen.findByText("Neues Gerät")).toBeInTheDocument();
    });

    it("opens the delete-device modal when a device delete is requested", async () => {
      currentSearchParams = new URLSearchParams("tab=geraete");
      setupSWR({ devices: [mockDevice] });

      await renderPage();

      fireEvent.click(
        await screen.findByRole("button", {
          name: /Aktionen für/,
        }),
      );
      fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));

      // The DeleteDeviceModal renders "Gerät löschen" as a heading.
      await waitFor(() => {
        expect(
          screen.getByRole("heading", { name: "Gerät löschen" }),
        ).toBeInTheDocument();
      });
    });

    it("calls deleteDevice and refreshes both devices and detail on confirm", async () => {
      currentSearchParams = new URLSearchParams("tab=geraete");
      setupSWR({ devices: [mockDevice] });
      mockDeleteDevice.mockResolvedValue(undefined);

      await renderPage();

      fireEvent.click(
        await screen.findByRole("button", {
          name: /Aktionen für/,
        }),
      );
      fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));

      // Two-step confirm: click "Ja, löschen" then "Endgültig löschen"
      fireEvent.click(await screen.findByText("Ja, löschen"));
      fireEvent.click(await screen.findByText("Endgültig löschen"));

      await waitFor(() => {
        expect(mockDeleteDevice).toHaveBeenCalledWith("1");
      });
      // handleDeviceDeleted runs Promise.all([mutateDevices, refreshDetail])
      // → both mutate functions are invoked.
      await waitFor(() => {
        expect(mockMutateDevices).toHaveBeenCalled();
      });
    });
  });

  describe("with the Personen tab active", () => {
    const mockPerson = {
      id: "5",
      firstName: "Anna",
      lastName: "Beispiel",
      fullName: "Anna Beispiel",
      hasAccount: true,
      accountEmail: "anna@example.com",
      hasRfidCard: false,
      isStaff: true,
      isStudent: false,
      schoolId: "10",
      schoolName: "Test School",
      organizationId: "1",
      organizationName: "Test Org",
      createdAt: "2026-01-01T00:00:00Z",
    };

    it("renders the persons table when persons are present", async () => {
      currentSearchParams = new URLSearchParams("tab=personen");
      setupSWR({ persons: [mockPerson] });

      await renderPage();

      expect(await screen.findByText("Anna Beispiel")).toBeInTheDocument();
    });

    it("renders empty state when there are no persons", async () => {
      currentSearchParams = new URLSearchParams("tab=personen");
      setupSWR({ persons: [] });

      await renderPage();

      expect(
        await screen.findByText("Keine Personen für diese Schule."),
      ).toBeInTheDocument();
    });

    it("opens the soft-delete person modal when delete is requested", async () => {
      currentSearchParams = new URLSearchParams("tab=personen");
      setupSWR({ persons: [mockPerson] });

      await renderPage();

      // The persons-table renders a "Löschen" button with title="Person löschen".
      const rowDelete = await screen.findByTitle("Person löschen");
      fireEvent.click(rowDelete);

      await waitFor(() => {
        expect(
          screen.getByRole("heading", { name: "Person löschen" }),
        ).toBeInTheDocument();
      });
    });
  });

  // --- Soft-delete school + redirect back to org drill-in ---

  it("soft-deletes the school, clears tenant cache, and routes back to the org drill-in", async () => {
    setupSWR();
    mockSoftDeleteSchool.mockResolvedValue(undefined);

    await renderPage();

    fireEvent.click(await screen.findByText("Löschen"));

    const confirmInput = await screen.findByLabelText(
      /Geben Sie den Schulnamen ein/,
    );
    fireEvent.change(confirmInput, { target: { value: "Test School" } });

    const dialog = await screen.findByRole("dialog", {
      name: "Schule löschen",
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Löschen" }));

    await waitFor(() => {
      expect(mockSoftDeleteSchool).toHaveBeenCalledWith("10");
    });
    await waitFor(() => {
      expect(revalidateTenantCache).toHaveBeenCalledWith(["test-school"]);
    });
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith(
        "/operator/organizations/test-org?tab=schulen",
      );
    });
  });

  // --- Tab navigation (covers setActiveTab + handleTabValueChange) ---
  // Tab clicks must mutate the URL via router.replace because the active tab
  // is stored in ?tab=… so deep-links and back/forward both round-trip
  // cleanly.

  it("pushes ?tab=geraete when the Geräte tab is clicked", async () => {
    setupSWR();

    await renderPage();

    const tab = await screen.findByRole("tab", { name: "Geräte" });
    // Radix Tabs activates on pointerdown + click; mirrors the sequence used
    // by the shared Tabs unit tests in components/ui/tabs.test.tsx.
    fireEvent.pointerDown(tab, { button: 0, pointerType: "mouse" });
    fireEvent.mouseDown(tab, { button: 0 });
    fireEvent.click(tab);

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith(
        "/operator/organizations/test-org/schools/test-school?tab=geraete",
        { scroll: false },
      );
    });
  });

  it("clears ?tab=… when navigating back to the default Konten tab", async () => {
    currentSearchParams = new URLSearchParams("tab=personen");
    setupSWR();

    await renderPage();

    const tab = await screen.findByRole("tab", { name: "Konten" });
    fireEvent.pointerDown(tab, { button: 0, pointerType: "mouse" });
    fireEvent.mouseDown(tab, { button: 0 });
    fireEvent.click(tab);

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith(
        "/operator/organizations/test-org/schools/test-school",
        { scroll: false },
      );
    });
  });

  // --- Toggle school active error path (covers handleToggleSchoolActive
  //     catch + slog) ---

  it("surfaces an error message when toggling the school status fails", async () => {
    setupSWR();
    mockUpdateSchool.mockRejectedValue(new Error("network down"));

    await renderPage();

    fireEvent.click(await screen.findByLabelText("Deaktivieren"));

    await waitFor(() => {
      expect(
        screen.getByText(
          "Fehler beim Ändern des Status. Bitte versuchen Sie es erneut.",
        ),
      ).toBeInTheDocument();
    });
  });

  // --- School not found / org not found redirect branches ---
  // These exercise the early-return error states that render the back-link
  // banner instead of the full drill-in.

  it("renders 'Schule nicht gefunden' when schoolSlug doesn't match", async () => {
    setupSWR({ schools: [{ ...mockSchool, slug: "other-school" }] });

    await renderPage();

    expect(
      await screen.findByText("Schule nicht gefunden."),
    ).toBeInTheDocument();
  });

  it("renders 'Träger nicht gefunden' when slug doesn't match", async () => {
    setupSWR({ orgs: [{ ...mockOrg, slug: "other-org" }] });

    await renderPage();

    expect(
      await screen.findByText("Träger nicht gefunden."),
    ).toBeInTheDocument();
  });

  // --- Tab action buttons (covers tabActions branches per tab) ---
  // Each tab surfaces a different action button row. These click-tests pin
  // that the right button is present on the right tab and that opening the
  // modal flips the open state without throwing.

  it("opens the Create Account modal from the Konten tab", async () => {
    setupSWR();

    await renderPage();

    fireEvent.click(await screen.findByText("Konto erstellen"));

    await waitFor(() =>
      expect(screen.getByTestId("modal")).toBeInTheDocument(),
    );
  });

  it("opens the Invite Admin modal from the Konten tab", async () => {
    setupSWR();

    await renderPage();

    fireEvent.click(await screen.findByText("Admin einladen"));

    await waitFor(() =>
      expect(screen.getByTestId("modal")).toBeInTheDocument(),
    );
  });

  // --- Edit school redirect (covers the EditSchoolModal.onUpdated branch) ---
  // Pins the redirect path: when the user renames the school slug, the page
  // calls listSchoolSummaries to re-resolve the canonical path and
  // router.replace's to it. This exercises the dense block of refresh +
  // organization-lookup + path-construction logic in the page.

  it("redirects to the new path when the school slug is renamed via the edit modal", async () => {
    const renamedSchool = {
      ...mockSchool,
      slug: "renamed-school",
      subdomain: "renamed-school",
    };
    mockListOrganizationSchools.mockResolvedValue([renamedSchool]);
    mockListSchoolSummaries.mockResolvedValue([renamedSchool]);
    mockUpdateSchool.mockResolvedValue(renamedSchool);
    mockMutateOrgs.mockResolvedValue([mockOrg]);
    mockMutateSchools.mockResolvedValue([renamedSchool]);
    setupSWR();

    await renderPage();

    fireEvent.click(await screen.findByText("Bearbeiten"));
    await waitFor(() =>
      expect(screen.getByTestId("modal")).toBeInTheDocument(),
    );

    fireEvent.change(screen.getByLabelText(/Slug/), {
      target: { value: "renamed-school" },
    });
    fireEvent.change(screen.getByLabelText(/Subdomain/), {
      target: { value: "renamed-school" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith(
        "/operator/organizations/test-org/schools/renamed-school",
      );
    });
  });

  it("opens the Create Device modal from the Geräte tab", async () => {
    currentSearchParams = new URLSearchParams("tab=geraete");
    setupSWR();

    await renderPage();

    fireEvent.click(await screen.findByText("Neues Gerät"));

    await waitFor(() =>
      expect(screen.getByTestId("modal")).toBeInTheDocument(),
    );
  });
});
