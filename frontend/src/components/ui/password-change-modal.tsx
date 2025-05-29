"use client";

import { useState } from "react";
import { Modal } from "./modal";
import { Alert } from "./alert";

interface PasswordChangeModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: () => void;
}

export function PasswordChangeModal({ isOpen, onClose, onSuccess }: PasswordChangeModalProps) {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [showCurrentPassword, setShowCurrentPassword] = useState(false);
  const [showNewPassword, setShowNewPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);

  // Password strength indicator
  const getPasswordStrength = (password: string): { strength: number; label: string; color: string } => {
    let strength = 0;
    if (password.length >= 8) strength++;
    if (/[a-z]/.exec(password) && /[A-Z]/.exec(password)) strength++;
    if (/\d/.exec(password)) strength++;
    if (/[^a-zA-Z\d]/.exec(password)) strength++;

    const labels = ["Schwach", "Mittel", "Gut", "Stark"];
    const colors = ["bg-red-500", "bg-orange-500", "bg-yellow-500", "bg-green-500"];

    return {
      strength,
      label: labels[strength] ?? labels[0] ?? "Schwach",
      color: colors[strength] ?? colors[0] ?? "bg-red-500",
    };
  };

  const passwordStrength = getPasswordStrength(newPassword);

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
      setError("Das neue Passwort darf nicht mit dem aktuellen Passwort identisch sein.");
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
        const data = await response.json() as { error?: string };
        throw new Error(data.error ?? "Passwortänderung fehlgeschlagen");
      }

      setSuccess(true);
      setTimeout(() => {
        onSuccess?.();
        handleClose();
      }, 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ein unerwarteter Fehler ist aufgetreten.");
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

  const PasswordToggle = ({ show, onToggle }: { show: boolean; onToggle: () => void }) => (
    <button
      type="button"
      onClick={onToggle}
      className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-700"
    >
      {show ? (
        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
        </svg>
      ) : (
        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
        </svg>
      )}
    </button>
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title="Passwort ändern"
    >
      {success ? (
        <div className="py-8 text-center">
          <div className="mx-auto mb-4 h-16 w-16 rounded-full bg-gradient-to-br from-[#83CD2D] to-[#70b525] flex items-center justify-center">
            <svg className="h-8 w-8 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
            </svg>
          </div>
          <h3 className="text-lg font-semibold text-gray-900 mb-2">Passwort erfolgreich geändert!</h3>
          <p className="text-sm text-gray-600">Sie werden in Kürze weitergeleitet...</p>
        </div>
      ) : (
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <Alert type="error" message={error} />
          )}

          {/* Current Password */}
          <div>
            <label htmlFor="current-password" className="block text-sm font-medium text-gray-700 mb-1">
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
                className="block w-full rounded-lg border-0 px-4 py-3 text-base text-gray-900 bg-white shadow-sm ring-1 ring-inset ring-gray-200 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-gray-900 transition-all duration-200 pr-10"
              />
              <PasswordToggle show={showCurrentPassword} onToggle={() => setShowCurrentPassword(!showCurrentPassword)} />
            </div>
          </div>

          {/* New Password */}
          <div>
            <label htmlFor="new-password" className="block text-sm font-medium text-gray-700 mb-1">
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
                className="block w-full rounded-lg border-0 px-4 py-3 text-base text-gray-900 bg-white shadow-sm ring-1 ring-inset ring-gray-200 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-gray-900 transition-all duration-200 pr-10"
              />
              <PasswordToggle show={showNewPassword} onToggle={() => setShowNewPassword(!showNewPassword)} />
            </div>
            
            {/* Password Strength Indicator */}
            {newPassword && (
              <div className="mt-2">
                <div className="flex items-center justify-between text-xs mb-1">
                  <span className="text-gray-600">Passwortstärke:</span>
                  <span className="font-medium">{passwordStrength.label}</span>
                </div>
                <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
                  <div
                    className={`h-full transition-all duration-300 ${passwordStrength.color}`}
                    style={{ width: `${(passwordStrength.strength + 1) * 25}%` }}
                  />
                </div>
              </div>
            )}
          </div>

          {/* Confirm Password */}
          <div>
            <label htmlFor="confirm-password" className="block text-sm font-medium text-gray-700 mb-1">
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
                className="block w-full rounded-lg border-0 px-4 py-3 text-base text-gray-900 bg-white shadow-sm ring-1 ring-inset ring-gray-200 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-gray-900 transition-all duration-200 pr-10"
              />
              <PasswordToggle show={showConfirmPassword} onToggle={() => setShowConfirmPassword(!showConfirmPassword)} />
            </div>
          </div>

          {/* Password Requirements */}
          <div className="bg-gray-50 rounded-lg p-3">
            <p className="text-xs font-medium text-gray-700 mb-2">Passwort-Anforderungen:</p>
            <ul className="space-y-1 text-xs text-gray-600">
              <li className="flex items-center gap-2">
                <span className={`h-1.5 w-1.5 rounded-full ${newPassword.length >= 8 ? 'bg-green-500' : 'bg-gray-300'}`} />
                Mindestens 8 Zeichen
              </li>
              <li className="flex items-center gap-2">
                <span className={`h-1.5 w-1.5 rounded-full ${/[a-z]/.exec(newPassword) && /[A-Z]/.exec(newPassword) ? 'bg-green-500' : 'bg-gray-300'}`} />
                Groß- und Kleinbuchstaben
              </li>
              <li className="flex items-center gap-2">
                <span className={`h-1.5 w-1.5 rounded-full ${/\d/.exec(newPassword) ? 'bg-green-500' : 'bg-gray-300'}`} />
                Mindestens eine Zahl
              </li>
              <li className="flex items-center gap-2">
                <span className={`h-1.5 w-1.5 rounded-full ${/[^a-zA-Z\d]/.exec(newPassword) ? 'bg-green-500' : 'bg-gray-300'}`} />
                Mindestens ein Sonderzeichen
              </li>
            </ul>
          </div>

          {/* Action Buttons */}
          <div className="flex gap-2.5 pt-4">
            <button
              type="button"
              onClick={handleClose}
              className="relative flex-1 px-4 py-2 text-sm text-gray-600 font-medium rounded-md bg-white border border-gray-200 hover:border-gray-300 focus:outline-none focus:ring-2 focus:ring-gray-200 focus:ring-offset-1 transition-all duration-300 group overflow-hidden"
            >
              {/* Hover effect */}
              <div className="absolute inset-0 bg-gradient-to-r from-transparent via-gray-50 to-transparent -translate-x-full group-hover:translate-x-full transition-transform duration-700 ease-out" />
              
              <span className="relative flex items-center justify-center gap-1.5">
                <svg 
                  className="w-4 h-4 transition-all duration-300 group-hover:rotate-90 group-hover:scale-110" 
                  fill="none" 
                  viewBox="0 0 24 24" 
                  stroke="currentColor"
                  strokeWidth={2.5}
                >
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
                <span className="transition-colors duration-300 group-hover:text-gray-900">Abbrechen</span>
              </span>
            </button>
            
            <button
              type="submit"
              disabled={isLoading}
              className="relative flex-1 px-4 py-2.5 text-sm font-medium rounded-lg text-white bg-gradient-to-br from-[#83CD2D] to-[#70b525] hover:shadow-lg hover:shadow-[#83CD2D]/25 transform transition-all duration-200 active:scale-95 focus:outline-none focus:ring-2 focus:ring-[#83CD2D] focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed group overflow-hidden"
            >
              {/* Gradient overlay that moves on hover */}
              <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/10 to-transparent -translate-x-full group-hover:translate-x-full transition-transform duration-1000 ease-out" />
              
              {/* Button content */}
              <span className="relative flex items-center justify-center gap-2">
                {isLoading ? (
                  <>
                    <svg className="animate-spin h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    <span>Wird geändert...</span>
                  </>
                ) : (
                  <>
                    <span className="relative flex items-center justify-center">
                      {/* Animated ring around icon */}
                      <span className="absolute h-7 w-7 rounded-full bg-white/20 scale-0 group-hover:scale-110 transition-transform duration-300 ease-out" />
                      <svg 
                        className="relative h-5 w-5 transition-transform duration-200" 
                        fill="none" 
                        viewBox="0 0 24 24" 
                        stroke="currentColor"
                        strokeWidth={2}
                      >
                        <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                      </svg>
                    </span>
                    <span className="font-medium tracking-wide">Änderungen speichern</span>
                  </>
                )}
              </span>
              
              {/* Subtle pulse on hover */}
              <span className="absolute inset-0 rounded-lg ring-2 ring-[#83CD2D] ring-opacity-0 group-hover:ring-opacity-30 transition-all duration-300 group-hover:scale-105 pointer-events-none" />
            </button>
          </div>
        </form>
      )}
    </Modal>
  );
}