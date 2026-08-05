"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import Image from "next/image";
import { Camera } from "lucide-react";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { updateProfile, uploadAvatar } from "~/lib/profile-api";
import type { ProfileUpdateRequest } from "~/lib/profile-helpers";
import { Loading } from "~/components/ui/loading";
import { useProfile } from "~/lib/profile-context";
import { compressAvatar } from "~/lib/image-utils";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { PasswordChangeModal } from "~/components/ui/password-change-modal";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { TrustedDevicesSection } from "~/components/settings/trusted-devices-section";
import { PasskeySettingsSection } from "~/components/settings/passkey-settings-section";
import { NotificationPreferencesSection } from "~/components/settings/notification-preferences-section";
import { PushNotificationSection } from "~/components/settings/push-notification-section";
import { getInitials } from "~/lib/format-utils";

const logger = createLogger({ component: "ProfilePage" });

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
    return <Loading fullPage={false} />;
  }

  if (!session?.user) {
    redirect("/");
  }

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Profil" />

      <div className="mx-auto max-w-2xl space-y-6 px-4 pb-8 md:px-6">
        {/* Avatar Section */}
        <div className="flex flex-col items-center pt-4">
          <div className="group relative">
            <div className="bg-moto-green-soft text-moto-green-strong relative flex h-28 w-28 items-center justify-center overflow-hidden rounded-full">
              {profile?.avatar ? (
                <Image
                  src={profile.avatar}
                  alt="Profile"
                  fill
                  className="object-cover"
                  sizes="112px"
                  priority
                  unoptimized
                />
              ) : (
                <span className="text-3xl font-bold">
                  {getInitials(
                    `${formData.firstName} ${formData.lastName}`.trim(),
                  )}
                </span>
              )}
            </div>
            <label
              htmlFor="avatar-upload"
              aria-label="Profilbild ändern"
              className="absolute inset-0 flex cursor-pointer items-center justify-center rounded-full bg-black/50 opacity-0 transition-opacity group-hover:opacity-100"
            >
              <Camera className="h-7 w-7 text-white" />
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
          </div>
          <button
            type="button"
            onClick={() => document.getElementById("avatar-upload")?.click()}
            className="mt-3 text-xs font-medium text-gray-500 transition-colors hover:text-gray-900"
          >
            Profilbild ändern
          </button>
        </div>

        {/* Profile Form */}
        <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm md:p-6">
          <div className="space-y-4">
            <Input
              label="Vorname"
              name="profile-firstname"
              type="text"
              value={formData.firstName}
              onChange={(e) =>
                setFormData({ ...formData, firstName: e.target.value })
              }
              disabled={!isEditing}
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
              disabled={!isEditing}
              maxLength={255}
            />
            <Input
              label="E-Mail"
              name="profile-email"
              type="email"
              value={formData.email}
              disabled
              maxLength={255}
            />
          </div>
        </div>

        {/* Action Buttons */}
        <div className="flex gap-3">
          {isEditing ? (
            <>
              <Button
                type="button"
                variant="outline"
                size="sm"
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
                size="sm"
                isLoading={isSaving}
                loadingText="Speichern..."
                onClick={() => void handleSaveProfile()}
              >
                Speichern
              </Button>
            </>
          ) : (
            <Button
              type="button"
              variant="primary"
              size="sm"
              onClick={() => setIsEditing(true)}
            >
              Bearbeiten
            </Button>
          )}
        </div>

        {/* Security Section */}
        <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm md:p-6">
          <h3 className="mb-3 text-base font-semibold text-gray-900">
            Passwort ändern
          </h3>
          <p className="mb-4 text-sm text-gray-600">
            Aktualisieren Sie Ihr Passwort regelmäßig für zusätzliche
            Sicherheit.
          </p>
          <Button
            type="button"
            variant="primary"
            size="sm"
            onClick={() => setShowPasswordModal(true)}
          >
            Passwort ändern
          </Button>
        </div>

        {/* Trusted Devices Section — personal device management.
            Mirrors the Operator profile page (app/operator/settings/page.tsx). */}
        {/* "Was" before "wo": pick the topics first, then the device. */}
        <NotificationPreferencesSection />
        <PushNotificationSection />
        <PasskeySettingsSection />
        <TrustedDevicesSection />
      </div>

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
    </div>
  );
}

export default function ProfilePage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <ProfileContent />
    </Suspense>
  );
}
