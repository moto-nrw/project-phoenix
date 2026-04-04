"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { operatorPath } from "~/lib/operator-url";
import {
  validateOperatorInvitation,
  acceptOperatorInvitation,
} from "~/lib/operator/operator-invitation-api";
import type { OperatorInvitationValidation } from "~/lib/operator/operator-invitation-helpers";
import { PASSWORD_RULES } from "~/lib/password-rules";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorInviteAcceptPage" });

type PageState = "loading" | "form" | "submitting" | "success" | "error";

const SESSION_STORAGE_KEY = "operator_invite_token";

function extractToken(): string | null {
  if (typeof window === "undefined") return null;

  // 1. Try URL fragment first (fresh link click)
  const hash = window.location.hash;
  if (hash.startsWith("#token=")) {
    const token = hash.slice("#token=".length);
    if (token) {
      try {
        sessionStorage.setItem(SESSION_STORAGE_KEY, token);
      } catch {
        // sessionStorage unavailable (private browsing) — fall through
      }
      return token;
    }
  }

  // 2. Fall back to sessionStorage (page reload)
  try {
    return sessionStorage.getItem(SESSION_STORAGE_KEY);
  } catch {
    return null;
  }
}

function clearPersistedToken(): void {
  try {
    sessionStorage.removeItem(SESSION_STORAGE_KEY);
  } catch {
    // ignore
  }
}

