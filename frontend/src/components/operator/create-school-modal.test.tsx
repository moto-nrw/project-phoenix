import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

/* eslint-disable @typescript-eslint/no-explicit-any */

const { mockCreateSchool } = vi.hoisted(() => ({
  mockCreateSchool: vi.fn(),
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    createSchool: mockCreateSchool,
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

import { CreateSchoolModal } from "./create-school-modal";
import { OperatorApiError } from "~/lib/operator/api-helpers";
import type { Organization } from "~/lib/operator/provisioning-helpers";

const mockOrgs: Organization[] = [
  {
    id: "1",
    createdAt: "2026-01-01",
    updatedAt: "2026-01-01",
    name: "Org Alpha",
    slug: "org-alpha",
    active: true,
  },
  {
    id: "2",
    createdAt: "2026-01-01",
    updatedAt: "2026-01-01",
    name: "Org Beta",
    slug: "org-beta",
    active: true,
  },
];

describe("CreateSchoolModal", () => {
  const mockOnClose = vi.fn();
  const mockOnCreated = vi.fn();
  const mockOnCreateOrganization = vi.fn();

  const defaultProps = {
    isOpen: true,
    onClose: mockOnClose,
    onCreated: mockOnCreated,
    organizations: mockOrgs,
    onCreateOrganization: mockOnCreateOrganization,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when closed", () => {
    render(<CreateSchoolModal {...defaultProps} isOpen={false} />);
    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal when open", () => {
    render(<CreateSchoolModal {...defaultProps} />);
    expect(screen.getByText("Neue Schule")).toBeInTheDocument();
  });

  it("shows no-org message when organizations is empty", () => {
    render(<CreateSchoolModal {...defaultProps} organizations={[]} />);
    expect(
      screen.getByText(/Es muss zuerst eine Organisation erstellt werden/),
    ).toBeInTheDocument();
  });

  it("triggers onCreateOrganization from no-org message", () => {
    render(<CreateSchoolModal {...defaultProps} organizations={[]} />);
    fireEvent.click(screen.getByText("Organisation erstellen"));
    expect(mockOnClose).toHaveBeenCalled();
    expect(mockOnCreateOrganization).toHaveBeenCalled();
  });

  it("renders org selector with all organizations", () => {
    render(<CreateSchoolModal {...defaultProps} />);
    expect(screen.getByText("Org Alpha")).toBeInTheDocument();
    expect(screen.getByText("Org Beta")).toBeInTheDocument();
  });

  it("auto-selects organization when only one exists", () => {
    render(
      <CreateSchoolModal {...defaultProps} organizations={[mockOrgs[0]!]} />,
    );
    const select = screen.getByLabelText("Organisation");
    expect((select as HTMLSelectElement).value).toBe("1");
  });

  it("auto-generates slug from name", () => {
    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "GGS Mühlenberg" },
    });
    const slugInput = screen.getByLabelText("Slug");
    expect((slugInput as HTMLInputElement).value).toBe("ggs-muehlenberg");
  });

  it("auto-generates subdomain from slug", () => {
    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "GGS Mühlenberg" },
    });
    const subdomainInput = screen.getByLabelText("Subdomain");
    expect((subdomainInput as HTMLInputElement).value).toBe("ggs-muehlenberg");
  });

  it("stops auto-generating slug when manually edited", () => {
    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Slug"), {
      target: { value: "custom" },
    });
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "New Name" },
    });
    expect(screen.getByLabelText<HTMLInputElement>("Slug").value).toBe(
      "custom",
    );
  });

  it("stops auto-generating subdomain when manually edited", () => {
    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Subdomain"), {
      target: { value: "custom-sub" },
    });
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "New Name" },
    });
    expect(screen.getByLabelText<HTMLInputElement>("Subdomain").value).toBe(
      "custom-sub",
    );
  });

  it("shows validation errors on empty submit", async () => {
    render(<CreateSchoolModal {...defaultProps} />);
    const form = document.getElementById("create-school-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(
        screen.getByText("Organisation ist erforderlich"),
      ).toBeInTheDocument();
      expect(screen.getByText("Name ist erforderlich")).toBeInTheDocument();
    });
  });

  it("validates slug format", async () => {
    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Organisation"), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "Test" },
    });
    fireEvent.change(screen.getByLabelText("Slug"), {
      target: { value: "INVALID!" },
    });
    // Set subdomain to valid value to isolate slug error
    fireEvent.change(screen.getByLabelText("Subdomain"), {
      target: { value: "valid" },
    });

    const form = document.getElementById("create-school-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt",
        ),
      ).toBeInTheDocument();
    });
  });

  it("validates subdomain format", async () => {
    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Organisation"), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "Test" },
    });
    fireEvent.change(screen.getByLabelText("Slug"), {
      target: { value: "valid" },
    });
    fireEvent.change(screen.getByLabelText("Subdomain"), {
      target: { value: "INVALID!" },
    });

    const form = document.getElementById("create-school-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt",
        ),
      ).toBeInTheDocument();
    });
  });

  it("submits form with required fields only", async () => {
    const createdSchool = { id: "5", name: "Test School" };
    mockCreateSchool.mockResolvedValue(createdSchool);

    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Organisation"), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "Test School" },
    });

    const form = document.getElementById("create-school-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(mockCreateSchool).toHaveBeenCalledWith({
        organization_id: 1,
        name: "Test School",
        slug: "test-school",
        subdomain: "test-school",
      });
      expect(mockOnCreated).toHaveBeenCalledWith(createdSchool);
      expect(mockOnClose).toHaveBeenCalled();
    });
  });

  it("submits form with optional fields", async () => {
    mockCreateSchool.mockResolvedValue({ id: "5" });

    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Organisation"), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "Test" },
    });
    fireEvent.change(screen.getByLabelText("Adresse"), {
      target: { value: "Musterstr. 1" },
    });
    fireEvent.change(screen.getByLabelText("PLZ"), {
      target: { value: "50667" },
    });
    fireEvent.change(screen.getByLabelText("Stadt"), {
      target: { value: "Köln" },
    });
    fireEvent.change(screen.getByLabelText("Telefon"), {
      target: { value: "0221 12345" },
    });
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "Info@Schule.de" },
    });

    const form = document.getElementById("create-school-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(mockCreateSchool).toHaveBeenCalledWith(
        expect.objectContaining({
          address: "Musterstr. 1",
          city: "Köln",
          zip: "50667",
          phone: "0221 12345",
          email: "info@schule.de",
        }),
      );
    });
  });

  it("shows subdomain conflict on 409", async () => {
    mockCreateSchool.mockRejectedValue(
      new OperatorApiError("subdomain already taken", 409),
    );

    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Organisation"), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "Test" },
    });

    const form = document.getElementById("create-school-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(
        screen.getByText("Subdomain ist bereits vergeben"),
      ).toBeInTheDocument();
    });
  });

  it("shows slug conflict on 409", async () => {
    mockCreateSchool.mockRejectedValue(
      new OperatorApiError("slug already exists", 409),
    );

    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Organisation"), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "Test" },
    });

    const form = document.getElementById("create-school-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("Slug ist bereits vergeben")).toBeInTheDocument();
    });
  });

  it("shows generic 409 error message", async () => {
    mockCreateSchool.mockRejectedValue(
      new OperatorApiError("some other conflict", 409),
    );

    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Organisation"), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "Test" },
    });

    const form = document.getElementById("create-school-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("some other conflict")).toBeInTheDocument();
    });
  });

  it("logs non-409 errors", async () => {
    mockCreateSchool.mockRejectedValue(new Error("Server error"));
    const consoleError = vi
      .spyOn(console, "error")
      // eslint-disable-next-line @typescript-eslint/no-empty-function
      .mockImplementation(() => {});

    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Organisation"), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "Test" },
    });

    const form = document.getElementById("create-school-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith(
        "school_create_failed",
        expect.objectContaining({ error: "Server error" }),
      );
    });
    consoleError.mockRestore();
  });

  it("logs non-Error exceptions as strings", async () => {
    mockCreateSchool.mockRejectedValue("string error");
    const consoleError = vi
      .spyOn(console, "error")
      // eslint-disable-next-line @typescript-eslint/no-empty-function
      .mockImplementation(() => {});

    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Organisation"), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "Test" },
    });

    const form = document.getElementById("create-school-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith(
        "school_create_failed",
        expect.objectContaining({ error: "string error" }),
      );
    });
    consoleError.mockRestore();
  });

  it("shows saving state", async () => {
    let resolveCreate: (value: unknown) => void;
    mockCreateSchool.mockReturnValue(
      new Promise((resolve) => {
        resolveCreate = resolve;
      }),
    );

    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Organisation"), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "Test" },
    });

    const form = document.getElementById("create-school-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("Wird erstellt...")).toBeInTheDocument();
    });

    resolveCreate!({ id: "1" });

    await waitFor(() => {
      expect(screen.queryByText("Wird erstellt...")).not.toBeInTheDocument();
    });
  });

  it("calls onClose when cancel is clicked", () => {
    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.click(screen.getByText("Abbrechen"));
    expect(mockOnClose).toHaveBeenCalled();
  });

  it("disables submit when required fields are empty", () => {
    render(<CreateSchoolModal {...defaultProps} />);
    const footer = screen.getByTestId("modal-footer");
    const submitButton = footer.querySelector("button:last-child");
    expect(submitButton).toBeDisabled();
  });

  it("resets form when modal reopens", () => {
    const { rerender } = render(
      <CreateSchoolModal {...defaultProps} isOpen={true} />,
    );

    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "Old Value" },
    });

    rerender(<CreateSchoolModal {...defaultProps} isOpen={false} />);
    rerender(<CreateSchoolModal {...defaultProps} isOpen={true} />);

    const nameInput = screen.getByLabelText("Schulname");
    expect((nameInput as HTMLInputElement).value).toBe("");
  });

  it("clears field errors when typing", async () => {
    render(<CreateSchoolModal {...defaultProps} />);
    const form = document.getElementById("create-school-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(
        screen.getByText("Organisation ist erforderlich"),
      ).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Organisation"), {
      target: { value: "1" },
    });
    expect(
      screen.queryByText("Organisation ist erforderlich"),
    ).not.toBeInTheDocument();
  });

  it("clears name error when typing in name field", async () => {
    render(<CreateSchoolModal {...defaultProps} />);
    const form = document.getElementById("create-school-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("Name ist erforderlich")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "a" },
    });
    expect(screen.queryByText("Name ist erforderlich")).not.toBeInTheDocument();
  });

  it("clears slug error when typing in slug field", async () => {
    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Organisation"), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "Test" },
    });
    fireEvent.change(screen.getByLabelText("Slug"), {
      target: { value: "INVALID!" },
    });
    // Set subdomain to valid value to isolate slug error
    fireEvent.change(screen.getByLabelText("Subdomain"), {
      target: { value: "valid" },
    });

    const form = document.getElementById("create-school-form")!;
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

  it("clears subdomain error when typing in subdomain field", async () => {
    render(<CreateSchoolModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("Organisation"), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText("Schulname"), {
      target: { value: "Test" },
    });
    fireEvent.change(screen.getByLabelText("Slug"), {
      target: { value: "valid" },
    });
    fireEvent.change(screen.getByLabelText("Subdomain"), {
      target: { value: "INVALID!" },
    });

    const form = document.getElementById("create-school-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt",
        ),
      ).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Subdomain"), {
      target: { value: "valid" },
    });
    expect(
      screen.queryByText(
        "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt",
      ),
    ).not.toBeInTheDocument();
  });

  it("shows .moto-app.de suffix next to subdomain input", () => {
    render(<CreateSchoolModal {...defaultProps} />);
    expect(screen.getByText(".moto-app.de")).toBeInTheDocument();
  });

  it("shows auto-generated hints for slug and subdomain when no errors", () => {
    render(<CreateSchoolModal {...defaultProps} />);
    const hints = screen.getAllByText(
      /Wird automatisch aus dem (Namen|Slug) generiert/,
    );
    expect(hints).toHaveLength(2);
  });
});
