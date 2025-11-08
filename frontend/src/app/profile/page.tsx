"use client";

import { useState, useEffect, Suspense } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import Image from "next/image";
import { ResponsiveLayout } from "~/components/dashboard";
import { Input, PasswordChangeModal } from "~/components/ui";
import { Alert } from "~/components/ui/alert";

import { useToast } from "~/contexts/ToastContext";
import { updateProfile, uploadAvatar } from "~/lib/profile-api";
import type { ProfileUpdateRequest } from "~/lib/profile-helpers";
import { Loading } from "~/components/ui/loading";
import { useProfile } from "~/lib/profile-context";
import { compressAvatar } from "~/lib/image-utils";

// Info Card Component
interface InfoCardProps {
  title: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

const InfoCard: React.FC<InfoCardProps> = ({ title, children, className }) => (
  <div
    className={`rounded-lg border border-gray-100 bg-white p-4 shadow-md md:p-6 ${className ?? ""}`}
  >
    <h3 className="mb-4 text-base font-semibold md:text-lg">{title}</h3>
    {children}
  </div>
);

// Avatar Upload Component
interface AvatarUploadProps {
  avatar: string | null;
  firstName: string;
  lastName: string;
  onAvatarChange: (file: File) => void;
}

const AvatarUpload: React.FC<AvatarUploadProps> = ({
  avatar,
  firstName,
  lastName,
  onAvatarChange,
}) => {
  const getInitials = () => {
    const first = firstName?.charAt(0) || "";
    const last = lastName?.charAt(0) || "";
    return (first + last).toUpperCase() || "?";
  };
  const initials = getInitials();

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      onAvatarChange(file);
    }
  };

  return (
    <div className="flex flex-col items-center">
      <div className="group relative">
        <div className="relative flex h-24 w-24 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-blue-500 to-purple-600 text-white shadow-lg md:h-32 md:w-32">
          {avatar ? (
            <Image
              src={avatar}
              alt="Profile"
              fill
              className="object-cover"
              sizes="(max-width: 768px) 96px, 128px"
              priority
              unoptimized
            />
          ) : (
            <span className="text-2xl font-bold md:text-3xl">{initials}</span>
          )}
        </div>
        <label
          htmlFor="avatar-upload"
          className="absolute inset-0 flex cursor-pointer items-center justify-center rounded-full bg-black/50 opacity-0 transition-opacity group-hover:opacity-100"
        >
          <svg
            className="h-6 w-6 text-white"
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
          onChange={handleFileChange}
          className="hidden"
        />
      </div>
      <button
        type="button"
        onClick={() => document.getElementById("avatar-upload")?.click()}
        className="mt-3 text-sm font-medium text-blue-600 hover:text-blue-800"
      >
        Foto ändern
      </button>
    </div>
  );
};

