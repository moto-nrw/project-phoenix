"use client";

import React, { useCallback, useEffect, useState } from "react";
import { Modal } from "./modal";
import { Alert } from "./alert";
import { Input } from "./input";
import { requestPasswordReset, type ApiError } from "~/lib/auth-api";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PasswordReset" });

interface PasswordResetModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onRequestReset?: (email: string) => Promise<{ message: string }>;
  readonly rateLimitStorageKey?: string;
  readonly copy?: PasswordResetModalCopy;
}

interface PasswordResetModalCopy {
  readonly title: string;
  readonly description: string;
  readonly emailLabel: string;
  readonly cancel: string;
  readonly submit: string;
  readonly submitting: string;
  readonly successTitle: string;
  readonly successMessage: string;
  readonly successHint: string;
  readonly close: string;
  readonly rateLimitError: (countdown: string) => string;
  readonly genericError: string;
}

const DEFAULT_PASSWORD_RESET_MODAL_COPY: PasswordResetModalCopy = {
  title: "Passwort zurücksetzen",
  description:
    "Geben Sie Ihre E-Mail-Adresse ein und wir senden Ihnen einen Link zum Zurücksetzen Ihres Passworts.",
  emailLabel: "E-Mail-Adresse",
  cancel: "Abbrechen",
  submit: "Link senden",
  submitting: "Wird gesendet...",
  successTitle: "E-Mail versendet!",
  successMessage:
    "Wir haben Ihnen eine E-Mail mit einem Link zum Zurücksetzen Ihres Passworts gesendet.",
  successHint:
    "Bitte überprüfen Sie Ihren Posteingang und folgen Sie den Anweisungen in der E-Mail.",
  close: "Schließen",
  rateLimitError: (countdown) =>
    `Zu viele Versuche. Bitte versuche es erneut in ${countdown}.`,
  genericError: "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
};

// Email Icon Component
const EmailIcon = ({ className }: { className?: string }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    width="48"
    height="48"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
  >
    <rect x="2" y="4" width="20" height="16" rx="2" />
    <path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7" />
  </svg>
);

// Success Check Icon Component
const CheckIcon = ({ className }: { className?: string }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    width="48"
    height="48"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
  >
    <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
    <polyline points="22 4 12 14.01 9 11.01" />
  </svg>
);

