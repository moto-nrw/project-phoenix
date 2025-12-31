"use client";

import { useState, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import type {
  GuardianFormData,
  GuardianWithRelationship,
} from "@/lib/guardian-helpers";
import { RELATIONSHIP_TYPES } from "@/lib/guardian-helpers";

interface GuardianFormModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (
    guardianData: GuardianFormData,
    relationshipData: RelationshipFormData,
  ) => Promise<void>;
  initialData?: GuardianWithRelationship;
  mode: "create" | "edit";
  isSubmitting?: boolean;
}

export interface RelationshipFormData {
  relationshipType: string;
  isPrimary: boolean;
  isEmergencyContact: boolean;
  canPickup: boolean;
  pickupNotes?: string;
  emergencyPriority: number;
}

export default function GuardianFormModal({
  isOpen,
  onClose,
  onSubmit,
  initialData,
  mode,
}: GuardianFormModalProps) {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Guardian profile data
  const [firstName, setFirstName] = useState(initialData?.firstName ?? "");
  const [lastName, setLastName] = useState(initialData?.lastName ?? "");
  const [email, setEmail] = useState(initialData?.email ?? "");
  const [phone, setPhone] = useState(initialData?.phone ?? "");
  const [mobilePhone, setMobilePhone] = useState(
    initialData?.mobilePhone ?? "",
  );

  // Relationship data
  const [relationshipType, setRelationshipType] = useState(
    initialData?.relationshipType ?? "parent",
  );
  const [isEmergencyContact, setIsEmergencyContact] = useState(
    initialData?.isEmergencyContact ?? false,
  );

  // Update form values when initialData changes
  useEffect(() => {
    if (initialData) {
      setFirstName(initialData.firstName ?? "");
      setLastName(initialData.lastName ?? "");
      setEmail(initialData.email ?? "");
      setPhone(initialData.phone ?? "");
      setMobilePhone(initialData.mobilePhone ?? "");
      setRelationshipType(initialData.relationshipType ?? "parent");
      setIsEmergencyContact(initialData.isEmergencyContact ?? false);
    } else {
      setFirstName("");
      setLastName("");
      setEmail("");
      setPhone("");
      setMobilePhone("");
      setRelationshipType("parent");
      setIsEmergencyContact(false);
    }
  }, [initialData]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // Validation
    if (!firstName.trim() || !lastName.trim()) {
      setError("Vorname und Nachname sind erforderlich");
      return;
    }

    if (!email.trim() && !phone.trim() && !mobilePhone.trim()) {
      setError(
        "Mindestens eine Kontaktmöglichkeit (E-Mail, Telefon oder Mobiltelefon) ist erforderlich",
      );
      return;
    }

    setIsLoading(true);

    try {
      const guardianData: GuardianFormData = {
        firstName: firstName.trim(),
        lastName: lastName.trim(),
        email: email.trim() || undefined,
        phone: phone.trim() || undefined,
        mobilePhone: mobilePhone.trim() || undefined,
      };

      const relationshipData: RelationshipFormData = {
        relationshipType,
        isPrimary: false,
        isEmergencyContact,
        canPickup: true,
        emergencyPriority: 1,
      };

      await onSubmit(guardianData, relationshipData);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Fehler beim Speichern");
    } finally {
      setIsLoading(false);
    }
  };

  const modalTitle =
    mode === "create"
      ? "Erziehungsberechtigte/n hinzufügen"
      : "Erziehungsberechtigte/n bearbeiten";

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={modalTitle}>
      <form onSubmit={handleSubmit} noValidate className="space-y-4 md:space-y-6">
        {/* Submit Error */}
        {error && (
          <div className="rounded-lg border border-red-200 bg-red-50 p-2 md:p-3">
            <p className="text-xs text-red-800 md:text-sm">{error}</p>
          </div>
        )}

        {/* Personal Information */}
        <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
          <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
            <svg
              className="h-3.5 w-3.5 text-blue-600 md:h-4 md:w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
              />
            </svg>
            Persönliche Daten
          </h3>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700">
                Vorname <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
                className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                placeholder="Max"
                required
                disabled={isLoading}
              />
            </div>

            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700">
                Nachname <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                placeholder="Mustermann"
                required
                disabled={isLoading}
              />
            </div>

            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700">
                Beziehung zum Kind
              </label>
              <div className="relative">
                <select
                  value={relationshipType}
                  onChange={(e) => setRelationshipType(e.target.value)}
                  className="block w-full appearance-none rounded-lg border border-gray-200 bg-white px-3 py-2 pr-10 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                  disabled={isLoading}
                >
                  {RELATIONSHIP_TYPES.map((type) => (
                    <option key={type.value} value={type.value}>
                      {type.label}
                    </option>
                  ))}
                </select>
                <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-3 text-gray-500">
                  <svg
                    className="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M19 9l-7 7-7-7"
                    />
                  </svg>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Contact Information */}
        <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
          <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
            <svg
              className="h-3.5 w-3.5 text-blue-600 md:h-4 md:w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
            Kontaktdaten
          </h3>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700">
                E-Mail
              </label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                placeholder="max.mustermann@example.com"
                disabled={isLoading}
              />
            </div>

            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700">
                Telefon
              </label>
              <input
                type="tel"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                placeholder="+49 123 456789"
                disabled={isLoading}
              />
            </div>

            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700">
                Mobiltelefon
              </label>
              <input
                type="tel"
                value={mobilePhone}
                onChange={(e) => setMobilePhone(e.target.value)}
                className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                placeholder="+49 170 1234567"
                disabled={isLoading}
              />
            </div>
          </div>
        </div>

        {/* Emergency Contact */}
        <div className="rounded-xl border border-red-200 bg-red-50 p-4">
          <label className="group flex cursor-pointer items-center gap-3">
            <input
              type="checkbox"
              checked={isEmergencyContact}
              onChange={(e) => setIsEmergencyContact(e.target.checked)}
              className="h-4 w-4 rounded border-gray-300 text-red-600 focus:ring-red-600"
              disabled={isLoading}
            />
            <div className="flex items-center gap-2">
              <svg
                className="h-5 w-5 text-red-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
              <span className="text-sm font-medium text-red-900">
                Notfallkontakt
              </span>
            </div>
          </label>
        </div>

        {/* Action Buttons */}
        <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex gap-2 border-t border-gray-100 bg-white/95 px-4 py-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:py-4">
          <button
            type="button"
            onClick={onClose}
            disabled={isLoading}
            className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-xs font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
          >
            Abbrechen
          </button>
          <button
            type="submit"
            disabled={isLoading}
            className="flex-1 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
          >
            {isLoading ? (
              <span className="flex items-center justify-center gap-2">
                <svg
                  className="h-4 w-4 animate-spin text-white"
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
                  />
                  <path
                    className="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  />
                </svg>
                Wird gespeichert...
              </span>
            ) : mode === "create" ? (
              "Hinzufügen"
            ) : (
              "Speichern"
            )}
          </button>
        </div>
      </form>
    </Modal>
  );
}
