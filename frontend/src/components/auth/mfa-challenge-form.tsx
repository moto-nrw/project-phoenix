"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ClipboardEvent,
  type KeyboardEvent,
} from "react";
import { Alert } from "~/components/ui";
import { createLogger } from "~/lib/logger";
import {
  germanMFAErrorMessage,
  resendChallenge,
  verifyMFA,
  verifyRecovery,
  type LoginScope,
  type MFATokenResponse,
} from "~/lib/mfa-api";

const logger = createLogger({ component: "MFAChallengeForm" });

const CODE_LENGTH = 6;
const DEFAULT_RESEND_COOLDOWN_SECONDS = 60;
const PRIMARY_BLUE = "#5080D8";
const PRIMARY_GREEN = "#83CD2D";

interface MFAChallengeFormProps {
  readonly scope: LoginScope;
  readonly challengeToken: string;
  readonly maskedEmail: string;
  readonly resendCooldownSeconds?: number;
  readonly onSuccess: (tokens: MFATokenResponse) => void | Promise<void>;
  readonly onCancel?: () => void;
}

export function MFAChallengeForm({
  scope,
  challengeToken,
  maskedEmail,
  resendCooldownSeconds = DEFAULT_RESEND_COOLDOWN_SECONDS,
  onSuccess,
  onCancel,
}: MFAChallengeFormProps) {
  const [digits, setDigits] = useState<string[]>(() =>
    Array.from({ length: CODE_LENGTH }, () => ""),
  );
  const [recoveryCode, setRecoveryCode] = useState("");
  const [useRecovery, setUseRecovery] = useState(false);
  const [rememberDevice, setRememberDevice] = useState(false);
  const [error, setError] = useState("");
  const [isVerifying, setIsVerifying] = useState(false);
  const [isResending, setIsResending] = useState(false);
  const [resendIn, setResendIn] = useState(resendCooldownSeconds);
  const inputRefs = useRef<(HTMLInputElement | null)[]>([]);
  const submittedRef = useRef(false);

  const code = useMemo(() => digits.join(""), [digits]);

  useEffect(() => {
    if (!useRecovery) {
      inputRefs.current[0]?.focus();
    }
  }, [useRecovery]);

  useEffect(() => {
    if (resendIn <= 0) return;
    const timer = setTimeout(() => setResendIn((s) => s - 1), 1000);
    return () => clearTimeout(timer);
  }, [resendIn]);

  const performVerify = useCallback(
    async (overrideCode?: string) => {
      if (submittedRef.current) return;
      submittedRef.current = true;
      setIsVerifying(true);
      setError("");
      try {
        const tokens = await verifyMFA(scope, {
          challengeToken,
          code: overrideCode ?? code,
          rememberDevice,
        });
        await onSuccess(tokens);
      } catch (err) {
        submittedRef.current = false;
        const msg = germanMFAErrorMessage(err);
        setError(msg);
        logger.warn("mfa_verify_failed", {
          scope,
          error: err instanceof Error ? err.message : String(err),
        });
        setDigits(Array.from({ length: CODE_LENGTH }, () => ""));
        inputRefs.current[0]?.focus();
      } finally {
        setIsVerifying(false);
      }
    },
    [scope, challengeToken, code, rememberDevice, onSuccess],
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
        void performVerify(next.join(""));
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

  const handleRecoverySubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (recoveryCode.trim().length === 0 || submittedRef.current) return;
    submittedRef.current = true;
    setIsVerifying(true);
    setError("");
    try {
      const tokens = await verifyRecovery(scope, {
        challengeToken,
        recoveryCode: recoveryCode.trim(),
        rememberDevice,
      });
      await onSuccess(tokens);
    } catch (err) {
      submittedRef.current = false;
      setError(germanMFAErrorMessage(err));
      logger.warn("mfa_recovery_failed", {
        scope,
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setIsVerifying(false);
    }
  };

  const handleResend = async () => {
    if (resendIn > 0 || isResending) return;
    setIsResending(true);
    setError("");
    try {
      await resendChallenge(scope, { challengeToken });
      setResendIn(resendCooldownSeconds);
      setDigits(Array.from({ length: CODE_LENGTH }, () => ""));
      inputRefs.current[0]?.focus();
    } catch (err) {
      setError(germanMFAErrorMessage(err));
      logger.warn("mfa_resend_failed", {
        scope,
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setIsResending(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="text-left">
        <h2 className="mb-1 text-xl font-semibold text-gray-900">
          Code-Eingabe
        </h2>
        <p className="text-sm text-gray-600">
          Wir haben einen 6-stelligen Code an{" "}
          <span className="font-medium text-gray-800">{maskedEmail}</span>{" "}
          gesendet. Der Code ist 10 Minuten gültig.
        </p>
      </div>

      {error && <Alert type="error" message={error} />}

      {!useRecovery && (
        <div>
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
                disabled={isVerifying}
                aria-label={`Stelle ${index + 1}`}
                className="h-14 w-12 rounded-lg border-0 bg-white text-center text-2xl font-semibold text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all ring-inset focus:outline-none focus-visible:ring-2 focus-visible:ring-[#5080D8] disabled:bg-gray-50 disabled:text-gray-400"
              />
            ))}
          </div>
          <p className="mt-3 text-center text-xs text-gray-500">
            {isVerifying
              ? "Code wird geprüft…"
              : "Der Code wird automatisch geprüft, sobald Sie alle 6 Stellen eingegeben haben."}
          </p>
        </div>
      )}

      {useRecovery && (
        <form onSubmit={handleRecoverySubmit} className="space-y-3">
          <label
            htmlFor="recovery-code"
            className="block text-sm font-medium text-gray-700"
          >
            Wiederherstellungscode
          </label>
          <input
            id="recovery-code"
            name="recovery-code"
            type="text"
            inputMode="text"
            autoComplete="one-time-code"
            value={recoveryCode}
            onChange={(e) => setRecoveryCode(e.target.value)}
            disabled={isVerifying}
            placeholder="xxxx-xxxx-xxxx-xxxx"
            className="block w-full rounded-lg border-0 bg-white px-4 py-3 font-mono text-base tracking-wider text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all ring-inset focus:outline-none focus-visible:ring-2 focus-visible:ring-[#5080D8] disabled:bg-gray-50"
          />
          <button
            type="submit"
            disabled={isVerifying || recoveryCode.trim().length === 0}
            className="w-full rounded-xl px-8 py-2.5 text-sm font-semibold text-white transition-all duration-200 hover:opacity-90 focus:outline-none active:scale-95 disabled:cursor-not-allowed disabled:opacity-50"
            style={{ backgroundColor: PRIMARY_BLUE }}
          >
            {isVerifying ? "Wird geprüft…" : "Wiederherstellungscode prüfen"}
          </button>
        </form>
      )}

      <div className="flex items-center justify-center gap-2">
        <input
          id="mfa-remember-device"
          type="checkbox"
          checked={rememberDevice}
          onChange={(e) => setRememberDevice(e.target.checked)}
          disabled={isVerifying}
          className="h-4 w-4 rounded border-gray-300 accent-[#83CD2D]"
          style={{ accentColor: PRIMARY_GREEN }}
        />
        <label
          htmlFor="mfa-remember-device"
          className="text-sm text-gray-700 select-none"
        >
          Auf diesem Gerät 30 Tage merken
        </label>
      </div>

      <div className="flex flex-col items-center gap-2 text-sm">
        <button
          type="button"
          onClick={() => {
            void handleResend();
          }}
          disabled={resendIn > 0 || isResending || isVerifying}
          className="text-gray-600 transition-colors hover:text-gray-900 hover:underline focus:underline focus:outline-none disabled:cursor-not-allowed disabled:text-gray-400 disabled:no-underline"
        >
          {resendIn > 0
            ? `Neuen Code anfordern (${resendIn}s)`
            : isResending
              ? "Code wird gesendet…"
              : "Neuen Code anfordern"}
        </button>

        <button
          type="button"
          onClick={() => {
            setError("");
            setRecoveryCode("");
            setDigits(Array.from({ length: CODE_LENGTH }, () => ""));
            setUseRecovery((v) => !v);
          }}
          disabled={isVerifying}
          className="text-gray-600 transition-colors hover:text-gray-900 hover:underline focus:underline focus:outline-none disabled:cursor-not-allowed disabled:text-gray-400"
        >
          {useRecovery
            ? "Zurück zur Code-Eingabe"
            : "Wiederherstellungscode verwenden"}
        </button>

        {onCancel && (
          <button
            type="button"
            onClick={onCancel}
            disabled={isVerifying}
            className="text-gray-500 transition-colors hover:text-gray-700 hover:underline focus:underline focus:outline-none disabled:cursor-not-allowed disabled:text-gray-300"
          >
            Abbrechen
          </button>
        )}
      </div>
    </div>
  );
}
