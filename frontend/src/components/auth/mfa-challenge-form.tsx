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
import { Alert, Button } from "~/components/ui";
import { LOCATION_COLORS } from "~/lib/location-helper";
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

interface MFAChallengeFormProps {
  readonly scope: LoginScope;
  readonly challengeToken: string;
  readonly maskedEmail: string;
  readonly resendCooldownSeconds?: number;
  readonly onSuccess: (tokens: MFATokenResponse) => void | Promise<void>;
  readonly onCancel?: () => void;
  /**
   * Mirrors security.mfa_trusted_device_enabled for the tenant. When false
   * we hide the "30 Tage merken" checkbox and skip its state in the verify
   * request. Undefined defaults to `true` for backwards compatibility with
   * older backends that haven't shipped the field.
   */
  readonly trustedDeviceEnabled?: boolean;
}

export function MFAChallengeForm({
  scope,
  challengeToken,
  maskedEmail,
  resendCooldownSeconds = DEFAULT_RESEND_COOLDOWN_SECONDS,
  onSuccess,
  onCancel,
  trustedDeviceEnabled = true,
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

  const resendLabel =
    resendIn > 0
      ? `Neuen Code anfordern (${resendIn}s)`
      : isResending
        ? "Code wird gesendet…"
        : "Neuen Code anfordern";

  const handleBack = () => {
    if (useRecovery) {
      setError("");
      setRecoveryCode("");
      setDigits(Array.from({ length: CODE_LENGTH }, () => ""));
      setUseRecovery(false);
      return;
    }
    onCancel?.();
  };

  const showBack = useRecovery || Boolean(onCancel);

  return (
    <div className="space-y-6 text-left">
      {showBack && (
        <div>
          <button
            type="button"
            onClick={handleBack}
            disabled={isVerifying}
            className="-ml-1 flex items-center gap-1.5 text-sm text-gray-500 transition-colors hover:text-gray-800 focus:outline-none disabled:cursor-not-allowed disabled:text-gray-300"
            aria-label={
              useRecovery ? "Zurück zur Code-Eingabe" : "Zurück zum Login"
            }
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
        <h2 className="text-xl font-semibold text-gray-900">
          {useRecovery ? "Wiederherstellungscode" : "Code-Eingabe"}
        </h2>
        <p className="text-sm leading-relaxed text-gray-600">
          {useRecovery ? (
            <>
              Geben Sie einen Ihrer gespeicherten Wiederherstellungscodes ein.
              Jeder Code kann nur einmal verwendet werden.
            </>
          ) : (
            <>
              Wir haben einen 6-stelligen Code an{" "}
              <span className="font-medium text-gray-800">{maskedEmail}</span>{" "}
              gesendet. Der Code ist 10 Minuten gültig.
            </>
          )}
        </p>
      </div>

      {error && <Alert type="error" message={error} />}

      {!useRecovery && (
        <div className="space-y-3">
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
          <p className="text-center text-xs text-gray-500">
            {isVerifying
              ? "Code wird geprüft…"
              : "Der Code wird automatisch geprüft, sobald Sie alle 6 Stellen eingegeben haben."}
          </p>
          <div className="flex justify-center">
            <button
              type="button"
              onClick={() => {
                void handleResend();
              }}
              disabled={resendIn > 0 || isResending || isVerifying}
              className="text-sm font-medium text-gray-700 underline-offset-2 transition-colors hover:text-gray-900 hover:underline focus:underline focus:outline-none disabled:cursor-not-allowed disabled:text-gray-400 disabled:no-underline"
            >
              {resendLabel}
            </button>
          </div>
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
          <Button
            type="submit"
            variant="primary"
            size="base"
            className="w-full"
            disabled={isVerifying || recoveryCode.trim().length === 0}
            isLoading={isVerifying}
            loadingText="Wird geprüft…"
          >
            Wiederherstellungscode prüfen
          </Button>
        </form>
      )}

      {trustedDeviceEnabled && (
        <div className="flex items-center justify-center gap-2">
          <input
            id="mfa-remember-device"
            type="checkbox"
            checked={rememberDevice}
            onChange={(e) => setRememberDevice(e.target.checked)}
            disabled={isVerifying}
            className="h-4 w-4 rounded border-gray-300"
            style={{ accentColor: LOCATION_COLORS.GROUP_ROOM }}
          />
          <label
            htmlFor="mfa-remember-device"
            className="text-sm text-gray-700 select-none"
          >
            Auf diesem Gerät 30 Tage merken
          </label>
        </div>
      )}

      {!useRecovery && (
        <div className="border-t border-gray-200 pt-4 text-center">
          <button
            type="button"
            onClick={() => {
              setError("");
              setRecoveryCode("");
              setDigits(Array.from({ length: CODE_LENGTH }, () => ""));
              setUseRecovery(true);
            }}
            disabled={isVerifying}
            className="text-sm text-gray-600 transition-colors hover:text-gray-900 hover:underline focus:underline focus:outline-none disabled:cursor-not-allowed disabled:text-gray-400 disabled:no-underline"
          >
            Wiederherstellungscode verwenden
          </button>
        </div>
      )}
    </div>
  );
}
