"use client";

// The page the school's mailed invitation opens (ADR 0077). The link names
// the school and carries the proof; nobody but the caterer has it. A new
// caterer sets its password, a caterer with an account confirms with its
// password and adds the school. Afterwards the caterer is signed in.

import { EmptyState } from "~/components/ui/empty-state";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { useParams } from "next/navigation";
import { Check } from "lucide-react";
import { AuthShell, MotoBrand } from "~/components/auth/auth-shell";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import {
  credentialError,
  LOGIN_REFUSALS,
} from "~/components/auth/credential-error";
import { MFAChallengeForm } from "~/components/auth/mfa-challenge-form";
import { PasswordRequirements } from "~/components/auth/password-requirements";
import { Alert } from "~/components/ui/alert";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Loading } from "~/components/ui/loading";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import {
  acceptCatererInvitation,
  fetchCatererInvitation,
  loginCaterer,
  type CatererInvitationPreview,
} from "~/lib/caterer-api";
import type { MFAStep } from "~/lib/sign-in-answer";
import { catererPath } from "~/lib/caterer-url";
import { PASSWORD_RULES } from "~/lib/password-rules";
import { createLogger } from "~/lib/logger";
import { Button, ButtonLink } from "~/components/ui/button";
import { InvitationSummary } from "~/components/auth/invitation-summary";

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

  if (loading) {
    return (
      <AuthShell
        title="Konto einrichten"
        subtitle="Wir prüfen Ihre Einladung."
        variant="tenant"
        brand={<MotoBrand />}
        formMaxWidth="max-w-[32rem]"
      >
        <Loading />
      </AuthShell>
    );
  }

  const existing = invitation?.hasCatererAccount ?? false;

  return (
    <AuthShell
      title={existing ? "Schule hinzufügen" : "Konto einrichten"}
      subtitle={
        invitation
          ? [
              `${invitation.schoolName || "Eine Schule"} lädt Sie ein.`,
              invitation.addressTaken
                ? ""
                : existing
                  ? "Bestätigen Sie mit Ihrem Passwort."
                  : "Wählen Sie Ihr Passwort.",
            ]
              .filter(Boolean)
              .join(" ")
          : "Die Einladung kommt von der Schule per E-Mail."
      }
      variant="tenant"
      brand={<MotoBrand />}
      formMaxWidth="max-w-[32rem]"
    >
      {invitation?.addressTaken ? (
        <Alert
          type="info"
          message="Diese E-Mail-Adresse gehört schon zu einem anderen moto-Konto. Ein Caterer-Konto braucht eine eigene Adresse. Bitte fragen Sie die Schule nach einer Einladung an eine andere Adresse."
        />
      ) : invitation ? (
        <AcceptForm schoolId={schoolId} token={token} invitation={invitation} />
      ) : (
        <div className="space-y-4">
          <LoadErrorAlert error={loadErrors.error} />
          <ButtonLink
            href={catererPath("/caterer/login")}
            variant="primary"
            size="field"
            className="w-full"
          >
            Zur Anmeldung
          </ButtonLink>
        </div>
      )}
    </AuthShell>
  );
}

/** Prüfung vor dem Senden. Schlüssel sind die Feldnamen des Backends. */
function acceptFieldErrors(
  existing: boolean,
  password: string,
  confirmPassword: string,
): Record<string, string> {
  if (existing) {
    return password === ""
      ? { password: "Bitte geben Sie Ihr Passwort ein." }
      : {};
  }
  if (!PASSWORD_RULES.every((rule) => rule.test(password))) {
    return {
      password: "Das Passwort erfüllt noch nicht alle Anforderungen.",
    };
  }
  if (password !== confirmPassword) {
    return { confirm_password: "Die Passwörter stimmen nicht überein." };
  }
  return {};
}

