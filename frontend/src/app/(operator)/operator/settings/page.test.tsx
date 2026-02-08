import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

const { mockUseOperatorAuth, mockFetch } = vi.hoisted(() => ({
  mockUseOperatorAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

global.fetch = mockFetch;

vi.mock("~/lib/operator/auth-context", () => ({
  useOperatorAuth: mockUseOperatorAuth,
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
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: { id: 1, email: "test@example.com", display_name: "Test User" },
      }),
    } as Response);
  });

  it("shows loading state when auth is loading", async () => {
    mockUseOperatorAuth.mockReturnValue({
      operator: null,
      isLoading: true,
      updateOperator: vi.fn(),
    });

    render(<OperatorSettingsPage />);

    await waitFor(() => {
      expect(screen.getByText("Loading...")).toBeInTheDocument();
    });
  });

  it("renders settings form with operator data", async () => {
    mockUseOperatorAuth.mockReturnValue({
      operator: { displayName: "John Doe", email: "john@example.com" },
      isLoading: false,
      updateOperator: vi.fn(),
    });

    render(<OperatorSettingsPage />);

    await waitFor(() => {
      const displayNameInput = screen.getByLabelText("Anzeigename");
      expect(displayNameInput).toHaveValue("John Doe");
    });
  });

  it("updates profile on save", async () => {
    const mockUpdateOperator = vi.fn();
    mockUseOperatorAuth.mockReturnValue({
      operator: { displayName: "John Doe", email: "john@example.com" },
      isLoading: false,
      updateOperator: mockUpdateOperator,
    });

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
  });
});
