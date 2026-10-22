"use client";

// Raumkategorien der Schule (#4352). Die Farbe der Ortsangabe eines Kindes
// hing vorher an jedem einzelnen Raum: elf Räume, elf Farben, ohne dass eine
// Farbe etwas bedeutete. Jetzt trägt die Kategorie die Farbe, und jeder Raum
// dieser Kategorie zeigt sie, solange er keine eigene Farbe hat. Die
// Statusfarben (eigener Gruppenraum, Unterwegs, Schulhof) gehen weiter vor.

import { Suspense, useCallback, useMemo } from "react";
import { useSearchParams } from "next/navigation";

import {
  CatalogPage,
  CATALOG_SELECTION_PARAM,
  type CatalogConfig,
} from "~/components/database/catalog/catalog-page";
import { CatalogColorField } from "~/components/ui/database/catalog-color-field";
import type { SectionConfig } from "~/lib/database/types";
import { formatCount } from "~/lib/format-utils";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  roomCategoryService,
  suggestedRoomCategoryColor,
  type RoomCategory,
} from "~/lib/room-category-api";
import {
  useSWRAuth,
  useTenantMutate,
  useTenantMutateMatching,
} from "~/lib/swr";
import {
  ROOM_DERIVED_CACHE_KEY_FRAGMENTS,
  ROOM_LIST_CACHE_KEYS,
} from "~/lib/swr/room-derived-caches";

const CACHE_KEY = "database-room-categories";
/** Spiegelt MaxRoomCategoryNameLength in Go. */
const NAME_MAX_LENGTH = 60;

function usageLabel(category: RoomCategory): string {
  if (category.roomCount === 0) return "Noch kein Raum";
  if (category.roomCount === 1) return "1 Raum";
  return `${formatCount(category.roomCount)} Räume`;
}

function describeDeletion(category: RoomCategory) {
  return (
    <>
      Die Raumkategorie <strong>{category.name}</strong> wird gelöscht. Das geht
      nur, wenn kein Raum sie mehr nutzt.
    </>
  );
}

function RoomCategoriesPageContent() {
  const searchParams = useSearchParams();
  const tenantMutate = useTenantMutate();

  const {
    data,
    isLoading,
    error: swrError,
    mutate,
  } = useSWRAuth<RoomCategory[]>(CACHE_KEY, () => roomCategoryService.list());
  // Ladefehler mit Katalogtext und Wiederholen statt eines eigenen Satzes.
  const loadError = useSwrLoadError(
    swrError,
    "die Liste der Raumkategorien",
    () => mutate(),
  );

  // Raumlisten und Ortsangaben der Kinder tragen die Kategoriefarbe mit;
  // nach einer Änderung laden sie neu, sonst bliebe die alte Farbe stehen.
  const refreshRoomConsumers = useTenantMutateMatching(
    ROOM_DERIVED_CACHE_KEY_FRAGMENTS,
  );
  const onChanged = useCallback(
    () =>
      Promise.all([
        tenantMutate(CACHE_KEY),
        ...ROOM_LIST_CACHE_KEYS.map((key) => tenantMutate(key)),
        refreshRoomConsumers(),
      ]),
    [tenantMutate, refreshRoomConsumers],
  );

  const items = useMemo(
    () =>
      data
        ? [...data].sort((a, b) => a.name.localeCompare(b.name, "de"))
        : undefined,
    [data],
  );

  // Die vorgeschlagene Farbe hängt am Bestand, deshalb wird die
  // Konfiguration hier gebaut.
  const config = useMemo<CatalogConfig<RoomCategory>>(() => {
    const newColor = suggestedRoomCategoryColor(items ?? []);
    const sections = (): SectionConfig[] => [
      {
        title: "Raumkategorie",
        fields: [
          {
            name: "name",
            label: "Name",
            type: "text",
            required: true,
            placeholder: "z. B. Bewegung",
            maxLength: NAME_MAX_LENGTH,
          },
          {
            name: "color",
            label: "Farbe",
            type: "custom",
            component: CatalogColorField,
            componentProps: {
              defaultHex: newColor,
              hint: "Diese Farbe zeigen alle Räume der Kategorie, wenn sie keine eigene Farbe haben.",
            },
          },
        ],
      },
    ];
    const payloadOf = (
      values: Record<string, unknown>,
      fallback: RoomCategory | null,
    ) => ({
      name: String(values.name ?? "").trim(),
      // „Zurücksetzen“ leert das Feld; eine Kategorie braucht aber immer eine
      // Farbe, also bleibt die bisherige oder die vorgeschlagene.
      color:
        typeof values.color === "string" && values.color
          ? values.color
          : (fallback?.color ?? newColor),
    });

    return {
      concept: "rooms",
      title: "Raumkategorien",
      singular: "Raumkategorie",
      purpose:
        "Räume derselben Kategorie haben dieselbe Farbe. Die Farbe steht bei der Ortsangabe der Kinder.",
      searchPlaceholder: "Raumkategorie suchen…",
      emptyDescription:
        "Legen Sie eine Raumkategorie an. Danach wählen Sie sie bei jedem Raum aus.",
      stats: (categories) =>
        `${formatCount(categories.length)} ${categories.length === 1 ? "Raumkategorie" : "Raumkategorien"}`,
      toRow: (category) => ({
        name: category.name,
        subtitle: usageLabel(category),
        color: category.color,
      }),
      sections,
      toFormValues: (category) => ({
        name: category.name,
        color: category.color,
      }),
      createDefaults: { name: "", color: newColor },
      // Ohne geladene Liste wäre die vorgeschlagene Farbe geraten.
      createDisabled: items === undefined,
      create: (values) => roomCategoryService.create(payloadOf(values, null)),
      update: (category, values) =>
        roomCategoryService.update(category.id, payloadOf(values, category)),
      remove: {
        menuLabel: "Löschen",
        confirmTitle: "Raumkategorie löschen",
        describe: describeDeletion,
        run: (category) => roomCategoryService.remove(category.id),
        toast: (category) =>
          `Die Raumkategorie „${category.name}“ ist gelöscht.`,
      },
    };
  }, [items]);

  return (
    <CatalogPage
      config={config}
      items={items}
      // Bis der Katalogtext des Ladefehlers da ist, bleibt das Skelett
      // stehen: sonst blitzt der Leerzustand auf.
      isLoading={
        (isLoading && data === undefined) ||
        (swrError !== undefined && loadError === null)
      }
      error={loadError}
      onChanged={onChanged}
      // Die Route liegt hinter rooms:manage (database/layout).
      canManage
      selectedId={searchParams.get(CATALOG_SELECTION_PARAM)}
    />
  );
}

export default function RoomCategoriesPage() {
  return (
    <Suspense fallback={null}>
      <RoomCategoriesPageContent />
    </Suspense>
  );
}