/** Wie lange der Erfolgsschirm steht, bevor die Seite wechselt. */
const ACCEPTED_SCREEN_MS = 1500;

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
  const [submitting, setSubmitting] = useState(false);
  const [accepted, setAccepted] = useState(false);
  const [mfaStep, setMfaStep] = useState<MFAStep | null>(null);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const existing = invitation.hasCatererAccount;

  const submit = async () => {
    const fields = acceptFieldErrors(existing, password, confirmPassword);
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", fields);
      return;
    }
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
    setAccepted(true);
    const shown = new Promise((resolve) =>
      setTimeout(resolve, ACCEPTED_SCREEN_MS),
    );
    let target = catererPath("/caterer");
    try {
      const result = await loginCaterer(invitation.email, password);
      if (result.status === "mfa_required") {
        // Two-factor sign-in is on (#4497): the sign-in already mailed its
        // code, so it is asked here instead of mailing a second one.
        await shown;
        setMfaStep(result);
        return;
      }
    } catch (cause) {
      // Accepted, only the automatic sign-in failed: the login page works.
      logger.warn("caterer_signin_after_accept_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      target = catererPath("/caterer/login");
    }
    await shown;
    window.location.href = target;
  };

  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
  });

  const passwordError = formErrors.fieldError("password");
  const confirmError = formErrors.fieldError("confirm_password");

  if (mfaStep) {
    return (
      <MFAChallengeForm
        scope="caterer"
        challengeToken={mfaStep.challengeToken}
        maskedEmail={mfaStep.maskedEmail}
        trustedDeviceEnabled={mfaStep.trustedDeviceEnabled}
        trustedDeviceDays={mfaStep.trustedDeviceDays}
        onSuccess={() => {
          window.location.href = catererPath("/caterer");
        }}
        onCancel={() => {
          window.location.href = catererPath("/caterer/login");
        }}
      />
    );
  }

  if (accepted) {
    return (
      <div role="status" aria-live="polite">
        {/* „weitergeleitet“, nicht „angemeldet“: scheitert die automatische
            Anmeldung, geht es zur Anmeldeseite. */}
        <EmptyState
          tone="success"
          icon={<Check aria-hidden="true" />}
          titleAs="h3"
          title={existing ? "Schule hinzugefügt" : "Konto erstellt"}
          description="Sie werden gleich weitergeleitet."
        />
      </div>
    );
  }

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

      <InvitationSummary email={invitation.email} />

      <div>
        <FieldLabel
          htmlFor="password"
          required
          invalid={Boolean(passwordError)}
        >
          Passwort
        </FieldLabel>
        <Input
          id="password"
          name="password"
          type="password"
          controlSize="compact"
          autoComplete={existing ? "current-password" : "new-password"}
          value={password}
          disabled={submitting}
          onChange={(event) => setPassword(event.target.value)}
          aria-invalid={Boolean(passwordError)}
          aria-describedby={passwordError ? "password-error" : undefined}
          required
          revealable
        />
        <FieldErrorHint id="password-error" message={passwordError} />
      </div>

      {existing ? null : (
        <>
          <div>
            <FieldLabel
              htmlFor="confirmPassword"
              required
              invalid={Boolean(confirmError)}
            >
              Passwort bestätigen
            </FieldLabel>
            <Input
              id="confirmPassword"
              name="confirm_password"
              type="password"
              controlSize="compact"
              autoComplete="new-password"
              value={confirmPassword}
              disabled={submitting}
              onChange={(event) => setConfirmPassword(event.target.value)}
              aria-invalid={Boolean(confirmError)}
              aria-describedby={
                confirmError ? "confirmPassword-error" : undefined
              }
              required
              revealable
            />
            <FieldErrorHint id="confirmPassword-error" message={confirmError} />
          </div>

          <PasswordRequirements
            requirements={PASSWORD_RULES.map((rule) => ({
              label: rule.label,
              met: rule.test(password),
            }))}
          />
        </>
      )}

      <Button
        type="submit"
        disabled={submitting}
        variant="primary"
        size="field"
        className="w-full"
      >
        <span className="relative z-10">
          {submitting ? "Wird übernommen…" : "Einladung akzeptieren"}
        </span>
      </Button>
    </form>
  );
}
