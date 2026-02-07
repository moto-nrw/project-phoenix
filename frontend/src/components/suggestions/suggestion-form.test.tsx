import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { SuggestionForm } from "./suggestion-form";
import type { Suggestion } from "~/lib/suggestions-helpers";

// ============================================================================
// Mocks
// ============================================================================

const mockCreateSuggestion = vi.hoisted(() => vi.fn());
const mockUpdateSuggestion = vi.hoisted(() => vi.fn());

vi.mock("~/lib/suggestions-api", () => ({
  createSuggestion: mockCreateSuggestion,
  updateSuggestion: mockUpdateSuggestion,
}));

const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
  }),
}));

// Mock the Modal component to simplify testing
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    children,
    title,
    footer,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
    footer: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <h2>{title}</h2>
        {children}
        <div data-testid="modal-footer">{footer}</div>
      </div>
    ) : null,
}));

// ============================================================================
// Test data
// ============================================================================

const editSuggestion: Suggestion = {
  id: "1",
  title: "Existing Title",
  description: "Existing Description",
  authorId: "10",
  authorName: "Author",
  status: "open",
  score: 0,
  upvotes: 0,
  downvotes: 0,
  commentCount: 0,
  unreadCount: 0,
  userVote: null,
  createdAt: "2025-01-01T00:00:00Z",
  updatedAt: "2025-01-01T00:00:00Z",
};

// ============================================================================
// Tests
// ============================================================================

