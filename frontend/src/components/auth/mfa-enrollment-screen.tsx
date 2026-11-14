"use client";

import { Check } from "lucide-react";
import { EmptyState } from "~/components/ui/empty-state";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { WizardStepper } from "~/components/ui/wizard-stepper";
import { createLogger } from "~/lib/logger";
import { credentialError, MFA_STEP_REFUSALS } from "./credential-error";
import {
  enrollConfirm,
  enrollStart,
  type LoginScope,
  type MFAResult,
} from "~/lib/mfa-api";
import type { OTPInputGridHandle } from "./otp-input-grid";
import {
  MFA_CODE_LENGTH,
  MFACodeConfirm,
  type CodeConfirmChoice,
} from "./mfa-code-confirm";

const logger = createLogger({ component: "MFAEnrollmentScreen" });

const STEPS = ["Start", "Bestätigung", "Abschluss"] as const;

type Step = "intro" | "code" | "success";

const STEP_INDEX: Record<Step, number> = {
  intro: 0,
  code: 1,
  success: 2,
};

interface MFAEnrollmentScreenProps {
  readonly scope: LoginScope;
  /**
   * Enrollment-scoped JWT issued at login for accounts on an mfa-required
   * tenant with no credential yet. Only authorizes /auth/mfa/enroll/*.
   * After successful confirmation the backend returns a full session
   * pair, installed by the BFF. This limited proof is then discarded.
   */
  readonly bearerToken: string;
  readonly userEmail: string;
  /**
   * Called after the user successfully confirms their enrollment. Receives
   * the token-free session confirmation.
   */
  readonly onComplete: (result: MFAResult) => void | Promise<void>;
  readonly onExit?: () => void;
  /** The Träger the Träger-Portal login resolved (#4651). */
  readonly organizationId?: string;
  /**
   * Whether the code step may remember the device, as at every sign-in
   * (#6955): the login's trusted_device_enabled. Undefined or false offers
   * only „Bestätigen“.
   */
  readonly trustedDeviceEnabled?: boolean;
  /** The login's trusted_device_days. */
  readonly trustedDeviceDays?: number;
}

