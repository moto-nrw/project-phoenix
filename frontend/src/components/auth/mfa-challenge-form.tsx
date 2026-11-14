"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { credentialError, MFA_STEP_REFUSALS } from "./credential-error";
import {
  resendChallenge,
  verifyMFA,
  type LoginScope,
  type MFAResult,
} from "~/lib/mfa-api";
import type { OTPInputGridHandle } from "./otp-input-grid";
import { Button } from "~/components/ui/button";
import {
  MFA_CODE_LENGTH,
  MFACodeConfirm,
  type CodeConfirmChoice,
} from "./mfa-code-confirm";

const logger = createLogger({ component: "MFAChallengeForm" });

const DEFAULT_RESEND_COOLDOWN_SECONDS = 60;

interface MFAChallengeFormProps {
  readonly scope: LoginScope;
  readonly challengeToken: string;
  readonly maskedEmail: string;
  readonly resendCooldownSeconds?: number;
  readonly onSuccess: (result: MFAResult) => void | Promise<void>;
  /**
   * Gives the embedding login flow a chance to handle a terminal verify
   * error, such as a portal handoff. Return true when the error is handled.
   */
  readonly onError?: (error: unknown) => boolean | Promise<boolean>;
  readonly onCancel?: () => void;
  /**
   * Whether the code step may remember the device (#6955): the backend's
   * trusted_device_enabled for this portal and school. When false only
   * „Bestätigen“ is offered and nothing is remembered.
   */
  readonly trustedDeviceEnabled?: boolean;
  /**
   * Day count of the „… merken“ button, the backend's trusted_device_days,
   * so the label matches the actual lifetime. Defaults to 90, the registry
   * default.
   */
  readonly trustedDeviceDays?: number;
  /** The Träger the Träger-Portal login resolved (#4651). */
  readonly organizationId?: string;
}

