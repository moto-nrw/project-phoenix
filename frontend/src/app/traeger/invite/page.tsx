"use client";

// Einladung ins Träger-Büro annehmen (#4650). Der Link aus der Mail führt
// hierher. Die Person gibt ihren Namen an (#4693): Er steht im
// Exportprotokoll und im Verlauf jeder Rechnung. Wer noch kein moto-Konto
// hat, wählt ein Passwort; wer eines hat, bestätigt mit dessen Passwort.
// Danach meldet die Seite die Person an.

import {
  Suspense,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSearchParams } from "next/navigation";
import { AuthShell, MotoBrand } from "~/components/auth/auth-shell";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import {
  credentialError,
  LOGIN_REFUSALS,
} from "~/components/auth/credential-error";
import { PasswordRequirements } from "~/components/auth/password-requirements";
import {
  LoadErrorAlert,
  FormErrorAlert,
} from "~/components/ui/form-error-alert";
import { Loading } from "~/components/ui/loading";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { PASSWORD_RULES } from "~/lib/password-rules";
import {
  OWN_NAME_REQUIRED,
  TRAEGER_NAME_HINT,
  TRAEGER_NAME_MAX_LENGTH,
} from "~/lib/traeger-person-name";
import {
  acceptTraegerInvitation,
  fetchTraegerInvitation,
  traegerLogin,
  type TraegerInvitationPreview,
} from "~/lib/traeger-api";
import { TRAEGER_ROLE_LABELS } from "~/lib/traeger-roles";
import { traegerPath } from "~/lib/traeger-url";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";

const logger = createLogger({ component: "TraegerInvitePage" });

const SHELL_COPY = {
  title: "Einladung ins Träger-Büro",
  subtitle: "Nehmen Sie die Einladung an. Danach melden Sie sich an.",
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
      window.location.assign(traegerPath("/traeger"));
      return;
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
  const [name, setName] = useState("");
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
    const fields: Record<string, string> = {};
    if (!name.trim()) fields.name = OWN_NAME_REQUIRED;
    if (!existing) {
      if (!requirements.every((requirement) => requirement.met)) {
        fields.password = "Das Passwort erfüllt noch nicht alle Anforderungen.";
      } else if (password !== confirmPassword) {
        fields.confirm_password = "Die Passwörter stimmen nicht überein.";
      }
    }
    if (Object.keys(fields).length > 0) {
      formErrors.invalid(
        fields.name
          ? "Bitte prüfen Sie Ihre Angaben."
          : "Bitte prüfen Sie Ihr Passwort.",
        fields,
      );
      return;
    }
    setSubmitting(true);
    let email: string;
    try {
      email = await acceptTraegerInvitation(token, {
        name: name.trim(),
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

      <Input
        id="name"
        name="name"
        label="Ihr Name"
        controlSize="compact"
        autoComplete="name"
        required
        disabled={submitting}
        value={name}
        onChange={(e) => setName(e.target.value)}
        error={formErrors.fieldError("name")}
        hint={TRAEGER_NAME_HINT}
        maxLength={TRAEGER_NAME_MAX_LENGTH}
      />

      {existing ? (
        <p className="text-sm text-gray-700">
          Sie haben schon ein moto-Konto. Geben Sie das Passwort dieses Kontos
          ein.
        </p>
      ) : null}

      <div>
        <FieldLabel
          htmlFor="password"
          required
          invalid={Boolean(formErrors.fieldError("password"))}
        >
          {existing ? "Passwort Ihres moto-Kontos" : "Neues Passwort"}
        </FieldLabel>
        <Input
          id="password"
          name="password"
          type="password"
          controlSize="touch"
          autoComplete={existing ? "current-password" : "new-password"}
          required
          disabled={submitting}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          aria-invalid={Boolean(formErrors.fieldError("password"))}
          aria-describedby={
            formErrors.fieldError("password") ? "password-error" : undefined
          }
          revealable
          revealed={showPassword}
          onRevealedChange={setShowPassword}
        />
        <FieldErrorHint
          id="password-error"
          message={formErrors.fieldError("password")}
        />
      </div>

      {existing ? null : (
        <>
          <div>
            <Input
              id="confirm_password"
              name="confirm_password"
              type={showPassword ? "text" : "password"}
              label="Passwort wiederholen"
              controlSize="touch"
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
            />
            <FieldErrorHint
              id="confirm-password-error"
              message={formErrors.fieldError("confirm_password")}
            />
          </div>
          <PasswordRequirements requirements={requirements} />
        </>
      )}

      <Button
        type="submit"
        disabled={submitting || !password}
        variant="primary"
        size="field"
        className="w-full"
      >
        <span className="relative z-10">
          {submitting ? "Wird angenommen …" : "Einladung annehmen"}
        </span>
      </Button>
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
    // A link without its token cannot be repaired by waiting; the page says
    // so instead of asking the server.
    if (!token) {
      setLoading(false);
      return;
    }
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

  if (!token) {
    return (
      <div className="space-y-4 text-left">
        <Alert
          type="warning"
          message="Der Link ist unvollständig. Öffnen Sie den Link aus Ihrer Einladungs-Mail noch einmal. Klappt das nicht, bitten Sie das Träger-Büro um eine neue Einladung."
        />
        <ButtonLink
          href={traegerPath("/traeger/login")}
          variant="outline"
          size="field"
          className="w-full"
        >
          Zur Anmeldung
        </ButtonLink>
      </div>
    );
  }
  if (loading) {
    return <Loading message="Einladung wird geprüft …" />;
  }
  if (invitationLoad.error || !invitation) {
    return (
      <div className="text-left">
        <LoadErrorAlert error={invitationLoad.error} />
      </div>
    );
  }
  return <InviteForm token={token} invitation={invitation} />;
}

export default function TraegerInvitePage() {
  return (
    <AuthShell {...SHELL_COPY} variant="tenant" brand={<MotoBrand />}>
      <Suspense fallback={<Loading message="Laden …" />}>
        <InviteContent />
      </Suspense>
    </AuthShell>
  );
}
