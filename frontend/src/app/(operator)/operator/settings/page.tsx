"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import { Loading } from "~/components/ui/loading";
import { useOperatorAuth } from "~/lib/operator/auth-context";
import { SettingsLayout } from "~/components/shared/settings-layout";

interface ProfileResponse {
  data: {
    id: number;
    email: string;
    display_name: string;
  };
}

function OperatorSettingsContent() {
  const {
    operator,
    isLoading: authLoading,
    updateOperator,
  } = useOperatorAuth();

  const [showAlert, setShowAlert] = useState(false);
  const [alertMessage, setAlertMessage] = useState("");
  const [alertType, setAlertType] = useState<"success" | "error">("success");

  // Profile editing state
  const [isEditing, setIsEditing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [formData, setFormData] = useState({
    displayName: "",
    email: "",
  });

  const handleAlertClose = useCallback(() => {
    setShowAlert(false);
  }, []);

  // Sync formData with operator from Context
  useEffect(() => {
    if (operator) {
      setFormData({
        displayName: operator.displayName || "",
        email: operator.email || "",
      });
    }
  }, [operator]);

  const handleSaveProfile = async () => {
    setIsSaving(true);
    try {
      const response = await fetch("/api/operator/profile", {
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

      const result = (await response.json()) as ProfileResponse;
      updateOperator({ displayName: result.data.display_name.toString() });

      setIsEditing(false);
      setAlertMessage("Profil erfolgreich aktualisiert");
      setAlertType("success");
      setShowAlert(true);
    } catch {
      setAlertMessage("Fehler beim Speichern des Profils");
      setAlertType("error");
      setShowAlert(true);
    } finally {
      setIsSaving(false);
    }
  };

  if (authLoading) {
    return <Loading fullPage={false} />;
  }

  const initials = formData.displayName
    .split(" ")
    .map((p) => p.charAt(0))
    .join("")
    .slice(0, 2)
    .toUpperCase();

  const profileTab = (
    <div className="space-y-6">
      {/* Initials Avatar */}
      <div className="mb-8">
        <div className="inline-block">
          <div className="flex flex-col items-center">
            <div className="relative flex h-32 w-32 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-gray-700 to-gray-900 text-white shadow-xl">
              <span className="text-4xl font-bold">{initials}</span>
            </div>
          </div>
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
                if (operator) {
                  setFormData({
                    displayName: operator.displayName || "",
                    email: operator.email || "",
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
        <span className="text-xl font-bold">{initials}</span>
      </div>
      <div className="flex-1 text-left">
        <p className="text-lg font-semibold text-gray-900">
          {formData.displayName}
        </p>
        <p className="text-sm text-gray-500">Profil, Anzeigename</p>
      </div>
    </>
  );

  return (
    <SettingsLayout
      profileTab={profileTab}
      mobileProfileCard={mobileProfileCard}
      passwordApiEndpoint="/api/operator/profile/password"
      onPasswordSuccess={() => {
        setAlertMessage("Passwort erfolgreich geändert");
        setAlertType("success");
        setShowAlert(true);
      }}
      alert={
        showAlert
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

export default function OperatorSettingsPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <OperatorSettingsContent />
    </Suspense>
  );
}
