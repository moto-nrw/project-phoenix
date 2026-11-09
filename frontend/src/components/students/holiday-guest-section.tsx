"use client";

import { HolidayHealthFields } from "~/components/holiday/holiday-health-lines";
import { StudentCareTimesSection } from "~/components/students/care-times-section";
import {
  DataField,
  DataGrid,
  InfoSection,
  InfoText,
} from "~/components/ui/detail-modal-components";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { formatDate } from "~/lib/date-helpers";
import {
  fetchHolidayGuestSheet,
  formatHolidayHostAnswer,
  holidayGuestLabel,
  type HolidayGuestContact,
} from "~/lib/holiday-host-api";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { studentKeys } from "~/lib/swr/portal-keys";

function ContactList({
  contacts,
  empty,
}: Readonly<{ contacts: HolidayGuestContact[]; empty: string }>) {
  if (contacts.length === 0) return <InfoText>{empty}</InfoText>;
  return (
    <DataGrid columns={1}>
      {contacts.map((contact) => (
        <DataField
          key={`${contact.name}-${contact.phone}`}
          label={
            contact.relationship
              ? `${contact.name} (${contact.relationship})`
              : contact.name
          }
        >
          {contact.phone ? (
            <a
              className="underline"
              href={`tel:${contact.phone.replaceAll(" ", "")}`}
            >
              {contact.phone}
            </a>
          ) : (
            "Keine Telefonnummer angegeben"
          )}
        </DataField>
      ))}
    </DataGrid>
  );
}

/**
 * Feriengast in der Kindakte (#4666): Für ein Kind, das als Feriengast in die
 * ausrichtende OGS kam, zeigt die Akte nur Betreuung, Notfall und Abholung,
 * dazu die Antworten, die der Träger freigibt (#5202), und die
 * Gesundheitsangaben, wenn die Eltern zugestimmt haben (#4722). Die Angaben
 * stehen auch im Gesundheitsfeld des Kindes. Die Angaben kommen aus
 * der bestätigten Ferienanmeldung beim Träger. Ohne
 * das Recht, Kinderdaten zu lesen, bleibt es beim Hinweis.
 */
export function HolidayGuestSection({
  studentId,
  canRead,
}: Readonly<{ studentId: string; canRead: boolean }>) {
  // Ein Tageswechsel bei offener Seite verschiebt den Vergleich mit (#5994).
  const today = useBerlinToday();
  // Reloads on focus and once a minute: a cancellation, changed contacts or
  // the end of the read-only days reach an open sheet without a reload.
  const sheet = useSWRAuth(
    canRead ? studentKeys.holidayGuest(studentId) : null,
    () => fetchHolidayGuestSheet(studentId),
    { revalidateOnFocus: true, refreshInterval: 60_000 },
  );
  const loadError = useSwrLoadError(
    sheet.error,
    "die Angaben zum Feriengast",
    () => sheet.mutate(),
  );

  const description =
    "Dieses Kind kommt als Feriengast. Hier stehen die Angaben, die der Träger aus der Anmeldung an Sie weitergibt.";
  if (!canRead) {
    return (
      <SectionCard title="Feriengast" description={description}>
        <InfoText>
          Notfall, Abholung und Gesundheit sieht, wer die Angaben der Kinder
          lesen darf. Bitte fragen Sie Ihre Leitung.
        </InfoText>
      </SectionCard>
    );
  }
  // A failed reload means the access ended (cancellation, expiry) or cannot
  // be checked: the cached contacts must not stay on the screen.
  const data = sheet.error ? undefined : sheet.data;
  // Soll und Ist der Ferienwoche (#5994): erst ab ihrem ersten Tag, denn
  // vorher ist das Kind noch nicht in der Schule.
  const careTimes =
    data && data.startsOn <= today
      ? { from: data.startsOn, to: data.endsOn < today ? data.endsOn : today }
      : undefined;
  return (
    <>
      <SectionCard title="Feriengast" description={description}>
        {loadError && <LoadErrorAlert error={loadError} />}
        {!data && !loadError && <Skeleton className="h-40 w-full" />}
        {data && (
          <div className="flex flex-col gap-4">
            {data.readOnly && (
              <InfoText>
                Die Ferienwoche ist vorbei. Die Angaben bleiben noch 7 Tage zum
                Nachlesen und werden dann gelöscht.
              </InfoText>
            )}
            <InfoSection title="Betreuung">
              <DataGrid>
                <DataField label="Ferienwoche">
                  {`${formatDate(data.startsOn)} bis ${formatDate(data.endsOn)}`}
                </DataField>
                <DataField label="Zeitmodell">
                  {`${data.timeModelName} (${data.timeModelStartsAt} bis ${data.timeModelEndsAt} Uhr)`}
                </DataField>
                <DataField label="Schule" fullWidth>
                  {holidayGuestLabel(data.schoolName)}
                </DataField>
              </DataGrid>
            </InfoSection>
            <InfoSection title="Notfall">
              <ContactList
                contacts={data.emergencyContacts}
                empty="Kein Notfallkontakt angegeben."
              />
            </InfoSection>
            <InfoSection title="Abholung">
              {data.pickupContacts.length > 0 && (
                <InfoText>Außer den Eltern dürfen abholen:</InfoText>
              )}
              <ContactList
                contacts={data.pickupContacts}
                empty="Nur die Eltern dürfen das Kind abholen."
              />
            </InfoSection>
            {data.health && (
              <InfoSection title="Gesundheit">
                <HolidayHealthFields health={data.health} />
              </InfoSection>
            )}
            {data.answers.length > 0 && (
              <InfoSection title="Angaben aus der Ferienanmeldung">
                <DataGrid>
                  {data.answers.map((answer) => (
                    <DataField key={answer.key} label={answer.label}>
                      {formatHolidayHostAnswer(answer)}
                    </DataField>
                  ))}
                </DataGrid>
              </InfoSection>
            )}
          </div>
        )}
      </SectionCard>
      {careTimes && (
        <StudentCareTimesSection
          key={`${careTimes.from}/${careTimes.to}`}
          studentId={studentId}
          active
          initialRange={careTimes}
          refreshKey={`${data?.timeModelStartsAt}-${data?.timeModelEndsAt}`}
        />
      )}
    </>
  );
}
