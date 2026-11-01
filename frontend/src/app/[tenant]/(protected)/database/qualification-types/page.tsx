"use client";

// Nachweisarten der Schule (#4808) als Katalog der Datenverwaltung: die
// Standardliste (Erste Hilfe am Kind, Rettungsfähigkeit, erweitertes
// Führungszeugnis) und eigene Arten, je mit Gültigkeit, Vorlauf der
// Erinnerung und Pflicht für alle. Archivieren statt Löschen: eingetragene
// Nachweise behalten ihre Art.

import { Suspense, useCallback } from "react";
import { useSearchParams } from "next/navigation";

import {
  CatalogPage,
  CATALOG_SELECTION_PARAM,
  type CatalogConfig,
} from "~/components/database/catalog/catalog-page";
import { staffConcept } from "~/lib/concepts/staff";
import type { SectionConfig } from "~/lib/database/types";
import { formatCount } from "~/lib/format-utils";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  staffQualificationService,
  type QualificationType,
  type QualificationTypeInput,
} from "~/lib/staff-qualification-api";
import { staffQualificationKeys } from "~/lib/swr/staff-keys";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";

const CACHE_KEY = staffQualificationKeys.types();

function sections(): SectionConfig[] {
  return [
    {
      title: "Nachweisart",
      fields: [
        {
          name: "name",
          label: "Name",
          type: "text",
          required: true,
          placeholder: "z. B. Hygieneschulung",
          maxLength: 100,
        },
        {
          name: "validityMonths",
          label: "Gültig für (Monate)",
          type: "number",
          min: 1,
          max: 240,
          helperText:
            "moto schlägt damit das Ablaufdatum vor. Leer lassen, wenn der Nachweis nicht abläuft.",
        },
        {
          name: "reminderLeadWeeks",
          label: "Erinnerung vor Ablauf (Wochen)",
          type: "number",
          required: true,
          min: 0,
          max: 52,
          helperText:
            "Die Leitung und die Person bekommen dann eine Nachricht und eine E-Mail. Am Ablauftag kommt eine zweite.",
        },
        {
          name: "mandatory",
          label: "Pflicht für alle Mitarbeitenden",
          type: "checkbox",
          colSpan: 2,
          helperText:
            "Fehlt der Nachweis bei einer Person, steht sie unter Nachweise als „Fehlt“.",
        },
      ],
    },
  ];
}

function numberOrNull(value: unknown): number | null {
  if (value === null || value === undefined || value === "") return null;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function toInput(
  values: Record<string, unknown>,
  archived: boolean,
): QualificationTypeInput {
  return {
    name: String(values.name ?? "").trim(),
    validityMonths: numberOrNull(values.validityMonths),
    reminderLeadWeeks: numberOrNull(values.reminderLeadWeeks) ?? 8,
    mandatory: Boolean(values.mandatory),
    archived,
  };
}

function inputOf(type: QualificationType, archived: boolean) {
  return {
    name: type.name,
    validityMonths: type.validityMonths,
    reminderLeadWeeks: type.reminderLeadWeeks,
    mandatory: type.mandatory,
    archived,
  };
}

function subtitle(type: QualificationType): string {
  return [
    type.validityMonths
      ? `${type.validityMonths} Monate gültig`
      : "Läuft nicht ab",
    `Erinnerung ${type.reminderLeadWeeks} Wochen vorher`,
    type.mandatory ? "Pflicht" : null,
  ]
    .filter(Boolean)
    .join(" · ");
}

const config: CatalogConfig<QualificationType> = {
  concept: staffConcept,
  title: "Nachweisarten",
  singular: "Nachweisart",
  purpose:
    "Arten von Nachweisen wie Erste Hilfe am Kind, mit Gültigkeit, Erinnerung und Pflicht. Wer welchen Nachweis hat, steht unter Nachweise.",
  searchPlaceholder: "Nachweisart suchen…",
  emptyDescription:
    "Erste Hilfe am Kind, Rettungsfähigkeit und erweitertes Führungszeugnis sind immer da. Eigene Arten legen Sie oben an.",
  stats: (types) => {
    const live = types.filter((type) => !type.archived);
    const mandatory = live.filter((type) => type.mandatory).length;
    return `${formatCount(live.length)} ${live.length === 1 ? "Art" : "Arten"} · ${formatCount(mandatory)} Pflicht`;
  },
  toRow: (type) => ({
    name: type.name,
    subtitle: subtitle(type),
    retired: type.archived,
  }),
  sections,
  toFormValues: (type) => ({
    name: type.name,
    validityMonths: type.validityMonths ?? "",
    reminderLeadWeeks: type.reminderLeadWeeks,
    mandatory: type.mandatory,
  }),
  createDefaults: {
    name: "",
    validityMonths: "",
    reminderLeadWeeks: 8,
    mandatory: false,
  },
  create: (values) =>
    staffQualificationService.createType(toInput(values, false)),
  update: (type, values) =>
    staffQualificationService.updateType(
      type.id,
      toInput(values, type.archived),
    ),
  retire: {
    menuLabel: "Archivieren",
    confirmTitle: "Nachweisart archivieren?",
    confirmLabel: "Archivieren",
    describe: (type) =>
      `„${type.name}“ steht bei neuen Nachweisen nicht mehr zur Auswahl. Eingetragene Nachweise bleiben, und ihre Erinnerungen kommen weiter.`,
    run: (type) =>
      staffQualificationService.updateType(type.id, inputOf(type, true)),
    toast: (type) => `Die Nachweisart „${type.name}“ ist archiviert.`,
  },
  restore: {
    menuLabel: "Wieder anbieten",
    run: (type) =>
      staffQualificationService.updateType(type.id, inputOf(type, false)),
    toast: (type) => `Die Nachweisart „${type.name}“ wird wieder angeboten.`,
  },
};

function QualificationTypesPageContent() {
  const searchParams = useSearchParams();
  const tenantMutate = useTenantMutate();
  const {
    data,
    isLoading,
    error: swrError,
    mutate,
  } = useSWRAuth<QualificationType[]>(CACHE_KEY, () =>
    staffQualificationService.listTypes(),
  );
  const loadError = useSwrLoadError(
    swrError,
    "die Liste der Nachweisarten",
    () => mutate(),
  );
  const onChanged = useCallback(() => tenantMutate(CACHE_KEY), [tenantMutate]);

  return (
    <CatalogPage
      config={config}
      items={data}
      isLoading={
        (isLoading && data === undefined) ||
        (swrError !== undefined && loadError === null)
      }
      error={loadError}
      onChanged={onChanged}
      // Die Route liegt hinter staff:stammdaten (database/layout).
      canManage
      selectedId={searchParams.get(CATALOG_SELECTION_PARAM)}
    />
  );
}

export default function QualificationTypesPage() {
  return (
    <Suspense fallback={null}>
      <QualificationTypesPageContent />
    </Suspense>
  );
}
