"use client";

import { useState, useEffect, useLayoutEffect, useRef } from "react";
import { useSearchParams } from "next/navigation";
import Image from "next/image";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  AuthFieldError,
  AuthShell,
  authInputClass,
  authPrimaryButtonClassName,
  type AuthTestimonialPanelCopy,
} from "~/components/auth/auth-shell";
import { credentialError } from "~/components/auth/credential-error";
import { formErrorMessage } from "~/components/ui/form-error";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Loading } from "~/components/ui/loading";
import { useApiFormError } from "~/contexts/ToastContext";
import Link from "next/link";
import { CheckIcon, SpinnerIcon } from "~/components/ui/icons";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { confirmPasswordReset } from "~/lib/auth-api";
import { errorStatus } from "~/lib/expected-failure";
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
  /**
   * The noun phrase the shared error path puts into its catalog text, for
   * example "das Zurücksetzen des Passworts" (#2517).
   */
  readonly errorObject?: string;
  readonly passwordTooShort: string;
  readonly passwordMissingUppercase: string;
  readonly passwordMissingLowercase: string;
  readonly passwordMissingNumber: string;
  readonly passwordMissingSpecial: string;
  readonly passwordMismatch: string;
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

const DEFAULT_ERROR_OBJECT = "das Zurücksetzen des Passworts";

const DEFAULT_RESET_PASSWORD_PAGE_COPY: ResetPasswordPageCopy = {
  missingToken:
    "Der Link ist unvollständig. Bitte fordern Sie einen neuen Link an.",
  passwordTooShort: "Das Passwort muss mindestens 8 Zeichen lang sein.",
  passwordMissingUppercase:
    "Das Passwort muss mindestens einen Großbuchstaben enthalten.",
  passwordMissingLowercase:
    "Das Passwort muss mindestens einen Kleinbuchstaben enthalten.",
  passwordMissingNumber: "Das Passwort muss mindestens eine Zahl enthalten.",
  passwordMissingSpecial:
    "Das Passwort muss mindestens ein Sonderzeichen enthalten.",
  passwordMismatch: "Die Passwörter stimmen nicht überein.",
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
  /** Field name of the backend, so a server field error marks it. */
  readonly name: string;
  /** Marks the field; an empty string marks it without a caption. */
  readonly error: string | undefined;
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
  name,
  error,
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
          name={name}
          type={visible ? "text" : "password"}
          autoComplete="new-password"
          required
          value={value}
          onChange={(e) => onChange(e.target.value)}
          aria-invalid={error !== undefined}
          aria-describedby={error ? `${id}-error` : undefined}
          className={`${authInputClass(error !== undefined)} pr-10`}
          disabled={disabled}
        />
        <PasswordToggleButton
          showPassword={visible}
          onToggle={onToggleVisible}
          showLabel={showPasswordLabel}
          hideLabel={hidePasswordLabel}
        />
      </div>
      <AuthFieldError id={`${id}-error`} message={error} />
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
  // Fehler über den gemeinsamen Weg (#2517): Katalogtext je Code, Feldfehler
  // am Feld. Vor der Anmeldung heißt 401 "abgelehnt", nicht "abgelaufen".
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { invalid: showInvalid } = formErrors;
  const latestSubmitRef = useRef<() => void>(() => undefined);
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

  // Keyed by the token value: invalid() sets state, so an effect keyed by
  // the searchParams object would run again on every render.
  const tokenParam = searchParams.get("token");
  useEffect(() => {
    if (tokenParam) {
      setToken(tokenParam);
    } else {
      showInvalid(copy.missingToken);
    }
  }, [copy.missingToken, tokenParam, showInvalid]);

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

  const submit = async () => {
    if (!token) {
      formErrors.invalid(copy.missingToken);
      return;
    }

    const passwordError = validatePassword(password);
    if (passwordError) {
      formErrors.invalid(passwordError, { new_password: passwordError });
      return;
    }

    if (password !== confirmPassword) {
      formErrors.invalid(copy.passwordMismatch, {
        confirm_password: copy.passwordMismatch,
      });
      return;
    }

    formErrors.clear();
    setIsLoading(true);

    try {
      await confirmReset(token, password, confirmPassword);
      setIsSuccess(true);

      setTimeout(() => {
        router.push(successRedirectPath);
      }, 3000);
    } catch (err) {
      // Ein abgelaufener Link oder ein zu schwaches Passwort ist kein Defekt.
      const status = errorStatus(err);
      const context = {
        error: err instanceof Error ? err.message : String(err),
        status,
      };
      if (status !== undefined && status >= 400 && status < 500) {
        logger.warn("password_reset_failed", context);
      } else {
        logger.error("password_reset_failed", context);
      }
      void formErrors.show(credentialError(err), {
        object: copy.errorObject ?? DEFAULT_ERROR_OBJECT,
        retry: () => latestSubmitRef.current(),
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    void submit();
  };

  // A check names one field; its sentence already stands in the alert, so
  // the field is only marked, not captioned twice.
  const fieldHint = (name: string): string | undefined => {
    const hint = formErrors.fieldError(name);
    return hint !== undefined && hint === formErrorMessage(formErrors.error)
      ? ""
      : hint;
  };

  // „Wiederholen“ sendet den Stand, der dann im Formular steht.
  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
  });

  if (isSuccess) {
    return (
      <AuthShell
        eyebrow={copy.successEyebrow}
        eyebrowClassName="text-moto-green"
        title={copy.successTitle}
        subtitle={copy.successSubtitle}
        variant="reset"
        brand={brand}
        testimonialPanelCopy={testimonialPanelCopy}
      >
        <div className="text-center">
          <div className="bg-moto-green-soft mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full">
            <CheckIcon className="text-moto-green-strong h-10 w-10" />
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
      eyebrowClassName="text-moto-green"
      title={copy.formTitle}
      subtitle={copy.formSubtitle}
      variant="reset"
      brand={brand}
      testimonialPanelCopy={testimonialPanelCopy}
    >
      <form
        ref={formRef}
        onSubmit={handleSubmit}
        noValidate
        className="space-y-4"
      >
        <FormErrorAlert message={formErrors.error} />

        <div className="space-y-4">
          <PasswordField
            id="password"
            name="new_password"
            error={fieldHint("new_password")}
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
            name="confirm_password"
            error={fieldHint("confirm_password")}
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