export function InviteContent() {
  const [token, setToken] = useState<string | null>(null);
  const [invitation, setInvitation] =
    useState<OperatorInvitationValidation | null>(null);
  const [state, setState] = useState<PageState>("loading");
  const [errorMessage, setErrorMessage] = useState("");

  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [formError, setFormError] = useState<string | null>(null);

  const primaryRef = useRef<HTMLButtonElement>(null);
  const currentTokenRef = useRef<string | null>(null);

  const processToken = useCallback(() => {
    const extracted = extractToken();
    if (!extracted) {
      setErrorMessage("Kein Token angegeben.");
      setState("error");
      return;
    }

    // Skip if we're already processing this exact token
    if (extracted === currentTokenRef.current) return;
    currentTokenRef.current = extracted;

    setToken(extracted);
    setState("loading");
    setFormError(null);
    setPassword("");
    setConfirmPassword("");

    // Strip fragment from URL to prevent shoulder-surfing, but keep in sessionStorage for reload
    if (window.location.hash) {
      window.history.replaceState({}, "", window.location.pathname);
    }

    validateOperatorInvitation(extracted)
      .then((data) => {
        setInvitation(data);
        setDisplayName(data.displayName ?? "");
        setState("form");
      })
      .catch((err) => {
        logger.error("invitation_validation_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setErrorMessage(
          err instanceof Error
            ? err.message
            : "Einladung nicht gefunden oder abgelaufen.",
        );
        setState("error");
      });
  }, []);

  // Process token on mount and when the hash changes (new invitation link in same tab)
  useEffect(() => {
    processToken();

    const handleHashChange = () => processToken();
    window.addEventListener("hashchange", handleHashChange);
    return () => window.removeEventListener("hashchange", handleHashChange);
  }, [processToken]);

  const allPasswordRulesMet = PASSWORD_RULES.every((rule) =>
    rule.test(password),
  );
  const passwordsMatch = password === confirmPassword && password.length > 0;

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!token || state === "submitting") return;

      setFormError(null);

      if (!displayName.trim()) {
        setFormError("Anzeigename ist erforderlich.");
        return;
      }
      if (!allPasswordRulesMet) {
        setFormError("Passwort erfüllt nicht alle Anforderungen.");
        return;
      }
      if (!passwordsMatch) {
        setFormError("Passwörter stimmen nicht überein.");
        return;
      }

      setState("submitting");

      try {
        await acceptOperatorInvitation(token, {
          display_name: displayName.trim(),
          password,
          confirm_password: confirmPassword,
        });
        clearPersistedToken();
        setState("success");
      } catch (err) {
        logger.error("invitation_accept_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setFormError(
          err instanceof Error
            ? err.message
            : "Konto konnte nicht erstellt werden.",
        );
        setState("form");
      }
    },
    [
      token,
      state,
      displayName,
      password,
      confirmPassword,
      allPasswordRulesMet,
      passwordsMatch,
    ],
  );

  if (state === "loading") {
    return null;
  }

  if (state === "success") {
    return (
      <div className="flex min-h-screen items-center justify-center p-4">
        <div className="w-full max-w-md rounded-2xl border border-gray-100 bg-white p-8 text-center shadow-lg">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-green-100">
            <svg
              className="h-8 w-8 text-green-600"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M5 13l4 4L19 7"
              />
            </svg>
          </div>
          <h1 className="mb-2 text-xl font-semibold text-gray-900">
            Konto erstellt
          </h1>
          <p className="mb-6 text-gray-600">
            Dein Operator-Konto wurde erfolgreich erstellt. Du kannst dich jetzt
            anmelden.
          </p>
          <Link
            href={operatorPath("/operator/login")}
            className="inline-block rounded-lg bg-gray-900 px-6 py-3 text-sm font-medium text-white transition-all hover:bg-gray-700"
          >
            Zur Anmeldung
          </Link>
        </div>
      </div>
    );
  }

  if (state === "error") {
    return (
      <div className="flex min-h-screen items-center justify-center p-4">
        <div className="w-full max-w-md rounded-2xl border border-gray-100 bg-white p-8 text-center shadow-lg">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-red-100">
            <svg
              className="h-8 w-8 text-red-600"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </div>
          <h1 className="mb-2 text-xl font-semibold text-gray-900">
            Einladung ungültig
          </h1>
          <p className="mb-6 text-gray-600">{errorMessage}</p>
          <Link
            href={operatorPath("/operator/login")}
            className="inline-block rounded-lg bg-gray-900 px-6 py-3 text-sm font-medium text-white transition-all hover:bg-gray-700"
          >
            Zur Anmeldung
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <div className="w-full max-w-lg rounded-2xl border border-gray-100 bg-white p-8 shadow-lg">
        <h1 className="mb-2 text-xl font-semibold text-gray-900">
          Operator-Konto erstellen
        </h1>
        {invitation && (
          <p className="mb-6 text-sm text-gray-500">
            Einladung für <strong>{invitation.email}</strong>
          </p>
        )}

        <form onSubmit={(e) => void handleSubmit(e)} className="space-y-5">
          <div>
            <label
              htmlFor="accept-display-name"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Anzeigename *
            </label>
            <input
              id="accept-display-name"
              type="text"
              required
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="Max Mustermann"
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-violet-500 focus:ring-1 focus:ring-violet-500 focus:outline-none"
            />
          </div>

          <div>
            <label
              htmlFor="accept-password"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Passwort *
            </label>
            <input
              id="accept-password"
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-violet-500 focus:ring-1 focus:ring-violet-500 focus:outline-none"
            />
            {password.length > 0 && (
              <ul className="mt-2 space-y-1">
                {PASSWORD_RULES.map((rule) => {
                  const met = rule.test(password);
                  return (
                    <li
                      key={rule.label}
                      className={`flex items-center gap-2 text-xs ${met ? "text-green-600" : "text-gray-400"}`}
                    >
                      <span>{met ? "\u2713" : "\u2717"}</span>
                      {rule.label}
                    </li>
                  );
                })}
              </ul>
            )}
          </div>

          <div>
            <label
              htmlFor="accept-confirm-password"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Passwort bestätigen *
            </label>
            <input
              id="accept-confirm-password"
              type="password"
              required
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-violet-500 focus:ring-1 focus:ring-violet-500 focus:outline-none"
            />
            {confirmPassword.length > 0 && !passwordsMatch && (
              <p className="mt-1 text-xs text-red-500">
                Passwörter stimmen nicht überein
              </p>
            )}
          </div>

          {formError && <p className="text-sm text-red-600">{formError}</p>}

          <button
            ref={primaryRef}
            type="submit"
            disabled={
              state === "submitting" ||
              !allPasswordRulesMet ||
              !passwordsMatch ||
              !displayName.trim()
            }
            className="w-full rounded-lg bg-violet-600 px-4 py-3 text-sm font-medium text-white transition-colors hover:bg-violet-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {state === "submitting" ? "Wird erstellt..." : "Konto erstellen"}
          </button>
        </form>
      </div>
    </div>
  );
}
