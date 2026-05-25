"use client";

import { useState, useEffect } from "react";
import { useSearchParams } from "next/navigation";
import Image from "next/image";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  AuthShell,
  authInputClassName,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { Loading } from "~/components/ui/loading";
import Link from "next/link";
import { CheckIcon, SpinnerIcon } from "~/components/ui/icons";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { confirmPasswordReset, type ApiError } from "~/lib/auth-api";
import { createLogger } from "~/lib/logger";
import { useTenantSafe } from "~/components/tenant/tenant-provider";
import { loginImageSrc } from "~/lib/tenant-api";

const logger = createLogger({ component: "ResetPasswordPage" });

interface PasswordFieldProps {
  readonly id: string;
  readonly label: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly visible: boolean;
  readonly onToggleVisible: () => void;
  readonly disabled: boolean;
}

function PasswordField({
  id,
  label,
  value,
  onChange,
  visible,
  onToggleVisible,
  disabled,
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
        />
      </div>
    </div>
  );
}

export function ResetPasswordPageContent() {
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
      setError(
        "Ungültiger oder fehlender Reset-Token. Bitte fordern Sie einen neuen Link an.",
      );
    }
  }, [searchParams]);

  const validatePassword = (pwd: string): string | null => {
    if (pwd.length < 8) {
      return "Das Passwort muss mindestens 8 Zeichen lang sein.";
    }
    if (!/[A-Z]/.test(pwd)) {
      return "Das Passwort muss mindestens einen Großbuchstaben enthalten.";
    }
    if (!/[a-z]/.test(pwd)) {
      return "Das Passwort muss mindestens einen Kleinbuchstaben enthalten.";
    }
    if (!/\d/.test(pwd)) {
      return "Das Passwort muss mindestens eine Zahl enthalten.";
    }
    if (!/[^A-Za-z0-9]/.test(pwd)) {
      return "Das Passwort muss mindestens ein Sonderzeichen enthalten.";
    }
    return null;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    if (!token) {
      setError("Ungültiger Reset-Token.");
      return;
    }

    const passwordError = validatePassword(password);
    if (passwordError) {
      setError(passwordError);
      return;
    }

    if (password !== confirmPassword) {
      setError("Die Passwörter stimmen nicht überein.");
      return;
    }

    setIsLoading(true);

    try {
      await confirmPasswordReset(token, password, confirmPassword);
      setIsSuccess(true);

      setTimeout(() => {
        router.push("/");
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
      let message =
        apiError?.message ??
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.";

      if (apiError?.status === 410) {
        message =
          "Dieser Passwort-Reset-Link ist abgelaufen. Bitte fordere einen neuen Link an.";
      } else if (apiError?.status === 404) {
        message =
          "Wir konnten diesen Passwort-Reset-Link nicht finden. Bitte fordere einen neuen Link an.";
      } else if (apiError?.status === 400 && apiError.message) {
        message = apiError.message;
      }

      setError(message);
    } finally {
      setIsLoading(false);
    }
  };

  if (isSuccess) {
    return (
      <AuthShell
        eyebrow="Passwort geändert"
        eyebrowClassName="text-[#83CD2D]"
        title="Passwort erfolgreich geändert"
        subtitle="Sie werden automatisch zur Anmeldeseite weitergeleitet."
        variant="reset"
        brand={brand}
      >
        <div className="text-center">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-[#EAF6D8]">
            <CheckIcon className="h-10 w-10 text-[#4E7D1B]" />
          </div>

          <p className="mb-6 text-sm text-gray-600">
            Die Anmeldung ist gleich wieder möglich.
          </p>

          <Loading fullPage={false} />
        </div>
      </AuthShell>
    );
  }

  return (
    <AuthShell
      eyebrow="Passwort zurücksetzen"
      eyebrowClassName="text-[#83CD2D]"
      title="Neues Passwort festlegen"
      subtitle="Wählen Sie ein starkes Passwort für Ihr Konto."
      variant="reset"
      brand={brand}
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
            label="Neues Passwort"
            value={password}
            onChange={setPassword}
            visible={showPassword}
            onToggleVisible={() => setShowPassword(!showPassword)}
            disabled={isLoading || !token}
          />
          <PasswordField
            id="confirmPassword"
            label="Passwort bestätigen"
            value={confirmPassword}
            onChange={setConfirmPassword}
            visible={showConfirmPassword}
            onToggleVisible={() => setShowConfirmPassword(!showConfirmPassword)}
            disabled={isLoading || !token}
          />
        </div>

        <div className="rounded-lg border border-gray-200 bg-gray-50 p-3 text-left">
          <p className="mb-1.5 text-xs font-medium text-gray-700">
            Passwort-Anforderungen:
          </p>
          <ul className="space-y-0.5 text-xs text-gray-600">
            <li>Mindestens 8 Zeichen lang</li>
            <li>Groß- und Kleinbuchstaben</li>
            <li>Mindestens eine Zahl</li>
            <li>Mindestens ein Sonderzeichen</li>
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
              <span>Wird gespeichert...</span>
            </>
          ) : (
            <span>Passwort ändern</span>
          )}
        </button>

        <div className="pt-2 text-center">
          <Link
            href="/"
            className="text-sm text-gray-600 transition-colors hover:text-gray-800 hover:underline"
          >
            Zurück zur Anmeldung
          </Link>
        </div>
      </form>
    </AuthShell>
  );
}
