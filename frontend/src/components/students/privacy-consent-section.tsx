"use client";

import { useState, useEffect } from "react";
import { fetchStudentPrivacyConsent } from "~/lib/student-api";
import type { PrivacyConsent } from "~/lib/student-helpers";

interface PrivacyConsentSectionProps {
  studentId: string;
}

export function PrivacyConsentSection({ studentId }: PrivacyConsentSectionProps) {
  const [consent, setConsent] = useState<PrivacyConsent | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const loadConsent = async () => {
      try {
        const consentData = await fetchStudentPrivacyConsent(studentId);
        setConsent(consentData);
      } catch (error) {
        console.error("Error loading privacy consent:", error);
      } finally {
        setLoading(false);
      }
    };

    void loadConsent();
  }, [studentId]);

  if (loading) {
    return (
      <div className="text-gray-500 text-sm">
        Lade Datenschutzeinstellungen...
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <div>
        <span className="font-medium">{consent?.dataRetentionDays ?? 30} Tage</span>
        <p className="text-xs text-gray-500 mt-1">
          Besuchsdaten werden nach {consent?.dataRetentionDays ?? 30} Tagen automatisch gelöscht
        </p>
      </div>
      
      {consent && (
        <div className="text-sm space-y-1">
          <div>
            Einwilligung: {consent.accepted ? (
              <span className="text-green-600 font-medium">Erteilt</span>
            ) : (
              <span className="text-red-600 font-medium">Nicht erteilt</span>
            )}
          </div>
          {consent.acceptedAt && (
            <div className="text-gray-600">
              Erteilt am: {new Date(consent.acceptedAt).toLocaleDateString('de-DE')}
            </div>
          )}
          {consent.expiresAt && (
            <div className="text-gray-600">
              Gültig bis: {new Date(consent.expiresAt).toLocaleDateString('de-DE')}
            </div>
          )}
          {consent.renewalRequired && (
            <div className="mt-2 p-2 bg-yellow-100 rounded text-yellow-800 text-sm">
              ⚠️ Einwilligung muss erneuert werden
            </div>
          )}
        </div>
      )}
      
      {!consent && (
        <div className="text-gray-600 text-sm">
          Keine Datenschutzeinwilligung hinterlegt. Bitte im Bearbeiten-Modus konfigurieren.
        </div>
      )}
    </div>
  );
}

export default PrivacyConsentSection;