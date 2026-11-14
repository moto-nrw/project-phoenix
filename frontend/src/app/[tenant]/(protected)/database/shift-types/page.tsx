"use client";

// Schichtarten (#1836) als eigene Route der Datenverwaltung (#3114). Vorher
// lagen Liste und Formular in einem Slide-over, das der Dienstplan öffnete.
// Der Dienstplan verlinkt jetzt hierher.

import {
  Suspense,
  useCallback,
  useLayoutEffect,
  useMemo,
  useState,
} from "react";
import { useSearchParams } from "next/navigation";

import {
  CatalogPage,
  CATALOG_SELECTION_PARAM,
  type CatalogConfig,
} from "~/components/database/catalog/catalog-page";
import { ModuleRouteGuard } from "~/components/tenant/module-route-guard";
import { Button } from "~/components/ui/button";
import { FramedList } from "~/components/ui/framed-list";
import { IconTile } from "~/components/ui/icon-tile";
import { SectionLabel } from "~/components/ui/section-label";
import { CalendarClock } from "lucide-react";
import { CatalogColorField } from "~/components/database/catalog-color-field";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import type { ActivityCategory } from "~/lib/activity-helpers";
import { categoryService } from "~/lib/category-api";
import type { SectionConfig } from "~/lib/database/types";
import { formatCount } from "~/lib/format-utils";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { shiftTypeService } from "~/lib/shift-type-api";
import type { ShiftType, ShiftTypeUsage } from "~/lib/shift-type-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  STALE_DATA_SUFFIX,
  useRefreshAfterWrite,
} from "~/lib/hooks/use-refresh-after-write";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ApiError } from "~/lib/api-error";
import { useDiscardRefusedEntity } from "~/lib/swr/hooks";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { useModuleBooked } from "~/lib/tenant-context";
import { staffPlanConcept } from "~/lib/concepts/staffPlan";

const logger = createLogger({ component: "DatabaseShiftTypesPage" });

const CACHE_KEY = "database-shift-types";
const CATEGORY_CACHE_KEY = "database-shift-type-categories";
const DEFAULT_COLOR: string = LOCATION_COLORS.GROUP_ROOM;

/** „12 geplante Dienste“ / „1 Schichtserie“ für die Rückfrage (#6845). */
function usageParts(usage: ShiftTypeUsage): string[] {
  const parts: string[] = [];
  if (usage.shifts > 0) {
    parts.push(
      usage.shifts === 1
        ? "1 geplanter Dienst"
        : `${formatCount(usage.shifts)} geplante Dienste`,
    );
  }
  if (usage.series > 0) {
    parts.push(
      usage.series === 1
        ? "1 Schichtserie"
        : `${formatCount(usage.series)} Schichtserien`,
    );
  }
  return parts;
}

/** Geplante Dienste oder Schichtserien tragen die Dienstart: Entfernen
 *  archiviert sie (#6845). */
function isArchivable(type: ShiftType): boolean {
  return (type.usage?.shifts ?? 0) > 0 || (type.usage?.series ?? 0) > 0;
}

/** Steht außerhalb der Seite, damit die Rückfrage nicht als bei jedem Render
 *  neu erzeugte Komponente gilt. Eine Dienstart, die geplante Dienste oder
 *  Schichtserien tragen, wird nicht gelöscht, sondern aus der Auswahl
 *  genommen: sonst verlören auch vergangene Dienste ihre Dienstart (#6845). */
