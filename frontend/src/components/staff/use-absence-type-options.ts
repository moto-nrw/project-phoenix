"use client";

// The option list behind the Abwesenheitsart dropdown (#2403): the five
// standard types plus whatever names the school added, in one list.
//
// Lives in the component layer rather than in lib/ because it produces
// option props for the dropdown and runs React hooks. The pure value↔request
// mapping stays in ~/lib/absence-type-select, which nothing in the UI layer
// needs to know about.

import { useMemo } from "react";

import { absenceTypeService, type AbsenceType } from "~/lib/absence-type-api";
import { customOptionValue } from "~/lib/absence-type-select";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";

const logger = createLogger({ component: "AbsenceTypeOptions" });

export interface AbsenceTypeOption {
  readonly value: string;
  readonly label: string;
  /** A standard type: code-owned, so it carries no rename/retire affordance. */
  readonly fixed?: boolean;
  /**
   * A retired type stays selectable only while it is the current value, so an
   * absence filed under it keeps rendering its own name instead of silently
   * falling back to something else.
   */
  readonly inactive?: boolean;
  readonly allowanceEnabled?: boolean;
  readonly overrunPolicy?: "warn" | "block";
}

/**
 * The standard types offered for self-service. Freizeitausgleich is absent on
 * purpose — it moves the Stundenkonto and stays manager-controlled.
 */
export const STANDARD_ABSENCE_OPTIONS: readonly AbsenceTypeOption[] = [
  { value: "sick", label: "Krank", fixed: true },
  { value: "vacation", label: "Urlaub", fixed: true },
  { value: "training", label: "Fortbildung", fixed: true },
  { value: "other", label: "Sonstige", fixed: true },
];

export interface UseAbsenceTypeOptionsResult {
  readonly options: AbsenceTypeOption[];
}

/**
 * useAbsenceTypeOptions loads the school's own Abwesenheitsarten and returns
 * them merged behind the standard ones.
 *
 * Gepflegt werden sie seit #3114 unter „Datenverwaltung → Abwesenheitsarten",
 * nicht mehr in den Zeilen des Auswahlfelds; dieser Haken liest nur noch.
 *
 * `standardOptions` lets a caller narrow which standard types are offered (the
 * admin path may include Freizeitausgleich, self-service may not).
 */
export function useAbsenceTypeOptions(
  canManage: boolean,
  standardOptions: readonly AbsenceTypeOption[] = STANDARD_ABSENCE_OPTIONS,
): UseAbsenceTypeOptionsResult {
  const { data: custom = [] } = useSWRAuth<AbsenceType[]>(
    "staff-absence-types",
    absenceTypeService.getAbsenceTypes.bind(absenceTypeService),
    {
      onError: (err) => {
        // A failing list must not block entering a plain absence: the standard
        // types stay available and the dropdown simply shows nothing extra.
        logger.warn("absence_types_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      },
    },
  );

  const options = useMemo<AbsenceTypeOption[]>(
    () => [
      ...standardOptions,
      ...custom
        .filter((type) => canManage || !type.allowanceEnabled)
        .map((type) => ({
          value: customOptionValue(type.id),
          label: type.name,
          inactive: !type.isActive,
          allowanceEnabled: type.allowanceEnabled,
          overrunPolicy: type.overrunPolicy,
        })),
    ],
    [standardOptions, custom, canManage],
  );

  return { options };
}
