"use client";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ClipboardEvent,
  type KeyboardEvent,
} from "react";
import { Alert } from "~/components/ui";
import { createLogger } from "~/lib/logger";
import {
  enrollConfirm,
  enrollStart,
  germanMFAErrorMessage,
  type LoginScope,
} from "~/lib/mfa-api";
import { RecoveryCodesDisplay } from "./recovery-codes-display";

const logger = createLogger({ component: "MFAEnrollmentScreen" });

const CODE_LENGTH = 6;
const PRIMARY_BLUE = "#5080D8";
const PRIMARY_GREEN = "#83CD2D";

type Step = "intro" | "code" | "recovery";

interface MFAEnrollmentScreenProps {
  readonly scope: LoginScope;
  readonly bearerToken: string;
  readonly userEmail: string;
  readonly onComplete: () => void | Promise<void>;
}

export function MFAEnrollmentScreen({
  scope,
  bearerToken,
  userEmail,
  onComplete,
}: MFAEnrollmentScreenProps) {
  const [step, setStep] = useState<Step>("intro");
  const [error, setError] = useState("");
  const [isStarting, setIsStarting] = useState(false);
  const [isConfirming, setIsConfirming] = useState(false);
  const [digits, setDigits] = useState<string[]>(() =>
    Array.from({ length: CODE_LENGTH }, () => ""),
  );
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);
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

  const performConfirm = useCallback(
    async (overrideCode?: string) => {
      if (submittedRef.current) return;
      submittedRef.current = true;
      setIsConfirming(true);
      setError("");
      const code = overrideCode ?? digits.join("");
      try {
        const response = await enrollConfirm(scope, bearerToken, code);
        setRecoveryCodes(response.recovery_codes);
        setStep("recovery");
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
        void performConfirm(next.join(""));
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

  if (step === "recovery") {
    return (
      <RecoveryCodesDisplay
        codes={recoveryCodes}
        onConfirm={() => {
          void onComplete();
        }}
      />
    );
  }

  return (
    <div className="space-y-6 text-left">
      <div>
        <h2 className="mb-1 text-xl font-semibold text-gray-900">
          Zwei-Faktor-Authentifizierung einrichten
        </h2>
        <p className="text-sm text-gray-600">
          Ihr Konto erfordert die Aktivierung der Zwei-Faktor-Authentifizierung.
          Beim Login wird zusätzlich zum Passwort ein 6-stelliger Code per
          E-Mail an{" "}
          <span className="font-medium text-gray-800">{userEmail}</span>{" "}
          gesendet.
        </p>
      </div>

      {error && <Alert type="error" message={error} />}

      {step === "intro" && (
        <div className="space-y-3">
          <div className="rounded-lg bg-blue-50 p-3 text-sm text-blue-900">
            <strong>So funktioniert&apos;s:</strong>
            <ol className="mt-1 ml-4 list-decimal space-y-1">
              <li>Sie erhalten einen Bestätigungscode per E-Mail.</li>
              <li>Sie geben den Code hier ein.</li>
              <li>
                Sie erhalten Wiederherstellungscodes für den Notfall (z. B. wenn
                Sie keinen E-Mail-Zugriff haben).
              </li>
            </ol>
          </div>
          <button
            type="button"
            onClick={() => {
              void handleStart();
            }}
            disabled={isStarting}
            className="w-full rounded-xl px-8 py-2.5 text-sm font-semibold text-white transition-all duration-200 hover:opacity-90 focus:outline-none active:scale-95 disabled:cursor-not-allowed disabled:opacity-50"
            style={{ backgroundColor: PRIMARY_GREEN }}
          >
            {isStarting ? "Code wird gesendet…" : "Code an meine E-Mail senden"}
          </button>
        </div>
      )}

      {step === "code" && (
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Wir haben einen Code an{" "}
            <span className="font-medium text-gray-800">{userEmail}</span>{" "}
            gesendet. Geben Sie ihn unten ein:
          </p>

          <div
            className="flex justify-center gap-2"
            role="group"
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
              : "Der Code wird automatisch geprüft, sobald Sie alle 6 Stellen eingegeben haben."}
          </p>

          <button
            type="button"
            onClick={() => {
              void handleStart();
            }}
            disabled={isStarting || isConfirming}
            className="w-full text-sm text-gray-600 transition-colors hover:text-gray-900 hover:underline focus:underline focus:outline-none disabled:cursor-not-allowed disabled:text-gray-400"
            style={{ color: PRIMARY_BLUE }}
          >
            Keinen Code erhalten? Erneut senden
          </button>
        </div>
      )}
    </div>
  );
}