function describeDeletion(type: ShiftType, linkedCategories: number) {
  const parts = type.usage ? usageParts(type.usage) : null;
  if (parts === null) {
    return (
      <p>
        Die Dienstart <strong className="text-gray-900">{type.name}</strong>{" "}
        wird entfernt. Tragen geplante Dienste sie noch, behalten diese sie. Für
        neue Dienste steht sie dann nicht mehr zur Auswahl.
      </p>
    );
  }
  if (parts.length > 0) {
    return (
      <div className="flex flex-col gap-3">
        <p>
          <strong className="text-gray-900">{type.name}</strong> verschwindet
          aus der Auswahl für neue Dienste. Geplante und vergangene Dienste
          behalten ihre Dienstart.
        </p>
        <SectionLabel as="h3">Bleibt im Dienstplan erhalten</SectionLabel>
        <FramedList>
          {parts.map((part) => (
            <li
              key={part}
              className="flex items-center gap-3 px-4 py-3 text-sm font-medium text-gray-900"
            >
              <IconTile
                icon={<CalendarClock aria-hidden />}
                size="xs"
                tone="neutral"
              />
              {part}
            </li>
          ))}
        </FramedList>
      </div>
    );
  }
  return (
    <p>
      Die Dienstart <strong className="text-gray-900">{type.name}</strong> wird
      gelöscht. Kein geplanter Dienst nutzt sie.
      {linkedCategories === 1 &&
        " 1 Kategorie des Betreuungsplans verliert diese Zuordnung."}
      {linkedCategories > 1 &&
        ` ${formatCount(linkedCategories)} Kategorien des Betreuungsplans verlieren diese Zuordnung.`}
    </p>
  );
}

