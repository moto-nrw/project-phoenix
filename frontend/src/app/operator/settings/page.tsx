"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import { useSession } from "next-auth/react";
import { Pencil } from "lucide-react";
import { Avatar } from "~/components/ui/avatar";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { useFormError } from "~/components/ui/form-error";
import { Input } from "~/components/ui/input";
import { PasswordChangeModal } from "~/components/ui/password-change-modal";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import {
  FormSkeleton,
  PageHeaderSkeleton,
  SkeletonRegion,
} from "~/components/ui/page-skeletons";
import { SectionCard } from "~/components/ui/section-card";
import { useToast } from "~/contexts/ToastContext";
import { sessionFetch } from "~/lib/session-cache";
import { TrustedDevicesSection } from "~/components/settings/trusted-devices-section";
import { PasskeySettingsSection } from "~/components/settings/passkey-settings-section";

interface OperatorProfile {
  id: number;
  email: string;
  display_name: string;
}

function isEmail(value: string | null | undefined): value is string {
  return typeof value === "string" && value.includes("@");
}

const operatorSettingsLoadingFallback = (
  <SkeletonRegion
    label="Profileinstellungen werden geladen"
    className="-mt-1.5 w-full"
  >
    <PageHeaderSkeleton search={false} />
    <div className="mx-auto max-w-2xl px-4 pb-8 md:px-6">
      <FormSkeleton fields={2} />
    </div>
  </SkeletonRegion>
);

