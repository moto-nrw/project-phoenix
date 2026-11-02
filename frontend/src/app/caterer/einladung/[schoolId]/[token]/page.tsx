"use client";

// The page the school's mailed invitation opens (ADR 0077). The link names
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
import Link from "next/link";
import { useParams } from "next/navigation";
import { Check } from "lucide-react";
import {
  AuthFieldError,
  AuthShell,
  MotoBrand,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { FieldLabel } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import {
  credentialError,
  LOGIN_REFUSALS,
} from "~/components/auth/credential-error";
import { MFAChallengeForm } from "~/components/auth/mfa-challenge-form";
import { PasswordRequirements } from "~/components/auth/password-requirements";
import { PasswordToggleButton } from "~/components/shared/password-toggle-button";
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
  type CatererLoginResult,
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

  if (loading) {
    return (
      <AuthShell
        eyebrow="Einladung"
        eyebrowClassName="text-moto-green"
        title="Konto einrichten"
        subtitle="Wir prüfen Ihre Einladung."
        variant="tenant"
        brand={<MotoBrand />}
        formMaxWidth="max-w-[32rem]"
      >
        <Loading fullPage={false} />
      </AuthShell>
    );
  }

  const existing = invitation?.hasCatererAccount ?? false;

  return (
    <AuthShell
      eyebrow="Einladung"
      eyebrowClassName="text-moto-green"
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
          <Link
            href={catererPath("/caterer/login")}
            className={authPrimaryButtonClassName}
          >
            Zur Anmeldung
          </Link>
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
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [accepted, setAccepted] = useState(false);
  const [mfaStep, setMfaStep] = useState<Extract<
    CatererLoginResult,
    { status: "mfa_required" }
  > | null>(null);
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
      <div
        role="status"
        aria-live="polite"
        className="flex flex-col items-center py-12"
      >
        <div className="mb-6 flex h-12 w-12 items-center justify-center rounded-full bg-gray-900">
          <Check className="h-6 w-6 text-white" aria-hidden="true" />
        </div>
        <h3 className="mb-1 text-base font-semibold text-gray-900">
          {existing ? "Schule hinzugefügt" : "Konto erstellt"}
        </h3>
        {/* „weitergeleitet“, nicht „angemeldet“: scheitert die automatische
            Anmeldung, geht es zur Anmeldeseite. */}
        <p className="text-sm text-gray-500">
          Sie werden gleich weitergeleitet.
        </p>
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

      <section className="rounded-lg border border-gray-200 bg-gray-50 px-3.5 py-3 text-sm">
        <p className="font-medium text-gray-700">
          Einladung für{" "}
          <span className="font-semibold text-gray-900">
            {invitation.email}
          </span>
        </p>
      </section>

      <div>
        <FieldLabel
          htmlFor="password"
          required
          invalid={Boolean(passwordError)}
        >
          Passwort
        </FieldLabel>
        <div className="relative">
          <Input
            id="password"
            name="password"
            type={showPassword ? "text" : "password"}
            controlSize="compact"
            autoComplete={existing ? "current-password" : "new-password"}
            value={password}
            disabled={submitting}
            onChange={(event) => setPassword(event.target.value)}
            aria-invalid={Boolean(passwordError)}
            aria-describedby={passwordError ? "password-error" : undefined}
            required
            className="pr-10"
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
          <div>
            <FieldLabel
              htmlFor="confirmPassword"
              required
              invalid={Boolean(confirmError)}
            >
              Passwort bestätigen
            </FieldLabel>
            <div className="relative">
              <Input
                id="confirmPassword"
                name="confirm_password"
                type={showConfirmPassword ? "text" : "password"}
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
                className="pr-10"
              />
              <PasswordToggleButton
                showPassword={showConfirmPassword}
                onToggle={() => setShowConfirmPassword(!showConfirmPassword)}
              />
            </div>
            <AuthFieldError id="confirmPassword-error" message={confirmError} />
          </div>

          <PasswordRequirements
            requirements={PASSWORD_RULES.map((rule) => ({
              label: rule.label,
              met: rule.test(password),
            }))}
          />
        </>
      )}

      <button
        type="submit"
        disabled={submitting}
        className={authPrimaryButtonClassName}
      >
        <span className="relative z-10">
          {submitting ? "Wird übernommen…" : "Einladung akzeptieren"}
        </span>
      </button>
    </form>
  );
}
