"use client";

import { useState } from "react";
import { Modal } from "~/components/ui/modal";
import type {
  GuardianFormData,
  GuardianWithRelationship,
} from "@/lib/guardian-helpers";
import {
  RELATIONSHIP_TYPES,
  CONTACT_METHODS,
  LANGUAGE_PREFERENCES,
} from "@/lib/guardian-helpers";

interface GuardianFormModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (guardianData: GuardianFormData, relationshipData: RelationshipFormData) => Promise<void>;
  initialData?: GuardianWithRelationship;
  mode: "create" | "edit";
  existingGuardianId?: string; // If linking existing guardian
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
  existingGuardianId,
}: GuardianFormModalProps) {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Guardian profile data
  const [firstName, setFirstName] = useState(initialData?.firstName || "");
  const [lastName, setLastName] = useState(initialData?.lastName || "");
  const [email, setEmail] = useState(initialData?.email || "");
  const [phone, setPhone] = useState(initialData?.phone || "");
  const [mobilePhone, setMobilePhone] = useState(initialData?.mobilePhone || "");
  const [addressStreet, setAddressStreet] = useState(initialData?.addressStreet || "");
  const [addressCity, setAddressCity] = useState(initialData?.addressCity || "");
  const [addressPostalCode, setAddressPostalCode] = useState(initialData?.addressPostalCode || "");
  const [preferredContactMethod, setPreferredContactMethod] = useState(
    initialData?.preferredContactMethod || "email"
  );
  const [languagePreference, setLanguagePreference] = useState(
    initialData?.languagePreference || "de"
  );
  const [occupation, setOccupation] = useState(initialData?.occupation || "");
  const [employer, setEmployer] = useState(initialData?.employer || "");
  const [notes, setNotes] = useState(initialData?.notes || "");

  // Relationship data
  const [relationshipType, setRelationshipType] = useState(
    initialData?.relationshipType || "parent"
  );
  const [isPrimary, setIsPrimary] = useState(initialData?.isPrimary || false);
  const [isEmergencyContact, setIsEmergencyContact] = useState(
    initialData?.isEmergencyContact || true
  );
  const [canPickup, setCanPickup] = useState(initialData?.canPickup || true);
  const [pickupNotes, setPickupNotes] = useState(initialData?.pickupNotes || "");
  const [emergencyPriority, setEmergencyPriority] = useState(
    initialData?.emergencyPriority || 1
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // Validation
    if (!firstName.trim() || !lastName.trim()) {
      setError("Vorname und Nachname sind erforderlich");
      return;
    }

    if (!email.trim() && !phone.trim() && !mobilePhone.trim()) {
      setError("Mindestens eine Kontaktmöglichkeit (E-Mail, Telefon oder Mobiltelefon) ist erforderlich");
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
        addressStreet: addressStreet.trim() || undefined,
        addressCity: addressCity.trim() || undefined,
        addressPostalCode: addressPostalCode.trim() || undefined,
        preferredContactMethod,
        languagePreference,
        occupation: occupation.trim() || undefined,
        employer: employer.trim() || undefined,
        notes: notes.trim() || undefined,
      };

      const relationshipData: RelationshipFormData = {
        relationshipType,
        isPrimary,
        isEmergencyContact,
        canPickup,
        pickupNotes: pickupNotes.trim() || undefined,
        emergencyPriority,
      };

      await onSubmit(guardianData, relationshipData);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Fehler beim Speichern");
    } finally {
      setIsLoading(false);
    }
  };

  const modalTitle = mode === "create"
    ? "Erziehungsberechtigte/n hinzufügen"
    : "Erziehungsberechtigte/n bearbeiten";

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={modalTitle}>
      <form onSubmit={handleSubmit} className="space-y-6">

        {error && (
          <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
            {error}
          </div>
        )}

        {/* Personal Information Section */}
        <div className="bg-purple-50 p-4 rounded-lg space-y-4">
          <h3 className="font-semibold text-purple-800 flex items-center gap-2">
            Persönliche Daten
          </h3>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Vorname <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-purple-500 focus:border-transparent"
                required
                disabled={isLoading}
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Nachname <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-purple-500 focus:border-transparent"
                required
                disabled={isLoading}
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Beziehung zum Kind
              </label>
              <select
                value={relationshipType}
                onChange={(e) => setRelationshipType(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-purple-500 focus:border-transparent"
                disabled={isLoading}
              >
                {RELATIONSHIP_TYPES.map((type) => (
                  <option key={type.value} value={type.value}>
                    {type.label}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Sprachpräferenz
              </label>
              <select
                value={languagePreference}
                onChange={(e) => setLanguagePreference(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-purple-500 focus:border-transparent"
                disabled={isLoading}
              >
                {LANGUAGE_PREFERENCES.map((lang) => (
                  <option key={lang.value} value={lang.value}>
                    {lang.label}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </div>

        {/* Contact Information Section */}
        <div className="bg-blue-50 p-4 rounded-lg space-y-4">
          <h3 className="font-semibold text-blue-800">Kontaktdaten</h3>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                E-Mail
              </label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                disabled={isLoading}
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Telefon
              </label>
              <input
                type="tel"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                disabled={isLoading}
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Mobiltelefon
              </label>
              <input
                type="tel"
                value={mobilePhone}
                onChange={(e) => setMobilePhone(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                disabled={isLoading}
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Bevorzugter Kontaktweg
              </label>
              <select
                value={preferredContactMethod}
                onChange={(e) => setPreferredContactMethod(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                disabled={isLoading}
              >
                {CONTACT_METHODS.map((method) => (
                  <option key={method.value} value={method.value}>
                    {method.label}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </div>

        {/* Address Section */}
        <div className="bg-gray-50 p-4 rounded-lg space-y-4">
          <h3 className="font-semibold text-gray-800">Adresse (optional)</h3>

          <div className="grid grid-cols-1 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Straße und Hausnummer
              </label>
              <input
                type="text"
                value={addressStreet}
                onChange={(e) => setAddressStreet(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-gray-500 focus:border-transparent"
                disabled={isLoading}
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Postleitzahl
                </label>
                <input
                  type="text"
                  value={addressPostalCode}
                  onChange={(e) => setAddressPostalCode(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-gray-500 focus:border-transparent"
                  disabled={isLoading}
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Stadt
                </label>
                <input
                  type="text"
                  value={addressCity}
                  onChange={(e) => setAddressCity(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-gray-500 focus:border-transparent"
                  disabled={isLoading}
                />
              </div>
            </div>
          </div>
        </div>

        {/* Permissions Section */}
        <div className="bg-green-50 p-4 rounded-lg space-y-4">
          <h3 className="font-semibold text-green-800">Berechtigungen</h3>

          <div className="space-y-3">
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={isPrimary}
                onChange={(e) => setIsPrimary(e.target.checked)}
                className="rounded border-gray-300 text-green-600 focus:ring-green-500"
                disabled={isLoading}
              />
              <span className="text-sm font-medium text-gray-700">
                Primäre/r Erziehungsberechtigte/r
              </span>
            </label>

            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={isEmergencyContact}
                onChange={(e) => setIsEmergencyContact(e.target.checked)}
                className="rounded border-gray-300 text-green-600 focus:ring-green-500"
                disabled={isLoading}
              />
              <span className="text-sm font-medium text-gray-700">
                Notfallkontakt
              </span>
            </label>

            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={canPickup}
                onChange={(e) => setCanPickup(e.target.checked)}
                className="rounded border-gray-300 text-green-600 focus:ring-green-500"
                disabled={isLoading}
              />
              <span className="text-sm font-medium text-gray-700">
                Abholberechtigt
              </span>
            </label>

            {isEmergencyContact && (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Notfall-Priorität (1 = höchste)
                </label>
                <input
                  type="number"
                  min="1"
                  max="10"
                  value={emergencyPriority}
                  onChange={(e) => setEmergencyPriority(parseInt(e.target.value))}
                  className="w-32 px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-transparent"
                  disabled={isLoading}
                />
              </div>
            )}

            {canPickup && (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Abholhinweise
                </label>
                <textarea
                  value={pickupNotes}
                  onChange={(e) => setPickupNotes(e.target.value)}
                  rows={2}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-transparent"
                  placeholder="z.B. nur mit Ausweis, nur an bestimmten Tagen..."
                  disabled={isLoading}
                />
              </div>
            )}
          </div>
        </div>

        {/* Additional Information */}
        <div className="bg-yellow-50 p-4 rounded-lg space-y-4">
          <h3 className="font-semibold text-yellow-800">Weitere Informationen (optional)</h3>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Beruf
              </label>
              <input
                type="text"
                value={occupation}
                onChange={(e) => setOccupation(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-yellow-500 focus:border-transparent"
                disabled={isLoading}
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Arbeitgeber
              </label>
              <input
                type="text"
                value={employer}
                onChange={(e) => setEmployer(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-yellow-500 focus:border-transparent"
                disabled={isLoading}
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Notizen
            </label>
            <textarea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              rows={3}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-yellow-500 focus:border-transparent"
              placeholder="Weitere Informationen..."
              disabled={isLoading}
            />
          </div>
        </div>

        {/* Actions */}
        <div className="flex gap-3 justify-end pt-4 border-t">
          <button
            type="button"
            onClick={onClose}
            className="px-6 py-2 border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors"
            disabled={isLoading}
          >
            Abbrechen
          </button>
          <button
            type="submit"
            className="px-6 py-2 bg-gradient-to-r from-teal-500 to-blue-500 text-white rounded-lg hover:from-teal-600 hover:to-blue-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            disabled={isLoading}
          >
            {isLoading ? "Speichern..." : mode === "create" ? "Hinzufügen" : "Speichern"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
