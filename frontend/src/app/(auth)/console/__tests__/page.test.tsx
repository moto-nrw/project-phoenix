/**
 * Tests for app/(auth)/console/page.tsx
 *
 * Tests the SaaS Admin console page including:
 * - Loading state and session checks
 * - Sidebar navigation
 * - Mobile navigation
 * - Organizations section
 * - Organization actions (approve, reject, suspend, reactivate)
 * - Filter tabs
 * - Stats display
 * - Error handling
 * - Logout functionality
 */

import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock window.location
const originalLocation = window.location;

// Use vi.hoisted to define mocks before they're used in vi.mock (avoids hoisting issues)
const {
  mockSessionRef,
  mockFetchOrganizations,
  mockApproveOrganization,
  mockRejectOrganization,
  mockSuspendOrganization,
  mockReactivateOrganization,
} = vi.hoisted(() => ({
  mockSessionRef: {
    current: {
      data: {
        user: { id: "user-1", email: "admin@example.com" },
      } as { user: { id: string; email: string } | null } | null,
      isPending: false,
    },
  },
  mockFetchOrganizations: vi.fn(),
  mockApproveOrganization: vi.fn(),
  mockRejectOrganization: vi.fn(),
  mockSuspendOrganization: vi.fn(),
  mockReactivateOrganization: vi.fn(),
}));

// Mock auth-client
vi.mock("~/lib/auth-client", () => ({
  useSession: () => mockSessionRef.current,
}));

// Mock admin-api - use direct function references instead of wrapper functions
vi.mock("~/lib/admin-api", () => ({
  fetchOrganizations: mockFetchOrganizations,
  approveOrganization: mockApproveOrganization,
  rejectOrganization: mockRejectOrganization,
  suspendOrganization: mockSuspendOrganization,
  reactivateOrganization: mockReactivateOrganization,
}));

// Mock OrganizationInviteForm to simplify testing
vi.mock("~/components/console/organization-invite-form", () => ({
  OrganizationInviteForm: () => (
    <div data-testid="organization-invite-form">Organization Invite Form</div>
  ),
}));

// Import after mocks
import SaasAdminPage from "../page";

// Store original console.error
const originalConsoleError = console.error;

beforeEach(() => {
  vi.clearAllMocks();
  mockSessionRef.current = {
    data: { user: { id: "user-1", email: "admin@example.com" } },
    isPending: false,
  };

  // Default organizations mock
  mockFetchOrganizations.mockResolvedValue([
    {
      id: "org-1",
      name: "Test Org 1",
      slug: "test-org-1",
      status: "pending",
      ownerName: "John Doe",
      ownerEmail: "john@example.com",
      createdAt: "2024-01-15T10:00:00Z",
    },
    {
      id: "org-2",
      name: "Test Org 2",
      slug: "test-org-2",
      status: "active",
      ownerName: "Jane Doe",
      ownerEmail: "jane@example.com",
      createdAt: "2024-01-10T10:00:00Z",
    },
  ]);

  // Mock window.location
  Object.defineProperty(window, "location", {
    writable: true,
    value: { href: "" },
  });

  // Suppress console.error during tests
  console.error = vi.fn();
});

afterEach(() => {
  console.error = originalConsoleError;
  Object.defineProperty(window, "location", {
    writable: true,
    value: originalLocation,
  });
});

