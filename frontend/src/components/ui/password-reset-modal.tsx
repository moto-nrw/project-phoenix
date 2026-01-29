"use client";

import React, { useEffect, useState } from "react";
import { Modal } from "./modal";
import { Input, Alert } from "./index";
import { requestPasswordReset, type ApiError } from "~/lib/auth-api";

interface PasswordResetModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
}

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
}: PasswordResetModalProps) {
  const [email, setEmail] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isSuccess, setIsSuccess] = useState(false);
  const [error, setError] = useState("");
  const [rateLimitUntil, setRateLimitUntil] = useState<number | null>(null);
  const [secondsRemaining, setSecondsRemaining] = useState(0);

  const RATE_LIMIT_STORAGE_KEY = "passwordResetRateLimitUntil";

  useEffect(() => {
    if (typeof globalThis === "undefined") return;
    const stored = globalThis.localStorage.getItem(RATE_LIMIT_STORAGE_KEY);
    if (!stored) return;

    const timestamp = Number(stored);
    if (!Number.isNaN(timestamp) && timestamp > Date.now()) {
      setRateLimitUntil(timestamp);
    } else if (!Number.isNaN(timestamp)) {
      globalThis.localStorage.removeItem(RATE_LIMIT_STORAGE_KEY);
    }
  }, []);

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
          globalThis.localStorage.removeItem(RATE_LIMIT_STORAGE_KEY);
        }
        setError("");
      } else {
        const diffSeconds = Math.ceil((rateLimitUntil - now) / 1000);
        setSecondsRemaining(diffSeconds);
        setError(
          `Zu viele Versuche. Bitte versuche es erneut in ${formatCountdown(diffSeconds)}.`,
        );
      }
    };

    updateCountdown();
    const intervalId = globalThis.setInterval(updateCountdown, 1000);
    return () => globalThis.clearInterval(intervalId);
  }, [rateLimitUntil]);

  const rateLimitActive =
    rateLimitUntil !== null && rateLimitUntil > Date.now();

  // Reset state when modal closes
  const handleClose = () => {
    setEmail("");
    setError("");
    setIsSuccess(false);
    setIsLoading(false);
    onClose();
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (rateLimitActive) {
      setError(
        `Zu viele Versuche. Bitte versuche es erneut in ${formatCountdown(Math.max(secondsRemaining, 0))}.`,
      );
      return;
    }

    setIsLoading(true);
    setError("");

    try {
      await requestPasswordReset(email);
      setIsSuccess(true);
    } catch (err) {
      const apiError = err as ApiError | undefined;

      if (apiError?.status === 429) {
        const retrySeconds =
          apiError.retryAfterSeconds && apiError.retryAfterSeconds > 0
            ? apiError.retryAfterSeconds
            : 3600;
        const retryUntil = Date.now() + retrySeconds * 1000;
        setRateLimitUntil(retryUntil);
        setSecondsRemaining(retrySeconds);
        if (typeof globalThis !== "undefined") {
          globalThis.localStorage.setItem(
            RATE_LIMIT_STORAGE_KEY,
            retryUntil.toString(),
          );
        }
        setError(
          `Zu viele Versuche. Bitte versuche es erneut in ${formatCountdown(retrySeconds)}.`,
        );
      } else {
        const errorMessage =
          apiError?.message ??
          "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.";
        setError(errorMessage);
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
            <CheckIcon className="mx-auto mb-4 h-12 w-12 text-[#83cd2d]" />
            <h1 className="mt-4 text-3xl font-bold tracking-tight text-gray-800 sm:text-4xl">
              E-Mail versendet!
            </h1>
            <p className="mt-4 text-gray-600">
              Wir haben Ihnen eine E-Mail mit einem Link zum Zurücksetzen Ihres
              Passworts gesendet.
            </p>
            <p className="mt-2 text-sm text-gray-500">
              Bitte überprüfen Sie Ihren Posteingang und folgen Sie den
              Anweisungen in der E-Mail.
            </p>
            <div className="mt-6">
              <button
                onClick={handleClose}
                className="inline-flex items-center rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white shadow-sm transition-all duration-150 hover:scale-110 hover:shadow-2xl hover:shadow-gray-500/25 focus:ring-2 focus:ring-gray-900 focus:ring-offset-2 focus:outline-none"
              >
                <span>Schließen</span>
              </button>
            </div>
          </>
        ) : (
          <>
            {/* Request Form State */}
            <EmailIcon className="mx-auto mb-4 h-12 w-12 text-gray-700" />
            <h1 className="mt-4 text-3xl font-bold tracking-tight text-gray-800 sm:text-4xl">
              Passwort zurücksetzen
            </h1>
            <p className="mt-4 text-gray-600">
              Geben Sie Ihre E-Mail-Adresse ein und wir senden Ihnen einen Link
              zum Zurücksetzen Ihres Passworts.
            </p>

            <form onSubmit={handleSubmit} noValidate className="mt-6 space-y-4">
              {error && <Alert type="error" message={error} />}

              <div className="text-left">
                <label
                  htmlFor="reset-email"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  E-Mail-Adresse
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
                  className="inline-flex items-center rounded-md bg-gray-200 px-4 py-2 text-sm font-medium text-gray-800 shadow-sm transition-colors hover:bg-gray-300 focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                >
                  Abbrechen
                </button>
                <button
                  type="submit"
                  disabled={isLoading || rateLimitActive}
                  className="inline-flex items-center rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white shadow-sm transition-all duration-150 hover:scale-110 hover:shadow-2xl hover:shadow-gray-500/25 focus:ring-2 focus:ring-gray-900 focus:ring-offset-2 focus:outline-none disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:scale-100"
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
                      <span>Wird gesendet...</span>
                    </>
                  ) : (
                    <span>Link senden</span>
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