function ProfilePageContent() {
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const { profile, updateProfileData, refreshProfile, isLoading } =
    useProfile();
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { success: toastSuccess } = useToast();

  const [isEditing, setIsEditing] = useState(false);
  const [showPasswordModal, setShowPasswordModal] = useState(false);

  // Form state
  const [formData, setFormData] = useState({
    firstName: "",
    lastName: "",
    bio: "",
    email: "",
    username: "",
  });

  // Sync formData with profile from Context
  useEffect(() => {
    if (profile) {
      setFormData({
        firstName: profile.firstName || "",
        lastName: profile.lastName || "",
        bio: profile.bio ?? "",
        email: profile.email,
        username: profile.username ?? "",
      });
    }
  }, [profile]);

  const handleInputChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>,
  ) => {
    const { name, value } = e.target;
    setFormData((prev) => ({ ...prev, [name]: value }));
  };

  const handleSave = async () => {
    try {
      setIsSaving(true);
      setError(null);

      // If no profile exists yet, firstName and lastName are required
      if (
        (!profile?.firstName || !profile?.lastName) &&
        (!formData.firstName || !formData.lastName)
      ) {
        setError(
          "Vorname und Nachname sind erforderlich, um Ihr Profil zu erstellen.",
        );
        setIsSaving(false);
        return;
      }

      const updateData: ProfileUpdateRequest = {
        firstName: formData.firstName,
        lastName: formData.lastName,
        bio: formData.bio || undefined,
        username: formData.username || undefined,
      };

      await updateProfile(updateData);

      // Update profile in Context (optimistic update)
      updateProfileData({
        firstName: formData.firstName,
        lastName: formData.lastName,
        bio: formData.bio || undefined,
        username: formData.username || undefined,
      });

      // Refresh from backend to ensure consistency
      await refreshProfile(true);

      setIsEditing(false);
      toastSuccess("Profil erfolgreich aktualisiert");
    } catch (err) {
      setError("Fehler beim Speichern des Profils");
      console.error(err);
    } finally {
      setIsSaving(false);
    }
  };

  const handleCancel = () => {
    if (profile) {
      setFormData({
        firstName: profile.firstName,
        lastName: profile.lastName,
        bio: profile.bio ?? "",
        email: profile.email,
        username: profile.username ?? "",
      });
    }
    setIsEditing(false);
  };

  const handleAvatarChange = async (file: File) => {
    setIsSaving(true);
    setError(null);

    try {
      // 1. Compress image in browser before upload
      const compressedFile = await compressAvatar(file);

      // 2. Upload compressed file (much faster!)
      await uploadAvatar(compressedFile);

      // 3. Refresh profile from backend to get new avatar URL
      await refreshProfile(true);

      toastSuccess("Profilbild erfolgreich aktualisiert");
    } catch (err) {
      console.error("Error uploading avatar:", err);
      setError(
        err instanceof Error
          ? err.message
          : "Fehler beim Hochladen des Profilbilds",
      );
    } finally {
      setIsSaving(false);
    }
  };

  if (status === "loading" || isLoading) {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  if (!profile) {
    return (
      <ResponsiveLayout>
        <div className="mx-auto max-w-4xl">
          <Alert type="error" message="Profil konnte nicht geladen werden" />
        </div>
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout>
      <div className="mx-auto max-w-4xl">
        {/* Header */}
        <div className="mb-6 md:mb-8">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h1 className="text-2xl font-bold text-gray-900 md:text-3xl">
                Mein Profil
              </h1>
              <p className="mt-1 text-sm text-gray-600 md:text-base">
                Verwalte deine persönlichen Informationen
              </p>
            </div>
            {/* Desktop edit button */}
            {!isEditing && (
              <button
                onClick={() => setIsEditing(true)}
                className="hidden self-start rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors duration-150 hover:bg-gray-800 focus:ring-2 focus:ring-gray-900 focus:ring-offset-2 focus:outline-none sm:block sm:self-auto"
              >
                Profil bearbeiten
              </button>
            )}
          </div>
        </div>

        {/* Alerts */}
        {error && (
          <div className="mb-6">
            <Alert type="error" message={error} />
          </div>
        )}

        {/* Profile Content */}
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
          {/* Left Column - Avatar and Quick Info */}
          <div className="lg:col-span-1">
            <InfoCard title="Profilbild">
              <AvatarUpload
                avatar={profile.avatar ?? null}
                firstName={formData.firstName}
                lastName={formData.lastName}
                onAvatarChange={handleAvatarChange}
              />

              {/* Account Info */}
              <div className="mt-6 space-y-3">
                <div className="text-center">
                  <p className="text-sm text-gray-500">Mitglied seit</p>
                  <p className="font-medium">
                    {new Date(profile.createdAt).toLocaleDateString("de-DE", {
                      year: "numeric",
                      month: "long",
                      day: "numeric",
                    })}
                  </p>
                </div>
                {profile.lastLogin && (
                  <div className="text-center">
                    <p className="text-sm text-gray-500">Letzte Anmeldung</p>
                    <p className="font-medium">
                      {new Date(profile.lastLogin).toLocaleDateString("de-DE", {
                        year: "numeric",
                        month: "short",
                        day: "numeric",
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </p>
                  </div>
                )}
              </div>
            </InfoCard>

            {/* RFID Wristband Status */}
            {profile.rfidCard && (
              <InfoCard title="RFID-Armband" className="mt-6">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="rounded-lg bg-green-100 p-2">
                      <svg
                        className="h-6 w-6 text-green-600"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                        />
                      </svg>
                    </div>
                    <div>
                      <p className="font-medium">Aktiv</p>
                      <p className="text-xs text-gray-500">
                        ID: {profile.rfidCard}
                      </p>
                    </div>
                  </div>
                </div>
              </InfoCard>
            )}
          </div>

          {/* Right Column - Personal Information */}
          <div className="lg:col-span-2">
            <InfoCard
              title={
                <div className="flex items-center justify-between">
                  <span>Persönliche Informationen</span>
                  {!isEditing && (
                    <button
                      onClick={() => setIsEditing(true)}
                      className="-mt-1 -mr-2 touch-manipulation rounded-lg p-2 text-gray-400 transition-all duration-150 hover:bg-gray-50 hover:text-gray-600 sm:hidden"
                      aria-label="Bearbeiten"
                    >
                      <svg
                        className="h-5 w-5"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={1.5}
                          d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"
                        />
                      </svg>
                    </button>
                  )}
                </div>
              }
            >
              {(!profile.firstName || !profile.lastName) && (
                <div className="mb-4 rounded-lg border border-blue-200 bg-blue-50 p-3">
                  <p className="text-sm text-blue-800">
                    <span className="font-medium">Profil unvollständig:</span>{" "}
                    Bitte vervollständigen Sie Ihren Vor- und Nachnamen.
                  </p>
                </div>
              )}
              <form className="space-y-4">
                <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                  <Input
                    label="Vorname"
                    name="firstName"
                    value={formData.firstName}
                    onChange={handleInputChange}
                    disabled={!isEditing}
                    required
                  />
                  <Input
                    label="Nachname"
                    name="lastName"
                    value={formData.lastName}
                    onChange={handleInputChange}
                    disabled={!isEditing}
                    required
                  />
                </div>

                <div>
                  <Input
                    label="E-Mail"
                    name="email"
                    type="email"
                    value={formData.email}
                    disabled
                  />
                </div>

                <Input
                  label="Benutzername"
                  name="username"
                  value={formData.username}
                  onChange={handleInputChange}
                  disabled={!isEditing}
                  placeholder="Optional"
                />

                <div>
                  <label className="mb-2 block text-sm font-medium text-gray-700">
                    Über mich
                  </label>
                  <textarea
                    name="bio"
                    value={formData.bio}
                    onChange={handleInputChange}
                    disabled={!isEditing}
                    rows={4}
                    maxLength={500}
                    className={`block w-full resize-none rounded-lg border-0 bg-white px-4 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:ring-2 focus:ring-gray-900 focus:ring-inset disabled:bg-gray-50 disabled:text-gray-500 disabled:ring-gray-200`}
                    placeholder="Erzähle etwas über dich..."
                  />
                </div>

                {/* Edit Actions */}
                {isEditing && (
                  <div className="flex flex-col gap-3 border-t border-gray-100 pt-4 sm:flex-row">
                    <button
                      onClick={handleCancel}
                      type="button"
                      className="order-2 rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition-colors duration-150 hover:bg-gray-50 focus:ring-2 focus:ring-gray-500 focus:ring-offset-2 focus:outline-none sm:order-1"
                    >
                      Abbrechen
                    </button>
                    <button
                      onClick={handleSave}
                      disabled={isSaving}
                      type="button"
                      className="order-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors duration-150 hover:bg-gray-800 focus:ring-2 focus:ring-gray-900 focus:ring-offset-2 focus:outline-none disabled:cursor-not-allowed disabled:opacity-50 sm:order-2"
                    >
                      {isSaving ? (
                        <span className="flex items-center">
                          <svg
                            className="mr-2 -ml-1 h-4 w-4 animate-spin text-white"
                            fill="none"
                            viewBox="0 0 24 24"
                          >
                            <circle
                              className="opacity-25"
                              cx="12"
                              cy="12"
                              r="10"
                              stroke="currentColor"
                              strokeWidth="4"
                            ></circle>
                            <path
                              className="opacity-75"
                              fill="currentColor"
                              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                            ></path>
                          </svg>
                          Wird gespeichert...
                        </span>
                      ) : (
                        "Änderungen speichern"
                      )}
                    </button>
                  </div>
                )}
              </form>
            </InfoCard>

            {/* System Settings Link */}
            <InfoCard title="Weitere Einstellungen" className="mt-6">
              <div className="space-y-3">
                <button
                  onClick={() => setShowPasswordModal(true)}
                  className="flex w-full items-center justify-between rounded-lg border border-gray-200 p-4 transition-colors hover:bg-gray-50"
                >
                  <div className="flex items-center space-x-3">
                    <svg
                      className="h-5 w-5 text-gray-600"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
                      />
                    </svg>
                    <div className="text-left">
                      <h4 className="font-medium text-gray-900">
                        Passwort ändern
                      </h4>
                      <p className="text-sm text-gray-600">
                        Aktualisieren Sie Ihr Konto-Passwort
                      </p>
                    </div>
                  </div>
                  <svg
                    className="h-5 w-5 text-gray-400"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M8.25 4.5l7.5 7.5-7.5 7.5"
                    />
                  </svg>
                </button>
                <a
                  href="/settings"
                  className="group block w-full rounded-lg bg-gray-100 px-4 py-2 text-center text-sm font-medium text-gray-900 transition-colors hover:bg-gray-200"
                >
                  <span className="flex items-center justify-center gap-2">
                    <svg
                      className="h-5 w-5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                      />
                    </svg>
                    Systemeinstellungen öffnen
                    <svg
                      className="h-4 w-4 transition-transform group-hover:translate-x-1"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M9 5l7 7-7 7"
                      />
                    </svg>
                  </span>
                </a>
                <p className="text-center text-xs text-gray-500">
                  Benachrichtigungen, Design, Datenschutz und mehr
                </p>
              </div>
            </InfoCard>
          </div>
        </div>
      </div>

      {/* Password Change Modal */}
      <PasswordChangeModal
        isOpen={showPasswordModal}
        onClose={() => setShowPasswordModal(false)}
        onSuccess={() => {
          toastSuccess("Passwort erfolgreich geändert!");
        }}
      />
    </ResponsiveLayout>
  );
}

export default function ProfilePage() {
  return (
    <Suspense
      fallback={
        <ResponsiveLayout>
          <Loading fullPage={false} />
        </ResponsiveLayout>
      }
    >
      <ProfilePageContent />
    </Suspense>
  );
}
