/**
 * Tests for Operator Geräte Page (Devices management).
 *
 * Ported from provisioning/page.test.tsx (Devices Tab). Device filtering
 * moved from client state to URL query params (schoolId, orgId).
 */
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("~/components/operator/transfer-device-modal", () => ({
  TransferDeviceModal: () => null,
}));

const {
  mockUseSession,
  mockUseSWR,
  mockMutateOrgs,
  mockMutateSchools,
  mockListOrganizations,
  mockListSchools,
  mockListSchoolDevices,
  mockListOrganizationDevices,
  mockListAllDevices,
  mockDeleteDevice,
  mockSearchParamsGet,
  mockReplace,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUseSWR: vi.fn(),
  mockMutateOrgs: vi.fn(),
  mockMutateSchools: vi.fn(),
  mockListOrganizations: vi.fn(),
  mockListSchools: vi.fn(),
  mockListSchoolDevices: vi.fn(),
  mockListOrganizationDevices: vi.fn(),
  mockListAllDevices: vi.fn(),
  mockDeleteDevice: vi.fn(),
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
      listSchoolDevices: mockListSchoolDevices,
      listOrganizationDevices: mockListOrganizationDevices,
      listAllDevices: mockListAllDevices,
      deleteDevice: mockDeleteDevice,
    },
  };
});

vi.mock("~/lib/iot-helpers", () => ({
  getDeviceTypeDisplayName: (type: string) => `type:${type}`,
  getDeviceStatusDisplayName: (status: string) => `status:${status}`,
  formatLastSeen: (lastSeen: string) => `formatted:${lastSeen}`,
  DEVICE_TYPE_OPTIONS: { terminal: "Terminal", info_point: "Info-Point" },
}));

