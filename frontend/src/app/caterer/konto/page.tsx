"use client";

// Konto-Seite des Caterer-Portals (#4496): die Anmelde-Adresse, „Passwort
// ändern“ und die freiwillige Zwei-Faktor-Anmeldung (#4497) mit Passkeys und
// gemerkten Geräten. Jeder Bereich ist eine eigene Karte.

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { useSession } from "next-auth/react";
import { credentialError } from "~/components/auth/credential-error";
import { TenantPasswordChangeModal } from "~/components/auth/tenant-password-change-modal";
import { PasskeySettingsSection } from "~/components/settings/passkey-settings-section";
import { TrustedDevicesSection } from "~/components/settings/trusted-devices-section";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import {
  confirmCatererMFA,
  disableCatererMFA,
  fetchCatererMFAStatus,
  startCatererMFA,
} from "~/lib/caterer-mfa-api";
import { createLogger } from "~/lib/logger";
import { useShellAuth } from "~/lib/shell-auth-context";

const logger = createLogger({ component: "CatererAccountPage" });

// A wrong code and a wrong password come back as 401 with their own code:
// they stay in the card instead of ending the session.
const CODE_REFUSALS = ["identity.mfa_code_invalid"] as const;
const PASSWORD_REFUSALS = ["identity.current_password_wrong"] as const;

type Step =
  | { kind: "idle" }
  | { kind: "code"; challengeToken: string; maskedEmail: string }
  | { kind: "disable" }
  | { kind: "reauthenticate" };

