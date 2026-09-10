"use client";

// Schichtarten (#1836) als eigene Route der Datenverwaltung (#3114). Vorher
// lagen Liste und Formular in einem Slide-over, das der Dienstplan öffnete.
// Der Dienstplan verlinkt jetzt hierher.

import { Suspense, useCallback, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";

import {
  CatalogPage,
  CATALOG_SELECTION_PARAM,
  type CatalogConfig,
} from "~/components/database/catalog/catalog-page";
import { Button } from "~/components/ui/button";
import { CatalogColorField } from "~/components/ui/database/catalog-color-field";
import { useToast } from "~/contexts/ToastContext";
import { getCategories } from "~/lib/activity-api";
import type { ActivityCategory } from "~/lib/activity-helpers";
import type { SectionConfig } from "~/lib/database/types";
import { formatCount } from "~/lib/format-utils";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { shiftTypeService } from "~/lib/shift-type-api";
import type { ShiftType } from "~/lib/shift-type-helpers";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";

const logger = createLogger({ component: "DatabaseShiftTypesPage" });

const CACHE_KEY = "database-shift-types";
const CATEGORY_CACHE_KEY = "database-shift-type-categories";
const DEFAULT_COLOR: string = LOCATION_COLORS.GROUP_ROOM;

/** Steht außerhalb der Seite, damit die Rückfrage nicht als bei jedem Render
 *  neu erzeugte Komponente gilt. */
function describeDeletion(type: ShiftType) {
  return (
    <>
      Die Schichtart <strong>{type.name}</strong> wird gelöscht. Bereits
      geplante Schichten bleiben erhalten, verlieren aber diese Kennzeichnung.
    </>
  );
}

function ShiftTypesPageContent() {
  const searchParams = useSearchParams();
  const tenantMutate = useTenantMutate();
  const toast = useToast();
  const [seeding, setSeeding] = useState(false);

  const {
    data,
    isLoading,
    error: loadError,
  } = useSWRAuth<ShiftType[]>(CACHE_KEY, () =>
    shiftTypeService.getShiftTypes(),
  );

  // Die Kategorien tragen ihre aktuelle Schichtart; daraus kommt die
  // Vorauswahl. Solange sie nicht geladen sind, zeigt das Formular das Feld
  // gar nicht und die Speichern-Aufrufe lassen `categoryIds` weg — eine leere
  // Liste würde bestehende Zuordnungen löschen (#1837).
  const { data: categories, error: categoriesError } = useSWRAuth<
    ActivityCategory[]
  >(CATEGORY_CACHE_KEY, () => getCategories());
  const categoriesReady = categories !== undefined && !categoriesError;

  const onChanged = useCallback(
    () =>
      Promise.all([tenantMutate(CACHE_KEY), tenantMutate(CATEGORY_CACHE_KEY)]),
    [tenantMutate],
  );

  const items = useMemo(
    () =>
      data
        ? [...data].sort((a, b) => a.name.localeCompare(b.name, "de"))
        : undefined,
    [data],
  );

  const config = useMemo<CatalogConfig<ShiftType>>(() => {
    const categoryList = categoriesReady ? (categories ?? []) : [];

    const sections = (): SectionConfig[] => [
      {
        title: "Schichtart",
        fields: [
          {
            name: "name",
            label: "Name",
            type: "text",
            required: true,
            placeholder: "z. B. Betreuung / Zeit am Kind",
            maxLength: 100,
          },
          {
            name: "color",
            label: "Farbe",
            type: "custom",
            component: CatalogColorField,
            componentProps: {
              defaultHex: DEFAULT_COLOR,
              hint: "Die Farbe kennzeichnet die Schichtart im Wochenplan.",
            },
          },
          {
            name: "description",
            label: "Beschreibung",
            type: "textarea",
            placeholder: "Optional",
            maxLength: 500,
          },
          {
            name: "isActive",
            label: "In der Auswahl neuer Schichten anbieten",
            type: "checkbox",
            colSpan: 2,
          },
          ...(categoriesReady
            ? [
                {
                  name: "categoryIds",
                  label: "Kategorien des Betreuungsplans",
                  type: "multiselect" as const,
                  colSpan: 2 as const,
                  placeholder: "Kategorie hinzufügen…",
                  options: categoryList.map((category) => ({
                    value: category.id,
                    label: category.name,
                  })),
                  helperText:
                    "Betreuungsblöcke dieser Kategorien gehören zu dieser Schichtart. Eine Kategorie kann nur zu einer Schichtart gehören.",
                },
              ]
            : []),
        ],
      },
    ];

    /** Was das Backend bekommt. `categoryIds` bleibt weg, solange die
     *  Kategorien nicht sicher geladen sind (#1837). */
    const payloadOf = (
      values: Record<string, unknown>,
      fallback: ShiftType | null,
    ) => ({
      name: String(values.name ?? fallback?.name ?? "").trim(),
      color:
        typeof values.color === "string"
          ? values.color
          : (fallback?.color ?? DEFAULT_COLOR),
      description: String(values.description ?? "").trim(),
      isActive: Boolean(values.isActive),
      ...(categoriesReady && Array.isArray(values.categoryIds)
        ? { categoryIds: values.categoryIds as string[] }
        : {}),
    });

    return {
      concept: "staffPlan",
      title: "Schichtarten",
      singular: "Schichtart",
      purpose:
        "Schichtarten benennen die Aufgabe einer geplanten Schicht, zum Beispiel Betreuung, Vorbereitung oder Pause.",
      searchPlaceholder: "Schichtart suchen…",
      emptyDescription:
        "Legen Sie eine Schichtart an oder fügen Sie die Beispiele hinzu, damit der Dienstplan Schichten benennen kann.",
      stats: (types) => {
        const inactive = types.filter((type) => !type.isActive).length;
        const active = types.length - inactive;
        return inactive > 0
          ? `${formatCount(active)} in der Auswahl · ${formatCount(inactive)} ausgeschaltet`
          : `${formatCount(active)} ${active === 1 ? "Schichtart" : "Schichtarten"}`;
      },
      toRow: (type) => ({
        name: type.name,
        subtitle: type.description,
        color: type.color,
        retired: !type.isActive,
      }),
      sections,
      toFormValues: (type) => ({
        name: type.name,
        color: type.color,
        description: type.description,
        isActive: type.isActive,
        categoryIds: categoryList
          .filter((category) => category.shiftTypeId === type.id)
          .map((category) => category.id),
      }),
      createDefaults: {
        name: "",
        color: DEFAULT_COLOR,
        description: "",
        isActive: true,
        categoryIds: [],
      },
      create: (values) =>
        shiftTypeService.createShiftType(payloadOf(values, null)),
      update: (type, values) =>
        shiftTypeService.updateShiftType(type.id, payloadOf(values, type)),
      retire: {
        menuLabel: "Nicht mehr anbieten",
        confirmTitle: "Schichtart nicht mehr anbieten?",
        confirmLabel: "Nicht mehr anbieten",
        describe: (type) =>
          `Die Schichtart „${type.name}" steht bei neuen Schichten nicht mehr zur Auswahl. Geplante Schichten behalten sie.`,
        run: (type) =>
          shiftTypeService.updateShiftType(type.id, {
            name: type.name,
            color: type.color,
            description: type.description,
            isActive: false,
          }),
        toast: (type) => `Schichtart „${type.name}" wird nicht mehr angeboten`,
      },
      restore: {
        menuLabel: "Wieder anbieten",
        run: (type) =>
          shiftTypeService.updateShiftType(type.id, {
            name: type.name,
            color: type.color,
            description: type.description,
            isActive: true,
          }),
        toast: (type) => `Schichtart „${type.name}" wird wieder angeboten`,
      },
      remove: {
        menuLabel: "Löschen",
        confirmTitle: "Schichtart löschen",
        describe: describeDeletion,
        run: (type) => shiftTypeService.deleteShiftType(type.id),
        toast: (type) => `Schichtart „${type.name}" gelöscht`,
      },
    };
  }, [categories, categoriesReady]);

  const handleSeed = useCallback(async () => {
    setSeeding(true);
    try {
      await shiftTypeService.createDefaults();
      await onChanged();
      toast.success("Beispiele hinzugefügt");
    } catch (err: unknown) {
      logger.error("shift_type_seed_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error(
        err instanceof Error && err.message
          ? err.message
          : "Die Beispiele konnten nicht angelegt werden.",
      );
    } finally {
      setSeeding(false);
    }
  }, [onChanged, toast]);

  return (
    <CatalogPage
      config={config}
      items={items}
      isLoading={isLoading && data === undefined}
      error={
        loadError
          ? "Die Schichtarten konnten nicht geladen werden. Bitte laden Sie die Seite neu."
          : null
      }
      onChanged={onChanged}
      // Die Route liegt hinter time_tracking:manage (database/layout).
      canManage
      selectedId={searchParams.get(CATALOG_SELECTION_PARAM)}
      headAction={
        <Button
          type="button"
          variant="outline"
          size="md"
          isLoading={seeding}
          loadingText="Wird angelegt…"
          onClick={() => void handleSeed()}
        >
          Beispiele hinzufügen
        </Button>
      }
    />
  );
}

export default function ShiftTypesPage() {
  return (
    <Suspense fallback={null}>
      <ShiftTypesPageContent />
    </Suspense>
  );
}
