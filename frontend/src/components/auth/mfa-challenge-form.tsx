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
import { OTPInputGrid, type OTPInputGridHandle } from "./otp-input-grid";
import { Button } from "~/components/ui/button";

const logger = createLogger({ component: "MFAChallengeForm" });

const CODE_LENGTH = 6;
const DEFAULT_RESEND_COOLDOWN_SECONDS = 60;

/**
 * Where each portal lists the remembered devices, as the sentence under the
 * two buttons names it. The words are the portal's own navigation labels.
 */
const DEVICE_LIST_PLACE: Record<LoginScope, string> = {
  tenant: "in Ihrem Profil",
  school: "in den Einstellungen",
  traeger: "unter „Konto“",
  caterer: "unter „Konto“",
  operator: "in Ihrem Profil",
};

/** Which of the two confirm buttons is waiting for the answer. */
type PendingChoice = "remember" | "once";

interface ConfirmChoiceProps {
  readonly variant: "primary" | "outline";
  readonly label: string;
  /** For whom the choice is meant, visible also where no tooltip shows. */
  readonly description: string;
  readonly tooltip: string;
  readonly codeComplete: boolean;
  /** A check is running, started by this or the other button. */
  readonly busy: boolean;
  /** This button started the running check. */
  readonly pending: boolean;
  readonly onClick: () => void;
}

/**
 * One of the two ways to confirm the code. While its check runs the label
 * says so and the description stays, so the button keeps its height.
 */
