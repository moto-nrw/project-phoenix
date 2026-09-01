"use client";

// The option list behind the Abwesenheitsart dropdown (#2403): the five
// standard types plus whatever names the school added, in one list.
//
// Lives in the component layer rather than in lib/ because it produces
// option props for the dropdown and runs React hooks. The pure value↔request
// mapping stays in ~/lib/absence-type-select, which nothing in the UI layer
// needs to know about.

import { useCallback, useMemo } from "react";

import { absenceTypeService, type AbsenceType } from "~/lib/absence-type-api";
import {
  customIdFromOptionValue,
  customOptionValue,
} from "~/lib/absence-type-select";
import { readableApiMessage } from "~/lib/api-error-message";
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
  /** Undefined for a user without time_tracking:manage — hides the affordance. */
  readonly create?: (name: string) => Promise<string>;
  readonly rename?: (value: string, name: string) => Promise<void>;
  readonly setActive?: (value: string, isActive: boolean) => Promise<void>;
  readonly update?: (
    value: string,
    changes: {
      name?: string;
      allowanceEnabled?: boolean;
      overrunPolicy?: "warn" | "block";
    },
  ) => Promise<void>;
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
/**
 * The dropdown shows the message of a failed write as it is, so it must read
 * like a sentence to a school user, not like a status line.
 */
async function withReadableError<T>(operation: Promise<T>): Promise<T> {
  try {
    return await operation;
  } catch (err) {
    const message = readableApiMessage(err);
    if (message === null) throw err;
    throw new Error(message, { cause: err });
  }
}

export function useAbsenceTypeOptions(
  canManage: boolean,
  standardOptions: readonly AbsenceTypeOption[] = STANDARD_ABSENCE_OPTIONS,
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

  const create = useCallback(
    async (name: string) => {
      const created = await withReadableError(
        absenceTypeService.createAbsenceType(name),
      );
      await mutate((previous = []) => [...previous, created], false);
      return customOptionValue(created.id);
    },
    [mutate],
  );

  const rename = useCallback(
    async (value: string, name: string) => {
      const updated = await withReadableError(
        absenceTypeService.updateAbsenceType(customIdFromOptionValue(value), {
          name,
        }),
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
      const updated = await withReadableError(
        absenceTypeService.updateAbsenceType(customIdFromOptionValue(value), {
          isActive,
        }),
      );
      await mutate(
        (previous = []) =>
          previous.map((type) => (type.id === updated.id ? updated : type)),
        false,
      );
    },
    [mutate],
  );

  const update = useCallback(
    async (
      value: string,
      changes: {
        name?: string;
        allowanceEnabled?: boolean;
        overrunPolicy?: "warn" | "block";
      },
    ) => {
      const updated = await withReadableError(
        absenceTypeService.updateAbsenceType(
          customIdFromOptionValue(value),
          changes,
        ),
      );
      await mutate(
        (previous = []) =>
          previous.map((type) => (type.id === updated.id ? updated : type)),
        false,
      );
    },
    [mutate],
  );

  return canManage
    ? { options, create, rename, setActive, update }
    : { options };
}
