"use client";

// Eine Ferienanmeldung prüfen (#4663): alles, was die Eltern angegeben
// haben, mit Status, Ferienbeitrag (#4664), Umbuchen und Verlauf. Jedes
// Öffnen steht im Protokoll.

import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { HolidayContributionCard } from "~/components/traeger/holiday-contribution-card";
import { HolidayRegistrationChildCard } from "~/components/traeger/holiday-registration-child-card";
import {
  HOLIDAY_REGISTRATION_STATUS_LABELS,
  HOLIDAY_REGISTRATION_STATUS_TONES,
  schoolNames,
} from "~/components/traeger/holiday-registration-format";
import { HolidayRegistrationHistory } from "~/components/traeger/holiday-registration-history";
import { HolidayRegistrationStatusCard } from "~/components/traeger/holiday-registration-status-card";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { BackLink } from "~/components/ui/back-link";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Loading } from "~/components/ui/loading";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiLoadError } from "~/contexts/ToastContext";
import { formatBerlinDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { fetchHolidayPeriod, type HolidayPeriod } from "~/lib/traeger-api";
import {
  fetchHolidayContribution,
  type HolidayContribution,
} from "~/lib/traeger-contributions-api";
import {
  fetchHolidayRegistration,
  type HolidayRegistrationContact,
  type HolidayRegistrationDetail,
} from "~/lib/traeger-holiday-registrations-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerHolidayRegistrationPage" });

function consentText(at: string | null): string {
  return at ? `Ja, am ${formatBerlinDate(at)}` : "Nein";
}

function ContactList({
  contacts,
  empty,
}: Readonly<{
  contacts: readonly HolidayRegistrationContact[];
  empty: string;
}>) {
  if (contacts.length === 0) {
    return <p className="text-sm text-gray-700">{empty}</p>;
  }
  return (
    <ul className="space-y-1 text-sm text-gray-900">
      {contacts.map((contact, index) => (
        <li key={`${contact.name}-${index}`}>
          <span className="font-medium">{contact.name}</span>
          {contact.relationship ? ` (${contact.relationship})` : ""}
          {contact.phone ? `, ${contact.phone}` : ""}
        </li>
      ))}
    </ul>
  );
}

export default function TraegerHolidayRegistrationPage() {
  const params = useParams<{ id: string; registrationId: string }>();
  const periodId = params?.id ?? "";
  const registrationId = params?.registrationId ?? "";
  const [period, setPeriod] = useState<HolidayPeriod | null>(null);
  const [registration, setRegistration] =
    useState<HolidayRegistrationDetail | null>(null);
  const [contribution, setContribution] = useState<HolidayContribution | null>(
    null,
  );
  const { schools, error: schoolsError } = useTraegerSchools();
  const registrationLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = registrationLoad;

  const load = useCallback(async () => {
    try {
      const [loadedPeriod, loadedRegistration, loadedContribution] =
        await Promise.all([
          fetchHolidayPeriod(periodId),
          fetchHolidayRegistration(periodId, registrationId),
          fetchHolidayContribution(periodId, registrationId),
        ]);
      setPeriod(loadedPeriod);
      setRegistration(loadedRegistration);
      setContribution(loadedContribution);
      clearLoadError();
    } catch (err) {
      logger.error("traeger_holiday_registration_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Anmeldung",
        retry: () => void load(),
      });
    }
  }, [periodId, registrationId, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  // A change of the status or the bookings changes what a confirmation
  // would freeze, so the contribution is read again with it.
  const changed = useCallback(
    (next: HolidayRegistrationDetail) => {
      setRegistration(next);
      fetchHolidayContribution(periodId, registrationId)
        .then(setContribution)
        .catch(async (err: unknown) => {
          logger.warn("traeger_holiday_contribution_refresh_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          await showLoadError(err, {
            object: "der Ferienbeitrag",
            retry: () => void load(),
          });
        });
    },
    [load, periodId, registrationId, showLoadError],
  );

  const loadError = registrationLoad.error ?? schoolsError;
  const names = schoolNames(schools);
  const emergency = (registration?.contacts ?? []).filter(
    (contact) => contact.kind === "emergency",
  );
  const pickup = (registration?.contacts ?? []).filter(
    (contact) => contact.kind === "pickup",
  );

  return (
    <div className="w-full space-y-6">
      <div className="space-y-2">
        <BackLink
          href={traegerPath(`/traeger/ferien/${periodId}/anmeldungen`)}
          label="Zurück zu den Anmeldungen"
        />
        {registration ? (
          <>
            <div className="flex flex-wrap items-center gap-3">
              <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
                Anmeldung Nr. {registration.id}
              </h1>
              <StatusBadge
                tone={HOLIDAY_REGISTRATION_STATUS_TONES[registration.status]}
                label={HOLIDAY_REGISTRATION_STATUS_LABELS[registration.status]}
              />
              {registration.frozen ? (
                <StatusBadge tone="gray" label="Eingefroren" />
              ) : null}
            </div>
            <p className="text-sm text-gray-500">
              {period ? `${period.name} · ` : ""}Eingegangen am{" "}
              {formatBerlinDate(registration.submittedAt)}
            </p>
          </>
        ) : null}
      </div>

      {loadError ? (
        <LoadErrorAlert error={loadError} />
      ) : registration && period && schools ? (
        <>
          <HolidayRegistrationStatusCard
            periodId={periodId}
            registration={registration}
            contribution={contribution}
            onChanged={changed}
            onConfirmed={load}
          />
          {contribution &&
          (registration.status !== "cancelled" || contribution.frozen) ? (
            <HolidayContributionCard
              contribution={contribution}
              registration={registration}
            />
          ) : null}
          <SectionCard title="Eltern">
            <DataGrid>
              <DataField label="Name">
                {registration.guardianFirstName} {registration.guardianLastName}
              </DataField>
              <DataField label="Telefon">
                {registration.guardianPhone}
              </DataField>
              <DataField label="E-Mail" fullWidth>
                {registration.guardianEmail}
              </DataField>
              <DataField label="Datenschutz zugestimmt">
                {consentText(registration.privacyConsentAt)}
              </DataField>
              <DataField label="Gesundheitsangaben zugestimmt">
                {consentText(registration.healthConsentAt)}
              </DataField>
              <DataField label="Verbindlich angemeldet">
                {consentText(registration.bindingConfirmedAt)}
              </DataField>
            </DataGrid>
          </SectionCard>
          {registration.children.map((child) => (
            <HolidayRegistrationChildCard
              key={child.id}
              period={period}
              registration={registration}
              child={child}
              health={registration.health.find(
                (entry) => entry.childId === child.id,
              )}
              names={names}
              onChanged={changed}
            />
          ))}
          <SectionCard title="Notfallkontakte und Abholung">
            <div className="space-y-4">
              <div className="space-y-1">
                <p className="text-sm font-semibold text-gray-900">
                  Notfallkontakte
                </p>
                <ContactList contacts={emergency} empty="Keine Angabe." />
              </div>
              <div className="space-y-1">
                <p className="text-sm font-semibold text-gray-900">
                  Abholberechtigte
                </p>
                <ContactList
                  contacts={pickup}
                  empty="Keine Angabe. Dann holen nur die Eltern ab."
                />
              </div>
            </div>
          </SectionCard>
          <HolidayRegistrationHistory
            period={period}
            registration={registration}
          />
        </>
      ) : (
        <Loading fullPage={false} />
      )}
    </div>
  );
}
