"use client";

// Abwesenheitsarten der Schule (#2403) als eigene Route der Datenverwaltung
// (#3114). Anlegen, Umbenennen und Abschalten lagen vorher in den Zeilen des
// Auswahlfelds „Art der Abwesenheit", das aus zwei Dialogen der Zeiterfassung
// aufging. Das Feld ist jetzt reine Auswahl und verlinkt hierher.

import { Suspense, useCallback, useMemo } from "react";
import { useSearchParams } from "next/navigation";

import {
  CatalogPage,
  CATALOG_SELECTION_PARAM,
  type CatalogConfig,
} from "~/components/database/catalog/catalog-page";
import { absenceTypeService, type AbsenceType } from "~/lib/absence-type-api";
import type { SectionConfig } from "~/lib/database/types";
import { formatCount } from "~/lib/format-utils";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";

const CACHE_KEY = "database-absence-types";

function sections(): SectionConfig[] {
  return [
    {
      title: "Abwesenheitsart",
      fields: [
        {
          name: "name",
          label: "Name",
          type: "text",
          required: true,
          placeholder: "z. B. Regenerationstag",
          maxLength: 100,
        },
        {
          name: "allowanceEnabled",
          label: "Eigenes Jahreskontingent",
          type: "checkbox",
          colSpan: 2,
          helperText:
            "Wie viele Tage jede Person im Jahr hat, tragen Sie im Profil der Person ein.",
        },
        {
          name: "overrunPolicy",
          label: "Wenn keine Tage mehr übrig sind",
          type: "select",
          options: [
            { value: "warn", label: "Warnen und trotzdem eintragen" },
            { value: "block", label: "Eintragen verhindern" },
          ],
          helperText:
            "Gilt nur, wenn diese Art ein eigenes Jahreskontingent hat.",
        },
      ],
    },
  ];
}

function policyOf(values: Record<string, unknown>): "warn" | "block" {
  return values.overrunPolicy === "block" ? "block" : "warn";
}

const config: CatalogConfig<AbsenceType> = {
  concept: "timeTracking",
  title: "Abwesenheitsarten",
  singular: "Abwesenheitsart",
  purpose:
    "Eigene Namen für Abwesenheiten, zusätzlich zu Urlaub, Krank, Fortbildung und Sonstige.",
  searchPlaceholder: "Abwesenheitsart suchen…",
  emptyDescription:
    "Legen Sie einen eigenen Namen an, wenn Urlaub, Krank, Fortbildung und Sonstige nicht reichen. Diese vier gehören zum System und stehen immer zur Verfügung.",
  stats: (types) => {
    const inactive = types.filter((type) => !type.isActive).length;
    const active = types.length - inactive;
    return inactive > 0
      ? `${formatCount(active)} in der Auswahl · ${formatCount(inactive)} ausgeschaltet`
      : `${formatCount(active)} eigene ${active === 1 ? "Art" : "Arten"}`;
  },
  toRow: (type) => ({
    name: type.name,
    subtitle: type.allowanceEnabled
      ? "Mit eigenem Jahreskontingent"
      : "Wird wie Sonstige berechnet",
    retired: !type.isActive,
  }),
  sections,
  toFormValues: (type) => ({
    name: type.name,
    allowanceEnabled: type.allowanceEnabled,
    overrunPolicy: type.overrunPolicy,
  }),
  createDefaults: {
    name: "",
    allowanceEnabled: false,
    overrunPolicy: "warn",
  },
  // Das Anlegen kennt nur den Namen; Kontingent und Verhalten schreibt der
  // gleiche Vorgang direkt hinterher, damit der Dialog eine Sache bleibt.
  create: async (values) => {
    const created = await absenceTypeService.createAbsenceType(
      String(values.name ?? "").trim(),
    );
    const allowanceEnabled = Boolean(values.allowanceEnabled);
    const overrunPolicy = policyOf(values);
    if (!allowanceEnabled && overrunPolicy === "warn") return created;
    return absenceTypeService.updateAbsenceType(created.id, {
      allowanceEnabled,
      overrunPolicy,
    });
  },
  update: (type, values) =>
    absenceTypeService.updateAbsenceType(type.id, {
      name: String(values.name ?? type.name).trim(),
      allowanceEnabled: Boolean(values.allowanceEnabled),
      overrunPolicy: policyOf(values),
    }),
  retire: {
    menuLabel: "Nicht mehr anbieten",
    confirmTitle: "Abwesenheitsart nicht mehr anbieten?",
    confirmLabel: "Nicht mehr anbieten",
    describe: (type) =>
      `„${type.name}" steht bei neuen Abwesenheiten nicht mehr zur Auswahl. Bereits eingetragene Abwesenheiten behalten den Namen.`,
    run: (type) =>
      absenceTypeService.updateAbsenceType(type.id, { isActive: false }),
    toast: (type) => `„${type.name}" wird nicht mehr angeboten`,
  },
  restore: {
    menuLabel: "Wieder anbieten",
    run: (type) =>
      absenceTypeService.updateAbsenceType(type.id, { isActive: true }),
    toast: (type) => `„${type.name}" wird wieder angeboten`,
  },
};

function AbsenceTypesPageContent() {
  const searchParams = useSearchParams();
  const tenantMutate = useTenantMutate();

  const {
    data,
    isLoading,
    error: loadError,
  } = useSWRAuth<AbsenceType[]>(CACHE_KEY, () =>
    absenceTypeService.getAbsenceTypes(),
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
          ? "Die Abwesenheitsarten konnten nicht geladen werden. Bitte laden Sie die Seite neu."
          : null
      }
      onChanged={onChanged}
      // Die Route liegt hinter time_tracking:manage (database/layout).
      canManage
      selectedId={searchParams.get(CATALOG_SELECTION_PARAM)}
    />
  );
}

export default function AbsenceTypesPage() {
  return (
    <Suspense fallback={null}>
      <AbsenceTypesPageContent />
    </Suspense>
  );
}
