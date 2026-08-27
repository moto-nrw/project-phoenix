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
import { SkeletonRegion, DetailSkeleton } from "~/components/ui/page-skeletons";
import { useProfile } from "~/lib/profile-context";
import { compressAvatar } from "~/lib/image-utils";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { PasswordChangeModal } from "~/components/ui/password-change-modal";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { TrustedDevicesSection } from "~/components/settings/trusted-devices-section";
import { PasskeySettingsSection } from "~/components/settings/passkey-settings-section";
import { NotificationPreferencesSection } from "~/components/settings/notification-preferences-section";
import { BirthdayVisibilitySection } from "~/components/settings/birthday-visibility-section";
import { PushNotificationSection } from "~/components/settings/push-notification-section";
import { getInitials } from "~/lib/format-utils";

const logger = createLogger({ component: "ProfilePage" });

// Der Kopf gehört nicht in den Skeleton: PageHeaderWithSearch rendert sofort,
// nur die Datenregion darunter skeletonisiert.
const profileLoadingFallback = (
  <div className="-mt-1.5 w-full">
    <PageHeaderWithSearch title="Profil" />
    <SkeletonRegion label="Profil wird geladen…">
      <DetailSkeleton sections={3} fieldsPerSection={3} />
    </SkeletonRegion>
  </div>
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

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Profil" />

      <div className="max-w-3xl space-y-6 pb-8">
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
            <button
              type="button"
              aria-label="Profilbild ändern"
              onClick={() => document.getElementById("avatar-upload")?.click()}
              className="absolute -right-1 -bottom-1 flex h-8 w-8 items-center justify-center rounded-full border border-gray-200 bg-white text-gray-600 shadow-sm transition-colors hover:text-gray-900"
            >
              <Camera className="h-4 w-4" />
            </button>
          </div>
        </div>

        {/* Profile Data */}
        <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
          <div className="mb-4 flex items-center justify-between">
            <h3 className="text-base font-semibold text-gray-900">
              Persönliche Daten
            </h3>
            {!isEditing && (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                onClick={() => setIsEditing(true)}
              >
                <Pencil className="h-3.5 w-3.5" />
                Bearbeiten
              </Button>
            )}
          </div>
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
                <span className="text-xs font-medium text-gray-500">
                  E-Mail
                </span>
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
                  loadingText="Speichern…"
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
        </div>

        {/* Security Section */}
        <div className="moto-content-surface flex items-center justify-between rounded-2xl border p-4 shadow-sm sm:p-6">
          <h3 className="text-base font-semibold text-gray-900">Passwort</h3>
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => setShowPasswordModal(true)}
          >
            Ändern
          </Button>
        </div>

        {/* Trusted Devices Section: personal device management.
            Mirrors the Operator profile page (app/operator/settings/page.tsx). */}
        {/* "Was" before "wo": pick the topics first, then the device. */}
        <NotificationPreferencesSection />
        {/* Persönliche Geburtstagsanzeige (#1542): steht bei den anderen
            Sichtbarkeits- und Benachrichtigungsentscheidungen des eigenen Kontos. */}
        <BirthdayVisibilitySection />
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
    <Suspense fallback={profileLoadingFallback}>
      <ProfileContent />
    </Suspense>
  );
}
