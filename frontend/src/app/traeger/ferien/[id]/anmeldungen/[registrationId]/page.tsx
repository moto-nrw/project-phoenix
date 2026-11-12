"use client";

// Eine Ferienanmeldung prüfen (#4663): alles, was die Eltern angegeben
// haben, mit Status, Ferienbeitrag (#4664), Umbuchen und Verlauf. Die
// Gesundheitsangaben lädt die Seite getrennt und nur mit dem Recht
// „Gesundheitsangaben sehen“ (#4722). Jedes Öffnen steht im Protokoll. Die
// Karten der Angaben wechseln mit „Ändern“ an Ort und Stelle in den
// Bearbeiten-Zustand (#5629); dessen Formulare lädt die Seite erst dann.
// Geschwister aus anderen Anmeldungen stehen mit „Trennen“ in einer eigenen
// Karte (#5628); zusammengeführt wird in der Liste.

import { useSession } from "next-auth/react";
import dynamic from "next/dynamic";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { HolidayContributionCard } from "~/components/traeger/holiday-contribution-card";
import {
  answeredQuestions,
  HolidayRegistrationAnswers,
} from "~/components/traeger/holiday-registration-answers";
import { HolidayRegistrationChildCard } from "~/components/traeger/holiday-registration-child-card";
import { HolidayRegistrationCriteriaCard } from "~/components/traeger/holiday-registration-criteria-card";
import {
  CANCELLED_EDIT_TEXT,
  editableQuestions,
} from "~/components/traeger/holiday-registration-edit-fields";
import {
  HolidayDeclarationFields,
  HolidayDeclarationTextsCard,
} from "~/components/traeger/holiday-registration-declaration";
import {
  HOLIDAY_REGISTRATION_STATUS_LABELS,
  HOLIDAY_REGISTRATION_STATUS_TONES,
  schoolNames,
} from "~/components/traeger/holiday-registration-format";
import { Alert } from "~/components/ui/alert";
import { useHolidayRegistrationPdf } from "~/components/traeger/use-holiday-registration-pdf";
import { holidayRegistrationDeletionNotice } from "~/components/traeger/holiday-registration-retention";
import { HolidayRegistrationHistory } from "~/components/traeger/holiday-registration-history";
import { HolidayRegistrationProofCard } from "~/components/traeger/holiday-registration-proof-card";
import { HolidayRegistrationStatusCard } from "~/components/traeger/holiday-registration-status-card";
import { useHolidayPeriodBillingHref } from "~/components/traeger/use-holiday-period-billing-href";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { isRefusal } from "~/components/traeger/use-traeger-confirmed-resource";
import { metadataLoadError } from "~/components/traeger/metadata-load-error";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import {
  DocumentAttachmentList,
  type AttachmentLabels,
} from "~/components/enrollment/attachment-list";
import { holidayDocumentPath } from "~/lib/traeger-holiday-form-api";
import { LegalMarkdown } from "~/components/enrollment/legal-markdown";
import { Button } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ExportMenu } from "~/components/ui/export-menu";
import { ConfirmationModal } from "~/components/ui/modal";
import NavigationLink from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { HolidayHeadFacts } from "~/components/traeger/holiday-period-head";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { childrenConcept } from "~/lib/concepts/children";
import { confirmationsConcept } from "~/lib/concepts/confirmations";
import { parentsConcept } from "~/lib/concepts/parents";
import { pickupConcept } from "~/lib/concepts/pickup";
import { formatBerlinDate, formatChatDateTime } from "~/lib/date-helpers";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import { useTraegerHolidayPeriod } from "~/components/traeger/use-traeger-resources";
import {
  fetchHolidayContribution,
  fetchHolidayFamilyDebtorNumber,
  type HolidayContribution,
} from "~/lib/traeger-contributions-api";
import {
  fetchHolidayConfirmationSettings,
  fetchHolidayRegistration,
  fetchHolidayRegistrationHealth,
  type HolidayConfirmationSettings,
  type HolidayRegistrationContact,
  type HolidayRegistrationDetail,
  type HolidayRegistrationHealthDetail,
  TRAEGER_HEALTH_RIGHT,
  withHolidayHealthAnswers,
} from "~/lib/traeger-holiday-registrations-api";
import { splitHolidaySibling } from "~/lib/traeger-sibling-groups-api";
import {
  knownChildrenKey,
  useHolidayKnownChildren,
} from "~/components/traeger/holiday-known-child";
import { traegerPath } from "~/lib/traeger-url";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { holidayWeeksConcept } from "~/lib/concepts/holidayWeeks";

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
const TRAEGER_BILLING_RIGHT = "traeger_billing:manage";

