"use client";

import { ChevronRight } from "lucide-react";

import Link from "~/components/ui/navigation-link";
import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useCareWithdrawalsPending } from "~/lib/hooks/use-care-withdrawals-pending";
import { useChangeRequestsPending } from "~/lib/hooks/use-change-requests-pending";
import { useEnrollmentRequestsPending } from "~/lib/hooks/use-enrollment-requests-pending";
import { useStaffAbsencesPending } from "~/lib/hooks/use-staff-absences-pending";
import { useTenantAwarePath } from "~/lib/tenant-path";

/**
 * Baustein „Offene Anfragen" (#2180): was auf eine Entscheidung wartet und
 * sonst liegen bleibt, weil niemand die Seite aufruft.
 *
 * Die Zähler kommen aus denselben Hooks wie die Zähler in der Seitenleiste —
 * eine zweite Quelle würde zwei verschiedene Zahlen für dieselbe Sache zeigen.
 * Jede Zeile führt genau dorthin, wo entschieden wird. Zeilen ohne
 * Berechtigung liefern von sich aus keine Zahl und fallen weg.
 */
export function OpenRequestsBlock() {
  const tenantPath = useTenantAwarePath();
  const changeRequests = useChangeRequestsPending();
  const enrollmentRequests = useEnrollmentRequestsPending();
  const careWithdrawals = useCareWithdrawalsPending();
  const staffAbsences = useStaffAbsencesPending();

  const rows = [
    {
      key: "parent",
      label: "Wünsche von Eltern",
      hint: "Stammdaten, Betreuungszeiten, Krankmeldungen",
      count: changeRequests.unreadCount,
      href: tenantPath("/anfragen"),
    },
    {
      key: "enrollment",
      label: "Änderungen an Anmeldungen",
      hint: "Eingereichte Änderungen zu laufenden Anmeldungen",
      count: enrollmentRequests.unreadCount,
      href: tenantPath("/anfragen"),
    },
    {
      key: "withdrawal",
      label: "Abmeldungen aus der Betreuung",
      hint: "Angekündigte Komplett-Abmeldungen",
      count: careWithdrawals.unreadCount,
      href: tenantPath("/anfragen"),
    },
    {
      key: "staff",
      label: "Anträge des Teams",
      hint: "Urlaub und Abwesenheiten von Mitarbeitenden",
      count: staffAbsences.unreadCount,
      href: tenantPath("/anfragen"),
    },
  ].filter((row) => row.count > 0);

  return (
    <SectionCard
      title="Offene Anfragen"
      leading={<MotoConceptIcon concept="requests" size={20} />}
      className="flex h-full flex-col"
      bodyClassName="mt-4 min-h-0 flex-1 overflow-y-auto"
      actions={
        <Link
          href={tenantPath("/anfragen")}
          aria-label="Offene Anfragen: alle ansehen"
          className="flex items-center gap-1 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
        >
          Alle ansehen
          <ChevronRight className="h-4 w-4" aria-hidden="true" />
        </Link>
      }
    >
      {rows.length === 0 ? (
        <EmptyState
          className="py-8"
          title="Nichts wartet auf eine Entscheidung"
          description="Neue Anfragen von Eltern und aus dem Team erscheinen hier."
        />
      ) : (
        <ul className="space-y-2">
          {rows.map((row) => (
            <li key={row.key}>
              <Link
                href={row.href}
                className="flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 p-3 transition-colors hover:bg-gray-100/50"
              >
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium text-gray-900">
                    {row.label}
                  </span>
                  <span className="block truncate text-xs text-gray-500">
                    {row.hint}
                  </span>
                </span>
                <StatusBadge
                  tone="orange"
                  label={`${row.count} offen`}
                />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}
