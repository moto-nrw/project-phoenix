/**
 * Tests for Operator Personen Page (People management).
 *
 * Ported from provisioning/page.test.tsx (Persons Tab). School selection
 * moved from client state (set via another tab) to URL query params
 * (schoolId, orgId) driven by the page's own OrgSchoolFilter.
 */
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const {
  mockUseSession,
  mockUseSWR,
  mockMutateOrgs,
  mockMutateSchools,
  mockMutatePersons,
  mockListOrganizations,
  mockListSchools,
  mockListSchoolPersons,
  mockSoftDeletePerson,
  mockSearchParamsGet,
  mockReplace,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUseSWR: vi.fn(),
  mockMutateOrgs: vi.fn(),
  mockMutateSchools: vi.fn(),
  mockMutatePersons: vi.fn(),
  mockListOrganizations: vi.fn(),
  mockListSchools: vi.fn(),
  mockListSchoolPersons: vi.fn(),
  mockSoftDeletePerson: vi.fn(),
  mockSearchParamsGet: vi.fn((_key: string): string | null => null),
  mockReplace: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: mockReplace }),
  useSearchParams: () => ({
    get: mockSearchParamsGet,
    toString: () => "",
  }),
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
      listSchoolPersons: mockListSchoolPersons,
      softDeletePerson: mockSoftDeletePerson,
    },
  };
});

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

vi.mock("~/components/ui/skeleton", () => ({
  Skeleton: ({ className }: any) => (
    <div data-testid="skeleton" className={className} />
  ),
}));

import OperatorPersonsPage from "./page";
import {
  mockSchool,
  setupSWR,
} from "~/test/helpers/operator-provisioning/provisioning-test-helpers";

const mockPerson = {
  id: "300",
  firstName: "Max",
  lastName: "Mustermann",
  fullName: "Max Mustermann",
  hasAccount: true,
  accountEmail: "max@test.de",
  hasRfidCard: true,
  isStaff: true,
  isStudent: false,
  schoolId: "10",
  schoolName: "Test School",
  organizationId: "1",
  organizationName: "Test Org",
  createdAt: "2026-01-01T00:00:00Z",
};

type SWROverrides = Partial<Omit<Parameters<typeof setupSWR>[0], "useSWRMock">>;
function withDefaultSWR(overrides: SWROverrides = {}) {
  setupSWR({
    useSWRMock: mockUseSWR,
    mutateOrgs: mockMutateOrgs,
    mutateSchools: mockMutateSchools,
    mutateSchoolPersons: mockMutatePersons,
    ...overrides,
  });
}