// The edit states load with the first „Ändern“, not with the page.
const editors = () => import("~/components/traeger/holiday-registration-edit");
const GuardiansEditor = dynamic(() =>
  editors().then((module) => module.GuardiansEditor),
);
const ChildEditor = dynamic(() =>
  editors().then((module) => module.ChildEditor),
);
const ContactsEditor = dynamic(() =>
  editors().then((module) => module.ContactsEditor),
);
const FamilyAnswersEditor = dynamic(() =>
  editors().then((module) => module.FamilyAnswersEditor),
);

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
  const { data: session } = useSession();
  const canSeeHealth = hasPermission(session, TRAEGER_HEALTH_RIGHT);
  const pdf = useHolidayRegistrationPdf({
    periodId,
    registrationId,
    canSeeHealth,
  });
  const canManageBillingSettings = hasPermission(
    session,
    TRAEGER_BILLING_RIGHT,
  );
  const [health, setHealth] = useState<HolidayRegistrationHealthDetail | null>(
    null,
  );
  const {
    data: period,
    loadError: periodError,
    error: periodFailure,
  } = useTraegerHolidayPeriod(periodId);
  // The way to the invoice of this period (#5688), for the billing right only.
  const billingHref = useHolidayPeriodBillingHref(period);
  const [registration, setRegistration] =
    useState<HolidayRegistrationDetail | null>(null);
  const [contribution, setContribution] = useState<HolidayContribution | null>(
    null,
  );
  // Whether the Träger asks for the debtor number (#5621).
  const [confirmation, setConfirmation] =
    useState<HolidayConfirmationSettings | null>(null);
  // The number the family already has at the Träger, proposed at the
  // confirmation (#6692).
  const [familyDebtorNumber, setFamilyDebtorNumber] = useState("");
  // The exemption the confirmation applies; it starts with the parents'
  // statement and the preview follows it.
  const [exemption, setExemption] = useState(false);
  // The cost bearer that pays instead of the family (#5627); null otherwise.
  const [costBearerId, setCostBearerId] = useState<string | null>(null);
  // While the preview is computed again, the confirmation waits for it: it
  // must freeze the amount the card shows.
  const [previewPending, setPreviewPending] = useState(false);
  const latestContribution = useRef(0);
  const latestRegistration = useRef(0);
  // The card in its edit state: "guardians", "contacts", "answers" or
  // "child:<id>"; one at a time.
  const [editing, setEditing] = useState<string | null>(null);
  const {
    schools,
    error: schoolsError,
    fetchError: schoolsFailure,
  } = useTraegerSchools();
  const registrationLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = registrationLoad;

  // Reads the registration and rejects on failure: after a write the card
  // shows that as a load error; the first load shows it as the page error.
  const fetchRegistration = useCallback(async () => {
    const request = ++latestRegistration.current;
    const refusedRead = (error: unknown): never => {
      if (request === latestRegistration.current && isRefusal(error)) {
        latestContribution.current++;
        setRegistration(null);
        setHealth(null);
        setContribution(null);
        setConfirmation(null);
        setFamilyDebtorNumber("");
        setEditing(null);
      }
      throw error;
    };
    try {
      const [
        loadedRegistration,
        loadedContribution,
        loadedHealth,
        loadedConfirmation,
        loadedFamilyDebtorNumber,
      ] = await Promise.all([
        fetchHolidayRegistration(periodId, registrationId).catch(refusedRead),
        fetchHolidayContribution(periodId, registrationId).catch(refusedRead),
        canSeeHealth
          ? fetchHolidayRegistrationHealth(periodId, registrationId).catch(
              refusedRead,
            )
          : Promise.resolve(null),
        fetchHolidayConfirmationSettings().catch(refusedRead),
        fetchHolidayFamilyDebtorNumber(periodId, registrationId).catch(
          refusedRead,
        ),
      ]);
      if (request !== latestRegistration.current) return;
      latestContribution.current++;
      setConfirmation(loadedConfirmation);
      setFamilyDebtorNumber(loadedFamilyDebtorNumber);
      setRegistration(loadedRegistration);
      setHealth(loadedHealth);
      setContribution(loadedContribution);
      setExemption(loadedRegistration.benefitsDeclared);
      setCostBearerId(null);
      clearLoadError();
    } catch (error) {
      if (request !== latestRegistration.current) return;
      throw error;
    }
  }, [periodId, registrationId, canSeeHealth, clearLoadError]);

  const load = useCallback(async () => {
    try {
      await fetchRegistration();
    } catch (err) {
      logger.error("traeger_holiday_registration_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Anmeldung",
        retry: () => void load(),
      });
    }
  }, [fetchRegistration, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  // A change of the bookings or of the exemption changes what a confirmation
  // would freeze, so the contribution is read again; only the newest answer
  // is shown.
  const refreshContribution = useCallback(
    async (withExemption: boolean, withCostBearer: string | null) => {
      const request = ++latestContribution.current;
      setPreviewPending(true);
      try {
        const next = await fetchHolidayContribution(
          periodId,
          registrationId,
          withCostBearer ? false : withExemption,
          withCostBearer ?? undefined,
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
      void refreshContribution(exemption, costBearerId);
    },
    [exemption, costBearerId, refreshContribution],
  );

  const saved = useCallback(
    (next: HolidayRegistrationDetail) => {
      setEditing(null);
      changed(next);
    },
    [changed],
  );

  // Siblings from another registration (#5628): split after asking.
  const [confirmingSplit, setConfirmingSplit] = useState(false);
  const [splitting, setSplitting] = useState(false);
  const splitErrors = useApiErrorDisplay();
  const { success } = useToast();
  // The page owns reload errors so they survive cards closing on refusal.
  // A confirmed write remains successful even if its reload fails (#6652).
  const afterWrite = useRefreshAfterWrite(
    "die Anzeige der Anmeldung",
    fetchRegistration,
  );
  const splitSiblings = async () => {
    setSplitting(true);
    try {
      await splitHolidaySibling(periodId, registrationId);
    } catch (err) {
      logger.error("holiday_registration_sibling_split_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirmingSplit(false);
      setSplitting(false);
      void splitErrors.show(err, { object: "die Geschwistergruppe" });
      return;
    }
    success("Die Anmeldung ist von den Geschwistern getrennt.");
    setConfirmingSplit(false);
    await afterWrite.refresh();
    setSplitting(false);
  };

  const changeExemption = useCallback(
    (next: boolean) => {
      setExemption(next);
      void refreshContribution(next, null);
    },
    [refreshContribution],
  );

  const changeCostBearer = useCallback(
    (next: string | null) => {
      setCostBearerId(next);
      if (next) void refreshContribution(false, next);
    },
    [refreshContribution],
  );

  // Confirmed data stays after transient failures; refusals remove it
  // (#6652).
  const loadError =
    (registration ? null : (afterWrite.error ?? registrationLoad.error)) ??
    metadataLoadError(
      periodError ?? schoolsError,
      Boolean(registration && period && schools),
      periodFailure,
      schoolsFailure,
    );
  const names = schoolNames(schools);
  const known = useHolidayKnownChildren(
    periodId,
    registrationId,
    knownChildrenKey(registration?.children),
  );
  // The health answers join the registration's answers, marked
  // "(Gesundheit)"; a child's stand in its Gesundheit box.
  const view = registration
    ? withHolidayHealthAnswers(registration, health)
    : null;
  const emergency = (registration?.contacts ?? []).filter(
    (contact) => contact.kind === "emergency",
  );
  const pickup = (registration?.contacts ?? []).filter(
    (contact) => contact.kind === "pickup",
  );

  const cancelled = registration?.status === "cancelled";
  const editAction = (section: string, label: string) =>
    cancelled || editing ? undefined : (
      <Button
        type="button"
        variant="outline"
        size="md"
        aria-label={label}
        onClick={() => setEditing(section)}
      >
        Ändern
      </Button>
    );
  const editorProps = registration
    ? {
        periodId,
        registration,
        onSaved: saved,
        onCancel: () => setEditing(null),
      }
    : null;

  // Der Titel nennt die Eltern, nicht die Datenbanknummer. Die Nummer bleibt
  // in der Statuszeile: Export und Hinweis auf Doppelanmeldungen verweisen
  // mit „Anmeldung Nr.“ auf sie.
  const title = registration
    ? `Anmeldung von ${registration.guardianFirstName} ${registration.guardianLastName}`
    : "Anmeldung";

  const deletionNotice = registration
    ? holidayRegistrationDeletionNotice(registration)
    : null;

  return (
    <TenantPage
      title={title}
      leading={<ConceptIconTile concept={holidayWeeksConcept} size="page" />}
      back
      backHref={traegerPath(`/traeger/ferien/${periodId}/anmeldungen`)}
      backLabel="Zurück zu den Anmeldungen"
      stats={
        registration ? (
          <HolidayHeadFacts>
            <StatusBadge
              tone={HOLIDAY_REGISTRATION_STATUS_TONES[registration.status]}
              label={HOLIDAY_REGISTRATION_STATUS_LABELS[registration.status]}
            />
            {registration.frozen ? (
              <StatusBadge tone="gray" label="Eingefroren" />
            ) : null}
            <span>Nr. {registration.id}</span>
            {period ? <span>{period.name}</span> : null}
            <span>
              Eingegangen am {formatBerlinDate(registration.submittedAt)}
            </span>
          </HolidayHeadFacts>
        ) : null
      }
      statsLoading={!registration && !loadError}
      actions={
        registration ? (
          <ExportMenu
            ariaLabel="Anmeldung herunterladen"
            options={pdf.options}
            busyKey={pdf.busyKey}
            onSelect={(option) => void pdf.download(option.key)}
          />
        ) : null
      }
      error={loadError}
      loading={!loadError && !(registration && period && schools)}
    >
      {registration && view && period && schools ? (
        <>
          <LoadErrorAlert error={registrationLoad.error} />
          <LoadErrorAlert error={afterWrite.error} className="mb-4" />
          {deletionNotice ? (
            <Alert type="info" message={deletionNotice} />
          ) : null}
          <HolidayRegistrationStatusCard
            periodId={periodId}
            registration={registration}
            contribution={contribution}
            previewPending={previewPending}
            debtorNumberRequired={confirmation?.debtorNumberRequired ?? false}
            debtorNumberFormat={confirmation?.debtorNumberFormat}
            familyDebtorNumber={familyDebtorNumber}
            exemption={exemption}
            onExemptionChange={changeExemption}
            costBearers={confirmation?.costBearers}
            costBearerId={costBearerId}
            onCostBearerChange={changeCostBearer}
            onChanged={changed}
            onReload={afterWrite.refresh}
            onDebtorNumberChanged={afterWrite.refresh}
            canManageBillingSettings={canManageBillingSettings}
            billingHref={billingHref}
          />
          <HolidayRegistrationCriteriaCard
            periodId={periodId}
            registration={registration}
            onChanged={changed}
          />
          {contribution?.siblings?.length ? (
            <SectionCard
              leading={<ConceptIconTile concept={childrenConcept} />}
              title="Geschwister"
              description="Die Kinder dieser Anmeldungen zählen beim Ferienbeitrag zusammen. Welches Kind das 1. Kind ist, steht in den Einstellungen unter Allgemein im Bereich Ferienbetreuung."
              action={
                contribution.siblingsInvoiced ? undefined : (
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    onClick={() => setConfirmingSplit(true)}
                  >
                    Trennen
                  </Button>
                )
              }
            >
              <div className="space-y-2 text-sm text-gray-700">
                <ul className="space-y-1">
                  {contribution.siblings.map((sibling) => (
                    <li key={sibling.registrationId} className="wrap-anywhere">
                      <NavigationLink
                        href={traegerPath(
                          `/traeger/ferien/${periodId}/anmeldungen/${sibling.registrationId}`,
                        )}
                        className="font-medium text-gray-900 underline"
                      >
                        Anmeldung Nr. {sibling.registrationId}
                      </NavigationLink>
                      {": "}
                      {sibling.children.map((child) => child.name).join(", ")}
                      {sibling.cancelled
                        ? " (storniert, zählt nicht mit)"
                        : null}
                    </li>
                  ))}
                </ul>
                {contribution.siblingsInvoiced ? (
                  <p>
                    Für eine dieser Anmeldungen gibt es schon eine Rechnung.
                    Geschwister können Sie jetzt nicht mehr ändern.
                  </p>
                ) : null}
              </div>
              <ConfirmationModal
                isOpen={confirmingSplit}
                title="Von den Geschwistern trennen?"
                confirmText="Trennen"
                isConfirmLoading={splitting}
                loadingText="Trennt …"
                onClose={() => setConfirmingSplit(false)}
                onConfirm={() => void splitSiblings()}
              >
                <p className="text-sm text-gray-700">
                  Die Kinder dieser Anmeldung zählen dann nicht mehr mit den
                  anderen Anmeldungen zusammen. moto berechnet den Ferienbeitrag
                  neu, auch einen schon bestätigten. Eltern bekommen keine
                  Nachricht von moto.
                </p>
              </ConfirmationModal>
            </SectionCard>
          ) : null}
          <HolidayRegistrationProofCard
            periodId={periodId}
            registration={registration}
            onUploaded={setRegistration}
          />
          {contribution &&
          (registration.status !== "cancelled" || contribution.frozen) ? (
            <HolidayContributionCard
              contribution={contribution}
              registration={registration}
              canManageBillingSettings={canManageBillingSettings}
            />
          ) : null}
          {editing === "guardians" && editorProps ? (
            <GuardiansEditor
              {...editorProps}
              invoiced={contribution?.invoiced ?? false}
            />
          ) : (
            <SectionCard
              leading={<ConceptIconTile concept={parentsConcept} />}
              title="Eltern"
              description={cancelled ? CANCELLED_EDIT_TEXT : undefined}
              action={editAction("guardians", "Angaben der Eltern ändern")}
            >
              <DataGrid>
                <DataField label="Name">
                  {registration.guardianFirstName}{" "}
                  {registration.guardianLastName}
                </DataField>
                <DataField label="Telefon">
                  {registration.guardianPhone}
                </DataField>
                <DataField label="E-Mail" fullWidth>
                  {registration.guardianEmail}
                </DataField>
                {registration.guardianAddress ? (
                  <DataField label="Anschrift" fullWidth>
                    {registration.guardianAddress.street},{" "}
                    {registration.guardianAddress.postalCode}{" "}
                    {registration.guardianAddress.city}
                  </DataField>
                ) : null}
                <DataField label="Datenschutz zugestimmt">
                  {consentText(registration.privacyConsentAt)}
                </DataField>
                <DataField label="Gesundheitsangaben zugestimmt">
                  {consentText(registration.healthConsentAt)}
                </DataField>
                <DataField label="Verbindlich angemeldet">
                  {consentText(registration.bindingConfirmedAt)}
                </DataField>
                <HolidayDeclarationFields registration={registration} />
              </DataGrid>
            </SectionCard>
          )}
          <HolidayDeclarationTextsCard registration={registration} />
          {registration.secondGuardian && editing !== "guardians" ? (
            <SectionCard
              leading={<ConceptIconTile concept={parentsConcept} />}
              title="Zweite sorgeberechtigte Person"
              action={editAction(
                "guardians",
                "Angaben der zweiten sorgeberechtigten Person ändern",
              )}
            >
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
          {view.children.map((child) => (
            <HolidayRegistrationChildCard
              key={child.id}
              period={period}
              registration={view}
              child={child}
              health={health?.children.find(
                (entry) => entry.childId === child.id,
              )}
              healthHidden={
                !canSeeHealth && registration.healthConsentAt !== null
              }
              names={names}
              knownSchoolId={known.get(child.id)}
              onChanged={changed}
              onEdit={
                cancelled || editing
                  ? undefined
                  : () => setEditing(`child:${child.id}`)
              }
              editor={
                editing === `child:${child.id}` && editorProps ? (
                  <ChildEditor
                    {...editorProps}
                    child={
                      registration.children.find(
                        (entry) => entry.id === child.id,
                      ) ?? child
                    }
                    schools={schools}
                  />
                ) : undefined
              }
            />
          ))}
          {editing === "answers" && editorProps ? (
            <FamilyAnswersEditor {...editorProps} />
          ) : registration.form.fields.length > 0 ? (
            <SectionCard
              title="Allgemeine Angaben"
              description={`Antworten auf die Fragen aus Formular-Version ${registration.form.version}. Antworten zu einem Kind stehen beim Kind.`}
              action={
                editableQuestions(registration.form.fields, false).length > 0
                  ? editAction("answers", "Allgemeine Angaben ändern")
                  : undefined
              }
            >
              {answeredQuestions(registration.form.fields, false, view.answers)
                .length > 0 ? (
                <HolidayRegistrationAnswers
                  questions={answeredQuestions(
                    registration.form.fields,
                    false,
                    view.answers,
                  )}
                  answers={view.answers}
                />
              ) : (
                <p className="text-sm text-gray-700">
                  Keine allgemeinen Antworten.
                </p>
              )}
            </SectionCard>
          ) : null}
          {registration.form.legalBlocks?.some((block) => block.enabled) ? (
            <SectionCard
              leading={<ConceptIconTile concept={confirmationsConcept} />}
              title="Hinweise und Einwilligungen"
            >
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
                        <p className="text-sm text-gray-700">{block.label}</p>
                        {block.text.trim() ? (
                          <div className="mt-2 text-sm text-gray-700">
                            <LegalMarkdown text={block.text} />
                          </div>
                        ) : null}
                        {block.kind === "notice" && !block.required ? null : (
                          <p className="text-sm text-gray-700">
                            {at
                              ? `Zugestimmt am ${formatChatDateTime(at)} Uhr`
                              : "Nicht zugestimmt"}
                          </p>
                        )}
                        <DocumentAttachmentList
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
          {editing === "contacts" && editorProps ? (
            <ContactsEditor {...editorProps} />
          ) : (
            <SectionCard
              leading={<ConceptIconTile concept={pickupConcept} />}
              title="Notfallkontakte und Abholung"
              action={editAction(
                "contacts",
                "Notfallkontakte und Abholung ändern",
              )}
            >
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
          )}
          <HolidayRegistrationHistory
            period={period}
            names={names}
            registration={registration}
          />
        </>
      ) : null}
    </TenantPage>
  );
}
