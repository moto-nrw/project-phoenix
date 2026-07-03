"use client";

import { useState, useEffect } from "react";
import { useSearchParams } from "next/navigation";
import Image from "next/image";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  AuthShell,
  authInputClassName,
  authPrimaryButtonClassName,
  type AuthTestimonialPanelCopy,
} from "~/components/auth/auth-shell";
import { Loading } from "~/components/ui/loading";
import Link from "next/link";
import { CheckIcon, SpinnerIcon } from "~/components/ui/icons";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { confirmPasswordReset, type ApiError } from "~/lib/auth-api";
import { createLogger } from "~/lib/logger";
import { useTenantSafe } from "~/lib/tenant-context";
import { loginImageSrc } from "~/lib/tenant-api";

const logger = createLogger({ component: "ResetPasswordPage" });

type ConfirmPasswordReset = (
  token: string,
  password: string,
  confirmPassword: string,
) => Promise<{ message: string }>;

interface ResetPasswordPageContentProps {
  readonly confirmReset?: ConfirmPasswordReset;
  readonly successRedirectPath?: string;
  readonly backHref?: string;
  readonly backLabel?: string;
  readonly copy?: ResetPasswordPageCopy;
  // Localized testimonial-panel copy. The parents portal passes its
  // translated copy here so the reset page doesn't fall back to the
  // German staff/moto testimonials next to a localized form.
  readonly testimonialPanelCopy?: AuthTestimonialPanelCopy;
}

export interface ResetPasswordPageCopy {
  readonly missingToken: string;
  readonly invalidToken: string;
  readonly passwordTooShort: string;
  readonly passwordMissingUppercase: string;
  readonly passwordMissingLowercase: string;
  readonly passwordMissingNumber: string;
  readonly passwordMissingSpecial: string;
  readonly passwordMismatch: string;
  readonly genericError: string;
  readonly invalidRequest: string;
  readonly expiredLink: string;
  readonly notFoundLink: string;
  readonly successEyebrow: string;
  readonly successTitle: string;
  readonly successSubtitle: string;
  readonly successBody: string;
  readonly formEyebrow: string;
  readonly formTitle: string;
  readonly formSubtitle: string;
  readonly passwordLabel: string;
  readonly confirmPasswordLabel: string;
  readonly showPassword: string;
  readonly hidePassword: string;
  readonly requirementsTitle: string;
  readonly requirements: readonly string[];
  readonly submitting: string;
  readonly submit: string;
}

const DEFAULT_RESET_PASSWORD_PAGE_COPY: ResetPasswordPageCopy = {
  missingToken:
    "Ungültiger oder fehlender Reset-Token. Bitte fordern Sie einen neuen Link an.",
  invalidToken: "Ungültiger Reset-Token.",
  passwordTooShort: "Das Passwort muss mindestens 8 Zeichen lang sein.",
  passwordMissingUppercase:
    "Das Passwort muss mindestens einen Großbuchstaben enthalten.",
  passwordMissingLowercase:
    "Das Passwort muss mindestens einen Kleinbuchstaben enthalten.",
  passwordMissingNumber: "Das Passwort muss mindestens eine Zahl enthalten.",
  passwordMissingSpecial:
    "Das Passwort muss mindestens ein Sonderzeichen enthalten.",
  passwordMismatch: "Die Passwörter stimmen nicht überein.",
  genericError: "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
  invalidRequest:
    "Bitte prüfen Sie den Link und die Passwort-Anforderungen und versuchen Sie es erneut.",
  expiredLink:
    "Dieser Passwort-Reset-Link ist abgelaufen. Bitte fordere einen neuen Link an.",
  notFoundLink:
    "Wir konnten diesen Passwort-Reset-Link nicht finden. Bitte fordere einen neuen Link an.",
  successEyebrow: "Passwort geändert",
  successTitle: "Passwort erfolgreich geändert",
  successSubtitle: "Sie werden automatisch zur Anmeldeseite weitergeleitet.",
  successBody: "Die Anmeldung ist gleich wieder möglich.",
  formEyebrow: "Passwort zurücksetzen",
  formTitle: "Neues Passwort festlegen",
  formSubtitle: "Wählen Sie ein starkes Passwort für Ihr Konto.",
  passwordLabel: "Neues Passwort",
  confirmPasswordLabel: "Passwort bestätigen",
  showPassword: "Passwort anzeigen",
  hidePassword: "Passwort verbergen",
  requirementsTitle: "Passwort-Anforderungen:",
  requirements: [
    "Mindestens 8 Zeichen lang",
    "Groß- und Kleinbuchstaben",
    "Mindestens eine Zahl",
    "Mindestens ein Sonderzeichen",
  ],
  submitting: "Wird gespeichert...",
  submit: "Passwort ändern",
};