describe("OperatorPersonsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParamsGet.mockImplementation(() => null);
    mockUseSession.mockReturnValue({
      data: { user: { id: "1" } },
      status: "authenticated",
    });
    mockMutateOrgs.mockResolvedValue(undefined);
    mockMutateSchools.mockResolvedValue([mockSchool]);
    mockMutatePersons.mockResolvedValue(undefined);
  });

  it("shows empty state when no school selected", () => {
    withDefaultSWR();

    render(<OperatorPersonsPage />);

    expect(screen.getByText("Keine Schule ausgewählt")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Wählen Sie eine Schule aus, um deren Personen anzuzeigen.",
      ),
    ).toBeInTheDocument();
  });

  it("shows person cards when school has persons", () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({ schoolPersons: [mockPerson] });

    render(<OperatorPersonsPage />);

    expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
  });

  it("shows role badges for staff person", () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({ schoolPersons: [mockPerson] });

    render(<OperatorPersonsPage />);

    expect(screen.getByText("Mitarbeiter")).toBeInTheDocument();
    expect(screen.getByText("RFID")).toBeInTheDocument();
    expect(screen.queryByText("Kinder")).not.toBeInTheDocument();
  });

  it("shows Kinder badge for student person", () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    const studentPerson = {
      ...mockPerson,
      id: "301",
      isStaff: false,
      isStudent: true,
      hasRfidCard: false,
      hasAccount: false,
      accountEmail: null,
    };
    withDefaultSWR({ schoolPersons: [studentPerson] });

    render(<OperatorPersonsPage />);

    expect(screen.getByText("Kinder")).toBeInTheDocument();
    expect(screen.queryByText("Mitarbeiter")).not.toBeInTheDocument();
    expect(screen.queryByText("RFID")).not.toBeInTheDocument();
  });

  it("shows account email when available", () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({ schoolPersons: [mockPerson] });

    render(<OperatorPersonsPage />);

    expect(screen.getByText("max@test.de")).toBeInTheDocument();
  });

  it("does not show email when accountEmail is null", () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({
      schoolPersons: [{ ...mockPerson, accountEmail: null }],
    });

    render(<OperatorPersonsPage />);

    expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
    expect(screen.queryByText("max@test.de")).not.toBeInTheDocument();
  });

  it("shows empty state for school with no persons", () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({ schoolPersons: [] });

    render(<OperatorPersonsPage />);

    expect(screen.getByText("Keine Personen")).toBeInTheDocument();
    expect(
      screen.getByText("Keine Personen in Test School vorhanden."),
    ).toBeInTheDocument();
  });

  it("shows persons loading state", () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({ schoolPersons: undefined, personsLoading: true });

    render(<OperatorPersonsPage />);

    expect(screen.getAllByTestId("skeleton").length).toBeGreaterThan(0);
  });

  it("shows person count with singular form", () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({ schoolPersons: [mockPerson] });

    render(<OperatorPersonsPage />);

    expect(screen.getByText(/1 Person in/)).toBeInTheDocument();
  });

  it("shows person count with plural form", () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({
      schoolPersons: [
        mockPerson,
        { ...mockPerson, id: "301", firstName: "Anna" },
      ],
    });

    render(<OperatorPersonsPage />);

    expect(screen.getByText(/2 Personen in/)).toBeInTheDocument();
  });

  it("shows school name in person count text", () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({ schoolPersons: [mockPerson] });

    render(<OperatorPersonsPage />);

    expect(screen.getByText("Test School")).toBeInTheDocument();
  });

  // --- Triple-confirm delete ---

  it("opens delete modal when trash button is clicked", async () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({ schoolPersons: [mockPerson] });

    render(<OperatorPersonsPage />);

    fireEvent.click(screen.getByTitle("Person löschen"));

    await waitFor(() => {
      expect(screen.getByText("Person löschen")).toBeInTheDocument();
    });
  });

  it("shows person name and school name in delete modal", async () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({ schoolPersons: [mockPerson] });

    render(<OperatorPersonsPage />);

    fireEvent.click(screen.getByTitle("Person löschen"));

    await waitFor(() => {
      expect(screen.getByText("Person löschen")).toBeInTheDocument();
    });

    expect(screen.getByText(/Möchten Sie/)).toBeInTheDocument();
    expect(screen.getByText(/wirklich löschen/)).toBeInTheDocument();
  });

  it("shows warning actions list in delete modal", async () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({ schoolPersons: [mockPerson] });

    render(<OperatorPersonsPage />);

    fireEvent.click(screen.getByTitle("Person löschen"));

    await waitFor(() => {
      expect(
        screen.getByText("Account wird deaktiviert und Login gesperrt"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Persönliche Daten werden anonymisiert"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("RFID-Karte wird freigegeben"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Diese Aktion kann nicht rückgängig gemacht werden"),
      ).toBeInTheDocument();
    });
  });

  it("disables delete button when input does not match name", async () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({ schoolPersons: [mockPerson] });

    render(<OperatorPersonsPage />);

    fireEvent.click(screen.getByTitle("Person löschen"));

    await waitFor(() => {
      expect(screen.getByText("Person löschen")).toBeInTheDocument();
    });

    const deleteButton = screen.getByText("Endgültig löschen");
    expect(deleteButton).toBeDisabled();

    fireEvent.change(screen.getByPlaceholderText("Max Mustermann"), {
      target: { value: "Wrong Name" },
    });

    expect(deleteButton).toBeDisabled();
  });

  it("enables delete button when input matches name exactly", async () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({ schoolPersons: [mockPerson] });

    render(<OperatorPersonsPage />);

    fireEvent.click(screen.getByTitle("Person löschen"));

    await waitFor(() => {
      expect(screen.getByText("Person löschen")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByPlaceholderText("Max Mustermann"), {
      target: { value: "Max Mustermann" },
    });

    expect(screen.getByText("Endgültig löschen")).not.toBeDisabled();
  });

  it("shows loading state during deletion", async () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    mockSoftDeletePerson.mockReturnValue(new Promise(() => {}));
    withDefaultSWR({ schoolPersons: [mockPerson] });

    render(<OperatorPersonsPage />);

    fireEvent.click(screen.getByTitle("Person löschen"));

    await waitFor(() => {
      expect(screen.getByText("Person löschen")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByPlaceholderText("Max Mustermann"), {
      target: { value: "Max Mustermann" },
    });

    fireEvent.click(screen.getByText("Endgültig löschen"));

    await waitFor(() => {
      expect(screen.getByText("Wird gelöscht…")).toBeInTheDocument();
    });
  });

  it("shows error message on API failure", async () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    mockSoftDeletePerson.mockRejectedValue(
      new Error("Person hat aktive Besuche"),
    );
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    withDefaultSWR({ schoolPersons: [mockPerson] });

    render(<OperatorPersonsPage />);

    fireEvent.click(screen.getByTitle("Person löschen"));

    await waitFor(() => {
      expect(screen.getByText("Person löschen")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByPlaceholderText("Max Mustermann"), {
      target: { value: "Max Mustermann" },
    });

    fireEvent.click(screen.getByText("Endgültig löschen"));

    await waitFor(() => {
      expect(screen.getByText("Person hat aktive Besuche")).toBeInTheDocument();
      expect(consoleError).toHaveBeenCalledWith(
        "person_soft_delete_failed",
        expect.objectContaining({
          error: "Person hat aktive Besuche",
        }),
      );
    });

    consoleError.mockRestore();
  });

  it("closes delete modal on cancel", async () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({ schoolPersons: [mockPerson] });

    render(<OperatorPersonsPage />);

    fireEvent.click(screen.getByTitle("Person löschen"));

    await waitFor(() => {
      expect(screen.getByText("Person löschen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Abbrechen"));

    await waitFor(() => {
      expect(screen.queryByText("Endgültig löschen")).not.toBeInTheDocument();
    });
  });

  it("calls softDeletePerson on successful confirmation", async () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    mockSoftDeletePerson.mockResolvedValue(undefined);
    withDefaultSWR({ schoolPersons: [mockPerson] });

    render(<OperatorPersonsPage />);

    fireEvent.click(screen.getByTitle("Person löschen"));

    await waitFor(() => {
      expect(screen.getByText("Person löschen")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByPlaceholderText("Max Mustermann"), {
      target: { value: "Max Mustermann" },
    });

    fireEvent.click(screen.getByText("Endgültig löschen"));

    await waitFor(() => {
      expect(mockSoftDeletePerson).toHaveBeenCalledWith("300");
    });
  });

  it("resets confirm input when opening modal for different person", async () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    const person2 = {
      ...mockPerson,
      id: "301",
      firstName: "Anna",
      lastName: "Schmidt",
      fullName: "Anna Schmidt",
    };
    withDefaultSWR({ schoolPersons: [mockPerson, person2] });

    render(<OperatorPersonsPage />);

    const deleteButtons = screen.getAllByTitle("Person löschen");
    fireEvent.click(deleteButtons[0]!);

    await waitFor(() => {
      expect(screen.getByPlaceholderText("Max Mustermann")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByPlaceholderText("Max Mustermann"), {
      target: { value: "Max" },
    });

    fireEvent.click(screen.getByText("Abbrechen"));

    await waitFor(() => {
      expect(screen.queryByText("Endgültig löschen")).not.toBeInTheDocument();
    });

    fireEvent.click(deleteButtons[1]!);

    await waitFor(() => {
      expect(screen.getByPlaceholderText("Anna Schmidt")).toBeInTheDocument();
      expect(
        (screen.getByPlaceholderText("Anna Schmidt") as HTMLInputElement).value,
      ).toBe("");
    });
  });
});
