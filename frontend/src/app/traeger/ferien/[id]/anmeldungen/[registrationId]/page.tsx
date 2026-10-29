"use client";

// Eine Ferienanmeldung prüfen (#4663): alles, was die Eltern angegeben
// haben, mit Status, Ferienbeitrag (#4664), Umbuchen und Verlauf. Jedes
// Öffnen steht im Protokoll.

import { useParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { HolidayContributionCard } from "~/components/traeger/holiday-contribution-card";
import {
  answeredQuestions,
  HolidayRegistrationAnswers,
} from "~/components/traeger/holiday-registration-answers";
import { HolidayRegistrationChildCard } from "~/components/traeger/holiday-registration-child-card";
import {
  HOLIDAY_REGISTRATION_STATUS_LABELS,
  HOLIDAY_REGISTRATION_STATUS_TONES,
  schoolNames,
} from "~/components/traeger/holiday-registration-format";
import { HolidayRegistrationHistory } from "~/components/traeger/holiday-registration-history";
import { HolidayRegistrationStatusCard } from "~/components/traeger/holiday-registration-status-card";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import {
  AttachmentList,
  type AttachmentLabels,
} from "~/components/enrollment/attachment-list";
import { holidayDocumentPath } from "~/lib/traeger-holiday-form-api";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { formatBerlinDate, formatChatDateTime } from "~/lib/date-helpers";
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

const PDF_LABELS: AttachmentLabels = {
  open: "Öffnen",
  download: "Herunterladen",
  openLabel: (label) => `${label} öffnen`,
  downloadLabel: (label) => `${label} herunterladen`,
  popupBlocked:
    "Das PDF konnte nicht geöffnet werden. Bitte erlauben Sie neue Fenster.",
  errorObject: "die PDF-Datei",
};

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
  // The exemption the confirmation applies; it starts with the parents'
  // statement and the preview follows it.
  const [exemption, setExemption] = useState(false);
  // While the preview is computed again, the confirmation waits for it: it
  // must freeze the amount the card shows.
  const [previewPending, setPreviewPending] = useState(false);
  const latestContribution = useRef(0);
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
      latestContribution.current++;
      setPeriod(loadedPeriod);
      setRegistration(loadedRegistration);
      setContribution(loadedContribution);
      setExemption(loadedRegistration.benefitsDeclared);
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

  // A change of the bookings or of the exemption changes what a confirmation
  // would freeze, so the contribution is read again; only the newest answer
  // is shown.
  const refreshContribution = useCallback(
    async (withExemption: boolean) => {
      const request = ++latestContribution.current;
      setPreviewPending(true);
      try {
        const next = await fetchHolidayContribution(
          periodId,
          registrationId,
          withExemption,
        );
        if (request === latestContribution.current) {
          setContribution(next);
          setPreviewPending(false);
        }
      } catch (err) {
        if (request !== latestContribution.current) return;
        setPreviewPending(false);
        logger.warn("traeger_holiday_contribution_refresh_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        await showLoadError(err, {
          object: "der Ferienbeitrag",
          retry: () => void load(),
        });
      }
    },
    [load, periodId, registrationId, showLoadError],
  );

  const changed = useCallback(
    (next: HolidayRegistrationDetail) => {
      setRegistration(next);
      void refreshContribution(exemption);
    },
    [exemption, refreshContribution],
  );

  const changeExemption = useCallback(
    (next: boolean) => {
      setExemption(next);
      void refreshContribution(next);
    },
    [refreshContribution],
  );

  const loadError = registrationLoad.error ?? schoolsError;
  const names = schoolNames(schools);
  const emergency = (registration?.contacts ?? []).filter(
    (contact) => contact.kind === "emergency",
  );
  const pickup = (registration?.contacts ?? []).filter(
    (contact) => contact.kind === "pickup",
  );

  // Der Titel nennt die Eltern, nicht die Datenbanknummer. Die Nummer bleibt
  // in der Statuszeile: Export und Hinweis auf Doppelanmeldungen verweisen
  // mit „Anmeldung Nr.“ auf sie.
  const title = registration
    ? `Anmeldung von ${registration.guardianFirstName} ${registration.guardianLastName}`
    : "Anmeldung";

  return (
    <TenantPage
      title={title}
      back
      backHref={traegerPath(`/traeger/ferien/${periodId}/anmeldungen`)}
      backLabel="Zurück zu den Anmeldungen"
      stats={
        registration ? (
          <span className="inline-flex flex-wrap items-center gap-x-2 gap-y-1">
            <StatusBadge
              tone={HOLIDAY_REGISTRATION_STATUS_TONES[registration.status]}
              label={HOLIDAY_REGISTRATION_STATUS_LABELS[registration.status]}
            />
            {registration.frozen ? (
              <StatusBadge tone="gray" label="Eingefroren" />
            ) : null}
            <span>
              Nr. {registration.id} · {period ? `${period.name} · ` : ""}
              Eingegangen am {formatBerlinDate(registration.submittedAt)}
            </span>
          </span>
        ) : null
      }
      statsLoading={!registration && !loadError}
      error={loadError}
      loading={!loadError && !(registration && period && schools)}
    >
      {registration && period && schools ? (
        <>
          <HolidayRegistrationStatusCard
            periodId={periodId}
            registration={registration}
            contribution={contribution}
            previewPending={previewPending}
            exemption={exemption}
            onExemptionChange={changeExemption}
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
          {registration.secondGuardian ? (
            <SectionCard title="Zweite sorgeberechtigte Person">
              <DataGrid>
                <DataField label="Name">
                  {registration.secondGuardian.firstName}{" "}
                  {registration.secondGuardian.lastName}
                </DataField>
                <DataField label="E-Mail">
                  {registration.secondGuardian.email || "Nicht angegeben"}
                </DataField>
                <DataField label="Telefon">
                  {registration.secondGuardian.phone || "Nicht angegeben"}
                </DataField>
              </DataGrid>
            </SectionCard>
          ) : null}
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
          {registration.form.fields.length > 0 ? (
            <SectionCard
              title="Weitere Angaben zur Familie"
              description={`Antworten auf die Fragen des Formulars, Version ${registration.form.version}. Antworten zu einem Kind stehen beim Kind.`}
            >
              {answeredQuestions(
                registration.form.fields,
                false,
                registration.answers,
              ).length > 0 ? (
                <HolidayRegistrationAnswers
                  questions={answeredQuestions(
                    registration.form.fields,
                    false,
                    registration.answers,
                  )}
                  answers={registration.answers}
                />
              ) : (
                <p className="text-sm text-gray-700">
                  Keine Antworten zur Familie.
                </p>
              )}
            </SectionCard>
          ) : null}
          {registration.form.legalBlocks?.some((block) => block.enabled) ? (
            <SectionCard title="Hinweise und Einwilligungen">
              <div className="space-y-4">
                {registration.form.legalBlocks
                  .filter((block) => block.enabled)
                  .map((block) => {
                    const at = registration.legalAcceptances?.[block.key];
                    return (
                      <div key={block.key}>
                        <h3 className="text-sm font-semibold text-gray-900">
                          {block.title}
                        </h3>
                        {block.kind === "notice" && !block.required ? null : (
                          <p className="text-sm text-gray-700">
                            {at
                              ? `Zugestimmt am ${formatChatDateTime(at)} Uhr`
                              : "Nicht zugestimmt"}
                          </p>
                        )}
                        <AttachmentList
                          labels={PDF_LABELS}
                          attachments={block.attachments}
                          documentURL={(documentId) =>
                            holidayDocumentPath(periodId, documentId)
                          }
                        />
                      </div>
                    );
                  })}
              </div>
            </SectionCard>
          ) : null}
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
      ) : null}
    </TenantPage>
  );
}
