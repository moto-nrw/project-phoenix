"use client";

import {
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { DevicesIcon, SignInIcon } from "@phosphor-icons/react";
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
import { IconTile } from "~/components/ui/icon-tile";
import { SpinnerIcon } from "~/components/ui/icons";
import { ListRowLink } from "~/components/ui/list-row-link";
import { SectionLabel } from "~/components/ui/section-label";

const logger = createLogger({ component: "MFAChallengeForm" });

const CODE_LENGTH = 6;
const DEFAULT_RESEND_COOLDOWN_SECONDS = 60;

/**
 * Where each portal lists the remembered devices, as the sentence under the
 * two answers names it. The words are the portal's own navigation labels.
 */
const DEVICE_LIST_PLACE: Record<LoginScope, string> = {
  tenant: "in Ihrem Profil",
  school: "in den Einstellungen",
  traeger: "unter „Konto“",
  caterer: "unter „Konto“",
  operator: "in Ihrem Profil",
  parent: "unter „Einstellungen → Anmeldung“",
};

/**
 * Every text of the code step. The German default fits every portal; the
 * parents portal passes its own in the parent's language (#7051).
 */
export interface MFAChallengeCopy {
  readonly back: string;
  readonly backLabel: string;
  readonly title: string;
  readonly sentTo: (maskedEmail: ReactNode) => ReactNode;
  readonly codeLabel: string;
  readonly digitLabel: (position: number) => string;
  readonly checking: string;
  readonly autoCheck: string;
  readonly question: string;
  readonly remember: string;
  readonly rememberHint: (days: number) => string;
  readonly once: string;
  readonly onceHint: string;
  readonly enterCodeFirst: string;
  readonly removeDevicesHint: string;
  readonly resend: string;
  readonly resendIn: (seconds: number) => string;
  readonly resending: string;
  readonly verifyObject: string;
  readonly resendObject: string;
}

/** The German copy of a portal's code step. */
export function germanMFAChallengeCopy(scope: LoginScope): MFAChallengeCopy {
  return {
    back: "Zurück",
    backLabel: "Zurück zum Login",
    title: "Code-Eingabe",
    sentTo: (maskedEmail) => (
      <>
        Wir haben einen 6-stelligen Code an{" "}
        <span className="font-medium text-gray-800">{maskedEmail}</span>{" "}
        gesendet. Der Code ist 10 Minuten gültig.
      </>
    ),
    codeLabel: "6-stelliger Bestätigungscode",
    digitLabel: (position) => `Stelle ${position}`,
    checking: "Code wird geprüft…",
    autoCheck:
      "Der Code wird automatisch geprüft, sobald Sie alle 6 Stellen eingegeben haben.",
    question: "Soll moto dieses Gerät merken?",
    remember: "Ja, merken und anmelden",
    rememberHint: (days) =>
      `${days} Tage kein Code auf diesem Gerät. Nur für Ihr eigenes Gerät.`,
    once: "Nein, nur anmelden",
    onceHint: "Für Geräte, die mehrere Personen nutzen.",
    enterCodeFirst: "Geben Sie zuerst den Code ein.",
    removeDevicesHint: `Gemerkte Geräte entfernen Sie ${DEVICE_LIST_PLACE[scope]}.`,
    resend: "Neuen Code anfordern",
    resendIn: (seconds) => `Neuen Code anfordern (${seconds}s)`,
    resending: "Code wird gesendet…",
    verifyObject: "die Prüfung des Codes",
    resendObject: "das Senden eines neuen Codes",
  };
}

/** Which of the two answers is waiting for the check. */
type PendingChoice = "remember" | "once";

interface ConfirmChoiceProps {
  readonly icon: ReactNode;
  readonly tone: "green" | "neutral";
  readonly title: string;
  /** For whom the answer is meant. */
  readonly subtitle: string;
  readonly disabled: boolean;
  /** This answer started the running check. */
  readonly pending: boolean;
  readonly onClick: () => void;
}

/**
 * One of the two answers to "Soll moto dieses Gerät merken?". A row of the
 * kit that leads on into the app: symbol, answer, for whom it is meant. While
 * its check runs, a spinner stands where the arrow was.
 */
function ConfirmChoice({
  icon,
  tone,
  title,
  subtitle,
  disabled,
  pending,
  onClick,
}: ConfirmChoiceProps) {
  return (
    <li>
      <ListRowLink
        framed
        icon={<IconTile size="sm" tone={tone} icon={icon} />}
        title={title}
        subtitle={subtitle}
        onClick={onClick}
        disabled={disabled}
        chevron={!pending}
        badge={pending ? <SpinnerIcon className="h-4 w-4" /> : undefined}
      />
    </li>
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
   * the person confirms with one of two answers and so decides each time
   * whether this device is remembered (#6955). When false, nothing can be
   * remembered and the code is checked as soon as it is complete. Undefined
   * defaults to `true` for backwards compatibility with older backends that
   * haven't shipped the field.
   */
  readonly trustedDeviceEnabled?: boolean;
  /**
   * Day count rendered into "N Tage kein Code auf diesem Gerät".
   * Mirrors security.mfa_trusted_device_days so the label always matches
   * the actual cookie lifetime. Defaults to 90 — the registry default —
   * if an older backend doesn't surface the field.
   */
  readonly trustedDeviceDays?: number;
  /** The Träger the Träger-Portal login resolved (#4651). */
  readonly organizationId?: string;
  /** The texts; the German default fits every portal. */
  readonly copy?: MFAChallengeCopy;
  /**
   * Redeems the code somewhere else than the portal's sign-in, such as the
   * code step in a running Träger session (#7068). Defaults to the sign-in's
   * verify of the scope.
   */
  readonly verify?: (params: {
    challengeToken: string;
    code: string;
    rememberDevice: boolean;
  }) => Promise<MFAResult>;
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
  copy: givenCopy,
  verify,
}: MFAChallengeFormProps) {
  const copy = givenCopy ?? germanMFAChallengeCopy(scope);
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
        const tokens = verify
          ? await verify({
              challengeToken: activeToken,
              code: submittedCode,
              rememberDevice,
            })
          : await verifyMFA(scope, {
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
          object: copy.verifyObject,
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
      verify,
      onSuccess,
      onError,
      clearError,
      showError,
      copy.verifyObject,
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
        object: copy.resendObject,
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
    resendLabel = copy.resendIn(resendIn);
  } else if (isResending) {
    resendLabel = copy.resending;
  } else {
    resendLabel = copy.resend;
  }

  const codeComplete = code.length === CODE_LENGTH;
  const questionId = useId();
  const hintId = useId();
  let hintText: string;
  if (isVerifying) hintText = copy.checking;
  else if (codeComplete) hintText = copy.removeDevicesHint;
  else hintText = copy.enterCodeFirst;

  // With the two answers below, a complete code waits for the person's
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
            aria-label={copy.backLabel}
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
            {copy.back}
          </button>
        </div>
      )}

      <div className="space-y-1.5">
        <h2 className="text-xl font-semibold text-gray-900">{copy.title}</h2>
        <p className="text-sm leading-relaxed text-gray-600">
          {copy.sentTo(maskedEmail)}
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
          ariaLabel={copy.codeLabel}
          digitLabel={copy.digitLabel}
        />
        {!trustedDeviceEnabled && (
          <p className="text-center text-xs text-gray-500">
            {isVerifying ? copy.checking : copy.autoCheck}
          </p>
        )}
      </div>

      {trustedDeviceEnabled && (
        <div className="space-y-2">
          <SectionLabel id={questionId}>{copy.question}</SectionLabel>
          {/* Two answers of equal weight: nothing is preselected and
              neither looks like the default. */}
          <ul
            aria-labelledby={questionId}
            aria-describedby={hintId}
            className="flex flex-col gap-2"
          >
            <ConfirmChoice
              icon={<DevicesIcon aria-hidden="true" />}
              tone="green"
              title={copy.remember}
              subtitle={copy.rememberHint(trustedDeviceDays)}
              disabled={!codeComplete || isVerifying}
              pending={pendingChoice === "remember"}
              onClick={() => confirm("remember")}
            />
            <ConfirmChoice
              icon={<SignInIcon aria-hidden="true" />}
              tone="neutral"
              title={copy.once}
              subtitle={copy.onceHint}
              disabled={!codeComplete || isVerifying}
              pending={pendingChoice === "once"}
              onClick={() => confirm("once")}
            />
          </ul>
          {/* One line, so nothing jumps: first what is missing, then where a
              remembered device can be removed again. */}
          <p
            id={hintId}
            className="text-xs leading-relaxed text-gray-500"
            aria-live="polite"
          >
            {hintText}
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
