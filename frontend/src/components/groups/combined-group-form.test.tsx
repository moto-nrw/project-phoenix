/**
 * Tests for CombinedGroupForm Component
 * Tests form rendering and validation
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import CombinedGroupForm from "./combined-group-form";

const mockInitialData = {
  name: "Test Group",
  is_active: true,
  access_policy: "manual" as const,
  valid_until: "2024-12-31",
  specific_group_id: "",
};

describe("CombinedGroupForm", () => {
  const mockOnSubmit = vi.fn();
  const mockOnCancel = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockOnSubmit.mockResolvedValue(undefined);
  });

  it("renders form with title", () => {
    render(
      <CombinedGroupForm
        onSubmitAction={mockOnSubmit}
        onCancelAction={mockOnCancel}
        isLoading={false}
        formTitle="Create Group"
        submitLabel="Save"
      />,
    );

    expect(screen.getByText("Create Group")).toBeInTheDocument();
  });

  it("renders name input field", () => {
    render(
      <CombinedGroupForm
        onSubmitAction={mockOnSubmit}
        onCancelAction={mockOnCancel}
        isLoading={false}
        formTitle="Create Group"
        submitLabel="Save"
      />,
    );

    expect(screen.getByLabelText(/Name der Kombination/)).toBeInTheDocument();
  });

  it("renders is_active checkbox", () => {
    render(
      <CombinedGroupForm
        onSubmitAction={mockOnSubmit}
        onCancelAction={mockOnCancel}
        isLoading={false}
        formTitle="Create Group"
        submitLabel="Save"
      />,
    );

    expect(screen.getByLabelText("Aktiv")).toBeInTheDocument();
  });

  it("renders access policy dropdown", () => {
    render(
      <CombinedGroupForm
        onSubmitAction={mockOnSubmit}
        onCancelAction={mockOnCancel}
        isLoading={false}
        formTitle="Create Group"
        submitLabel="Save"
      />,
    );

    expect(screen.getByLabelText(/Zugriffsmethode/)).toBeInTheDocument();
    expect(screen.getByText("Alle Gruppen")).toBeInTheDocument();
    expect(screen.getByText("Erste Gruppe")).toBeInTheDocument();
    expect(screen.getByText("Spezifische Gruppe")).toBeInTheDocument();
    expect(screen.getByText("Manuell")).toBeInTheDocument();
  });

  it("renders valid_until date input", () => {
    render(
      <CombinedGroupForm
        onSubmitAction={mockOnSubmit}
        onCancelAction={mockOnCancel}
        isLoading={false}
        formTitle="Create Group"
        submitLabel="Save"
      />,
    );

    expect(screen.getByLabelText("Gültig bis")).toBeInTheDocument();
  });

  it("renders submit and cancel buttons", () => {
    render(
      <CombinedGroupForm
        onSubmitAction={mockOnSubmit}
        onCancelAction={mockOnCancel}
        isLoading={false}
        formTitle="Create Group"
        submitLabel="Save"
      />,
    );

    expect(screen.getByText("Save")).toBeInTheDocument();
    expect(screen.getByText("Abbrechen")).toBeInTheDocument();
  });

  it("populates form with initial data", async () => {
    render(
      <CombinedGroupForm
        initialData={mockInitialData}
        onSubmitAction={mockOnSubmit}
        onCancelAction={mockOnCancel}
        isLoading={false}
        formTitle="Edit Group"
        submitLabel="Update"
      />,
    );

    await waitFor(() => {
      const nameInput = screen.getByLabelText(/Name der Kombination/);
      expect(nameInput.value).toBe("Test Group");
    });
  });

  it("shows specific group field when access policy is specific", async () => {
    render(
      <CombinedGroupForm
        onSubmitAction={mockOnSubmit}
        onCancelAction={mockOnCancel}
        isLoading={false}
        formTitle="Create Group"
        submitLabel="Save"
      />,
    );

    const policySelect = screen.getByLabelText(/Zugriffsmethode/);
    fireEvent.change(policySelect, { target: { value: "specific" } });

    await waitFor(() => {
      expect(screen.getByLabelText(/Spezifische Gruppe/)).toBeInTheDocument();
    });
  });

  it("validates required name field", async () => {
    render(
      <CombinedGroupForm
        onSubmitAction={mockOnSubmit}
        onCancelAction={mockOnCancel}
        isLoading={false}
        formTitle="Create Group"
        submitLabel="Save"
      />,
    );

    const submitButton = screen.getByText("Save");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(/Bitte geben Sie einen Namen/),
      ).toBeInTheDocument();
    });
  });

  it("validates specific group when policy is specific", async () => {
    render(
      <CombinedGroupForm
        onSubmitAction={mockOnSubmit}
        onCancelAction={mockOnCancel}
        isLoading={false}
        formTitle="Create Group"
        submitLabel="Save"
      />,
    );

    const nameInput = screen.getByLabelText(/Name der Kombination/);
    fireEvent.change(nameInput, { target: { value: "Test Group" } });

    const policySelect = screen.getByLabelText(/Zugriffsmethode/);
    fireEvent.change(policySelect, { target: { value: "specific" } });

    const submitButton = screen.getByText("Save");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(/Bitte wählen Sie eine spezifische Gruppe/),
      ).toBeInTheDocument();
    });
  });

  it("calls onSubmit with form data", async () => {
    render(
      <CombinedGroupForm
        onSubmitAction={mockOnSubmit}
        onCancelAction={mockOnCancel}
        isLoading={false}
        formTitle="Create Group"
        submitLabel="Save"
      />,
    );

    const nameInput = screen.getByLabelText(/Name der Kombination/);
    fireEvent.change(nameInput, { target: { value: "New Group" } });

    const submitButton = screen.getByText("Save");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "New Group",
          is_active: true,
          access_policy: "manual",
        }),
      );
    });
  });

  it("calls onCancel when cancel button clicked", () => {
    render(
      <CombinedGroupForm
        onSubmitAction={mockOnSubmit}
        onCancelAction={mockOnCancel}
        isLoading={false}
        formTitle="Create Group"
        submitLabel="Save"
      />,
    );

    const cancelButton = screen.getByText("Abbrechen");
    fireEvent.click(cancelButton);

    expect(mockOnCancel).toHaveBeenCalledTimes(1);
  });

  it("disables form when loading", () => {
    render(
      <CombinedGroupForm
        onSubmitAction={mockOnSubmit}
        onCancelAction={mockOnCancel}
        isLoading={true}
        formTitle="Create Group"
        submitLabel="Save"
      />,
    );

    const submitButton = screen.getByText("Wird gespeichert...");

    expect(submitButton).toBeDisabled();
  });
});
