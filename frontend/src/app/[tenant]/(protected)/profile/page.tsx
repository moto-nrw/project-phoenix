"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { Camera } from "lucide-react";
import {
  Avatar,
  Button,
  Card,
  Divider,
  Input,
  Spinner,
} from "@moto-nrw/design-system";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { updateProfile, uploadAvatar } from "~/lib/profile-api";
import type { ProfileUpdateRequest } from "~/lib/profile-helpers";
import { useProfile } from "~/lib/profile-context";
import { compressAvatar } from "~/lib/image-utils";
import { PasswordChangeModal } from "~/components/ui";

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
    <div className="mx-auto max-w-2xl space-y-6 px-4 py-6 md:px-6">
      {/* Avatar Section */}
      <div className="flex flex-col items-center">
        <div className="group relative">
          <Avatar name={fullName || "?"} src={profile?.avatar} size="lg" />
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
          className="text-steel-600 hover:text-steel-900 mt-3 text-[11px] font-medium transition-colors"
        >
          Profilbild ändern
        </button>
      </div>

      {/* Profile Form */}
      <Card variant="glass" padding="lg">
        <div className="space-y-4">
          <Input
            label="Vorname"
            id="profile-firstname"
            value={formData.firstName}
            onChange={(e) =>
              setFormData({ ...formData, firstName: e.target.value })
            }
            disabled={!isEditing}
            maxLength={255}
          />
          <Input
            label="Nachname"
            id="profile-lastname"
            value={formData.lastName}
            onChange={(e) =>
              setFormData({ ...formData, lastName: e.target.value })
            }
            disabled={!isEditing}
            maxLength={255}
          />
          <Input
            label="E-Mail"
            id="profile-email"
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
              onClick={() => void handleSaveProfile()}
              isLoading={isSaving}
              loadingText="Speichern..."
            >
              Speichern
            </Button>
          </>
        ) : (
          <Button variant="primary" onClick={() => setIsEditing(true)}>
            Bearbeiten
          </Button>
        )}
      </div>

      <Divider />

      {/* Security Section */}
      <Card variant="glass" padding="lg">
        <h3 className="text-steel-900 mb-1 text-base font-semibold">
          Passwort ändern
        </h3>
        <p className="text-steel-600 mb-4 text-sm">
          Aktualisieren Sie Ihr Passwort regelmäßig für zusätzliche Sicherheit.
        </p>
        <Button variant="primary" onClick={() => setShowPasswordModal(true)}>
          Passwort ändern
        </Button>
      </Card>

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
