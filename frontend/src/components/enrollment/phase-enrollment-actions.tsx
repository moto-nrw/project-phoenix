"use client";

import { useEffect, useState, type FormEvent } from "react";
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
import { Input } from "~/components/ui/input";
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
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!isOpen) return;
    setGuardianEmail("");
    setReason("");
    setGeneratedUrl("");
    setError(null);
    setLoading(false);
  }, [isOpen]);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);
    if (!phaseUrl) {
      setError("Der öffentliche Phasenlink konnte nicht ermittelt werden.");
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
      setError(
        err instanceof Error
          ? err.message
          : "Nachzügler-Link konnte nicht erstellt werden",
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Nachzügler-Link erstellen">
      <form onSubmit={handleSubmit} className="space-y-4">
        <p className="text-sm leading-6 text-gray-600">
          {phase.name}: Der Link erlaubt genau dieser E-Mail-Adresse eine
          Anmeldung, auch wenn die Frist geschlossen ist.
        </p>
        {error ? (
          <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-lg border px-3 py-2 text-sm">
            {error}
          </div>
        ) : null}
        <Input
          name="late_invite_guardian_email"
          type="email"
          label="E-Mail der erziehungsberechtigten Person"
          value={guardianEmail}
          onChange={(event) => setGuardianEmail(event.target.value)}
          required
        />
        <label className="block">
          <span className="mb-2 block text-sm font-medium text-gray-700">
            Interner Grund
          </span>
          <textarea
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            rows={3}
            className="block w-full rounded-lg border-0 bg-white px-4 py-3 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 ring-inset placeholder:text-gray-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400"
            placeholder="z. B. Frist verpasst, telefonisch geklärt"
          />
        </label>

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
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const configurationError =
    gradeLevelMax === null
      ? "Die Klassenstufen-Konfiguration ist nicht verfügbar."
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
    setLoadError(null);
    setLoading(true);
    void fetchManualEnrollmentBootstrap(phase.id)
      .then((bootstrap) => {
        if (!cancelled) setPrefetchedData(toManualPrefetchedData(bootstrap));
      })
      .catch((err) => {
        if (!cancelled) {
          setLoadError(
            err instanceof Error
              ? err.message
              : "Formularvorlage konnte nicht geladen werden",
          );
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
        setBookingStats({});
        logger.warn("manual_enrollment_booking_stats_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, phase.id]);

  const submitter = async (
    payload: SubmitEnrollmentPayload,
  ): Promise<SubmitEnrollmentResult> => {
    const trimmedReason = reason.trim();
    if (!trimmedReason) {
      throw new Error("Bitte einen internen Grund angeben.");
    }
    if (!externalConsentConfirmed) {
      throw new Error(
        "Bitte bestätigen, dass die Einwilligung extern vorliegt.",
      );
    }
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
          {loadError || configurationError ? (
            <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-lg border px-3 py-2 text-sm">
              {loadError ?? configurationError}
            </div>
          ) : null}
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
            <label className="block">
              <span className="mb-2 block text-sm font-medium text-gray-700">
                Interner Grund
              </span>
              <textarea
                value={reason}
                onChange={(event) => setReason(event.target.value)}
                rows={3}
                className="block w-full rounded-lg border-0 bg-white px-4 py-3 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 ring-inset placeholder:text-gray-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400"
                placeholder="z. B. verspätete Rückmeldung telefonisch bestätigt"
              />
            </label>
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
