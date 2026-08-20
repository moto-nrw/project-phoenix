"use client";

// The option list behind the Abwesenheitsart dropdown (#2403): the five
// standard types plus whatever names the school added, in one list.
//
// Lives in the component layer rather than in lib/ because it produces
// component props (CreatableSelectOption) and runs React hooks. The pure
// value↔request mapping stays in ~/lib/absence-type-select, which nothing in
// the UI layer needs to know about.

import { useCallback, useMemo } from "react";

import type { CreatableSelectOption } from "~/components/ui/creatable-select";
import { absenceTypeService, type AbsenceType } from "~/lib/absence-type-api";
import {
  customIdFromOptionValue,
  customOptionValue,
} from "~/lib/absence-type-select";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";

const logger = createLogger({ component: "AbsenceTypeOptions" });

/**
 * The standard types offered for self-service. Freizeitausgleich is absent on
 * purpose — it moves the Stundenkonto and stays manager-controlled.
 */
export const STANDARD_ABSENCE_OPTIONS: readonly CreatableSelectOption[] = [
  { value: "sick", label: "Krank", fixed: true },
  { value: "vacation", label: "Urlaub", fixed: true },
  { value: "training", label: "Fortbildung", fixed: true },
  { value: "other", label: "Sonstige", fixed: true },
];

interface UseAbsenceTypeOptionsResult {
  readonly options: CreatableSelectOption[];
  /** Undefined for a user without time_tracking:manage — hides the affordance. */
  readonly create?: (name: string) => Promise<string>;
  readonly rename?: (value: string, name: string) => Promise<void>;
  readonly setActive?: (value: string, isActive: boolean) => Promise<void>;
}

/**
 * useAbsenceTypeOptions loads the school's own Abwesenheitsarten and returns
 * them merged behind the standard ones, plus the write callbacks — but only
 * when the caller may use them, so the component renders as a plain searchable
 * select for everyone else.
 *
 * `standardOptions` lets a caller narrow which standard types are offered (the
 * admin path may include Freizeitausgleich, self-service may not).
 */
export function useAbsenceTypeOptions(
  canManage: boolean,
  standardOptions: readonly CreatableSelectOption[] = STANDARD_ABSENCE_OPTIONS,
): UseAbsenceTypeOptionsResult {
  const { data: custom = [], mutate } = useSWRAuth<AbsenceType[]>(
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

  const options = useMemo<CreatableSelectOption[]>(
    () => [
      ...standardOptions,
      ...custom.map((type) => ({
        value: customOptionValue(type.id),
        label: type.name,
        inactive: !type.isActive,
      })),
    ],
    [standardOptions, custom],
  );

  const create = useCallback(
    async (name: string) => {
      const created = await absenceTypeService.createAbsenceType(name);
      await mutate((previous = []) => [...previous, created], false);
      return customOptionValue(created.id);
    },
    [mutate],
  );

  const rename = useCallback(
    async (value: string, name: string) => {
      const updated = await absenceTypeService.updateAbsenceType(
        customIdFromOptionValue(value),
        { name },
      );
      await mutate(
        (previous = []) =>
          previous.map((type) => (type.id === updated.id ? updated : type)),
        false,
      );
    },
    [mutate],
  );

  const setActive = useCallback(
    async (value: string, isActive: boolean) => {
      const updated = await absenceTypeService.updateAbsenceType(
        customIdFromOptionValue(value),
        { isActive },
      );
      await mutate(
        (previous = []) =>
          previous.map((type) => (type.id === updated.id ? updated : type)),
        false,
      );
    },
    [mutate],
  );

  return canManage ? { options, create, rename, setActive } : { options };
}
