import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
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

  it("shows loading skeleton initially", () => {
    // eslint-disable-next-line @typescript-eslint/no-empty-function
    mockListAllTenants.mockReturnValue(new Promise(() => {})); // never resolves

    render(<RootPage />);

    expect(screen.getByText("Willkommen bei moto!")).toBeInTheDocument();
    expect(screen.getByText("Einrichtung")).toBeInTheDocument();
    // Select should not be rendered while loading
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  });

  it("renders tenant options in select after loading", async () => {
    mockListAllTenants.mockResolvedValue({
      status: "ok",
      tenants: [
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
      ],
    });

    render(<RootPage />);

    await waitFor(() => {
      expect(screen.getByRole("combobox")).toBeInTheDocument();
    });

    const select = screen.getByRole("combobox");
    const options = select.querySelectorAll("option");
    // Placeholder + 2 tenants
    expect(options).toHaveLength(3);
    expect(options[1]?.textContent).toBe("Testschule A");
    expect(options[2]?.textContent).toBe("Testschule B");
  });

  it("disables button when no tenant is selected", async () => {
    mockListAllTenants.mockResolvedValue({
      status: "ok",
      tenants: [
        {
          tenantId: 0,
          slug: "school-a",
          name: "School A",
          subdomain: "school-a",
          organizationId: 0,
          organizationName: "",
          settings: {},
        },
      ],
    });

    render(<RootPage />);

    await waitFor(() => {
      expect(screen.getByRole("combobox")).toBeInTheDocument();
    });

    const button = screen.getByRole("button", { name: "Weiter" });
    expect(button).toBeDisabled();
  });

  it("enables button when a tenant is selected", async () => {
    mockListAllTenants.mockResolvedValue({
      status: "ok",
      tenants: [
        {
          tenantId: 0,
          slug: "school-a",
          name: "School A",
          subdomain: "school-a",
          organizationId: 0,
          organizationName: "",
          settings: {},
        },
      ],
    });

    render(<RootPage />);

    await waitFor(() => {
      expect(screen.getByRole("combobox")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole("combobox"), {
      target: { value: "school-a" },
    });

    const button = screen.getByRole("button", { name: "Weiter" });
    expect(button).toBeEnabled();
  });

  it("navigates to tenant subdomain when clicking Weiter", async () => {
    mockListAllTenants.mockResolvedValue({
      status: "ok",
      tenants: [
        {
          tenantId: 0,
          slug: "school-a",
          name: "School A",
          subdomain: "school-a",
          organizationId: 0,
          organizationName: "",
          settings: {},
        },
      ],
    });

    render(<RootPage />);

    await waitFor(() => {
      expect(screen.getByRole("combobox")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole("combobox"), {
      target: { value: "school-a" },
    });

    // Mock window.location.href setter
    const hrefSpy = vi.spyOn(window.location, "href", "set");

    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(hrefSpy).toHaveBeenCalledWith(expect.stringContaining("school-a."));

    hrefSpy.mockRestore();
  });

  it("does not navigate when selected slug has no matching tenant", async () => {
    mockListAllTenants.mockResolvedValue({
      status: "ok",
      tenants: [
        {
          tenantId: 0,
          slug: "school-a",
          name: "School A",
          subdomain: "school-a",
          organizationId: 0,
          organizationName: "",
          settings: {},
        },
      ],
    });

    render(<RootPage />);

    await waitFor(() => {
      expect(screen.getByRole("combobox")).toBeInTheDocument();
    });

    // Force a slug value that doesn't match any tenant
    fireEvent.change(screen.getByRole("combobox"), {
      target: { value: "nonexistent" },
    });

    const hrefSpy = vi.spyOn(window.location, "href", "set");

    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(hrefSpy).not.toHaveBeenCalled();

    hrefSpy.mockRestore();
  });

  it("shows backend error when listAllTenants returns error status", async () => {
    mockListAllTenants.mockResolvedValue({ tenants: [], status: "error" });

    render(<RootPage />);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Backend nicht erreichbar — bitte versuchen Sie es später erneut.",
        ),
      ).toBeInTheDocument();
    });

    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  });

  it("shows empty message when backend returns no tenants", async () => {
    mockListAllTenants.mockResolvedValue({ tenants: [], status: "ok" });

    render(<RootPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Aktuell sind keine Einrichtungen verfügbar."),
      ).toBeInTheDocument();
    });

    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  });
});