const formatCountdown = (totalSeconds: number): string => {
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes.toString().padStart(2, "0")}:${seconds.toString().padStart(2, "0")}`;
};

export function PasswordResetModal({
  isOpen,
  onClose,
  onRequestReset = requestPasswordReset,
  rateLimitStorageKey = "passwordResetRateLimitUntil",
  copy = DEFAULT_PASSWORD_RESET_MODAL_COPY,
}: PasswordResetModalProps) {
  const [email, setEmail] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isSuccess, setIsSuccess] = useState(false);
  const [error, setError] = useState("");
  const errorRef = useScrollToError(error);
  const [rateLimitUntil, setRateLimitUntil] = useState<number | null>(null);
  const [secondsRemaining, setSecondsRemaining] = useState(0);

  useEffect(() => {
    if (typeof globalThis === "undefined") return;
    const stored = globalThis.localStorage.getItem(rateLimitStorageKey);
    if (!stored) return;

    const timestamp = Number(stored);
    if (!Number.isNaN(timestamp) && timestamp > Date.now()) {
      setRateLimitUntil(timestamp);
    } else if (!Number.isNaN(timestamp)) {
      globalThis.localStorage.removeItem(rateLimitStorageKey);
    }
  }, [rateLimitStorageKey]);

  useEffect(() => {
    if (!rateLimitUntil) {
      setSecondsRemaining(0);
      return;
    }

    const updateCountdown = () => {
      const now = Date.now();
      if (rateLimitUntil <= now) {
        setRateLimitUntil(null);
        setSecondsRemaining(0);
        if (typeof globalThis !== "undefined") {
          globalThis.localStorage.removeItem(rateLimitStorageKey);
        }
        setError("");
      } else {
        const diffSeconds = Math.ceil((rateLimitUntil - now) / 1000);
        setSecondsRemaining(diffSeconds);
        setError(copy.rateLimitError(formatCountdown(diffSeconds)));
      }
    };

    updateCountdown();
    const intervalId = globalThis.setInterval(updateCountdown, 1000);
    return () => globalThis.clearInterval(intervalId);
  }, [copy, rateLimitStorageKey, rateLimitUntil]);

  const rateLimitActive =
    rateLimitUntil !== null && rateLimitUntil > Date.now();

  // Reset state when modal closes
  const handleClose = useCallback(() => {
    setEmail("");
    setError("");
    setIsSuccess(false);
    setIsLoading(false);
    onClose();
  }, [onClose]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (rateLimitActive) {
      setError(
        copy.rateLimitError(formatCountdown(Math.max(secondsRemaining, 0))),
      );
      return;
    }

    setIsLoading(true);
    setError("");

    try {
      await onRequestReset(email);
      setIsSuccess(true);
    } catch (err) {
      const apiError = err as ApiError | undefined;

      if (apiError?.status === 429) {
        logger.warn("password_reset_rate_limited", {
          error: "rate_limit_exceeded",
        });
        const retrySeconds =
          apiError.retryAfterSeconds && apiError.retryAfterSeconds > 0
            ? apiError.retryAfterSeconds
            : 3600;
        const retryUntil = Date.now() + retrySeconds * 1000;
        setRateLimitUntil(retryUntil);
        setSecondsRemaining(retrySeconds);
        if (typeof globalThis !== "undefined") {
          // This is a numeric rate-limit expiry, not an authentication token.
          globalThis.localStorage.setItem(
            rateLimitStorageKey,
            retryUntil.toString(),
          );
        }
        setError(copy.rateLimitError(formatCountdown(retrySeconds)));
      } else {
        logger.error("password_reset_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError(copy.genericError);
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={handleClose} title="" footer={undefined}>
      <div className="text-center">
        {isSuccess ? (
          <>
            {/* Success State */}
            <CheckIcon className="text-moto-green-strong mx-auto mb-4 h-12 w-12" />
            <h1 className="mt-4 text-3xl font-bold tracking-tight text-gray-800 sm:text-4xl">
              {copy.successTitle}
            </h1>
            <p className="mt-4 text-gray-600">{copy.successMessage}</p>
            <p className="mt-2 text-sm text-gray-500">{copy.successHint}</p>
            <div className="mt-6">
              <button
                type="button"
                onClick={handleClose}
                className="inline-flex items-center rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white shadow-sm transition-all duration-150 hover:scale-110 hover:shadow-2xl hover:shadow-gray-500/25 focus:outline-none"
              >
                <span>{copy.close}</span>
              </button>
            </div>
          </>
        ) : (
          <>
            {/* Request Form State */}
            <EmailIcon className="mx-auto mb-4 h-12 w-12 text-gray-700" />
            <h1 className="mt-4 text-3xl font-bold tracking-tight text-gray-800 sm:text-4xl">
              {copy.title}
            </h1>
            <p className="mt-4 text-gray-600">{copy.description}</p>

            <form onSubmit={handleSubmit} noValidate className="mt-6 space-y-4">
              {error && (
                <div ref={errorRef}>
                  <Alert type="error" message={error} />
                </div>
              )}

              <div className="text-left">
                <label
                  htmlFor="reset-email"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  {copy.emailLabel}
                </label>
                <Input
                  id="reset-email"
                  name="email"
                  type="email"
                  autoComplete="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full"
                  label=""
                  disabled={isLoading}
                />
              </div>

              <div className="flex justify-center gap-3 pt-2">
                <button
                  type="button"
                  onClick={handleClose}
                  disabled={isLoading}
                  className="inline-flex items-center rounded-md bg-gray-200 px-4 py-2 text-sm font-medium text-gray-800 shadow-sm transition-colors hover:bg-gray-300 focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {copy.cancel}
                </button>
                <button
                  type="submit"
                  disabled={isLoading || rateLimitActive}
                  className="inline-flex items-center rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white shadow-sm transition-all duration-150 hover:scale-110 hover:shadow-2xl hover:shadow-gray-500/25 focus:outline-none disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:scale-100"
                >
                  {isLoading ? (
                    <>
                      <svg
                        className="mr-2 -ml-1 h-4 w-4 animate-spin text-white"
                        xmlns="http://www.w3.org/2000/svg"
                        fill="none"
                        viewBox="0 0 24 24"
                      >
                        <circle
                          className="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          strokeWidth="4"
                        ></circle>
                        <path
                          className="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                        ></path>
                      </svg>
                      <span>{copy.submitting}</span>
                    </>
                  ) : (
                    <span>{copy.submit}</span>
                  )}
                </button>
              </div>
            </form>
          </>
        )}
      </div>
    </Modal>
  );
}
