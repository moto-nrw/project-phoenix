"use client";

// The second factor a person switches on in their own account: a code by
// e-mail after the password. The caterer portal (#4497) and the parents
// portal (#7051) share this card; each passes its routes and, where the
// portal speaks the parent's language, its copy.

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { credentialError } from "~/components/auth/credential-error";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import type { EmailCodeFactorApi } from "~/lib/email-code-factor-api";
import { createLogger } from "~/lib/logger";
import { useShellAuth } from "~/lib/shell-auth-context";

const logger = createLogger({ component: "EmailCodeFactorSection" });

// A wrong code and a wrong password come back as 401 with their own code:
// they stay in the card instead of ending the session.
const CODE_REFUSALS = ["identity.mfa_code_invalid"] as const;
const PASSWORD_REFUSALS = ["identity.current_password_wrong"] as const;
const CODE_LENGTH = 6;

/** Every text of the card; the German default is the caterer portal's. */
export interface EmailCodeFactorCopy {
  readonly title: string;
  readonly description: string;
  readonly enabled: string;
  readonly disabled: string;
  readonly switchOn: string;
  readonly switchOnTooltip: string;
  readonly switchOff: string;
  readonly switchOffTooltip: string;
  readonly codeGoesTo: string;
  readonly sentTo: (maskedEmail: string) => string;
  readonly codeLabel: string;
  readonly confirm: string;
  readonly confirmTooltip: string;
  readonly resend: string;
  readonly resendTooltip: string;
  readonly cancel: string;
  readonly disableIntro: string;
  readonly disableTooltip: string;
  readonly currentPassword: string;
  readonly enabledToast: string;
  readonly disabledToast: string;
  readonly renewFailed: string;
  readonly reauthenticate: string;
  readonly reauthenticateTooltip: string;
  /** Shown instead of the switch when the account cannot switch it on. */
  readonly unavailable: string;
  readonly loadObject: string;
  readonly startObject: string;
  readonly confirmObject: string;
  readonly disableObject: string;
}

const GERMAN_EMAIL_CODE_FACTOR_COPY: EmailCodeFactorCopy = {
  title: "Zwei-Faktor-Anmeldung",
  description:
    "Freiwillig. Ist sie eingeschaltet, geben Sie bei der Anmeldung nach dem Passwort einen Code ein. Den Code schickt moto an Ihre E-Mail-Adresse.",
  enabled: "Eingeschaltet",
  disabled: "Ausgeschaltet",
  switchOn: "Einschalten",
  switchOnTooltip: "Schickt einen Code an Ihre E-Mail-Adresse.",
  switchOff: "Ausschalten",
  switchOffTooltip: "Fragt zur Sicherheit nach Ihrem Passwort.",
  codeGoesTo: "Der Code geht an",
  sentTo: (maskedEmail) =>
    `Wir haben einen Code an ${maskedEmail || "Ihre E-Mail-Adresse"} geschickt. Geben Sie die 6 Ziffern hier ein.`,
  codeLabel: "Code",
  confirm: "Einschalten",
  confirmTooltip: "Schaltet die Anmeldung mit Code für Ihr Konto ein.",
  resend: "Neuen Code senden",
  resendTooltip: "Schickt einen neuen Code. Der alte gilt dann nicht mehr.",
  cancel: "Abbrechen",
  disableIntro:
    "Danach reicht bei der Anmeldung wieder Ihr Passwort. Geben Sie zur Sicherheit Ihr aktuelles Passwort ein.",
  disableTooltip: "Schaltet die Anmeldung mit Code sofort aus.",
  currentPassword: "Aktuelles Passwort",
  enabledToast: "Die Zwei-Faktor-Anmeldung ist eingeschaltet.",
  disabledToast: "Die Zwei-Faktor-Anmeldung ist ausgeschaltet.",
  renewFailed:
    "Die Zwei-Faktor-Anmeldung ist eingeschaltet. Die Anmeldung konnte nicht erneuert werden. Bitte melden Sie sich neu an.",
  reauthenticate: "Neu anmelden",
  reauthenticateTooltip: "Meldet Sie ab. Danach melden Sie sich mit Code an.",
  unavailable: "",
  loadObject: "die Zwei-Faktor-Anmeldung",
  startObject: "das Senden des Codes",
  confirmObject: "das Einschalten der Zwei-Faktor-Anmeldung",
  disableObject: "das Ausschalten der Zwei-Faktor-Anmeldung",
};

