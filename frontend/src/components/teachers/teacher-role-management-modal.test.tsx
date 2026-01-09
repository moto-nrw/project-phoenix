import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";

// Mock modules that have environment dependencies
vi.mock("~/lib/auth-service", () => ({
  authService: {
    getRoles: vi.fn(),
    getAccountRoles: vi.fn(),
    assignRoleToAccount: vi.fn(),
    removeRoleFromAccount: vi.fn(),
  },
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: vi.fn(),
    error: vi.fn(),
  }),
}));

// Import after mocks are set up
import { RoleInfo } from "./teacher-role-management-modal";

describe("RoleInfo", () => {
  it("displays translated system role name", () => {
    const role = {
      id: "1",
      name: "admin",
      description: "Admin role",
      permissions: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    render(<RoleInfo role={role} />);

    expect(screen.getByText("Administrator")).toBeInTheDocument();
  });

  it("displays translated role description for system roles", () => {
    const role = {
      id: "1",
      name: "user",
      description: "Standard user",
      permissions: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    render(<RoleInfo role={role} />);

    expect(
      screen.getByText("Standardbenutzer mit grundlegenden Berechtigungen"),
    ).toBeInTheDocument();
  });

  it("displays permission count when permissions exist", () => {
    const role = {
      id: "1",
      name: "guest",
      description: "Guest role",
      permissions: [
        {
          id: "1",
          name: "read",
          description: "Read permission",
          resource: "users",
          action: "read",
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        {
          id: "2",
          name: "write",
          description: "Write permission",
          resource: "users",
          action: "write",
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
      ],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    render(<RoleInfo role={role} />);

    expect(screen.getByText("2 Berechtigungen")).toBeInTheDocument();
  });

  it("does not display permissions section when no permissions", () => {
    const role = {
      id: "1",
      name: "admin",
      description: "Admin role",
      permissions: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    render(<RoleInfo role={role} />);

    expect(screen.queryByText(/Berechtigungen/)).not.toBeInTheDocument();
  });

  it("displays original name for non-system roles", () => {
    const role = {
      id: "1",
      name: "teacher",
      description: "Teaching staff",
      permissions: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    render(<RoleInfo role={role} />);

    expect(screen.getByText("teacher")).toBeInTheDocument();
    expect(screen.getByText("Teaching staff")).toBeInTheDocument();
  });
});