interface PasswordFieldProps {
  readonly id: string;
  readonly label: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly visible: boolean;
  readonly onToggleVisible: () => void;
  readonly disabled: boolean;
  readonly showPasswordLabel: string;
  readonly hidePasswordLabel: string;
}

function PasswordField({
  id,
  label,
  value,
  onChange,
  visible,
  onToggleVisible,
  disabled,
  showPasswordLabel,
  hidePasswordLabel,
}: PasswordFieldProps) {
  return (
    <div className="text-left">
      <label
        htmlFor={id}
        className="mb-1 block text-sm font-medium text-gray-700"
      >
        {label}
      </label>
      <div className="relative">
        <input
          id={id}
          name={id}
          type={visible ? "text" : "password"}
          autoComplete="new-password"
          required
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className={`${authInputClassName} pr-10`}
          disabled={disabled}
        />
        <PasswordToggleButton
          showPassword={visible}
          onToggle={onToggleVisible}
          showLabel={showPasswordLabel}
          hideLabel={hidePasswordLabel}
        />
      </div>
    </div>
  );
}

export function ResetPasswordPageContent({
  confirmReset = confirmPasswordReset,
  successRedirectPath = "/",
  backHref = "/",
  backLabel = "Zurück zur Anmeldung",
  copy = DEFAULT_RESET_PASSWORD_PAGE_COPY,
  testimonialPanelCopy,
}: ResetPasswordPageContentProps = {}) {
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isSuccess, setIsSuccess] = useState(false);
  const [token, setToken] = useState<string | null>(null);
  const router = useTenantRouter();
  const searchParams = useSearchParams();
  const tenantContext = useTenantSafe();
  const tenant = tenantContext?.tenant;
  const brand = tenant?.settings?.loginImageUrl ? (
    <Image
      src={loginImageSrc(tenant.settings.loginImageUrl)}
      alt={`${tenant.name} Logo`}
      width={180}
      height={104}
      className="max-h-[104px] w-auto object-contain"
      priority
      unoptimized
    />
  ) : null;

  useEffect(() => {
    const tokenParam = searchParams.get("token");
    if (tokenParam) {
      setToken(tokenParam);
    } else {
      setError(copy.missingToken);
    }
  }, [copy.missingToken, searchParams]);

  const validatePassword = (pwd: string): string | null => {
    if (pwd.length < 8) {
      return copy.passwordTooShort;
    }
    if (!/[A-Z]/.test(pwd)) {
      return copy.passwordMissingUppercase;
    }
    if (!/[a-z]/.test(pwd)) {
      return copy.passwordMissingLowercase;
    }
    if (!/\d/.test(pwd)) {
      return copy.passwordMissingNumber;
    }
    if (!/[^A-Za-z0-9]/.test(pwd)) {
      return copy.passwordMissingSpecial;
    }
    return null;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    if (!token) {
      setError(copy.invalidToken);
      return;
    }

    const passwordError = validatePassword(password);
    if (passwordError) {
      setError(passwordError);
      return;
    }

    if (password !== confirmPassword) {
      setError(copy.passwordMismatch);
      return;
    }

    setIsLoading(true);

    try {
      await confirmReset(token, password, confirmPassword);
      setIsSuccess(true);

      setTimeout(() => {
        router.push(successRedirectPath);
      }, 3000);
    } catch (err) {
      const apiError = err as ApiError | undefined;
      const status = apiError?.status;
      if (status === 410 || status === 404) {
        logger.warn("password_reset_failed", {
          error: err instanceof Error ? err.message : String(err),
          status,
        });
      } else {
        logger.error("password_reset_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      }
      let message = copy.genericError;

      if (apiError?.status === 410) {
        message = copy.expiredLink;
      } else if (apiError?.status === 404) {
        message = copy.notFoundLink;
      } else if (apiError?.status === 400) {
        message = copy.invalidRequest;
      }

      setError(message);
    } finally {
      setIsLoading(false);
    }
  };

  if (isSuccess) {
    return (
      <AuthShell
        eyebrow={copy.successEyebrow}
        eyebrowClassName="text-[#83CD2D]"
        title={copy.successTitle}
        subtitle={copy.successSubtitle}
        variant="reset"
        brand={brand}
        testimonialPanelCopy={testimonialPanelCopy}
      >
        <div className="text-center">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-[#EAF6D8]">
            <CheckIcon className="h-10 w-10 text-[#4E7D1B]" />
          </div>

          <p className="mb-6 text-sm text-gray-600">{copy.successBody}</p>

          <Loading fullPage={false} />
        </div>
      </AuthShell>
    );
  }

  return (
    <AuthShell
      eyebrow={copy.formEyebrow}
      eyebrowClassName="text-[#83CD2D]"
      title={copy.formTitle}
      subtitle={copy.formSubtitle}
      variant="reset"
      brand={brand}
      testimonialPanelCopy={testimonialPanelCopy}
    >
      <form onSubmit={handleSubmit} noValidate className="space-y-4">
        {error && (
          <div className="rounded-xl border border-red-200 bg-red-50 p-4">
            <div className="flex items-start gap-3">
              <svg
                className="mt-0.5 h-5 w-5 flex-shrink-0 text-red-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
              <p className="text-sm text-red-700">{error}</p>
            </div>
          </div>
        )}

        <div className="space-y-4">
          <PasswordField
            id="password"
            label={copy.passwordLabel}
            value={password}
            onChange={setPassword}
            visible={showPassword}
            onToggleVisible={() => setShowPassword(!showPassword)}
            disabled={isLoading || !token}
            showPasswordLabel={copy.showPassword}
            hidePasswordLabel={copy.hidePassword}
          />
          <PasswordField
            id="confirmPassword"
            label={copy.confirmPasswordLabel}
            value={confirmPassword}
            onChange={setConfirmPassword}
            visible={showConfirmPassword}
            onToggleVisible={() => setShowConfirmPassword(!showConfirmPassword)}
            disabled={isLoading || !token}
            showPasswordLabel={copy.showPassword}
            hidePasswordLabel={copy.hidePassword}
          />
        </div>

        <div className="rounded-lg border border-gray-200 bg-gray-50 p-3 text-left">
          <p className="mb-1.5 text-xs font-medium text-gray-700">
            {copy.requirementsTitle}
          </p>
          <ul className="space-y-0.5 text-xs text-gray-600">
            {copy.requirements.map((requirement) => (
              <li key={requirement}>{requirement}</li>
            ))}
          </ul>
        </div>

        <button
          type="submit"
          disabled={isLoading || !token}
          className={authPrimaryButtonClassName}
        >
          {isLoading ? (
            <>
              <SpinnerIcon className="mr-2 -ml-1 h-4 w-4 text-white" />
              <span>{copy.submitting}</span>
            </>
          ) : (
            <span>{copy.submit}</span>
          )}
        </button>

        <div className="pt-2 text-center">
          <Link
            href={backHref}
            className="text-sm text-gray-600 transition-colors hover:text-gray-800 hover:underline"
          >
            {backLabel}
          </Link>
        </div>
      </form>
    </AuthShell>
  );
}
