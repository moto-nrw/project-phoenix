import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MODULE_KEYS } from "~/lib/modules";
import type { TenantInfo } from "~/lib/tenant-api";
import {
  useBookedModules,
  useTenantRoutingModeSafe,
  useTenantSafe,
} from "~/lib/tenant-context";

import { NotBookedPageProvider } from "~/lib/not-booked-page-context";

import { TenantPage } from "./tenant-page";

const navigation = vi.hoisted(() => ({ pathname: "/time-tracking" }));
vi.mock("next/navigation", () => ({
  usePathname: () => navigation.pathname,
}));

const auth = vi.hoisted(() => ({ permissions: [] as string[] }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "1", roles: [], permissions: auth.permissions },
      expires: "",
    },
    status: "authenticated",
  }),
}));

const TENANT = { tenantId: 1, slug: "demo" } as TenantInfo;

function renderGate(title = "Zeiterfassung") {
  return render(
    <NotBookedPageProvider>
      <TenantPage
        title={title}
        actions={<button type="button">Anlegen</button>}
      >
        <div>Inhalt der Seite</div>
      </TenantPage>
    </NotBookedPageProvider>,
  );
}

// Seiten eines nicht gebuchten Moduls erklären sich im Seitengerüst (#4894).
describe("TenantPage bei nicht gebuchtem Modul", () => {
  beforeEach(() => {
    navigation.pathname = "/time-tracking";
    auth.permissions = [];
    vi.mocked(useTenantSafe).mockReturnValue({
      tenantSlug: "demo",
      tenant: TENANT,
      routingMode: "subdomain",
    });
    vi.mocked(useTenantRoutingModeSafe).mockReturnValue("subdomain");
    vi.mocked(useBookedModules).mockReturnValue(MODULE_KEYS);
  });

  it("zeigt die Seite, wenn das Modul gebucht ist", () => {
    renderGate();

    expect(screen.getByText("Inhalt der Seite")).toBeInTheDocument();
  });

  it("zeigt Basis-Seiten auch ohne ein gebuchtes Modul", () => {
    vi.mocked(useBookedModules).mockReturnValue([]);
    navigation.pathname = "/students/search";

    renderGate();

    expect(screen.getByText("Inhalt der Seite")).toBeInTheDocument();
  });

  it("erklärt der Leitung ein nicht gebuchtes Modul und nennt das moto-Team", () => {
    auth.permissions = ["config:manage"];
    vi.mocked(useBookedModules).mockReturnValue(
      MODULE_KEYS.filter((key) => key !== "zeiterfassung"),
    );

    renderGate();

    expect(screen.queryByText("Inhalt der Seite")).not.toBeInTheDocument();
    // Ohne Aktionen der Seite: nichts lädt zu einer Handlung ein, die nicht
    // geht.
    expect(
      screen.queryByRole("button", { name: "Anlegen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Zeiterfassung" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Zeiterfassung ist nicht gebucht"),
    ).toBeInTheDocument();
    expect(screen.getByText(/wenden Sie sich an das moto-Team/)).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Gebuchte Module ansehen" }),
    ).toHaveAttribute("href", "/settings?tab=modules");
  });

  it("verweist eine Betreuungskraft an ihre Leitung, ohne Link in die Einstellungen", () => {
    vi.mocked(useBookedModules).mockReturnValue(
      MODULE_KEYS.filter((key) => key !== "zeiterfassung"),
    );

    renderGate();

    expect(screen.queryByText("Inhalt der Seite")).not.toBeInTheDocument();
    expect(
      screen.getByText(/sprechen Sie mit Ihrer Leitung/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Gebuchte Module ansehen" }),
    ).not.toBeInTheDocument();
  });

  it.each(["/dienstplan", "/dienstplan/details"])(
    "schützt die kanonische Modulseite %s",
    (pathname) => {
      vi.mocked(useBookedModules).mockReturnValue(
        MODULE_KEYS.filter((key) => key !== "dienstplan"),
      );
      navigation.pathname = pathname;

      renderGate("Dienstplan");

      expect(screen.getByText("Dienstplan ist nicht gebucht")).toBeVisible();
    },
  );

  it("rendert die Seite, solange die Schuldaten fehlen", () => {
    vi.mocked(useTenantSafe).mockReturnValue({
      tenantSlug: "demo",
      tenant: null,
      routingMode: "subdomain",
    });
    vi.mocked(useBookedModules).mockReturnValue([]);

    renderGate();

    expect(screen.getByText("Inhalt der Seite")).toBeInTheDocument();
  });
});
