import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

// ============================================================================
// Mocks
// ============================================================================

const { mockListAllTenants } = vi.hoisted(() => ({
  mockListAllTenants: vi.fn(),
}));

vi.mock("~/lib/tenant-api", () => ({
  listAllTenants: mockListAllTenants,
}));

vi.mock("next/image", () => ({
  default: (props: Record<string, unknown>) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img {...props} alt={props.alt as string} />
  ),
}));

import RootPage from "./page";

// ============================================================================
// Tests
// ============================================================================

describe("RootPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows loading skeletons initially", () => {
    // eslint-disable-next-line @typescript-eslint/no-empty-function
    mockListAllTenants.mockReturnValue(new Promise(() => {})); // never resolves

    render(<RootPage />);

    expect(screen.getByText("Schule auswählen")).toBeInTheDocument();
  });

  it("renders tenant cards after loading", async () => {
    mockListAllTenants.mockResolvedValue([
      {
        tenantId: 0,
        slug: "school-a",
        name: "Testschule A",
        subdomain: "school-a",
        organizationId: 0,
        organizationName: "",
        settings: {},
      },
      {
        tenantId: 0,
        slug: "school-b",
        name: "Testschule B",
        subdomain: "school-b",
        organizationId: 0,
        organizationName: "",
        settings: {},
      },
    ]);

    render(<RootPage />);

    await waitFor(() => {
      expect(screen.getByText("Testschule A")).toBeInTheDocument();
    });

    expect(screen.getByText("Testschule B")).toBeInTheDocument();
  });

  it("uses NEXT_PUBLIC_TENANT_DOMAIN for tenant links", async () => {
    mockListAllTenants.mockResolvedValue([
      {
        tenantId: 0,
        slug: "school-a",
        name: "School A",
        subdomain: "school-a",
        organizationId: 0,
        organizationName: "",
        settings: {},
      },
    ]);

    render(<RootPage />);

    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });

    const link = screen.getByText("School A").closest("a");
    expect(link).toBeInTheDocument();
    // href should contain the subdomain (not hardcoded localhost)
    expect(link?.href).toContain("school-a.");
  });

  it("shows fallback notice when backend is unreachable", async () => {
    mockListAllTenants.mockRejectedValue(new Error("Network error"));

    render(<RootPage />);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Backend nicht erreichbar — statische Links werden angezeigt.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows fallback when API returns empty list", async () => {
    mockListAllTenants.mockResolvedValue([]);

    render(<RootPage />);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Backend nicht erreichbar — statische Links werden angezeigt.",
        ),
      ).toBeInTheDocument();
    });

    // Fallback tenants should be rendered
    expect(screen.getByText("Testschule A")).toBeInTheDocument();
    expect(screen.getByText("Testschule B")).toBeInTheDocument();
  });
});
