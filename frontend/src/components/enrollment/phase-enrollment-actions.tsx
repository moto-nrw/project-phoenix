"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
import {
  createLateInvite,
  createManualApprovedEnrollment,
  fetchManualEnrollmentBootstrap,
  type AdminExistingChildCandidate,
  type ExistingChildChoice,
} from "~/lib/enrollment-admin-api";
import type {
  SubmitEnrollmentPayload,
  SubmitEnrollmentResult,
} from "~/lib/enrollment-submission-api";
import type { Phase } from "~/lib/enrollment-phase-api";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceModal } from "~/components/ui/choice-modal";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { Modal } from "~/components/ui/modal";
import {
  SlideOver,
  SlideOverContent,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { EnrollmentForm } from "~/components/enrollment/enrollment-form";
import {
  prefetchedDataFromBootstrap,
  type EnrollmentFormPrefetchedData,
} from "~/components/enrollment/enrollment-form-config";
import { PublicLinkCopyButton } from "~/components/enrollment/public-link-copy-button";
import {
  existingChildChoiceDescription,
  existingChildChoiceOptions,
  sendWithExistingChildChoice,
} from "~/components/enrollment/existing-child-choice";
import {
  type CareOfferingBookingStats,
  fetchCareOfferingBookingStats,
} from "~/lib/care-offering-booking-stats";
import { createLogger } from "~/lib/logger";
import { withLateInviteQuery } from "~/lib/enrollment-http";
import { Loading } from "~/components/ui/loading";

const logger = createLogger({ component: "PhaseEnrollmentActions" });

export function LateInviteModal({
  isOpen,
  onClose,
  phase,
  phaseUrl,
}: Readonly<{
  isOpen: boolean;
  onClose: () => void;
  phase: Phase;
  phaseUrl: string;
}>) {
  const [guardianEmail, setGuardianEmail] = useState("");
  const [reason, setReason] = useState("");
  const [generatedUrl, setGeneratedUrl] = useState("");
  const [loading, setLoading] = useState(false);
  const errors = useApiFormError();
  const clearErrors = errors.clear;
  // „Wiederholen“ sendet den aktuellen Stand, nicht den vom Fehler.
  const latestCreateRef = useRef<() => Promise<void>>(async () => undefined);

  useEffect(() => {
    if (!isOpen) return;
    setGuardianEmail("");
    setReason("");
    setGeneratedUrl("");
    clearErrors();
    setLoading(false);
  }, [isOpen, clearErrors]);

  const create = async () => {
    errors.clear();
    if (!phaseUrl) {
      errors.invalid(
        "Der Link zur Anmeldephase fehlt. Bitte laden Sie die Seite neu.",
      );
      return;
    }
    setLoading(true);
    try {
      const result = await createLateInvite(phase.id, {
        guardian_email: guardianEmail.trim(),
        reason: reason.trim() || undefined,
      });
      setGeneratedUrl(withLateInviteQuery(phaseUrl, result.token));
    } catch (err) {
      logger.warn("late_invite_create_failed", {
        error: err instanceof Error ? err.message : "unknown",
      });
      await errors.show(err, {
        object: "die Einladung",
        retry: () => void latestCreateRef.current(),
      });
    } finally {
      setLoading(false);
    }
  };
  useLayoutEffect(() => {
    latestCreateRef.current = create;
  });

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void create();
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Nachzügler-Link erstellen">
      <form onSubmit={handleSubmit} className="space-y-4">
        <p className="text-sm leading-6 text-gray-600">
          {phase.name}: Der Link erlaubt genau dieser E-Mail-Adresse eine
          Anmeldung, auch wenn die Frist geschlossen ist.
        </p>
        <FormErrorAlert message={errors.error} />
        <Input
          name="guardian_email"
          error={errors.fieldError("guardian_email")}
          type="email"
          label="E-Mail der erziehungsberechtigten Person"
          value={guardianEmail}
          onChange={(event) => setGuardianEmail(event.target.value)}
          required
        />
        <Textarea
          id="late-invite-reason"
          name="reason"
          label="Interner Grund"
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          rows={3}
          placeholder="z. B. Frist verpasst, telefonisch geklärt"
          error={errors.fieldError("reason")}
        />

        {generatedUrl ? (
          <Alert
            type="success"
            message="Link wurde erstellt."
            actionLayout="stacked"
            action={
              <div className="flex items-center gap-2">
                <div className="min-w-0 flex-1">
                  <Input
                    aria-label="Erstellter Nachzügler-Link"
                    readOnly
                    value={generatedUrl}
                    controlSize="compact"
                  />
                </div>
                <PublicLinkCopyButton
                  url={generatedUrl}
                  componentId={`LateInvite:${phase.id}:${generatedUrl}`}
                  label="Nachzügler-Link kopieren"
                />
              </div>
            }
          />
        ) : null}

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            Schließen
          </Button>
          <Button
            type="submit"
            size="md"
            isLoading={loading}
            loadingText="Erstelle..."
          >
            Link erstellen
          </Button>
        </div>
      </form>
    </Modal>
  );
}

