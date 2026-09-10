"use client";

// Kategorien der Schule (#2131) als eigene Route der Datenverwaltung (#3114).
// Vorher lagen Liste und Formular in einem Slide-over, das der Termin-Assistent
// über sich selbst öffnete; die Fläche ist jetzt eine Sammlung wie Räume und
// Gruppen, und das Termin-Formular verlinkt nur noch hierher.

import { Suspense, useCallback, useMemo } from "react";
import { useSearchParams } from "next/navigation";

import {
  CatalogPage,
  CATALOG_SELECTION_PARAM,
  type CatalogConfig,
} from "~/components/database/catalog/catalog-page";
import { CatalogColorField } from "~/components/ui/database/catalog-color-field";
import type { SectionConfig } from "~/lib/database/types";
import type { ActivityCategory } from "~/lib/activity-helpers";
import { categoryService } from "~/lib/category-api";
import { formatCount } from "~/lib/format-utils";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";

const CACHE_KEY = "database-categories";
const DEFAULT_COLOR: string = LOCATION_COLORS.GROUP_ROOM;
/** Spiegelt maxCategoryNameLength / maxCategoryDescriptionLength in Go. */
const NAME_MAX_LENGTH = 60;
const DESCRIPTION_MAX_LENGTH = 255;

function usageLabel(category: ActivityCategory): string {
  const count = category.usageCount ?? 0;
  if (count === 0) return "Noch nicht verwendet";
  if (count === 1) return "In 1 Termin oder 1 Aktivität verwendet";
  return `In ${count} Terminen und Aktivitäten verwendet`;
}

function sections(): SectionConfig[] {
  return [
    {
      title: "Kategorie",
      fields: [
        {
          name: "name",
          label: "Name",
          type: "text",
          required: true,
          placeholder: "z. B. Essen",
          maxLength: NAME_MAX_LENGTH,
        },
        {
          name: "color",
          label: "Farbe",
          type: "custom",
          component: CatalogColorField,
          componentProps: {
            defaultHex: DEFAULT_COLOR,
            hint: "Die Farbe kennzeichnet die Kategorie im Betreuungsplan.",
          },
        },
        {
          name: "description",
          label: "Beschreibung",
          type: "textarea",
          placeholder: "Optional",
          maxLength: DESCRIPTION_MAX_LENGTH,
        },
      ],
    },
  ];
}

function payloadOf(values: Record<string, unknown>) {
  return {
    name: String(values.name ?? "").trim(),
    description: String(values.description ?? "").trim(),
    color: typeof values.color === "string" ? values.color : "",
  };
}

const config: CatalogConfig<ActivityCategory> = {
  concept: "activities",
  title: "Kategorien",
  singular: "Kategorie",
  purpose:
    "Kategorien ordnen Termine und Aktivitäten ein, zum Beispiel Essen, Lernzeit oder Freispiel.",
  searchPlaceholder: "Kategorie suchen…",
  emptyDescription:
    "Legen Sie eine Kategorie an, damit Termine und Aktivitäten sich einordnen lassen.",
  stats: (items) => {
    const archived = items.filter((item) => item.archivedAt).length;
    const active = items.length - archived;
    return archived > 0
      ? `${formatCount(active)} in der Auswahl · ${formatCount(archived)} archiviert`
      : `${formatCount(active)} ${active === 1 ? "Kategorie" : "Kategorien"}`;
  },
  toRow: (item) => ({
    name: item.name,
    subtitle: usageLabel(item),
    color: item.color ?? null,
    retired: Boolean(item.archivedAt),
  }),
  sections,
  toFormValues: (item) => ({
    name: item.name,
    description: item.description ?? "",
    color: item.color ?? null,
  }),
  createDefaults: { name: "", description: "", color: DEFAULT_COLOR },
  create: (values) => categoryService.createCategory(payloadOf(values)),
  update: (item, values) =>
    categoryService.updateCategory(item.id, payloadOf(values)),
  retire: {
    menuLabel: "Archivieren",
    confirmTitle: "Kategorie archivieren?",
    confirmLabel: "Archivieren",
    describe: (item) =>
      `Die Kategorie „${item.name}" wird für neue Termine und Aktivitäten nicht mehr angeboten. Bestehende Einträge behalten sie und bleiben gültig.`,
    run: (item) => categoryService.archiveCategory(item.id),
    toast: (item) => `Kategorie „${item.name}" archiviert`,
  },
  restore: {
    menuLabel: "Wieder anbieten",
    run: (item) => categoryService.restoreCategory(item.id),
    toast: (item) => `Kategorie „${item.name}" wird wieder angeboten`,
  },
};

function CategoriesPageContent() {
  const searchParams = useSearchParams();
  const tenantMutate = useTenantMutate();

  const {
    data,
    isLoading,
    error: loadError,
  } = useSWRAuth<ActivityCategory[]>(CACHE_KEY, () =>
    categoryService.getManagedCategories(),
  );

  const onChanged = useCallback(() => tenantMutate(CACHE_KEY), [tenantMutate]);

  const items = useMemo(
    () =>
      data
        ? [...data].sort((a, b) => a.name.localeCompare(b.name, "de"))
        : undefined,
    [data],
  );

  return (
    <CatalogPage
      config={config}
      items={items}
      isLoading={isLoading && data === undefined}
      error={
        loadError
          ? "Die Kategorien konnten nicht geladen werden. Bitte laden Sie die Seite neu."
          : null
      }
      onChanged={onChanged}
      // Die Route liegt hinter activities:manage_categories (database/layout);
      // wer sie öffnen darf, darf hier auch schreiben.
      canManage
      selectedId={searchParams.get(CATALOG_SELECTION_PARAM)}
    />
  );
}

export default function CategoriesPage() {
  return (
    <Suspense fallback={null}>
      <CategoriesPageContent />
    </Suspense>
  );
}
