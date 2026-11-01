"use client";

import { HolidayHealthFields } from "~/components/holiday/holiday-health-lines";
import {
  DataField,
  DataGrid,
  InfoSection,
  InfoText,
} from "~/components/ui/detail-modal-components";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { formatDate } from "~/lib/date-helpers";
import { fetchHolidayStudentHealth } from "~/lib/holiday-host-api";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { holidayKeys } from "~/lib/swr/portal-keys";

/**
 * Angaben aus der Ferienanmeldung bei einem eigenen Kind der ausrichtenden
 * OGS (#4722): Melden die Eltern ihr Kind für eine Ferienwoche an, die diese
 * OGS ausrichtet, stehen ihre Gesundheitsangaben bis zum letzten Tag der
 * Woche hier. Sie ändern die Gesundheitsinformationen der Schule nicht;
 * der Abschnitt zeigt den Eintrag der Schule daneben, damit eine Abweichung
 * auffällt. Ohne Angaben zeigt die Akte nichts.
 */
export function HolidayStudentHealthSection({
  studentId,
  schoolHealthInfo,
}: Readonly<{ studentId: string; schoolHealthInfo: string | null }>) {
  const entries = useSWRAuth(
    holidayKeys.studentHealth(studentId),
    () => fetchHolidayStudentHealth(studentId),
    { revalidateOnFocus: true },
  );
  const loadError = useSwrLoadError(
    entries.error,
    "die Angaben aus der Ferienanmeldung",
    () => entries.mutate(),
  );
  const description =
    "Diese Gesundheitsangaben haben die Eltern in der Ferienanmeldung gemacht. Die Gesundheitsinformationen des Kindes bleiben, wie sie sind.";
  if (loadError) {
    return (
      <SectionCard title="Angaben aus der Ferienanmeldung">
        <LoadErrorAlert error={loadError} />
      </SectionCard>
    );
  }
  const data = entries.data ?? [];
  if (data.length === 0) return null;
  return (
    <SectionCard
      title="Angaben aus der Ferienanmeldung"
      description={description}
    >
      <div className="flex flex-col gap-4">
        <InfoSection title="In moto eingetragen">
          <DataGrid>
            <DataField label="Gesundheitsinformationen" fullWidth>
              <InfoText>
                {schoolHealthInfo?.trim() || "Nichts eingetragen."}
              </InfoText>
            </DataField>
          </DataGrid>
        </InfoSection>
        {data.map((entry) => (
          <InfoSection
            key={entry.weekId}
            title={`Ferienwoche ${formatDate(entry.startsOn)} bis ${formatDate(entry.endsOn)}`}
          >
            <HolidayHealthFields health={entry.health} />
          </InfoSection>
        ))}
      </div>
    </SectionCard>
  );
}
