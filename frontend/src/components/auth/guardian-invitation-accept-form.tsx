"use client";

import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- redirects target tenant root, not a tenant route helper
import { useRouter } from "next/navigation";
import { Check, Circle } from "lucide-react";
import { useTranslations } from "next-intl";
import {
  authInputClassName,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { credentialError } from "~/components/auth/credential-error";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import {
  acceptGuardianInvitation,
  type GuardianInvitationValidation,
} from "~/lib/guardian-invitation-api";
import { PASSWORD_RULES } from "~/lib/password-rules";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GuardianInvitationAccept" });

interface Props {
  readonly token: string;
  readonly invitation: GuardianInvitationValidation;
}

/**
 * What the server page learned when the invitation could not be loaded. Only
 * serializable facts cross into the client; the text comes from the shared
 * error path in the reader's language (#2518).
 */
export interface GuardianInvitationLoadFailure {
  readonly status?: number;
  readonly code?: string;
  readonly requestId?: string;
}

/** The failed invitation load, shown where the form would be. */
export function GuardianInvitationLoadError({
  failure,
}: Readonly<{ failure: GuardianInvitationLoadFailure }>) {
  const t = useTranslations("guardianInvite");
  const { error, show } = useApiLoadError();
  const { status, code, requestId } = failure;

  useEffect(() => {
    void show(
      new ApiError("guardian invitation could not be loaded", status, {
        code,
        instance: requestId,
      }),
      {
        object: t("errorObject"),
        // The server page loads the invitation; a reload asks again.
        retry: () => globalThis.location.reload(),
      },
    );
  }, [code, requestId, show, status, t]);

  return <LoadErrorAlert error={error} />;
}

export function GuardianInvitationAcceptForm({ token, invitation }: Props) {
  const t = useTranslations("guardianInvite");
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  // Fehler über den gemeinsamen Weg (#2518): Katalogtext je Code im
  // Fehlerkasten, Prüfungen vor dem Senden markieren ihr Feld.
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isAccepted, setIsAccepted] = useState(false);

  const requirementStatus = useMemo(
    () =>
      PASSWORD_RULES.map(({ test }, index) => ({
        label: t(`passwordRules.${index}`),
        met: test(password),
      })),
    [password, t],
  );

  const allRequirementsMet = useMemo(
    () => requirementStatus.every((requirement) => requirement.met),
    [requirementStatus],
  );

  const guardianFullName = [invitation.firstName, invitation.lastName]
    .filter((value) => value && value.trim().length > 0)
    .join(" ")
    .trim();

  const submit = async () => {
    formErrors.clear();

    if (!allRequirementsMet) {
      // The sentence stands in the alert; the field is only marked.
      formErrors.invalid(t("formErrors.passwordRules"), { password: "" });
      return;
    }
    if (password !== confirmPassword) {
      formErrors.invalid(t("formErrors.passwordMismatch"), {
        confirmPassword: "",
      });
      return;
    }

    try {
      setIsSubmitting(true);
      await acceptGuardianInvitation(token, {
        password,
        confirmPassword,
      });

      // Redirect to the parents portal login. The accept invite page
      // is unauth, so there is no NextAuth session to clear here.
      let redirectUrl: string | null = null;
      if (globalThis.window !== undefined) {
        const parentsHostname = process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;
        if (parentsHostname) {
          const { protocol } = globalThis.window.location;
          redirectUrl = `${protocol}//${parentsHostname}/login`;
        }
      }

      setIsAccepted(true);
      setTimeout(() => {
        if (redirectUrl) {
          globalThis.window.location.href = redirectUrl;
          return;
        }
        router.push("/");
      }, 1500);
    } catch (err) {
      logger.error("guardian_invitation_accept_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // Vor der Anmeldung heißt 401 "abgelehnt", nicht "Sitzung abgelaufen".
      void formErrors.show(credentialError(err), {
        object: t("errorObject"),
        retry: () => latestSubmitRef.current(),
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    void submit();
  };

  // „Wiederholen“ sendet die Passwörter, die dann in den Feldern stehen.
  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
  });

  const passwordInvalid = formErrors.fieldError("password") !== undefined;
  const confirmInvalid =
    formErrors.fieldError("confirmPassword") !== undefined ||
    formErrors.fieldError("confirm_password") !== undefined;

  if (isAccepted) {
    return (
      <div className="flex flex-col items-center py-12">
        <div className="mb-6 flex h-12 w-12 items-center justify-center rounded-full bg-gray-900">
          <svg
            className="h-6 w-6 text-white"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.5}
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M5 13l4 4L19 7"
            />
          </svg>
        </div>
        <h3 className="mb-1 text-base font-semibold text-gray-900">
          {t("acceptedTitle")}
        </h3>
        <p className="mb-6 text-sm text-gray-500">{t("acceptedText")}</p>
        <div className="h-1 w-16 overflow-hidden rounded-full bg-gray-100">
          <div
            className="h-full rounded-full bg-gray-900"
            style={{
              animation: "guardianProgressFill 1.5s ease-in-out forwards",
            }}
          />
        </div>
        <style>{`
          @keyframes guardianProgressFill {
            from { width: 0%; }
            to { width: 100%; }
          }
        `}</style>
      </div>
    );
  }

  return (
    <form
      ref={formRef}
      onSubmit={handleSubmit}
      noValidate
      className="space-y-5 sm:space-y-6"
    >
      <FormErrorAlert message={formErrors.error} />

      <section className="rounded-lg border border-gray-200 bg-gray-50 px-3.5 py-3 text-sm">
        <p className="font-medium text-gray-700">
          {t("invitationFor")}{" "}
          <span className="font-semibold text-gray-900">
            {guardianFullName || invitation.email}
          </span>
        </p>
        {guardianFullName && (
          <p className="mt-1 text-gray-500">{invitation.email}</p>
        )}
      </section>

      <div>
        <label
          htmlFor="password"
          className={`mb-2 block text-sm font-medium ${passwordInvalid ? "text-moto-red-strong" : "text-gray-700"}`}
        >
          {t("passwordLabel")}
        </label>
        <div className="relative">
          <input
            id="password"
            name="password"
            type={showPassword ? "text" : "password"}
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            disabled={isSubmitting}
            autoComplete="new-password"
            aria-invalid={passwordInvalid || undefined}
            className={`${authInputClassName} pr-12 ${passwordInvalid ? "ring-moto-red/35 ring-2" : ""}`}
            required
          />
          <PasswordToggleButton
            showPassword={showPassword}
            onToggle={() => setShowPassword(!showPassword)}
            showLabel={t("showPassword")}
            hideLabel={t("hidePassword")}
          />
        </div>
      </div>

      <div>
        <label
          htmlFor="confirmPassword"
          className={`mb-2 block text-sm font-medium ${confirmInvalid ? "text-moto-red-strong" : "text-gray-700"}`}
        >
          {t("confirmPasswordLabel")}
        </label>
        <div className="relative">
          <input
            id="confirmPassword"
            name="confirmPassword"
            type={showConfirmPassword ? "text" : "password"}
            value={confirmPassword}
            onChange={(event) => setConfirmPassword(event.target.value)}
            disabled={isSubmitting}
            autoComplete="new-password"
            aria-invalid={confirmInvalid || undefined}
            className={`${authInputClassName} pr-12 ${confirmInvalid ? "ring-moto-red/35 ring-2" : ""}`}
            required
          />
          <PasswordToggleButton
            showPassword={showConfirmPassword}
            onToggle={() => setShowConfirmPassword(!showConfirmPassword)}
            showLabel={t("showPassword")}
            hideLabel={t("hidePassword")}
          />
        </div>
      </div>

      <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
        <p className="text-xs font-medium text-gray-700">
          {t("requirementsTitle")}
        </p>
        <div className="mt-3 grid gap-1.5 sm:grid-cols-2 sm:gap-2">
          {requirementStatus.map((requirement) => (
            <div
              key={requirement.label}
              className="flex items-center gap-2 text-sm"
            >
              <span
                className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-full border ${
                  requirement.met
                    ? "border-moto-green bg-moto-green/10 text-moto-green-strong"
                    : "border-gray-300 bg-white text-gray-400"
                }`}
                aria-hidden="true"
              >
                {requirement.met ? (
                  <Check className="h-3.5 w-3.5" />
                ) : (
                  <Circle className="h-3.5 w-3.5" />
                )}
              </span>
              <span
                className={requirement.met ? "text-gray-700" : "text-gray-500"}
              >
                {requirement.label}
              </span>
            </div>
          ))}
        </div>
      </div>

      <button
        type="submit"
        disabled={isSubmitting}
        className={authPrimaryButtonClassName}
      >
        {isSubmitting ? t("submitting") : t("submit")}
      </button>
    </form>
  );
}
