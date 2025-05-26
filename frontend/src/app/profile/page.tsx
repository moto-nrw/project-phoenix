"use client";

import { useState, useEffect, Suspense } from "react";
import { useSession, signOut } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Input } from "~/components/ui";
import { Alert } from "~/components/ui/alert";
import { fetchProfile, updateProfile, uploadAvatar } from "~/lib/profile-api";
import type { Profile, ProfileUpdateRequest } from "~/lib/profile-helpers";

// Info Card Component
interface InfoCardProps {
  title: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

const InfoCard: React.FC<InfoCardProps> = ({ title, children, className }) => (
  <div className={`rounded-lg border border-gray-100 bg-white p-4 md:p-6 shadow-md ${className ?? ""}`}>
    <h3 className="text-base md:text-lg font-semibold mb-4">{title}</h3>
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

const AvatarUpload: React.FC<AvatarUploadProps> = ({ avatar, firstName, lastName, onAvatarChange }) => {
  const getInitials = () => {
    const first = firstName?.charAt(0) || '';
    const last = lastName?.charAt(0) || '';
    return (first + last).toUpperCase() || '?';
  };
  const initials = getInitials();

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      onAvatarChange(file);
    }
  };

  // Debug avatar URL
  if (avatar) {
    console.log('Avatar URL:', avatar);
  }

  return (
    <div className="flex flex-col items-center">
      <div className="relative group">
        <div className="w-24 h-24 md:w-32 md:h-32 rounded-full overflow-hidden bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center text-white shadow-lg">
          {avatar ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={avatar} alt="Profile" className="w-full h-full object-cover" />
          ) : (
            <span className="text-2xl md:text-3xl font-bold">{initials}</span>
          )}
        </div>
        <label htmlFor="avatar-upload" className="absolute inset-0 flex items-center justify-center bg-black/50 rounded-full opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer">
          <svg className="w-6 h-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z" />
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 13a3 3 0 11-6 0 3 3 0 016 0z" />
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
      <button className="mt-3 text-sm text-blue-600 hover:text-blue-800 font-medium">
        Foto ändern
      </button>
    </div>
  );
};

