"use client";

// Planungsspuren als eigene Route der Datenverwaltung (#3114). Anlegen,
// Umbenennen, Umsortieren und Archivieren lagen vorher in einem Popover, das
// aus dem Auswahlfeld „Planungsspur" im Termin-Formular aufging — eine
// Verwaltungsoberfläche in einem Auswahlfeld. Das Feld ist jetzt reine
// Auswahl und verlinkt hierher.

import { Suspense, useCallback, useMemo } from "react";
import { useSearchParams } from "next/navigation";

import {
  CatalogPage,
  CATALOG_SELECTION_PARAM,
  type CatalogConfig,
} from "~/components/database/catalog/catalog-page";
import { PlanningDisabledState } from "~/components/planning/planning-disabled-state";
import { CatalogColorField } from "~/components/ui/database/catalog-color-field";
import type { SectionConfig } from "~/lib/database/types";
import { formatCount } from "~/lib/format-utils";
import { LOCATION_COLORS } from "~/lib/location-helper";
import {
  planningTrackService,
  type PlanningTrack,
} from "~/lib/planning-track-api";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { useTimetableEnabled } from "~/lib/tenant-context";

const CACHE_KEY = "database-planning-tracks";
const DEFAULT_COLOR: string = LOCATION_COLORS.OTHER_ROOM;

function sections(): SectionConfig[] {
  return [
    {
      title: "Planungsspur",
      fields: [
        {
          name: "name",
          label: "Name",
          type: "text",
          required: true,
          placeholder: "z. B. Jahrgang 1",
          maxLength: 100,
        },
        {
          name: "color",
          label: "Farbe",
          type: "custom",
          component: CatalogColorField,
          componentProps: {
            defaultHex: DEFAULT_COLOR,
            hint: "Die Farbe kennzeichnet die Spur im Betreuungsplan.",
          },
        },
      ],
    },
  ];
}

function PlanningTracksPageContent() {
  const searchParams = useSearchParams();
  const tenantMutate = useTenantMutate();
  const timetableEnabled = useTimetableEnabled();

  const {
    data,
    isLoading,
    error: loadError,
  } = useSWRAuth<PlanningTrack[]>(timetableEnabled ? CACHE_KEY : null, () =>
    planningTrackService.list(),
  );

  const onChanged = useCallback(() => tenantMutate(CACHE_KEY), [tenantMutate]);

  // Die Liste kommt bereits in Sortierreihenfolge; die Pfeile in der Sammlung
  // schreiben genau diese Folge zurück.
  const items = useMemo(
    () =>
      data ? [...data].sort((a, b) => a.sortOrder - b.sortOrder) : undefined,
    [data],
  );

  // Der Sortierwert eines neuen Eintrags hängt am aktuellen Bestand, deshalb
  // wird die Konfiguration hier gebaut und nicht im Modul.
  const config = useMemo<CatalogConfig<PlanningTrack>>(() => {
    const nextSortOrder =
      (items ?? [])
        .filter((track) => !track.archivedAt)
        .reduce((highest, track) => Math.max(highest, track.sortOrder), -1) + 1;

    return {
      concept: "carePlan",
      title: "Planungsspuren",
      singular: "Planungsspur",
      purpose:
        "Planungsspuren bündeln Regeltermine farblich, zum Beispiel nach Jahrgang.",
      searchPlaceholder: "Planungsspur suchen…",
      emptyDescription:
        "Legen Sie eine Planungsspur an, um Regeltermine im Betreuungsplan farblich zu bündeln.",
      stats: (tracks) => {
        const archived = tracks.filter((track) => track.archivedAt).length;
        const active = tracks.length - archived;
        return archived > 0
          ? `${formatCount(active)} in der Auswahl · ${formatCount(archived)} archiviert`
          : `${formatCount(active)} ${active === 1 ? "Planungsspur" : "Planungsspuren"}`;
      },
      toRow: (track) => ({
        name: track.name,
        color: track.color,
        retired: Boolean(track.archivedAt),
      }),
      sections,
      toFormValues: (track) => ({ name: track.name, color: track.color }),
      createDefaults: { name: "", color: DEFAULT_COLOR },
      // Neue Spuren folgen der bekannten aktiven Reihenfolge. Vor dem Laden
      // wäre ihre Position geraten und könnte mit einem Bestandseintrag
      // kollidieren.
      createDisabled: items === undefined,
      create: (values) =>
        planningTrackService.create({
          name: String(values.name ?? "").trim(),
          color:
            typeof values.color === "string" ? values.color : DEFAULT_COLOR,
          sort_order: nextSortOrder,
        }),
      update: (track, values) =>
        planningTrackService.update(track.id, {
          name: String(values.name ?? "").trim(),
          color:
            typeof values.color === "string" ? values.color : DEFAULT_COLOR,
          sort_order: track.sortOrder,
        }),
      reorder: (orderedIds) => planningTrackService.reorder(orderedIds),
      retire: {
        menuLabel: "Archivieren",
        confirmTitle: "Planungsspur archivieren?",
        confirmLabel: "Archivieren",
        describe: (track) =>
          `Die Planungsspur „${track.name}“ wird für neue Termine nicht mehr angeboten. Bestehende Termine behalten sie. Sie können die Spur jederzeit wiederherstellen.`,
        run: (track) => planningTrackService.archive(track.id),
        toast: (track) => `Planungsspur „${track.name}“ archiviert`,
      },
      restore: {
        menuLabel: "Wieder anbieten",
        run: (track) => planningTrackService.restore(track.id),
        toast: (track) => `Planungsspur „${track.name}“ wird wieder angeboten`,
      },
    };
  }, [items]);

  if (!timetableEnabled) {
    return (
      <PlanningDisabledState
        pageTitle="Planungsspuren"
        heading="Planungsspuren sind nicht verfügbar"
        description="Der Betreuungsplan ist für diese Schule ausgeschaltet."
        testId="planning-tracks-disabled-state"
      />
    );
  }

  return (
    <CatalogPage
      config={config}
      items={items}
      isLoading={isLoading && data === undefined}
      error={
        loadError
          ? "Die Planungsspuren konnten nicht geladen werden. Bitte laden Sie die Seite neu."
          : null
      }
      onChanged={onChanged}
      // Die Route liegt hinter schedules:manage (database/layout).
      canManage
      selectedId={searchParams.get(CATALOG_SELECTION_PARAM)}
    />
  );
}

export default function PlanningTracksPage() {
  return (
    <Suspense fallback={null}>
      <PlanningTracksPageContent />
    </Suspense>
  );
}
