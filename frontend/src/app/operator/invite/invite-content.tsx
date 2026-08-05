"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { Check, Circle } from "lucide-react";
import {
  AuthShell,
  OperatorBrand,
  authInputClassName,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { Loading } from "~/components/ui/loading";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { operatorPath } from "~/lib/operator-url";
import {
  establishOperatorInvitationSession,
  validateOperatorInvitation,
  acceptOperatorInvitation,
} from "~/lib/operator/operator-invitation-api";
import type { OperatorInvitationValidation } from "~/lib/operator/operator-invitation-helpers";
import { PASSWORD_RULES } from "~/lib/password-rules";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorInviteAcceptPage" });

type PageState = "loading" | "form" | "submitting" | "success" | "error";

function extractQueryToken(): string | null {
  if (typeof window === "undefined") return null;
  return new URLSearchParams(window.location.search).get("token");
}

function extractQueryFlowID(): string | null {
  if (typeof window === "undefined") return null;
  return new URLSearchParams(window.location.search).get("flow");
}

export function InviteContent() {
  const [invitation, setInvitation] =
    useState<OperatorInvitationValidation | null>(null);
  const [flowID, setFlowID] = useState<string | null>(null);
  const [state, setState] = useState<PageState>("loading");
  const [errorMessage, setErrorMessage] = useState("");

  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const primaryRef = useRef<HTMLButtonElement>(null);
  const processingRef = useRef(false);

  const processToken = useCallback(async () => {
    if (processingRef.current) return;
    processingRef.current = true;
    setState("loading");
    setFormError(null);
    setPassword("");
    setConfirmPassword("");

    try {
      const queryToken = extractQueryToken();
      let nextFlowID = extractQueryFlowID();
      if (queryToken) {
        nextFlowID = await establishOperatorInvitationSession(queryToken);
        const safeURL = new URL(window.location.href);
        safeURL.search = "";
        safeURL.searchParams.set("flow", nextFlowID);
        window.history.replaceState(
          {},
          "",
          `${safeURL.pathname}${safeURL.search}`,
        );
      }
      if (!nextFlowID) throw new Error("Kein Einladungsvorgang angegeben.");

      const data = await validateOperatorInvitation(nextFlowID);
      setFlowID(nextFlowID);
      setInvitation(data);
      setDisplayName(data.displayName ?? "");
      setState("form");
    } catch (err) {
      logger.error("invitation_validation_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setErrorMessage(
        err instanceof Error
          ? err.message
          : "Einladung nicht gefunden oder abgelaufen.",
      );
      setState("error");
    }
  }, []);

  // Process token on mount. Query-string changes trigger a full navigation
  // (unlike fragment changes), so no event listener is needed — the component
  // remounts and this effect re-fires when the user clicks a fresh invite link.
  useEffect(() => {
    processToken();
  }, [processToken]);

  const allPasswordRulesMet = PASSWORD_RULES.every((rule) =>
    rule.test(password),
  );
  const passwordsMatch = password === confirmPassword && password.length > 0;

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (state === "submitting") return;

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
        if (!flowID) throw new Error("Kein Einladungsvorgang angegeben.");
        await acceptOperatorInvitation(flowID, {
          displayName: displayName.trim(),
          password,
          confirmPassword,
        });
        window.history.replaceState({}, "", window.location.pathname);
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
      state,
      displayName,
      password,
      confirmPassword,
      allPasswordRulesMet,
      passwordsMatch,
      flowID,
    ],
  );

  if (state === "loading") {
    return (
      <AuthShell
        eyebrow="Operator"
        title="Einladung prüfen"
        subtitle="Wir laden die Einladung."
        variant="operator"
        brand={<OperatorBrand />}
        formMaxWidth="max-w-[31rem]"
      >
        <Loading fullPage={false} />
      </AuthShell>
    );
  }

  if (state === "success") {
    return (
      <AuthShell
        eyebrow="Operator"
        title="Konto erstellt"
        subtitle="Dein Operator-Konto wurde erfolgreich erstellt."
        variant="operator"
        brand={<OperatorBrand />}
        formMaxWidth="max-w-[31rem]"
      >
        <div className="text-center">
          <div className="bg-moto-green-soft mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full">
            <Check
              className="text-moto-green-strong h-9 w-9"
              aria-hidden="true"
            />
          </div>
          <Link
            href={operatorPath("/operator/login")}
            className={authPrimaryButtonClassName}
          >
            Zur Anmeldung
          </Link>
        </div>
      </AuthShell>
    );
  }

  if (state === "error") {
    return (
      <AuthShell
        eyebrow="Operator"
        title="Einladung ungültig"
        subtitle={errorMessage}
        variant="operator"
        brand={<OperatorBrand />}
        formMaxWidth="max-w-[31rem]"
      >
        <div className="text-center">
          <div className="bg-moto-red-soft mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full">
            <span className="text-moto-red text-2xl font-semibold">!</span>
          </div>
          <Link
            href={operatorPath("/operator/login")}
            className={authPrimaryButtonClassName}
          >
            Zur Anmeldung
          </Link>
        </div>
      </AuthShell>
    );
  }

  return (
    <AuthShell
      eyebrow="Operator"
      title="Operator-Konto erstellen"
      subtitle="Lege dein Operator-Konto an und melde dich danach im Operator-Portal an."
      variant="operator"
      brand={<OperatorBrand />}
      formMaxWidth="max-w-[32rem]"
    >
      {invitation && (
        <div className="mb-5 rounded-lg border border-gray-200 bg-gray-50 px-3.5 py-3 text-sm text-gray-600">
          Einladung für{" "}
          <strong className="text-gray-900">{invitation.email}</strong>
        </div>
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
            className={authInputClassName}
          />
        </div>

        <div>
          <label
            htmlFor="accept-password"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Passwort *
          </label>
          <div className="relative">
            <input
              id="accept-password"
              type={showPassword ? "text" : "password"}
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className={`${authInputClassName} pr-10`}
            />
            <PasswordToggleButton
              showPassword={showPassword}
              onToggle={() => setShowPassword(!showPassword)}
            />
          </div>
          {password.length > 0 && (
            <ul className="mt-2 grid grid-cols-2 gap-x-3 gap-y-1.5">
              {PASSWORD_RULES.map((rule) => {
                const met = rule.test(password);
                return (
                  <li
                    key={rule.label}
                    className="flex items-center gap-1.5 text-xs"
                  >
                    <span
                      className={`flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full border ${
                        met
                          ? "border-moto-green bg-moto-green/10 text-moto-green-strong"
                          : "border-gray-300 bg-white text-gray-400"
                      }`}
                      aria-hidden="true"
                    >
                      {met ? (
                        <Check className="h-3 w-3" />
                      ) : (
                        <Circle className="h-3 w-3" />
                      )}
                    </span>
                    <span className={met ? "text-gray-700" : "text-gray-500"}>
                      {rule.label}
                    </span>
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
          <div className="relative">
            <input
              id="accept-confirm-password"
              type={showConfirmPassword ? "text" : "password"}
              required
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              className={`${authInputClassName} pr-10`}
            />
            <PasswordToggleButton
              showPassword={showConfirmPassword}
              onToggle={() => setShowConfirmPassword(!showConfirmPassword)}
            />
          </div>
          {confirmPassword.length > 0 && !passwordsMatch && (
            <p className="mt-1 text-xs text-red-500">
              Passwörter stimmen nicht überein
            </p>
          )}
        </div>

        {formError && (
          <div className="rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            {formError}
          </div>
        )}

        <button
          ref={primaryRef}
          type="submit"
          disabled={
            state === "submitting" ||
            !allPasswordRulesMet ||
            !passwordsMatch ||
            !displayName.trim()
          }
          className={authPrimaryButtonClassName}
        >
          {state === "submitting" ? "Wird erstellt..." : "Konto erstellen"}
        </button>
      </form>
    </AuthShell>
  );
}
