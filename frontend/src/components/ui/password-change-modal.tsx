"use client";

import { useState } from "react";
import { Modal } from "./modal";
import { Alert } from "./alert";
import { EyeIcon, EyeOffIcon, CheckIcon, SpinnerIcon } from "./icons";

interface PasswordToggleProps {
  readonly show: boolean;
  readonly onToggle: () => void;
}

// Extracted component to avoid re-creating on each parent render
function PasswordToggle({ show, onToggle }: PasswordToggleProps) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className="absolute top-1/2 right-3 -translate-y-1/2 p-1 text-gray-500 hover:text-gray-700"
    >
      {show ? <EyeOffIcon /> : <EyeIcon />}
    </button>
  );
}

interface PasswordChangeModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSuccess?: () => void;
}

export function PasswordChangeModal({
  isOpen,
  onClose,
  onSuccess,
}: PasswordChangeModalProps) {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [showCurrentPassword, setShowCurrentPassword] = useState(false);
  const [showNewPassword, setShowNewPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // Validation
    if (!currentPassword || !newPassword || !confirmPassword) {
      setError("Bitte füllen Sie alle Felder aus.");
      return;
    }

    if (newPassword !== confirmPassword) {
      setError("Die neuen Passwörter stimmen nicht überein.");
      return;
    }

    if (newPassword.length < 8) {
      setError("Das neue Passwort muss mindestens 8 Zeichen lang sein.");
      return;
    }

    if (currentPassword === newPassword) {
      setError(
        "Das neue Passwort darf nicht mit dem aktuellen Passwort identisch sein.",
      );
      return;
    }

    setIsLoading(true);

    try {
      const response = await fetch("/api/auth/password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          currentPassword,
          newPassword,
          confirmPassword,
        }),
      });

      if (!response.ok) {
        const data = (await response.json()) as { error?: string };
        throw new Error(data.error ?? "Passwortänderung fehlgeschlagen");
      }

      setSuccess(true);
      setTimeout(() => {
        onSuccess?.();
        handleClose();
      }, 2000);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Ein unerwarteter Fehler ist aufgetreten.",
      );
    } finally {
      setIsLoading(false);
    }
  };

  const handleClose = () => {
    setCurrentPassword("");
    setNewPassword("");
    setConfirmPassword("");
    setError(null);
    setSuccess(false);
    setShowCurrentPassword(false);
    setShowNewPassword(false);
    setShowConfirmPassword(false);
    onClose();
  };

  return (
    <Modal isOpen={isOpen} onClose={handleClose} title="Passwort ändern">
      {success ? (
        <div className="py-8 text-center">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-[#83CD2D]">
            <CheckIcon className="h-8 w-8 text-white" />
          </div>
          <h3 className="mb-2 text-lg font-semibold text-gray-900">
            Passwort erfolgreich geändert!
          </h3>
          <p className="text-sm text-gray-600">
            Sie werden in Kürze weitergeleitet...
          </p>
        </div>
      ) : (
        <form onSubmit={handleSubmit} noValidate className="space-y-4">
          {error && <Alert type="error" message={error} />}

          {/* Current Password */}
          <div>
            <label
              htmlFor="current-password"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Aktuelles Passwort
            </label>
            <div className="relative">
              <input
                id="current-password"
                type={showCurrentPassword ? "text" : "password"}
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                placeholder="••••••••"
                required
                className="block w-full rounded-lg border border-gray-200 bg-white px-4 py-3 pr-12 text-base text-gray-900 transition-colors placeholder:text-gray-400 focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
              />
              <PasswordToggle
                show={showCurrentPassword}
                onToggle={() => setShowCurrentPassword(!showCurrentPassword)}
              />
            </div>
          </div>

          {/* New Password */}
          <div>
            <label
              htmlFor="new-password"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Neues Passwort
            </label>
            <div className="relative">
              <input
                id="new-password"
                type={showNewPassword ? "text" : "password"}
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="••••••••"
                required
                className="block w-full rounded-lg border border-gray-200 bg-white px-4 py-3 pr-12 text-base text-gray-900 transition-colors placeholder:text-gray-400 focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
              />
              <PasswordToggle
                show={showNewPassword}
                onToggle={() => setShowNewPassword(!showNewPassword)}
              />
            </div>
          </div>

          {/* Confirm Password */}
          <div>
            <label
              htmlFor="confirm-password"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Neues Passwort bestätigen
            </label>
            <div className="relative">
              <input
                id="confirm-password"
                type={showConfirmPassword ? "text" : "password"}
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="••••••••"
                required
                className="block w-full rounded-lg border border-gray-200 bg-white px-4 py-3 pr-12 text-base text-gray-900 transition-colors placeholder:text-gray-400 focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
              />
              <PasswordToggle
                show={showConfirmPassword}
                onToggle={() => setShowConfirmPassword(!showConfirmPassword)}
              />
            </div>
          </div>

          {/* Simple Password Requirements */}
          <div className="rounded-lg border border-blue-200 bg-blue-50 p-3">
            <h5 className="mb-2 text-sm font-medium text-gray-900">
              Passwort-Anforderungen
            </h5>
            <ul className="space-y-1 text-xs text-gray-600">
              <li>• Mindestens 8 Zeichen</li>
              <li>• Groß- und Kleinbuchstaben empfohlen</li>
              <li>• Zahlen und Sonderzeichen empfohlen</li>
            </ul>
          </div>

          {/* Action Buttons */}
          <div className="flex gap-3 pt-4">
            <button
              type="button"
              onClick={handleClose}
              className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100"
            >
              Abbrechen
            </button>

            <button
              type="submit"
              disabled={isLoading}
              className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-gray-700 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:scale-100"
            >
              {isLoading ? (
                <span className="flex items-center justify-center gap-2">
                  <SpinnerIcon className="h-4 w-4 text-white" />
                  Wird geändert...
                </span>
              ) : (
                "Passwort ändern"
              )}
            </button>
          </div>
        </form>
      )}
    </Modal>
  );
}