function ProfilePageContent() {
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const [profile, setProfile] = useState<Profile | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [hasLoadedProfile, setHasLoadedProfile] = useState(false);

  // Form state
  const [formData, setFormData] = useState({
    firstName: "",
    lastName: "",
    bio: "",
    email: "",
    username: "",
  });

  // Load profile data
  useEffect(() => {
    if (session?.user?.token && !hasLoadedProfile) {
      void loadProfile();
    }
  }, [session, hasLoadedProfile]);

  const loadProfile = async () => {
    try {
      setIsLoading(true);
      setError(null);
      setHasLoadedProfile(true); // Mark as attempted to prevent loops
      
      const data = await fetchProfile();
      setProfile(data);
      setFormData({
        firstName: data.firstName || "",
        lastName: data.lastName || "",
        bio: data.bio ?? "",
        email: data.email,
        username: data.username ?? "",
      });
    } catch (err) {
      console.error("Error loading profile:", err);
      
      // Check if it's an authentication error
      if (err instanceof Error && err.message.includes('401')) {
        setError("Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.");
        // Redirect to login after a short delay
        setTimeout(() => {
          void signOut({ callbackUrl: '/' });
        }, 2000);
      } else {
        // For mock data, we shouldn't get errors, but if we do, just log them
        console.error("Unexpected error with mock data:", err);
        setError("Fehler beim Laden des Profils");
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const { name, value } = e.target;
    setFormData(prev => ({ ...prev, [name]: value }));
  };

  const handleSave = async () => {
    try {
      setIsSaving(true);
      setError(null);
      setSuccessMessage(null);

      // If no profile exists yet, firstName and lastName are required
      if ((!profile?.firstName || !profile?.lastName) && 
          (!formData.firstName || !formData.lastName)) {
        setError("Vorname und Nachname sind erforderlich, um Ihr Profil zu erstellen.");
        setIsSaving(false);
        return;
      }

      const updateData: ProfileUpdateRequest = {
        firstName: formData.firstName,
        lastName: formData.lastName,
        bio: formData.bio || undefined,
        username: formData.username || undefined,
      };

      const updatedProfile = await updateProfile(updateData);
      setProfile(updatedProfile);
      setIsEditing(false);
      setSuccessMessage("Profil erfolgreich aktualisiert");
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
      const updatedProfile = await uploadAvatar(file);
      setProfile(updatedProfile);
      setSuccessMessage("Profilbild erfolgreich aktualisiert");
      setTimeout(() => setSuccessMessage(null), 3000);
    } catch (err) {
      console.error("Error uploading avatar:", err);
      setError(err instanceof Error ? err.message : "Fehler beim Hochladen des Profilbilds");
    } finally {
      setIsSaving(false);
    }
  };

  if (status === "loading" || isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
          <p className="text-gray-600">Profil wird geladen...</p>
        </div>
      </div>
    );
  }

  if (!profile) {
    return (
      <ResponsiveLayout userName={session?.user?.name ?? "Root"}>
        <div className="max-w-4xl mx-auto">
          <Alert type="error" message="Profil konnte nicht geladen werden" />
        </div>
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout userName={session?.user?.name ?? "Root"}>
      <div className="max-w-4xl mx-auto">
        {/* Header */}
        <div className="mb-6 md:mb-8">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div>
              <h1 className="text-2xl md:text-3xl font-bold text-gray-900">Mein Profil</h1>
              <p className="mt-1 text-sm md:text-base text-gray-600">
                Verwalte deine persönlichen Informationen
              </p>
            </div>
            {/* Desktop edit button */}
            {!isEditing && (
              <button
                onClick={() => setIsEditing(true)}
                className="hidden sm:block self-start sm:self-auto px-4 py-2 bg-gray-900 text-white rounded-lg hover:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gray-900 transition-colors duration-150 text-sm font-medium"
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
        {successMessage && (
          <div className="mb-6">
            <Alert type="success" message={successMessage} />
          </div>
        )}

        {/* Profile Content */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
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
                    {new Date(profile.createdAt).toLocaleDateString('de-DE', {
                      year: 'numeric',
                      month: 'long',
                      day: 'numeric'
                    })}
                  </p>
                </div>
                {profile.lastLogin && (
                  <div className="text-center">
                    <p className="text-sm text-gray-500">Letzte Anmeldung</p>
                    <p className="font-medium">
                      {new Date(profile.lastLogin).toLocaleDateString('de-DE', {
                        year: 'numeric',
                        month: 'short',
                        day: 'numeric',
                        hour: '2-digit',
                        minute: '2-digit'
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
                    <div className="p-2 bg-green-100 rounded-lg">
                      <svg className="w-6 h-6 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                      </svg>
                    </div>
                    <div>
                      <p className="font-medium">Aktiv</p>
                      <p className="text-xs text-gray-500">ID: {profile.rfidCard}</p>
                    </div>
                  </div>
                </div>
              </InfoCard>
            )}
          </div>

          {/* Right Column - Personal Information */}
          <div className="lg:col-span-2">
            <InfoCard title={
              <div className="flex items-center justify-between">
                <span>Persönliche Informationen</span>
                {!isEditing && (
                  <button
                    onClick={() => setIsEditing(true)}
                    className="sm:hidden -mt-1 -mr-2 p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-50 rounded-lg transition-all duration-150 touch-manipulation"
                    aria-label="Bearbeiten"
                  >
                    <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                    </svg>
                  </button>
                )}
              </div>
            }>
              {(!profile.firstName || !profile.lastName) && (
                <div className="mb-4 p-3 bg-blue-50 border border-blue-200 rounded-lg">
                  <p className="text-sm text-blue-800">
                    <span className="font-medium">Profil unvollständig:</span> Bitte vervollständigen Sie Ihren Vor- und Nachnamen.
                  </p>
                </div>
              )}
              <form className="space-y-4">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
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
                  <p className="mt-1 text-xs text-gray-500">E-Mail kann in den Einstellungen geändert werden</p>
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
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Über mich
                  </label>
                  <textarea
                    name="bio"
                    value={formData.bio}
                    onChange={handleInputChange}
                    disabled={!isEditing}
                    rows={4}
                    maxLength={500}
                    className={`
                      block w-full px-4 py-3 
                      text-base text-gray-900 
                      bg-white 
                      border-0 rounded-lg 
                      shadow-sm ring-1 ring-inset ring-gray-200 
                      placeholder:text-gray-400 
                      focus:ring-2 focus:ring-inset focus:ring-gray-900 
                      disabled:bg-gray-50 disabled:text-gray-500 disabled:ring-gray-200
                      transition-all duration-200
                      resize-none
                    `}
                    placeholder="Erzähle etwas über dich..."
                  />
                </div>

                {/* Edit Actions */}
                {isEditing && (
                  <div className="flex flex-col sm:flex-row gap-3 pt-4 border-t border-gray-100">
                    <button
                      onClick={handleCancel}
                      type="button"
                      className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gray-500 transition-colors duration-150 order-2 sm:order-1"
                    >
                      Abbrechen
                    </button>
                    <button
                      onClick={handleSave}
                      disabled={isSaving}
                      type="button"
                      className="px-4 py-2 text-sm font-medium text-white bg-gray-900 rounded-lg hover:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gray-900 disabled:opacity-50 disabled:cursor-not-allowed transition-colors duration-150 order-1 sm:order-2"
                    >
                      {isSaving ? (
                        <span className="flex items-center">
                          <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                          </svg>
                          Wird gespeichert...
                        </span>
                      ) : "Änderungen speichern"}
                    </button>
                  </div>
                )}
              </form>
            </InfoCard>

            {/* Account Security Info */}
            <InfoCard title="Kontosicherheit" className="mt-6">
              <div className="flex items-center justify-between py-3">
                <div>
                  <p className="font-medium">Passwort</p>
                  <p className="text-sm text-gray-500">Zuletzt geändert vor 30 Tagen</p>
                </div>
                <a
                  href="/settings"
                  className="text-sm font-medium text-gray-900 hover:text-gray-700 flex items-center gap-1 transition-colors"
                >
                  Ändern
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                  </svg>
                </a>
              </div>
            </InfoCard>
          </div>
        </div>
      </div>
    </ResponsiveLayout>
  );
}

export default function ProfilePage() {
  return (
    <Suspense fallback={
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
      </div>
    }>
      <ProfilePageContent />
    </Suspense>
  );
}