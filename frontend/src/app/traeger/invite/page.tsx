"use client";

// Einladung ins Träger-Büro annehmen (#4650). Der Link aus der Mail führt
// hierher. Wer noch kein moto-Konto hat, wählt ein Passwort; wer eines hat,
// bestätigt mit dessen Passwort. Danach meldet die Seite die Person an.

import {
  Suspense,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { signIn } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import {
  AuthFieldError,
  AuthShell,
  MotoBrand,
  authInputClass,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import {
  credentialError,
  LOGIN_REFUSALS,
} from "~/components/auth/credential-error";
import { PasswordRequirements } from "~/components/auth/password-requirements";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
import {
  LoadErrorAlert,
  FormErrorAlert,
} from "~/components/ui/form-error-alert";
import { Loading } from "~/components/ui/loading";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { PASSWORD_RULES } from "~/lib/password-rules";
import {
  acceptTraegerInvitation,
  fetchTraegerInvitation,
  traegerLogin,
  type TraegerInvitationPreview,
} from "~/lib/traeger-api";
import { TRAEGER_ROLE_LABELS } from "~/lib/traeger-roles";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerInvitePage" });

const SHELL_COPY = {
  eyebrow: "Träger-Portal",
  title: "Einladung ins Träger-Büro",
  subtitle: "Nehmen Sie die Einladung an. Danach sind Sie angemeldet.",
} as const;

/**
 * Signs the new member in. The invitation is already spent, so any failure
 * here sends the person to the Träger login instead of offering to accept
 * again; a member of several Träger chooses there.
 */
async function signInAfterAcceptance(email: string, password: string) {
  try {
    const result = await traegerLogin({ email, password });
    if (result.status === "authenticated") {
      const seeded = await signIn("traeger-credentials", {
        redirect: false,
        internalRefresh: "true",
        token: result.accessToken,
        refreshToken: result.refreshToken,
      });
      if (!seeded?.error) {
        window.location.assign(traegerPath("/traeger"));
        return;
      }
      logger.error("session_seed_failed", { error: seeded.error });
    }
  } catch (err) {
    logger.warn("traeger_sign_in_after_acceptance_failed", {
      error: err instanceof Error ? err.message : String(err),
    });
  }
  window.location.assign(traegerPath("/traeger/login"));
}

function InviteForm({
  token,
  invitation,
}: Readonly<{ token: string; invitation: TraegerInvitationPreview }>) {
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const existing = invitation.requiresExistingPassword;

  const requirements = useMemo(
    () =>
      PASSWORD_RULES.map(({ label, test }) => ({ label, met: test(password) })),
    [password],
  );

  const submit = async () => {
    formErrors.clear();
    if (!existing) {
      const fields: Record<string, string> = {};
      if (!requirements.every((requirement) => requirement.met)) {
        fields.password = "Das Passwort erfüllt noch nicht alle Anforderungen.";
      } else if (password !== confirmPassword) {
        fields.confirm_password = "Die Passwörter stimmen nicht überein.";
      }
      if (Object.keys(fields).length > 0) {
        formErrors.invalid("Bitte prüfen Sie Ihr Passwort.", fields);
        return;
      }
    }
    setSubmitting(true);
    let email: string;
    try {
      email = await acceptTraegerInvitation(token, {
        password,
        confirmPassword: existing ? undefined : confirmPassword,
      });
    } catch (err) {
      logger.error("traeger_invitation_accept_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(credentialError(err, LOGIN_REFUSALS), {
        object: "die Einladung",
        retry: () => latestSubmitRef.current(),
      });
      setSubmitting(false);
      return;
    }
    await signInAfterAcceptance(email, password);
  };

  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
  });

  const roles = invitation.roles.map((role) => TRAEGER_ROLE_LABELS[role]);

  return (
    <form
      ref={formRef}
      noValidate
      className="space-y-6 text-left"
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      <FormErrorAlert message={formErrors.error} />
      <div className="space-y-1 text-sm wrap-anywhere text-gray-700">
        <p>
          Sie sind ins Träger-Büro von{" "}
          <span className="font-semibold">{invitation.organizationName}</span>{" "}
          eingeladen.
        </p>
        <p>
          Ihre Rolle: <span className="font-semibold">{roles.join(", ")}</span>
        </p>
        <p>
          Ihre E-Mail-Adresse:{" "}
          <span className="font-semibold">{invitation.email}</span>
        </p>
      </div>

      {existing ? (
        <p className="text-sm text-gray-700">
          Sie haben schon ein moto-Konto. Geben Sie das Passwort dieses Kontos
          ein.
        </p>
      ) : null}

      <div>
        <label
          htmlFor="password"
          className="mb-1 block text-sm font-medium text-gray-700"
        >
          {existing ? "Passwort Ihres moto-Kontos" : "Neues Passwort"}
        </label>
        <div className="relative">
          <input
            id="password"
            name="password"
            type={showPassword ? "text" : "password"}
            autoComplete={existing ? "current-password" : "new-password"}
            required
            disabled={submitting}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            aria-invalid={Boolean(formErrors.fieldError("password"))}
            aria-describedby={
              formErrors.fieldError("password") ? "password-error" : undefined
            }
            className={`${authInputClass(Boolean(formErrors.fieldError("password")))} pr-10`}
          />
          <PasswordToggleButton
            showPassword={showPassword}
            onToggle={() => setShowPassword(!showPassword)}
          />
        </div>
        <AuthFieldError
          id="password-error"
          message={formErrors.fieldError("password")}
        />
      </div>

      {existing ? null : (
        <>
          <div>
            <label
              htmlFor="confirm_password"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Passwort wiederholen
            </label>
            <input
              id="confirm_password"
              name="confirm_password"
              type={showPassword ? "text" : "password"}
              autoComplete="new-password"
              required
              disabled={submitting}
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              aria-invalid={Boolean(formErrors.fieldError("confirm_password"))}
              aria-describedby={
                formErrors.fieldError("confirm_password")
                  ? "confirm-password-error"
                  : undefined
              }
              className={authInputClass(
                Boolean(formErrors.fieldError("confirm_password")),
              )}
            />
            <AuthFieldError
              id="confirm-password-error"
              message={formErrors.fieldError("confirm_password")}
            />
          </div>
          <PasswordRequirements requirements={requirements} />
        </>
      )}

      <button
        type="submit"
        disabled={submitting || !password}
        className={authPrimaryButtonClassName}
      >
        <span className="relative z-10">
          {submitting ? "Wird angenommen …" : "Einladung annehmen"}
        </span>
      </button>
    </form>
  );
}

