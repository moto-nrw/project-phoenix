"use client";

import { QualificationEntryList } from "~/components/staff/qualification-entry-list";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { staffQualificationService } from "~/lib/staff-qualification-api";
import { staffQualificationKeys } from "~/lib/swr/staff-keys";
import { useSWRAuth } from "~/lib/swr";

/**
 * „Meine Nachweise“ im Profil (#4808): die eigenen Nachweise mit Status, zum
 * Nachlesen. Eintragen kann sie die Leitung; ohne Personalakte bleibt die
 * Karte weg.
 */
export function OwnQualificationsSection() {
  const { data, error, mutate } = useSWRAuth(staffQualificationKeys.own(), () =>
    staffQualificationService.ownEntries(),
  );
  const loadError = useSwrLoadError(error, "meine Nachweise", () => mutate());
  if (loadError) {
    return (
      <SectionCard title="Meine Nachweise">
        <LoadErrorAlert error={loadError} />
      </SectionCard>
    );
  }
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
