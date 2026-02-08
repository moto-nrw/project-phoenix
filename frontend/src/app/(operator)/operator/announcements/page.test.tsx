/**
 * Tests for Operator Announcements Page
 * Tests the rendering, CRUD operations, and modal interactions for announcements
 */
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Hoisted mocks
const {
  mockUseOperatorAuth,
  mockUseSWR,
  mockMutate,
  mockFetchAll,
  mockCreate,
  mockUpdate,
  mockDelete,
  mockPublish,
  mockFetchStats,
} = vi.hoisted(() => ({
  mockUseOperatorAuth: vi.fn(),
  mockUseSWR: vi.fn(),
  mockMutate: vi.fn(),
  mockFetchAll: vi.fn(),
  mockCreate: vi.fn(),
  mockUpdate: vi.fn(),
  mockDelete: vi.fn(),
  mockPublish: vi.fn(),
  mockFetchStats: vi.fn(),
}));

// Mock hooks and contexts
vi.mock("~/lib/operator/auth-context", () => ({
  useOperatorAuth: mockUseOperatorAuth,
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("swr", () => ({
  default: mockUseSWR,
}));

// Mock announcements API
vi.mock("~/lib/operator/announcements-api", () => ({
  operatorAnnouncementsService: {
    fetchAll: mockFetchAll,
    create: mockCreate,
    update: mockUpdate,
    delete: mockDelete,
    publish: mockPublish,
    fetchStats: mockFetchStats,
  },
}));

// Mock UI components
/* eslint-disable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-return, @typescript-eslint/prefer-optional-chain */
vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({ title, badge, filters, actionButton }: any) => (
    <div data-testid="page-header">
      <h1>{title}</h1>
      {badge && <span data-testid="badge">{badge.count}</span>}
      {filters &&
        filters.map((f: any) => (
          <select
            key={f.id}
            data-testid={`filter-${f.id}`}
            value={f.value}
            onChange={(e) => f.onChange(e.target.value)}
          >
            {f.options.map((opt: any) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        ))}
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
  ConfirmationModal: ({ isOpen, children, title, onClose, onConfirm }: any) =>
    isOpen ? (
      <div data-testid="confirmation-modal">
        <h2>{title}</h2>
        {children}
        <button onClick={onClose}>Cancel</button>
        <button onClick={onConfirm} data-testid="confirm-button">
          Confirm
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/ui/skeleton", () => ({
  Skeleton: ({ className }: any) => (
    <div data-testid="skeleton" className={className} />
  ),
}));

vi.mock("~/components/ui/date-picker", () => ({
  DatePicker: ({ value, onChange }: any) => (
    <input
      data-testid="date-picker"
      type="date"
      value={value ? value.toISOString().split("T")[0] : ""}
      onChange={(e) =>
        onChange(e.target.value ? new Date(e.target.value) : null)
      }
    />
  ),
}));

vi.mock("~/components/operator/announcement-views-accordion", () => ({
  AnnouncementViewsAccordion: () => <div data-testid="views-accordion" />,
}));

vi.mock("framer-motion", () => ({
  AnimatePresence: ({ children }: any) => <div>{children}</div>,
  LayoutGroup: ({ children }: any) => <div>{children}</div>,
  motion: {
    div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
  },
}));
/* eslint-enable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-return, @typescript-eslint/prefer-optional-chain */ vi.mock(
  "lucide-react",
  () => ({
    MoreVertical: () => <span>MoreVertical</span>,
    Pencil: () => <span>Pencil</span>,
    Trash2: () => <span>Trash2</span>,
    Send: () => <span>Send</span>,
    Check: () => <span>Check</span>,
  }),
);

import OperatorAnnouncementsPage from "./page";

describe("OperatorAnnouncementsPage", () => {
  const mockAnnouncement = {
    id: "1",
    title: "Test Announcement",
    content: "Test content",
    type: "announcement" as const,
    severity: "info" as const,
    status: "draft" as const,
    version: "1.0.0",
    expiresAt: "2025-12-31",
    targetRoles: ["admin" as const],
    createdAt: new Date("2025-01-01"),
    publishedAt: null,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseOperatorAuth.mockReturnValue({
      isAuthenticated: true,
      operator: { id: "1", email: "test@example.com" },
    });
    mockUseSWR.mockReturnValue({
      data: [mockAnnouncement],
      isLoading: false,
      mutate: mockMutate,
    });
    mockMutate.mockResolvedValue(undefined);
  });

  it("renders loading state", () => {
    mockUseSWR.mockReturnValue({
      data: undefined,
      isLoading: true,
      mutate: mockMutate,
    });

    render(<OperatorAnnouncementsPage />);

    // 3 skeleton cards × 6 Skeleton components each = 18
    expect(screen.getAllByTestId("skeleton")).toHaveLength(18);
  });

  it("renders empty state when no announcements", () => {
    mockUseSWR.mockReturnValue({
      data: [],
      isLoading: false,
      mutate: mockMutate,
    });

    render(<OperatorAnnouncementsPage />);

    expect(screen.getByText("Keine Ankündigungen")).toBeInTheDocument();
    expect(screen.getByText("Neue Ankündigung")).toBeInTheDocument();
  });

  it("renders announcements list", () => {
    render(<OperatorAnnouncementsPage />);

    expect(screen.getByText("Test Announcement")).toBeInTheDocument();
    expect(screen.getByText("Test content")).toBeInTheDocument();
  });

  it("filters announcements by status", async () => {
    const publishedAnnouncement = {
      ...mockAnnouncement,
      id: "2",
      status: "published" as const,
      publishedAt: new Date("2025-01-02"),
    };

    mockUseSWR.mockReturnValue({
      data: [mockAnnouncement, publishedAnnouncement],
      isLoading: false,
      mutate: mockMutate,
    });

    render(<OperatorAnnouncementsPage />);

    const statusFilter = screen.getByTestId("filter-status");
    fireEvent.change(statusFilter, { target: { value: "draft" } });

    await waitFor(() => {
      expect(screen.getByText("Test Announcement")).toBeInTheDocument();
    });
  });

  it("opens create form modal", async () => {
    render(<OperatorAnnouncementsPage />);

    const createButton = screen.getByText("Neue Ankündigung");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });

  it("creates new announcement", async () => {
    mockCreate.mockResolvedValue({ ...mockAnnouncement });

    render(<OperatorAnnouncementsPage />);

    // Open create modal
    fireEvent.click(screen.getByText("Neue Ankündigung"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Fill form - use input type selectors since labels lack htmlFor
    const textInputs = screen.getAllByRole("textbox");
    const titleInput = textInputs[0]!; // First textbox is title
    const contentInput = textInputs[1]!; // Second is content (textarea)
    fireEvent.change(titleInput, { target: { value: "New Announcement" } });
    fireEvent.change(contentInput, { target: { value: "New content" } });

    // Submit - "Erstellen" button is in the footer
    const createButton = screen.getByText("Erstellen");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          title: "New Announcement",
          content: "New content",
        }),
      );
      expect(mockMutate).toHaveBeenCalled();
    });
  });

  it("opens edit form with announcement data", async () => {
    render(<OperatorAnnouncementsPage />);

    // Open menu
    const menuButtons = screen.getAllByLabelText("Menü öffnen");
    if (menuButtons[0]) {
      fireEvent.click(menuButtons[0]);
    }

    // Click edit
    await waitFor(() => {
      const editButton = screen.getByText("Bearbeiten");
      fireEvent.click(editButton);
    });

    await waitFor(() => {
      const textInputs = screen.getAllByRole("textbox");
      const titleInput = textInputs[0] as HTMLInputElement;
      expect(titleInput.value).toBe("Test Announcement");
    });
  });

  it("updates announcement", async () => {
    mockUpdate.mockResolvedValue({ ...mockAnnouncement });

    render(<OperatorAnnouncementsPage />);

    // Open edit modal
    const menuButtons = screen.getAllByLabelText("Menü öffnen");
    if (menuButtons[0]) {
      fireEvent.click(menuButtons[0]);
    }

    await waitFor(() => {
      fireEvent.click(screen.getByText("Bearbeiten"));
    });

    // Update form
    await waitFor(() => {
      const textInputs = screen.getAllByRole("textbox");
      const titleInput = textInputs[0]!;
      fireEvent.change(titleInput, { target: { value: "Updated Title" } });
    });

    // Submit
    const saveButton = screen.getByText("Speichern");
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith(
        "1",
        expect.objectContaining({
          title: "Updated Title",
        }),
      );
      expect(mockMutate).toHaveBeenCalled();
    });
  });

  it("deletes announcement after confirmation", async () => {
    mockDelete.mockResolvedValue(undefined);

    render(<OperatorAnnouncementsPage />);

    // Open menu and click delete
    const menuButtons = screen.getAllByLabelText("Menü öffnen");
    if (menuButtons[0]) {
      fireEvent.click(menuButtons[0]);
    }

    await waitFor(() => {
      fireEvent.click(screen.getByText("Löschen"));
    });

    // Confirm deletion
    await waitFor(() => {
      const confirmButton = screen.getByTestId("confirm-button");
      fireEvent.click(confirmButton);
    });

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalledWith("1");
      expect(mockMutate).toHaveBeenCalled();
    });
  });

  it("publishes draft announcement", async () => {
    mockPublish.mockResolvedValue(undefined);

    render(<OperatorAnnouncementsPage />);

    // Click publish button
    const publishButton = screen.getByText("Veröffentlichen");
    fireEvent.click(publishButton);

    // Confirm publish
    await waitFor(() => {
      const confirmButton = screen.getByTestId("confirm-button");
      fireEvent.click(confirmButton);
    });

    await waitFor(() => {
      expect(mockPublish).toHaveBeenCalledWith("1");
      expect(mockMutate).toHaveBeenCalled();
    });
  });

  it("changes announcement type", async () => {
    render(<OperatorAnnouncementsPage />);

    // Open create modal
    fireEvent.click(screen.getByText("Neue Ankündigung"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Find type buttons inside the modal
    const buttons = screen.getAllByRole("button");
    const releaseButton = buttons.find((b) => b.textContent === "Release");
    expect(releaseButton).toBeDefined();
  });

  it("shows version field for release type", async () => {
    render(<OperatorAnnouncementsPage />);

    // Open create modal
    fireEvent.click(screen.getByText("Neue Ankündigung"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Click "Release" type button
    const buttons = screen.getAllByRole("button");
    const releaseButton = buttons.find((b) => b.textContent === "Release");
    expect(releaseButton).toBeDefined();
    if (releaseButton) {
      fireEvent.click(releaseButton);
    }

    await waitFor(() => {
      expect(screen.getByPlaceholderText("z.B. 2.1.0")).toBeInTheDocument();
    });
  });

  it("toggles severity dropdown", async () => {
    render(<OperatorAnnouncementsPage />);

    // Open create modal
    fireEvent.click(screen.getByText("Neue Ankündigung"));

    await waitFor(() => {
      const severityButton = screen.getByText("Info");
      fireEvent.click(severityButton);
    });

    // Dropdown should be open
    await waitFor(() => {
      expect(screen.getByText("Warnung")).toBeInTheDocument();
    });
  });

  it("toggles target role selection", async () => {
    render(<OperatorAnnouncementsPage />);

    // Open create modal
    fireEvent.click(screen.getByText("Neue Ankündigung"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Find the Administratoren button inside the modal form
    const modal = screen.getByTestId("modal");
    const adminButtons = Array.from(modal.querySelectorAll("button")).filter(
      (b) => b.textContent?.includes("Administratoren"),
    );
    expect(adminButtons.length).toBeGreaterThan(0);
    fireEvent.click(adminButtons[0]!);

    // Role should be selected (button gets the selected class)
    await waitFor(() => {
      const updatedModal = screen.getByTestId("modal");
      const updatedAdminBtn = Array.from(
        updatedModal.querySelectorAll("button"),
      ).find((b) => b.textContent?.includes("Administratoren"));
      expect(updatedAdminBtn?.className).toContain("border-[#83CD2D]");
    });
  });

  it("handles form validation - empty title", async () => {
    render(<OperatorAnnouncementsPage />);

    // Open create modal
    fireEvent.click(screen.getByText("Neue Ankündigung"));

    await waitFor(() => {
      const createButton = screen.getByText("Erstellen");
      // Button should be disabled when title is empty
      expect(createButton).toBeDisabled();
    });
  });

  it("closes modal on cancel", async () => {
    render(<OperatorAnnouncementsPage />);

    // Open create modal
    fireEvent.click(screen.getByText("Neue Ankündigung"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Click cancel
    const cancelButton = screen.getByText("Abbrechen");
    if (cancelButton) {
      fireEvent.click(cancelButton);
    }

    await waitFor(() => {
      expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
    });
  });

  it("closes delete modal on cancel", async () => {
    render(<OperatorAnnouncementsPage />);

    // Open delete modal
    const menuButtons = screen.getAllByLabelText("Menü öffnen");
    if (menuButtons[0]) {
      fireEvent.click(menuButtons[0]);
    }

    await waitFor(() => {
      fireEvent.click(screen.getByText("Löschen"));
    });

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    // Cancel
    const cancelButton = screen.getByText("Cancel");
    if (cancelButton) {
      fireEvent.click(cancelButton);
    }

    await waitFor(() => {
      expect(
        screen.queryByTestId("confirmation-modal"),
      ).not.toBeInTheDocument();
    });
  });

  it("renders published announcement with timestamp", () => {
    const publishedAnnouncement = {
      ...mockAnnouncement,
      status: "published" as const,
      publishedAt: new Date("2025-01-02"),
    };

    mockUseSWR.mockReturnValue({
      data: [publishedAnnouncement],
      isLoading: false,
      mutate: mockMutate,
    });

    render(<OperatorAnnouncementsPage />);

    // Find the timestamp text (not the dropdown option)
    expect(screen.getByText(/Veröffentlicht vor/)).toBeInTheDocument();
  });

  it("renders views accordion for published announcements with stats", () => {
    const publishedAnnouncement = {
      ...mockAnnouncement,
      status: "published" as const,
      publishedAt: new Date("2025-01-02"),
    };

    mockUseSWR.mockReturnValue({
      data: [publishedAnnouncement],
      isLoading: false,
      mutate: mockMutate,
    });

    // Mock stats SWR call
    const originalMockUseSWR = mockUseSWR.getMockImplementation();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    mockUseSWR.mockImplementation((key: any, fetcher: any, options: any) => {
      if (key === "operator-announcements") {
        return {
          data: [publishedAnnouncement],
          isLoading: false,
          mutate: mockMutate,
        };
      }
      if (typeof key === "string" && key.startsWith("announcement-stats-")) {
        return {
          data: { seen_count: 5, dismissed_count: 2 },
          isLoading: false,
          mutate: mockMutate,
        };
      }
      // eslint-disable-next-line @typescript-eslint/no-unsafe-return
      return originalMockUseSWR?.(key, fetcher, options);
    });

    render(<OperatorAnnouncementsPage />);

    expect(screen.getByTestId("views-accordion")).toBeInTheDocument();
  });

  it("handles API errors gracefully", async () => {
    mockCreate.mockRejectedValue(new Error("API Error"));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      // noop - suppress console.error in test
    });

    render(<OperatorAnnouncementsPage />);

    // Open create modal
    fireEvent.click(screen.getByText("Neue Ankündigung"));

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Fill form using textbox role
    const textInputs = screen.getAllByRole("textbox");
    fireEvent.change(textInputs[0]!, { target: { value: "Test" } });
    fireEvent.change(textInputs[1]!, { target: { value: "Test" } });

    // Submit
    const createButton = screen.getByText("Erstellen");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith("announcement_save_failed", {
        error: "API Error",
      });
    });

    consoleError.mockRestore();
  });
});
