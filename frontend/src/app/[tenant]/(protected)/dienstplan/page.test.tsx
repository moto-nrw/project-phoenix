import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MODULE_KEYS } from "~/lib/modules";
import type { TenantInfo } from "~/lib/tenant-api";
import { useBookedModules, useTenantSafe } from "~/lib/tenant-context";

const viewRender = vi.fn();

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

vi.mock("~/components/staff/dienstplan-view", () => ({
  DienstplanView: () => {
    viewRender();
    return <p>Dienstplan-Daten</p>;
  },
}));

import DienstplanPage from "./page";

const TENANT = { tenantId: 1, slug: "demo" } as TenantInfo;

describe("DienstplanPage", () => {
  beforeEach(() => {
    viewRender.mockClear();
    vi.mocked(useTenantSafe).mockReturnValue({
      tenantSlug: "demo",
      tenant: TENANT,
      routingMode: "subdomain",
    });
    vi.mocked(useBookedModules).mockReturnValue(MODULE_KEYS);
  });

  it("keeps Dienstplan data hooks closed when the module is not booked", () => {
    vi.mocked(useBookedModules).mockReturnValue(
      MODULE_KEYS.filter((key) => key !== "dienstplan"),
    );

    render(<DienstplanPage />);

    // Anker: Der gesperrte Seitenzustand ist gerendert.
    expect(screen.getByText("Dienstplan ist nicht gebucht")).toBeVisible();
    expect(viewRender).not.toHaveBeenCalled();
    expect(screen.queryByText("Dienstplan-Daten")).not.toBeInTheDocument();
  });
});
