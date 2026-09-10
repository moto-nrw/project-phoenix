import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  CatalogPage,
  type CatalogConfig,
} from "~/components/database/catalog/catalog-page";
import type { SectionConfig } from "~/lib/database/types";

const mockUpdateUrlParams = vi.hoisted(() => vi.fn());
vi.mock("~/hooks/useUpdateUrlParams", () => ({
  useUpdateUrlParams: () => mockUpdateUrlParams,
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/test-tenant/database/categories",
  useSearchParams: () => new URLSearchParams(),
}));

const toastSuccess = vi.hoisted(() => vi.fn());
const toastError = vi.hoisted(() => vi.fn());
const toastWarning = vi.hoisted(() => vi.fn());
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: toastSuccess,
    error: toastError,
    warning: toastWarning,
    info: vi.fn(),
    remove: vi.fn(),
  }),
}));

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: () => false,
}));

interface Sorte {
  readonly id: string;
  readonly name: string;
  readonly archived: boolean;
  readonly sortOrder: number;
}

const ITEMS: Sorte[] = [
  { id: "1", name: "Essen", archived: false, sortOrder: 0 },
  { id: "2", name: "Lernzeit", archived: false, sortOrder: 1 },
  { id: "3", name: "Altes", archived: true, sortOrder: 2 },
];

const sections = (): SectionConfig[] => [
  {
    title: "Eintrag",
    fields: [{ name: "name", label: "Name", type: "text", required: true }],
  },
];

function buildConfig(overrides: Partial<CatalogConfig<Sorte>> = {}) {
  const create = vi.fn().mockResolvedValue(undefined);
  const update = vi.fn().mockResolvedValue(undefined);
  const retire = vi.fn().mockResolvedValue(undefined);
  const restore = vi.fn().mockResolvedValue(undefined);
  const remove = vi.fn().mockResolvedValue(undefined);
  const reorder = vi.fn().mockResolvedValue(undefined);

  const config: CatalogConfig<Sorte> = {
    concept: "activities",
    title: "Kategorien",
    singular: "Kategorie",
    purpose: "Kategorien ordnen Termine ein.",
    searchPlaceholder: "Kategorie suchen…",
    emptyDescription: "Legen Sie eine Kategorie an.",
    stats: (items) => `${items.length} Einträge`,
    toRow: (item) => ({
      name: item.name,
      subtitle: item.archived ? "Archiviert" : "In der Auswahl",
      retired: item.archived,
    }),
    sections,
    toFormValues: (item) => ({ name: item.name }),
    createDefaults: { name: "" },
    create,
    update,
    retire: {
      menuLabel: "Archivieren",
      confirmTitle: "Kategorie archivieren?",
      confirmLabel: "Archivieren",
      describe: (item) => `„${item.name}" wird nicht mehr angeboten.`,
      run: retire,
      toast: (item) => `„${item.name}" archiviert`,
    },
    restore: {
      menuLabel: "Wieder anbieten",
      run: restore,
      toast: (item) => `„${item.name}" wird wieder angeboten`,
    },
    remove: {
      menuLabel: "Löschen",
      confirmTitle: "Kategorie löschen",
      describe: (item) => `„${item.name}" wird gelöscht.`,
      run: remove,
      toast: (item) => `„${item.name}" gelöscht`,
    },
    reorder,
    ...overrides,
  };

  return { config, create, update, retire, restore, remove, reorder };
}

function renderPage(
  options: {
    selectedId?: string | null;
    items?: Sorte[];
    configOverrides?: Partial<CatalogConfig<Sorte>>;
    canManage?: boolean;
    onChanged?: () => Promise<unknown>;
  } = {},
) {
  const built = buildConfig(options.configOverrides);
  const onChanged = options.onChanged ?? vi.fn().mockResolvedValue(undefined);
  render(
    <CatalogPage
      config={built.config}
      items={options.items ?? ITEMS}
      isLoading={false}
      error={null}
      onChanged={onChanged}
      canManage={options.canManage ?? true}
      selectedId={options.selectedId ?? null}
    />,
  );
  return { ...built, onChanged };
}

