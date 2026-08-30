"use client";

import Link from "next/link";

import { TodayNoticeList } from "~/components/staff-notices/today-notice-list";
import { InfoCard } from "~/components/ui/info-card";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { fetchTodaysNotices } from "~/lib/staff-notices-api";
import type { StaffNotice } from "~/lib/staff-notices-api";
import { useSWRAuth } from "~/lib/swr";
import { useTenantAwarePath } from "~/lib/tenant-path";

// Wie viele Hinweise auf dem Dashboard ausgeschrieben stehen. Mehr als zwei
// schieben alles andere aus dem Bild; der Rest steht eine Seite weiter.
const PREVIEW_LIMIT = 2;

/**
 * "Tagesinformationen" (#2180) auf dem Dashboard: die Hinweise der Leitung,
 * die HEUTE gelten. Rendert null, wenn heute nichts anliegt — eine leere
 * Tafel wäre tägliches Rauschen. Die vollständige Liste und die Verwaltung
 * liegen unter Kommunikation -> Tagesinformationen.
 */
export function TagesinfoCard() {
  const tenantPath = useTenantAwarePath();
  const { data, mutate } = useSWRAuth<StaffNotice[]>(
    "dashboard-staff-notices",
    fetchTodaysNotices,
    { revalidateOnFocus: false, errorRetryCount: 1 },
  );

  const notices = data ?? [];
  if (notices.length === 0) return null;

  return (
    <InfoCard
      title="Tagesinformationen"
      icon={<MotoConceptIcon concept="announcements" size={20} />}
    >
      <p className="mb-3 text-sm text-gray-600">
        Hinweise der Leitung für heute.
      </p>
      <TodayNoticeList
        notices={notices.slice(0, PREVIEW_LIMIT)}
        onChanged={mutate}
        compact
      />
      {notices.length > PREVIEW_LIMIT && (
        <Link
          href={tenantPath("/tagesinformationen")}
          className="text-moto-blue-strong mt-3 inline-block text-sm font-medium hover:underline"
        >
          Alle {notices.length} Hinweise
        </Link>
      )}
    </InfoCard>
  );
}
