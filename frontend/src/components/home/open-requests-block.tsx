"use client";

import Link from "~/components/ui/navigation-link";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { HOME_CARD_BODY, HomeCardIcon } from "~/components/home/home-card";
import { HomeCardLink } from "~/components/home/home-card-rows";
import { StatusBadge } from "~/components/ui/status-badge";
import { useCareWithdrawalsPending } from "~/lib/hooks/use-care-withdrawals-pending";
import { useChangeRequestsPending } from "~/lib/hooks/use-change-requests-pending";
import { useEnrollmentRequestsPending } from "~/lib/hooks/use-enrollment-requests-pending";
import { useEnrollmentsUnread } from "~/lib/hooks/use-enrollments-unread";
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
  const unreadEnrollments = useEnrollmentsUnread();

  const rows = [
    {
      key: "parent",
      label: "Wünsche von Eltern",
      hint: "Stammdaten, Betreuungszeiten, Krankmeldungen",
      count: changeRequests.unreadCount,
      href: tenantPath("/anfragen"),
    },
    {
      // Eine Anmeldung ist keine Anfrage (#3778), wartet aber genauso auf
      // die Leitung; die Zeile führt deshalb zu den Anmeldungen.
      key: "unread-enrollment",
      label: "Neue Anmeldungen",
      hint: "Noch nicht gelesene Anmeldungen von Eltern",
      count: unreadEnrollments.unreadCount,
      badge: "ungelesen",
      href: tenantPath("/admin/enrollments"),
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
      leading={<HomeCardIcon concept="requests" />}
      className="flex h-full flex-col"
      bodyClassName={HOME_CARD_BODY}
      inlineActions
      actions={
        <HomeCardLink
          href={tenantPath("/anfragen")}
          label="Offene Anfragen: alle ansehen"
        >
          Alle ansehen
        </HomeCardLink>
      }
    >
      {rows.length === 0 ? (
        <EmptyState
          className="py-4"
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
                  label={`${row.count} ${row.badge ?? "offen"}`}
                />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}
