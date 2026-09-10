import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import CategoriesPage from "./page";
interface CapturedConfig {
  isReadOnly: (item: { isSystem?: boolean; archivedAt?: string }) => boolean;
  update: (item: { id: string }, values: Record<string, unknown>) => unknown;
}

const captured = vi.hoisted(() => ({
  config: undefined as CapturedConfig | undefined,
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("~/components/database/catalog/catalog-page", () => ({
  CatalogPage: ({ config }: { config: unknown }) => {
    captured.config = config as CapturedConfig;
    return null;
  },
  CATALOG_SELECTION_PARAM: "eintrag",
}));

const categoryService = vi.hoisted(() => ({
  getManagedCategories: vi.fn(),
  createCategory: vi.fn(),
  updateCategory: vi.fn(),
  archiveCategory: vi.fn(),
  restoreCategory: vi.fn(),
}));
vi.mock("~/lib/category-api", () => ({
  categoryService,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({ data: [], isLoading: false, error: undefined }),
  useTenantMutate: () => vi.fn(),
}));

describe("Terminkategorien", () => {
  beforeEach(() => {
    captured.config = undefined;
    vi.clearAllMocks();
  });

  it("treats system categories as read-only", () => {
    render(<CategoriesPage />);

    expect(
      captured.config?.isReadOnly({
        isSystem: true,
      }),
    ).toBe(true);
  });

  it("clears the stored color after resetting an existing category", () => {
    render(<CategoriesPage />);

    captured.config?.update(
      { id: "7" },
      { name: "Essen", description: "Mittagessen", color: null },
    );

    expect(categoryService.updateCategory).toHaveBeenCalledWith("7", {
      name: "Essen",
      description: "Mittagessen",
      color: "",
    });
  });
});
