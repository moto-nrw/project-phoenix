"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import Image from "next/image";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { updateProfile, uploadAvatar } from "~/lib/profile-api";
import type { ProfileUpdateRequest } from "~/lib/profile-helpers";
import { Loading } from "~/components/ui/loading";
import { useProfile } from "~/lib/profile-context";
import { compressAvatar } from "~/lib/image-utils";
import { SettingsLayout } from "~/components/shared/settings-layout";

const logger = createLogger({ component: "SettingsPage" });

function SettingsContent() {
  const { data: session, status } = useSession({ required: true });
  const { success: toastSuccess } = useToast();
  const { profile, updateProfileData, refreshProfile } = useProfile();

  const [showAlert, setShowAlert] = useState(false);
  const [alertMessage, setAlertMessage] = useState("");
  const [alertType, setAlertType] = useState<"success" | "error">("success");

  // Profile editing state
  const [isEditing, setIsEditing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [formData, setFormData] = useState({
    firstName: "",
    lastName: "",
    email: "",
  });

  const handleAlertClose = useCallback(() => {
    setShowAlert(false);
  }, []);

  // Sync formData with profile from Context
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

      // Update profile in Context (optimistic update)
      updateProfileData({
        firstName: formData.firstName,
        lastName: formData.lastName,
      });

      // Refresh from backend to ensure consistency
      await refreshProfile(true);

      setIsEditing(false);
      toastSuccess("Profil erfolgreich aktualisiert");
    } catch (err) {
      logger.error("profile_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setAlertMessage("Fehler beim Speichern des Profils");
      setAlertType("error");
      setShowAlert(true);
    } finally {
      setIsSaving(false);
    }
  };

  const handleAvatarChange = async (file: File) => {
    if (!session?.user?.token) return;

    try {
      setIsSaving(true);

      // 1. Compress image in browser before upload
      const compressedFile = await compressAvatar(file);

      // 2. Upload compressed file (much faster!)
      await uploadAvatar(compressedFile);

      // 3. Refresh profile from backend to get new avatar URL
      await refreshProfile(true);

      toastSuccess("Profilbild erfolgreich aktualisiert");
    } catch (err) {
      logger.error("avatar_upload_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setAlertMessage("Fehler beim Hochladen des Profilbilds");
      setAlertType("error");
      setShowAlert(true);
    } finally {
      setIsSaving(false);
    }
  };

  if (status === "loading") {
    return <Loading fullPage={false} />;
  }

  if (!session?.user) {
    redirect("/");
  }

  const profileTab = (
    <div className="space-y-6">
      {/* Avatar Section */}
      <div className="mb-8">
        <div className="inline-block">
          <div className="flex flex-col items-center">
            <div className="group relative">
              <div className="relative flex h-32 w-32 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-gray-700 to-gray-900 text-white shadow-xl">
                {profile?.avatar ? (
                  <Image
                    src={profile.avatar}
                    alt="Profile"
                    fill
                    className="object-cover"
                    sizes="128px"
                    priority
                    unoptimized
                  />
                ) : (
                  <span className="text-4xl font-bold">
                    {(formData.firstName?.charAt(0) || "") +
                      (formData.lastName?.charAt(0) || "")}
                  </span>
                )}
              </div>
              <label
                htmlFor="avatar-upload"
                aria-label="Profilbild ändern"
                className="absolute inset-0 flex cursor-pointer items-center justify-center rounded-full bg-black/50 opacity-0 transition-opacity group-hover:opacity-100"
              >
                <svg
                  className="h-8 w-8 text-white"
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
        </div>
      </div>

      {/* Profile Form */}
      <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm md:p-6">
        <div className="space-y-4">
          <div>
            <label
              htmlFor="settings-firstname"
              className="mb-2 block text-sm font-medium text-gray-700"
            >
              Vorname
            </label>
            <input
              id="settings-firstname"
              type="text"
              value={formData.firstName}
              onChange={(e) =>
                setFormData({ ...formData, firstName: e.target.value })
              }
              disabled={!isEditing}
              className="w-full rounded-lg border border-gray-200 px-4 py-3 text-base transition-all focus:ring-2 focus:ring-[#5080D8] focus:outline-none disabled:bg-gray-50 disabled:text-gray-500"
            />
          </div>
          <div>
            <label
              htmlFor="settings-lastname"
              className="mb-2 block text-sm font-medium text-gray-700"
            >
              Nachname
            </label>
            <input
              id="settings-lastname"
              type="text"
              value={formData.lastName}
              onChange={(e) =>
                setFormData({ ...formData, lastName: e.target.value })
              }
              disabled={!isEditing}
              className="w-full rounded-lg border border-gray-200 px-4 py-3 text-base transition-all focus:ring-2 focus:ring-[#5080D8] focus:outline-none disabled:bg-gray-50 disabled:text-gray-500"
            />
          </div>
          <div>
            <label
              htmlFor="settings-email"
              className="mb-2 block text-sm font-medium text-gray-700"
            >
              E-Mail
            </label>
            <input
              id="settings-email"
              type="email"
              value={formData.email}
              disabled
              className="w-full rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-base text-gray-500"
            />
          </div>
        </div>
      </div>

      {/* Action Buttons */}
      <div className="flex gap-3">
        {isEditing ? (
          <>
            <button
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
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100"
            >
              Abbrechen
            </button>
            <button
              onClick={() => void handleSaveProfile()}
              disabled={isSaving}
              className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-gray-700 hover:shadow-lg active:scale-100 disabled:opacity-50 disabled:hover:scale-100"
            >
              {isSaving ? "Speichern..." : "Speichern"}
            </button>
          </>
        ) : (
          <button
            onClick={() => setIsEditing(true)}
            className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-gray-700 hover:shadow-lg active:scale-100"
          >
            Bearbeiten
          </button>
        )}
      </div>
    </div>
  );

  const mobileProfileCard = (
    <>
      <div className="relative flex h-16 w-16 flex-shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-gray-700 to-gray-900 text-white">
        {profile?.avatar ? (
          <Image
            src={profile.avatar}
            alt="Profile"
            fill
            className="object-cover"
            sizes="64px"
            unoptimized
          />
        ) : (
          <span className="text-xl font-bold">
            {(formData.firstName?.charAt(0) || "") +
              (formData.lastName?.charAt(0) || "")}
          </span>
        )}
      </div>
      <div className="flex-1 text-left">
        <p className="text-lg font-semibold text-gray-900">
          {formData.firstName} {formData.lastName}
        </p>
        <p className="text-sm text-gray-500">Profil, Profilbild</p>
      </div>
    </>
  );

  return (
    <SettingsLayout
      profileTab={profileTab}
      mobileProfileCard={mobileProfileCard}
      onPasswordSuccess={() => {
        toastSuccess("Passwort erfolgreich geändert");
      }}
      alert={
        showAlert && alertType !== "success"
          ? {
              show: true,
              type: alertType,
              message: alertMessage,
              onClose: handleAlertClose,
            }
          : undefined
      }
    />
  );
}

export default function SettingsPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <SettingsContent />
    </Suspense>
  );
}
