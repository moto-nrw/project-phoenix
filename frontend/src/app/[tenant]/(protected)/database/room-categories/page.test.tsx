import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import RoomCategoriesPage from "./page";

interface Category {
  id: string;
  name: string;
  color: string;
  roomCount: number;
}

interface CapturedConfig {
  createDefaults: Record<string, unknown>;
  toRow: (item: Category) => { subtitle?: string };
  create: (values: Record<string, unknown>) => unknown;
  update: (item: Category, values: Record<string, unknown>) => unknown;
  remove: { run: (item: Category) => unknown };
}

const captured = vi.hoisted(() => ({
  config: undefined as CapturedConfig | undefined,
}));
const listed = vi.hoisted(() => ({ data: [] as Category[] }));

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

const roomCategoryService = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
}));
vi.mock("~/lib/room-category-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/room-category-api")>()),
  roomCategoryService,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({ data: listed.data, isLoading: false, error: undefined }),
  useTenantMutate: () => vi.fn(),
  useTenantMutateMatching: () => vi.fn(),
}));

const sport: Category = {
  id: "4",
  name: "Sport",
  color: MOTO_COLOR_PALETTE.wine.base,
  roomCount: 2,
};

describe("Raumkategorien", () => {
  beforeEach(() => {
    captured.config = undefined;
    listed.data = [sport];
    vi.clearAllMocks();
  });

  it("suggests a color no category carries yet", () => {
    render(<RoomCategoriesPage />);

    expect(captured.config?.createDefaults.color).toBe(
      MOTO_COLOR_PALETTE.gold.base,
    );
  });

  it("names how many rooms use a category", () => {
    render(<RoomCategoriesPage />);

    expect(captured.config?.toRow(sport).subtitle).toBe("2 Räume");
    expect(captured.config?.toRow({ ...sport, roomCount: 0 }).subtitle).toBe(
      "Noch kein Raum",
    );
  });

  it("keeps the stored color when the picker is reset", () => {
    render(<RoomCategoriesPage />);

    captured.config?.update(sport, { name: " Bewegung ", color: null });

    expect(roomCategoryService.update).toHaveBeenCalledWith("4", {
      name: "Bewegung",
      color: MOTO_COLOR_PALETTE.wine.base,
    });
  });

  it("creates with the chosen color and deletes by id", () => {
    render(<RoomCategoriesPage />);

    captured.config?.create({ name: "Ruhe", color: "#006064" });
    captured.config?.remove.run(sport);

    expect(roomCategoryService.create).toHaveBeenCalledWith({
      name: "Ruhe",
      color: "#006064",
    });
    expect(roomCategoryService.remove).toHaveBeenCalledWith("4");
  });
});