function InviteContent() {
  const token = useSearchParams().get("token") ?? "";
  const [invitation, setInvitation] = useState<TraegerInvitationPreview | null>(
    null,
  );
  const [loading, setLoading] = useState(true);
  const invitationLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = invitationLoad;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setInvitation(await fetchTraegerInvitation(token));
      clearLoadError();
    } catch (err) {
      logger.warn("traeger_invitation_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setInvitation(null);
      await showLoadError(err, { object: "die Einladung" });
    } finally {
      setLoading(false);
    }
  }, [token, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) {
    return <Loading fullPage={false} message="Einladung wird geprüft …" />;
  }
  if (invitationLoad.error || !invitation) {
    return (
      <div className="space-y-4 text-left">
        <LoadErrorAlert error={invitationLoad.error} />
        <p className="text-sm text-gray-700">
          Bitten Sie die Person, die Sie eingeladen hat, um eine neue Einladung.
        </p>
      </div>
    );
  }
  return <InviteForm token={token} invitation={invitation} />;
}

export default function TraegerInvitePage() {
  return (
    <AuthShell {...SHELL_COPY} variant="tenant" brand={<MotoBrand />}>
      <Suspense fallback={<Loading fullPage={false} message="Laden …" />}>
        <InviteContent />
      </Suspense>
    </AuthShell>
  );
}
