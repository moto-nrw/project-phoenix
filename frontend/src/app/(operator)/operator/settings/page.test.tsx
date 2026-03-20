import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

const { mockUseSession, mockUpdateSession, mockFetch } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUpdateSession: vi.fn(),
  mockFetch: vi.fn(),
}));

global.fetch = mockFetch;

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div>Loading...</div>,
}));

vi.mock("~/components/shared/settings-layout", () => ({
  SettingsLayout: ({ profileTab }: { profileTab: React.ReactNode }) => (
    <div>{profileTab}</div>
  ),
}));

import OperatorSettingsPage from "./page";

describe("OperatorSettingsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUpdateSession.mockResolvedValue(undefined);
    mockUseSession.mockReturnValue({
      data: {
        user: { name: "John Doe", email: "john@example.com" },
      },
      status: "authenticated",
      update: mockUpdateSession,
    });
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: { id: 1, email: "test@example.com", display_name: "Test User" },
      }),
    } as Response);
  });

  it("shows loading state when auth is loading", async () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "loading",
      update: mockUpdateSession,
    });

    render(<OperatorSettingsPage />);

    await waitFor(() => {
      expect(screen.getByText("Loading...")).toBeInTheDocument();
    });
  });

  it("renders settings form with operator data", async () => {
    render(<OperatorSettingsPage />);

    await waitFor(() => {
      const displayNameInput = screen.getByLabelText("Anzeigename");
      expect(displayNameInput).toHaveValue("John Doe");
    });
  });

  it("updates profile on save", async () => {
    render(<OperatorSettingsPage />);

    await waitFor(() => {
      const editButton = screen.getByText("Bearbeiten");
      fireEvent.click(editButton);
    });

    const displayNameInput = screen.getByLabelText("Anzeigename");
    fireEvent.change(displayNameInput, { target: { value: "Jane Doe" } });

    const saveButton = screen.getByText("Speichern");
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/operator/profile",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ display_name: "Jane Doe" }),
        }),
      );
    });

    await waitFor(() => {
      expect(mockUpdateSession).toHaveBeenCalled();
    });
  });

  it("handles save error gracefully", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: "Something went wrong" }),
    } as Response);

    render(<OperatorSettingsPage />);

    await waitFor(() => {
      fireEvent.click(screen.getByText("Bearbeiten"));
    });

    const saveButton = screen.getByText("Speichern");
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(mockUpdateSession).not.toHaveBeenCalled();
    });
  });

  it("cancels editing and restores original values", async () => {
    render(<OperatorSettingsPage />);

    await waitFor(() => {
      fireEvent.click(screen.getByText("Bearbeiten"));
    });

    const displayNameInput = screen.getByLabelText("Anzeigename");
    fireEvent.change(displayNameInput, { target: { value: "Changed Name" } });

    fireEvent.click(screen.getByText("Abbrechen"));

    await waitFor(() => {
      expect(screen.getByLabelText("Anzeigename")).toHaveValue("John Doe");
    });
  });

  it("displays initials correctly", async () => {
    render(<OperatorSettingsPage />);

    await waitFor(() => {
      expect(screen.getByText("JD")).toBeInTheDocument();
    });
  });

  it("handles single word name initials", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: { name: "Admin", email: "admin@example.com" },
      },
      status: "authenticated",
      update: mockUpdateSession,
    });

    render(<OperatorSettingsPage />);

    await waitFor(() => {
      expect(screen.getByText("A")).toBeInTheDocument();
    });
  });
});