/** Die Zeile eines Eintrags: der Knopf um seinen Namen, nicht die Pfeile
 *  daneben („Essen nach oben"). */
function row(name: string): HTMLElement {
  const label = screen.getByText(name);
  const button = label.closest("button");
  if (!button) throw new Error(`Keine Zeile für „${name}" gefunden`);
  return button;
}

/** Das Gerüst rendert die Kopfkarte für Telefon und Schreibtisch; beide
 *  Suchfelder hängen am selben Zustand. */
function search(): HTMLElement {
  const fields = screen.getAllByPlaceholderText("Kategorie suchen…");
  const first = fields[0];
  if (!first) throw new Error("Kein Suchfeld gefunden");
  return first;
}

beforeEach(() => {
  vi.stubGlobal(
    "matchMedia",
    vi.fn((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

// Die gemeinsame Fläche der Stammdaten-Kataloge (#3114): Sammlung links,
// Objektansicht rechts, Anlegen als Kopf-Aktion.
describe("CatalogPage", () => {
  it("splits the collection into what is offered and what is not", () => {
    renderPage();

    expect(row("Essen")).toBeInTheDocument();
    expect(screen.getByText("Nicht mehr in der Auswahl")).toBeInTheDocument();
    expect(row("Altes")).toBeInTheDocument();
  });

  it("puts the chosen entry in the address so a reload shows it again", () => {
    renderPage();

    fireEvent.click(row("Essen"));

    expect(mockUpdateUrlParams).toHaveBeenCalledWith({ eintrag: "1" });
  });

  it("filters the collection by the search", () => {
    renderPage();

    fireEvent.change(search(), { target: { value: "lern" } });

    expect(screen.queryByText("Essen")).not.toBeInTheDocument();
    expect(row("Lernzeit")).toBeInTheDocument();
  });

  it("shows the catalog's purpose above its collection", () => {
    renderPage();

    expect(screen.getByText("Kategorien ordnen Termine ein.")).toBeVisible();
  });

  it("keeps the open entry while the search hides its row", () => {
    renderPage({ selectedId: "1" });

    fireEvent.change(search(), { target: { value: "lern" } });

    // Die Objektansicht bleibt: sie hängt am Eintrag, nicht an der Zeile.
    expect(screen.getByDisplayValue("Essen")).toBeInTheDocument();
  });

  it("saves an edit at the object", async () => {
    const { update, onChanged } = renderPage({ selectedId: "1" });

    fireEvent.change(screen.getByDisplayValue("Essen"), {
      target: { value: "Mittagessen" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(update).toHaveBeenCalledWith(
        ITEMS[0],
        expect.objectContaining({ name: "Mittagessen" }),
      ),
    );
    expect(onChanged).toHaveBeenCalled();
  });

  it("keeps a saved edit successful when the refresh fails", async () => {
    const onChanged = vi.fn().mockRejectedValue(new Error("nicht erreichbar"));
    const { update } = renderPage({ selectedId: "1", onChanged });

    fireEvent.change(screen.getByDisplayValue("Essen"), {
      target: { value: "Mittagessen" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(update).toHaveBeenCalled());
    expect(toastError).not.toHaveBeenCalled();
    expect(toastWarning).toHaveBeenCalledWith(
      "Die Änderung wurde gespeichert. Laden Sie die Seite neu.",
    );
  });

  it("asks before taking an entry out of the selection", async () => {
    const { retire } = renderPage({ selectedId: "1" });

    fireEvent.click(screen.getByRole("button", { name: /Aktionen für Essen/ }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Archivieren" }));

    expect(await screen.findByText(/wird nicht mehr angeboten/)).toBeTruthy();
    expect(retire).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Archivieren" }));

    await waitFor(() => expect(retire).toHaveBeenCalledWith(ITEMS[0]));
  });

  it("keeps a completed archive successful when the refresh fails", async () => {
    const onChanged = vi.fn().mockRejectedValue(new Error("nicht erreichbar"));
    const { retire } = renderPage({ selectedId: "1", onChanged });

    fireEvent.click(screen.getByRole("button", { name: /Aktionen für Essen/ }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Archivieren" }));
    fireEvent.click(screen.getByRole("button", { name: "Archivieren" }));

    await waitFor(() => expect(retire).toHaveBeenCalledWith(ITEMS[0]));
    expect(toastError).not.toHaveBeenCalled();
    expect(toastWarning).toHaveBeenCalledWith(
      "Die Änderung wurde gespeichert. Laden Sie die Seite neu.",
    );
  });

  it("offers a retired entry back", async () => {
    const { restore } = renderPage({ selectedId: "3" });

    fireEvent.click(screen.getByRole("button", { name: /Aktionen für Altes/ }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Wieder anbieten" }));

    await waitFor(() => expect(restore).toHaveBeenCalledWith(ITEMS[2]));
  });

  it("requires restoration before an archived read-only entry can be edited", async () => {
    const { restore } = renderPage({
      selectedId: "3",
      configOverrides: {
        isReadOnly: (item) => item.archived,
        readOnlyHint:
          "Bieten Sie die Kategorie wieder an. Dann können Sie sie ändern.",
      },
    });

    expect(screen.queryByDisplayValue("Altes")).not.toBeInTheDocument();
    expect(
      screen.getAllByText(
        "Bieten Sie die Kategorie wieder an. Dann können Sie sie ändern.",
      ),
    ).not.toHaveLength(0);

    fireEvent.click(screen.getByRole("button", { name: /Aktionen für Altes/ }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Wieder anbieten" }));

    await waitFor(() => expect(restore).toHaveBeenCalledWith(ITEMS[2]));
  });

  it("reorders through the visible arrows", async () => {
    const { reorder } = renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Lernzeit nach oben" }));

    await waitFor(() => expect(reorder).toHaveBeenCalledWith(["2", "1"]));
  });

  it("keeps hidden active entries in the reorder payload", async () => {
    const { reorder } = renderPage({
      items: [
        { id: "1", name: "Essen", archived: false, sortOrder: 0 },
        { id: "2", name: "Lernzeit A", archived: false, sortOrder: 1 },
        { id: "3", name: "Lernzeit B", archived: false, sortOrder: 2 },
      ],
    });

    fireEvent.change(search(), { target: { value: "lernzeit" } });
    fireEvent.click(
      screen.getByRole("button", { name: "Lernzeit B nach oben" }),
    );

    await waitFor(() => expect(reorder).toHaveBeenCalledWith(["1", "3", "2"]));
  });

  it("carries no reorder arrows when a single entry has no order", () => {
    const single = ITEMS[0];
    if (!single) throw new Error("Beispieldaten fehlen");
    renderPage({ items: [single] });

    // Zwei dauerhaft graue Pfeile neben dem einzigen Eintrag sind
    // Bedienelemente, die nie etwas tun.
    expect(
      screen.queryByRole("button", { name: /nach oben/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /nach unten/ }),
    ).not.toBeInTheDocument();
  });

  it("hides every write affordance without the permission", () => {
    renderPage({ selectedId: "1", canManage: false });

    expect(
      screen.queryByRole("button", { name: "Kategorie anlegen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Aktionen für Essen/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByDisplayValue("Essen")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Essen/ })).toBeInTheDocument();
  });

  it("shows a load error instead of an empty collection", () => {
    render(
      <CatalogPage
        config={buildConfig().config}
        items={undefined}
        isLoading={false}
        error="Die Kategorien konnten nicht geladen werden."
        onChanged={vi.fn()}
        canManage
        selectedId={null}
      />,
    );

    expect(
      screen.getByText("Die Kategorien konnten nicht geladen werden."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Noch keine Kategorien")).not.toBeInTheDocument();
  });

  it("offers the next step when nothing is there yet", () => {
    renderPage({ items: [] });

    expect(screen.getByText("Noch keine Kategorien")).toBeInTheDocument();
    expect(
      screen.getByText("Legen Sie eine Kategorie an."),
    ).toBeInTheDocument();
  });
});