function ShiftTypesPageContent() {
  const searchParams = useSearchParams();
  const tenantMutate = useTenantMutate();
  const toast = useToast();
  // „Beispiele hinzufügen“ ist eine Aktion ohne Formular: Fehler als Toast.
  const { show: showSeedError } = useApiErrorDisplay();
  const [seeding, setSeeding] = useState(false);

  const discardRefused = useDiscardRefusedEntity();
  const {
    data: cachedData,
    isLoading,
    error: swrError,
    mutate,
  } = useSWRAuth<ShiftType[]>(CACHE_KEY, () =>
    // Die Zahlen sagen der Rückfrage, ob gelöscht oder archiviert wird.
    shiftTypeService.getShiftTypes({ withUsage: true }),
  );
  const accessRefused =
    swrError instanceof ApiError &&
    (swrError.status === 401 || swrError.status === 403);
  const data = accessRefused ? undefined : cachedData;
  useLayoutEffect(() => {
    if (!accessRefused) return;
    discardRefused(swrError, {
      codes: [swrError.code ?? "general.permission"],
      matches: (key) => key === CACHE_KEY,
      refusedKey: CACHE_KEY,
    });
  }, [accessRefused, swrError, discardRefused]);
  // Ladefehler mit Katalogtext und Wiederholen statt eines eigenen Satzes
  // (#2517).
  const loadError = useSwrLoadError(
    swrError,
    "die Liste der Dienstarten",
    () => mutate(),
    data === undefined ? undefined : STALE_DATA_SUFFIX,
  );

  // Die Kategorien tragen ihre aktuelle Schichtart; daraus kommt die
  // Vorauswahl. Solange sie nicht geladen sind, zeigt das Formular das Feld
  // gar nicht und die Speichern-Aufrufe lassen `categoryIds` weg — eine leere
  // Liste würde bestehende Zuordnungen löschen (#1837).
  const { data: categories, error: categoriesError } = useSWRAuth<
    ActivityCategory[]
  >(CATEGORY_CACHE_KEY, () => categoryService.getManagedCategories());
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

    const sections = (type: ShiftType | null): SectionConfig[] => [
      {
        title: "Dienstart",
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
              hint: "Die Farbe kennzeichnet die Dienstart im Wochenplan.",
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
            label: "In der Auswahl neuer Dienste anbieten",
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
                  // Archivierte Kategorien dürfen nicht neu zugeordnet werden.
                  // Eine vorhandene Zuordnung bleibt aber sichtbar und damit
                  // beim Speichern erhalten, bis sie bewusst entfernt wird.
                  options: categoryList
                    .filter(
                      (category) =>
                        !category.isSystem &&
                        (!category.archivedAt ||
                          category.shiftTypeId === type?.id),
                    )
                    .map((category) => ({
                      value: category.id,
                      label: category.archivedAt
                        ? `${category.name} (nicht mehr angeboten)`
                        : category.name,
                    })),
                  helperText:
                    "Betreuungsblöcke dieser Kategorien gehören zu dieser Dienstart. Eine Kategorie kann nur zu einer Dienstart gehören.",
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
      concept: staffPlanConcept,
      title: "Dienstarten",
      singular: "Dienstart",
      purpose:
        "Dienstarten benennen die Aufgabe eines geplanten Dienstes, zum Beispiel Betreuung, Vorbereitung oder Pause.",
      searchPlaceholder: "Dienstart suchen…",
      emptyDescription:
        "Legen Sie eine Dienstart an oder fügen Sie die Beispiele hinzu, damit der Dienstplan Dienste benennen kann.",
      preserveDraftOnSectionsChange: true,
      stats: (types) => {
        const inactive = types.filter((type) => !type.isActive).length;
        const active = types.length - inactive;
        return inactive > 0
          ? `${formatCount(active)} in der Auswahl · ${formatCount(inactive)} ausgeschaltet`
          : `${formatCount(active)} ${active === 1 ? "Dienstart" : "Dienstarten"}`;
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
        confirmTitle: "Dienstart nicht mehr anbieten?",
        confirmLabel: "Nicht mehr anbieten",
        loadingLabel: "Wird aus der Auswahl genommen …",
        describe: (type) =>
          `Die Dienstart „${type.name}“ steht bei neuen Diensten nicht mehr zur Auswahl. Geplante Dienste behalten sie.`,
        run: (type) =>
          shiftTypeService.updateShiftType(type.id, {
            name: type.name,
            color: type.color,
            description: type.description,
            isActive: false,
          }),
        toast: (type) =>
          `Die Dienstart „${type.name}“ wird nicht mehr angeboten.`,
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
        toast: (type) => `Die Dienstart „${type.name}“ wird wieder angeboten.`,
      },
      remove: {
        menuLabel: "Löschen",
        menuDescription:
          "Löscht oder archiviert die Dienstart, moto fragt vorher nach.",
        confirmTitle: (type) =>
          isArchivable(type) ? "Dienstart archivieren?" : "Dienstart löschen?",
        confirmLabels: (type) =>
          isArchivable(type)
            ? { first: "Ja, archivieren", confirm: "Archivieren" }
            : undefined,
        describe: (type) =>
          describeDeletion(
            type,
            categoryList.filter((category) => category.shiftTypeId === type.id)
              .length,
          ),
        run: (type) => shiftTypeService.deleteShiftType(type.id),
        toast: (type, removal) =>
          removal === "archived"
            ? `Die Dienstart „${type.name}“ steht nicht mehr zur Auswahl. Geplante Dienste behalten sie.`
            : `Die Dienstart „${type.name}“ ist gelöscht.`,
      },
    };
  }, [categories, categoriesReady]);

  // Die Beispiele laden nach dem Anlegen; ein Fehler dabei steht über der
  // Liste und legt nichts erneut an (#6652).
  const seedAfterWrite = useRefreshAfterWrite(
    "die Anzeige der Dienstarten",
    onChanged,
  );

  const handleSeed = useCallback(async () => {
    setSeeding(true);
    try {
      await shiftTypeService.createDefaults();
    } catch (err: unknown) {
      logger.error("shift_type_seed_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showSeedError(err, {
        object: "das Anlegen der Beispiele",
        retry: () => void handleSeed(),
      });
      setSeeding(false);
      return;
    }
    setSeeding(false);
    toast.success("Die Beispiele sind hinzugefügt.");
    await seedAfterWrite.refresh();
  }, [seedAfterWrite, toast, showSeedError]);

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
      notice={<LoadErrorAlert error={seedAfterWrite.error} />}
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
          tooltip="Legt sofort typische Dienstarten an, die noch fehlen."
          onClick={() => void handleSeed()}
        >
          Beispiele hinzufügen
        </Button>
      }
    />
  );
}

function BookedShiftTypesPage() {
  const dienstplanBooked = useModuleBooked("dienstplan");

  // Die Buchung ist noch nicht bekannt: nicht mit einem leeren Katalog
  // antworten und erst recht keine Katalogdaten laden. Ist das Modul sicher
  // nicht gebucht, ersetzt TenantPage diesen Ladezustand durch die Erklärung.
  if (!dienstplanBooked) {
    return <TenantPage title="Dienstarten" loading />;
  }

  return <ShiftTypesPageContent />;
}

export default function ShiftTypesPage() {
  return (
    <Suspense fallback={null}>
      <ModuleRouteGuard module="dienstplan" title="Dienstarten">
        <BookedShiftTypesPage />
      </ModuleRouteGuard>
    </Suspense>
  );
}
