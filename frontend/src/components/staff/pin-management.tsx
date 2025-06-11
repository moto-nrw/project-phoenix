"use client";

import { useState, useEffect } from "react";
import { Input, Button, Alert } from "~/components/ui";

// Simple icon component
const Icon: React.FC<{ path: string; className?: string }> = ({ path, className }) => (
  <svg
    className={className}
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    strokeWidth={2}
  >
    <path strokeLinecap="round" strokeLinejoin="round" d={path} />
  </svg>
);

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
      
      // The route wrapper returns { success: boolean, message: string, data: T }
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
    // Check if new PIN is provided
    if (!newPin.trim()) {
      return "Neue PIN ist erforderlich";
    }

    // Validate PIN format (4 digits)
    if (!/^\d{4}$/.test(newPin)) {
      return "PIN muss aus genau 4 Ziffern bestehen";
    }

    // Check PIN confirmation
    if (newPin !== confirmPin) {
      return "PIN-Bestätigung stimmt nicht überein";
    }

    // If user has existing PIN, current PIN is required
    if (pinStatus?.has_pin && !currentPin.trim()) {
      return "Aktuelle PIN ist erforderlich";
    }

    // Validate current PIN format if provided
    if (currentPin && !/^\d{4}$/.test(currentPin)) {
      return "Aktuelle PIN muss aus genau 4 Ziffern bestehen";
    }

    return null;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSuccess(null);

    // Validate form
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

      // The route wrapper returns { success: boolean, message: string, data: BackendPINUpdateResponse }
      const responseData = await response.json() as { success: boolean; message: string; data: { status: string; data: { success: boolean; message: string }; message: string } };
      
      // Extract success message from nested structure
      const updateMessage = responseData.data?.data?.message || "PIN erfolgreich aktualisiert";
      
      // Clear form
      setCurrentPin("");
      setNewPin("");
      setConfirmPin("");
      
      // Update PIN status
      void loadPinStatus();
      
      // Show success message
      setSuccess(updateMessage || (pinStatus?.has_pin ? "PIN erfolgreich geändert" : "PIN erfolgreich erstellt"));
      
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
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return "Unbekannt";
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <div className="h-6 w-6 animate-spin rounded-full border-b-2 border-t-2 border-gray-900"></div>
        <span className="ml-2 text-sm text-gray-600">PIN-Status wird geladen...</span>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h4 className="text-lg font-semibold text-gray-900">RFID-PIN Verwaltung</h4>
        <p className="mt-1 text-sm text-gray-600">
          Verwalten Sie Ihre 4-stellige PIN für den RFID-Gerätezugang
        </p>
      </div>

      {/* PIN Status Display */}
      <div className="rounded-lg border border-gray-200 bg-gray-50 p-4">
        <div className="flex items-center space-x-3">
          <div className={`rounded-full p-2 ${pinStatus?.has_pin ? "bg-green-100" : "bg-yellow-100"}`}>
            <Icon 
              path={pinStatus?.has_pin 
                ? "M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" 
                : "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              } 
              className={`h-5 w-5 ${pinStatus?.has_pin ? "text-green-600" : "text-yellow-600"}`} 
            />
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
            <Input
              type="password"
              label="Aktuelle PIN"
              value={currentPin}
              onChange={(e) => setCurrentPin(e.target.value.replace(/\D/g, "").slice(0, 4))}
              maxLength={4}
              placeholder="0000"
              disabled={isSubmitting}
              error={error && currentPin === "" ? "Aktuelle PIN ist erforderlich" : undefined}
            />
            <p className="mt-1 text-xs text-gray-500">
              Geben Sie Ihre aktuelle 4-stellige PIN ein
            </p>
          </div>
        )}

        {/* New PIN */}
        <div>
          <Input
            type="password"
            label="Neue PIN"
            value={newPin}
            onChange={(e) => setNewPin(e.target.value.replace(/\D/g, "").slice(0, 4))}
            maxLength={4}
            placeholder="0000"
            disabled={isSubmitting}
            error={error && (newPin === "" || !/^\d{4}$/.test(newPin)) ? "4-stellige PIN erforderlich" : undefined}
          />
          <p className="mt-1 text-xs text-gray-500">
            Wählen Sie eine neue 4-stellige PIN (nur Ziffern)
          </p>
        </div>

        {/* Confirm PIN */}
        <div>
          <Input
            type="password"
            label="PIN bestätigen"
            value={confirmPin}
            onChange={(e) => setConfirmPin(e.target.value.replace(/\D/g, "").slice(0, 4))}
            maxLength={4}
            placeholder="0000"
            disabled={isSubmitting}
            error={error && newPin !== confirmPin ? "PIN-Bestätigung stimmt nicht überein" : undefined}
          />
          <p className="mt-1 text-xs text-gray-500">
            Geben Sie die neue PIN erneut ein
          </p>
        </div>

        {/* Submit Button */}
        <div className="flex justify-end pt-4">
          <Button
            type="submit"
            variant="success"
            disabled={isSubmitting}
            className="min-w-[160px]"
          >
            {isSubmitting ? (
              <span className="flex items-center gap-2">
                <div className="h-4 w-4 animate-spin rounded-full border-b-2 border-t-2 border-white"></div>
                Wird gespeichert...
              </span>
            ) : (
              <span className="flex items-center gap-2">
                <Icon 
                  path="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" 
                  className="h-4 w-4" 
                />
                {pinStatus?.has_pin ? "PIN ändern" : "PIN erstellen"}
              </span>
            )}
          </Button>
        </div>
      </form>

      {/* Info Box */}
      <div className="rounded-lg border border-blue-200 bg-blue-50 p-4">
        <div className="flex items-start space-x-3">
          <Icon 
            path="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" 
            className="h-5 w-5 text-blue-600 flex-shrink-0 mt-0.5" 
          />
          <div>
            <h5 className="font-medium text-gray-900">RFID-PIN Information</h5>
            <div className="mt-1 text-sm text-gray-600 space-y-1">
              <p>• Die PIN wird für die Authentifizierung an RFID-Geräten verwendet</p>
              <p>• Sie muss aus genau 4 Ziffern bestehen</p>
              <p>• Nach mehreren falschen Versuchen wird das Konto vorübergehend gesperrt</p>
              <p>• Bewahren Sie Ihre PIN sicher auf</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}