export function MFAEnrollmentScreen({
  scope,
  bearerToken,
  userEmail,
  onComplete,
  onExit,
  organizationId,
  trustedDeviceEnabled = false,
  trustedDeviceDays = 90,
}: MFAEnrollmentScreenProps) {
  const [step, setStep] = useState<Step>("intro");
  // Fehler über den gemeinsamen Weg (#2517). Ein abgelehnter Code bleibt im
  // Formular (MFA_STEP_REFUSALS); ein 401 ohne diesen Code heißt, dass die
  // Einrichtungs-Anmeldung abgelaufen ist, und führt zur Anmeldeseite.
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, clear: clearError } = formErrors;
  const latestStartRef = useRef<() => void>(() => undefined);
  const [isStarting, setIsStarting] = useState(false);
  // Which button is being checked; null while nothing is sent.
  const [pending, setPending] = useState<CodeConfirmChoice | null>(null);
  const isConfirming = pending !== null;
  const [code, setCode] = useState("");
  // Keep the completion result until the explicit Continue action. For
  // every portal it contains status only.
  const resultRef = useRef<MFAResult | null>(null);
  // The challenge enroll/start minted; null for the operator.
  const challengeTokenRef = useRef<string | null>(null);
  const otpRef = useRef<OTPInputGridHandle | null>(null);

  useEffect(() => {
    if (step === "code") {
      otpRef.current?.focus();
    }
  }, [step]);

  const handleStart = async () => {
    setIsStarting(true);
    clearError();
    try {
      challengeTokenRef.current = await enrollStart(scope, bearerToken);
      setStep("code");
    } catch (err) {
      logger.warn("enroll_start_failed", {
        scope,
        error: err instanceof Error ? err.message : String(err),
      });
      void showError(credentialError(err, MFA_STEP_REFUSALS), {
        object: "das Senden des Codes",
        retry: () => latestStartRef.current(),
      });
    } finally {
      setIsStarting(false);
    }
  };

  useLayoutEffect(() => {
    latestStartRef.current = () => void handleStart();
  });

  const handleBack = () => {
    clearError();
    if (step === "code") {
      otpRef.current?.reset();
      setCode("");
      setStep("intro");
      return;
    }
    onExit?.();
  };

  const performConfirm = useCallback(
    async (submittedCode: string, choice: CodeConfirmChoice) => {
      setPending(choice);
      clearError();
      try {
        const result = await enrollConfirm(
          scope,
          bearerToken,
          submittedCode,
          challengeTokenRef.current,
          organizationId,
          // Only the explicit second button remembers the device (#6955).
          trustedDeviceEnabled && choice === "remember",
        );
        resultRef.current = result;
        setStep("success");
      } catch (err) {
        logger.warn("enroll_confirm_failed", {
          scope,
          error: err instanceof Error ? err.message : String(err),
        });
        otpRef.current?.reset();
        setCode("");
        // Kein Wiederholen: der Code steht nicht mehr im Feld.
        void showError(credentialError(err, MFA_STEP_REFUSALS), {
          object: "die Prüfung des Codes",
        });
      } finally {
        setPending(null);
      }
    },
    [
      scope,
      bearerToken,
      organizationId,
      trustedDeviceEnabled,
      clearError,
      showError,
    ],
  );

  const showBack = step !== "success";

  return (
    <div ref={formRef} className="space-y-7 text-left">
      {showBack && (
        <div>
          <button
            type="button"
            onClick={handleBack}
            className="-ml-1 flex items-center gap-1.5 text-sm text-gray-500 transition-colors hover:text-gray-800"
            aria-label="Zurück zum vorherigen Schritt"
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

      <WizardStepper steps={[...STEPS]} current={STEP_INDEX[step]} />

      {step === "success" ? (
        <div className="space-y-5 text-center">
          <EmptyState
            tone="success"
            icon={<Check aria-hidden="true" />}
            titleAs="h2"
            title="2FA erfolgreich aktiviert"
            description="Beim nächsten Login erhalten Sie zusätzlich zu Ihrem Passwort einen Code per E-Mail."
            className="py-0"
          />
          <FormErrorAlert message={formErrors.error} />
          <Button
            type="button"
            variant="primary"
            size="field"
            className="w-full"
            onClick={() => {
              const result = resultRef.current;
              if (!result) {
                // Defensive: success step is only reachable after confirm
                // creates the BFF session and sets resultRef. Otherwise show
                // an error instead of continuing with a broken session.
                formErrors.invalid(
                  "Das hat leider nicht geklappt. Bitte melden Sie sich erneut an.",
                );
                return;
              }
              void Promise.resolve()
                .then(() => onComplete(result))
                .catch((error: unknown) => {
                  // The BFF may have installed the session while the following
                  // browser-safe session read failed. Keep Continue available
                  // and show the failure in this screen instead of dropping it.
                  void showError(error, { object: "die Anmeldung" });
                });
            }}
          >
            Weiter zum Dashboard
          </Button>
        </div>
      ) : (
        <>
          <div className="space-y-1.5">
            <h2 className="text-xl font-semibold text-gray-900">
              {step === "intro"
                ? "Zwei-Faktor-Authentifizierung einrichten"
                : "Code aus E-Mail eingeben"}
            </h2>
            <p className="text-sm leading-relaxed text-gray-600">
              {step === "intro" ? (
                <>
                  Sie brauchen zusätzlich einen Code per E-Mail. Wir senden
                  dafür einen 6-stelligen Code an{" "}
                  <span className="font-medium text-gray-800">{userEmail}</span>
                  {"."}
                </>
              ) : (
                <>
                  Wir haben einen Code an{" "}
                  <span className="font-medium text-gray-800">{userEmail}</span>{" "}
                  gesendet. Geben Sie ihn unten ein.
                </>
              )}
            </p>
          </div>

          <FormErrorAlert message={formErrors.error} />

          {step === "intro" && (
            <div className="space-y-5">
              <Alert
                type="info"
                message={
                  scope === "traeger"
                    ? "Kein Zugriff auf Ihre E-Mails? Wenden Sie sich an das moto-Team. Im Träger-Portal ist der Code immer Pflicht."
                    : "Kein Zugriff auf Ihre E-Mails? Wenden Sie sich an Ihre Schulverwaltung. Der zusätzliche Nachweis bleibt bei besonderen Zugriffsrechten Pflicht."
                }
              />
              <Button
                type="button"
                variant="primary"
                size="field"
                className="w-full"
                onClick={() => {
                  void handleStart();
                }}
                disabled={isStarting}
                isLoading={isStarting}
                loadingText="Code wird gesendet…"
              >
                Code an meine E-Mail senden
              </Button>
            </div>
          )}

          {step === "code" && (
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
                void performConfirm(submitted, choice);
              }}
              onIncomplete={() =>
                formErrors.invalid(
                  `Bitte geben Sie alle ${MFA_CODE_LENGTH} Stellen des Codes ein.`,
                )
              }
            >
              <div className="flex items-center justify-center">
                <Button
                  type="button"
                  variant="link"
                  size="md"
                  onClick={() => {
                    void handleStart();
                  }}
                  disabled={isStarting || isConfirming}
                  tooltip="Schickt einen neuen Code an Ihre E-Mail-Adresse."
                >
                  Keinen Code erhalten? Erneut senden
                </Button>
              </div>
            </MFACodeConfirm>
          )}
        </>
      )}
    </div>
  );
}
