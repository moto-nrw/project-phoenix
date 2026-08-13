"use client";

import { useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- redirects target tenant root, not a tenant route helper
import { useRouter } from "next/navigation";
import { Check, Circle } from "lucide-react";
import { useTranslations } from "next-intl";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import {
  authInputClassName,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { Alert } from "~/components/ui/alert";
import {
  acceptGuardianInvitation,
  type GuardianInvitationValidation,
} from "~/lib/guardian-invitation-api";
import type { ApiError } from "~/lib/auth-api";
import { PASSWORD_RULES } from "~/lib/password-rules";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GuardianInvitationAccept" });

interface Props {
  readonly token: string;
  readonly invitation: GuardianInvitationValidation;
}

interface GuardianInvitationError {
  readonly message: string;
  readonly contactOgs: boolean;
}

const getErrorMessage = (
  apiError: ApiError | undefined,
  err: unknown,
  t: ReturnType<typeof useTranslations<"guardianInvite">>,
): string => {
  if (apiError?.status === 410) {
    return t("formErrors.expired");
  }
  if (apiError?.status === 404) {
    return t("formErrors.notFound");
  }
  if (apiError?.status === 409) {
    return t("formErrors.conflict");
  }
  if (apiError?.status === 400) {
    return apiError.message ?? t("formErrors.invalid");
  }
  return (
    apiError?.message ??
    (err instanceof Error ? err.message : undefined) ??
    t("formErrors.generic")
  );
};

export function GuardianInvitationAcceptForm({ token, invitation }: Props) {
  const t = useTranslations("guardianInvite");
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [error, setError] = useState<GuardianInvitationError | null>(null);
  const [errorFieldName, setErrorFieldName] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isAccepted, setIsAccepted] = useState(false);
  const errorRef = useScrollToError(error?.message ?? null);

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

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);
    setErrorFieldName(null);

    if (!allRequirementsMet) {
      setError({ message: t("formErrors.passwordRules"), contactOgs: false });
      setErrorFieldName("password");
      return;
    }
    if (password !== confirmPassword) {
      setError({
        message: t("formErrors.passwordMismatch"),
        contactOgs: false,
      });
      setErrorFieldName("confirmPassword");
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
      if (typeof navigator !== "undefined" && !navigator.onLine) {
        logger.warn("guardian_invitation_accept_offline", {
          error: "no_network_connection",
        });
        setError({ message: t("formErrors.offline"), contactOgs: false });
        setIsSubmitting(false);
        return;
      }
      logger.error("guardian_invitation_accept_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      const apiError = err as ApiError | undefined;
      setError({
        message: getErrorMessage(apiError, err, t),
        contactOgs: apiError?.status === 410,
      });
    } finally {
      setIsSubmitting(false);
    }
  };

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
    <form onSubmit={handleSubmit} noValidate className="space-y-5 sm:space-y-6">
      {error && (
        <div ref={errorRef}>
          <Alert
            type="error"
            message={
              error.contactOgs
                ? `${error.message} ${t("contactOgs")}`
                : error.message
            }
          />
        </div>
      )}

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
          className={`mb-2 block text-sm font-medium ${errorFieldName === "password" ? "text-moto-red-strong" : "text-gray-700"}`}
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
            className={`${authInputClassName} pr-12 ${errorFieldName === "password" ? "ring-moto-red/35 ring-2" : ""}`}
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
          className={`mb-2 block text-sm font-medium ${errorFieldName === "confirmPassword" ? "text-moto-red-strong" : "text-gray-700"}`}
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
            className={`${authInputClassName} pr-12 ${errorFieldName === "confirmPassword" ? "ring-moto-red/35 ring-2" : ""}`}
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
