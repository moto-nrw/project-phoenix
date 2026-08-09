"use client";

import { Suspense, useState, useEffect, useCallback, useRef } from "react";
import { useSession } from "next-auth/react";
import { Loading } from "~/components/ui/loading";
import { PasswordChangeModal } from "~/components/ui/password-change-modal";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
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

  // Email change state
  const [showEmailChangeDialog, setShowEmailChangeDialog] = useState(false);
  const [emailChangeError, setEmailChangeError] = useState<string | null>(null);
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

  const handleEmailChange = async () => {
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
      setShowEmailChangeDialog(false);
      setEmailChangeNewEmail("");
      setEmailChangePassword("");
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

  const emailDialogRef = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = emailDialogRef.current;
    if (!dialog) return;
    if (showEmailChangeDialog && !dialog.open) {
      dialog.showModal();
    } else if (!showEmailChangeDialog && dialog.open) {
      dialog.close();
    }
  }, [showEmailChangeDialog]);

  const handleCloseEmailDialog = () => {
    setShowEmailChangeDialog(false);
    setEmailChangeError(null);
    setEmailChangeNewEmail("");
    setEmailChangePassword("");
  };

  if (status === "loading") {
    return <Loading fullPage={false} />;
  }

  const initials = formData.displayName
    .split(" ")
    .map((p) => p.charAt(0))
    .join("")
    .slice(0, 2)
    .toUpperCase();

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Profil" concept="settings" />

      <div className="mx-auto max-w-2xl space-y-6 px-4 pb-8 md:px-6">
        {/* Avatar Section */}
        <div className="flex flex-col items-center pt-4">
          <div className="relative flex h-28 w-28 items-center justify-center overflow-hidden rounded-full bg-gray-800 text-white shadow-xl">
            <span className="text-3xl font-bold">{initials}</span>
          </div>
        </div>

        {/* Profile Form */}
        <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm md:p-6">
          <div className="space-y-4">
            <div>
              <label
                htmlFor="settings-displayname"
                className="mb-2 block text-sm font-medium text-gray-700"
              >
                Anzeigename
              </label>
              <input
                id="settings-displayname"
                type="text"
                value={formData.displayName}
                onChange={(e) =>
                  setFormData({ ...formData, displayName: e.target.value })
                }
                disabled={!isEditing}
                maxLength={255}
                className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-4 py-3 text-base transition-all focus:ring-2 focus:outline-none disabled:bg-gray-50 disabled:text-gray-500"
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
                maxLength={255}
                className="w-full rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-base text-gray-500"
              />
              {isEditing && (
                <button
                  type="button"
                  onClick={() => setShowEmailChangeDialog(true)}
                  className="text-moto-blue hover:text-moto-blue-hover mt-2 text-sm font-medium transition-colors"
                >
                  E-Mail ändern
                </button>
              )}
            </div>
          </div>
          <div className="mt-4 flex gap-3">
            {isEditing ? (
              <>
                <button
                  type="button"
                  onClick={() => {
                    setIsEditing(false);
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
                  }}
                  className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100"
                >
                  Abbrechen
                </button>
                <button
                  type="button"
                  onClick={() => void handleSaveProfile()}
                  disabled={isSaving}
                  className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-gray-700 hover:shadow-lg active:scale-100 disabled:opacity-50 disabled:hover:scale-100"
                >
                  {isSaving ? "Speichern..." : "Speichern"}
                </button>
              </>
            ) : (
              <button
                type="button"
                onClick={() => setIsEditing(true)}
                className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-gray-700 hover:shadow-lg active:scale-100"
              >
                Bearbeiten
              </button>
            )}
          </div>
        </div>

        {/* Security Section */}
        <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm md:p-6">
          <h3 className="mb-3 text-base font-semibold text-gray-900">
            Passwort ändern
          </h3>
          <p className="mb-4 text-sm text-gray-600">
            Aktualisieren Sie Ihr Passwort regelmäßig für zusätzliche
            Sicherheit.
          </p>
          <button
            type="button"
            onClick={() => setShowPasswordModal(true)}
            className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-gray-700 hover:shadow-lg active:scale-100"
          >
            Passwort ändern
          </button>
        </div>

        {/* Trusted Devices Section */}
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

      <dialog
        ref={emailDialogRef}
        aria-labelledby="email-change-title"
        onClose={handleCloseEmailDialog}
        className="m-auto w-full max-w-md rounded-2xl bg-white p-6 shadow-xl backdrop:bg-black/50"
      >
        {showEmailChangeDialog && (
          <>
            <h2
              id="email-change-title"
              className="mb-4 text-lg font-semibold text-gray-900"
            >
              E-Mail-Adresse ändern
            </h2>

            {emailChangeError && (
              <div
                role="alert"
                className="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-700"
              >
                {emailChangeError}
              </div>
            )}

            <form
              onSubmit={(e) => {
                e.preventDefault();
                void handleEmailChange();
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  e.currentTarget.requestSubmit();
                }
              }}
            >
              <div className="space-y-4">
                <div>
                  <label
                    htmlFor="new-email"
                    className="mb-1 block text-sm font-medium text-gray-700"
                  >
                    Neue E-Mail-Adresse
                  </label>
                  <input
                    id="new-email"
                    type="email"
                    required
                    autoFocus
                    autoComplete="email"
                    pattern="[A-Za-z0-9._+%\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]+"
                    title="Bitte geben Sie eine gültige E-Mail-Adresse ein (z.B. name@beispiel.de)"
                    value={emailChangeNewEmail}
                    onChange={(e) => setEmailChangeNewEmail(e.target.value)}
                    className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-4 py-3 text-base transition-all focus:ring-2 focus:outline-none"
                    placeholder="neue@email.de"
                  />
                </div>
                <div>
                  <label
                    htmlFor="confirm-password"
                    className="mb-1 block text-sm font-medium text-gray-700"
                  >
                    Aktuelles Passwort
                  </label>
                  <input
                    id="confirm-password"
                    type="password"
                    required
                    autoComplete="current-password"
                    value={emailChangePassword}
                    onChange={(e) => setEmailChangePassword(e.target.value)}
                    className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-4 py-3 text-base transition-all focus:ring-2 focus:outline-none"
                  />
                </div>
              </div>

              <div className="mt-6 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={handleCloseEmailDialog}
                  className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all hover:bg-gray-50"
                >
                  Abbrechen
                </button>
                <button
                  type="submit"
                  disabled={
                    emailChangeLoading ||
                    !emailChangeNewEmail ||
                    !emailChangePassword
                  }
                  className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all hover:bg-gray-700 disabled:opacity-50"
                >
                  {emailChangeLoading
                    ? "Wird gesendet..."
                    : "E-Mail-Änderung anfordern"}
                </button>
              </div>
            </form>
          </>
        )}
      </dialog>
    </div>
  );
}

export default function OperatorSettingsPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <OperatorSettingsContent />
    </Suspense>
  );
}
