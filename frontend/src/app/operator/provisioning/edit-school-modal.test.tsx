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
        <div data-testid="modal-footer">{footer}</div>
      </div>
    ) : null,
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    updateSchool: mockUpdateSchool,
  },
}));

vi.mock("~/lib/hooks/use-scroll-to-error", () => ({
  useScrollToError: () => vi.fn(),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: mockLoggerError,
  }),
}));

import { EditSchoolModal } from "./edit-school-modal";
import type { School, Organization } from "~/lib/operator/provisioning-helpers";

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
  phone: "+49123456",
  email: "info@test.de",
  active: true,
  hidden: false,
  deletedAt: null,
  createdAt: "2025-01-01T00:00:00Z",
  updatedAt: "2025-01-01T00:00:00Z",
  organization: mockOrg,
};

function renderModal(schoolOverrides: Partial<School> = {}) {
  const onClose = vi.fn();
  const onUpdated = vi.fn().mockResolvedValue(undefined);
  const school = { ...mockSchool, ...schoolOverrides };

  const result = render(
    <EditSchoolModal
      isOpen={true}
      onClose={onClose}
      school={school}
      organizations={[mockOrg]}
      onUpdated={onUpdated}
    />,
  );

  return { ...result, onClose, onUpdated, school };
}

describe("EditSchoolModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUpdateSchool.mockResolvedValue(mockSchool);
    // Mock fetch for revalidation endpoint
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ status: "ok" })),
    );
  });

  it("shows Sichtbar toggle for non-hidden school", () => {
    renderModal();

    expect(screen.getByText("Sichtbar")).toBeInTheDocument();
    expect(screen.queryByText("Verborgen")).not.toBeInTheDocument();
  });

  it("shows Verborgen toggle for hidden school", () => {
    renderModal({ hidden: true });

    expect(screen.getByText("Verborgen")).toBeInTheDocument();
    expect(screen.queryByText("Sichtbar")).not.toBeInTheDocument();
  });

  it("toggles visibility state when clicked", () => {
    renderModal();

    // Initially visible
    const toggle = screen.getByRole("checkbox");
    expect(toggle).toBeChecked();
    expect(screen.getByText("Sichtbar")).toBeInTheDocument();

    // Click to hide
    fireEvent.click(toggle);

    expect(toggle).not.toBeChecked();
    expect(screen.getByText("Verborgen")).toBeInTheDocument();

    // Click to unhide
    fireEvent.click(toggle);

    expect(toggle).toBeChecked();
    expect(screen.getByText("Sichtbar")).toBeInTheDocument();
  });

  it("submits hidden=true after toggling to hidden", async () => {
    renderModal();

    // Toggle to hidden
    fireEvent.click(screen.getByRole("checkbox"));
    expect(screen.getByText("Verborgen")).toBeInTheDocument();

    // Submit form
    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(mockUpdateSchool).toHaveBeenCalledWith(
        "10",
        expect.objectContaining({
          hidden: true,
          active: true,
        }),
      );
    });
  });

  it("submits hidden=false for non-hidden school without toggling", async () => {
    renderModal();

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

  it("submits hidden=false after toggling hidden school back to visible", async () => {
    renderModal({ hidden: true });

    // Toggle back to visible
    fireEvent.click(screen.getByRole("checkbox"));
    expect(screen.getByText("Sichtbar")).toBeInTheDocument();

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
