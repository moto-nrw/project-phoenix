import { act, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import { useInvalidateMatching } from "~/lib/swr";
import { SWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { staffQualificationKeys } from "~/lib/swr/staff-keys";
import { ToastProvider } from "~/contexts/ToastContext";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import type { FormErrorInput } from "~/components/ui/form-error";
import CategoriesPage from "~/app/[tenant]/(protected)/database/categories/page";
import PlanningTracksPage from "~/app/[tenant]/(protected)/database/planning-tracks/page";
import ShiftTypesPage from "~/app/[tenant]/(protected)/database/shift-types/page";
import AbsenceTypesPage from "~/app/[tenant]/(protected)/database/absence-types/page";
import QualificationTypesPage from "~/app/[tenant]/(protected)/database/qualification-types/page";

vi.unmock("swr");
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "A", tenantId: 1, scope: "tenant", permissions: ["*"] },
    },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
  useTimetableEnabled: () => true,
  useModuleBooked: () => true,
}));
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));
vi.mock("~/lib/session-cache", () => ({ announceSessionChange: vi.fn() }));
vi.mock("~/components/tenant/module-route-guard", () => ({
  ModuleRouteGuard: ({ children }: { children: ReactNode }) => children,
}));
vi.mock("~/components/database/catalog/catalog-page", () => ({
  CATALOG_SELECTION_PARAM: "eintrag",
  CatalogPage: ({
    items,
    error,
  }: {
    items?: { id: string; name: string }[];
    error: FormErrorInput;
  }) => (
    <div>
      <LoadErrorAlert error={error} />
      {items?.map((item) => (
        <p key={item.id}>{item.name}</p>
      ))}
    </div>
  ),
}));
const { read } = vi.hoisted(() => ({ read: vi.fn() }));
vi.mock("~/lib/category-api", () => ({
  categoryService: { getManagedCategories: read },
}));
vi.mock("~/lib/planning-track-api", () => ({
  planningTrackService: { list: read },
}));
vi.mock("~/lib/shift-type-api", () => ({
  shiftTypeService: { getShiftTypes: read },
}));
vi.mock("~/lib/absence-type-api", () => ({
  absenceTypeService: { getAbsenceTypes: read },
}));
vi.mock("~/lib/staff-qualification-api", () => ({
  staffQualificationService: { listTypes: read },
}));

const item = {
  id: "1",
  name: "Bestätigter Eintrag",
  active: true,
  archivedAt: null,
  sortOrder: 0,
};

describe.each([
  ["categories", CategoriesPage, "database-categories"],
  ["planning tracks", PlanningTracksPage, "database-planning-tracks"],
  ["shift types", ShiftTypesPage, "database-shift-types"],
  ["absence types", AbsenceTypesPage, "database-absence-types"],
  [
    "qualification types",
    QualificationTypesPage,
    staffQualificationKeys.types(),
  ],
] as const)("catalog access: %s", (_name, Page, key) => {
  it.each([403, 503])("handles a %s after a confirmed read", async (status) => {
    read.mockReset().mockResolvedValue([item]);
    const cache = new Map();
    let invalidate!: ReturnType<typeof useInvalidateMatching>;
    function Refresh() {
      invalidate = useInvalidateMatching();
      return null;
    }
    const view = render(
      <SWRConfig value={{ provider: () => cache, shouldRetryOnError: false }}>
        <ToastProvider>
          <SWRScopeBoundary />
          <Refresh />
          <Page />
        </ToastProvider>
      </SWRConfig>,
    );
    expect(await screen.findByText(item.name)).toBeVisible();
    read.mockRejectedValue(new ApiError("failed", status));
    await act(async () => {
      await invalidate((candidate) => candidate === key);
    });
    // Anker: der Katalog zeigt den Fehler des abgeschlossenen Abrufs.
    expect(await screen.findByRole("alert")).toBeVisible();
    if (status === 403) {
      await waitFor(() =>
        expect(screen.queryByText(item.name)).not.toBeInTheDocument(),
      );
      const entry = [...cache.entries()].find(([candidate]) =>
        candidate.endsWith(`:${key}`),
      )?.[1];
      expect(entry?.data).toBeUndefined();
    } else expect(screen.getByText(item.name)).toBeVisible();
    view.unmount();
  });
});
