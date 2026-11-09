"use client";

import { EmptyState } from "~/components/ui/empty-state";
import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useSession } from "next-auth/react";
import { Mail, Check, X } from "lucide-react";
import { Button, buttonClassName } from "~/components/ui/button";
import { Loading } from "~/components/ui/loading";
import { SectionCard } from "~/components/ui/section-card";
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

function ResultCard({ children }: { readonly children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <SectionCard overflow="visible" className="w-full max-w-md text-center">
        {children}
      </SectionCard>
    </div>
  );
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
        <Loading />
      </div>
    );
  }

  if (state === "idle") {
    return (
      <ResultCard>
        <EmptyState
          tone="neutral"
          icon={<Mail aria-hidden="true" />}
          titleAs="h1"
          title="E-Mail-Adresse bestätigen"
          description="Klicke auf den Button, um deine neue E-Mail-Adresse zu bestätigen."
          className="py-0 pb-6"
        />
        <Button
          type="button"
          variant="primary"
          size="base"
          ref={primaryRef as React.RefObject<HTMLButtonElement>}
          onClick={() => void handleConfirm()}
        >
          Jetzt bestätigen
        </Button>
      </ResultCard>
    );
  }

  if (state === "confirming") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <Loading />
          <p className="mt-4 text-gray-600">E-Mail wird bestätigt...</p>
        </div>
      </div>
    );
  }

  if (state === "success") {
    return (
      <ResultCard>
        <EmptyState
          tone="success"
          icon={<Check aria-hidden="true" />}
          titleAs="h1"
          title="E-Mail-Adresse geändert"
          description="Deine E-Mail-Adresse wurde erfolgreich geändert. Es kann einige Minuten dauern, bis die Änderung in deinem Profil sichtbar ist. Eine erneute Anmeldung übernimmt die Änderung sofort."
          className="py-0 pb-6"
        />
        <Link
          href={operatorPath("/operator/settings?tab=profile")}
          ref={primaryRef as React.RefObject<HTMLAnchorElement>}
          className={buttonClassName({
            variant: "primary",
            size: "base",
            className: "",
          })}
        >
          Weiter
        </Link>
      </ResultCard>
    );
  }

  return (
    <ResultCard>
      <EmptyState
        tone="error"
        icon={<X aria-hidden="true" />}
        titleAs="h1"
        title="Bestätigung fehlgeschlagen"
        className="py-0 pb-4"
      />
      <FormErrorAlert
        message={confirmErrors.error}
        className="mb-6 text-left"
      />
      <div className="flex flex-col items-center gap-3">
        <Link
          href={operatorPath("/operator/settings")}
          ref={primaryRef as React.RefObject<HTMLAnchorElement>}
          className={buttonClassName({
            variant: "primary",
            size: "base",
            className: "",
          })}
        >
          Zu den Einstellungen
        </Link>
      </div>
    </ResultCard>
  );
}