function ConfirmChoice({
  variant,
  label,
  description,
  tooltip,
  codeComplete,
  busy,
  pending,
  onClick,
}: ConfirmChoiceProps) {
  const primary = variant === "primary";
  return (
    <Button
      type="button"
      variant={variant}
      size="md"
      className="w-full"
      onClick={onClick}
      disabled={!codeComplete || busy}
      disabledReason={
        codeComplete ? undefined : "Geben Sie zuerst den Code ein."
      }
      tooltip={tooltip}
    >
      <span className="flex w-full min-w-0 flex-col items-start gap-0.5 text-left wrap-anywhere">
        <span
          className={`text-sm font-semibold ${primary ? "text-white" : "text-gray-900"}`}
          aria-live="polite"
        >
          {pending ? "Code wird geprüft…" : label}
        </span>
        <span
          className={`text-xs font-normal text-pretty ${primary ? "text-white/80" : "text-gray-500"}`}
        >
          {description}
        </span>
      </span>
    </Button>
  );
}

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
   * Mirrors security.mfa_trusted_device_enabled for the tenant. When true,
   * the person confirms with one of two buttons and so decides each time
   * whether this device is remembered (#6955). When false, nothing can be
   * remembered and the code is checked as soon as it is complete. Undefined
   * defaults to `true` for backwards compatibility with older backends that
   * haven't shipped the field.
   */
  readonly trustedDeviceEnabled?: boolean;
  /**
   * Day count rendered into "Bestätigen und dieses Gerät N Tage merken".
   * Mirrors security.mfa_trusted_device_days so the label always matches
   * the actual cookie lifetime. Defaults to 90 — the registry default —
   * if an older backend doesn't surface the field.
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
  trustedDeviceEnabled = true,
  trustedDeviceDays = 90,
  organizationId,
}: MFAChallengeFormProps) {
  // Nothing is preselected: the device is remembered only when the person
  // presses the button that says so (#6955). A shared tablet in a room must
  // never become a remembered device by a default.
  const [code, setCode] = useState("");
  const [pendingChoice, setPendingChoice] = useState<PendingChoice | null>(
    null,
  );
  // Fehler über den gemeinsamen Weg (#2517). Ein abgelehnter Code bleibt im
  // Formular (MFA_STEP_REFUSALS); jeder andere 401 ist eine abgelaufene
  // Anmeldung und führt zur Anmeldeseite.
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, clear: clearError } = formErrors;
  const latestResendRef = useRef<() => void>(() => undefined);
  const [isVerifying, setIsVerifying] = useState(false);
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
    async (submittedCode: string, rememberDevice: boolean) => {
      setIsVerifying(true);
      clearError();
      try {
        const tokens = await verifyMFA(scope, {
          challengeToken: activeToken,
          code: submittedCode,
          rememberDevice,
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
        // Kein Wiederholen: der Code steht nicht mehr im Feld, man tippt
        // ihn neu ein.
        void showError(credentialError(err, MFA_STEP_REFUSALS), {
          object: "die Prüfung des Codes",
        });
      } finally {
        setIsVerifying(false);
        setPendingChoice(null);
      }
    },
    [
      scope,
      activeToken,
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

  const codeComplete = code.length === CODE_LENGTH;

  // With the two buttons below, a complete code waits for the person's
  // choice. Leaving the field closes the on-screen keyboard of a tablet or
  // phone, which would otherwise cover the buttons.
  useEffect(() => {
    if (trustedDeviceEnabled && codeComplete) otpRef.current?.blur();
  }, [trustedDeviceEnabled, codeComplete]);

  const confirm = (choice: PendingChoice) => {
    if (!codeComplete || isVerifying) return;
    setPendingChoice(choice);
    void performVerify(code, choice === "remember");
  };

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

      <div className="space-y-3">
        <OTPInputGrid
          ref={otpRef}
          length={CODE_LENGTH}
          disabled={isVerifying}
          onChange={setCode}
          onComplete={(completeCode) => {
            // Without the choice there is nothing to decide: the code is
            // checked right away. Fire-and-forget: errors surface through
            // state, not the caller.
            if (!trustedDeviceEnabled) void performVerify(completeCode, false);
          }}
          ariaLabel="6-stelliger Bestätigungscode"
        />
        {!trustedDeviceEnabled && (
          <p className="text-center text-xs text-gray-500">
            {isVerifying
              ? "Code wird geprüft…"
              : "Der Code wird automatisch geprüft, sobald Sie alle 6 Stellen eingegeben haben."}
          </p>
        )}
      </div>

      {trustedDeviceEnabled && (
        <div className="space-y-3">
          {/* The kit's choice shape (ChoiceModal): full-width buttons that
              each name their consequence. The texts wrap on a phone, so the
              buttons keep the kit's `md` padding instead of a fixed height. */}
          <div
            role="group"
            aria-label="Anmeldung bestätigen"
            className="flex flex-col gap-2"
          >
            <ConfirmChoice
              variant="primary"
              label={`Bestätigen und dieses Gerät ${trustedDeviceDays} Tage merken`}
              description="Nur auf Ihrem eigenen Gerät. Dort entfällt der Code so lange."
              tooltip={`Meldet Sie an. Hier fragt moto ${trustedDeviceDays} Tage lang keinen Code ab.`}
              codeComplete={codeComplete}
              busy={isVerifying}
              pending={pendingChoice === "remember"}
              onClick={() => confirm("remember")}
            />
            <ConfirmChoice
              variant="outline"
              label="Nur dieses Mal bestätigen"
              description="Für Geräte, die mehrere Personen nutzen."
              tooltip="Meldet Sie an. Beim nächsten Anmelden fragt moto wieder nach einem Code."
              codeComplete={codeComplete}
              busy={isVerifying}
              pending={pendingChoice === "once"}
              onClick={() => confirm("once")}
            />
          </div>
          <p className="text-center text-xs leading-relaxed text-gray-500">
            Gemerkte Geräte entfernen Sie {DEVICE_LIST_PLACE[scope]}.
          </p>
        </div>
      )}

      <div className="flex justify-center">
        <Button
          type="button"
          variant="link"
          size="md"
          onClick={() => {
            void handleResend();
          }}
          disabled={resendIn > 0 || isResending || isVerifying}
        >
          {resendLabel}
        </Button>
      </div>
    </div>
  );
}