function OperatorSettingsContent() {
  const { data: session, status, update: updateSession } = useSession();
  const { success: toastSuccess, error: toastError } = useToast();
  const sessionName = session?.user?.name ?? "";
  const sessionEmail = session?.user?.email ?? "";

  const [showPasswordModal, setShowPasswordModal] = useState(false);

  const [isEditing, setIsEditing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [formData, setFormData] = useState({
    displayName: "",
    email: "",
  });
  const [profileData, setProfileData] = useState<OperatorProfile | null>(null);

  // E-Mail-Wechsel: eigener Ablauf mit Passwortabfrage und Bestätigungslink,
  // darum ein eigener Dialog, der unabhängig vom Bearbeiten-Zustand der
  // Stammdaten erreichbar ist (#3117).
  const [showEmailChangeDialog, setShowEmailChangeDialog] = useState(false);
  const [emailChangeError, setEmailChangeError] = useFormError();
  const [emailChangeLoading, setEmailChangeLoading] = useState(false);
  const [emailChangeNewEmail, setEmailChangeNewEmail] = useState("");
  const [emailChangePassword, setEmailChangePassword] = useState("");

  const handleClosePasswordModal = useCallback(() => {
    setShowPasswordModal(false);
  }, []);

  useEffect(() => {
    if (status !== "authenticated") {
      return;
    }

    let ignore = false;
    const sessionFallback = {
      displayName: sessionName,
      email: isEmail(sessionEmail) ? sessionEmail : "",
    };

    setFormData(sessionFallback);

    const loadProfile = async () => {
      try {
        const response = await sessionFetch("/api/operator/profile", {
          method: "GET",
        });
        if (!response.ok) return;

        const data = (await response.json()) as { data?: OperatorProfile };
        const profile = data.data;
        if (!profile || ignore) return;

        setProfileData(profile);
        setFormData({
          displayName: profile.display_name,
          email: profile.email,
        });
      } catch {
        // Keep the safe session fallback. The profile form should not surface
        // token subjects like "operator:2" as an email address.
      }
    };

    void loadProfile();

    return () => {
      ignore = true;
    };
  }, [status, sessionName, sessionEmail]);

  const resetFormFromProfile = useCallback(() => {
    if (profileData) {
      setFormData({
        displayName: profileData.display_name,
        email: profileData.email,
      });
    } else if (status === "authenticated") {
      setFormData({
        displayName: sessionName,
        email: isEmail(sessionEmail) ? sessionEmail : "",
      });
    }
  }, [profileData, status, sessionName, sessionEmail]);

  const handleSaveProfile = async () => {
    setIsSaving(true);
    try {
      const response = await sessionFetch("/api/operator/profile", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ display_name: formData.displayName }),
      });

      if (!response.ok) {
        const data = (await response.json()) as { error?: string };
        throw new Error(
          data.error ?? "Profil konnte nicht aktualisiert werden",
        );
      }

      const data = (await response.json()) as { data?: OperatorProfile };
      const updatedProfile = data.data;
      const nextDisplayName =
        updatedProfile?.display_name ?? formData.displayName;
      const nextEmail = updatedProfile?.email ?? formData.email;

      if (updatedProfile) {
        setProfileData(updatedProfile);
      }
      setFormData({
        displayName: nextDisplayName,
        email: nextEmail,
      });

      await updateSession({ name: nextDisplayName, email: nextEmail });

      setIsEditing(false);
      toastSuccess("Profil erfolgreich aktualisiert", { duration: 3000 });
    } catch {
      toastError("Fehler beim Speichern des Profils", { duration: 3000 });
    } finally {
      setIsSaving(false);
    }
  };

  const handleCloseEmailDialog = () => {
    setShowEmailChangeDialog(false);
    setEmailChangeError(null);
    setEmailChangeNewEmail("");
    setEmailChangePassword("");
  };

  const handleEmailChange = async () => {
    if (!emailChangeNewEmail || !emailChangePassword) return;
    setEmailChangeLoading(true);
    setEmailChangeError(null);
    try {
      const response = await sessionFetch(
        "/api/operator/profile/email-change",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            new_email: emailChangeNewEmail,
            current_password: emailChangePassword,
          }),
        },
      );

      const data = (await response.json()) as {
        error?: string;
        message?: string;
      };
      if (!response.ok) {
        const errorMsg =
          data.error ??
          data.message ??
          "Ein unbekannter Fehler ist aufgetreten.";
        setEmailChangeError(
          errorMsg.includes("aktuelle Passwort ist falsch")
            ? "Falsches Passwort"
            : errorMsg,
        );
        return;
      }

      const confirmedEmail = emailChangeNewEmail;
      handleCloseEmailDialog();
      toastSuccess(
        `Eine Bestätigungs-E-Mail wird an ${confirmedEmail} gesendet. Bitte überprüfe dein Postfach.`,
        { duration: 3000 },
      );
    } catch {
      setEmailChangeError("Ein Netzwerkfehler ist aufgetreten.");
    } finally {
      setEmailChangeLoading(false);
    }
  };

  if (status === "loading") {
    return operatorSettingsLoadingFallback;
  }

  const emailChangeReady =
    !emailChangeLoading && !!emailChangeNewEmail && !!emailChangePassword;

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Profil" concept="settings" />

      <div className="mx-auto max-w-2xl space-y-6 px-4 pb-8 md:px-6">
        <div className="flex flex-col items-center pt-4">
          <Avatar name={formData.displayName} size="xl" />
        </div>

        <SectionCard
          title="Persönliche Daten"
          actions={
            isEditing ? undefined : (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                onClick={() => setIsEditing(true)}
              >
                <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
                Bearbeiten
              </Button>
            )
          }
        >
          {isEditing ? (
            <div className="space-y-4">
              <Input
                label="Anzeigename"
                name="settings-displayname"
                type="text"
                value={formData.displayName}
                onChange={(e) =>
                  setFormData({ ...formData, displayName: e.target.value })
                }
                maxLength={255}
              />
              <EditActions
                onCancel={() => {
                  setIsEditing(false);
                  resetFormFromProfile();
                }}
                onSave={() => void handleSaveProfile()}
                saving={isSaving}
              />
            </div>
          ) : (
            <DataGrid>
              <DataField label="Anzeigename" fullWidth>
                {formData.displayName || "–"}
              </DataField>
            </DataGrid>
          )}
        </SectionCard>

        <SectionCard
          title="E-Mail-Adresse"
          description="Die neue Adresse gilt erst, nachdem Sie den Link in der Bestätigungs-E-Mail geöffnet haben."
          actions={
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => setShowEmailChangeDialog(true)}
            >
              E-Mail ändern
            </Button>
          }
        >
          <DataGrid>
            <DataField label="E-Mail" fullWidth>
              {formData.email || "–"}
            </DataField>
          </DataGrid>
        </SectionCard>

        <SectionCard
          title="Passwort"
          description="Aktualisieren Sie Ihr Passwort regelmäßig für zusätzliche Sicherheit."
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

        <PasskeySettingsSection scope="operator" />
        <TrustedDevicesSection scope="operator" />
      </div>

      {showPasswordModal && (
        <PasswordChangeModal
          isOpen={showPasswordModal}
          onClose={handleClosePasswordModal}
          apiEndpoint="/api/operator/profile/password"
          onSuccess={() => {
            handleClosePasswordModal();
            toastSuccess("Passwort erfolgreich geändert", { duration: 3000 });
          }}
        />
      )}

      <FormModal
        isOpen={showEmailChangeDialog}
        onClose={handleCloseEmailDialog}
        title="E-Mail-Adresse ändern"
        size="sm"
        mobilePosition="center"
        closeDisabled={emailChangeLoading}
        error={emailChangeError}
        footer={
          <>
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={handleCloseEmailDialog}
              disabled={emailChangeLoading}
            >
              Abbrechen
            </Button>
            <Button
              form="operator-email-change-form"
              type="submit"
              variant="primary"
              size="md"
              disabled={!emailChangeReady}
            >
              {emailChangeLoading
                ? "Wird gesendet..."
                : "E-Mail-Änderung anfordern"}
            </Button>
          </>
        }
      >
        <form
          id="operator-email-change-form"
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            void handleEmailChange();
          }}
        >
          <Input
            label="Neue E-Mail-Adresse"
            name="new-email"
            type="email"
            required
            autoFocus
            autoComplete="email"
            pattern="[A-Za-z0-9._+%\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]+"
            title="Bitte geben Sie eine gültige E-Mail-Adresse ein (z.B. name@beispiel.de)"
            value={emailChangeNewEmail}
            onChange={(e) => setEmailChangeNewEmail(e.target.value)}
            placeholder="neue@email.de"
          />
          <Input
            label="Aktuelles Passwort"
            name="confirm-password"
            type="password"
            required
            autoComplete="current-password"
            value={emailChangePassword}
            onChange={(e) => setEmailChangePassword(e.target.value)}
          />
        </form>
      </FormModal>
    </div>
  );
}

export default function OperatorSettingsPage() {
  return (
    <Suspense fallback={operatorSettingsLoadingFallback}>
      <OperatorSettingsContent />
    </Suspense>
  );
}
