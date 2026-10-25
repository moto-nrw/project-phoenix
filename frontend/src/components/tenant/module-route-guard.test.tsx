import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MODULE_KEYS } from "~/lib/modules";
import type { TenantInfo } from "~/lib/tenant-api";
import { useBookedModules, useTenantSafe } from "~/lib/tenant-context";

import { ModuleRouteGuard } from "./module-route-guard";

const childRender = vi.fn();

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

const TENANT = { tenantId: 1, slug: "demo" } as TenantInfo;

function DataPage() {
  childRender();
  return <p>Daten der Seite</p>;
}

describe("ModuleRouteGuard", () => {
  beforeEach(() => {
    childRender.mockClear();
    vi.mocked(useTenantSafe).mockReturnValue({
      tenantSlug: "demo",
      tenant: TENANT,
      routingMode: "subdomain",
    });
    vi.mocked(useBookedModules).mockReturnValue(MODULE_KEYS);
  });

  it("does not render a page's data hooks when its module is not booked", () => {
    vi.mocked(useBookedModules).mockReturnValue(
      MODULE_KEYS.filter((key) => key !== "nfc_anwesenheit"),
    );

    render(
      <ModuleRouteGuard module="nfc_anwesenheit" title="Geräte">
        <DataPage />
      </ModuleRouteGuard>,
    );

    // Anker: Der gesperrte Seitenzustand ist gerendert.
    expect(screen.getByText("NFC-Anwesenheit ist nicht gebucht")).toBeVisible();
    expect(childRender).not.toHaveBeenCalled();
    expect(screen.queryByText("Daten der Seite")).not.toBeInTheDocument();
  });

  it("renders the page after the module booking is known", () => {
    render(
      <ModuleRouteGuard module="nfc_anwesenheit" title="Geräte">
        <DataPage />
      </ModuleRouteGuard>,
    );

    expect(screen.getByText("Daten der Seite")).toBeVisible();
    expect(childRender).toHaveBeenCalledOnce();
  });

  it("keeps the page open while tenant data is unavailable", () => {
    vi.mocked(useTenantSafe).mockReturnValue({
      tenantSlug: "demo",
      tenant: null,
      routingMode: "subdomain",
    });
    vi.mocked(useBookedModules).mockReturnValue([]);

    render(
      <ModuleRouteGuard module="nfc_anwesenheit" title="Geräte">
        <DataPage />
      </ModuleRouteGuard>,
    );

    expect(screen.getByText("Daten der Seite")).toBeVisible();
    expect(childRender).toHaveBeenCalledOnce();
  });
});
