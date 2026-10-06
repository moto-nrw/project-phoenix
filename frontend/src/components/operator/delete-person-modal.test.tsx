/**
 * Tests for DeletePersonModal.
 *
 * Covers: null person returns null, type-to-confirm behaviour (button disabled
 * until full name matches), success path triggers onDeleted + onClose,
 * cancel resets state, and error path surfaces the API message and keeps
 * the modal open.
 */
import {
  render as renderPlain,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { ERROR_CATALOG } from "~/lib/error-catalog.generated";
import { catalogText } from "~/test/error-catalog-text";

function render(ui: React.ReactElement) {
  return renderPlain(ui, { wrapper: ToastProvider });
}

/** What the shared path shows for a failure that is no API error. */
function crashText(object: string) {
  const text = ERROR_CATALOG.de.actions.crash.replace("{object}", object);
  return text.charAt(0).toUpperCase() + text.slice(1);
}

const { mockSoftDeletePerson } = vi.hoisted(() => ({
  mockSoftDeletePerson: vi.fn(),
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    softDeletePerson: mockSoftDeletePerson,
  },
}));

import { DeletePersonModal } from "./delete-person-modal";
import type { OperatorPerson } from "~/lib/operator/provisioning-helpers";

function makePerson(overrides: Partial<OperatorPerson> = {}): OperatorPerson {
  return {
    id: "5",
    firstName: "Anna",
    lastName: "Beispiel",
    fullName: "Anna Beispiel",
    hasAccount: true,
    accountEmail: "anna@example.com",
    hasRfidCard: false,
    isStaff: true,
    isStudent: false,
    schoolId: "10",
    schoolName: "Schule A",
    organizationId: "42",
    organizationName: "Träger A",
    createdAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function getDeleteButton(): HTMLButtonElement {
  const buttons = screen.getAllByRole("button", { name: "Endgültig löschen" });
  return buttons[0] as HTMLButtonElement;
}

describe("DeletePersonModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when person is null", () => {
    const { container } = renderPlain(
      <DeletePersonModal person={null} onClose={vi.fn()} onDeleted={vi.fn()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders person name and school", () => {
    render(
      <DeletePersonModal
        person={makePerson()}
        onClose={vi.fn()}
        onDeleted={vi.fn()}
      />,
    );

    expect(screen.getByText("Person löschen")).toBeInTheDocument();
    // Name appears multiple times (prompt + confirm prompt + placeholder)
    expect(screen.getAllByText(/Anna Beispiel/).length).toBeGreaterThan(0);
    expect(screen.getByText("Schule A")).toBeInTheDocument();
  });

  it("disables the delete button until the full name is typed", () => {
    render(
      <DeletePersonModal
        person={makePerson()}
        onClose={vi.fn()}
        onDeleted={vi.fn()}
      />,
    );

    expect(getDeleteButton()).toBeDisabled();

    const input = screen.getByLabelText(/Geben Sie den vollständigen Namen/);
    fireEvent.change(input, { target: { value: "Anna" } });
    expect(getDeleteButton()).toBeDisabled();

    fireEvent.change(input, { target: { value: "Anna Beispiel" } });
    expect(getDeleteButton()).not.toBeDisabled();
  });

  it("does not call softDeletePerson when name does not match", () => {
    render(
      <DeletePersonModal
        person={makePerson()}
        onClose={vi.fn()}
        onDeleted={vi.fn()}
      />,
    );

    fireEvent.click(getDeleteButton());

    expect(mockSoftDeletePerson).not.toHaveBeenCalled();
  });

  it("soft-deletes the person, calls onDeleted, and closes on success", async () => {
    mockSoftDeletePerson.mockResolvedValue(undefined);
    const onClose = vi.fn();
    const onDeleted = vi.fn().mockResolvedValue(undefined);

    render(
      <DeletePersonModal
        person={makePerson()}
        onClose={onClose}
        onDeleted={onDeleted}
      />,
    );

    fireEvent.change(
      screen.getByLabelText(/Geben Sie den vollständigen Namen/),
      { target: { value: "Anna Beispiel" } },
    );
    fireEvent.click(getDeleteButton());

    await waitFor(() => {
      expect(mockSoftDeletePerson).toHaveBeenCalledWith("5");
    });
    await waitFor(() => {
      expect(onDeleted).toHaveBeenCalledTimes(1);
    });
    await waitFor(() => {
      expect(onClose).toHaveBeenCalledTimes(1);
    });
  });

  it("calls onClose without invoking the API when Abbrechen is clicked", () => {
    const onClose = vi.fn();
    render(
      <DeletePersonModal
        person={makePerson()}
        onClose={onClose}
        onDeleted={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(mockSoftDeletePerson).not.toHaveBeenCalled();
  });

  it("surfaces the API error message and keeps the modal open on failure", async () => {
    mockSoftDeletePerson.mockRejectedValue(
      new ApiError("person has open visits", 409, {
        code: "general.business_rejection",
      }),
    );
    const onClose = vi.fn();
    const onDeleted = vi.fn();

    render(
      <DeletePersonModal
        person={makePerson()}
        onClose={onClose}
        onDeleted={onDeleted}
      />,
    );

    fireEvent.change(
      screen.getByLabelText(/Geben Sie den vollständigen Namen/),
      { target: { value: "Anna Beispiel" } },
    );
    fireEvent.click(getDeleteButton());

    expect(
      await screen.findByText(
        catalogText("general.business_rejection", "das Löschen der Person"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("person has open visits")).toBeNull();
    expect(onDeleted).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("disables both buttons and shows in-flight label while delete is pending", async () => {
    let resolveDelete: (() => void) | undefined;
    mockSoftDeletePerson.mockReturnValue(
      new Promise<void>((resolve) => {
        resolveDelete = resolve;
      }),
    );

    render(
      <DeletePersonModal
        person={makePerson()}
        onClose={vi.fn()}
        onDeleted={vi.fn()}
      />,
    );

    fireEvent.change(
      screen.getByLabelText(/Geben Sie den vollständigen Namen/),
      { target: { value: "Anna Beispiel" } },
    );
    fireEvent.click(getDeleteButton());

    await waitFor(() => {
      expect(screen.getByText("Wird gelöscht…")).toBeInTheDocument();
    });
    expect(
      screen.getByRole("button", { name: "Wird gelöscht…" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();

    resolveDelete?.();
  });

  it("does not fire a second soft-delete request when the button is clicked twice rapidly", async () => {
    let resolveDelete: (() => void) | undefined;
    mockSoftDeletePerson.mockReturnValue(
      new Promise<void>((resolve) => {
        resolveDelete = resolve;
      }),
    );

    render(
      <DeletePersonModal
        person={makePerson()}
        onClose={vi.fn()}
        onDeleted={vi.fn()}
      />,
    );

    fireEvent.change(
      screen.getByLabelText(/Geben Sie den vollständigen Namen/),
      { target: { value: "Anna Beispiel" } },
    );
    const button = getDeleteButton();
    fireEvent.click(button);
    fireEvent.click(button);
    fireEvent.click(button);

    await waitFor(() => {
      expect(mockSoftDeletePerson).toHaveBeenCalledTimes(1);
    });

    resolveDelete?.();
  });

  it("shows the generic text when the failure is no API error", async () => {
    mockSoftDeletePerson.mockRejectedValue("non-error value");

    render(
      <DeletePersonModal
        person={makePerson()}
        onClose={vi.fn()}
        onDeleted={vi.fn()}
      />,
    );

    fireEvent.change(
      screen.getByLabelText(/Geben Sie den vollständigen Namen/),
      { target: { value: "Anna Beispiel" } },
    );
    fireEvent.click(getDeleteButton());

    await waitFor(() => {
      expect(
        screen.getByText(crashText("das Löschen der Person")),
      ).toBeInTheDocument();
    });
  });
});
