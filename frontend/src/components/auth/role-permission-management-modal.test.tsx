/**
 * Tests for RolePermissionManagementModal Component
 * Tests the rendering and basic functionality of role permission management
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RolePermissionManagementModal } from "./role-permission-management-modal";
import type { Role, Permission } from "~/lib/auth-helpers";

// Mock functions using vi.hoisted to avoid hoisting issues
const {
  mockToastSuccess,
  mockGetPermissions,
  mockGetRolePermissions,
  mockAssignPermission,
  mockRemovePermission,
} = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockGetPermissions: vi.fn((): Promise<unknown[]> => Promise.resolve([])),
  mockGetRolePermissions: vi.fn((): Promise<unknown[]> => Promise.resolve([])),
  mockAssignPermission: vi.fn(() => Promise.resolve()),
  mockRemovePermission: vi.fn(() => Promise.resolve()),
}));

// Mock ToastContext
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
  }),
}));

// Mock auth-service
vi.mock("~/lib/auth-service", () => ({
  authService: {
    getPermissions: mockGetPermissions,
    getRolePermissions: mockGetRolePermissions,
    assignPermissionToRole: mockAssignPermission,
    removePermissionFromRole: mockRemovePermission,
  },
}));

// Mock auth-helpers
vi.mock("~/lib/auth-helpers", () => ({
  getRoleDisplayName: (name: string) => name,
}));

// Mock permission-labels
vi.mock("~/lib/permission-labels", () => ({
  localizeAction: (action: string) => action,
  localizeResource: (resource: string) => resource,
  formatPermissionDisplay: (resource: string, action: string) =>
    `${resource}:${action}`,
}));

// Mock UI components
// SlideOver laeuft ueber Vaul; in jsdom ersetzt dieser Mock die Bibliothek.
vi.mock("vaul", async () => {
  const React = await import("react");

  return {
    Drawer: {
      Root: ({
        children,
        open,
      }: {
        children: React.ReactNode;
        open?: boolean;
      }) => (open === false ? null : <div>{children}</div>),
      Portal: ({ children }: { children: React.ReactNode }) => <>{children}</>,
      Overlay: React.forwardRef<
        HTMLDivElement,
        React.HTMLAttributes<HTMLDivElement>
      >((props, ref) => <div ref={ref} {...props} />),
      Content: React.forwardRef<
        HTMLDivElement,
        React.HTMLAttributes<HTMLDivElement>
      >((props, ref) => <div ref={ref} {...props} />),
      Close: React.forwardRef<
        HTMLButtonElement,
        React.ButtonHTMLAttributes<HTMLButtonElement>
      >((props, ref) => <button ref={ref} {...props} />),
      Title: React.forwardRef<
        HTMLHeadingElement,
        React.HTMLAttributes<HTMLHeadingElement>
      >(({ children, ...props }, ref) => (
        <h2 ref={ref} {...props}>
          {children ?? "Titel"}
        </h2>
      )),
      Description: React.forwardRef<
        HTMLParagraphElement,
        React.HTMLAttributes<HTMLParagraphElement>
      >((props, ref) => <p ref={ref} {...props} />),
    },
  };
});

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ type, message }: { type: string; message: string }) =>
    message ? <div data-testid={`alert-${type}`}>{message}</div> : null,
}));

describe("RolePermissionManagementModal", () => {
  const mockRole: Role = {
    id: "1",
    name: "teacher",
    description: "Teacher role",
    isSystem: false,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };

  const mockPermissions: Permission[] = [
    {
      id: "1",
      name: "Read Students",
      description: "Can read students",
      resource: "students",
      action: "read",
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
    {
      id: "2",
      name: "Write Students",
      description: "Can write students",
      resource: "students",
      action: "write",
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
    {
      id: "3",
      name: "Read Rooms",
      description: "Can read rooms",
      resource: "rooms",
      action: "read",
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
    {
      id: "4",
      name: "Write Rooms",
      description: "Can write rooms",
      resource: "rooms",
      action: "write",
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
  ];

  const mockOnClose = vi.fn();
  const mockOnUpdate = vi.fn();

  const renderModal = (overrides?: Partial<{ isOpen: boolean }>) =>
    render(
      <RolePermissionManagementModal
        isOpen={overrides?.isOpen ?? true}
        onClose={mockOnClose}
        role={mockRole}
        onUpdate={mockOnUpdate}
      />,
    );

  /** Wait for permissions to load (loading spinner disappears). */
  const waitForLoad = () =>
    waitFor(() => {
      expect(screen.queryByText(/Laden.../i)).not.toBeInTheDocument();
    });

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetPermissions.mockResolvedValue(mockPermissions);
    mockGetRolePermissions.mockResolvedValue([]);
    mockAssignPermission.mockResolvedValue(undefined);
    mockRemovePermission.mockResolvedValue(undefined);
  });

  // =========================================================================
  // Rendering
  // =========================================================================

  it("renders the modal when open", async () => {
    renderModal();
    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: /Berechtigungen verwalten/ }),
      ).toBeInTheDocument();
    });
  });

  it("does not render when closed", () => {
    renderModal({ isOpen: false });
    expect(
      screen.queryByRole("heading", { name: /Berechtigungen verwalten/ }),
    ).not.toBeInTheDocument();
  });

  it("displays the role name in title", async () => {
    renderModal();
    await waitFor(() => {
      expect(
        screen.getByText(/Berechtigungen verwalten - teacher/i),
      ).toBeInTheDocument();
    });
  });

  it("displays loading state initially", () => {
    renderModal();
    expect(screen.getByText(/Laden.../i)).toBeInTheDocument();
  });

  it("loads permissions when opened", async () => {
    renderModal();
    await waitFor(() => {
      expect(mockGetPermissions).toHaveBeenCalled();
      expect(mockGetRolePermissions).toHaveBeenCalledWith(mockRole.id);
    });
  });

  it("displays search input", async () => {
    renderModal();
    await waitForLoad();
    expect(
      screen.getByPlaceholderText(/Berechtigungen suchen.../i),
    ).toBeInTheDocument();
  });

  it("renders save and cancel buttons", async () => {
    renderModal();
    await waitForLoad();
    expect(
      screen.getByRole("button", { name: /Abbrechen/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Speichern/i }),
    ).toBeInTheDocument();
  });

  it("displays permissions grouped by resource after loading", async () => {
    renderModal();
    await waitForLoad();
    expect(screen.getByText("students:read")).toBeInTheDocument();
    expect(screen.getByText("students:write")).toBeInTheDocument();
    expect(screen.getByText("rooms:read")).toBeInTheDocument();
    expect(screen.getByText("rooms:write")).toBeInTheDocument();
  });

  it("handles empty permissions list", async () => {
    mockGetPermissions.mockResolvedValue([]);
    renderModal();
    await waitForLoad();
    expect(
      screen.getByText(/Keine Berechtigungen gefunden/i),
    ).toBeInTheDocument();
  });

  // =========================================================================
  // Stats
  // =========================================================================

  it("displays total assigned count and total permissions", async () => {
    mockGetRolePermissions.mockResolvedValue([
      mockPermissions[0],
      mockPermissions[2],
    ]);
    renderModal();
    await waitForLoad();
    expect(screen.getByText("2 / 4")).toBeInTheDocument();
  });

  it("keeps total stats accurate when search filters results", async () => {
    mockGetRolePermissions.mockResolvedValue([
      mockPermissions[0],
      mockPermissions[2],
    ]);
    renderModal();
    await waitForLoad();

    // Search for "rooms" — only 2 of 4 visible, but stats should still show 2/4
    fireEvent.change(screen.getByPlaceholderText(/suchen/i), {
      target: { value: "rooms" },
    });
    expect(screen.getByText("2 / 4")).toBeInTheDocument();
  });

  // =========================================================================
  // Resource group headers
  // =========================================================================

  it("shows resource group headers", async () => {
    renderModal();
    await waitForLoad();
    // localizeResource is mocked to identity, so raw resource names appear
    expect(screen.getByText("students")).toBeInTheDocument();
    expect(screen.getByText("rooms")).toBeInTheDocument();
  });

  it("shows group assigned counts", async () => {
    mockGetRolePermissions.mockResolvedValue([mockPermissions[0]]);
    renderModal();
    await waitForLoad();
    // students group: 1 assigned / 2 total
    expect(screen.getByText("1/2")).toBeInTheDocument();
    // rooms group: 0 / 2
    expect(screen.getByText("0/2")).toBeInTheDocument();
  });

  // =========================================================================
  // Toggle individual permission
  // =========================================================================

  it("toggles a permission on when clicking its switch", async () => {
    renderModal();
    await waitForLoad();

    const switches = screen.getAllByRole("switch");
    expect(switches[0]).toHaveAttribute("aria-checked", "false");

    fireEvent.click(switches[0]!);
    expect(switches[0]).toHaveAttribute("aria-checked", "true");
  });

  it("toggles a permission off when clicking an assigned switch", async () => {
    // Assign rooms:read (id "3") — rooms group sorts first
    mockGetRolePermissions.mockResolvedValue([mockPermissions[2]]);
    renderModal();
    await waitForLoad();

    const switches = screen.getAllByRole("switch");
    // First switch is rooms:read which is assigned
    expect(switches[0]).toHaveAttribute("aria-checked", "true");

    fireEvent.click(switches[0]!);
    expect(switches[0]).toHaveAttribute("aria-checked", "false");
  });

  // =========================================================================
  // Select All / Deselect All
  // =========================================================================

  it("selects all permissions with 'Alle auswählen'", async () => {
    renderModal();
    await waitForLoad();

    fireEvent.click(screen.getByText("Alle auswählen"));

    const switches = screen.getAllByRole("switch");
    for (const s of switches) {
      expect(s).toHaveAttribute("aria-checked", "true");
    }
  });

  it("deselects all permissions with 'Alle abwählen'", async () => {
    mockGetRolePermissions.mockResolvedValue([...mockPermissions]);
    renderModal();
    await waitForLoad();

    // All are assigned → button says "Alle abwählen"
    fireEvent.click(screen.getByText("Alle abwählen"));

    const switches = screen.getAllByRole("switch");
    for (const s of switches) {
      expect(s).toHaveAttribute("aria-checked", "false");
    }
  });

  it("shows 'Alle auswählen' when not all are checked", async () => {
    mockGetRolePermissions.mockResolvedValue([mockPermissions[0]]);
    renderModal();
    await waitForLoad();
    expect(screen.getByText("Alle auswählen")).toBeInTheDocument();
  });

  it("shows 'Alle abwählen' when all are checked", async () => {
    mockGetRolePermissions.mockResolvedValue([...mockPermissions]);
    renderModal();
    await waitForLoad();
    expect(screen.getByText("Alle abwählen")).toBeInTheDocument();
  });

  // =========================================================================
  // Per-resource group toggle
  // =========================================================================

  it("toggles all permissions in a resource group via group checkbox", async () => {
    renderModal();
    await waitForLoad();

    // Find group checkbox for "students" (first group header checkbox)
    const groupCheckboxes = screen.getAllByTitle(/Alle.*Berechtigungen/);
    fireEvent.click(groupCheckboxes[0]!); // rooms group (sorted alphabetically)

    // Check that rooms permissions are toggled on
    const switches = screen.getAllByRole("switch");
    // With mock localizeResource=identity, sort is alphabetical: rooms first, then students
    expect(switches[0]).toHaveAttribute("aria-checked", "true"); // rooms:read
    expect(switches[1]).toHaveAttribute("aria-checked", "true"); // rooms:write
  });

  // =========================================================================
  // Collapse / Expand groups
  // =========================================================================

  it("collapses a group when clicking the group header", async () => {
    renderModal();
    await waitForLoad();

    // All 4 permission rows visible
    expect(screen.getAllByRole("switch")).toHaveLength(4);

    // Click first group header to collapse — groups are sorted: rooms, students
    const groupHeaders = screen.getAllByText(/^(rooms|students)$/);
    fireEvent.click(groupHeaders[0]!); // collapse "rooms"

    // Only students group visible (2 switches)
    expect(screen.getAllByRole("switch")).toHaveLength(2);
  });

  it("expands a collapsed group when clicking again", async () => {
    renderModal();
    await waitForLoad();

    const groupHeaders = screen.getAllByText(/^(rooms|students)$/);
    fireEvent.click(groupHeaders[0]!); // collapse
    expect(screen.getAllByRole("switch")).toHaveLength(2);

    fireEvent.click(groupHeaders[0]!); // expand
    expect(screen.getAllByRole("switch")).toHaveLength(4);
  });

  // =========================================================================
  // Search / filter
  // =========================================================================

  it("filters permissions by search term", async () => {
    renderModal();
    await waitForLoad();

    fireEvent.change(screen.getByPlaceholderText(/suchen/i), {
      target: { value: "rooms" },
    });

    // Only rooms permissions visible
    expect(screen.getByText("rooms:read")).toBeInTheDocument();
    expect(screen.getByText("rooms:write")).toBeInTheDocument();
    expect(screen.queryByText("students:read")).not.toBeInTheDocument();
    expect(screen.queryByText("students:write")).not.toBeInTheDocument();
  });

  it("shows no results message when search matches nothing", async () => {
    renderModal();
    await waitForLoad();

    fireEvent.change(screen.getByPlaceholderText(/suchen/i), {
      target: { value: "nonexistent" },
    });
    expect(
      screen.getByText(/Keine Berechtigungen gefunden/i),
    ).toBeInTheDocument();
  });

  it("disables 'Alle auswählen' when search yields no results", async () => {
    renderModal();
    await waitForLoad();

    fireEvent.change(screen.getByPlaceholderText(/suchen/i), {
      target: { value: "nonexistent" },
    });
    expect(screen.getByText("Alle auswählen")).toBeDisabled();
  });

  // =========================================================================
  // Save changes
  // =========================================================================

  it("saves assigned and removed permissions on save", async () => {
    // Start with rooms:read (id "3") assigned — rooms group sorts first
    mockGetRolePermissions.mockResolvedValue([mockPermissions[2]]);
    renderModal();
    await waitForLoad();

    const switches = screen.getAllByRole("switch");
    // switches[0] = rooms:read (assigned), switches[1] = rooms:write (not assigned)
    fireEvent.click(switches[0]!); // unassign rooms:read
    fireEvent.click(switches[1]!); // assign rooms:write

    fireEvent.click(screen.getByRole("button", { name: /Speichern/i }));

    await waitFor(() => {
      // rooms:write (id "4") assigned
      expect(mockAssignPermission).toHaveBeenCalledWith(mockRole.id, "4");
      // rooms:read (id "3") removed
      expect(mockRemovePermission).toHaveBeenCalledWith(mockRole.id, "3");
    });
  });

  it("calls onClose and onUpdate after successful save", async () => {
    renderModal();
    await waitForLoad();

    // Toggle a permission to enable save
    const switches = screen.getAllByRole("switch");
    fireEvent.click(switches[0]!);

    fireEvent.click(screen.getByRole("button", { name: /Speichern/i }));

    await waitFor(() => {
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Berechtigungen aktualisiert",
      );
      expect(mockOnUpdate).toHaveBeenCalled();
      expect(mockOnClose).toHaveBeenCalled();
    });
  });

  it("disables save button when no changes", async () => {
    renderModal();
    await waitForLoad();
    expect(screen.getByRole("button", { name: /Speichern/i })).toBeDisabled();
  });

  it("enables save button after toggling a permission", async () => {
    renderModal();
    await waitForLoad();

    const switches = screen.getAllByRole("switch");
    fireEvent.click(switches[0]!);

    expect(
      screen.getByRole("button", { name: /Speichern/i }),
    ).not.toBeDisabled();
  });

  // =========================================================================
  // Error handling
  // =========================================================================

  it("shows error alert when fetching permissions fails", async () => {
    mockGetPermissions.mockRejectedValue(new Error("Network error"));
    renderModal();

    await waitFor(() => {
      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
      expect(
        screen.getByText(/Fehler beim Laden der Berechtigungen/i),
      ).toBeInTheDocument();
    });
  });

  it("clears a stale fetch error after reopening and loading successfully", async () => {
    mockGetPermissions.mockRejectedValueOnce(new Error("Network error"));
    const { rerender } = renderModal();

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Berechtigungen/i),
      ).toBeInTheDocument();
    });

    rerender(
      <RolePermissionManagementModal
        isOpen={false}
        onClose={mockOnClose}
        role={mockRole}
        onUpdate={mockOnUpdate}
      />,
    );
    rerender(
      <RolePermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitForLoad();

    expect(screen.queryByTestId("alert-error")).not.toBeInTheDocument();
    expect(screen.getByText("students:read")).toBeInTheDocument();
  });

  it("shows error alert when saving fails", async () => {
    mockAssignPermission.mockRejectedValue(new Error("Save failed"));
    renderModal();
    await waitForLoad();

    // Toggle and save
    const switches = screen.getAllByRole("switch");
    fireEvent.click(switches[0]!);
    fireEvent.click(screen.getByRole("button", { name: /Speichern/i }));

    await waitFor(() => {
      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
      expect(
        screen.getByText(/Fehler beim Aktualisieren der Berechtigungen/i),
      ).toBeInTheDocument();
    });
  });

  it("clears a stale save error after retrying successfully", async () => {
    mockAssignPermission.mockRejectedValueOnce(new Error("Save failed"));
    renderModal();
    await waitForLoad();

    const switches = screen.getAllByRole("switch");
    fireEvent.click(switches[0]!);
    fireEvent.click(screen.getByRole("button", { name: /Speichern/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Aktualisieren der Berechtigungen/i),
      ).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /Speichern/i }));

    await waitFor(() => {
      expect(mockAssignPermission).toHaveBeenCalledTimes(2);
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Berechtigungen aktualisiert",
      );
    });
    expect(screen.queryByTestId("alert-error")).not.toBeInTheDocument();
  });

  // =========================================================================
  // Cancel
  // =========================================================================

  it("calls onClose when cancel is clicked", async () => {
    renderModal();
    await waitForLoad();

    fireEvent.click(screen.getByRole("button", { name: /Abbrechen/i }));
    expect(mockOnClose).toHaveBeenCalled();
  });
});
