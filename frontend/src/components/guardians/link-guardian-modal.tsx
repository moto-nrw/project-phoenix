"use client";

import { useState, useEffect } from "react";
import { ArrowLeft, Loader2, Users } from "lucide-react";
import { Modal } from "~/components/ui/modal";
import SearchableGuardianSelect from "./searchable-guardian-select";
import type { GuardianSearchResult } from "@/lib/guardian-helpers";
import { RELATIONSHIP_TYPES } from "@/lib/guardian-helpers";

interface RelationshipConfig {
  relationshipType: string;
  isPrimary: boolean;
  isEmergencyContact: boolean;
  canPickup: boolean;
  emergencyPriority: number;
}

interface LinkGuardianModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onLink: (
    guardianId: string,
    config: RelationshipConfig,
  ) => Promise<void>;
  readonly studentId: string;
}

type Step = "search" | "configure";

export default function LinkGuardianModal({
  isOpen,
  onClose,
  onLink,
  studentId,
}: LinkGuardianModalProps) {
  const [step, setStep] = useState<Step>("search");
  const [selectedGuardian, setSelectedGuardian] =
    useState<GuardianSearchResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Relationship configuration
  const [relationshipType, setRelationshipType] = useState("parent");
  const [isPrimary, setIsPrimary] = useState(false);
  const [isEmergencyContact, setIsEmergencyContact] = useState(false);
  const [canPickup, setCanPickup] = useState(true);

  // Reset state when modal opens/closes
  useEffect(() => {
    if (isOpen) {
      setStep("search");
      setSelectedGuardian(null);
      setError(null);
      setRelationshipType("parent");
      setIsPrimary(false);
      setIsEmergencyContact(false);
      setCanPickup(true);
    }
  }, [isOpen]);

  // Handle guardian selection
  const handleGuardianSelect = (guardian: GuardianSearchResult) => {
    setSelectedGuardian(guardian);
    setStep("configure");
    setError(null);
  };

  // Go back to search step
  const handleBack = () => {
    setStep("search");
    setSelectedGuardian(null);
    setError(null);
  };

  // Submit the link
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!selectedGuardian) return;

    setIsLoading(true);
    setError(null);

    try {
      await onLink(selectedGuardian.id, {
        relationshipType,
        isPrimary,
        isEmergencyContact,
        canPickup,
        emergencyPriority: 1,
      });
      onClose();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Fehler beim Verknüpfen",
      );
    } finally {
      setIsLoading(false);
    }
  };

  // Format student list for display
  const formatStudentList = (students: GuardianSearchResult["students"]) => {
    if (students.length === 0) return "Noch keinem Schüler zugeordnet";
    return students
      .map((s) => `${s.firstName} ${s.lastName} (${s.schoolClass})`)
      .join(", ");
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={
        step === "search"
          ? "Bestehenden Erziehungsberechtigten verknüpfen"
          : "Beziehung konfigurieren"
      }
    >
      {/* Error Display */}
      {error && (
        <div className="mb-4 rounded-lg border border-red-200 bg-red-50 p-3">
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      {/* Step 1: Search */}
      {step === "search" && (
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Suchen Sie nach einem bestehenden Erziehungsberechtigten, um ihn mit
            diesem Schüler zu verknüpfen. Dies ist besonders nützlich für
            Geschwisterkinder.
          </p>

          <SearchableGuardianSelect
            onSelect={handleGuardianSelect}
            excludeStudentId={studentId}
            disabled={isLoading}
          />

          {/* Action Buttons */}
          <div className="flex justify-end gap-2 border-t border-gray-100 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
            >
              Abbrechen
            </button>
          </div>
        </div>
      )}

      {/* Step 2: Configure Relationship */}
      {step === "configure" && selectedGuardian && (
        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Back Button */}
          <button
            type="button"
            onClick={handleBack}
            className="flex items-center gap-1.5 text-sm text-gray-600 transition-colors hover:text-gray-900"
          >
            <ArrowLeft className="h-4 w-4" />
            Zurück zur Suche
          </button>

          {/* Selected Guardian Card */}
          <div className="rounded-xl border border-gray-200 bg-gray-50 p-4">
            <div className="flex items-start gap-3">
              <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-purple-100 text-purple-600">
                <Users className="h-5 w-5" />
              </div>
              <div className="min-w-0 flex-1">
                <h4 className="font-medium text-gray-900">
                  {selectedGuardian.firstName} {selectedGuardian.lastName}
                </h4>
                {selectedGuardian.email && (
                  <p className="text-sm text-gray-500">
                    {selectedGuardian.email}
                  </p>
                )}
                {selectedGuardian.phone && (
                  <p className="text-sm text-gray-500">
                    {selectedGuardian.phone}
                  </p>
                )}
                {selectedGuardian.students.length > 0 && (
                  <p className="mt-1 text-xs text-gray-600">
                    <span className="font-medium">Bereits zugeordnet zu:</span>{" "}
                    {formatStudentList(selectedGuardian.students)}
                  </p>
                )}
              </div>
            </div>
          </div>

          {/* Relationship Configuration */}
          <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-4">
            <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
              <svg
                className="h-4 w-4 text-blue-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
                />
              </svg>
              Beziehung zum Kind
            </h3>

            {/* Relationship Type */}
            <div className="mb-4">
              <label
                htmlFor="relationship-type"
                className="mb-1 block text-xs font-medium text-gray-700"
              >
                Beziehungstyp
              </label>
              <div className="relative">
                <select
                  id="relationship-type"
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

            {/* Checkboxes */}
            <div className="space-y-3">
              {/* Primary Contact */}
              <label className="flex cursor-pointer items-center gap-3">
                <input
                  type="checkbox"
                  checked={isPrimary}
                  onChange={(e) => setIsPrimary(e.target.checked)}
                  className="h-4 w-4 rounded border-gray-300 text-[#5080D8] focus:ring-[#5080D8]"
                  disabled={isLoading}
                />
                <span className="text-sm text-gray-700">
                  Hauptansprechpartner
                </span>
              </label>

              {/* Can Pickup */}
              <label className="flex cursor-pointer items-center gap-3">
                <input
                  type="checkbox"
                  checked={canPickup}
                  onChange={(e) => setCanPickup(e.target.checked)}
                  className="h-4 w-4 rounded border-gray-300 text-[#5080D8] focus:ring-[#5080D8]"
                  disabled={isLoading}
                />
                <span className="text-sm text-gray-700">
                  Darf das Kind abholen
                </span>
              </label>
            </div>
          </div>

          {/* Emergency Contact */}
          <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3">
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
          <div className="flex gap-2 border-t border-gray-100 pt-4">
            <button
              type="button"
              onClick={onClose}
              disabled={isLoading}
              className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
            >
              Abbrechen
            </button>
            <button
              type="submit"
              disabled={isLoading}
              className="flex flex-1 items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {isLoading ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Wird verknüpft...
                </>
              ) : (
                "Verknüpfen"
              )}
            </button>
          </div>
        </form>
      )}
    </Modal>
  );
}
