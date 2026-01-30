/**
 * Tests for TeacherEditModal
 * Tests the rendering and functionality of the teacher edit modal
 */
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { TeacherEditModal } from "./teacher-edit-modal";
import type { Teacher } from "@/lib/teacher-api";

// Mock Modal component
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    onClose,
    title,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <h2>{title}</h2>
        <button onClick={onClose}>Close</button>
        {children}
      </div>
    ) : null,
}));

// Mock TeacherForm component
vi.mock("./teacher-form", () => ({
  TeacherForm: ({
    onSubmitAction,
    onCancelAction,
    isLoading,
    initialData,
  }: {
    onSubmitAction: () => void;
    onCancelAction: () => void;
    isLoading: boolean;
    initialData?: Teacher;
    submitLabel: string;
  }) => (
    <div data-testid="teacher-form">
      <div data-testid="initial-data">{JSON.stringify(initialData)}</div>
      <button
        onClick={onSubmitAction}
        disabled={isLoading}
        data-testid="submit-button"
      >
        Submit
      </button>
      <button onClick={onCancelAction} data-testid="cancel-button">
        Cancel
      </button>
    </div>
  ),
}));

describe("TeacherEditModal", () => {
  const mockTeacher: Teacher = {
    id: "1",
    name: "John Doe",
    first_name: "John",
    last_name: "Doe",
    email: "john.doe@example.com",
  };

  const mockOnClose = vi.fn();
  const mockOnSave = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the modal when open with teacher data", async () => {
    render(
      <TeacherEditModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });

  it("does not render when closed", () => {
    render(
      <TeacherEditModal
        isOpen={false}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onSave={mockOnSave}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("returns null when teacher is null", () => {
    const { container } = render(
      <TeacherEditModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={null}
        onSave={mockOnSave}
      />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("displays the correct title", async () => {
    render(
      <TeacherEditModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Betreuer bearbeiten")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <TeacherEditModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onSave={mockOnSave}
        loading={true}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
    });
  });

  it("renders the teacher form when not loading", async () => {
    render(
      <TeacherEditModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onSave={mockOnSave}
        loading={false}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("teacher-form")).toBeInTheDocument();
    });
  });

  it("passes teacher data as initial data to form", async () => {
    render(
      <TeacherEditModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      const initialDataText =
        screen.getByTestId("initial-data").textContent ?? "";
      const initialData = JSON.parse(initialDataText) as Teacher;
      expect(initialData).toHaveProperty("id", "1");
      expect(initialData).toHaveProperty("firstName", "John");
      expect(initialData).toHaveProperty("email", "john.doe@example.com");
    });
  });

  it("calls onClose when modal close button is clicked", async () => {
    render(
      <TeacherEditModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Close")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByText("Close"));
    });

    expect(mockOnClose).toHaveBeenCalledTimes(1);
  });

  it("calls onSave when submit button is clicked", async () => {
    mockOnSave.mockResolvedValue(undefined);

    render(
      <TeacherEditModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("submit-button")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("submit-button"));
    });

    expect(mockOnSave).toHaveBeenCalledTimes(1);
  });
});
