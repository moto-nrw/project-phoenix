"use client";

// Tagesinformationen auf der Klassenansicht (#2208): die Hinweise der
// OGS-Leitung, die heute für Lehrkräfte gelten, als Karte oben auf der
// Startseite — damit sie einem begegnen, statt gesucht werden zu müssen. Ohne
// Hinweis rendert die Karte nichts, statt täglich leer dazustehen. Bestätigt
// wird auf der eigenen Seite; hier steht nur, was heute ansteht.

import { useSession } from "next-auth/react";

import NavigationLink from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  SCHOOL_NOTICES_TODAY_KEY,
  schoolStaffNoticesApi,
} from "~/lib/school-staff-notices-api";
import { schoolPath } from "~/lib/school-url";
import type { StaffNotice } from "~/lib/staff-notices-api";
import { useSWRAuth } from "~/lib/swr";

const SCHOOL_NOTICES_ROUTE = "/school/tagesinformationen";

export function TodayNoticesCard() {
  const { data: session } = useSession();
  const { data } = useSWRAuth<StaffNotice[]>(
    session ? SCHOOL_NOTICES_TODAY_KEY : null,
    schoolStaffNoticesApi.fetchTodaysNotices,
    { revalidateOnFocus: false },
  );
  const notices = data ?? [];
  if (notices.length === 0) return null;

  const pending = notices.filter(
    (n) => n.requires_acknowledgement && !n.acknowledged_at,
  ).length;

  return (
    <SectionCard
      title="Tagesinformationen der OGS"
      testId="school-today-notices"
      action={
        <NavigationLink
          href={schoolPath(SCHOOL_NOTICES_ROUTE)}
          className="text-sm font-medium text-gray-700 underline-offset-4 hover:underline"
        >
          Alle anzeigen
        </NavigationLink>
      }
    >
      <ul className="space-y-2">
        {notices.map((notice) => (
          <li key={notice.id} className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-medium text-gray-900">
              {notice.title}
            </span>
            {notice.priority === "important" && (
              <StatusBadge label="Wichtig" tone="orange" />
            )}
            {notice.requires_acknowledgement && !notice.acknowledged_at && (
              <StatusBadge label="Bitte bestätigen" tone="blue" />
            )}
          </li>
        ))}
      </ul>
      {pending > 0 && (
        <p className="mt-3 text-sm text-gray-500">
          {pending === 1
            ? "Ein Hinweis wartet auf Ihre Kenntnisnahme."
            : `${pending} Hinweise warten auf Ihre Kenntnisnahme.`}
        </p>
      )}
    </SectionCard>
  );
}
