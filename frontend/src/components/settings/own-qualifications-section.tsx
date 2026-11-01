"use client";

import { QualificationEntryList } from "~/components/staff/qualification-entry-list";
import { SectionCard } from "~/components/ui/section-card";
import { staffQualificationService } from "~/lib/staff-qualification-api";
import { staffQualificationKeys } from "~/lib/swr/staff-keys";
import { useSWRAuth } from "~/lib/swr";

/**
 * „Meine Nachweise“ im Profil (#4808): die eigenen Nachweise mit Status, zum
 * Nachlesen. Eintragen kann sie die Leitung; ohne Personalakte bleibt die
 * Karte weg.
 */
export function OwnQualificationsSection() {
  const { data } = useSWRAuth(staffQualificationKeys.own(), () =>
    staffQualificationService.ownEntries(),
  );
  if (!data || data.length === 0) return null;
  return (
    <SectionCard
      title="Meine Nachweise"
      description="Nur zur Information. Einen neuen Nachweis geben Sie bei der Leitung ab."
    >
      <QualificationEntryList entries={data} />
    </SectionCard>
  );
}
