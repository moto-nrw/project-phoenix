"use client";

// Elternkonto aus dem Link der Ferienanmeldung (#6673, ADR 0216). Drei
// Zustände: Es gibt noch kein Konto (Passwort festlegen), es gibt schon
// eins (anmelden), oder der Link gilt nicht mehr. In beiden ersten Fällen
// trägt der Server den Zugang zum Träger ein; danach geht es zum Login und
// von dort zu den eigenen Ferienanmeldungen.

import { Check } from "lucide-react";
import { useTranslations } from "next-intl";
import {
  type FormEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  AuthShell,
  AuthShellSkeleton,
  MotoBrand,
} from "~/components/auth/auth-shell";
import {
  credentialError,
  INVITATION_REFUSALS,
} from "~/components/auth/credential-error";
import { InvitationSummary } from "~/components/auth/invitation-summary";
import { buildParentAuthShellCopy } from "~/components/auth/parent-auth-shell-copy";
import { PasswordRequirements } from "~/components/auth/password-requirements";
import { LanguageSwitcher } from "~/components/parent/language-switcher";
import { Button, ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import {
  completeHolidayParentSetup,
  fetchHolidayParentSetup,
  type HolidayParentSetup,
} from "~/lib/holiday-parent-setup-api";
import { createLogger } from "~/lib/logger";
import { holidayRegistrationsLoginPath } from "~/lib/parent-holiday-registration";
import { PASSWORD_RULES } from "~/lib/password-rules";

const logger = createLogger({ component: "HolidayParentSetup" });

type LoadState =
  | { readonly kind: "loading" }
  | { readonly kind: "ready"; readonly setup: HolidayParentSetup }
  /** The link is unknown or expired (404, `identity.holiday_parent_setup_invalid`). */
  | { readonly kind: "invalid" }
  | { readonly kind: "failed" };

/** An unknown, expired or used-up link: the page explains, no error text. */
function isInvalidLink(error: unknown): boolean {
  return (
    error instanceof ApiError && (error.status === 404 || error.status === 410)
  );
}

// A Träger name such as „Ganztag Nord e. V.“ ends the sentence it stands
// in; without its own full stop the sentence does not end on two.
function nameBeforeFullStop(name: string): string {
  return name.replace(/\.$/, "");
}

export function HolidayParentSetupView({ token }: Readonly<{ token: string }>) {
  const t = useTranslations("holidayParentSetup");
  const tAuthShell = useTranslations("parentAuthShell");
  const testimonialPanelCopy = useMemo(
    () => buildParentAuthShellCopy(tAuthShell),
    [tAuthShell],
  );
  const [state, setState] = useState<LoadState>({ kind: "loading" });
  const [done, setDone] = useState(false);
  const loadError = useApiLoadError();
  const { show, clear } = loadError;

  const load = useCallback(async () => {
    try {
      const setup = await fetchHolidayParentSetup(token);
      clear();
      setState({ kind: "ready", setup });
    } catch (err) {
      if (isInvalidLink(err)) {
        clear();
        setState({ kind: "invalid" });
        return;
      }
      logger.error("holiday_parent_setup_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setState({ kind: "failed" });
      await show(err, { object: t("errorObject"), retry: () => void load() });
    }
  }, [clear, show, t, token]);

  useEffect(() => {
    void load();
  }, [load]);

  if (state.kind === "loading") return <AuthShellSkeleton />;

  const shell = (
    title: string,
    subtitle: string | undefined,
    children: ReactNode,
  ) => (
    <AuthShell
      title={title}
      subtitle={subtitle}
      variant="parents"
      brand={<MotoBrand />}
      formMaxWidth="max-w-[32rem]"
      footer={<LanguageSwitcher />}
      testimonialPanelCopy={testimonialPanelCopy}
    >
      {children}
    </AuthShell>
  );

  if (state.kind === "failed") {
    return shell(
      t("newTitle"),
      undefined,
      <LoadErrorAlert error={loadError.error} />,
    );
  }

  if (state.kind === "invalid") {
    return shell(
      t("invalidTitle"),
      undefined,
      <div className="space-y-5">
        <div className="space-y-2 text-center text-sm leading-6 text-gray-700">
          <p>{t("invalidText")}</p>
          <p>{t("invalidNext")}</p>
        </div>
        <ButtonLink
          href={holidayRegistrationsLoginPath()}
          variant="primary"
          size="field"
          className="w-full"
        >
          {t("login")}
        </ButtonLink>
      </div>,
    );
  }

  const { setup } = state;
  if (done) {
    return shell(
      t("doneTitle"),
      undefined,
      <EmptyState
        tone="success"
        icon={<Check aria-hidden="true" />}
        description={t("doneText", { email: setup.email })}
        action={
          <ButtonLink
            href={holidayRegistrationsLoginPath()}
            variant="primary"
            size="field"
          >
            {t("login")}
          </ButtonLink>
        }
      />,
    );
  }

  if (setup.accountExists) {
    return shell(
      t("existsTitle"),
      t("existsSubtitle", { traeger: nameBeforeFullStop(setup.traegerName) }),
      <ExistingAccount token={token} email={setup.email} />,
    );
  }

  return shell(
    t("newTitle"),
    t("newSubtitle", { traeger: nameBeforeFullStop(setup.traegerName) }),
    <NewAccountForm
      token={token}
      email={setup.email}
      onDone={(accountExists) => {
        if (accountExists) {
          setState({ kind: "ready", setup: { ...setup, accountExists: true } });
        } else {
          setDone(true);
        }
      }}
    />,
  );
}

/**
 * The address has an account: one action. It adds the access at the Träger
 * without touching the password and then opens the login.
 */
function ExistingAccount({
  token,
  email,
}: Readonly<{ token: string; email: string }>) {
  const t = useTranslations("holidayParentSetup");
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const [sending, setSending] = useState(false);

  const submit = async () => {
    formErrors.clear();
    setSending(true);
    try {
      await completeHolidayParentSetup(token);
      globalThis.location.assign(holidayRegistrationsLoginPath());
    } catch (err) {
      logger.error("holiday_parent_setup_link_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setSending(false);
      void formErrors.show(err, {
        object: t("errorObject"),
        retry: () => void submit(),
      });
    }
  };

  return (
    <form
      ref={formRef}
      noValidate
      className="space-y-5"
      onSubmit={(event: FormEvent) => {
        event.preventDefault();
        void submit();
      }}
    >
      <FormErrorAlert message={formErrors.error} />
      <InvitationSummary label={t("accountFor")} email={email} />
      <Button
        type="submit"
        variant="primary"
        size="field"
        className="w-full"
        isLoading={sending}
        loadingText={t("opening")}
      >
        {t("login")}
      </Button>
    </form>
  );
}

/** No account yet: the address stays fixed, the parent sets a password. */
function NewAccountForm({
  token,
  email,
  onDone,
}: Readonly<{
  token: string;
  email: string;
  onDone: (accountExists: boolean) => void;
}>) {
  const t = useTranslations("holidayParentSetup");
  // Labels, rules and hints of the guardian invitation: one password form
  // for every way into the parents app.
  const tPassword = useTranslations("guardianInvite");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [sending, setSending] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const latestSubmitRef = useRef<() => void>(() => undefined);

  const requirements = useMemo(
    () =>
      PASSWORD_RULES.map(({ test }, index) => ({
        label: tPassword(`passwordRules.${index}`),
        met: test(password),
      })),
    [password, tPassword],
  );

  const submit = async () => {
    formErrors.clear();
    if (!requirements.every((requirement) => requirement.met)) {
      formErrors.invalid(tPassword("formErrors.passwordRules"), {
        password: "",
      });
      return;
    }
    if (password !== confirmPassword) {
      formErrors.invalid(tPassword("formErrors.passwordMismatch"), {
        confirm_password: "",
      });
      return;
    }
    setSending(true);
    try {
      const result = await completeHolidayParentSetup(token, {
        password,
        confirmPassword,
      });
      onDone(result.accountExists);
    } catch (err) {
      logger.error("holiday_parent_setup_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // Before the login a 401 means "refused", not "session expired".
      void formErrors.show(credentialError(err, INVITATION_REFUSALS), {
        object: t("errorObject"),
        retry: () => latestSubmitRef.current(),
      });
    } finally {
      setSending(false);
    }
  };

  // „Wiederholen“ sendet die Passwörter, die dann in den Feldern stehen.
  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
  });

  const passwordInvalid = formErrors.fieldError("password") !== undefined;
  const confirmInvalid =
    formErrors.fieldError("confirm_password") !== undefined;

  return (
    <form
      ref={formRef}
      noValidate
      className="space-y-5 sm:space-y-6"
      onSubmit={(event: FormEvent) => {
        event.preventDefault();
        void submit();
      }}
    >
      <FormErrorAlert message={formErrors.error} />
      <InvitationSummary label={t("accountFor")} email={email} />
      <Input
        id="holiday-parent-password"
        name="password"
        type="password"
        label={tPassword("passwordLabel")}
        value={password}
        onChange={(event) => setPassword(event.target.value)}
        disabled={sending}
        autoComplete="new-password"
        aria-invalid={passwordInvalid || undefined}
        controlSize="compact"
        required
        revealable
        showPasswordLabel={tPassword("showPassword")}
        hidePasswordLabel={tPassword("hidePassword")}
      />
      <Input
        id="holiday-parent-confirm-password"
        name="confirm_password"
        type="password"
        label={tPassword("confirmPasswordLabel")}
        value={confirmPassword}
        onChange={(event) => setConfirmPassword(event.target.value)}
        disabled={sending}
        autoComplete="new-password"
        aria-invalid={confirmInvalid || undefined}
        controlSize="compact"
        required
        revealable
        showPasswordLabel={tPassword("showPassword")}
        hidePasswordLabel={tPassword("hidePassword")}
      />
      <PasswordRequirements
        title={tPassword("requirementsTitle")}
        requirements={requirements}
      />
      <Button
        type="submit"
        variant="primary"
        size="field"
        className="w-full"
        isLoading={sending}
        loadingText={t("submitting")}
      >
        {t("submit")}
      </Button>
    </form>
  );
}
