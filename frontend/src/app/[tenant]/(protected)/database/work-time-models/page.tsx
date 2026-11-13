"use client";

// Arbeitszeitmodell-Vorlagen der Schule (#6891). Vorher ließ sich eine
// Vorlage nur aus der Personalakte heraus anlegen; hier sieht die Schule alle
// Vorlagen, legt sie an, ändert und archiviert sie. Das Muster bearbeitet
// derselbe Editor wie im Reiter „Arbeitszeitmodell“.

import { Suspense, useCallback, useLayoutEffect, useMemo } from "react";
import { useSearchParams } from "next/navigation";

import {
  CatalogPage,
  CATALOG_SELECTION_PARAM,
  type CatalogConfig,
} from "~/components/database/catalog/catalog-page";
import {
  validatePatternField,
  WorkTimeModelPatternField,
  type PatternFieldValue,
} from "~/components/staff/work-time-model-pattern-field";
import {
  draftFromPattern,
  patternEntries,
} from "~/components/staff/work-time-pattern-draft";
import { ModuleRouteGuard } from "~/components/tenant/module-route-guard";
import { ApiError } from "~/lib/api-error";
import { timeTrackingConcept } from "~/lib/concepts/timeTracking";
import type { SectionConfig } from "~/lib/database/types";
import { berlinTodayISO, parseISODate } from "~/lib/date-helpers";
import { formatCount } from "~/lib/format-utils";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { STALE_DATA_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import { workTimeModelService, type WorkTimeModel } from "~/lib/staff-api";
import { startOfWeek, toDateKey } from "~/lib/staff-metrics-helpers";
import { useInvalidateMatching, useSWRAuth, useTenantMutate } from "~/lib/swr";
import { useDiscardRefusedEntity } from "~/lib/swr/hooks";
import { workTimeModelKeys } from "~/lib/swr/staff-keys";
import {
  formatDecimalHours,
  isStaleAfterModelSave,
} from "~/components/staff/arbeitszeit-hours";

const CACHE_KEY = workTimeModelKeys.catalog();
// Der Reiter „Arbeitszeitmodell“ liest die Vorlagen unter diesem Schlüssel.
const STAFF_PICKER_KEY = workTimeModelKeys.picker();
// Ohne gesetzten Beginn liefert das Backend dieses Datum.
const UNSET_ANCHOR = "0000-00-00";

function rotationLabel(length: number): string {
  return length === 1 ? "1 Woche" : `${length}-Wochen-Rotation`;
}

function usageLabel(count: number | undefined): string | null {
  if (count === undefined) return null;
  if (count === 0) return "nicht genutzt";
  return count === 1 ? "1 Person" : `${formatCount(count)} Personen`;
}

/** „40 Std. pro Woche · 1 Woche · 3 Personen“ */
function subtitleOf(model: WorkTimeModel): string {
  const total = model.weeklyTotals.reduce((sum, minutes) => sum + minutes, 0);
  const perWeek = Math.round(total / Math.max(1, model.rotationLength));
  const perWeekText = `${formatDecimalHours(perWeek)} Std. pro Woche`;
  const hours = model.rotationLength > 1 ? `Ø ${perWeekText}` : perWeekText;
  return [
    hours,
    rotationLabel(model.rotationLength),
    usageLabel(model.assignedStaffCount),
  ]
    .filter(Boolean)
    .join(" · ");
}

function currentMonday(): string {
  return toDateKey(startOfWeek(parseISODate(berlinTodayISO())));
}

function patternValueOf(model: WorkTimeModel | null): PatternFieldValue {
  if (!model) {
    return { draft: draftFromPattern(1, []), anchorDate: currentMonday() };
  }
  return {
    draft: draftFromPattern(model.rotationLength, model.entries),
    anchorDate:
      model.rotationAnchorDate === UNSET_ANCHOR ? "" : model.rotationAnchorDate,
  };
}

function inputOf(
  values: Record<string, unknown>,
  fallback: WorkTimeModel | null,
) {
  const pattern = values.pattern as PatternFieldValue;
  // Eine Woche hat keine Woche A; der Beginn bleibt dann, wie er war.
  const anchor =
    pattern.anchorDate ||
    (fallback && fallback.rotationAnchorDate !== UNSET_ANCHOR
      ? fallback.rotationAnchorDate
      : currentMonday());
  return {
    name: String(values.name ?? fallback?.name ?? "").trim(),
    rotationLength: pattern.draft.rotationLength,
    rotationAnchorDate: anchor,
    entries: patternEntries(pattern.draft),
  };
}

function sectionsOf(model: WorkTimeModel | null): SectionConfig[] {
  return [
    {
      title: "Vorlage",
      fields: [
        {
          name: "name",
          label: "Name",
          type: "text",
          required: true,
          placeholder: "z. B. Teilzeit 30 Stunden A/B",
          maxLength: 100,
          colSpan: 2,
        },
        {
          name: "pattern",
          label: "Soll-Stunden",
          type: "custom",
          component: WorkTimeModelPatternField,
          componentProps: { assignedCount: model?.assignedStaffCount ?? 0 },
          validation: validatePatternField,
          colSpan: 2,
        },
      ],
    },
  ];
}

const config: CatalogConfig<WorkTimeModel> = {
  concept: timeTrackingConcept,
  title: "Arbeitszeitmodelle",
  singular: "Vorlage",
  purpose:
    "Vorlagen legen die Soll-Stunden für viele Personen auf einmal fest. Zuordnen können Sie eine Vorlage in der Personalakte.",
  searchPlaceholder: "Vorlage suchen…",
  emptyDescription:
    "Legen Sie eine Vorlage an, zum Beispiel für Vollzeit oder Teilzeit. Danach ordnen Sie sie in der Personalakte zu.",
  stats: (models) => {
    const archived = models.filter((model) => model.archivedAt).length;
    const offered = models.length - archived;
    const head = `${formatCount(offered)} ${offered === 1 ? "Vorlage" : "Vorlagen"}`;
    return archived > 0
      ? `${head} · ${formatCount(archived)} nicht mehr angeboten`
      : head;
  },
  toRow: (model) => ({
    name: model.name,
    subtitle: subtitleOf(model),
    retired: model.archivedAt !== null,
  }),
  sections: sectionsOf,
  toFormValues: (model) => ({
    name: model.name,
    pattern: patternValueOf(model),
  }),
  createDefaults: { name: "", pattern: patternValueOf(null) },
  create: (values) => workTimeModelService.create(inputOf(values, null)),
  update: (model, values) =>
    workTimeModelService.update(model.id, inputOf(values, model)),
  retire: {
    menuLabel: "Nicht mehr anbieten",
    confirmTitle: "Vorlage nicht mehr anbieten?",
    confirmLabel: "Nicht mehr anbieten",
    loadingLabel: "Wird aus der Auswahl genommen …",
    describe: (model) =>
      `„${model.name}“ steht in der Personalakte nicht mehr zur Auswahl. Wer die Vorlage schon nutzt, behält sie und ihr Soll.`,
    run: (model) => workTimeModelService.archive(model.id),
    toast: (model) => `Die Vorlage „${model.name}“ wird nicht mehr angeboten.`,
  },
  restore: {
    menuLabel: "Wieder anbieten",
    run: (model) => workTimeModelService.restore(model.id),
    toast: (model) => `Die Vorlage „${model.name}“ wird wieder angeboten.`,
  },
  remove: {
    menuLabel: "Löschen",
    confirmTitle: "Vorlage löschen",
    describe: (model) => (
      <>
        Die Vorlage <strong>{model.name}</strong> wird gelöscht. Das geht nur,
        wenn niemand sie nutzt. Sonst nehmen Sie sie aus der Auswahl.
      </>
    ),
    run: (model) => workTimeModelService.remove(model.id),
    toast: (model) => `Die Vorlage „${model.name}“ ist gelöscht.`,
  },
};

function WorkTimeModelsPageContent() {
  const searchParams = useSearchParams();
  const tenantMutate = useTenantMutate();
  const invalidate = useInvalidateMatching();
  const discardRefused = useDiscardRefusedEntity();
  const {
    data: cachedData,
    isLoading,
    error: swrError,
    mutate,
  } = useSWRAuth<WorkTimeModel[]>(workTimeModelKeys.catalog(), () =>
    workTimeModelService.listWithUsage(),
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
  const loadError = useSwrLoadError(
    swrError,
    "die Liste der Vorlagen",
    () => mutate(),
    data === undefined ? undefined : STALE_DATA_SUFFIX,
  );

  // Eine Änderung schreibt das Soll aller zugeordneten Personen neu: die
  // Auswahl in der Personalakte und jede Soll-Ansicht laden neu.
  const onChanged = useCallback(
    () =>
      Promise.all([
        tenantMutate(CACHE_KEY),
        tenantMutate(STAFF_PICKER_KEY),
        invalidate(isStaleAfterModelSave),
      ]),
    [tenantMutate, invalidate],
  );

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
      isLoading={
        (isLoading && data === undefined) ||
        (swrError !== undefined && loadError === null)
      }
      error={loadError}
      onChanged={onChanged}
      // Die Route liegt hinter time_tracking:manage (database/layout).
      canManage
      selectedId={searchParams.get(CATALOG_SELECTION_PARAM)}
    />
  );
}

export default function WorkTimeModelsPage() {
  return (
    <ModuleRouteGuard module="zeiterfassung" title="Arbeitszeitmodelle">
      <Suspense fallback={null}>
        <WorkTimeModelsPageContent />
      </Suspense>
    </ModuleRouteGuard>
  );
}