export function ManualApprovedEnrollmentModal({
  isOpen,
  onClose,
  phase,
  gradeLevelMax,
}: Readonly<{
  isOpen: boolean;
  onClose: () => void;
  phase: Phase;
  gradeLevelMax: number | null;
}>) {
  const [prefetchedData, setPrefetchedData] =
    useState<EnrollmentFormPrefetchedData | null>(null);
  // Occupancy per offering (#2186). Advisory only: a failed load simply
  // hides the capacity lines, it must never block a manual enrollment.
  const [bookingStats, setBookingStats] = useState<
    Record<string, CareOfferingBookingStats>
  >({});
  const [reason, setReason] = useState("");
  const [externalConsentConfirmed, setExternalConsentConfirmed] =
    useState(false);
  const [sendNotification, setSendNotification] = useState(false);
  const [statusUrl, setStatusUrl] = useState("");
  // Whether the saved enrollment renewed an enrolled child (#6148).
  const [renewedChildName, setRenewedChildName] = useState<string | null>(null);
  // The open question „Ist das Kind schon angemeldet?“ and the answer the
  // waiting submission resumes with.
  const [existingChildQuestion, setExistingChildQuestion] = useState<{
    firstName: string;
    lastName: string;
    candidates: AdminExistingChildCandidate[];
    answer: (choice: ExistingChildChoice | null) => void;
  } | null>(null);
  // True from sending until the answer is in, the question included.
  const [sending, setSending] = useState(false);
  const [loading, setLoading] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const loadError = useApiLoadError();
  const showLoadError = loadError.show;
  const clearLoadError = loadError.clear;
  const checks = useApiFormError();
  const clearChecks = checks.clear;
  const reload = useCallback(() => setAttempt((value) => value + 1), []);
  const configurationError =
    gradeLevelMax === null
      ? "Die Klassenstufen der Schule fehlen. Bitte laden Sie die Seite neu."
      : null;

  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;
    setPrefetchedData(null);
    setBookingStats({});
    setReason("");
    setExternalConsentConfirmed(false);
    setSendNotification(false);
    setStatusUrl("");
    setRenewedChildName(null);
    clearLoadError();
    clearChecks();
    setLoading(true);
    void fetchManualEnrollmentBootstrap(phase.id)
      .then((bootstrap) => {
        if (!cancelled) setPrefetchedData(toManualPrefetchedData(bootstrap));
      })
      .catch((err: unknown) => {
        logger.warn("manual_enrollment_bootstrap_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!cancelled) {
          void showLoadError(err, { object: "das Formular", retry: reload });
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    void fetchCareOfferingBookingStats(phase.id)
      .then((stats) => {
        if (!cancelled) setBookingStats(stats);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        // Die Belegung ist nur ein Hinweis: ohne sie meldet erst das Speichern
        // ein volles Angebot. Das Formular bleibt bedienbar.
        setBookingStats({});
        logger.warn("manual_enrollment_booking_stats_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [
    isOpen,
    phase.id,
    attempt,
    reload,
    clearLoadError,
    clearChecks,
    showLoadError,
  ]);

  const submitter = async (
    payload: SubmitEnrollmentPayload,
  ): Promise<SubmitEnrollmentResult | null> => {
    const trimmedReason = reason.trim();
    // Grund und Einwilligung stehen über dem Formular. Der Kasten hier sagt,
    // was fehlt; `null` hält das Formular davon ab, Erfolg oder einen
    // zweiten Fehler zu melden.
    if (!trimmedReason) {
      checks.invalid("Bitte geben Sie einen internen Grund an.", {
        reason: "Bitte geben Sie einen Grund an.",
      });
      return null;
    }
    if (!externalConsentConfirmed) {
      checks.invalid(
        "Bitte bestätigen Sie, dass die Einwilligung der Eltern vorliegt.",
      );
      return null;
    }
    checks.clear();
    setRenewedChildName(null);
    const child = payload.children[0];
    setSending(true);
    const result = await sendWithExistingChildChoice(
      (choice) =>
        createManualApprovedEnrollment(phase.id, {
          ...payload,
          external_consent_confirmed: true,
          reason: trimmedReason,
          send_notification: sendNotification,
          ...(choice && "existingStudentId" in choice
            ? { existing_student_id: choice.existingStudentId }
            : {}),
          ...(choice && "createNewStudent" in choice
            ? { create_new_student: true as const }
            : {}),
        }).then((created) => ({ created, choice })),
      // The form keeps its send button busy until the office answers.
      (candidates) =>
        new Promise<ExistingChildChoice | null>((resolve) => {
          setExistingChildQuestion({
            firstName: child?.first_name ?? "",
            lastName: child?.last_name ?? "",
            candidates,
            answer: resolve,
          });
        }),
    ).finally(() => setSending(false));
    if (!result) return null;
    const renewed =
      result.choice !== undefined && "existingStudentId" in result.choice;
    setRenewedChildName(
      renewed
        ? `${child?.first_name ?? ""} ${child?.last_name ?? ""}`.trim()
        : null,
    );
    setStatusUrl(result.created.status_url);
    return result.created;
  };

  const answerExistingChildQuestion = (choice: ExistingChildChoice | null) => {
    existingChildQuestion?.answer(choice);
    setExistingChildQuestion(null);
  };

  return (
    <SlideOver
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      // The question dialog sits outside the panel: neither a click on it
      // nor the focus it hands back may close the form whose submission
      // waits for the answer.
      isDismissDisabled={sending}
    >
      <SlideOverContent size="xl">
        <SlideOverHeader>
          <SlideOverTitle>Kind manuell über Anmeldung freigeben</SlideOverTitle>
        </SlideOverHeader>
        <div className="flex-1 space-y-4 overflow-y-auto px-5 py-4">
          <p className="text-sm leading-6 text-gray-600">
            {phase.name}: Diese Eingabe nutzt dieselbe Vorlage, dieselben
            Betreuungsangebote und dieselbe Freigabe-Logik wie die
            Online-Anmeldung. Nach dem Absenden wird das Kind direkt bestätigt.
          </p>
          <LoadErrorAlert error={loadError.error ?? configurationError} />
          <FormErrorAlert message={checks.error} />
          {statusUrl ? (
            <Alert
              type="success"
              message={
                renewedChildName
                  ? `Die Anmeldung von ${renewedChildName} wurde verlängert. Es wurde kein neues Kind angelegt.`
                  : "Die manuelle Anmeldung wurde angelegt und freigegeben."
              }
              actionLayout="stacked"
              action={
                <div className="flex items-center gap-2">
                  <div className="min-w-0 flex-1">
                    <Input
                      aria-label="Statuslink der manuellen Anmeldung"
                      readOnly
                      value={statusUrl}
                      controlSize="compact"
                    />
                  </div>
                  <PublicLinkCopyButton
                    url={statusUrl}
                    componentId={`ManualEnrollment:${phase.id}:${statusUrl}`}
                    label="Statuslink kopieren"
                  />
                </div>
              }
            />
          ) : null}
          <div className="moto-content-surface grid gap-3 rounded-xl border p-4 sm:grid-cols-[minmax(0,1fr)_16rem]">
            <Textarea
              id="manual-enrollment-reason"
              name="reason"
              label="Interner Grund"
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              rows={3}
              placeholder="z. B. verspätete Rückmeldung telefonisch bestätigt"
              error={checks.fieldError("reason")}
            />
            <div className="space-y-3 text-sm text-gray-700">
              <label
                htmlFor="manual-enrollment-external-consent"
                className="flex items-start gap-2"
              >
                <Checkbox
                  id="manual-enrollment-external-consent"
                  checked={externalConsentConfirmed}
                  onChange={(event) =>
                    setExternalConsentConfirmed(event.target.checked)
                  }
                  className="mt-0.5"
                />
                <span>Einwilligung der Eltern liegt extern vor.</span>
              </label>
              <label
                htmlFor="manual-enrollment-send-notification"
                className="flex items-start gap-2"
              >
                <Checkbox
                  id="manual-enrollment-send-notification"
                  checked={sendNotification}
                  onChange={(event) =>
                    setSendNotification(event.target.checked)
                  }
                  className="mt-0.5"
                />
                <span>Eltern per E-Mail benachrichtigen.</span>
              </label>
            </div>
          </div>
          {loading ? (
            <Loading message="Formularvorlage wird geladen..." />
          ) : prefetchedData && gradeLevelMax !== null ? (
            <EnrollmentForm
              phaseID={phase.id}
              gradeLevelMax={gradeLevelMax}
              onSubmitted={(url) => setStatusUrl(url)}
              prefetchedData={prefetchedData}
              submitter={submitter}
              skipCaptcha
              lockChildStructure
              withoutSignature
              showBlockedOfferings
              offeringBookingStats={bookingStats}
              submitLabel="Kind anlegen und freigeben"
            />
          ) : null}
        </div>
      </SlideOverContent>
      {existingChildQuestion ? (
        <ChoiceModal
          isOpen
          title="Ist das Kind schon angemeldet?"
          description={existingChildChoiceDescription({
            first_name: existingChildQuestion.firstName,
            last_name: existingChildQuestion.lastName,
            possible_existing_children: existingChildQuestion.candidates,
          })}
          options={existingChildChoiceOptions({
            first_name: existingChildQuestion.firstName,
            last_name: existingChildQuestion.lastName,
            possible_existing_children: existingChildQuestion.candidates,
          })}
          onClose={() => answerExistingChildQuestion(null)}
          onSelect={(value) =>
            answerExistingChildQuestion(
              value === "new"
                ? { createNewStudent: true }
                : { existingStudentId: value },
            )
          }
        />
      ) : null}
    </SlideOver>
  );
}

function toManualPrefetchedData(
  bootstrap: Awaited<ReturnType<typeof fetchManualEnrollmentBootstrap>>,
): EnrollmentFormPrefetchedData {
  // The school confirms consent outside the form, so no block is required.
  // Staff entry bypasses the phase's audience and grade gates.
  return prefetchedDataFromBootstrap(
    {
      ...bootstrap,
      legal_texts: {
        ...bootstrap.legal_texts,
        blocks: (bootstrap.legal_texts.blocks ?? []).map((block) => ({
          ...block,
          required: false,
        })),
      },
    },
    { phaseRestrictions: false },
  );
}
