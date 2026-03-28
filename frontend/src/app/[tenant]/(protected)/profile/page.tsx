"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { Avatar, Button, Card, Input, Spinner } from "@moto-nrw/design-system";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { updateProfile, uploadAvatar } from "~/lib/profile-api";
import type { ProfileUpdateRequest } from "~/lib/profile-helpers";
import { useProfile } from "~/lib/profile-context";
import { compressAvatar } from "~/lib/image-utils";
import { PasswordChangeModal } from "~/components/ui";
import { PageHeaderWithSearch } from "~/components/ui/page-header";

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

  useEffect(() => {
    if (profile) {
      setFormData({
        firstName: profile.firstName || "",
        lastName: profile.lastName || "",
        email: profile.email || "",
      });
    }
  }, [profile]);

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
    return <Spinner size="lg" label="Laden..." />;
  }

  if (!session?.user) {
    redirect("/");
  }

  const fullName = `${formData.firstName} ${formData.lastName}`.trim();

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Profil" />

      <div className="mx-auto max-w-2xl space-y-6 px-4 pb-8 md:px-6">
        {/* Avatar Section */}
        <div className="flex flex-col items-center pt-4">
          <div className="group relative">
            <Avatar name={fullName} src={profile?.avatar} size="lg" />
            <label
              htmlFor="avatar-upload"
              aria-label="Profilbild ändern"
              className="absolute inset-0 flex cursor-pointer items-center justify-center rounded-full bg-black/50 opacity-0 transition-opacity group-hover:opacity-100"
            >
              <svg
                className="h-7 w-7 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z"
                />
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M15 13a3 3 0 11-6 0 3 3 0 016 0z"
                />
              </svg>
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
            className="mt-3 text-[11px] font-medium text-gray-600 transition-colors hover:text-gray-900"
          >
            Profilbild ändern
          </button>
        </div>

        {/* Profile Form */}
        <Card variant="glass" padding="lg">
          <div className="space-y-4">
            <Input
              label="Vorname"
              name="firstName"
              value={formData.firstName}
              onChange={(e) =>
                setFormData({ ...formData, firstName: e.target.value })
              }
              disabled={!isEditing}
              maxLength={255}
            />
            <Input
              label="Nachname"
              name="lastName"
              value={formData.lastName}
              onChange={(e) =>
                setFormData({ ...formData, lastName: e.target.value })
              }
              disabled={!isEditing}
              maxLength={255}
            />
            <Input
              label="E-Mail"
              name="email"
              type="email"
              value={formData.email}
              disabled
              maxLength={255}
            />
          </div>
        </Card>

        {/* Action Buttons */}
        <div className="flex gap-3">
          {isEditing ? (
            <>
              <Button
                variant="outline"
                size="md"
                type="button"
                onClick={() => {
                  setIsEditing(false);
                  if (profile) {
                    setFormData({
                      firstName: profile.firstName || "",
                      lastName: profile.lastName || "",
                      email: profile.email || "",
                    });
                  }
                }}
              >
                Abbrechen
              </Button>
              <Button
                variant="primary"
                size="md"
                type="button"
                isLoading={isSaving}
                loadingText="Speichern..."
                onClick={() => void handleSaveProfile()}
              >
                Speichern
              </Button>
            </>
          ) : (
            <Button
              variant="primary"
              size="md"
              type="button"
              onClick={() => setIsEditing(true)}
            >
              Bearbeiten
            </Button>
          )}
        </div>

        {/* Security Section */}
        <Card variant="glass" padding="lg">
          <h3 className="mb-3 text-base font-semibold text-gray-900">
            Passwort ändern
          </h3>
          <p className="mb-4 text-sm text-gray-600">
            Aktualisieren Sie Ihr Passwort regelmäßig für zusätzliche
            Sicherheit.
          </p>
          <Button
            variant="primary"
            size="md"
            type="button"
            onClick={() => setShowPasswordModal(true)}
          >
            Passwort ändern
          </Button>
        </Card>
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
    <Suspense fallback={<Spinner size="lg" label="Laden..." />}>
      <ProfileContent />
    </Suspense>
  );
}
