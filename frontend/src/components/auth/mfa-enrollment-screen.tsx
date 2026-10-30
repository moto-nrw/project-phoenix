"use client";

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
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { credentialError, MFA_STEP_REFUSALS } from "./credential-error";
import {
  enrollConfirm,
  enrollStart,
  type LoginScope,
  type MFAResult,
} from "~/lib/mfa-api";
import { OTPInputGrid, type OTPInputGridHandle } from "./otp-input-grid";

const logger = createLogger({ component: "MFAEnrollmentScreen" });

const CODE_LENGTH = 6;
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
}

export function MFAEnrollmentScreen({
  scope,
  bearerToken,
  userEmail,
  onComplete,
  onExit,
  organizationId,
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
  const [isConfirming, setIsConfirming] = useState(false);
  // Keep the completion result until the explicit Continue action. For
  // every portal it contains status only.
  const resultRef = useRef<MFAResult | null>(null);
  // The school enroll flow binds the confirm to the exact challenge that
  // enroll/start minted; tenant/operator return no token here (null).
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
      setStep("intro");
      return;
    }
    onExit?.();
  };

  const performConfirm = useCallback(
    async (submittedCode: string) => {
      setIsConfirming(true);
      clearError();
      try {
        const result = await enrollConfirm(
          scope,
          bearerToken,
          submittedCode,
          false,
          challengeTokenRef.current ?? undefined,
          organizationId,
        );
        resultRef.current = result;
        setStep("success");
      } catch (err) {
        logger.warn("enroll_confirm_failed", {
          scope,
          error: err instanceof Error ? err.message : String(err),
        });
        otpRef.current?.reset();
        // Kein Wiederholen: der Code steht nicht mehr im Feld.
        void showError(credentialError(err, MFA_STEP_REFUSALS), {
          object: "die Prüfung des Codes",
        });
      } finally {
        setIsConfirming(false);
      }
    },
    [scope, bearerToken, organizationId, clearError, showError],
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
          <div
            className="mx-auto flex h-14 w-14 items-center justify-center rounded-full"
            style={{ backgroundColor: `${LOCATION_COLORS.GROUP_ROOM}22` }}
            aria-hidden="true"
          >
            <svg
              className="h-7 w-7"
              fill="none"
              stroke={LOCATION_COLORS.GROUP_ROOM}
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2.5}
                d="M5 13l4 4L19 7"
              />
            </svg>
          </div>
          <div className="space-y-1.5">
            <h2 className="text-xl font-semibold text-gray-900">
              2FA erfolgreich aktiviert
            </h2>
            <p className="text-sm leading-relaxed text-gray-600">
              Beim nächsten Login erhalten Sie zusätzlich zu Ihrem Passwort
              einen Code per E-Mail.
            </p>
          </div>
          <FormErrorAlert message={formErrors.error} />
          <Button
            type="button"
            variant="primary"
            size="base"
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
                size="base"
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
            <div className="space-y-5">
              <OTPInputGrid
                ref={otpRef}
                length={CODE_LENGTH}
                disabled={isConfirming}
                onComplete={(code) => {
                  // Fire-and-forget: errors surface through state, not the caller.
                  void performConfirm(code);
                }}
                ariaLabel="6-stelliger Bestätigungscode"
              />
              <p className="text-center text-xs text-gray-500">
                {isConfirming
                  ? "Code wird geprüft…"
                  : "Der Code wird automatisch geprüft, sobald alle 6 Stellen eingegeben sind."}
              </p>

              <div className="flex items-center justify-center">
                <button
                  type="button"
                  onClick={() => {
                    void handleStart();
                  }}
                  disabled={isStarting || isConfirming}
                  className="text-sm font-medium text-gray-700 underline-offset-2 transition-colors hover:text-gray-900 hover:underline focus:underline focus:outline-none disabled:cursor-not-allowed disabled:text-gray-400 disabled:no-underline"
                >
                  Keinen Code erhalten? Erneut senden
                </button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
