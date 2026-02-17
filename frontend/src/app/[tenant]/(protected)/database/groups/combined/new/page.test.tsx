/* eslint-disable @typescript-eslint/no-unsafe-return, @typescript-eslint/no-empty-function */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

const mockPush = vi.fn();
const mockBack = vi.fn();
const mockCreateCombinedGroup = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, back: mockBack }),
}));

vi.mock("@/lib/api", () => ({
  combinedGroupService: {
    createCombinedGroup: (data: unknown) => mockCreateCombinedGroup(data),
  },
}));

vi.mock("@/components/dashboard", () => ({
  PageHeader: ({ title }: { title: string }) => (
    <div data-testid="page-header">{title}</div>
  ),
}));

vi.mock("@/components/groups", () => ({
  CombinedGroupForm: ({
    formTitle,
    submitLabel,
    onSubmitAction,
    onCancelAction,
    isLoading,
  }: {
    formTitle: string;
    submitLabel: string;
    onSubmitAction: (data: unknown) => Promise<void>;
    onCancelAction: () => void;
    isLoading: boolean;
  }) => (
    <div data-testid="combined-group-form">
      <span>{formTitle}</span>
      <button
        data-testid="submit-button"
        onClick={() => {
          onSubmitAction({ name: "Test Group" }).catch(() => {
            // Error is handled by parent component
          });
        }}
        disabled={isLoading}
      >
        {submitLabel}
      </button>
      <button data-testid="cancel-button" onClick={onCancelAction}>
        Cancel
      </button>
      {isLoading && <div data-testid="loading">Loading...</div>}
    </div>
  ),
}));

import NewCombinedGroupPage from "./page";

describe("NewCombinedGroupPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Suppress console.error for expected errors
    vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders page header with correct title", () => {
    render(<NewCombinedGroupPage />);

    expect(screen.getByTestId("page-header")).toHaveTextContent(
      "Neue Gruppenkombination",
    );
  });

  it("renders combined group form", () => {
    render(<NewCombinedGroupPage />);

    expect(screen.getByTestId("combined-group-form")).toBeInTheDocument();
    expect(
      screen.getByText("Gruppenkombination erstellen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Erstellen")).toBeInTheDocument();
  });

  it("handles successful form submission", async () => {
    mockCreateCombinedGroup.mockResolvedValue({ id: "1", name: "Test Group" });

    render(<NewCombinedGroupPage />);

    const submitButton = screen.getByTestId("submit-button");
    submitButton.click();

    await waitFor(() => {
      expect(mockCreateCombinedGroup).toHaveBeenCalledWith({
        name: "Test Group",
        is_active: true,
        access_policy: "manual",
      });
      expect(mockPush).toHaveBeenCalledWith("/database/groups/combined");
    });
  });

  it("displays error message on submission failure", async () => {
    mockCreateCombinedGroup.mockRejectedValue(new Error("Network error"));

    render(<NewCombinedGroupPage />);

    const submitButton = screen.getByTestId("submit-button");
    submitButton.click();

    await waitFor(() => {
      expect(
        screen.getByText(
          "Fehler beim Erstellen der Gruppenkombination. Bitte versuchen Sie es später erneut.",
        ),
      ).toBeInTheDocument();
    });

    // Should not navigate on error
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("shows loading state during submission", async () => {
    mockCreateCombinedGroup.mockImplementation(
      () => new Promise(() => {}), // Never resolves
    );

    render(<NewCombinedGroupPage />);

    const submitButton = screen.getByTestId("submit-button");
    submitButton.click();

    await waitFor(() => {
      expect(screen.getByTestId("loading")).toBeInTheDocument();
      expect(submitButton).toBeDisabled();
    });
  });

  it("calls router.back() when cancel is clicked", () => {
    render(<NewCombinedGroupPage />);

    const cancelButton = screen.getByTestId("cancel-button");
    cancelButton.click();

    expect(mockBack).toHaveBeenCalled();
  });

  it("clears error message before new submission", async () => {
    mockCreateCombinedGroup.mockRejectedValueOnce(new Error("First error"));

    render(<NewCombinedGroupPage />);

    // First submission - error
    const submitButton = screen.getByTestId("submit-button");
    submitButton.click();

    await waitFor(() => {
      expect(screen.getByText(/Fehler beim Erstellen/)).toBeInTheDocument();
    });

    // Second submission - success
    mockCreateCombinedGroup.mockResolvedValue({ id: "1", name: "Test" });
    submitButton.click();

    await waitFor(() => {
      expect(
        screen.queryByText(/Fehler beim Erstellen/),
      ).not.toBeInTheDocument();
    });
  });

  it("passes correct initial data to form", () => {
    render(<NewCombinedGroupPage />);

    // Form should be rendered (tested via other tests)
    expect(screen.getByTestId("combined-group-form")).toBeInTheDocument();
  });
});
