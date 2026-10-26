"use client";

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
  type HolidayGuestContact,
  type HolidayGuestSheet,
} from "~/lib/holiday-host-api";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";

function schoolLine(sheet: HolidayGuestSheet): string {
  if (sheet.homeSchoolId !== null)
    return "Kind einer anderen Schule des Trägers";
  if (sheet.guestSchoolName)
    return `Ohne Schule beim Träger (${sheet.guestSchoolName})`;
  return "Ohne Schule beim Träger";
}

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
 * ausrichtende OGS kam, zeigt die Akte nur Betreuung, Notfall und Abholung.
 * Die Angaben kommen aus der bestätigten Ferienanmeldung beim Träger. Ohne
 * das Recht, Kinderdaten zu lesen, bleibt es beim Hinweis.
 */
export function HolidayGuestSection({
  studentId,
  canRead,
}: Readonly<{ studentId: string; canRead: boolean }>) {
  // Reloads on focus and once a minute: a cancellation, changed contacts or
  // the end of the read-only days reach an open sheet without a reload.
  const sheet = useSWRAuth(
    canRead ? `holiday-guest:${studentId}` : null,
    () => fetchHolidayGuestSheet(studentId),
    { revalidateOnFocus: true, refreshInterval: 60_000 },
  );
  const loadError = useSwrLoadError(
    sheet.error,
    "die Angaben zum Feriengast",
    () => sheet.mutate(),
  );

  const description =
    "Dieses Kind kommt als Feriengast. Hier stehen nur Betreuung, Notfall und Abholung aus der Anmeldung beim Träger.";
  if (!canRead) {
    return (
      <SectionCard title="Feriengast" description={description}>
        <InfoText>
          Notfall und Abholung sieht, wer die Angaben der Kinder lesen darf.
          Bitte fragen Sie Ihre Leitung.
        </InfoText>
      </SectionCard>
    );
  }
  // A failed reload means the access ended (cancellation, expiry) or cannot
  // be checked: the cached contacts must not stay on the screen.
  const data = sheet.error ? undefined : sheet.data;
  return (
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
                {schoolLine(data)}
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
        </div>
      )}
    </SectionCard>
  );
}
