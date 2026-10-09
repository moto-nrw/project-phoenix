"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useSession } from "next-auth/react";
import { Mail, Check, X } from "lucide-react";
import { Loading } from "~/components/ui/loading";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { apiErrorFromResponse, transportFetch } from "~/lib/api-error";
import { operatorPath } from "~/lib/operator-url";
import { createLogger } from "~/lib/logger";
const logger = createLogger({ component: "OperatorEmailConfirmPage" });

type ConfirmState = "loading" | "idle" | "confirming" | "success" | "error";

/**
 * Extracts the token from the URL query string (?token=...).
 * The caller strips the token from the URL immediately after extraction via
 * history.replaceState, so it does not persist in the address bar, browser
 * history, or any subsequent Referer header.
 */
function extractToken(): string | null {
  if (typeof window === "undefined") return null;
  return new URLSearchParams(window.location.search).get("token");
}

export function EmailConfirmContent() {
  const [token, setToken] = useState<string | null>(null);
  const [state, setState] = useState<ConfirmState>("loading");
  const confirmErrors = useApiFormError();
  const { show: showConfirmError, invalid: invalidConfirm } = confirmErrors;
  const { update: updateSession, status: sessionStatus } = useSession();
  const primaryRef = useRef<HTMLButtonElement | HTMLAnchorElement>(null);

  // Extract token from URL query string on mount, then strip it from the URL
  // to prevent leaking via browser history or shoulder-surfing.
  useEffect(() => {
    const extracted = extractToken();
    if (extracted) {
      setToken(extracted);
      setState("idle");
      window.history.replaceState({}, "", window.location.pathname);
    } else {
      invalidConfirm(
        "Der Link enthält keinen Bestätigungscode. Bitte öffnen Sie den Link aus der E-Mail erneut.",
      );
      setState("error");
    }
  }, [invalidConfirm]);

  useEffect(() => {
    primaryRef.current?.focus();
  }, [state]);

  const handleConfirm = useCallback(async () => {
    if (!token || state === "confirming") return;

    setState("confirming");
    try {
      const response = await transportFetch(
        "/api/operator/auth/email-confirm",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ token }),
        },
      );

      if (!response.ok) {
        throw await apiErrorFromResponse(
          response,
          `Email confirm failed (${response.status})`,
        );
      }

      setState("success");
      // If the user has an active session, force-expire the access token
      // so the next JWT callback triggers a proactive refresh — picking up
      // the new email from the backend without waiting ~10 minutes.
      if (sessionStatus === "authenticated") {
        try {
          await updateSession({ emailChanged: true });
        } catch {
          // Best-effort: session refresh will happen naturally on next token cycle
        }
      }
    } catch (err) {
      logger.warn("email_confirm_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showConfirmError(err, {
        object: "die Bestätigung der E-Mail-Adresse",
        retry: () => void handleConfirm(),
      });
      setState("error");
    }
  }, [token, state, sessionStatus, updateSession, showConfirmError]);

  if (state === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Loading fullPage={false} />
      </div>
    );
  }

  if (state === "idle") {
    return (
      <div className="flex min-h-screen items-center justify-center p-4">
        <div className="moto-content-surface w-full max-w-md rounded-2xl border p-8 text-center shadow-sm">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-xl bg-gray-100">
            <Mail className="h-8 w-8 text-gray-600" aria-hidden="true" />
          </div>
          <h1 className="mb-2 text-xl font-semibold text-gray-900">
            E-Mail-Adresse bestätigen
          </h1>
          <p className="mb-6 text-gray-600">
            Klicke auf den Button, um deine neue E-Mail-Adresse zu bestätigen.
          </p>
          <button
            type="button"
            ref={primaryRef as React.RefObject<HTMLButtonElement>}
            onClick={() => void handleConfirm()}
            className="inline-block rounded-lg bg-gray-900 px-6 py-3 text-sm font-medium text-white transition-all hover:bg-gray-700"
          >
            Jetzt bestätigen
          </button>
        </div>
      </div>
    );
  }

  if (state === "confirming") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <Loading fullPage={false} />
          <p className="mt-4 text-gray-600">E-Mail wird bestätigt...</p>
        </div>
      </div>
    );
  }

  if (state === "success") {
    return (
      <div className="flex min-h-screen items-center justify-center p-4">
        <div className="moto-content-surface w-full max-w-md rounded-2xl border p-8 text-center shadow-sm">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-xl bg-gray-100">
            <Check className="text-moto-green h-8 w-8" aria-hidden="true" />
          </div>
          <h1 className="mb-2 text-xl font-semibold text-gray-900">
            E-Mail-Adresse geändert
          </h1>
          <p className="mb-2 text-gray-600">
            Deine E-Mail-Adresse wurde erfolgreich geändert.
          </p>
          <p className="mb-6 text-sm text-gray-400">
            Es kann einige Minuten dauern, bis die Änderung in deinem Profil
            sichtbar ist. Eine erneute Anmeldung übernimmt die Änderung sofort.
          </p>
          <Link
            href={operatorPath("/operator/settings?tab=profile")}
            ref={primaryRef as React.RefObject<HTMLAnchorElement>}
            className="inline-block rounded-lg bg-gray-900 px-6 py-3 text-sm font-medium text-white transition-all hover:bg-gray-700"
          >
            Weiter
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <div className="moto-content-surface w-full max-w-md rounded-2xl border p-8 text-center shadow-sm">
        <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-xl bg-gray-100">
          <X className="text-moto-red h-8 w-8" aria-hidden="true" />
        </div>
        <h1 className="mb-2 text-xl font-semibold text-gray-900">
          Bestätigung fehlgeschlagen
        </h1>
        <FormErrorAlert
          message={confirmErrors.error}
          className="mb-6 text-left"
        />
        <div className="flex flex-col items-center gap-3">
          <Link
            href={operatorPath("/operator/settings")}
            ref={primaryRef as React.RefObject<HTMLAnchorElement>}
            className="inline-block rounded-lg bg-gray-900 px-6 py-3 text-sm font-medium text-white transition-all hover:bg-gray-700"
          >
            Zu den Einstellungen
          </Link>
        </div>
      </div>
    </div>
  );
}
