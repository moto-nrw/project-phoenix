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
} from "~/lib/enrollment-admin-api";
import type {
  SubmitEnrollmentPayload,
  SubmitEnrollmentResult,
} from "~/lib/enrollment-submission-api";
import type { Phase } from "~/lib/enrollment-phase-api";
import { Button } from "~/components/ui/button";
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
  SlideOverCloseButton,
} from "~/components/ui/slide-over";
import {
  EnrollmentForm,
  type EnrollmentFormPrefetchedData,
} from "~/components/enrollment/enrollment-form";
import { PublicLinkCopyButton } from "~/components/enrollment/public-link-copy-button";
import {
  type CareOfferingBookingStats,
  fetchCareOfferingBookingStats,
} from "~/lib/care-offering-booking-stats";
import { createLogger } from "~/lib/logger";

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
      setGeneratedUrl(buildLateInviteUrl(phaseUrl, result.token));
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
          <div className="border-moto-green/30 bg-moto-green/10 rounded-xl border p-3">
            <p className="text-moto-green-strong text-sm font-medium">
              Link wurde erstellt.
            </p>
            <div className="mt-3 flex items-center gap-2">
              <input
                aria-label="Erstellter Nachzügler-Link"
                readOnly
                value={generatedUrl}
                className="ring-moto-green/30 min-w-0 flex-1 rounded-lg border-0 bg-white px-3 py-2 text-xs text-gray-700 shadow-sm ring-1 ring-inset"
              />
              <PublicLinkCopyButton
                url={generatedUrl}
                componentId={`LateInvite:${phase.id}:${generatedUrl}`}
                label="Nachzügler-Link kopieren"
              />
            </div>
          </div>
        ) : null}

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" size="md" onClick={onClose}>
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
    const result = await createManualApprovedEnrollment(phase.id, {
      ...payload,
      external_consent_confirmed: true,
      reason: trimmedReason,
      send_notification: sendNotification,
    });
    setStatusUrl(result.status_url);
    return result;
  };

  return (
    <SlideOver
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SlideOverContent widthClass="sm:w-[900px]">
        <SlideOverHeader className="flex-row items-start justify-between gap-3">
          <div className="min-w-0">
            <SlideOverTitle>
              Kind manuell über Anmeldung freigeben
            </SlideOverTitle>
          </div>
          <SlideOverCloseButton />
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
            <div className="border-moto-green/30 bg-moto-green/10 rounded-xl border p-3">
              <p className="text-moto-green-strong text-sm font-medium">
                Die manuelle Anmeldung wurde angelegt und freigegeben.
              </p>
              <div className="mt-3 flex items-center gap-2">
                <input
                  aria-label="Statuslink der manuellen Anmeldung"
                  readOnly
                  value={statusUrl}
                  className="ring-moto-green/30 min-w-0 flex-1 rounded-lg border-0 bg-white px-3 py-2 text-xs text-gray-700 shadow-sm ring-1 ring-inset"
                />
                <PublicLinkCopyButton
                  url={statusUrl}
                  componentId={`ManualEnrollment:${phase.id}:${statusUrl}`}
                  label="Statuslink kopieren"
                />
              </div>
            </div>
          ) : null}
          <div className="moto-content-surface grid gap-3 rounded-xl border p-4 shadow-sm sm:grid-cols-[minmax(0,1fr)_16rem]">
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
              <label className="flex items-start gap-2">
                <input
                  type="checkbox"
                  checked={externalConsentConfirmed}
                  onChange={(event) =>
                    setExternalConsentConfirmed(event.target.checked)
                  }
                  className="mt-1 h-4 w-4 rounded border-gray-300 text-gray-900 focus:ring-gray-400"
                />
                <span>Einwilligung der Eltern liegt extern vor.</span>
              </label>
              <label className="flex items-start gap-2">
                <input
                  type="checkbox"
                  checked={sendNotification}
                  onChange={(event) =>
                    setSendNotification(event.target.checked)
                  }
                  className="mt-1 h-4 w-4 rounded border-gray-300 text-gray-900 focus:ring-gray-400"
                />
                <span>Eltern per E-Mail benachrichtigen.</span>
              </label>
            </div>
          </div>
          {loading ? (
            <p className="text-sm text-gray-500">
              Formularvorlage wird geladen...
            </p>
          ) : prefetchedData && gradeLevelMax !== null ? (
            <EnrollmentForm
              phaseID={phase.id}
              gradeLevelMax={gradeLevelMax}
              onSubmitted={(url) => setStatusUrl(url)}
              prefetchedData={prefetchedData}
              submitter={submitter}
              skipCaptcha
              lockChildStructure
              showBlockedOfferings
              offeringBookingStats={bookingStats}
              submitLabel="Kind anlegen und freigeben"
            />
          ) : null}
        </div>
      </SlideOverContent>
    </SlideOver>
  );
}

function toManualPrefetchedData(
  bootstrap: Awaited<ReturnType<typeof fetchManualEnrollmentBootstrap>>,
): EnrollmentFormPrefetchedData {
  return {
    schema: bootstrap.schema,
    offerings: bootstrap.offerings,
    careOfferingSelectionMode: bootstrap.care_offering_selection_mode,
    collectGradeLevel: bootstrap.collect_grade_level,
    careOfferingsEnabled: bootstrap.care_offerings_enabled,
    captchaConfig: null,
    legalTexts: {
      ...bootstrap.legal_texts,
      blocks: (bootstrap.legal_texts.blocks ?? []).map((block) => ({
        ...block,
        required: false,
      })),
    },
    profile: null,
    schoolClass: bootstrap.school_class,
  };
}

function buildLateInviteUrl(phaseUrl: string, token: string): string {
  try {
    const url = new URL(phaseUrl);
    url.searchParams.set("late_invite", token);
    return url.toString();
  } catch {
    const separator = phaseUrl.includes("?") ? "&" : "?";
    return `${phaseUrl}${separator}late_invite=${encodeURIComponent(token)}`;
  }
}