function TwoFactorSection() {
  const { logout } = useShellAuth();
  const toast = useToast();
  const [enabled, setEnabled] = useState<boolean | null>(null);
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
      const status = await fetchCatererMFAStatus();
      setEnabled(status.enabled);
    } catch (err) {
      logger.warn("caterer_mfa_status_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showLoadError(err, {
        object: "die Zwei-Faktor-Anmeldung",
        retry: () => latestLoadRef.current(),
      });
    }
  }, [clear, showLoadError]);

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
      const challenge = await startCatererMFA();
      setCode("");
      setStep({
        kind: "code",
        challengeToken: challenge.challengeToken,
        maskedEmail: challenge.maskedEmail,
      });
    } catch (err) {
      logger.warn("caterer_mfa_start_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, {
        object: "das Senden des Codes",
        retry: () => latestStartRef.current(),
      });
    } finally {
      setBusy(false);
    }
  };

  const confirm = async () => {
    if (step.kind !== "code") return;
    setBusy(true);
    formErrors.clear();
    try {
      const { sessionRenewed } = await confirmCatererMFA(
        step.challengeToken,
        code.trim(),
      );
      // The answer is a new session with the second factor proven, which
      // the BFF installs; the old one would lose access at its next renewal.
      if (!sessionRenewed) {
        logger.error("caterer_mfa_session_seed_failed", {});
        setEnabled(true);
        setCode("");
        setStep({ kind: "reauthenticate" });
        formErrors.invalid(
          "Die Zwei-Faktor-Anmeldung ist eingeschaltet. Die Anmeldung konnte nicht erneuert werden. Bitte melden Sie sich neu an.",
        );
        return;
      }
      setEnabled(true);
      reset();
      toast.success("Die Zwei-Faktor-Anmeldung ist eingeschaltet.");
    } catch (err) {
      logger.warn("caterer_mfa_confirm_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(credentialError(err, CODE_REFUSALS), {
        object: "das Einschalten der Zwei-Faktor-Anmeldung",
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
      await disableCatererMFA(password);
      setEnabled(false);
      reset();
      toast.success("Die Zwei-Faktor-Anmeldung ist ausgeschaltet.");
    } catch (err) {
      logger.warn("caterer_mfa_disable_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(credentialError(err, PASSWORD_REFUSALS), {
        object: "das Ausschalten der Zwei-Faktor-Anmeldung",
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

  const action =
    enabled === null || step.kind !== "idle" ? undefined : (
      <Button
        type="button"
        variant="outline"
        size="md"
        disabled={busy}
        onClick={() => {
          if (enabled) {
            formErrors.clear();
            setStep({ kind: "disable" });
          } else {
            void start();
          }
        }}
      >
        {enabled ? "Ausschalten" : "Einschalten"}
      </Button>
    );

  return (
    <SectionCard
      title="Zwei-Faktor-Anmeldung"
      description="Freiwillig. Ist sie eingeschaltet, geben Sie bei der Anmeldung nach dem Passwort einen Code ein. Den Code schickt moto an Ihre E-Mail-Adresse."
      actions={action}
    >
      <div ref={formRef} className="space-y-3">
        <LoadErrorAlert error={loadError} />
        <FormErrorAlert message={formErrors.error} />
        {enabled !== null && step.kind === "idle" && (
          <DataGrid>
            <DataField label="Status" fullWidth>
              {enabled ? "Eingeschaltet" : "Ausgeschaltet"}
            </DataField>
          </DataGrid>
        )}
        {step.kind === "reauthenticate" && (
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void logout()}
          >
            Neu anmelden
          </Button>
        )}
        {step.kind === "code" && (
          <div className="space-y-3">
            <p className="text-sm text-gray-600">
              Wir haben einen Code an{" "}
              {step.maskedEmail || "Ihre E-Mail-Adresse"} geschickt. Geben Sie
              die 6 Ziffern hier ein.
            </p>
            <Input
              label="Code"
              name="code"
              value={code}
              onChange={(event) => setCode(event.target.value)}
              error={formErrors.fieldError("code")}
              inputMode="numeric"
              autoComplete="one-time-code"
              maxLength={6}
            />
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="primary"
                size="md"
                disabled={busy || code.trim().length !== 6}
                onClick={() => void confirm()}
              >
                Einschalten
              </Button>
              <Button
                type="button"
                variant="outline"
                size="md"
                disabled={busy}
                onClick={() => void start()}
              >
                Neuen Code senden
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="md"
                disabled={busy}
                onClick={reset}
              >
                Abbrechen
              </Button>
            </div>
          </div>
        )}
        {step.kind === "disable" && (
          <div className="space-y-3">
            <p className="text-sm text-gray-600">
              Danach reicht bei der Anmeldung wieder Ihr Passwort. Geben Sie zur
              Sicherheit Ihr aktuelles Passwort ein.
            </p>
            <Input
              label="Aktuelles Passwort"
              name="current_password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              error={formErrors.fieldError("current_password")}
            />
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="primary"
                size="md"
                disabled={busy || password.length === 0}
                onClick={() => void disable()}
              >
                Ausschalten
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="md"
                disabled={busy}
                onClick={reset}
              >
                Abbrechen
              </Button>
            </div>
          </div>
        )}
      </div>
    </SectionCard>
  );
}

export default function CatererAccountPage() {
  const { data: session } = useSession();
  const { logout } = useShellAuth();
  const [showPasswordModal, setShowPasswordModal] = useState(false);

  return (
    <div className="space-y-6">
      <SectionCard
        title="Anmeldung"
        description="Mit dieser E-Mail-Adresse melden Sie sich an."
      >
        <DataGrid>
          <DataField label="E-Mail" fullWidth>
            {session?.user?.email ?? "–"}
          </DataField>
        </DataGrid>
      </SectionCard>

      <SectionCard
        title="Passwort"
        description="Nach dem Ändern melden Sie sich auf allen Geräten neu an."
        actions={
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => setShowPasswordModal(true)}
          >
            Passwort ändern
          </Button>
        }
      />

      <TwoFactorSection />
      <PasskeySettingsSection scope="caterer" />
      <TrustedDevicesSection scope="caterer" />

      <TenantPasswordChangeModal
        isOpen={showPasswordModal}
        onClose={() => setShowPasswordModal(false)}
        apiEndpoint="/api/caterer/account/password"
        // The change ended every session of the account, this one too. If the
        // logout request itself fails, the revoked session still opens
        // nothing; its next request ends it (#5194).
        onSuccess={() => void logout().catch(() => undefined)}
      />
    </div>
  );
}
