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
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
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
  const invitationLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = invitationLoad;

  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const {
    show: showFormError,
    invalid: invalidForm,
    clear: clearFormError,
  } = formErrors;

  const primaryRef = useRef<HTMLButtonElement>(null);
  const processingRef = useRef(false);

  const processToken = useCallback(async () => {
    if (processingRef.current) return;
    processingRef.current = true;
    setState("loading");
    clearLoadError();
    clearFormError();
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
      if (!nextFlowID) {
        // Without token and flow there is no invitation to look up.
        throw new ApiError("Missing invitation flow", 404, {
          code: "identity.invitation_not_found",
        });
      }

      const data = await validateOperatorInvitation(nextFlowID);
      setFlowID(nextFlowID);
      setInvitation(data);
      setDisplayName(data.displayName ?? "");
      setState("form");
    } catch (err) {
      logger.warn("invitation_validation_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showLoadError(err, {
        object: "die Einladung",
        retry: () => {
          processingRef.current = false;
          void processToken();
        },
      });
      setState("error");
    }
  }, [showLoadError, clearLoadError, clearFormError]);

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

      clearFormError();

      if (!displayName.trim()) {
        invalidForm("Bitte geben Sie einen Anzeigenamen ein.", {
          display_name: "Bitte geben Sie einen Anzeigenamen ein.",
        });
        return;
      }
      if (!allPasswordRulesMet) {
        invalidForm("Das Passwort erfüllt noch nicht alle Anforderungen.", {
          password: "Das Passwort erfüllt noch nicht alle Anforderungen.",
        });
        return;
      }
      if (!passwordsMatch) {
        invalidForm("Die Passwörter stimmen nicht überein.", {
          confirm_password: "Die Passwörter stimmen nicht überein.",
        });
        return;
      }

      setState("submitting");

      try {
        if (!flowID) {
          throw new ApiError("Missing invitation flow", 404, {
            code: "identity.invitation_not_found",
          });
        }
        await acceptOperatorInvitation(flowID, {
          displayName: displayName.trim(),
          password,
          confirmPassword,
        });
        window.history.replaceState({}, "", window.location.pathname);
        setState("success");
      } catch (err) {
        logger.warn("invitation_accept_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setState("form");
        void showFormError(err, { object: "die Erstellung des Kontos" });
      }
    },
    [
      showFormError,
      invalidForm,
      clearFormError,
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
        title="Einladung nicht geöffnet"
        subtitle="Die Einladung konnte nicht geöffnet werden."
        variant="operator"
        brand={<OperatorBrand />}
        formMaxWidth="max-w-[31rem]"
      >
        <div className="text-center">
          <div className="bg-moto-red-soft mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full">
            <span className="text-moto-red text-2xl font-semibold">!</span>
          </div>
          <LoadErrorAlert
            error={invitationLoad.error}
            className="mb-4 text-left"
          />
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

      <form
        ref={formRef}
        onSubmit={(e) => void handleSubmit(e)}
        noValidate
        className="space-y-5"
      >
        <FormErrorAlert message={formErrors.error} />
        <div>
          <label
            htmlFor="accept-display-name"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Anzeigename *
          </label>
          <input
            id="accept-display-name"
            name="display_name"
            aria-invalid={
              formErrors.fieldError("display_name") ? true : undefined
            }
            aria-describedby={
              formErrors.fieldError("display_name")
                ? "accept-display-name-error"
                : undefined
            }
            type="text"
            required
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="Max Mustermann"
            className={authInputClassName}
          />
          <FieldErrorText
            id="accept-display-name-error"
            message={formErrors.fieldError("display_name")}
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
              name="password"
              aria-invalid={
                formErrors.fieldError("password") ? true : undefined
              }
              aria-describedby={
                formErrors.fieldError("password")
                  ? "accept-password-error"
                  : undefined
              }
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
          <FieldErrorText
            id="accept-password-error"
            message={formErrors.fieldError("password")}
          />
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
              name="confirm_password"
              aria-invalid={
                formErrors.fieldError("confirm_password") ? true : undefined
              }
              aria-describedby={
                formErrors.fieldError("confirm_password")
                  ? "accept-confirm-password-error"
                  : undefined
              }
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
          {formErrors.fieldError("confirm_password") ? (
            <FieldErrorText
              id="accept-confirm-password-error"
              message={formErrors.fieldError("confirm_password")}
            />
          ) : confirmPassword.length > 0 && !passwordsMatch ? (
            <p className="text-moto-red mt-1 text-xs">
              Passwörter stimmen nicht überein
            </p>
          ) : null}
        </div>

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

function FieldErrorText({
  id,
  message,
}: {
  readonly id: string;
  readonly message: string | undefined;
}) {
  if (!message) return null;
  return (
    <p id={id} className="text-moto-red mt-1 text-xs">
      {message}
    </p>
  );
}
