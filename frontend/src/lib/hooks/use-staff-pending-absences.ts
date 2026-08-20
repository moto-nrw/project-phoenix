"use client";

import { useSession } from "next-auth/react";

import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { staffAbsenceService, type StaffAbsenceRow } from "~/lib/staff-api";
import { useSWRAuth } from "~/lib/swr";

// SWR key for the tenant-wide open-request list. Deliberately shares the
// "staff-pending-absences-" prefix with the per-staff detail-tab badge so the
// existing includes()-mutate in abwesenheiten-tab.tsx invalidates both.
const STAFF_PENDING_ABSENCES_KEY = "staff-pending-absences-all";

/**
 * Tenant-wide open absence requests. Feeds the per-card pending indicators on
 * the Mitarbeiter page and the counter on its pointer to the Anfragen module
 * (#2433; before that the inbox that sat there, #1419). Gated on
 * vacation:approve — matching the backend endpoint — so non-approvers never
 * fire the request.
 */
export function useStaffPendingAbsences() {
  const { data: session } = useSession();
  const canReview =
    isAdmin(session) || hasPermission(session, "vacation:approve");
  const { data } = useSWRAuth<StaffAbsenceRow[]>(
    canReview ? STAFF_PENDING_ABSENCES_KEY : null,
    () => staffAbsenceService.listPending(),
    { revalidateOnFocus: true },
  );
  return { rows: data ?? [], canReview };
}
