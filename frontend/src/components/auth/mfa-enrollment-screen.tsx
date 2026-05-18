"use client";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ClipboardEvent,
  type KeyboardEvent,
} from "react";
import { Alert, Button, WizardStepper } from "~/components/ui";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import {
  enrollConfirm,
  enrollStart,
  germanMFAErrorMessage,
  type LoginScope,
} from "~/lib/mfa-api";

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
  readonly bearerToken: string;
  readonly userEmail: string;
  readonly onComplete: () => void | Promise<void>;
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
  const [digits, setDigits] = useState<string[]>(() =>
    Array.from({ length: CODE_LENGTH }, () => ""),
  );
  const inputRefs = useRef<(HTMLInputElement | null)[]>([]);
  const submittedRef = useRef(false);

  useEffect(() => {
    if (step === "code") {
      inputRefs.current[0]?.focus();
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
      setDigits(Array.from({ length: CODE_LENGTH }, () => ""));
      submittedRef.current = false;
      setStep("intro");
      return;
    }
    onExit?.();
  };

  const performConfirm = useCallback(
    async (overrideCode?: string) => {
      if (submittedRef.current) return;
      submittedRef.current = true;
      setIsConfirming(true);
      setError("");
      const code = overrideCode ?? digits.join("");
      try {
        await enrollConfirm(scope, bearerToken, code);
        setStep("success");
      } catch (err) {
        submittedRef.current = false;
        setError(germanMFAErrorMessage(err));
        logger.warn("enroll_confirm_failed", {
          scope,
          error: err instanceof Error ? err.message : String(err),
        });
        setDigits(Array.from({ length: CODE_LENGTH }, () => ""));
        inputRefs.current[0]?.focus();
      } finally {
        setIsConfirming(false);
      }
    },
    [scope, bearerToken, digits],
  );

  const handleDigitChange = (index: number, value: string) => {
    const sanitized = value.replace(/\D/g, "");
    if (sanitized.length === 0) {
      setDigits((prev) => {
        const next = [...prev];
        next[index] = "";
        return next;
      });
      return;
    }

    setDigits((prev) => {
      const next = [...prev];
      const chars = sanitized.slice(0, CODE_LENGTH - index).split("");
      for (let i = 0; i < chars.length; i++) {
        next[index + i] = chars[i] ?? "";
      }
      const focusTarget = Math.min(index + chars.length, CODE_LENGTH - 1);
      requestAnimationFrame(() => inputRefs.current[focusTarget]?.focus());
      const filled = next.every((d) => d !== "");
      if (filled && !submittedRef.current) {
        // Fire-and-forget: the confirm call is awaited internally and
        // surfaces errors through state, not the caller.
        void performConfirm(next.join("")); //NOSONAR(typescript:S3735)
      }
      return next;
    });
  };

  const handleKeyDown = (index: number, e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Backspace") {
      if (digits[index]) {
        setDigits((prev) => {
          const next = [...prev];
          next[index] = "";
          return next;
        });
        return;
      }
      if (index > 0) {
        e.preventDefault();
        inputRefs.current[index - 1]?.focus();
        setDigits((prev) => {
          const next = [...prev];
          next[index - 1] = "";
          return next;
        });
      }
    } else if (e.key === "ArrowLeft" && index > 0) {
      e.preventDefault();
      inputRefs.current[index - 1]?.focus();
    } else if (e.key === "ArrowRight" && index < CODE_LENGTH - 1) {
      e.preventDefault();
      inputRefs.current[index + 1]?.focus();
    }
  };

  const handlePaste = (e: ClipboardEvent<HTMLInputElement>) => {
    const pasted = e.clipboardData.getData("text").replace(/\D/g, "");
    if (pasted.length === 0) return;
    e.preventDefault();
    handleDigitChange(0, pasted);
  };

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
              void onComplete();
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
              <div
                className="flex justify-center gap-2"
                aria-label="6-stelliger Bestätigungscode"
              >
                {digits.map((digit, index) => (
                  <input
                    // eslint-disable-next-line react/no-array-index-key -- positional digit slots
                    key={index}
                    ref={(el) => {
                      inputRefs.current[index] = el;
                    }}
                    type="text"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    pattern="[0-9]"
                    maxLength={CODE_LENGTH}
                    value={digit}
                    onChange={(e) => handleDigitChange(index, e.target.value)}
                    onKeyDown={(e) => handleKeyDown(index, e)}
                    onPaste={handlePaste}
                    disabled={isConfirming}
                    aria-label={`Stelle ${index + 1}`}
                    className="h-14 w-12 rounded-lg border-0 bg-white text-center text-2xl font-semibold text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all ring-inset focus:outline-none focus-visible:ring-2 focus-visible:ring-[#5080D8] disabled:bg-gray-50 disabled:text-gray-400"
                  />
                ))}
              </div>
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
