"use client";

import { useState, useEffect } from "react";
import { Alert } from "~/components/ui";

interface PINStatus {
  has_pin: boolean;
  last_changed?: string | null;
}

interface PINManagementProps {
  onSuccess?: () => void;
}

export function PINManagement({ onSuccess }: PINManagementProps) {
  const [pinStatus, setPinStatus] = useState<PINStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  
  // Form state
  const [currentPin, setCurrentPin] = useState("");
  const [newPin, setNewPin] = useState("");
  const [confirmPin, setConfirmPin] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Load PIN status on component mount
  useEffect(() => {
    void loadPinStatus();
  }, []);

  const loadPinStatus = async () => {
    try {
      setLoading(true);
      const response = await fetch("/api/staff/pin");
      
      if (!response.ok) {
        const errorData = await response.json() as { error?: string };
        throw new Error(errorData.error ?? "Fehler beim Laden des PIN-Status");
      }
      
      const responseData = await response.json() as { success: boolean; message: string; data: PINStatus };
      setPinStatus(responseData.data);
    } catch (err) {
      console.error("Error loading PIN status:", err);
      setError(err instanceof Error ? err.message : "Fehler beim Laden des PIN-Status");
    } finally {
      setLoading(false);
    }
  };

  const validateForm = (): string | null => {
    if (!newPin.trim()) {
      return "Neue PIN ist erforderlich";
    }

    if (!/^\d{4}$/.test(newPin)) {
      return "PIN muss aus genau 4 Ziffern bestehen";
    }

    if (newPin !== confirmPin) {
      return "PIN-Bestätigung stimmt nicht überein";
    }

    if (pinStatus?.has_pin && !currentPin.trim()) {
      return "Aktuelle PIN ist erforderlich";
    }

    if (currentPin && !/^\d{4}$/.test(currentPin)) {
      return "Aktuelle PIN muss aus genau 4 Ziffern bestehen";
    }

    return null;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSuccess(null);

    const validationError = validateForm();
    if (validationError) {
      setError(validationError);
      return;
    }

    try {
      setIsSubmitting(true);

      const requestBody = {
        new_pin: newPin,
        current_pin: pinStatus?.has_pin ? currentPin : null,
      };

      const response = await fetch("/api/staff/pin", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(requestBody),
      });

      if (!response.ok) {
        const errorData = await response.json() as { error?: string };
        throw new Error(errorData.error ?? "Fehler beim Aktualisieren der PIN");
      }

      // Clear form
      setCurrentPin("");
      setNewPin("");
      setConfirmPin("");
      
      // Update PIN status
      void loadPinStatus();
      
      // Show success message
      setSuccess(pinStatus?.has_pin ? "PIN erfolgreich geändert" : "PIN erfolgreich erstellt");
      
      // Call success callback
      onSuccess?.();
      
    } catch (err) {
      console.error("Error updating PIN:", err);
      setError(err instanceof Error ? err.message : "Fehler beim Aktualisieren der PIN");
    } finally {
      setIsSubmitting(false);
    }
  };

  const formatLastChanged = (dateString?: string | null): string => {
    if (!dateString) return "Nie";
    
    try {
      const date = new Date(dateString);
      return date.toLocaleDateString("de-DE", {
        year: "numeric",
        month: "long",
        day: "numeric",
      });
    } catch {
      return "Unbekannt";
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <div className="h-6 w-6 animate-spin rounded-full border-b-2 border-t-2 border-[#5080D8]"></div>
        <span className="ml-2 text-sm text-gray-600">PIN-Status wird geladen...</span>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h4 className="text-lg font-semibold text-gray-900">RFID-PIN Verwaltung</h4>
        <p className="mt-1 text-sm text-gray-600">
          4-stellige PIN für RFID-Geräte
        </p>
      </div>

      {/* PIN Status Display */}
      <div className={`rounded-xl p-4 border ${pinStatus?.has_pin ? "bg-green-50 border-green-200" : "bg-yellow-50 border-yellow-200"}`}>
        <div className="flex items-center gap-3">
          <div className={`flex-shrink-0 ${pinStatus?.has_pin ? "text-green-600" : "text-yellow-600"}`}>
            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              {pinStatus?.has_pin ? (
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              ) : (
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              )}
            </svg>
          </div>
          <div>
            <p className="font-medium text-gray-900">
              {pinStatus?.has_pin ? "PIN ist eingerichtet" : "Keine PIN eingerichtet"}
            </p>
            <p className="text-sm text-gray-600">
              Letzte Änderung: {formatLastChanged(pinStatus?.last_changed)}
            </p>
          </div>
        </div>
      </div>

      {/* Error/Success Messages */}
      {error && <Alert type="error" message={error} />}
      {success && <Alert type="success" message={success} />}

      {/* PIN Form */}
      <form onSubmit={handleSubmit} className="space-y-4">
        {/* Current PIN (only if PIN exists) */}
        {pinStatus?.has_pin && (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Aktuelle PIN
            </label>
            <input
              type="password"
              value={currentPin}
              onChange={(e) => setCurrentPin(e.target.value.replace(/\D/g, "").slice(0, 4))}
              maxLength={4}
              placeholder="0000"
              disabled={isSubmitting}
              className="block w-full rounded-lg border border-gray-200 px-4 py-3 text-base text-gray-900 bg-white placeholder:text-gray-400 focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] transition-colors"
            />
          </div>
        )}

        {/* New PIN */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">
            Neue PIN
          </label>
          <input
            type="password"
            value={newPin}
            onChange={(e) => setNewPin(e.target.value.replace(/\D/g, "").slice(0, 4))}
            maxLength={4}
            placeholder="0000"
            disabled={isSubmitting}
            className="block w-full rounded-lg border border-gray-200 px-4 py-3 text-base text-gray-900 bg-white placeholder:text-gray-400 focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] transition-colors"
          />
        </div>

        {/* Confirm PIN */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">
            PIN bestätigen
          </label>
          <input
            type="password"
            value={confirmPin}
            onChange={(e) => setConfirmPin(e.target.value.replace(/\D/g, "").slice(0, 4))}
            maxLength={4}
            placeholder="0000"
            disabled={isSubmitting}
            className="block w-full rounded-lg border border-gray-200 px-4 py-3 text-base text-gray-900 bg-white placeholder:text-gray-400 focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] transition-colors"
          />
        </div>

        {/* Submit Button */}
        <div className="pt-2">
          <button
            type="submit"
            disabled={isSubmitting}
            className="w-full px-6 py-3 bg-[#83CD2D] text-white font-medium rounded-xl hover:bg-[#70B525] active:scale-95 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isSubmitting ? (
              <span className="flex items-center justify-center gap-2">
                <svg className="animate-spin h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                Wird gespeichert...
              </span>
            ) : (
              pinStatus?.has_pin ? "PIN ändern" : "PIN erstellen"
            )}
          </button>
        </div>
      </form>

      {/* Simple Info Box */}
      <div className="rounded-xl border border-blue-200 bg-blue-50 p-4">
        <h5 className="font-medium text-gray-900 mb-2">PIN-Information</h5>
        <ul className="text-sm text-gray-600 space-y-1">
          <li>• Für RFID-Geräte-Authentifizierung</li>
          <li>• Genau 4 Ziffern erforderlich</li>
          <li>• Sicher aufbewahren</li>
        </ul>
      </div>
    </div>
  );
}