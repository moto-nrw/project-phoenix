"use client";

// Shared plumbing for the Abwesenheitsart dropdown (#2403).
//
// The dropdown mixes two things that look alike and are not: the five standard
// types, which are code constants on both sides, and the school's own names,
// which are rows. Both must sit in one list — a person picking "Regenerationstag"
// should not have to know which of the two kinds it is — so this module gives
// them one option value space and converts back at the request boundary.
//
// Standard types keep their canonical value ("sick", "vacation", …). A school's
// own art is `custom:<id>`. Nothing else in the app has to know the difference.

import { useCallback, useEffect, useMemo, useState } from "react";

import type { CreatableSelectOption } from "~/components/ui/creatable-select";
import { absenceTypeService, type AbsenceType } from "~/lib/absence-type-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AbsenceTypeSelect" });

const CUSTOM_PREFIX = "custom:";

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

export function customOptionValue(id: string): string {
  return `${CUSTOM_PREFIX}${id}`;
}

/**
 * selectValueFor maps a stored absence back onto its option: the school's own
 * art when the row carries one, otherwise the canonical type.
 */
export function selectValueFor(
  absenceType: string,
  absenceTypeId?: string | null,
): string {
  return absenceTypeId ? customOptionValue(absenceTypeId) : absenceType;
}

/**
 * absenceRequestFor turns an option back into the two request fields. The base
 * type sent along with a custom art is only a hint — the backend overwrites it
 * from the art itself, which is what keeps a name from choosing its own
 * arithmetic.
 */
export function absenceRequestFor(selectValue: string): {
  absence_type: string;
  absence_type_id: number | null;
} {
  if (selectValue.startsWith(CUSTOM_PREFIX)) {
    return {
      absence_type: "other",
      absence_type_id: Number(selectValue.slice(CUSTOM_PREFIX.length)),
    };
  }
  return { absence_type: selectValue, absence_type_id: null };
}

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
  const [custom, setCustom] = useState<AbsenceType[]>([]);

  const load = useCallback(async () => {
    try {
      setCustom(await absenceTypeService.getAbsenceTypes());
    } catch (err) {
      // A failing list must not block entering a plain absence: the standard
      // types stay available and the dropdown simply shows nothing extra.
      logger.warn("absence_types_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setCustom([]);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

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

  const create = useCallback(async (name: string) => {
    const created = await absenceTypeService.createAbsenceType(name);
    setCustom((previous) => [...previous, created]);
    return customOptionValue(created.id);
  }, []);

  const rename = useCallback(async (value: string, name: string) => {
    const id = value.slice(CUSTOM_PREFIX.length);
    const updated = await absenceTypeService.updateAbsenceType(id, { name });
    setCustom((previous) =>
      previous.map((type) => (type.id === updated.id ? updated : type)),
    );
  }, []);

  const setActive = useCallback(async (value: string, isActive: boolean) => {
    const id = value.slice(CUSTOM_PREFIX.length);
    const updated = await absenceTypeService.updateAbsenceType(id, {
      isActive,
    });
    setCustom((previous) =>
      previous.map((type) => (type.id === updated.id ? updated : type)),
    );
  }, []);

  return canManage ? { options, create, rename, setActive } : { options };
}
