import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

/* eslint-disable @typescript-eslint/no-explicit-any */

const { mockCreateOrganization } = vi.hoisted(() => ({
  mockCreateOrganization: vi.fn(),
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    createOrganization: mockCreateOrganization,
  },
}));

vi.mock("~/lib/operator/api-helpers", () => ({
  OperatorApiError: class OperatorApiError extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.name = "OperatorApiError";
      this.status = status;
    }
  },
  isOperatorApiError: (error: any) =>
    error instanceof Error && error.name === "OperatorApiError",
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({ isOpen, children, title, footer }: any) =>
    isOpen ? (
      <div data-testid="modal">
        <h2>{title}</h2>
        {children}
        <div data-testid="modal-footer">{footer}</div>
      </div>
    ) : null,
}));

import { CreateOrganizationModal } from "./create-organization-modal";
import { OperatorApiError } from "~/lib/operator/api-helpers";

describe("CreateOrganizationModal", () => {
  const mockOnClose = vi.fn();
  const mockOnCreated = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when closed", () => {
    render(
      <CreateOrganizationModal
        isOpen={false}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );
    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal when open", () => {
    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );
    expect(screen.getByTestId("modal")).toBeInTheDocument();
    expect(screen.getByText("Neue Organisation")).toBeInTheDocument();
  });

  it("renders name and slug inputs", () => {
    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );
    expect(screen.getByLabelText("Name")).toBeInTheDocument();
    expect(screen.getByLabelText("Slug")).toBeInTheDocument();
  });

  it("auto-generates slug from name", () => {
    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );
    const nameInput = screen.getByLabelText("Name");
    fireEvent.change(nameInput, { target: { value: "Schulverband Köln" } });

    const slugInput = screen.getByLabelText("Slug");
    expect(slugInput.value).toBe("schulverband-koeln");
  });

  it("stops auto-generating slug once manually edited", () => {
    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );
    const slugInput = screen.getByLabelText("Slug");
    fireEvent.change(slugInput, { target: { value: "custom" } });

    const nameInput = screen.getByLabelText("Name");
    fireEvent.change(nameInput, { target: { value: "New Name" } });

    expect((slugInput as HTMLInputElement).value).toBe("custom");
  });

  it("shows name error when submitting empty name", async () => {
    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    // Set slug to bypass slug validation
    const slugInput = screen.getByLabelText("Slug");
    fireEvent.change(slugInput, { target: { value: "valid-slug" } });

    // Submit via form
    const form = document.getElementById("create-org-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("Name ist erforderlich")).toBeInTheDocument();
    });
    expect(mockCreateOrganization).not.toHaveBeenCalled();
  });

  it("shows slug error for invalid slug", async () => {
    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    const nameInput = screen.getByLabelText("Name");
    fireEvent.change(nameInput, { target: { value: "Test" } });

    const slugInput = screen.getByLabelText("Slug");
    fireEvent.change(slugInput, { target: { value: "INVALID SLUG!" } });

    const form = document.getElementById("create-org-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt",
        ),
      ).toBeInTheDocument();
    });
  });

  it("submits form successfully", async () => {
    const createdOrg = {
      id: "1",
      name: "Test Org",
      slug: "test-org",
      createdAt: "2026-01-01",
      updatedAt: "2026-01-01",
      active: true,
    };
    mockCreateOrganization.mockResolvedValue(createdOrg);

    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Test Org" },
    });

    const form = document.getElementById("create-org-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(mockCreateOrganization).toHaveBeenCalledWith({
        name: "Test Org",
        slug: "test-org",
      });
      expect(mockOnCreated).toHaveBeenCalledWith(createdOrg);
      expect(mockOnClose).toHaveBeenCalled();
    });
  });

  it("shows slug conflict error on 409", async () => {
    mockCreateOrganization.mockRejectedValue(
      new OperatorApiError("slug already exists", 409),
    );

    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Test Org" },
    });

    const form = document.getElementById("create-org-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("Slug ist bereits vergeben")).toBeInTheDocument();
    });
  });

  it("shows generic 409 error when not a slug conflict", async () => {
    mockCreateOrganization.mockRejectedValue(
      new OperatorApiError("duplicate name", 409),
    );

    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Test Org" },
    });

    const form = document.getElementById("create-org-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("duplicate name")).toBeInTheDocument();
    });
  });

  it("logs non-409 errors", async () => {
    mockCreateOrganization.mockRejectedValue(new Error("Network error"));
    const consoleError = vi
      .spyOn(console, "error")
      // eslint-disable-next-line @typescript-eslint/no-empty-function
      .mockImplementation(() => {});

    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Test" },
    });

    const form = document.getElementById("create-org-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith(
        "organization_create_failed",
        expect.objectContaining({ error: "Network error" }),
      );
    });

    consoleError.mockRestore();
  });

  it("logs non-Error exceptions as strings", async () => {
    mockCreateOrganization.mockRejectedValue("string error");
    const consoleError = vi
      .spyOn(console, "error")
      // eslint-disable-next-line @typescript-eslint/no-empty-function
      .mockImplementation(() => {});

    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Test" },
    });

    const form = document.getElementById("create-org-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith(
        "organization_create_failed",
        expect.objectContaining({ error: "string error" }),
      );
    });

    consoleError.mockRestore();
  });

  it("shows saving state while submitting", async () => {
    let resolveCreate: (value: unknown) => void;
    mockCreateOrganization.mockReturnValue(
      new Promise((resolve) => {
        resolveCreate = resolve;
      }),
    );

    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Test" },
    });

    const form = document.getElementById("create-org-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("Wird erstellt...")).toBeInTheDocument();
    });

    resolveCreate!({
      id: "1",
      name: "Test",
      slug: "test",
      createdAt: "",
      updatedAt: "",
      active: true,
    });

    await waitFor(() => {
      expect(screen.queryByText("Wird erstellt...")).not.toBeInTheDocument();
    });
  });

  it("disables submit button when name or slug is empty", () => {
    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    const footer = screen.getByTestId("modal-footer");
    const submitButton = footer.querySelector("button:last-child");
    expect(submitButton).toBeDisabled();
  });

  it("calls onClose when cancel is clicked", () => {
    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    fireEvent.click(screen.getByText("Abbrechen"));
    expect(mockOnClose).toHaveBeenCalled();
  });

  it("resets form when modal reopens", () => {
    const { rerender } = render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Old Value" },
    });

    rerender(
      <CreateOrganizationModal
        isOpen={false}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    rerender(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    const nameInput = screen.getByLabelText("Name");
    expect(nameInput.value).toBe("");
  });

  it("clears name error when typing in name field", async () => {
    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    const slugInput = screen.getByLabelText("Slug");
    fireEvent.change(slugInput, { target: { value: "valid" } });

    const form = document.getElementById("create-org-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("Name ist erforderlich")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "a" },
    });

    expect(screen.queryByText("Name ist erforderlich")).not.toBeInTheDocument();
  });

  it("clears slug error when typing in slug field", async () => {
    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Test" },
    });
    fireEvent.change(screen.getByLabelText("Slug"), {
      target: { value: "INVALID!" },
    });

    const form = document.getElementById("create-org-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt",
        ),
      ).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Slug"), {
      target: { value: "valid" },
    });

    expect(
      screen.queryByText(
        "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt",
      ),
    ).not.toBeInTheDocument();
  });

  it("shows auto-generated hint when no slug error", () => {
    render(
      <CreateOrganizationModal
        isOpen={true}
        onClose={mockOnClose}
        onCreated={mockOnCreated}
      />,
    );

    expect(
      screen.getByText("Wird automatisch aus dem Namen generiert"),
    ).toBeInTheDocument();
  });
});
