/**
 * Tests for Operator Announcements Page
 * Tests the rendering, CRUD operations, and modal interactions for announcements
 */
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Hoisted mocks
const {
  mockUseSession,
  mockUseSWR,
  mockMutate,
  mockFetchAll,
  mockCreate,
  mockUpdate,
  mockDelete,
  mockPublish,
  mockFetchStats,
  mockListOrganizations,
  mockListSchools,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUseSWR: vi.fn(),
  mockMutate: vi.fn(),
  mockFetchAll: vi.fn(),
  mockCreate: vi.fn(),
  mockUpdate: vi.fn(),
  mockDelete: vi.fn(),
  mockPublish: vi.fn(),
  mockFetchStats: vi.fn(),
  mockListOrganizations: vi.fn(),
  mockListSchools: vi.fn(),
}));

// Mock hooks and contexts
vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
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

// Mock provisioning API
vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    listOrganizations: mockListOrganizations,
    listSchools: mockListSchools,
  },
}));

// Mock UI components

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
vi.mock("lucide-react", () => ({
  MoreVertical: () => <span>MoreVertical</span>,
  Pencil: () => <span>Pencil</span>,
  Trash2: () => <span>Trash2</span>,
  Send: () => <span>Send</span>,
  Check: () => <span>Check</span>,
}));

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
    mockUseSession.mockReturnValue({
      data: { user: { scope: "platform" } },
      status: "authenticated",
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

  it("renders expired announcement with Lesebestätigungen and expiration date", () => {
    const expiredAnnouncement = {
      ...mockAnnouncement,
      status: "expired" as const,
      publishedAt: new Date("2025-01-02"),
      expiresAt: new Date("2025-02-01").toISOString(),
    };

    mockUseSWR.mockImplementation((key: any) => {
      if (key === "operator-announcements") {
        return {
          data: [expiredAnnouncement],
          isLoading: false,
          mutate: mockMutate,
        };
      }
      if (typeof key === "string" && key.startsWith("announcement-stats-")) {
        return {
          data: { seen_count: 3, dismissed_count: 1 },
          isLoading: false,
          mutate: mockMutate,
        };
      }
      return { data: undefined, isLoading: false, mutate: mockMutate };
    });

    render(<OperatorAnnouncementsPage />);

    expect(screen.getByText(/Veröffentlicht vor/)).toBeInTheDocument();
    expect(screen.getByText(/Abgelaufen vor/)).toBeInTheDocument();
    expect(screen.getByTestId("views-accordion")).toBeInTheDocument();
  });

  it("shows future expiration date for published announcement", () => {
    const futureDate = new Date();
    futureDate.setFullYear(futureDate.getFullYear() + 1);

    const publishedWithExpiry = {
      ...mockAnnouncement,
      status: "published" as const,
      publishedAt: new Date("2025-01-02"),
      expiresAt: futureDate.toISOString(),
    };

    mockUseSWR.mockReturnValue({
      data: [publishedWithExpiry],
      isLoading: false,
      mutate: mockMutate,
    });

    render(<OperatorAnnouncementsPage />);

    expect(screen.getByText(/Läuft ab am/)).toBeInTheDocument();
  });

  it("does not show expiration date when expiresAt is null", () => {
    const publishedNoExpiry = {
      ...mockAnnouncement,
      status: "published" as const,
      publishedAt: new Date("2025-01-02"),
      expiresAt: null,
    };

    mockUseSWR.mockReturnValue({
      data: [publishedNoExpiry],
      isLoading: false,
      mutate: mockMutate,
    });

    render(<OperatorAnnouncementsPage />);

    expect(screen.getByText(/Veröffentlicht vor/)).toBeInTheDocument();
    expect(screen.queryByText(/Abgelaufen vor/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Läuft ab am/)).not.toBeInTheDocument();
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
      return originalMockUseSWR?.(key, fetcher, options);
    });

    render(<OperatorAnnouncementsPage />);

    expect(screen.getByTestId("views-accordion")).toBeInTheDocument();
  });

  describe("expand/collapse announcement content", () => {
    it("does not show expand button when content fits within clamp", () => {
      render(<OperatorAnnouncementsPage />);

      expect(screen.queryByText("Mehr anzeigen")).not.toBeInTheDocument();
      expect(screen.queryByText("Weniger anzeigen")).not.toBeInTheDocument();
    });

    it("shows 'Mehr anzeigen' button when content overflows", () => {
      const originalScrollHeight = Object.getOwnPropertyDescriptor(
        HTMLElement.prototype,
        "scrollHeight",
      );
      const originalClientHeight = Object.getOwnPropertyDescriptor(
        HTMLElement.prototype,
        "clientHeight",
      );
      Object.defineProperty(HTMLElement.prototype, "scrollHeight", {
        configurable: true,
        get() {
          return 100;
        },
      });
      Object.defineProperty(HTMLElement.prototype, "clientHeight", {
        configurable: true,
        get() {
          return 40;
        },
      });

      render(<OperatorAnnouncementsPage />);

      expect(screen.getByText("Mehr anzeigen")).toBeInTheDocument();

      if (originalScrollHeight) {
        Object.defineProperty(
          HTMLElement.prototype,
          "scrollHeight",
          originalScrollHeight,
        );
      }
      if (originalClientHeight) {
        Object.defineProperty(
          HTMLElement.prototype,
          "clientHeight",
          originalClientHeight,
        );
      }
    });

    it("toggles between 'Mehr anzeigen' and 'Weniger anzeigen'", () => {
      const originalScrollHeight = Object.getOwnPropertyDescriptor(
        HTMLElement.prototype,
        "scrollHeight",
      );
      const originalClientHeight = Object.getOwnPropertyDescriptor(
        HTMLElement.prototype,
        "clientHeight",
      );
      Object.defineProperty(HTMLElement.prototype, "scrollHeight", {
        configurable: true,
        get() {
          return 100;
        },
      });
      Object.defineProperty(HTMLElement.prototype, "clientHeight", {
        configurable: true,
        get() {
          return 40;
        },
      });

      render(<OperatorAnnouncementsPage />);

      const expandButton = screen.getByText("Mehr anzeigen");
      fireEvent.click(expandButton);
      expect(screen.getByText("Weniger anzeigen")).toBeInTheDocument();
      expect(screen.queryByText("Mehr anzeigen")).not.toBeInTheDocument();

      fireEvent.click(screen.getByText("Weniger anzeigen"));
      expect(screen.getByText("Mehr anzeigen")).toBeInTheDocument();

      if (originalScrollHeight) {
        Object.defineProperty(
          HTMLElement.prototype,
          "scrollHeight",
          originalScrollHeight,
        );
      }
      if (originalClientHeight) {
        Object.defineProperty(
          HTMLElement.prototype,
          "clientHeight",
          originalClientHeight,
        );
      }
    });

    it("removes line-clamp-2 class when expanded", () => {
      const originalScrollHeight = Object.getOwnPropertyDescriptor(
        HTMLElement.prototype,
        "scrollHeight",
      );
      const originalClientHeight = Object.getOwnPropertyDescriptor(
        HTMLElement.prototype,
        "clientHeight",
      );
      Object.defineProperty(HTMLElement.prototype, "scrollHeight", {
        configurable: true,
        get() {
          return 100;
        },
      });
      Object.defineProperty(HTMLElement.prototype, "clientHeight", {
        configurable: true,
        get() {
          return 40;
        },
      });

      render(<OperatorAnnouncementsPage />);

      const content = screen.getByText("Test content");
      expect(content).toHaveClass("line-clamp-2");

      fireEvent.click(screen.getByText("Mehr anzeigen"));
      expect(content).not.toHaveClass("line-clamp-2");

      if (originalScrollHeight) {
        Object.defineProperty(
          HTMLElement.prototype,
          "scrollHeight",
          originalScrollHeight,
        );
      }
      if (originalClientHeight) {
        Object.defineProperty(
          HTMLElement.prototype,
          "clientHeight",
          originalClientHeight,
        );
      }
    });
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

  describe("org/tenant targeting", () => {
    const mockOrganizations = [
      { id: "1", name: "Org Alpha", slug: "org-alpha" },
      { id: "2", name: "Org Beta", slug: "org-beta" },
    ];

    const mockSchools = [
      { id: "10", name: "Schule A", organizationId: "1", slug: "schule-a" },
      { id: "20", name: "Schule B", organizationId: "1", slug: "schule-b" },
      { id: "30", name: "Schule C", organizationId: "2", slug: "schule-c" },
    ];

    function setupSWRWithOrgAndSchools(
      announcementsData: any[] = [mockAnnouncement],
    ) {
      mockUseSWR.mockImplementation((key: any) => {
        if (key === "operator-announcements") {
          return {
            data: announcementsData,
            isLoading: false,
            mutate: mockMutate,
          };
        }
        if (key === "operator-organizations") {
          return {
            data: mockOrganizations,
            isLoading: false,
            mutate: mockMutate,
          };
        }
        if (key === "operator-schools") {
          return {
            data: mockSchools,
            isLoading: false,
            mutate: mockMutate,
          };
        }
        return { data: undefined, isLoading: false, mutate: mockMutate };
      });
    }

    it("renders org checkboxes from organizations data", async () => {
      setupSWRWithOrgAndSchools();
      render(<OperatorAnnouncementsPage />);

      fireEvent.click(screen.getByText("Neue Ankündigung"));

      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });

      expect(screen.getByText("Org Alpha")).toBeInTheDocument();
      expect(screen.getByText("Org Beta")).toBeInTheDocument();
    });

    it("renders school checkboxes from schools data", async () => {
      setupSWRWithOrgAndSchools();
      render(<OperatorAnnouncementsPage />);

      fireEvent.click(screen.getByText("Neue Ankündigung"));

      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });

      expect(screen.getByText("Schule A")).toBeInTheDocument();
      expect(screen.getByText("Schule B")).toBeInTheDocument();
      expect(screen.getByText("Schule C")).toBeInTheDocument();
    });

    it("filters schools by selected org", async () => {
      setupSWRWithOrgAndSchools();
      render(<OperatorAnnouncementsPage />);

      fireEvent.click(screen.getByText("Neue Ankündigung"));

      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });

      // Select Org Beta (id=2) — only Schule C belongs to it
      fireEvent.click(screen.getByText("Org Beta"));

      await waitFor(() => {
        expect(screen.getByText("Schule C")).toBeInTheDocument();
        expect(screen.queryByText("Schule A")).not.toBeInTheDocument();
        expect(screen.queryByText("Schule B")).not.toBeInTheDocument();
      });
    });

    it("shows all schools when no org is selected", async () => {
      setupSWRWithOrgAndSchools();
      render(<OperatorAnnouncementsPage />);

      fireEvent.click(screen.getByText("Neue Ankündigung"));

      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });

      // No org selected — all schools visible
      expect(screen.getByText("Schule A")).toBeInTheDocument();
      expect(screen.getByText("Schule B")).toBeInTheDocument();
      expect(screen.getByText("Schule C")).toBeInTheDocument();
    });

    it("creates announcement with org targeting", async () => {
      setupSWRWithOrgAndSchools();
      mockCreate.mockResolvedValue({ ...mockAnnouncement });

      render(<OperatorAnnouncementsPage />);

      fireEvent.click(screen.getByText("Neue Ankündigung"));

      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });

      // Fill required fields
      const textInputs = screen.getAllByRole("textbox");
      fireEvent.change(textInputs[0]!, { target: { value: "Org Targeted" } });
      fireEvent.change(textInputs[1]!, {
        target: { value: "Content for orgs" },
      });

      // Select Org Alpha
      fireEvent.click(screen.getByText("Org Alpha"));

      // Submit
      fireEvent.click(screen.getByText("Erstellen"));

      await waitFor(() => {
        expect(mockCreate).toHaveBeenCalledWith(
          expect.objectContaining({
            target_org_ids: [1],
          }),
        );
      });
    });

    it("creates announcement with tenant targeting", async () => {
      setupSWRWithOrgAndSchools();
      mockCreate.mockResolvedValue({ ...mockAnnouncement });

      render(<OperatorAnnouncementsPage />);

      fireEvent.click(screen.getByText("Neue Ankündigung"));

      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });

      // Fill required fields
      const textInputs = screen.getAllByRole("textbox");
      fireEvent.change(textInputs[0]!, {
        target: { value: "Tenant Targeted" },
      });
      fireEvent.change(textInputs[1]!, {
        target: { value: "Content for schools" },
      });

      // Select Schule A and Schule C
      fireEvent.click(screen.getByText("Schule A"));
      fireEvent.click(screen.getByText("Schule C"));

      // Submit
      fireEvent.click(screen.getByText("Erstellen"));

      await waitFor(() => {
        expect(mockCreate).toHaveBeenCalledWith(
          expect.objectContaining({
            target_tenant_ids: [10, 30],
          }),
        );
      });
    });

    it("updates announcement with org and tenant targeting", async () => {
      setupSWRWithOrgAndSchools();
      mockUpdate.mockResolvedValue({ ...mockAnnouncement });

      render(<OperatorAnnouncementsPage />);

      // Open edit modal via kebab menu
      const menuButtons = screen.getAllByLabelText("Menü öffnen");
      if (menuButtons[0]) {
        fireEvent.click(menuButtons[0]);
      }

      await waitFor(() => {
        fireEvent.click(screen.getByText("Bearbeiten"));
      });

      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });

      // Select org and school targeting
      fireEvent.click(screen.getByText("Org Alpha"));
      fireEvent.click(screen.getByText("Schule A"));

      // Submit
      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(mockUpdate).toHaveBeenCalledWith(
          "1",
          expect.objectContaining({
            target_org_ids: [1],
            target_tenant_ids: [10],
          }),
        );
      });
    });

    it("displays targeted org names on announcement card", () => {
      const announcementWithOrgs = {
        ...mockAnnouncement,
        targetOrgIds: [1, 2],
        targetTenantIds: [],
      };

      setupSWRWithOrgAndSchools([announcementWithOrgs]);
      render(<OperatorAnnouncementsPage />);

      expect(screen.getByText("Org Alpha, Org Beta")).toBeInTheDocument();
    });

    it("displays targeted school names on announcement card", () => {
      const announcementWithTenants = {
        ...mockAnnouncement,
        targetOrgIds: [],
        targetTenantIds: [10, 30],
      };

      setupSWRWithOrgAndSchools([announcementWithTenants]);
      render(<OperatorAnnouncementsPage />);

      expect(screen.getByText("Schule A, Schule C")).toBeInTheDocument();
    });

    it("shows empty state when no schools available for selected orgs", async () => {
      // Set up with schools that belong to neither of the orgs we will select
      mockUseSWR.mockImplementation((key: any) => {
        if (key === "operator-announcements") {
          return {
            data: [mockAnnouncement],
            isLoading: false,
            mutate: mockMutate,
          };
        }
        if (key === "operator-organizations") {
          return {
            data: [{ id: "99", name: "Empty Org", slug: "empty-org" }],
            isLoading: false,
            mutate: mockMutate,
          };
        }
        if (key === "operator-schools") {
          // No schools belong to org 99
          return {
            data: mockSchools,
            isLoading: false,
            mutate: mockMutate,
          };
        }
        return { data: undefined, isLoading: false, mutate: mockMutate };
      });

      render(<OperatorAnnouncementsPage />);

      fireEvent.click(screen.getByText("Neue Ankündigung"));

      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });

      // Select org 99 which has no schools
      fireEvent.click(screen.getByText("Empty Org"));

      await waitFor(() => {
        expect(screen.getByText("Keine Schulen verfügbar")).toBeInTheDocument();
      });
    });

    it("removes tenant selections when org is unchecked", async () => {
      setupSWRWithOrgAndSchools();
      mockCreate.mockResolvedValue({ ...mockAnnouncement });

      render(<OperatorAnnouncementsPage />);

      fireEvent.click(screen.getByText("Neue Ankündigung"));

      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });

      // Fill required fields
      const textInputs = screen.getAllByRole("textbox");
      fireEvent.change(textInputs[0]!, {
        target: { value: "Test removal" },
      });
      fireEvent.change(textInputs[1]!, {
        target: { value: "Content" },
      });

      // Select Org Alpha, then select its schools
      fireEvent.click(screen.getByText("Org Alpha"));

      await waitFor(() => {
        expect(screen.getByText("Schule A")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Schule A"));
      fireEvent.click(screen.getByText("Schule B"));

      // Now uncheck Org Alpha — should remove Schule A and B from targetTenantIds
      fireEvent.click(screen.getByText("Org Alpha"));

      // Submit to verify tenant IDs were cleared
      // First re-fill title since all schools are visible again but none selected
      fireEvent.click(screen.getByText("Erstellen"));

      await waitFor(() => {
        expect(mockCreate).toHaveBeenCalledWith(
          expect.objectContaining({
            target_org_ids: [],
            target_tenant_ids: [],
          }),
        );
      });
    });

    it("populates targeting fields when editing existing announcement", async () => {
      const announcementWithTargeting = {
        ...mockAnnouncement,
        targetOrgIds: [1],
        targetTenantIds: [10],
      };

      setupSWRWithOrgAndSchools([announcementWithTargeting]);
      render(<OperatorAnnouncementsPage />);

      // Open edit modal
      const menuButtons = screen.getAllByLabelText("Menü öffnen");
      if (menuButtons[0]) {
        fireEvent.click(menuButtons[0]);
      }

      await waitFor(() => {
        fireEvent.click(screen.getByText("Bearbeiten"));
      });

      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });

      // Org Alpha should be checked (green border)
      const modal = screen.getByTestId("modal");
      const orgAlphaBtn = Array.from(modal.querySelectorAll("button")).find(
        (b) => b.textContent?.includes("Org Alpha"),
      );
      expect(orgAlphaBtn?.className).toContain("border-[#83CD2D]");

      // Schule A should be checked
      const schuleABtn = Array.from(modal.querySelectorAll("button")).find(
        (b) => b.textContent?.includes("Schule A"),
      );
      expect(schuleABtn?.className).toContain("border-[#83CD2D]");
    });

    it("handles announcement with undefined targetOrgIds gracefully", () => {
      const announcementNoTargeting = {
        ...mockAnnouncement,
        targetOrgIds: undefined,
        targetTenantIds: undefined,
      };

      setupSWRWithOrgAndSchools([announcementNoTargeting]);

      // Should render without throwing
      render(<OperatorAnnouncementsPage />);

      expect(screen.getByText("Test Announcement")).toBeInTheDocument();
      // No org/school targeting rows should be displayed
      expect(screen.queryByText("Org Alpha, Org Beta")).not.toBeInTheDocument();
    });

    it("publish confirmation shows global message when no targeting set", async () => {
      const globalDraft = {
        ...mockAnnouncement,
        title: "Global Draft",
        targetOrgIds: [],
        targetTenantIds: [],
      };

      setupSWRWithOrgAndSchools([globalDraft]);
      render(<OperatorAnnouncementsPage />);

      fireEvent.click(screen.getByText("Veröffentlichen"));

      await waitFor(() => {
        expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
      });

      expect(screen.getByText(/für alle Nutzer sichtbar/)).toBeInTheDocument();
    });

    it("publish confirmation shows targeted message when org targeting set", async () => {
      const orgTargetedDraft = {
        ...mockAnnouncement,
        title: "Org Targeted Draft",
        targetOrgIds: [1],
        targetTenantIds: [],
      };

      setupSWRWithOrgAndSchools([orgTargetedDraft]);
      render(<OperatorAnnouncementsPage />);

      fireEvent.click(screen.getByText("Veröffentlichen"));

      await waitFor(() => {
        expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
      });

      expect(
        screen.getByText(/ausgewählten Organisationen\/Schulen/),
      ).toBeInTheDocument();
    });

    it("publish confirmation shows targeted message when tenant targeting set", async () => {
      const tenantTargetedDraft = {
        ...mockAnnouncement,
        title: "Tenant Targeted Draft",
        targetOrgIds: [],
        targetTenantIds: [10],
      };

      setupSWRWithOrgAndSchools([tenantTargetedDraft]);
      render(<OperatorAnnouncementsPage />);

      fireEvent.click(screen.getByText("Veröffentlichen"));

      await waitFor(() => {
        expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
      });

      expect(
        screen.getByText(/ausgewählten Organisationen\/Schulen/),
      ).toBeInTheDocument();
    });

    it("publish confirmation shows global message when targeting arrays undefined", async () => {
      const undefinedTargetDraft = {
        ...mockAnnouncement,
        title: "Undefined Target Draft",
        targetOrgIds: undefined,
        targetTenantIds: undefined,
      };

      setupSWRWithOrgAndSchools([undefinedTargetDraft]);
      render(<OperatorAnnouncementsPage />);

      fireEvent.click(screen.getByText("Veröffentlichen"));

      await waitFor(() => {
        expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
      });

      expect(screen.getByText(/für alle Nutzer sichtbar/)).toBeInTheDocument();
    });

    it("preserves tenant selections when schools data is undefined during org deselection", async () => {
      // Setup SWR where schools returns undefined
      mockUseSWR.mockImplementation((key: any) => {
        if (key === "operator-announcements") {
          return {
            data: [mockAnnouncement],
            isLoading: false,
            mutate: mockMutate,
          };
        }
        if (key === "operator-organizations") {
          return {
            data: [{ id: "1", name: "Org Alpha", slug: "org-alpha" }],
            isLoading: false,
            mutate: mockMutate,
          };
        }
        if (key === "operator-schools") {
          return {
            data: undefined,
            isLoading: true,
            mutate: mockMutate,
          };
        }
        return { data: undefined, isLoading: false, mutate: mockMutate };
      });

      render(<OperatorAnnouncementsPage />);

      // Open create form
      fireEvent.click(screen.getByText("Neue Ankündigung"));

      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });

      // Select org (should work even without schools loaded)
      const orgButton = screen.getByText("Org Alpha");
      fireEvent.click(orgButton);

      // Deselect org — tenant selections should be preserved (not errored)
      fireEvent.click(orgButton);

      // The form should still be functional (no crash)
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });
});
