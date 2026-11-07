import { Suspense } from "react";
import { act, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { TEST_CLOCK_INSTANT } from "~/test/clock";

const mocks = vi.hoisted(() => ({
  search: new URLSearchParams(),
  fetchEnrollmentPreviewBootstrap: vi.fn(),
  listPhases: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => mocks.search,
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({
    tenantSlug: "demo",
    tenant: { name: "Demo School", gradeLevelMax: 4 },
  }),
  useTenantSlugSafe: () => "demo",
  useTenantRoutingModeSafe: () => "path",
}));

vi.mock("~/lib/enrollment-form-schema-api", () => ({
  fetchEnrollmentPreviewBootstrap: mocks.fetchEnrollmentPreviewBootstrap,
  schemaToPublicFormSchema: vi.fn(),
}));

vi.mock("~/lib/enrollment-phase-api", () => ({
  listPhases: mocks.listPhases,
}));

vi.mock("~/components/enrollment/enrollment-form", () => ({
  EnrollmentForm: () => null,
}));

vi.mock("~/components/enrollment/public-enrollment-shell", () => ({
  PublicEnrollmentBrand: () => null,
  PublicEnrollmentLocaleSwitcher: () => null,
  PublicEnrollmentPageShell: ({ children }: { children: React.ReactNode }) => (
    <main>{children}</main>
  ),
  PublicEnrollmentSteps: () => null,
  PublicInfoCard: ({
    title,
    children,
  }: {
    title: string;
    children: React.ReactNode;
  }) => (
    <section aria-label={title}>
      <h3>{title}</h3>
      {children}
    </section>
  ),
}));

import EnrollmentPreviewPage from "./page";

async function renderPage() {
  await act(async () => {
    render(
      <Suspense fallback={null}>
        <EnrollmentPreviewPage />
      </Suspense>,
    );
  });
}

describe("EnrollmentPreviewPage", () => {
  beforeEach(() => {
    mocks.search = new URLSearchParams();
    mocks.fetchEnrollmentPreviewBootstrap.mockReset();
    mocks.listPhases.mockReset();
    mocks.listPhases.mockResolvedValue([]);
  });

  // Der Rückweg zeigte auf `/enrollment-form` ohne Schule und landete im
  // Pfad-Modus außerhalb des Tenant-Portals (#4364).
  it("links back to the enrollment forms of the same school", async () => {
    await renderPage();

    expect(
      await screen.findByRole("link", {
        name: "Zurück zu den Anmeldeformularen",
      }),
    ).toHaveAttribute("href", "/demo/enrollment-form");
  });

  // #6044: a school created offerings, opened this template preview and
  // found no offerings. The page pointed to a „Phasenvorschau“ that exists
  // nowhere. It must lead to the parents' view of each phase that uses the
  // template, where the offerings are.
  it("links to the parents' view of every phase that uses the template", async () => {
    mocks.search = new URLSearchParams("schemaId=80");
    mocks.fetchEnrollmentPreviewBootstrap.mockResolvedValue({
      schema: { id: "80", name: "Anmeldung Ganztag", fields: [] },
      assigned_phase_count: 3,
      active_assigned_phase_count: 3,
    });
    mocks.listPhases.mockResolvedValue([
      {
        id: "13",
        name: "2. Halbjahr 2026/27",
        form_schema_id: "80",
        is_active: true,
      },
      {
        id: "14",
        name: "Schuljahr 2027/28",
        form_schema_id: "80",
        is_active: true,
      },
      {
        id: "15",
        name: "Wiederanmeldung",
        form_schema_id: "80",
        audience: "existing_students",
        is_active: true,
      },
      { id: "16", name: "Ferien", form_schema_id: "99" },
    ]);

    await renderPage();

    expect(
      await screen.findByRole("link", {
        name: "Elternansicht von „2. Halbjahr 2026/27“ öffnen",
      }),
    ).toHaveAttribute("href", "/demo/anmeldung/13");
    expect(
      screen.getByRole("link", {
        name: "Elternansicht von „Schuljahr 2027/28“ öffnen",
      }),
    ).toHaveAttribute("href", "/demo/anmeldung/14");
    // A phase only invited families reach has no public link to open.
    expect(screen.getByText("Wiederanmeldung")).toBeVisible();
    expect(
      screen.queryByRole("link", {
        name: "Elternansicht von „Wiederanmeldung“ öffnen",
      }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Ferien")).not.toBeInTheDocument();
    expect(
      screen.getAllByText(
        "Diese Vorlage wird in 3 aktiven Anmeldephasen verwendet.",
        { exact: false },
      ).length,
    ).toBeGreaterThan(0);
    expect(screen.queryByText(/Phasenvorschau/)).not.toBeInTheDocument();
  });

  it.each([
    {
      state: "inactive",
      phase: { is_active: false },
      hint: "Schalten Sie unter Anmeldephasen bei dieser Phase „Aktiv“ ein.",
    },
    {
      state: "not yet open",
      phase: {
        enrollment_open_at: new Date(
          TEST_CLOCK_INSTANT.getTime() + 60_000,
        ).toISOString(),
      },
      hint: "Die Elternansicht ist erst ab der Öffnung erreichbar.",
    },
    {
      state: "at closing time",
      phase: { enrollment_close_at: TEST_CLOCK_INSTANT.toISOString() },
      hint: "Das Anmeldefenster ist geschlossen.",
    },
    {
      state: "already closed",
      phase: {
        enrollment_close_at: new Date(
          TEST_CLOCK_INSTANT.getTime() - 60_000,
        ).toISOString(),
      },
      hint: "Das Anmeldefenster ist geschlossen.",
    },
  ])(
    "explains why a $state phase has no parent-view link",
    async ({ phase, hint }) => {
      mocks.search = new URLSearchParams("schemaId=80");
      mocks.fetchEnrollmentPreviewBootstrap.mockResolvedValue({
        schema: { id: "80", name: "Anmeldung Ganztag", fields: [] },
        assigned_phase_count: 1,
        active_assigned_phase_count: phase.is_active === false ? 0 : 1,
      });
      mocks.listPhases.mockResolvedValue([
        {
          id: "13",
          name: "2. Halbjahr 2026/27",
          form_schema_id: "80",
          is_active: true,
          ...phase,
        },
      ]);

      await renderPage();

      // Anker: die zugeordnete Phase ist mit ihrer Voraussetzung geladen.
      expect(await screen.findByText(hint, { exact: false })).toBeVisible();
      expect(screen.getByText("2. Halbjahr 2026/27")).toBeVisible();
      expect(
        screen.queryByRole("link", {
          name: "Elternansicht von „2. Halbjahr 2026/27“ öffnen",
        }),
      ).not.toBeInTheDocument();
    },
  );

  it("offers the parent view from opening until, but not including, closing", async () => {
    mocks.search = new URLSearchParams("schemaId=80");
    mocks.fetchEnrollmentPreviewBootstrap.mockResolvedValue({
      schema: { id: "80", name: "Anmeldung Ganztag", fields: [] },
      assigned_phase_count: 1,
      active_assigned_phase_count: 1,
    });
    mocks.listPhases.mockResolvedValue([
      {
        id: "13",
        name: "2. Halbjahr 2026/27",
        form_schema_id: "80",
        is_active: true,
        enrollment_open_at: TEST_CLOCK_INSTANT.toISOString(),
        enrollment_close_at: new Date(
          TEST_CLOCK_INSTANT.getTime() + 60_000,
        ).toISOString(),
      },
    ]);

    await renderPage();

    expect(
      await screen.findByRole("link", {
        name: "Elternansicht von „2. Halbjahr 2026/27“ öffnen",
      }),
    ).toHaveAttribute("href", "/demo/anmeldung/13");
  });

  it("says where to assign a template no phase uses", async () => {
    mocks.search = new URLSearchParams("schemaId=80");
    mocks.fetchEnrollmentPreviewBootstrap.mockResolvedValue({
      schema: { id: "80", name: "Anmeldung Ganztag", fields: [] },
      assigned_phase_count: 0,
      active_assigned_phase_count: 0,
    });

    await renderPage();

    expect(
      await screen.findByText(
        "Diese Vorlage gehört noch zu keiner Anmeldephase.",
        { exact: false },
      ),
    ).toBeVisible();
    expect(
      screen.queryByRole("link", { name: /Elternansicht von/ }),
    ).not.toBeInTheDocument();
  });
});
