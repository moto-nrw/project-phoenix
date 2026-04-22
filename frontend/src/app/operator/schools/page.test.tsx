/**
 * Tests for Operator Schulen Page (Schools management).
 *
 * Ported from provisioning/page.test.tsx — school-specific behaviour
 * (create, edit, toggle active, soft-delete, slug/subdomain validation,
 * tenant cache revalidation side-effects, invite admin flow).
 */
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
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
} from "../provisioning/provisioning-test-helpers";

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
    expect(screen.getByText("Main St 1, 10115, Berlin")).toBeInTheDocument();
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
    expect((orgSelect as HTMLSelectElement).value).toBe("1");
  });

  // --- Invite admin ---

  it("opens invite admin modal", async () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Admin einladen"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(
        screen.getByText(/Admin einladen — Test School/),
      ).toBeInTheDocument();
    });
  });

  it("sends admin invitation and shows success", async () => {
    withDefaultSWR();
    mockInviteSchoolAdmin.mockResolvedValue({
      id: "1",
      email: "admin@school.de",
      deliveryStatus: "sent",
      emailError: null,
    });

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Admin einladen"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/E-Mail/), {
      target: { value: "admin@school.de" },
    });

    fireEvent.click(screen.getByText("Einladung senden"));

    await waitFor(() => {
      expect(mockInviteSchoolAdmin).toHaveBeenCalledWith("10", {
        email: "admin@school.de",
      });
      expect(screen.getByText("Einladung erstellt")).toBeInTheDocument();
      expect(screen.getByText("Gesendet")).toBeInTheDocument();
    });
  });

  it("handles invite error gracefully", async () => {
    withDefaultSWR();
    mockInviteSchoolAdmin.mockRejectedValue(new Error("Invite failed"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop
    });

    render(<OperatorSchoolsPage />);

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
    withDefaultSWR();
    mockInviteSchoolAdmin.mockResolvedValue({
      id: "1",
      email: "admin@school.de",
      deliveryStatus: "sent",
      emailError: null,
    });

    render(<OperatorSchoolsPage />);

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

  it("shows invite result with email error", async () => {
    withDefaultSWR();
    mockInviteSchoolAdmin.mockResolvedValue({
      id: "1",
      email: "admin@school.de",
      deliveryStatus: "failed",
      emailError: "SMTP timeout",
    });

    render(<OperatorSchoolsPage />);

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

  // --- Edit ---

  it("opens edit school modal with pre-filled data", async () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Schule bearbeiten")).toBeInTheDocument();
      expect(screen.getByDisplayValue("Test School")).toBeInTheDocument();
      expect(screen.getByDisplayValue("Main St 1")).toBeInTheDocument();
    });
  });

  it("updates school and mutates", async () => {
    withDefaultSWR();
    mockUpdateSchool.mockResolvedValue({
      ...mockSchool,
      name: "Renamed School",
    });

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByDisplayValue("Test School"), {
      target: { value: "Renamed School" },
    });

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
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const subdomainInputs = screen.getAllByDisplayValue("test-school");
    fireEvent.change(subdomainInputs[1]!, {
      target: { value: "new-subdomain" },
    });

    await waitFor(() => {
      expect(
        screen.getByText(
          /Subdomain-Änderungen erfordern, dass alle Benutzer die neue Adresse verwenden/,
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows slug warning when changing school slug", async () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const slugInputs = screen.getAllByDisplayValue("test-school");
    fireEvent.change(slugInputs[0]!, { target: { value: "new-slug" } });

    await waitFor(() => {
      expect(
        screen.getByText(
          /Slug-Änderungen können bestehende Verweise ungültig machen/,
        ),
      ).toBeInTheDocument();
    });
  });

  it("calls revalidation endpoint when subdomain changes", async () => {
    withDefaultSWR();
    mockUpdateSchool.mockResolvedValue({ ...mockSchool, subdomain: "new-sub" });
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(new Response(JSON.stringify({ status: "ok" })));

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const subdomainInputs = screen.getAllByDisplayValue("test-school");
    fireEvent.change(subdomainInputs[1]!, { target: { value: "new-sub" } });

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
    withDefaultSWR();
    mockUpdateSchool.mockResolvedValue(mockSchool);
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(new Response(JSON.stringify({ status: "ok" })));

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByDisplayValue("Test School"), {
      target: { value: "Renamed" },
    });

    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(mockUpdateSchool).toHaveBeenCalled();
    });

    expect(fetchSpy).not.toHaveBeenCalledWith(
      "/api/operator/provisioning/revalidate-tenant",
      expect.anything(),
    );

    fetchSpy.mockRestore();
  });

  it("shows subdomain conflict error on school update", async () => {
    withDefaultSWR();
    const { OperatorApiError } = await import("~/lib/operator/api-helpers");
    mockUpdateSchool.mockRejectedValue(
      new OperatorApiError("subdomain already exists", 409),
    );

    render(<OperatorSchoolsPage />);

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
    withDefaultSWR();
    const { OperatorApiError } = await import("~/lib/operator/api-helpers");
    mockUpdateSchool.mockRejectedValue(
      new OperatorApiError("slug conflict", 409),
    );

    render(<OperatorSchoolsPage />);

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

  it("handles update school error gracefully", async () => {
    withDefaultSWR();
    mockUpdateSchool.mockRejectedValue(new Error("Update failed"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop
    });

    render(<OperatorSchoolsPage />);

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

  it("validates invalid slug when updating school", async () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

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
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

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

  it("closes edit school modal on cancel", async () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

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
    withDefaultSWR({ orgs: [mockOrg, secondOrg] });

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const orgSelect = screen.getByDisplayValue("Test Org");
    fireEvent.change(orgSelect, { target: { value: "2" } });

    await waitFor(() => {
      expect(
        screen.getByText(/Trägerwechsel kann die Slug-Eindeutigkeit/),
      ).toBeInTheDocument();
    });
  });

  // --- Toggle active ---

  it("toggles school active status", async () => {
    withDefaultSWR();
    mockUpdateSchool.mockResolvedValue({ ...mockSchool, active: false });

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByLabelText("Deaktivieren"));

    await waitFor(() => {
      expect(mockUpdateSchool).toHaveBeenCalled();
      expect(mockMutateSchools).toHaveBeenCalled();
    });
  });

  it("handles toggle school active error gracefully", async () => {
    withDefaultSWR();
    mockUpdateSchool.mockRejectedValue(new Error("Toggle failed"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop
    });

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByLabelText("Deaktivieren"));

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith(
        "school_toggle_active_failed",
        expect.objectContaining({ error: "Toggle failed" }),
      );
    });

    consoleError.mockRestore();
  });

  // --- Account creation launch ---

  it("shows Konto erstellen button on school card", () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    expect(screen.getByText("Konto erstellen")).toBeInTheDocument();
  });

  it("opens create account modal when Konto erstellen is clicked", async () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Konto erstellen"));

    await waitFor(() => {
      const modal = screen.getByTestId("create-account-modal");
      expect(modal).toBeInTheDocument();
      expect(modal).toHaveTextContent("Test School");
    });
  });

  it("renders CreateAccountModal with correct school info", async () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Konto erstellen"));

    await waitFor(() => {
      const modal = screen.getByTestId("create-account-modal");
      expect(modal).toBeInTheDocument();
      expect(modal).toHaveTextContent("Test School");
      expect(modal).toHaveTextContent("10");
    });
  });

  // --- Konten navigation ---

  it("navigates to Konten page when Konten button is clicked", () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Konten"));

    expect(mockPush).toHaveBeenCalledWith(
      expect.stringContaining("/operator/accounts"),
    );
    const target = mockPush.mock.calls.at(-1)?.[0] as string;
    expect(target).toContain(`orgId=${encodeURIComponent("1")}`);
    expect(target).toContain(`schoolId=${encodeURIComponent("10")}`);
  });

  // --- Soft-delete flow ---

  it("shows Löschen button on school cards", () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    expect(screen.getByText("Löschen")).toBeInTheDocument();
  });

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

  it("opens triple-confirm dialog when Löschen clicked", async () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getByText("Löschen"));

    await waitFor(() => {
      expect(screen.getByText("Schule löschen")).toBeInTheDocument();
      expect(
        screen.getByText("Geben Sie den Schulnamen ein:"),
      ).toBeInTheDocument();
      expect(screen.getByPlaceholderText("Test School")).toBeInTheDocument();
    });
  });

  it("keeps delete button disabled until school name is typed", async () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getAllByText("Löschen")[0]!);

    await waitFor(() => {
      expect(screen.getByText("Schule löschen")).toBeInTheDocument();
    });

    const confirmButtons = screen.getAllByRole("button", { name: "Löschen" });
    const deleteBtn = confirmButtons[confirmButtons.length - 1]!;
    expect(deleteBtn).toBeDisabled();

    const input = screen.getByPlaceholderText("Test School");
    fireEvent.change(input, { target: { value: "Test" } });
    expect(deleteBtn).toBeDisabled();

    fireEvent.change(input, { target: { value: "Test School" } });
    expect(deleteBtn).not.toBeDisabled();
  });

  it("calls softDeleteSchool after typing name and confirming", async () => {
    withDefaultSWR();
    mockSoftDeleteSchool.mockResolvedValue(undefined);

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getAllByText("Löschen")[0]!);

    await waitFor(() => {
      expect(screen.getByText("Schule löschen")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByPlaceholderText("Test School"), {
      target: { value: "Test School" },
    });

    const confirmButtons = screen.getAllByRole("button", { name: "Löschen" });
    fireEvent.click(confirmButtons[confirmButtons.length - 1]!);

    await waitFor(() => {
      expect(mockSoftDeleteSchool).toHaveBeenCalledWith("10");
      expect(mockMutateSchools).toHaveBeenCalled();
    });
  });

  it("resets confirm input when dialog is closed and reopened", async () => {
    withDefaultSWR();

    render(<OperatorSchoolsPage />);

    fireEvent.click(screen.getAllByText("Löschen")[0]!);

    await waitFor(() => {
      expect(screen.getByPlaceholderText("Test School")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByPlaceholderText("Test School"), {
      target: { value: "Test" },
    });

    fireEvent.click(screen.getByText("Abbrechen"));

    await waitFor(() => {
      expect(screen.getAllByText("Löschen").length).toBeGreaterThan(0);
    });

    fireEvent.click(screen.getAllByText("Löschen")[0]!);

    await waitFor(() => {
      const input = screen.getByPlaceholderText("Test School");
      expect(input).toHaveValue("");
    });
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
});
