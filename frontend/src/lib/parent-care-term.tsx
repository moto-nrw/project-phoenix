"use client";

import { useMemo, type ReactNode } from "react";
import {
  CareFacilityTermScope,
  normalizeCareFacilityTerm,
} from "~/lib/care-facility-term";
import {
  useParentChildren,
  useParentFeatures,
} from "~/lib/hooks/use-parent-resources";

/**
 * The term texts without a child use (#6082): one account can have children
 * at schools with different terms. One term when all schools share it,
 * otherwise every term once, in the order of the child list („OGS/GTS“), so
 * no family reads a name that is wrong for one of its schools. Undefined
 * while no term is known; the texts then say „OGS“.
 */
export function accountCareTerm(
  terms: readonly (string | undefined)[],
): string | undefined {
  const distinct = [
    ...new Set(
      terms
        .filter((term): term is string => term !== undefined)
        .map(normalizeCareFacilityTerm),
    ),
  ];
  return distinct.length > 0 ? distinct.join("/") : undefined;
}

interface ParentCareTerms {
  /** The term for texts without a child. */
  readonly account: string | undefined;
  /** The term of one child's school, else the account's term. */
  readonly forChild: (studentId: string | undefined) => string | undefined;
  /** The `{careTerm}` value for one child's row in a list of children. */
  readonly termOf: (studentId: string | undefined) => string;
  /**
   * The `{careTerm}` value for a row that names a school, not a child (a
   * group message): the term of the account's children at that school.
   */
  readonly termOfSchool: (schoolName: string | undefined) => string;
}

/**
 * The care term of every linked child, from the child features the shell
 * already loads for each child (same cache entry, no extra request).
 */
export function useParentCareTerms(enabled = true): ParentCareTerms {
  const { data: children } = useParentChildren(enabled);
  const { data: features } = useParentFeatures(enabled ? children : undefined);
  return useMemo(() => {
    const byChild = new Map<string, string>();
    const bySchool = new Map<string, string>();
    for (const child of children ?? []) {
      const term = features?.[child.student_id]?.care_facility_term;
      if (term === undefined) continue;
      byChild.set(child.student_id, term);
      bySchool.set(child.school_name, term);
    }
    const account = accountCareTerm([...byChild.values()]);
    const forChild = (studentId: string | undefined) =>
      (studentId === undefined ? undefined : byChild.get(studentId)) ?? account;
    return {
      account,
      forChild,
      termOf: (studentId) => normalizeCareFacilityTerm(forChild(studentId)),
      termOfSchool: (schoolName) =>
        normalizeCareFacilityTerm(
          (schoolName === undefined ? undefined : bySchool.get(schoolName)) ??
            account,
        ),
    };
  }, [children, features]);
}

/** Gives every text inside the account's care term. */
export function ParentCareTermScope({
  enabled,
  children,
}: {
  readonly enabled: boolean;
  readonly children: ReactNode;
}) {
  const { account } = useParentCareTerms(enabled);
  return (
    <CareFacilityTermScope term={account}>{children}</CareFacilityTermScope>
  );
}

/** Gives every text inside the care term of one child's school. */
export function ParentChildCareTermScope({
  studentId,
  children,
}: {
  readonly studentId: string | undefined;
  readonly children: ReactNode;
}) {
  const { forChild } = useParentCareTerms();
  return (
    <CareFacilityTermScope term={forChild(studentId)}>
      {children}
    </CareFacilityTermScope>
  );
}