type Step =
  | { kind: "idle" }
  | { kind: "code"; challengeToken: string; maskedEmail: string }
  | { kind: "disable" }
  | { kind: "reauthenticate" };

interface EmailCodeFactorSectionProps {
  readonly api: EmailCodeFactorApi;
  readonly copy?: EmailCodeFactorCopy;
  /** Symbol tile before the title. */
  readonly leading?: ReactNode;
  readonly headingLevel?: 2 | 3;
  /**
   * The device card only makes sense while the factor is on: switching it
   * off deletes the remembered devices in the backend.
   */
  readonly onEnabledChange: (enabled: boolean) => void;
  /** More the portal shows while no step runs, given the factor's state. */
  readonly renderIdle?: (enabled: boolean, available: boolean) => ReactNode;
}

interface FactorState {
  readonly enabled: boolean;
  /** Absent means available: older answers name no restriction. */
  readonly available?: boolean;
  readonly maskedEmail: string;
}

export function EmailCodeFactorSection({
  api,
  copy = GERMAN_EMAIL_CODE_FACTOR_COPY,
  leading,
  headingLevel = 2,
  onEnabledChange,
  renderIdle,
}: EmailCodeFactorSectionProps) {
  const { logout } = useShellAuth();
  const toast = useToast();
  const [state, setStateValue] = useState<FactorState | null>(null);
  const setState = useCallback(
    (value: FactorState) => {
      setStateValue(value);
      onEnabledChange(value.enabled);
    },
    [onEnabledChange],
  );
  const [step, setStep] = useState<Step>({ kind: "idle" });
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const { error: loadError, show: showLoadError, clear } = useApiLoadError();
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const latestLoadRef = useRef<() => void>(() => undefined);
  const latestStartRef = useRef<() => void>(() => undefined);
  const latestConfirmRef = useRef<() => void>(() => undefined);
  const latestDisableRef = useRef<() => void>(() => undefined);

  const load = useCallback(async () => {
    clear();
    try {
      setState(await api.status());
    } catch (err) {
      logger.warn("email_code_factor_status_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showLoadError(err, {
        object: copy.loadObject,
        retry: () => latestLoadRef.current(),
      });
    }
  }, [api, clear, copy.loadObject, setState, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const reset = () => {
    setStep({ kind: "idle" });
    setCode("");
    setPassword("");
    formErrors.clear();
  };

  const start = async () => {
    setBusy(true);
    formErrors.clear();
    try {
      const challenge = await api.start();
      setCode("");
      setStep({
        kind: "code",
        challengeToken: challenge.challengeToken,
        maskedEmail: challenge.maskedEmail,
      });
    } catch (err) {
      logger.warn("email_code_factor_start_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, {
        object: copy.startObject,
        retry: () => latestStartRef.current(),
      });
    } finally {
      setBusy(false);
    }
  };

  const switchedOn = () => {
    if (state) setState({ ...state, enabled: true });
  };

  const confirm = async () => {
    if (step.kind !== "code") return;
    setBusy(true);
    formErrors.clear();
    try {
      const { sessionRenewed } = await api.confirm(
        step.challengeToken,
        code.trim(),
      );
      // The answer is a new session with the second factor proven, which
      // the BFF installs; the old one would lose access at its next renewal.
      if (!sessionRenewed) {
        logger.error("email_code_factor_session_seed_failed", {});
        switchedOn();
        setCode("");
        setStep({ kind: "reauthenticate" });
        formErrors.invalid(copy.renewFailed);
        return;
      }
      switchedOn();
      reset();
      toast.success(copy.enabledToast);
    } catch (err) {
      logger.warn("email_code_factor_confirm_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(credentialError(err, CODE_REFUSALS), {
        object: copy.confirmObject,
        retry: () => latestConfirmRef.current(),
      });
    } finally {
      setBusy(false);
    }
  };

  const disable = async () => {
    setBusy(true);
    formErrors.clear();
    try {
      await api.disable(password);
      if (state) setState({ ...state, enabled: false });
      reset();
      toast.success(copy.disabledToast);
    } catch (err) {
      logger.warn("email_code_factor_disable_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(credentialError(err, PASSWORD_REFUSALS), {
        object: copy.disableObject,
        retry: () => latestDisableRef.current(),
      });
    } finally {
      setBusy(false);
    }
  };

  useLayoutEffect(() => {
    latestLoadRef.current = () => void load();
    latestStartRef.current = () => void start();
    latestConfirmRef.current = () => void confirm();
    latestDisableRef.current = () => void disable();
  });

  const switchable =
    state !== null &&
    step.kind === "idle" &&
    (state.enabled || state.available !== false);
  const action = switchable ? (
    <Button
      type="button"
      variant={state.enabled ? "outline" : "primary"}
      size="md"
      disabled={busy}
      isLoading={busy && !state.enabled}
      tooltip={state.enabled ? copy.switchOffTooltip : copy.switchOnTooltip}
      onClick={() => {
        if (state.enabled) {
          formErrors.clear();
          setStep({ kind: "disable" });
        } else {
          void start();
        }
      }}
    >
      {state.enabled ? copy.switchOff : copy.switchOn}
    </Button>
  ) : undefined;

  return (
    <SectionCard
      title={copy.title}
      titleBadge={
        state ? (
          <StatusBadge
            tone={state.enabled ? "green" : "gray"}
            label={state.enabled ? copy.enabled : copy.disabled}
          />
        ) : undefined
      }
      description={copy.description}
      leading={leading}
      headingLevel={headingLevel}
      actions={action}
      loading={state === null && !loadError}
      loadingRows={1}
    >
      <div ref={formRef} className="space-y-4">
        <LoadErrorAlert error={loadError} />
        <FormErrorAlert message={formErrors.error} />
        {state &&
        step.kind === "idle" &&
        !state.enabled &&
        state.available === false ? (
          <Alert type="info" message={copy.unavailable} />
        ) : null}
        {state?.enabled && step.kind === "idle" && state.maskedEmail ? (
          <DataGrid>
            <DataField label={copy.codeGoesTo} fullWidth>
              {state.maskedEmail}
            </DataField>
          </DataGrid>
        ) : null}
        {state && step.kind === "idle" && renderIdle
          ? renderIdle(state.enabled, state.available !== false)
          : null}
        {step.kind === "reauthenticate" && (
          <Button
            type="button"
            variant="primary"
            size="md"
            tooltip={copy.reauthenticateTooltip}
            onClick={() => void logout()}
          >
            {copy.reauthenticate}
          </Button>
        )}
        {step.kind === "code" && (
          <div className="space-y-3">
            <p className="text-sm text-gray-600">
              {copy.sentTo(step.maskedEmail)}
            </p>
            <div className="max-w-xs">
              <Input
                label={copy.codeLabel}
                name="code"
                value={code}
                onChange={(event) =>
                  setCode(event.target.value.replace(/\D/g, ""))
                }
                error={formErrors.fieldError("code")}
                inputMode="numeric"
                autoComplete="one-time-code"
                maxLength={CODE_LENGTH}
                controlSize="compact"
                className="font-mono tracking-[0.4em]"
              />
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="primary"
                size="md"
                disabled={busy || code.trim().length !== CODE_LENGTH}
                tooltip={copy.confirmTooltip}
                onClick={() => void confirm()}
              >
                {copy.confirm}
              </Button>
              <Button
                type="button"
                variant="outline"
                size="md"
                disabled={busy}
                tooltip={copy.resendTooltip}
                onClick={() => void start()}
              >
                {copy.resend}
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="md"
                disabled={busy}
                noTooltip="Bricht nur ab, ohne Folgen."
                onClick={reset}
              >
                {copy.cancel}
              </Button>
            </div>
          </div>
        )}
        {step.kind === "disable" && (
          <div className="space-y-3">
            <p className="text-sm text-gray-600">{copy.disableIntro}</p>
            <div className="max-w-sm">
              <Input
                label={copy.currentPassword}
                name="current_password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                error={formErrors.fieldError("current_password")}
                controlSize="compact"
              />
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="danger"
                size="md"
                disabled={busy || password.length === 0}
                tooltip={copy.disableTooltip}
                onClick={() => void disable()}
              >
                {copy.switchOff}
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="md"
                disabled={busy}
                noTooltip="Bricht nur ab, ohne Folgen."
                onClick={reset}
              >
                {copy.cancel}
              </Button>
            </div>
          </div>
        )}
      </div>
    </SectionCard>
  );
}