export function MFAChallengeForm({
  scope,
  challengeToken,
  maskedEmail,
  resendCooldownSeconds = DEFAULT_RESEND_COOLDOWN_SECONDS,
  onSuccess,
  onError,
  onCancel,
  trustedDeviceEnabled = false,
  trustedDeviceDays = 90,
  organizationId,
}: MFAChallengeFormProps) {
  // Fehler über den gemeinsamen Weg (#2517). Ein abgelehnter Code bleibt im
  // Formular (MFA_STEP_REFUSALS); jeder andere 401 ist eine abgelaufene
  // Anmeldung und führt zur Anmeldeseite.
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, clear: clearError } = formErrors;
  const latestResendRef = useRef<() => void>(() => undefined);
  // Which button is being checked; null while nothing is sent.
  const [pending, setPending] = useState<CodeConfirmChoice | null>(null);
  const isVerifying = pending !== null;
  const [code, setCode] = useState("");
  const [isResending, setIsResending] = useState(false);
  const [resendIn, setResendIn] = useState(resendCooldownSeconds);
  // The backend rotates the challenge JWT on every resend. The prop is
  // the initial token from /auth/login; we shadow it locally so the
  // verify call after a resend travels with the renewed JWT. (#1430
  // review round 2 — the previous shape produced a dead-end where the
  // freshly emailed code couldn't be verified once the original JWT
  // expired.)
  const [activeToken, setActiveToken] = useState(challengeToken);
  const otpRef = useRef<OTPInputGridHandle | null>(null);

  useEffect(() => {
    if (resendIn <= 0) return;
    const timer = setTimeout(() => setResendIn((s) => s - 1), 1000);
    return () => clearTimeout(timer);
  }, [resendIn]);

  const performVerify = useCallback(
    async (submittedCode: string, choice: CodeConfirmChoice) => {
      setPending(choice);
      clearError();
      try {
        const tokens = await verifyMFA(scope, {
          challengeToken: activeToken,
          code: submittedCode,
          // Only the explicit second button remembers the device (#6955).
          rememberDevice: trustedDeviceEnabled && choice === "remember",
          organizationId,
        });
        await onSuccess(tokens);
      } catch (err) {
        if (await onError?.(err)) {
          return;
        }
        logger.warn("mfa_verify_failed", {
          scope,
          error: err instanceof Error ? err.message : String(err),
        });
        otpRef.current?.reset();
        setCode("");
        // Kein Wiederholen: der Code steht nicht mehr im Feld, man tippt
        // ihn neu ein.
        void showError(credentialError(err, MFA_STEP_REFUSALS), {
          object: "die Prüfung des Codes",
        });
      } finally {
        setPending(null);
      }
    },
    [
      scope,
      activeToken,
      trustedDeviceEnabled,
      organizationId,
      onSuccess,
      onError,
      clearError,
      showError,
    ],
  );

  const handleResend = async () => {
    if (resendIn > 0 || isResending) return;
    setIsResending(true);
    clearError();
    try {
      const renewed = await resendChallenge(scope, {
        challengeToken: activeToken,
      });
      // Swap the in-flight token so the next verify travels with the
      // JWT that's bound to the freshly emailed code's lifetime.
      setActiveToken(renewed);
      setResendIn(resendCooldownSeconds);
      otpRef.current?.reset();
      setCode("");
    } catch (err) {
      logger.warn("mfa_resend_failed", {
        scope,
        error: err instanceof Error ? err.message : String(err),
      });
      void showError(credentialError(err, MFA_STEP_REFUSALS), {
        object: "das Senden eines neuen Codes",
        retry: () => latestResendRef.current(),
      });
    } finally {
      setIsResending(false);
    }
  };

  useLayoutEffect(() => {
    latestResendRef.current = () => void handleResend();
  });

  let resendLabel: string;
  if (resendIn > 0) {
    resendLabel = `Neuen Code anfordern (${resendIn}s)`;
  } else if (isResending) {
    resendLabel = "Code wird gesendet…";
  } else {
    resendLabel = "Neuen Code anfordern";
  }

  const handleBack = () => {
    onCancel?.();
  };

  const showBack = Boolean(onCancel);

  return (
    <div ref={formRef} className="space-y-6 text-left">
      {showBack && (
        <div>
          <button
            type="button"
            onClick={handleBack}
            disabled={isVerifying}
            className="-ml-1 flex items-center gap-1.5 text-sm text-gray-500 transition-colors hover:text-gray-800 focus:outline-none disabled:cursor-not-allowed disabled:text-gray-300"
            aria-label="Zurück zum Login"
          >
            <svg
              className="h-4 w-4"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              aria-hidden="true"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M15 19l-7-7 7-7"
              />
            </svg>
            Zurück
          </button>
        </div>
      )}

      <div className="space-y-1.5">
        <h2 className="text-xl font-semibold text-gray-900">Code-Eingabe</h2>
        <p className="text-sm leading-relaxed text-gray-600">
          Wir haben einen 6-stelligen Code an{" "}
          <span className="font-medium text-gray-800">{maskedEmail}</span>{" "}
          gesendet. Der Code ist 10 Minuten gültig.
        </p>
      </div>

      <FormErrorAlert message={formErrors.error} />

      <MFACodeConfirm
        scope={scope}
        otpRef={otpRef}
        code={code}
        onCodeChange={(next) => {
          setCode(next);
          if (formErrors.error) clearError();
        }}
        pending={pending}
        trustedDeviceEnabled={trustedDeviceEnabled}
        trustedDeviceDays={trustedDeviceDays}
        onConfirm={(submitted, choice) => {
          // Fire-and-forget: errors surface through state, not the caller.
          void performVerify(submitted, choice);
        }}
        onIncomplete={() =>
          formErrors.invalid(
            `Bitte geben Sie alle ${MFA_CODE_LENGTH} Stellen des Codes ein.`,
          )
        }
      >
        <div className="flex justify-center">
          <Button
            type="button"
            variant="link"
            size="md"
            onClick={() => {
              void handleResend();
            }}
            disabled={resendIn > 0 || isResending || isVerifying}
            tooltip="Schickt einen neuen Code an Ihre E-Mail-Adresse."
          >
            {resendLabel}
          </Button>
        </div>
      </MFACodeConfirm>
    </div>
  );
}
