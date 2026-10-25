import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MODULE_KEYS } from "~/lib/modules";
import type { TenantInfo } from "~/lib/tenant-api";
import { useBookedModules, useTenantSafe } from "~/lib/tenant-context";

const useSWRAuth = vi.hoisted(() => vi.fn());

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth,
  useTenantMutate: () => vi.fn(),
}));

import AbsenceTypesPage from "./page";

const TENANT = { tenantId: 1, slug: "demo" } as TenantInfo;

describe("AbsenceTypesPage", () => {
  beforeEach(() => {
    useSWRAuth.mockClear();
    vi.mocked(useTenantSafe).mockReturnValue({
      tenantSlug: "demo",
      tenant: TENANT,
      routingMode: "subdomain",
    });
    vi.mocked(useBookedModules).mockReturnValue(MODULE_KEYS);
  });

  it("keeps absence-type data hooks closed when the module is not booked", () => {
    vi.mocked(useBookedModules).mockReturnValue(
      MODULE_KEYS.filter((key) => key !== "zeiterfassung"),
    );

    render(<AbsenceTypesPage />);

    // Anker: Der gesperrte Seitenzustand ist gerendert.
    expect(screen.getByText("Zeiterfassung ist nicht gebucht")).toBeVisible();
    expect(useSWRAuth).not.toHaveBeenCalled();
  });
});
