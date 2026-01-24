import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import NewCombinedGroupPage from "./page";

// Mock next/navigation
const mockPush = vi.fn();
const mockBack = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({
    push: mockPush,
    back: mockBack,
  })),
}));

// Mock combinedGroupService
const mockCreateCombinedGroup = vi.fn();
vi.mock("@/lib/api", () => ({
  combinedGroupService: {
    createCombinedGroup: (data: unknown) => mockCreateCombinedGroup(data),
  },
}));

// Mock UI components
vi.mock("@/components/dashboard", () => ({
  PageHeader: ({ title, backUrl }: { title: string; backUrl: string }) => (
    <div data-testid="page-header">
      <h1>{title}</h1>
      <a href={backUrl} data-testid="back-link">
        Back
      </a>
    </div>
  ),
}));

vi.mock("@/components/groups", () => ({
  CombinedGroupForm: ({
    initialData,
    onSubmitAction,
    onCancelAction,
    isLoading,
    formTitle,
    submitLabel,
  }: {
    initialData: {
      name: string;
      is_active: boolean;
      access_policy: string;
      valid_until?: string;
      specific_group_id?: string;
    };
    onSubmitAction: (data: {
      name: string;
      is_active: boolean;
      access_policy: string;
    }) => Promise<void>;
    onCancelAction: () => void;
    isLoading: boolean;
    formTitle: string;
    submitLabel: string;
  }) => (
    <div data-testid="combined-group-form">
      <h2>{formTitle}</h2>
      <span data-testid="initial-name">{initialData.name}</span>
      <span data-testid="initial-active">{String(initialData.is_active)}</span>
      <span data-testid="initial-policy">{initialData.access_policy}</span>
      <span data-testid="loading">{String(isLoading)}</span>
      <button
        data-testid="submit-button"
        onClick={() => {
          // Catch the error to prevent unhandled rejections in tests
          // The component re-throws the error, but the form would normally handle it
          onSubmitAction({
            name: "Test Combined Group",
            is_active: true,
            access_policy: "all",
          }).catch(() => {
            // Error handling is done by the component
          });
        }}
      >
        {submitLabel}
      </button>
      <button data-testid="cancel-button" onClick={onCancelAction}>
        Cancel
      </button>
    </div>
  ),
}));

describe("NewCombinedGroupPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the page with correct title", () => {
    render(<NewCombinedGroupPage />);

    expect(screen.getByText("Neue Gruppenkombination")).toBeInTheDocument();
  });

  it("renders the form with correct initial data", () => {
    render(<NewCombinedGroupPage />);

    expect(screen.getByTestId("initial-name")).toHaveTextContent("");
    expect(screen.getByTestId("initial-active")).toHaveTextContent("true");
    expect(screen.getByTestId("initial-policy")).toHaveTextContent("manual");
  });

  it("renders the form with correct labels", () => {
    render(<NewCombinedGroupPage />);

    expect(
      screen.getByText("Gruppenkombination erstellen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Erstellen")).toBeInTheDocument();
  });

  it("navigates back when cancel is clicked", () => {
    render(<NewCombinedGroupPage />);

    const cancelButton = screen.getByTestId("cancel-button");
    fireEvent.click(cancelButton);

    expect(mockBack).toHaveBeenCalled();
  });

  it("calls create service when form is submitted", async () => {
    mockCreateCombinedGroup.mockResolvedValueOnce({ id: "1" });

    render(<NewCombinedGroupPage />);

    const submitButton = screen.getByTestId("submit-button");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockCreateCombinedGroup).toHaveBeenCalledWith({
        name: "Test Combined Group",
        is_active: true,
        access_policy: "all",
      });
    });
  });

  it("navigates to combined groups list on successful creation", async () => {
    mockCreateCombinedGroup.mockResolvedValueOnce({ id: "1" });

    render(<NewCombinedGroupPage />);

    const submitButton = screen.getByTestId("submit-button");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/database/groups/combined");
    });
  });

  it("shows error message when creation fails", async () => {
    mockCreateCombinedGroup.mockRejectedValueOnce(new Error("Creation failed"));

    render(<NewCombinedGroupPage />);

    const submitButton = screen.getByTestId("submit-button");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Erstellen der Gruppenkombination/),
      ).toBeInTheDocument();
    });
  });

  it("shows loading state during submission", async () => {
    // Create a promise that we can control
    let resolvePromise: (value: { id: string }) => void;
    const promise = new Promise<{ id: string }>((resolve) => {
      resolvePromise = resolve;
    });
    mockCreateCombinedGroup.mockReturnValueOnce(promise);

    render(<NewCombinedGroupPage />);

    expect(screen.getByTestId("loading")).toHaveTextContent("false");

    const submitButton = screen.getByTestId("submit-button");
    fireEvent.click(submitButton);

    // Loading should be true while waiting
    await waitFor(() => {
      expect(screen.getByTestId("loading")).toHaveTextContent("true");
    });

    // Resolve the promise
    resolvePromise!({ id: "1" });

    await waitFor(() => {
      expect(screen.getByTestId("loading")).toHaveTextContent("false");
    });
  });

  it("re-throws error for form component to handle", async () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const testError = new Error("Creation failed");
    mockCreateCombinedGroup.mockRejectedValueOnce(testError);

    render(<NewCombinedGroupPage />);

    const submitButton = screen.getByTestId("submit-button");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(consoleSpy).toHaveBeenCalledWith(
        "Error creating combined group:",
        testError,
      );
    });

    consoleSpy.mockRestore();
  });

  it("handles default values when partial data is provided", async () => {
    mockCreateCombinedGroup.mockResolvedValueOnce({ id: "1" });

    render(<NewCombinedGroupPage />);

    // The form initial values should include defaults
    expect(screen.getByTestId("initial-name")).toHaveTextContent("");
    expect(screen.getByTestId("initial-active")).toHaveTextContent("true");
    expect(screen.getByTestId("initial-policy")).toHaveTextContent("manual");
  });
});
