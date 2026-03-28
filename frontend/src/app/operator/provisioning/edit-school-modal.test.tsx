import type { ReactNode } from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { mockUpdateSchool, mockLoggerError } = vi.hoisted(() => ({
  mockUpdateSchool: vi.fn(),
  mockLoggerError: vi.fn(),
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: ReactNode;
    footer?: ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <h2>{title}</h2>
        {children}
        <div>{footer}</div>
      </div>
    ) : null,
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    updateSchool: (...args: unknown[]) => mockUpdateSchool(...args),
  },
}));

vi.mock("~/lib/operator/provisioning-helpers", async (importOriginal) => {
  const actual =
    await importOriginal<
      typeof import("~/lib/operator/provisioning-helpers")
    >();
  return {
    ...actual,
  };
});

vi.mock("~/lib/operator/api-helpers", () => ({
  isOperatorApiError: () => false,
}));

vi.mock("~/lib/hooks/use-scroll-to-error", () => ({
  useScrollToError: () => vi.fn(),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: mockLoggerError,
  }),
}));

import { EditSchoolModal } from "./edit-school-modal";
import type { Organization, School } from "~/lib/operator/provisioning-helpers";

const mockOrg: Organization = {
  id: "1",
  name: "Test Org",
  slug: "test-org",
  active: true,
  createdAt: "2025-01-01T00:00:00Z",
  updatedAt: "2025-01-01T00:00:00Z",
};

const mockSchool: School = {
  id: "10",
  organizationId: "1",
  name: "Test School",
  slug: "test-school",
  subdomain: "test-school",
  address: "Main St 1",
  city: "Berlin",
  zip: "10115",
  phone: "",
  email: "",
  active: true,
  hidden: false,
  deletedAt: null,
  createdAt: "2025-01-01T00:00:00Z",
  updatedAt: "2025-01-01T00:00:00Z",
  organization: mockOrg,
};

describe("EditSchoolModal", () => {
  const onClose = vi.fn();
  const onUpdated = vi.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    vi.clearAllMocks();
    onUpdated.mockResolvedValue(undefined);
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ status: "ok" })),
    );
  });

  it("renders visibility toggle with Sichtbar when school is not hidden", () => {
    render(
      <EditSchoolModal
        isOpen={true}
        onClose={onClose}
        school={mockSchool}
        organizations={[mockOrg]}
        onUpdated={onUpdated}
      />,
    );

    expect(screen.getByText("Sichtbar")).toBeInTheDocument();
    expect(
      screen.getByText(/Schule wird auf der Startseite/),
    ).toBeInTheDocument();
  });

  it("renders visibility toggle with Verborgen when school is hidden", () => {
    const hiddenSchool = { ...mockSchool, hidden: true };
    render(
      <EditSchoolModal
        isOpen={true}
        onClose={onClose}
        school={hiddenSchool}
        organizations={[mockOrg]}
        onUpdated={onUpdated}
      />,
    );

    expect(screen.getByText("Verborgen")).toBeInTheDocument();
    expect(screen.getByText(/nicht auf der Startseite/)).toBeInTheDocument();
  });

  it("toggles visibility state when switch is clicked", () => {
    render(
      <EditSchoolModal
        isOpen={true}
        onClose={onClose}
        school={mockSchool}
        organizations={[mockOrg]}
        onUpdated={onUpdated}
      />,
    );

    expect(screen.getByText("Sichtbar")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("switch", { name: "Sichtbarkeit umschalten" }),
    );

    expect(screen.getByText("Verborgen")).toBeInTheDocument();
  });

  it("sends hidden=true when saving with visibility toggled off", async () => {
    mockUpdateSchool.mockResolvedValue(mockSchool);

    render(
      <EditSchoolModal
        isOpen={true}
        onClose={onClose}
        school={mockSchool}
        organizations={[mockOrg]}
        onUpdated={onUpdated}
      />,
    );

    // Toggle visibility off
    fireEvent.click(
      screen.getByRole("switch", { name: "Sichtbarkeit umschalten" }),
    );

    // Submit
    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(mockUpdateSchool).toHaveBeenCalledWith(
        "10",
        expect.objectContaining({
          hidden: true,
          name: "Test School",
        }),
      );
    });
  });

  it("sends hidden=false when school is visible", async () => {
    mockUpdateSchool.mockResolvedValue(mockSchool);

    render(
      <EditSchoolModal
        isOpen={true}
        onClose={onClose}
        school={mockSchool}
        organizations={[mockOrg]}
        onUpdated={onUpdated}
      />,
    );

    // Submit without toggling
    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(mockUpdateSchool).toHaveBeenCalledWith(
        "10",
        expect.objectContaining({
          hidden: false,
        }),
      );
    });
  });
});