vi.mock("~/lib/format-utils", () => ({
  getRelativeTime: (dateStr: string) => `relative(${dateStr})`,
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

import OperatorDevicesPage from "./page";
import {
  mockSchool,
  setupSWR,
} from "~/test/helpers/operator-provisioning/provisioning-test-helpers";

// Matches the old provisioning/page.test.tsx Devices Tab fixture.
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

type SWROverrides = Partial<Omit<Parameters<typeof setupSWR>[0], "useSWRMock">>;
function withDefaultSWR(overrides: SWROverrides = {}) {
  setupSWR({
    useSWRMock: mockUseSWR,
    mutateOrgs: mockMutateOrgs,
    mutateSchools: mockMutateSchools,
    ...overrides,
  });
}

describe("OperatorDevicesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParamsGet.mockImplementation(() => null);
    mockUseSession.mockReturnValue({
      data: { user: { id: "1" } },
      status: "authenticated",
    });
    mockMutateOrgs.mockResolvedValue(undefined);
    mockMutateSchools.mockResolvedValue([mockSchool]);
  });

  it("renders devices tab with all devices view", () => {
    withDefaultSWR({ allDevices: [mockDevice] });

    render(<OperatorDevicesPage />);

    expect(screen.getByText("DEV-001")).toBeInTheDocument();
    expect(screen.getByText("Eingang")).toBeInTheDocument();
    expect(screen.getByText("type:terminal")).toBeInTheDocument();
    expect(screen.getByText("Online")).toBeInTheDocument();
    expect(screen.getByText("****-key")).toBeInTheDocument();
  });

  it("shows empty state for all devices", () => {
    withDefaultSWR({ allDevices: [] });

    render(<OperatorDevicesPage />);

    expect(screen.getByText("Keine Geräte")).toBeInTheDocument();
    expect(
      screen.getByText("Es gibt noch keine registrierten Geräte im System."),
    ).toBeInTheDocument();
  });

  it("filters devices by organization", () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "orgId" ? "1" : null,
    );
    withDefaultSWR({ orgDevices: [mockDevice] });

    render(<OperatorDevicesPage />);

    expect(screen.getByText("DEV-001")).toBeInTheDocument();
  });

  it("shows empty state for org-level devices", () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "orgId" ? "1" : null,
    );
    withDefaultSWR({ orgDevices: [] });

    render(<OperatorDevicesPage />);

    expect(screen.getByText("Keine Geräte")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Für diesen Träger gibt es noch keine registrierten Geräte.",
      ),
    ).toBeInTheDocument();
  });

  it("filters devices by school", () => {
    mockSearchParamsGet.mockImplementation((key: string) => {
      if (key === "schoolId") return "10";
      if (key === "orgId") return "1";
      return null;
    });
    withDefaultSWR({ schoolDevices: [mockDevice] });

    render(<OperatorDevicesPage />);

    expect(screen.getByText("DEV-001")).toBeInTheDocument();
    expect(screen.getByText("Test School")).toBeInTheDocument();
  });

  it("shows school-level device count badge", () => {
    mockSearchParamsGet.mockImplementation((key: string) => {
      if (key === "schoolId") return "10";
      if (key === "orgId") return "1";
      return null;
    });
    withDefaultSWR({ schoolDevices: [mockDevice] });

    render(<OperatorDevicesPage />);

    expect(screen.getByText("1 Gerät")).toBeInTheDocument();
  });

  it("shows plural Geräte for multiple devices", () => {
    mockSearchParamsGet.mockImplementation((key: string) => {
      if (key === "schoolId") return "10";
      if (key === "orgId") return "1";
      return null;
    });
    const device2 = { ...mockDevice, id: "201", deviceId: "DEV-002" };
    withDefaultSWR({ schoolDevices: [mockDevice, device2] });

    render(<OperatorDevicesPage />);

    expect(screen.getByText("2 Geräte")).toBeInTheDocument();
  });

  it("shows empty state for school-level devices", () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "schoolId" ? "10" : null,
    );
    withDefaultSWR({ schoolDevices: [] });

    render(<OperatorDevicesPage />);

    expect(screen.getByText("Keine Geräte")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Für diese Schule gibt es noch keine registrierten Geräte.",
      ),
    ).toBeInTheDocument();
  });

  // --- Status badges ---

  it("renders offline device with status badge", () => {
    const offlineDevice = { ...mockDevice, isOnline: false, status: "offline" };
    withDefaultSWR({ allDevices: [offlineDevice] });

    render(<OperatorDevicesPage />);

    expect(screen.getByText("status:offline")).toBeInTheDocument();
  });

  it("renders maintenance status badge", () => {
    const maintenanceDevice = {
      ...mockDevice,
      isOnline: false,
      status: "maintenance",
    };
    withDefaultSWR({ allDevices: [maintenanceDevice] });

    render(<OperatorDevicesPage />);

    expect(screen.getByText("status:maintenance")).toBeInTheDocument();
  });

  // --- Dash fallbacks ---

  it("renders device with no lastSeen as Nie", () => {
    const neverSeenDevice = {
      ...mockDevice,
      lastSeen: null,
      isOnline: false,
      status: "inactive",
    };
    withDefaultSWR({ allDevices: [neverSeenDevice] });

    render(<OperatorDevicesPage />);

    expect(screen.getByText("Nie")).toBeInTheDocument();
  });

  it("renders device with no name as dash", () => {
    const noNameDevice = { ...mockDevice, name: "" };
    withDefaultSWR({ allDevices: [noNameDevice] });

    render(<OperatorDevicesPage />);

    const cells = screen.getAllByText("—");
    expect(cells.length).toBeGreaterThan(0);
  });

  it("renders device with no maskedApiKey as dash", () => {
    const noKeyDevice = { ...mockDevice, maskedApiKey: "", apiKey: "" };
    withDefaultSWR({ allDevices: [noKeyDevice] });

    render(<OperatorDevicesPage />);

    const cells = screen.getAllByText("—");
    expect(cells.length).toBeGreaterThan(0);
  });

  // --- Clipboard ---

  it("copies API key to clipboard", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      writable: true,
      configurable: true,
    });

    withDefaultSWR({ allDevices: [mockDevice] });

    render(<OperatorDevicesPage />);

    fireEvent.click(screen.getByTitle("API-Key kopieren"));

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

    withDefaultSWR({ allDevices: [mockDevice] });

    render(<OperatorDevicesPage />);

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

  // --- Loading + sorting ---

  it("shows devices loading state", () => {
    withDefaultSWR({ devicesLoading: true });

    render(<OperatorDevicesPage />);

    expect(screen.getAllByTestId("skeleton").length).toBeGreaterThan(0);
  });

  it("sorts devices by clicking column header", () => {
    const device1 = { ...mockDevice, deviceId: "AAA-001" };
    const device2 = { ...mockDevice, id: "201", deviceId: "ZZZ-999" };
    withDefaultSWR({ allDevices: [device2, device1] });

    render(<OperatorDevicesPage />);

    fireEvent.click(screen.getByText("Geräte-ID"));

    const rows = screen.getAllByText(/[AZ]{3}-\d{3}/);
    expect(rows[0]!.textContent).toBe("AAA-001");
    expect(rows[1]!.textContent).toBe("ZZZ-999");

    fireEvent.click(screen.getByText("Geräte-ID"));

    const rowsReversed = screen.getAllByText(/[AZ]{3}-\d{3}/);
    expect(rowsReversed[0]!.textContent).toBe("ZZZ-999");
    expect(rowsReversed[1]!.textContent).toBe("AAA-001");
  });

  it("sorts devices by api key column", () => {
    const device1 = { ...mockDevice, id: "201", maskedApiKey: "aaaa-key" };
    const device2 = { ...mockDevice, id: "202", maskedApiKey: "zzzz-key" };
    withDefaultSWR({ allDevices: [device2, device1] });

    render(<OperatorDevicesPage />);

    fireEvent.click(screen.getByText("API-Key"));

    const apiKeyButtons = screen.getAllByTitle("API-Key kopieren");
    expect(apiKeyButtons[0]).toHaveTextContent("aaaa-key");
    expect(apiKeyButtons[1]).toHaveTextContent("zzzz-key");
  });

  it("sorts devices by name column", () => {
    const device1 = { ...mockDevice, id: "201", name: "Alpha" };
    const device2 = { ...mockDevice, id: "202", name: "Zulu" };
    withDefaultSWR({ allDevices: [device2, device1] });

    render(<OperatorDevicesPage />);

    fireEvent.click(screen.getByText("Name"));

    const names = screen.getAllByText(/Alpha|Zulu/);
    expect(names[0]!.textContent).toBe("Alpha");
    expect(names[1]!.textContent).toBe("Zulu");
  });

  it("sorts devices by type, status, and last seen", () => {
    const olderDevice = {
      ...mockDevice,
      id: "201",
      deviceType: "info_point",
      status: "offline",
      lastSeen: "2025-01-01T00:00:00Z",
    };
    const newerDevice = {
      ...mockDevice,
      id: "202",
      deviceId: "DEV-999",
      deviceType: "terminal",
      status: "maintenance",
      lastSeen: "2025-02-01T00:00:00Z",
    };
    withDefaultSWR({ allDevices: [newerDevice, olderDevice] });

    render(<OperatorDevicesPage />);

    fireEvent.click(screen.getByText("Typ"));
    let rows = screen.getAllByText(/DEV-001|DEV-999/);
    expect(rows[0]!.textContent).toBe("DEV-001");
    expect(rows[1]!.textContent).toBe("DEV-999");

    fireEvent.click(screen.getByText("Status"));
    rows = screen.getAllByText(/DEV-001|DEV-999/);
    expect(rows[0]!.textContent).toBe("DEV-999");
    expect(rows[1]!.textContent).toBe("DEV-001");

    fireEvent.click(screen.getByText("Zuletzt online"));
    rows = screen.getAllByText(/DEV-001|DEV-999/);
    expect(rows[0]!.textContent).toBe("DEV-001");
    expect(rows[1]!.textContent).toBe("DEV-999");
  });

  // --- Delete ---

  it("opens the delete device confirmation dialog", async () => {
    withDefaultSWR({ allDevices: [mockDevice] });

    render(<OperatorDevicesPage />);

    fireEvent.click(screen.getByRole("button", { name: /Aktionen für/ }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));

    await waitFor(() => {
      expect(screen.getByText("Gerät löschen")).toBeInTheDocument();
      expect(screen.getByText(/wirklich löschen\?/)).toBeInTheDocument();
    });
  });

  // --- Columns and headers ---

  it("shows school column in all-devices view", () => {
    withDefaultSWR({ allDevices: [mockDevice] });

    render(<OperatorDevicesPage />);

    const schuleElements = screen.getAllByText("Schule");
    expect(schuleElements.length).toBeGreaterThanOrEqual(2);
  });

  it("shows organization name in school-level header", () => {
    mockSearchParamsGet.mockImplementation((key: string) => {
      if (key === "schoolId") return "10";
      if (key === "orgId") return "1";
      return null;
    });
    withDefaultSWR({ schoolDevices: [mockDevice] });

    render(<OperatorDevicesPage />);

    const orgNameElements = screen.getAllByText("Test Org");
    expect(orgNameElements.length).toBeGreaterThanOrEqual(2);
  });

  // --- Neues Gerät button + action column ---

  it("shows create device button in devices tab", () => {
    withDefaultSWR({ allDevices: [mockDevice] });

    render(<OperatorDevicesPage />);

    expect(screen.getByText("Neues Gerät")).toBeInTheDocument();
  });

  it("shows device actions for each device", () => {
    withDefaultSWR({ allDevices: [mockDevice] });

    render(<OperatorDevicesPage />);

    fireEvent.click(screen.getByRole("button", { name: /Aktionen für/ }));

    expect(
      screen.getByRole("menuitem", { name: "Schule wechseln" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "API-Key ändern" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Löschen" }),
    ).toBeInTheDocument();
  });

  it("opens the set api key modal from the devices table", async () => {
    withDefaultSWR({ allDevices: [mockDevice] });

    render(<OperatorDevicesPage />);

    fireEvent.click(screen.getByRole("button", { name: /Aktionen für/ }));
    fireEvent.click(screen.getByRole("menuitem", { name: "API-Key ändern" }));

    await waitFor(() => {
      expect(screen.getByText("API-Key ändern")).toBeInTheDocument();
    });
  });

  it("opens the create-device modal when the desktop 'Neues Gerät' button is clicked", async () => {
    withDefaultSWR({ allDevices: [mockDevice] });

    render(<OperatorDevicesPage />);

    // The desktop button has visible text "Neues Gerät"
    const button = screen.getAllByText("Neues Gerät")[0];
    expect(button).toBeDefined();
    if (button) fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });
});