describe("SuggestionForm", () => {
  const onClose = vi.fn();
  const onSuccess = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when closed", () => {
    render(
      <SuggestionForm isOpen={false} onClose={onClose} onSuccess={onSuccess} />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders create form when open without editSuggestion", () => {
    render(
      <SuggestionForm isOpen={true} onClose={onClose} onSuccess={onSuccess} />,
    );

    expect(screen.getByText("Neuer Beitrag")).toBeInTheDocument();
    expect(screen.getByLabelText(/Titel/)).toBeInTheDocument();
    expect(screen.getByLabelText(/Beschreibung/)).toBeInTheDocument();
    expect(screen.getByText("Einreichen")).toBeInTheDocument();
  });

  it("renders edit form with pre-filled values", () => {
    render(
      <SuggestionForm
        isOpen={true}
        onClose={onClose}
        onSuccess={onSuccess}
        editSuggestion={editSuggestion}
      />,
    );

    expect(screen.getByText("Beitrag bearbeiten")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Existing Title")).toBeInTheDocument();
    expect(
      screen.getByDisplayValue("Existing Description"),
    ).toBeInTheDocument();
    expect(screen.getByText("Speichern")).toBeInTheDocument();
  });

  it("shows error for empty title on submit", async () => {
    render(
      <SuggestionForm isOpen={true} onClose={onClose} onSuccess={onSuccess} />,
    );

    const form = document.getElementById("suggestion-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("Titel ist erforderlich.")).toBeInTheDocument();
    });
  });

  it("shows error for empty description on submit", async () => {
    render(
      <SuggestionForm isOpen={true} onClose={onClose} onSuccess={onSuccess} />,
    );

    const titleInput = screen.getByLabelText(/Titel/);
    fireEvent.change(titleInput, { target: { value: "Valid title" } });

    const form = document.getElementById("suggestion-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(
        screen.getByText("Beschreibung ist erforderlich."),
      ).toBeInTheDocument();
    });
  });

  it("shows error for title exceeding 200 characters", async () => {
    render(
      <SuggestionForm isOpen={true} onClose={onClose} onSuccess={onSuccess} />,
    );

    const titleInput = screen.getByLabelText(/Titel/);
    fireEvent.change(titleInput, {
      target: { value: "a".repeat(201) },
    });

    const descInput = screen.getByLabelText(/Beschreibung/);
    fireEvent.change(descInput, { target: { value: "Valid desc" } });

    const form = document.getElementById("suggestion-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(
        screen.getByText("Titel darf maximal 200 Zeichen lang sein."),
      ).toBeInTheDocument();
    });
  });

  it("calls createSuggestion for new submissions", async () => {
    mockCreateSuggestion.mockResolvedValue(editSuggestion);

    render(
      <SuggestionForm isOpen={true} onClose={onClose} onSuccess={onSuccess} />,
    );

    fireEvent.change(screen.getByLabelText(/Titel/), {
      target: { value: "New Title" },
    });
    fireEvent.change(screen.getByLabelText(/Beschreibung/), {
      target: { value: "New Description" },
    });

    const form = document.getElementById("suggestion-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(mockCreateSuggestion).toHaveBeenCalledWith({
        title: "New Title",
        description: "New Description",
      });
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Beitrag wurde eingereicht.",
      );
      expect(onSuccess).toHaveBeenCalled();
      expect(onClose).toHaveBeenCalled();
    });
  });

  it("calls updateSuggestion for edits", async () => {
    mockUpdateSuggestion.mockResolvedValue(editSuggestion);

    render(
      <SuggestionForm
        isOpen={true}
        onClose={onClose}
        onSuccess={onSuccess}
        editSuggestion={editSuggestion}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Titel/), {
      target: { value: "Updated Title" },
    });

    const form = document.getElementById("suggestion-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(mockUpdateSuggestion).toHaveBeenCalledWith("1", {
        title: "Updated Title",
        description: "Existing Description",
      });
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Beitrag wurde aktualisiert.",
      );
    });
  });

  it("shows error toast on create failure", async () => {
    mockCreateSuggestion.mockRejectedValue(new Error("Network error"));

    render(
      <SuggestionForm isOpen={true} onClose={onClose} onSuccess={onSuccess} />,
    );

    fireEvent.change(screen.getByLabelText(/Titel/), {
      target: { value: "Title" },
    });
    fireEvent.change(screen.getByLabelText(/Beschreibung/), {
      target: { value: "Description" },
    });

    const form = document.getElementById("suggestion-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Fehler beim Einreichen des Beitrags.",
      );
    });
  });

  it("shows error toast on update failure", async () => {
    mockUpdateSuggestion.mockRejectedValue(new Error("Network error"));

    render(
      <SuggestionForm
        isOpen={true}
        onClose={onClose}
        onSuccess={onSuccess}
        editSuggestion={editSuggestion}
      />,
    );

    const form = document.getElementById("suggestion-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Fehler beim Aktualisieren des Beitrags.",
      );
    });
  });

  it("disables buttons while submitting", async () => {
    let resolveCreate: (value: unknown) => void;
    const createPromise = new Promise((resolve) => {
      resolveCreate = resolve;
    });
    mockCreateSuggestion.mockReturnValue(createPromise);

    render(
      <SuggestionForm isOpen={true} onClose={onClose} onSuccess={onSuccess} />,
    );

    fireEvent.change(screen.getByLabelText(/Titel/), {
      target: { value: "Title" },
    });
    fireEvent.change(screen.getByLabelText(/Beschreibung/), {
      target: { value: "Description" },
    });

    const form = document.getElementById("suggestion-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("Wird gespeichert...")).toBeInTheDocument();
    });

    // Resolve to clean up
    resolveCreate!(editSuggestion);
  });

  it("renders character counter for description", () => {
    render(
      <SuggestionForm isOpen={true} onClose={onClose} onSuccess={onSuccess} />,
    );

    expect(screen.getByText("0 / 5.000")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/Beschreibung/), {
      target: { value: "Hello" },
    });

    expect(screen.getByText("5 / 5.000")).toBeInTheDocument();
  });

  it("calls onClose when Abbrechen is clicked", () => {
    render(
      <SuggestionForm isOpen={true} onClose={onClose} onSuccess={onSuccess} />,
    );

    fireEvent.click(screen.getByText("Abbrechen"));
    expect(onClose).toHaveBeenCalled();
  });
});
