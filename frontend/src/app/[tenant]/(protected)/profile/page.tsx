"use client";

import {
  Suspense,
  useState,
  useEffect,
  useCallback,
  useLayoutEffect,
  useRef,
} from "react";
import { useSession } from "next-auth/react";
import { hasSessionAccess } from "~/lib/session-access";
import { redirect } from "next/navigation";
import { Camera, Pencil } from "lucide-react";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { updateProfile, uploadAvatar } from "~/lib/profile-api";
import type { ProfileUpdateRequest } from "~/lib/profile-helpers";
import { useProfile } from "~/lib/profile-context";
import { compressAvatar } from "~/lib/image-utils";
import { Avatar } from "~/components/ui/avatar";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { Input } from "~/components/ui/input";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { TenantPasswordChangeModal } from "~/components/auth/tenant-password-change-modal";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { TrustedDevicesSection } from "~/components/settings/trusted-devices-section";
import { PasskeySettingsSection } from "~/components/settings/passkey-settings-section";
import { NotificationPreferencesSection } from "~/components/settings/notification-preferences-section";
import { TenantAnalyticsConsentSection } from "~/components/settings/analytics-consent-section";
import { MessageCountSection } from "~/components/settings/message-count-section";
import { BirthdayVisibilitySection } from "~/components/settings/birthday-visibility-section";
import { PushNotificationSection } from "~/components/settings/push-notification-section";
import { OwnQualificationsSection } from "~/components/settings/own-qualifications-section";

const logger = createLogger({ component: "ProfilePage" });

// Der Ladezustand kommt aus dem Seitengeruest, nicht aus einem eigenen
// Skelett: dieselbe Kopfkarte und dieselben Platzhalterflaechen wie auf jeder
// anderen Flaeche des Portals. Der Vorlesetext benennt die Profilseite.
const profileLoadingFallback = (
  <TenantPage
    title="Profil"
    statsLoading
    loading
    loadingLabel="Profil wird geladen…"
  />
);

