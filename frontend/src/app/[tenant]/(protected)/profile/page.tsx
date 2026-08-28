"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import Image from "next/image";
import { Camera, Pencil } from "lucide-react";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { updateProfile, uploadAvatar } from "~/lib/profile-api";
import type { ProfileUpdateRequest } from "~/lib/profile-helpers";
import { useProfile } from "~/lib/profile-context";
import { compressAvatar } from "~/lib/image-utils";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { PasswordChangeModal } from "~/components/ui/password-change-modal";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { TrustedDevicesSection } from "~/components/settings/trusted-devices-section";
import { PasskeySettingsSection } from "~/components/settings/passkey-settings-section";
import { NotificationPreferencesSection } from "~/components/settings/notification-preferences-section";
import { BirthdayVisibilitySection } from "~/components/settings/birthday-visibility-section";
import { PushNotificationSection } from "~/components/settings/push-notification-section";
import { getInitials } from "~/lib/format-utils";

const logger = createLogger({ component: "ProfilePage" });

// Der Ladezustand kommt aus dem Seitengeruest, nicht aus einem eigenen
// Skelett: dieselbe Kopfkarte und dieselben Platzhalterflaechen wie auf jeder
// anderen Flaeche des Portals. Der Vorlesetext bildet `TenantPage` aus dem
// Titel („Profil wird geladen…").
const profileLoadingFallback = (
  <TenantPage title="Profil" statsLoading loading />
);

function ProfileContent() {
  const {
    data: session,
    status,
    update: updateSession,
  } = useSession({
    required: true,
  });
  const { success: toastSuccess, error: toastError } = useToast();
  const { profile, updateProfileData, refreshProfile } = useProfile();

  const [isEditing, setIsEditing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
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
    if (!session?.user?.token || !profile) return;

    setIsSaving(true);
    try {
      const updateData: ProfileUpdateRequest = {
        firstName: formData.firstName,
        lastName: formData.lastName,
      };

      await updateProfile(updateData);

      updateProfileData({
        firstName: formData.firstName,
        lastName: formData.lastName,
      });

      await refreshProfile(true);

      const newName =
        `${formData.firstName} ${formData.lastName}`.trim() || undefined;
      await updateSession({ name: newName });

      setIsEditing(false);
      toastSuccess("Profil erfolgreich aktualisiert");
    } catch (err) {
      logger.error("profile_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toastError("Fehler beim Speichern des Profils");
    } finally {
      setIsSaving(false);
    }
  };

  const handleAvatarChange = async (file: File) => {
    if (!session?.user?.token) return;

    try {
      setIsSaving(true);
      const compressedFile = await compressAvatar(file);
      await uploadAvatar(compressedFile);
      await refreshProfile(true);
      toastSuccess("Profilbild erfolgreich aktualisiert");
    } catch (err) {
      logger.error("avatar_upload_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toastError("Fehler beim Hochladen des Profilbilds");
    } finally {
      setIsSaving(false);
    }
  };

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
      leading={
        // Das Profilbild sitzt im Kopf der Seite, wie der Avatar auf jeder
        // Detailseite. Vorher stand es freischwebend über der ersten Karte.
        <div className="group relative">
          <div className="bg-moto-green-soft text-moto-green-strong relative flex h-14 w-14 items-center justify-center overflow-hidden rounded-full">
            {profile?.avatar ? (
              <Image
                src={profile.avatar}
                alt="Profilbild"
                fill
                className="object-cover"
                sizes="56px"
                priority
                unoptimized
              />
            ) : (
              <span className="text-lg font-bold">{getInitials(fullName)}</span>
            )}
          </div>
          <label
            htmlFor="avatar-upload"
            aria-label="Profilbild ändern"
            className="absolute inset-0 flex cursor-pointer items-center justify-center rounded-full bg-black/50 opacity-0 transition-opacity group-hover:opacity-100"
          >
            <Camera className="h-5 w-5 text-white" />
          </label>
          <input
            id="avatar-upload"
            type="file"
            accept="image/*"
            onChange={(e) => {
              const file = e.target.files?.[0];
              if (file) void handleAvatarChange(file);
            }}
            className="hidden"
          />
          <button
            type="button"
            aria-label="Profilbild ändern"
            onClick={() => document.getElementById("avatar-upload")?.click()}
            className="absolute -right-1 -bottom-1 flex h-6 w-6 items-center justify-center rounded-full border border-gray-200 bg-white text-gray-600 shadow-sm transition-colors hover:text-gray-900"
          >
            <Camera className="h-3 w-3" />
          </button>
        </div>
      }
    >
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
              <Pencil className="h-3.5 w-3.5" />
              Bearbeiten
            </Button>
          )
        }
      >
        {isEditing ? (
          <div className="space-y-4">
            <Input
              label="Vorname"
              name="profile-firstname"
              type="text"
              value={formData.firstName}
              onChange={(e) =>
                setFormData({ ...formData, firstName: e.target.value })
              }
              maxLength={255}
            />
            <Input
              label="Nachname"
              name="profile-lastname"
              type="text"
              value={formData.lastName}
              onChange={(e) =>
                setFormData({ ...formData, lastName: e.target.value })
              }
              maxLength={255}
            />
            <div>
              <span className="text-xs font-medium text-gray-500">E-Mail</span>
              <p className="text-sm font-medium text-gray-900">
                {formData.email}
              </p>
            </div>
            <div className="flex justify-end gap-3 pt-1">
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={() => {
                  setIsEditing(false);
                  resetFormFromProfile();
                }}
              >
                Abbrechen
              </Button>
              <Button
                type="button"
                variant="primary"
                size="md"
                isLoading={isSaving}
                loadingText="Speichern..."
                onClick={() => void handleSaveProfile()}
              >
                Speichern
              </Button>
            </div>
          </div>
        ) : (
          <dl className="space-y-3">
            <div>
              <dt className="text-xs font-medium text-gray-500">Vorname</dt>
              <dd className="text-sm font-medium text-gray-900">
                {formData.firstName || "–"}
              </dd>
            </div>
            <div>
              <dt className="text-xs font-medium text-gray-500">Nachname</dt>
              <dd className="text-sm font-medium text-gray-900">
                {formData.lastName || "–"}
              </dd>
            </div>
            <div>
              <dt className="text-xs font-medium text-gray-500">E-Mail</dt>
              <dd className="text-sm font-medium text-gray-900">
                {formData.email || "–"}
              </dd>
            </div>
          </dl>
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
          >
            Ändern
          </Button>
        }
      />

      {/* „Was" vor „wo": erst die Themen, dann das Gerät. */}
      <NotificationPreferencesSection />
      {/* Persönliche Geburtstagsanzeige (#1542) — steht bei den anderen
          Sichtbarkeits- und Benachrichtigungsentscheidungen des eigenen Kontos. */}
      <BirthdayVisibilitySection />
      <PushNotificationSection />
      <PasskeySettingsSection />
      <TrustedDevicesSection />

      {showPasswordModal && (
        <PasswordChangeModal
          isOpen={showPasswordModal}
          onClose={handleClosePasswordModal}
          onSuccess={() => {
            handleClosePasswordModal();
            toastSuccess("Passwort erfolgreich geändert");
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
