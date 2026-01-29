/* eslint-disable @typescript-eslint/no-unsafe-return, @typescript-eslint/no-empty-function */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import DevicesPage from "./page";

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: {
      user: {
        id: "1",
        name: "Test User",
        email: "test@test.com",
        token: "tok",
        roles: ["admin"],
      },
    },
    status: "authenticated",
  })),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
}));

const mockGetList = vi.fn();
vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: () => ({
    getList: (...args: unknown[]) => mockGetList(...args),
    getOne: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
  }),
}));

vi.mock("@/lib/database/configs/devices.config", () => ({
  devicesConfig: {
    name: { singular: "Gerät", plural: "Geräte" },
    form: {},
  },
}));

vi.mock("~/components/database/database-page-layout", () => ({
  DatabasePageLayout: ({
    children,
    loading,
    sessionLoading,
  }: {
    children: React.ReactNode;
    loading: boolean;
    sessionLoading: boolean;
  }) =>
    loading || sessionLoading ? (
      <div data-testid="loading">Loading...</div>
    ) : (
      <div data-testid="database-layout">{children}</div>
    ),
}));

vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({ badge }: { badge?: { count: number } }) => (
    <div data-testid="page-header">
      {badge && <span data-testid="badge-count">{badge.count}</span>}
    </div>
  ),
}));

vi.mock("@/components/devices", () => ({
  DeviceCreateModal: () => <div data-testid="create-modal" />,
  DeviceDetailModal: () => <div data-testid="detail-modal" />,
  DeviceEditModal: () => <div data-testid="edit-modal" />,
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: () => <div data-testid="confirm-modal" />,
}));

vi.mock("@/lib/iot-helpers", () => ({
  getDeviceTypeDisplayName: (type: string) => type,
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: vi.fn(), error: vi.fn() }),
}));

vi.mock("~/hooks/useIsMobile", () => ({
  useIsMobile: () => false,
}));

vi.mock("~/hooks/useDeleteConfirmation", () => ({
  useDeleteConfirmation: () => ({
    showConfirmModal: false,
    handleDeleteClick: vi.fn(),
    handleDeleteCancel: vi.fn(),
    confirmDelete: vi.fn(),
  }),
}));

vi.mock("@/lib/use-notification", () => ({
  getDbOperationMessage: () => "Success",
}));

describe("DevicesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows loading state initially", () => {
    mockGetList.mockReturnValue(new Promise(() => {}));
    render(<DevicesPage />);
    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders devices after loading", async () => {
    mockGetList.mockResolvedValue({
      data: [
        {
          id: "1",
          device_id: "DEV-001",
          name: "Reader 1",
          device_type: "rfid_reader",
        },
      ],
    });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Reader 1")).toBeInTheDocument();
    });
  });

  it("shows empty state when no devices", async () => {
    mockGetList.mockResolvedValue({ data: [] });

    render(<DevicesPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Geräte vorhanden")).toBeInTheDocument();
    });
  });

  it("shows error when API fails", async () => {
    mockGetList.mockRejectedValue(new Error("API Error"));

    render(<DevicesPage />);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Fehler beim Laden der Geräte. Bitte versuchen Sie es später erneut.",
        ),
      ).toBeInTheDocument();
    });
  });
});
