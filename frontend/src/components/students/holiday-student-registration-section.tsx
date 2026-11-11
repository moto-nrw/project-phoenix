"use client";

import { HolidayHealthFields } from "~/components/holiday/holiday-health-lines";
import { HolidayContactList } from "~/components/students/holiday-guest-section";
import {
  DataField,
  DataGrid,
  InfoSection,
  InfoText,
} from "~/components/ui/detail-modal-components";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import { holidayWeeksConcept } from "~/lib/concepts/holidayWeeks";
import { formatDate } from "~/lib/date-helpers";
import { holidayDayList } from "~/lib/holiday-days";
import {
  fetchHolidayStudentHealth,
  fetchHolidayStudentRegistrations,
  type HolidayStudentHealth,
  type HolidayStudentRegistration,
} from "~/lib/holiday-host-api";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { holidayKeys } from "~/lib/swr/portal-keys";

// Nach dem letzten Tag der Woche oder einer Stornierung verschwinden die
// Angaben auch in einem offenen Tab, wie beim Feriengast.
const REFRESH = { revalidateOnFocus: true, refreshInterval: 60_000 };

/** Eine Ferienwoche mit dem, was die Anmeldung für sie angibt. */
interface RegistrationWeek {
  readonly weekId: string;
  readonly startsOn: string;
  readonly endsOn: string;
  readonly registration?: HolidayStudentRegistration;
  readonly health?: HolidayStudentHealth;
}

/** Führt Betreuung und Gesundheit je Woche zusammen, die früheste zuerst. */
function registrationWeeks(
  registrations: readonly HolidayStudentRegistration[],
  health: readonly HolidayStudentHealth[],
): RegistrationWeek[] {
  const weeks = new Map<string, RegistrationWeek>();
  for (const registration of registrations) {
    weeks.set(registration.weekId, {
      weekId: registration.weekId,
      startsOn: registration.startsOn,
      endsOn: registration.endsOn,
      registration,
    });
  }
  for (const entry of health) {
    weeks.set(entry.weekId, {
      weekId: entry.weekId,
      startsOn: entry.startsOn,
      endsOn: entry.endsOn,
      ...weeks.get(entry.weekId),
      health: entry,
    });
  }
  return [...weeks.values()].sort((a, b) =>
    a.startsOn.localeCompare(b.startsOn),
  );
}

function CareFields({
  registration,
}: Readonly<{ registration: HolidayStudentRegistration }>) {
  return (
    <InfoSection title="Betreuung">
      <DataGrid>
        <DataField label="Zeitmodell">
          {`${registration.timeModelName} (${registration.timeModelStartsAt} bis ${registration.timeModelEndsAt} Uhr)`}
        </DataField>
        <DataField label="Gebuchte Tage">
          {registration.days.length > 0
            ? holidayDayList(registration.days)
            : "ganze Woche"}
        </DataField>
        <DataField label="Im Check-in" fullWidth>
          Der Check-in erwartet das Kind an den gebuchten Tagen.
        </DataField>
      </DataGrid>
    </InfoSection>
  );
}

/**
 * Angaben aus der Ferienanmeldung bei einem eigenen Kind der ausrichtenden
 * OGS (#4722, #6678): Melden die Eltern ihr Kind für eine Ferienwoche an,
 * die diese OGS ausrichtet, stehen Betreuung, Notfall, Abholung und die
 * Gesundheitsangaben aus der Anmeldung bis zum letzten Tag der Woche hier.
 * Sie ändern die Stammdaten der Schule nicht; der Abschnitt zeigt den
 * Gesundheitseintrag der Schule daneben, damit eine Abweichung auffällt.
 * Ohne Angaben zeigt die Akte nichts.
 */
export function HolidayStudentRegistrationSection({
  studentId,
  schoolHealthInfo,
}: Readonly<{ studentId: string; schoolHealthInfo: string | null }>) {
  const registrations = useSWRAuth(
    holidayKeys.studentRegistration(studentId),
    () => fetchHolidayStudentRegistrations(studentId),
    REFRESH,
  );
  const health = useSWRAuth(
    holidayKeys.studentHealth(studentId),
    () => fetchHolidayStudentHealth(studentId),
    REFRESH,
  );
  const registrationError = useSwrLoadError(
    registrations.error,
    "Betreuung, Notfall und Abholung aus der Ferienanmeldung",
    () => registrations.mutate(),
  );
  const healthError = useSwrLoadError(
    health.error,
    "die Gesundheitsangaben aus der Ferienanmeldung",
    () => health.mutate(),
  );
  // A failed reload means the access ended or cannot be checked: cached
  // contacts must not stay on the screen.
  const weeks = registrationWeeks(
    registrations.error ? [] : (registrations.data ?? []),
    health.error ? [] : (health.data ?? []),
  );
  if (weeks.length === 0 && !registrationError && !healthError) return null;

  return (
    <SectionCard
      title="Angaben aus der Ferienanmeldung"
      concept={holidayWeeksConcept}
      description="Das haben die Eltern für die Ferienbetreuung angegeben. Die Stammdaten des Kindes ändern sich nicht."
    >
      <div className="flex flex-col gap-6">
        {registrationError && <LoadErrorAlert error={registrationError} />}
        {healthError && <LoadErrorAlert error={healthError} />}
        {weeks.map((week) => (
          <div key={week.weekId} className="flex flex-col gap-4">
            <SectionLabel as="h3">
              {`Ferienwoche ${formatDate(week.startsOn)} bis ${formatDate(week.endsOn)}`}
            </SectionLabel>
            {week.registration && (
              <>
                <CareFields registration={week.registration} />
                <InfoSection title="Notfall">
                  <HolidayContactList
                    contacts={week.registration.emergencyContacts}
                    empty="Kein Notfallkontakt angegeben."
                  />
                </InfoSection>
                <InfoSection title="Abholung">
                  {week.registration.pickupContacts.length > 0 && (
                    <InfoText>Außer den Eltern dürfen abholen:</InfoText>
                  )}
                  <HolidayContactList
                    contacts={week.registration.pickupContacts}
                    empty="Nur die Eltern dürfen das Kind abholen."
                  />
                </InfoSection>
              </>
            )}
            {week.health && (
              <InfoSection title="Gesundheit">
                <HolidayHealthFields health={week.health.health} />
              </InfoSection>
            )}
          </div>
        ))}
        {weeks.some((week) => week.health) && (
          <InfoSection title="In moto eingetragen">
            <DataGrid>
              <DataField label="Gesundheitsinformationen" fullWidth>
                <InfoText>
                  {schoolHealthInfo?.trim() || "Nichts eingetragen."}
                </InfoText>
              </DataField>
            </DataGrid>
          </InfoSection>
        )}
      </div>
    </SectionCard>
  );
}
