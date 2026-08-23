import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

class MockResizeObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}

vi.stubGlobal("ResizeObserver", MockResizeObserver);

import { PermissionsMasterDetail } from "./permissions-master-detail";
import type { Permission } from "@/lib/auth-helpers";

const readPermission: Permission = {
  id: "1",
  name: "students:read",
  description: "Kinderdaten lesen",
  resource: "students",
  action: "read",
  createdAt: "2026-01-01",
  updatedAt: "2026-01-02",
};

const writePermission: Permission = {
  id: "2",
  name: "students:write",
  description: "Kinderdaten bearbeiten",
  resource: "students",
  action: "write",
  createdAt: "2026-01-01",
  updatedAt: "2026-01-02",
};

describe("PermissionsMasterDetail", () => {
  const onSelect = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows the empty detail state when nothing is selected", () => {
    render(
      <PermissionsMasterDetail
        permissions={[readPermission]}
        selectedId={null}
        selectedPermission={null}
        onSelect={onSelect}
      />,
    );

    expect(screen.queryByText("Bearbeiten")).not.toBeInTheDocument();
    // The list row is rendered.
    expect(screen.getAllByRole("button").length).toBeGreaterThan(0);
  });

  it("renders the selected permission detail with technical name", () => {
    render(
      <PermissionsMasterDetail
        permissions={[readPermission]}
        selectedId="1"
        selectedPermission={readPermission}
        onSelect={onSelect}
      />,
    );

    expect(screen.getAllByText("students:read").length).toBeGreaterThan(0);
    // Section title.
    expect(screen.getByText("Berechtigungsdetails")).toBeInTheDocument();
  });

  it("does not show mutation actions in the selected detail", () => {
    render(
      <PermissionsMasterDetail
        permissions={[readPermission]}
        selectedId="1"
        selectedPermission={readPermission}
        onSelect={onSelect}
      />,
    );

    expect(screen.queryByText("Bearbeiten")).not.toBeInTheDocument();
    expect(screen.queryByText("Löschen")).not.toBeInTheDocument();
  });

  it("groups permissions under a single 'Alle Berechtigungen' bucket and renders a Stammdaten tab", () => {
    render(
      <PermissionsMasterDetail
        permissions={[readPermission, writePermission]}
        selectedId="1"
        selectedPermission={readPermission}
        onSelect={onSelect}
      />,
    );

    expect(screen.getByText("Alle Berechtigungen")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Stammdaten" })).toBeInTheDocument();
  });

  it("forwards a row click to onSelect with that row's permission id", () => {
    render(
      <PermissionsMasterDetail
        permissions={[readPermission, writePermission]}
        selectedId={null}
        selectedPermission={null}
        onSelect={onSelect}
      />,
    );

    // List rows surface the German description as the title. Click the row
    // matching writePermission and assert it selects id="2", confirming each
    // row is wired to its own id rather than a captured stale value.
    fireEvent.click(screen.getByText("Kinderdaten bearbeiten"));
    expect(onSelect).toHaveBeenCalledWith("2");
  });

  it("falls back to a 'resource · action' subtitle when description is blank", () => {
    const permissionWithoutDescription: Permission = {
      ...readPermission,
      id: "9",
      description: "   ",
    };

    render(
      <PermissionsMasterDetail
        permissions={[permissionWithoutDescription]}
        selectedId={null}
        selectedPermission={null}
        onSelect={onSelect}
      />,
    );

    // The subtitle must be the localized "resource · action" composite, not
    // an empty string. We assert on the separator dot — the localized parts
    // are tested in auth-helpers — to stay decoupled from translations.
    expect(screen.getByText(/·/)).toBeInTheDocument();
  });
});
