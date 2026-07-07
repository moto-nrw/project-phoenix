"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { WizardStepper } from "~/components/ui/wizard-stepper";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import {
  enrollConfirm,
  enrollStart,
  germanMFAErrorMessage,
  type LoginScope,
  type MFATokenResponse,
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
   * pair, which is handed to onComplete — this token is discarded.
   */
  readonly bearerToken: string;
  readonly userEmail: string;
  /**
   * Called after the user successfully confirms their enrollment. Receives
   * the freshly-minted access/refresh tokens returned by the confirm
   * endpoint so the caller can seed the real NextAuth session.
   */
  readonly onComplete: (tokens: MFATokenResponse) => void | Promise<void>;
  readonly onExit?: () => void;
}

export function MFAEnrollmentScreen({
  scope,
  bearerToken,
  userEmail,
  onComplete,
  onExit,
}: MFAEnrollmentScreenProps) {
  const [step, setStep] = useState<Step>("intro");
  const [error, setError] = useState("");
  const [isStarting, setIsStarting] = useState(false);
  const [isConfirming, setIsConfirming] = useState(false);
  // tokensRef holds the access/refresh pair returned by /enroll/confirm.
  // We stash them in a ref instead of state to avoid an extra render
  // between confirm success and the success-screen render; the success
  // button passes them through to onComplete.
  const tokensRef = useRef<MFATokenResponse | null>(null);
  const otpRef = useRef<OTPInputGridHandle | null>(null);

  useEffect(() => {
    if (step === "code") {
      otpRef.current?.focus();
    }
  }, [step]);

  const handleStart = async () => {
    setIsStarting(true);
    setError("");
    try {
      await enrollStart(scope, bearerToken);
      setStep("code");
    } catch (err) {
      setError(germanMFAErrorMessage(err));
      logger.warn("enroll_start_failed", {
        scope,
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setIsStarting(false);
    }
  };

  const handleBack = () => {
    setError("");
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
      setError("");
      try {
        const tokens = await enrollConfirm(scope, bearerToken, submittedCode);
        tokensRef.current = tokens;
        setStep("success");
      } catch (err) {
        setError(germanMFAErrorMessage(err));
        logger.warn("enroll_confirm_failed", {
          scope,
          error: err instanceof Error ? err.message : String(err),
        });
        otpRef.current?.reset();
      } finally {
        setIsConfirming(false);
      }
    },
    [scope, bearerToken],
  );

  const showBack = step !== "success";

  return (
    <div className="space-y-7 text-left">
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
          <Button
            type="button"
            variant="primary"
            size="base"
            className="w-full"
            onClick={() => {
              const tokens = tokensRef.current;
              if (!tokens) {
                // Defensive: success step is only reachable after confirm
                // resolves with tokens, so tokensRef must be set. If it
                // isn't, surface an error rather than silently falling
                // through to a broken session.
                setError(
                  "Etwas ist schiefgelaufen. Bitte melden Sie sich erneut an.",
                );
                return;
              }
              void onComplete(tokens);
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
                  Ihre Schule verlangt eine zweite Sicherheitsstufe beim Login.
                  Wir senden dafür einen 6-stelligen Code an{" "}
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

          {error && <Alert type="error" message={error} />}

          {step === "intro" && (
            <div className="space-y-5">
              <Alert
                type="info"
                message="Falls Sie keinen Zugriff mehr auf Ihr E-Mail-Postfach haben, kann Ihre Schul-Administration die 2FA für Sie zurücksetzen."
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
                  void performConfirm(code); // NOSONAR typescript:S3735 fire-and-forget pattern matches project convention (10+ existing sites)
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