function ProfileContent() {
  const {
    data: session,
    status,
    update: updateSession,
  } = useSession({
    required: true,
  });
  const { success: toastSuccess } = useToast();
  const { profile, updateProfileData, refreshProfile } = useProfile();
  const [isSaving, setIsSaving] = useState(false);
  const profileAfterWrite = useRefreshAfterWrite(
    "die Anzeige des Profils",
    async (options: { sessionName?: string } = {}) => {
      setIsSaving(true);
      try {
        await refreshProfile(true);
        if ("sessionName" in options) {
          await updateSession({ name: options.sessionName });
        }
      } finally {
        setIsSaving(false);
      }
    },
  );

  const [isEditing, setIsEditing] = useState(false);
  // Speicherfehler stehen oben im Bearbeiten-Bereich der Karte, nicht als
  // Toast: Bauart 2 Regel 5. Katalogtext und Wiederholen (#2517).
  const formRef = useRef<HTMLDivElement>(null);
  const avatarInputRef = useRef<HTMLInputElement>(null);
  const formErrors = useApiFormError(formRef);
  // Das Profilbild wird ohne Formular gewechselt: ein Fehler kommt als Toast.
  const { show: showAvatarError } = useApiErrorDisplay();
  const latestSaveRef = useRef<() => void>(() => undefined);
  const latestAvatarRef = useRef<(file: File) => void>(() => undefined);
  const [showPasswordModal, setShowPasswordModal] = useState(false);
  const [formData, setFormData] = useState({
    firstName: "",
    lastName: "",
    email: "",
  });

  const resetFormFromProfile = useCallback(() => {
    if (profile) {
      setFormData({
        firstName: profile.firstName || "",
        lastName: profile.lastName || "",
        email: profile.email || "",
      });
    }
  }, [profile]);

  useEffect(() => {
    resetFormFromProfile();
  }, [resetFormFromProfile]);

  const handleSaveProfile = async () => {
    if (!hasSessionAccess(session) || !profile || isSaving) return;

    setIsSaving(true);
    formErrors.clear();
    try {
      const updateData: ProfileUpdateRequest = {
        firstName: formData.firstName,
        lastName: formData.lastName,
      };

      await updateProfile(updateData);
    } catch (err) {
      logger.error("profile_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, {
        object: "das Profil",
        retry: () => latestSaveRef.current(),
      });
      setIsSaving(false);
      return;
    }

    updateProfileData({
      firstName: formData.firstName,
      lastName: formData.lastName,
    });
    setIsEditing(false);
    toastSuccess("Ihr Profil ist gespeichert.");
    // Profil und Sitzung laden nach dem Speichern; ein Fehler dabei ist ein
    // Ladefehler, kein Fehler des Speicherns (#6652).
    await profileAfterWrite.refresh({
      sessionName:
        `${formData.firstName} ${formData.lastName}`.trim() || undefined,
    });
  };

  const handleAvatarChange = async (file: File) => {
    if (!hasSessionAccess(session) || isSaving) return;

    try {
      setIsSaving(true);
      const compressedFile = await compressAvatar(file);
      await uploadAvatar(compressedFile);
    } catch (err) {
      logger.error("avatar_upload_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showAvatarError(err, {
        object: "das Profilbild",
        retry: () => latestAvatarRef.current(file),
      });
      setIsSaving(false);
      return;
    }
    toastSuccess("Ihr Profilbild ist gespeichert.");
    await profileAfterWrite.refresh();
  };

  useLayoutEffect(() => {
    latestSaveRef.current = () => void handleSaveProfile();
    latestAvatarRef.current = (file) => void handleAvatarChange(file);
  });

  const handleClosePasswordModal = useCallback(() => {
    setShowPasswordModal(false);
  }, []);

  if (status === "loading") {
    return profileLoadingFallback;
  }

  if (!session?.user) {
    redirect("/");
  }

  const fullName = `${formData.firstName} ${formData.lastName}`.trim();

  return (
    <TenantPage
      title="Profil"
      stats={[fullName, formData.email].filter(Boolean).join(" · ")}
      // Das Profilbild sitzt im Kopf der Seite, wie der Avatar auf jeder
      // Detailseite. Ändern ist eine Aktion im Kopf: das frühere Hover-Overlay
      // gab es auf Touch-Geräten gar nicht (#5599).
      leading={<Avatar imageUrl={profile?.avatar} name={fullName} size="lg" />}
      actions={
        <>
          <input
            ref={avatarInputRef}
            id="avatar-upload"
            type="file"
            accept="image/*"
            aria-label="Profilbild auswählen"
            disabled={isSaving}
            hidden
            onChange={(e) => {
              const file = e.target.files?.[0];
              e.target.value = "";
              if (file) void handleAvatarChange(file);
            }}
          />
          <Button
            type="button"
            variant="outline"
            size="md"
            className="gap-2"
            onClick={() => avatarInputRef.current?.click()}
            disabled={isSaving}
            tooltip="Ersetzt Ihr Profilbild sofort durch ein Bild von Ihrem Gerät."
          >
            <Camera className="h-4 w-4" aria-hidden="true" />
            Profilbild ändern
          </Button>
        </>
      }
    >
      <LoadErrorAlert error={profileAfterWrite.error} />
      <SectionCard
        title="Persönliche Daten"
        actions={
          isEditing ? undefined : (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => setIsEditing(true)}
              disabled={isSaving}
              className="gap-1.5"
              tooltip="Zeigt Felder, um Vor- und Nachnamen zu ändern."
            >
              <Pencil className="h-3.5 w-3.5" />
              Bearbeiten
            </Button>
          )
        }
      >
        {isEditing ? (
          <div ref={formRef} className="space-y-4">
            <FormErrorAlert message={formErrors.error} />
            <Input
              label="Vorname"
              name="first_name"
              type="text"
              value={formData.firstName}
              onChange={(e) =>
                setFormData({ ...formData, firstName: e.target.value })
              }
              error={formErrors.fieldError("first_name")}
              maxLength={255}
            />
            <Input
              label="Nachname"
              name="last_name"
              type="text"
              value={formData.lastName}
              onChange={(e) =>
                setFormData({ ...formData, lastName: e.target.value })
              }
              error={formErrors.fieldError("last_name")}
              maxLength={255}
            />
            <DataGrid>
              <DataField label="E-Mail" fullWidth>
                {formData.email || "–"}
              </DataField>
            </DataGrid>
            <div className="flex justify-end gap-3 pt-1">
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={() => {
                  setIsEditing(false);
                  formErrors.clear();
                  resetFormFromProfile();
                }}
                tooltip="Verwirft Ihre Änderungen am Namen."
              >
                Abbrechen
              </Button>
              <Button
                type="button"
                variant="primary"
                size="md"
                isLoading={isSaving}
                loadingText="Speichern…"
                onClick={() => void handleSaveProfile()}
                noTooltip="Speichert nur Vor- und Nachnamen, ohne weitere Folgen."
              >
                Speichern
              </Button>
            </div>
          </div>
        ) : (
          <DataGrid>
            <DataField label="Vorname">{formData.firstName || "–"}</DataField>
            <DataField label="Nachname">{formData.lastName || "–"}</DataField>
            <DataField label="E-Mail" fullWidth>
              {formData.email || "–"}
            </DataField>
          </DataGrid>
        )}
      </SectionCard>

      <SectionCard
        title="Passwort"
        actions={
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => setShowPasswordModal(true)}
            tooltip="Öffnet ein Fenster, um Ihr Passwort zu ändern."
          >
            Ändern
          </Button>
        }
      />

      {/* Eigene Nachweise wie Erste Hilfe am Kind, mit Ablauf (#4808). */}
      <OwnQualificationsSection />

      {/* „Was" vor „wo": erst die Themen, dann das Gerät. */}
      <NotificationPreferencesSection />
      {/* Welche Elternnachrichten die eigene Zahl bei „Nachrichten" zählt
          (#3673). Steht bei den Benachrichtigungen, weil beides nur die
          eigene Person betrifft. */}
      <MessageCountSection />
      {/* Persönliche Geburtstagsanzeige (#1542) — steht bei den anderen
          Sichtbarkeits- und Benachrichtigungsentscheidungen des eigenen Kontos. */}
      <BirthdayVisibilitySection />
      <PushNotificationSection />
      <PasskeySettingsSection />
      <TrustedDevicesSection />
      {/* Eigene Einwilligung zur Nutzungsanalyse (#4901), mit Hinweis auf
          Aufzeichnungen, solange die Schule die Analyse-Freigabe erteilt hat. */}
      <TenantAnalyticsConsentSection />

      {showPasswordModal && (
        <TenantPasswordChangeModal
          isOpen={showPasswordModal}
          onClose={handleClosePasswordModal}
          onSuccess={() => {
            handleClosePasswordModal();
            toastSuccess("Ihr Passwort ist geändert.");
          }}
        />
      )}
    </TenantPage>
  );
}

export default function ProfilePage() {
  return (
    <Suspense fallback={profileLoadingFallback}>
      <ProfileContent />
    </Suspense>
  );
}
