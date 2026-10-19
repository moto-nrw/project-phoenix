"use client";

// The page the school's mailed invitation opens (ADR 0074). The link names
// the school and carries the proof; nobody but the caterer has it. A new
// caterer sets its password, a caterer with an account confirms with its
// password and adds the school. Afterwards the caterer is signed in.

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { useParams } from "next/navigation";
import { signIn } from "next-auth/react";
import { Check, Circle } from "lucide-react";
import {
  AuthFieldError,
  AuthShell,
  AuthShellSkeleton,
  MotoBrand,
  authInputClass,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import {
  credentialError,
  LOGIN_REFUSALS,
} from "~/components/auth/credential-error";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import { Alert } from "~/components/ui/alert";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import {
  acceptCatererInvitation,
  fetchCatererInvitation,
  loginCaterer,
  type CatererInvitationPreview,
} from "~/lib/caterer-api";
import { catererPath } from "~/lib/caterer-url";
import { PASSWORD_RULES } from "~/lib/password-rules";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CatererInvitationPage" });

function routeParam(value: string | string[] | undefined): string {
  return Array.isArray(value) ? (value[0] ?? "") : (value ?? "");
}

export default function CatererInvitationPage() {
  const params = useParams<{ schoolId: string; token: string }>();
  const schoolId = routeParam(params.schoolId);
  const token = routeParam(params.token);
  const [invitation, setInvitation] = useState<CatererInvitationPreview | null>(
    null,
  );
  const [loading, setLoading] = useState(true);
  const loadErrors = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = loadErrors;

  const load = useCallback(async () => {
    setLoading(true);
    clearLoadError();
    try {
      setInvitation(await fetchCatererInvitation(schoolId, token));
    } catch (cause) {
      logger.warn("caterer_invitation_load_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void showLoadError(cause, {
        object: "die Einladung",
        retry: () => void load(),
      });
    } finally {
      setLoading(false);
    }
  }, [clearLoadError, schoolId, showLoadError, token]);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) return <AuthShellSkeleton />;

  return (
    <AuthShell
      eyebrow="Einladung"
      title={
        invitation?.schoolName
          ? `${invitation.schoolName} lädt Sie ein`
          : "Diese Einladung lässt sich nicht öffnen"
      }
      subtitle={
        invitation
          ? "Mit diesem Konto sehen Sie, was die Schule bei Ihnen bestellt."
          : "Die Einladung kommt von der Schule per E-Mail."
      }
      variant="tenant"
      brand={<MotoBrand />}
    >
      {invitation?.addressTaken ? (
        <Alert
          type="info"
          message="Diese E-Mail-Adresse gehört schon zu einem anderen moto-Konto. Ein Caterer-Konto braucht eine eigene Adresse. Bitte fragen Sie die Schule nach einer Einladung an eine andere Adresse."
        />
      ) : invitation ? (
        <AcceptForm schoolId={schoolId} token={token} invitation={invitation} />
      ) : (
        <LoadErrorAlert error={loadErrors.error} />
      )}
    </AuthShell>
  );
}

function AcceptForm({
  schoolId,
  token,
  invitation,
}: Readonly<{
  schoolId: string;
  token: string;
  invitation: CatererInvitationPreview;
}>) {
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const existing = invitation.hasCatererAccount;

  const submit = async () => {
    setSubmitting(true);
    formErrors.clear();
    try {
      await acceptCatererInvitation(
        schoolId,
        token,
        password,
        existing ? password : confirmPassword,
      );
    } catch (cause) {
      void formErrors.show(credentialError(cause, LOGIN_REFUSALS), {
        object: "die Einladung",
        retry: () => latestSubmitRef.current(),
      });
      setSubmitting(false);
      return;
    }
    try {
      const tokens = await loginCaterer(invitation.email, password);
      await signIn("caterer-credentials", {
        redirect: false,
        internalRefresh: "true",
        token: tokens.accessToken,
        refreshToken: tokens.refreshToken,
      });
      window.location.href = catererPath("/caterer");
    } catch (cause) {
      // Accepted, only the automatic sign-in failed: the login page works.
      logger.warn("caterer_signin_after_accept_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      window.location.href = catererPath("/caterer/login");
    }
  };

  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
  });

  const passwordError = formErrors.fieldError("password");
  const confirmError = formErrors.fieldError("confirm_password");

  return (
    <form
      ref={formRef}
      noValidate
      className="space-y-6"
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      {formErrors.error ? <FormErrorAlert message={formErrors.error} /> : null}
      <p className="text-left text-sm text-gray-700">
        {existing
          ? `Sie haben schon ein Caterer-Konto für ${invitation.email}. Bestätigen Sie mit Ihrem Passwort. Dann kommt ${invitation.schoolName} zu Ihren Schulen dazu.`
          : `Sie melden sich mit ${invitation.email} an. Legen Sie jetzt ein Passwort fest.`}
      </p>

      <div className="space-y-4 text-left">
        <div>
          <label
            htmlFor="password"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            {existing ? "Ihr Passwort" : "Neues Passwort"}
          </label>
          <div className="relative">
            <input
              id="password"
              type={showPassword ? "text" : "password"}
              autoComplete={existing ? "current-password" : "new-password"}
              value={password}
              disabled={submitting}
              onChange={(event) => setPassword(event.target.value)}
              aria-invalid={Boolean(passwordError)}
              aria-describedby={passwordError ? "password-error" : undefined}
              className={`${authInputClass(Boolean(passwordError))} pr-10`}
            />
            <PasswordToggleButton
              showPassword={showPassword}
              onToggle={() => setShowPassword(!showPassword)}
            />
          </div>
          <AuthFieldError id="password-error" message={passwordError} />
        </div>

        {existing ? null : (
          <>
            <ul
              className="space-y-1 text-sm"
              aria-label="Das braucht das Passwort"
            >
              {PASSWORD_RULES.map((rule) => {
                const met = rule.test(password);
                return (
                  <li
                    key={rule.label}
                    className={`flex items-center gap-2 ${met ? "text-gray-900" : "text-gray-500"}`}
                  >
                    {met ? (
                      <Check className="h-4 w-4" aria-hidden="true" />
                    ) : (
                      <Circle className="h-4 w-4" aria-hidden="true" />
                    )}
                    {rule.label}
                  </li>
                );
              })}
            </ul>
            <div>
              <label
                htmlFor="confirm-password"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Passwort wiederholen
              </label>
              <input
                id="confirm-password"
                type={showPassword ? "text" : "password"}
                autoComplete="new-password"
                value={confirmPassword}
                disabled={submitting}
                onChange={(event) => setConfirmPassword(event.target.value)}
                aria-invalid={Boolean(confirmError)}
                aria-describedby={confirmError ? "confirm-error" : undefined}
                className={authInputClass(Boolean(confirmError))}
              />
              <AuthFieldError id="confirm-error" message={confirmError} />
            </div>
          </>
        )}
      </div>

      <button
        type="submit"
        disabled={submitting || password === ""}
        className={authPrimaryButtonClassName}
      >
        <span className="relative z-10">
          {submitting
            ? "Wird gespeichert …"
            : existing
              ? "Schule hinzufügen"
              : "Konto anlegen"}
        </span>
      </button>
    </form>
  );
}