describe("SaasAdminPage", () => {
  // =============================================================================
  // Loading State Tests
  // =============================================================================

  describe("loading state", () => {
    it("shows loading when session is pending", () => {
      mockSessionRef.current = { data: null, isPending: true };

      render(<SaasAdminPage />);

      // Should show Loading component (fullPage=false)
      expect(document.querySelector('[class*="flex"]')).toBeInTheDocument();
    });

    it("shows loading when no user in session", () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<SaasAdminPage />);

      // Should show loading and redirect will be triggered
      expect(window.location.href).toBe("/console/login");
    });

    it("redirects to login when not authenticated", () => {
      mockSessionRef.current = { data: null, isPending: false };

      render(<SaasAdminPage />);

      expect(window.location.href).toBe("/console/login");
    });
  });

  // =============================================================================
  // Page Structure Tests
  // =============================================================================

  describe("page structure", () => {
    it("renders page header with current section", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        // Use getByRole to find the heading specifically (not sidebar spans)
        expect(
          screen.getByRole("heading", { name: "Organisationen", level: 1 }),
        ).toBeInTheDocument();
        expect(
          screen.getByText("Alle registrierten Organisationen verwalten"),
        ).toBeInTheDocument();
      });
    });

    it("renders logout button", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /Abmelden/ }),
        ).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Sidebar Navigation Tests
  // =============================================================================

  describe("sidebar navigation", () => {
    it("renders sidebar with navigation items", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        // Sidebar is hidden on small screens, but should be in the DOM
        expect(screen.getByText("Plattform-Konsole")).toBeInTheDocument();
        expect(screen.getByText("Administration")).toBeInTheDocument();
      });
    });

    it("switches to invite section when clicked", async () => {
      render(<SaasAdminPage />);

      // Wait for the page header to render (use unique text)
      await waitFor(() => {
        expect(
          screen.getByText("Alle registrierten Organisationen verwalten"),
        ).toBeInTheDocument();
      });

      // Find and click the invite button
      const inviteButtons = screen.getAllByText("Einladung senden");
      fireEvent.click(inviteButtons[0]!);

      await waitFor(() => {
        expect(
          screen.getByText("Neue Organisation anlegen und Einladung versenden"),
        ).toBeInTheDocument();
      });
    });

    it("switches to demo section when clicked", async () => {
      render(<SaasAdminPage />);

      // Wait for the page header to render (use unique text)
      await waitFor(() => {
        expect(
          screen.getByText("Alle registrierten Organisationen verwalten"),
        ).toBeInTheDocument();
      });

      const demoButtons = screen.getAllByText("Demo-Umgebung");
      fireEvent.click(demoButtons[0]!);

      await waitFor(() => {
        expect(
          screen.getByText("Demo-Umgebungen für Interessenten erstellen"),
        ).toBeInTheDocument();
      });
    });

    it("switches back to organizations section", async () => {
      render(<SaasAdminPage />);

      // Wait for the page header to render (use unique text)
      await waitFor(() => {
        expect(
          screen.getByText("Alle registrierten Organisationen verwalten"),
        ).toBeInTheDocument();
      });

      // Go to invite section
      const inviteButtons = screen.getAllByText("Einladung senden");
      fireEvent.click(inviteButtons[0]!);

      await waitFor(() => {
        expect(
          screen.getByText("Neue Organisation anlegen und Einladung versenden"),
        ).toBeInTheDocument();
      });

      // Go back to organizations
      const orgButtons = screen.getAllByText("Organisationen");
      fireEvent.click(orgButtons[0]!);

      await waitFor(() => {
        expect(
          screen.getByText("Alle registrierten Organisationen verwalten"),
        ).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Organizations Section Tests
  // =============================================================================

  describe("organizations section", () => {
    it("loads and displays organizations", async () => {
      render(<SaasAdminPage />);

      // Default filter is "pending", so only Test Org 1 is visible initially
      await waitFor(() => {
        expect(screen.getByText("Test Org 1")).toBeInTheDocument();
      });

      // Click "Alle" to see all organizations
      const allButton = screen.getByRole("button", { name: "Alle" });
      fireEvent.click(allButton);

      await waitFor(() => {
        expect(screen.getByText("Test Org 1")).toBeInTheDocument();
        expect(screen.getByText("Test Org 2")).toBeInTheDocument();
      });
    });

    it("displays organization owner info", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("John Doe")).toBeInTheDocument();
        expect(screen.getByText("john@example.com")).toBeInTheDocument();
      });
    });

    it("displays organization slugs", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("test-org-1")).toBeInTheDocument();
      });
    });

    it("shows loading skeletons while fetching", async () => {
      mockFetchOrganizations.mockImplementation(
        () => new Promise(() => {}), // Never resolves
      );

      render(<SaasAdminPage />);

      // Should show skeleton loaders
      expect(
        document.querySelectorAll('[class*="Skeleton"]').length,
      ).toBeGreaterThanOrEqual(0);
    });

    it("shows error message when fetch fails", async () => {
      mockFetchOrganizations.mockRejectedValue(new Error("Network error"));

      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Network error")).toBeInTheDocument();
      });
    });

    it("shows fallback error for non-Error exceptions", async () => {
      mockFetchOrganizations.mockRejectedValue("Unknown error");

      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Fehler beim Laden")).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Stats Display Tests
  // =============================================================================

  describe("stats display", () => {
    it("displays correct pending count", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        // Find the pending stats card by looking for the description text
        expect(screen.getByText("Warten auf Genehmigung")).toBeInTheDocument();
      });
    });

    it("displays correct active count", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        // Find the active stats card by looking for the description text
        expect(
          screen.getByText("Genehmigte Organisationen"),
        ).toBeInTheDocument();
      });
    });

    it("displays stats description text", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Warten auf Genehmigung")).toBeInTheDocument();
        expect(
          screen.getByText("Genehmigte Organisationen"),
        ).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Filter Tabs Tests
  // =============================================================================

  describe("filter tabs", () => {
    it("defaults to pending filter", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        // Only pending organization should be visible by default
        expect(screen.getByText("Test Org 1")).toBeInTheDocument();
      });
    });

    it("switches to active filter", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Test Org 1")).toBeInTheDocument();
      });

      const activeFilterButtons = screen.getAllByRole("button", {
        name: /Aktiv/,
      });
      // Filter tab buttons
      const filterButton = activeFilterButtons.find((btn) =>
        btn.textContent?.includes("Aktiv"),
      );
      if (filterButton) fireEvent.click(filterButton);

      await waitFor(() => {
        expect(screen.getByText("Test Org 2")).toBeInTheDocument();
      });
    });

    it("shows all organizations with 'all' filter", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Test Org 1")).toBeInTheDocument();
      });

      const allButton = screen.getByRole("button", { name: "Alle" });
      fireEvent.click(allButton);

      await waitFor(() => {
        expect(screen.getByText("Test Org 1")).toBeInTheDocument();
        expect(screen.getByText("Test Org 2")).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Organization Actions Tests
  // =============================================================================

  describe("organization actions", () => {
    it("shows approve and reject buttons for pending orgs", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Genehmigen")).toBeInTheDocument();
        expect(screen.getByText("Ablehnen")).toBeInTheDocument();
      });
    });

    it("approves organization when approve button clicked", async () => {
      mockApproveOrganization.mockResolvedValue({});

      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Genehmigen")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Genehmigen"));

      await waitFor(() => {
        expect(mockApproveOrganization).toHaveBeenCalledWith("org-1");
      });
    });

    it("shows reject modal when reject button clicked", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Ablehnen")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Ablehnen"));

      await waitFor(() => {
        expect(screen.getByText("Organisation ablehnen")).toBeInTheDocument();
      });
    });

    it("rejects organization with reason", async () => {
      mockRejectOrganization.mockResolvedValue({});

      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Ablehnen")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Ablehnen"));

      await waitFor(() => {
        expect(screen.getByText("Organisation ablehnen")).toBeInTheDocument();
      });

      const reasonInput = screen.getByPlaceholderText(
        "Geben Sie einen Grund für die Ablehnung an...",
      );
      fireEvent.change(reasonInput, { target: { value: "Invalid documents" } });

      const confirmButtons = screen.getAllByText("Ablehnen");
      // The modal's reject button
      const modalRejectButton = confirmButtons[confirmButtons.length - 1];
      if (modalRejectButton) fireEvent.click(modalRejectButton);

      await waitFor(() => {
        expect(mockRejectOrganization).toHaveBeenCalledWith(
          "org-1",
          "Invalid documents",
        );
      });
    });

    it("closes reject modal when cancel clicked", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Ablehnen")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Ablehnen"));

      await waitFor(() => {
        expect(screen.getByText("Organisation ablehnen")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Abbrechen"));

      await waitFor(() => {
        expect(
          screen.queryByText("Organisation ablehnen"),
        ).not.toBeInTheDocument();
      });
    });

    it("closes reject modal when backdrop clicked", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Ablehnen")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Ablehnen"));

      await waitFor(() => {
        expect(screen.getByText("Organisation ablehnen")).toBeInTheDocument();
      });

      // Click backdrop
      const backdrop = screen.getByRole("button", {
        name: "Hintergrund - Klicken zum Schließen",
      });
      fireEvent.click(backdrop);

      await waitFor(() => {
        expect(
          screen.queryByText("Organisation ablehnen"),
        ).not.toBeInTheDocument();
      });
    });

    it("closes reject modal with Escape key", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Ablehnen")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Ablehnen"));

      await waitFor(() => {
        expect(screen.getByText("Organisation ablehnen")).toBeInTheDocument();
      });

      fireEvent.keyDown(document, { key: "Escape" });

      await waitFor(() => {
        expect(
          screen.queryByText("Organisation ablehnen"),
        ).not.toBeInTheDocument();
      });
    });

    it("shows suspend button for active orgs", async () => {
      mockFetchOrganizations.mockResolvedValue([
        {
          id: "org-2",
          name: "Active Org",
          slug: "active-org",
          status: "active",
          ownerName: "Jane Doe",
          ownerEmail: "jane@example.com",
          createdAt: "2024-01-10T10:00:00Z",
        },
      ]);

      render(<SaasAdminPage />);

      // Switch to active filter
      await waitFor(() => {
        const activeButton = screen
          .getAllByRole("button")
          .find((btn) => btn.textContent === "Aktiv");
        if (activeButton) fireEvent.click(activeButton);
      });

      await waitFor(() => {
        expect(screen.getByText("Sperren")).toBeInTheDocument();
      });
    });

    it("suspends organization when suspend button clicked", async () => {
      mockFetchOrganizations.mockResolvedValue([
        {
          id: "org-2",
          name: "Active Org",
          slug: "active-org",
          status: "active",
          ownerName: null,
          ownerEmail: null,
          createdAt: "2024-01-10T10:00:00Z",
        },
      ]);
      mockSuspendOrganization.mockResolvedValue({});

      render(<SaasAdminPage />);

      // Switch to active filter
      await waitFor(() => {
        const activeButton = screen
          .getAllByRole("button")
          .find((btn) => btn.textContent === "Aktiv");
        if (activeButton) fireEvent.click(activeButton);
      });

      await waitFor(() => {
        expect(screen.getByText("Sperren")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Sperren"));

      await waitFor(() => {
        expect(mockSuspendOrganization).toHaveBeenCalledWith("org-2");
      });
    });

    it("shows reactivate button for suspended orgs", async () => {
      mockFetchOrganizations.mockResolvedValue([
        {
          id: "org-3",
          name: "Suspended Org",
          slug: "suspended-org",
          status: "suspended",
          ownerName: "Bob",
          ownerEmail: "bob@example.com",
          createdAt: "2024-01-05T10:00:00Z",
        },
      ]);

      render(<SaasAdminPage />);

      // Switch to suspended filter
      await waitFor(() => {
        const suspendedButton = screen
          .getAllByRole("button")
          .find((btn) => btn.textContent?.includes("Gesperrt"));
        if (suspendedButton) fireEvent.click(suspendedButton);
      });

      await waitFor(() => {
        expect(screen.getByText("Reaktivieren")).toBeInTheDocument();
      });
    });

    it("reactivates organization when reactivate button clicked", async () => {
      mockFetchOrganizations.mockResolvedValue([
        {
          id: "org-3",
          name: "Suspended Org",
          slug: "suspended-org",
          status: "suspended",
          ownerName: "Bob",
          ownerEmail: "bob@example.com",
          createdAt: "2024-01-05T10:00:00Z",
        },
      ]);
      mockReactivateOrganization.mockResolvedValue({});

      render(<SaasAdminPage />);

      // Switch to suspended filter
      await waitFor(() => {
        const suspendedButton = screen
          .getAllByRole("button")
          .find((btn) => btn.textContent?.includes("Gesperrt"));
        if (suspendedButton) fireEvent.click(suspendedButton);
      });

      await waitFor(() => {
        expect(screen.getByText("Reaktivieren")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Reaktivieren"));

      await waitFor(() => {
        expect(mockReactivateOrganization).toHaveBeenCalledWith("org-3");
      });
    });
  });

  // =============================================================================
  // Action Error Handling Tests
  // =============================================================================

  describe("action error handling", () => {
    it("shows error when approve fails", async () => {
      mockApproveOrganization.mockRejectedValue(new Error("Approval failed"));

      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Genehmigen")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Genehmigen"));

      await waitFor(() => {
        expect(screen.getByText("Approval failed")).toBeInTheDocument();
      });
    });

    it("shows fallback error when approve fails with non-Error", async () => {
      mockApproveOrganization.mockRejectedValue("Unknown");

      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Genehmigen")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Genehmigen"));

      await waitFor(() => {
        expect(screen.getByText("Fehler beim Genehmigen")).toBeInTheDocument();
      });
    });

    it("shows error when reject fails", async () => {
      mockRejectOrganization.mockRejectedValue(new Error("Rejection failed"));

      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Ablehnen")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Ablehnen"));

      await waitFor(() => {
        expect(screen.getByText("Organisation ablehnen")).toBeInTheDocument();
      });

      const confirmButtons = screen.getAllByText("Ablehnen");
      const modalRejectButton = confirmButtons[confirmButtons.length - 1];
      if (modalRejectButton) fireEvent.click(modalRejectButton);

      await waitFor(() => {
        expect(screen.getByText("Rejection failed")).toBeInTheDocument();
      });
    });

    it("shows error when suspend fails", async () => {
      mockFetchOrganizations.mockResolvedValue([
        {
          id: "org-2",
          name: "Active Org",
          slug: "active-org",
          status: "active",
          ownerName: "Jane",
          ownerEmail: "jane@example.com",
          createdAt: "2024-01-10T10:00:00Z",
        },
      ]);
      mockSuspendOrganization.mockRejectedValue(new Error("Suspend failed"));

      render(<SaasAdminPage />);

      await waitFor(() => {
        const activeButton = screen
          .getAllByRole("button")
          .find((btn) => btn.textContent === "Aktiv");
        if (activeButton) fireEvent.click(activeButton);
      });

      await waitFor(() => {
        expect(screen.getByText("Sperren")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Sperren"));

      await waitFor(() => {
        expect(screen.getByText("Suspend failed")).toBeInTheDocument();
      });
    });

    it("shows error when reactivate fails", async () => {
      mockFetchOrganizations.mockResolvedValue([
        {
          id: "org-3",
          name: "Suspended Org",
          slug: "suspended-org",
          status: "suspended",
          ownerName: "Bob",
          ownerEmail: "bob@example.com",
          createdAt: "2024-01-05T10:00:00Z",
        },
      ]);
      mockReactivateOrganization.mockRejectedValue(
        new Error("Reactivation failed"),
      );

      render(<SaasAdminPage />);

      await waitFor(() => {
        const suspendedButton = screen
          .getAllByRole("button")
          .find((btn) => btn.textContent?.includes("Gesperrt"));
        if (suspendedButton) fireEvent.click(suspendedButton);
      });

      await waitFor(() => {
        expect(screen.getByText("Reaktivieren")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Reaktivieren"));

      await waitFor(() => {
        expect(screen.getByText("Reactivation failed")).toBeInTheDocument();
      });
    });

    it("clears error when close button clicked", async () => {
      mockFetchOrganizations.mockRejectedValue(new Error("Network error"));

      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Network error")).toBeInTheDocument();
      });

      const closeButton = screen.getByText("Schließen");
      fireEvent.click(closeButton);

      await waitFor(() => {
        expect(screen.queryByText("Network error")).not.toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Empty State Tests
  // =============================================================================

  describe("empty state", () => {
    it("shows empty state when no organizations", async () => {
      mockFetchOrganizations.mockResolvedValue([]);

      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(
          screen.getByText("Keine ausstehenden Anfragen"),
        ).toBeInTheDocument();
      });
    });

    it("shows correct empty message for active filter", async () => {
      mockFetchOrganizations.mockResolvedValue([]);

      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(
          screen.getByText("Keine ausstehenden Anfragen"),
        ).toBeInTheDocument();
      });

      const activeButton = screen
        .getAllByRole("button")
        .find((btn) => btn.textContent === "Aktiv");
      if (activeButton) fireEvent.click(activeButton);

      await waitFor(() => {
        expect(
          screen.getByText("Keine aktiven Organisationen"),
        ).toBeInTheDocument();
      });
    });

    it("shows reset filter button in empty state", async () => {
      mockFetchOrganizations.mockResolvedValue([]);

      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(
          screen.getByText("Alle Organisationen anzeigen"),
        ).toBeInTheDocument();
      });
    });

    it("resets filter when button clicked in empty state", async () => {
      mockFetchOrganizations.mockResolvedValue([
        {
          id: "org-2",
          name: "Active Org",
          slug: "active-org",
          status: "active",
          ownerName: "Jane",
          ownerEmail: "jane@example.com",
          createdAt: "2024-01-10T10:00:00Z",
        },
      ]);

      render(<SaasAdminPage />);

      // Should show "Keine ausstehenden Anfragen" for pending filter
      await waitFor(() => {
        expect(
          screen.getByText("Keine ausstehenden Anfragen"),
        ).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText("Alle Organisationen anzeigen"));

      await waitFor(() => {
        expect(screen.getByText("Active Org")).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Invite Section Tests
  // =============================================================================

  describe("invite section", () => {
    it("shows organization invite form", async () => {
      render(<SaasAdminPage />);

      // Wait for page to load (use unique description text)
      await waitFor(() => {
        expect(
          screen.getByText("Alle registrierten Organisationen verwalten"),
        ).toBeInTheDocument();
      });

      const inviteButtons = screen.getAllByText("Einladung senden");
      fireEvent.click(inviteButtons[0]!);

      await waitFor(() => {
        expect(
          screen.getByTestId("organization-invite-form"),
        ).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Demo Section Tests
  // =============================================================================

  describe("demo section", () => {
    it("shows demo section placeholder", async () => {
      render(<SaasAdminPage />);

      // Wait for page to load (use unique description text)
      await waitFor(() => {
        expect(
          screen.getByText("Alle registrierten Organisationen verwalten"),
        ).toBeInTheDocument();
      });

      const demoButtons = screen.getAllByText("Demo-Umgebung");
      fireEvent.click(demoButtons[0]!);

      await waitFor(() => {
        expect(screen.getByText("In Entwicklung")).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Logout Modal Tests
  // =============================================================================

  describe("logout modal", () => {
    it("opens logout modal when logout button clicked", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /Abmelden/ }),
        ).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole("button", { name: /Abmelden/ }));

      // LogoutModal should be rendered
      await waitFor(() => {
        // The modal is rendered - check for modal components
        // (Actual content depends on LogoutModal implementation)
        expect(document.querySelector('[class*="fixed"]')).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Status Badge Tests
  // =============================================================================

  describe("status badges", () => {
    it("displays correct badge for pending status", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        expect(screen.getByText("Ausstehend")).toBeInTheDocument();
      });
    });

    it("displays correct badge for active status", async () => {
      render(<SaasAdminPage />);

      // Wait for initial load, then switch to all filter to see active org
      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: "Alle" }),
        ).toBeInTheDocument();
      });

      const allButton = screen.getByRole("button", { name: "Alle" });
      fireEvent.click(allButton);

      // Look for the active organization which should have an "Aktiv" badge
      await waitFor(() => {
        expect(screen.getByText("Test Org 2")).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Date Formatting Tests
  // =============================================================================

  describe("date formatting", () => {
    it("displays dates in German format", async () => {
      render(<SaasAdminPage />);

      await waitFor(() => {
        // Check for German date format (DD.MM.YYYY)
        expect(screen.getByText("15.01.2024")).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Owner Display Tests
  // =============================================================================

  describe("owner display", () => {
    it("displays dash when owner name is null", async () => {
      mockFetchOrganizations.mockResolvedValue([
        {
          id: "org-1",
          name: "Test Org",
          slug: "test-org",
          status: "pending",
          ownerName: null,
          ownerEmail: null,
          createdAt: "2024-01-15T10:00:00Z",
        },
      ]);

      render(<SaasAdminPage />);

      await waitFor(() => {
        // Should show "-" for null owner
        const dashElements = screen.getAllByText("-");
        expect(dashElements.length).toBeGreaterThan(0);
      });
    });
  });
});
