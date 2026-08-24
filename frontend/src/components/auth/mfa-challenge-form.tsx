"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import {
  germanMFAErrorMessage,
  resendChallenge,
  verifyMFA,
  type LoginScope,
  type MFATokenResponse,
} from "~/lib/mfa-api";
import { OTPInputGrid, type OTPInputGridHandle } from "./otp-input-grid";

const logger = createLogger({ component: "MFAChallengeForm" });

const CODE_LENGTH = 6;
const DEFAULT_RESEND_COOLDOWN_SECONDS = 60;

interface MFAChallengeFormProps {
  readonly scope: LoginScope;
  readonly challengeToken: string;
  readonly maskedEmail: string;
  readonly resendCooldownSeconds?: number;
  readonly onSuccess: (tokens: MFATokenResponse) => void | Promise<void>;
  /**
   * Gives the embedding login flow a chance to handle a terminal verify
   * error, such as a portal handoff. Return true when the error is handled.
   */
  readonly onError?: (error: unknown) => boolean | Promise<boolean>;
  readonly onCancel?: () => void;
  /**
   * Mirrors security.mfa_trusted_device_enabled for the tenant. When false
   * we hide the "N Tage merken" checkbox and skip its state in the verify
   * request. Undefined defaults to `true` for backwards compatibility with
   * older backends that haven't shipped the field.
   */
  readonly trustedDeviceEnabled?: boolean;
  /**
   * Day count rendered into the "Auf diesem Gerät N Tage merken" label.
   * Mirrors security.mfa_trusted_device_days so the label always matches
   * the actual cookie lifetime. Defaults to 90 — the registry default —
   * if an older backend doesn't surface the field.
   */
  readonly trustedDeviceDays?: number;
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
}: MFAChallengeFormProps) {
  const [rememberDevice, setRememberDevice] = useState(false);
  const [error, setError] = useState("");
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
    async (submittedCode: string) => {
      setIsVerifying(true);
      setError("");
      try {
        const tokens = await verifyMFA(scope, {
          challengeToken: activeToken,
          code: submittedCode,
          rememberDevice,
        });
        await onSuccess(tokens);
      } catch (err) {
        if (await onError?.(err)) {
          return;
        }
        const msg = germanMFAErrorMessage(err);
        setError(msg);
        logger.warn("mfa_verify_failed", {
          scope,
          error: err instanceof Error ? err.message : String(err),
        });
        otpRef.current?.reset();
      } finally {
        setIsVerifying(false);
      }
    },
    [scope, activeToken, rememberDevice, onSuccess, onError],
  );

  const handleResend = async () => {
    if (resendIn > 0 || isResending) return;
    setIsResending(true);
    setError("");
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
      setError(germanMFAErrorMessage(err));
      logger.warn("mfa_resend_failed", {
        scope,
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setIsResending(false);
    }
  };

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
    <div className="space-y-6 text-left">
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

      {error && <Alert type="error" message={error} />}

      <div className="space-y-3">
        <OTPInputGrid
          ref={otpRef}
          length={CODE_LENGTH}
          disabled={isVerifying}
          onComplete={(code) => {
            // Fire-and-forget: errors surface through state, not the caller.
            void performVerify(code); // NOSONAR typescript:S3735 fire-and-forget pattern matches project convention (10+ existing sites)
          }}
          ariaLabel="6-stelliger Bestätigungscode"
        />
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
            Auf diesem Gerät {trustedDeviceDays} Tage merken
          </label>
        </div>
      )}
    </div>
  );
}
