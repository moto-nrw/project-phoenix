"use client";

import { useState, useEffect } from "react";
import { Users, Plus, Trash2 } from "lucide-react";
import { Modal } from "~/components/ui/modal";
import type { Student } from "@/lib/api";
import {
  PersonalInfoSection,
  BusStatusSection,
  PickupStatusSection,
} from "./student-form-fields";
import { StudentCommonFormSections } from "./student-common-form-sections";
import {
  validateStudentForm,
  handleStudentFormSubmit,
} from "~/lib/student-form-validation";
import GuardianFormModal, {
  type RelationshipFormData,
} from "~/components/guardians/guardian-form-modal";
import {
  RELATIONSHIP_TYPES,
  type GuardianFormData,
  type PhoneType,
  type StudentGuardianPayload,
} from "@/lib/guardian-helpers";

// Shape returned by GuardianFormModal's onSubmit for each entry.
type GuardianSubmitEntry = {
  id: string;
  guardianData: GuardianFormData;
  relationshipData: RelationshipFormData;
  phoneNumbers: Array<{
    id?: string;
    phoneNumber: string;
    phoneType: PhoneType;
    label?: string;
    isPrimary: boolean;
  }>;
};

// Maps GuardianFormModal entries onto the snake_case payload the create-student
// endpoint expects, so the same component used on the detail page feeds the
// atomic create flow without rebuilding any guardian fields.
function toGuardianPayloads(
  entries: GuardianSubmitEntry[],
): StudentGuardianPayload[] {
  return entries.map((entry) => ({
    first_name: entry.guardianData.firstName,
    last_name: entry.guardianData.lastName,
    email: entry.guardianData.email,
    address_street: entry.guardianData.addressStreet,
    address_city: entry.guardianData.addressCity,
    address_postal_code: entry.guardianData.addressPostalCode,
    // preferred_contact_method and pickup_notes are part of the shared guardian
    // types but GuardianFormModal's create mode does not expose inputs for them
    // yet, so they arrive undefined today. Mapped through deliberately so they
    // flow automatically once the form adds those fields (the backend applies
    // its own contact-method default when omitted).
    preferred_contact_method: entry.guardianData.preferredContactMethod,
    language_preference: entry.guardianData.languagePreference,
    notes: entry.guardianData.notes,
    relationship_type: entry.relationshipData.relationshipType,
    is_primary: entry.relationshipData.isPrimary,
    is_emergency_contact: entry.relationshipData.isEmergencyContact,
    can_pickup: entry.relationshipData.canPickup,
    pickup_notes: entry.relationshipData.pickupNotes,
    emergency_priority: entry.relationshipData.emergencyPriority,
    phone_numbers: entry.phoneNumbers.map((phone) => ({
      phone_number: phone.phoneNumber,
      phone_type: phone.phoneType,
      label: phone.label,
      is_primary: phone.isPrimary,
    })),
  }));
}

// Human-readable relationship label for the summary list.
function relationshipLabel(value: string): string {
  return RELATIONSHIP_TYPES.find((t) => t.value === value)?.label ?? value;
}

interface StudentCreateModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (
    data: Partial<Student> & { guardians?: StudentGuardianPayload[] },
  ) => Promise<void>;
  readonly groups?: Array<{ readonly value: string; readonly label: string }>;
}

const EMPTY_GROUPS: NonNullable<StudentCreateModalProps["groups"]> = [];

export function StudentCreateModal({
  isOpen,
  onClose,
  onCreate,
  groups = EMPTY_GROUPS,
}: StudentCreateModalProps) {
  const [formData, setFormData] = useState<Partial<Student>>({
    first_name: "",
    second_name: "",
    school_class: "",
    group_id: "",
    birthday: "",
    health_info: "",
    supervisor_notes: "",
    extra_info: "",
    privacy_consent_accepted: false,
    data_retention_days: 30,
    bus: false,
    pickup_status: "",
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saveLoading, setSaveLoading] = useState(false);
  const [guardians, setGuardians] = useState<StudentGuardianPayload[]>([]);
  const [guardianModalOpen, setGuardianModalOpen] = useState(false);

  // Reset form when modal opens/closes
  useEffect(() => {
    if (isOpen) {
      setFormData({
        first_name: "",
        second_name: "",
        school_class: "",
        group_id: "",
        birthday: "",
        health_info: "",
        supervisor_notes: "",
        extra_info: "",
        privacy_consent_accepted: false,
        data_retention_days: 30,
        bus: false,
        pickup_status: "",
      });
      setErrors({});
      setGuardians([]);
      setGuardianModalOpen(false);
    }
  }, [isOpen]);

  // Collect guardians from the reused GuardianFormModal into local state. The
  // guardians are persisted together with the student in one request — no API
  // calls happen here.
  const handleGuardianSubmit = async (entries: GuardianSubmitEntry[]) => {
    setGuardians((prev) => [...prev, ...toGuardianPayloads(entries)]);
    setGuardianModalOpen(false);
  };

  const removeGuardian = (index: number) => {
    setGuardians((prev) => prev.filter((_, i) => i !== index));
  };

  const validateForm = (): boolean => {
    const newErrors = validateStudentForm(formData, {
      firstName: true,
      lastName: true,
      schoolClass: true,
    });
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = (e: React.FormEvent) => {
    const payload: Partial<Student> & { guardians?: StudentGuardianPayload[] } =
      guardians.length > 0 ? { ...formData, guardians } : formData;
    return handleStudentFormSubmit(
      e,
      payload,
      validateForm,
      onCreate,
      setSaveLoading,
      setErrors,
    );
  };

  const handleChange = (
    field: keyof Student,
    value: string | boolean | number | null,
  ) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
    // Clear error for this field
    if (errors[field]) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[field];
        return newErrors;
      });
    }
  };

  return (
    <>
      <Modal isOpen={isOpen} onClose={onClose} title="Neuer Schüler">
        <form
          onSubmit={handleSubmit}
          noValidate
          className="space-y-4 md:space-y-6"
        >
          {/* Submit Error */}
          {errors.submit && (
            <div className="rounded-lg border border-red-200 bg-red-50 p-2 md:p-3">
              <p className="text-xs text-red-800 md:text-sm">{errors.submit}</p>
            </div>
          )}

          {/* Personal Information */}
          <PersonalInfoSection
            formData={formData}
            onChange={handleChange}
            errors={errors}
            groups={groups}
          />

          {/* Guardian Information — add directly during creation. Styling
              matches the other modal sections (border-gray-100 + blue tint,
              blue heading icon) and reuses the guardian modal's own dashed
              add-button and red remove patterns. */}
          <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
            <div className="mb-3 flex items-center justify-between md:mb-4">
              <h3 className="flex items-center gap-2 text-xs font-semibold text-gray-900 md:text-sm">
                <Users className="h-3.5 w-3.5 text-blue-600 md:h-4 md:w-4" />
                Erziehungsberechtigte
              </h3>
              <span className="text-xs text-gray-500">
                {guardians.length > 0
                  ? `${guardians.length} hinzugefügt`
                  : "Optional"}
              </span>
            </div>

            {guardians.length > 0 && (
              <ul className="mb-3 space-y-2">
                {guardians.map((guardian, index) => {
                  const flags = [
                    guardian.can_pickup ? "Abholberechtigt" : null,
                    guardian.is_emergency_contact ? "Notfallkontakt" : null,
                    guardian.is_primary ? "Primär" : null,
                  ].filter(Boolean);
                  return (
                    <li
                      key={`${guardian.first_name}-${guardian.last_name}-${index}`}
                      className="flex items-start justify-between gap-2 rounded-lg border border-gray-100 bg-white p-2 md:p-3"
                    >
                      <div className="min-w-0">
                        <p className="truncate text-xs font-medium text-gray-900 md:text-sm">
                          {`${guardian.first_name} ${guardian.last_name}`.trim() ||
                            "Erziehungsberechtigte/r"}
                          <span className="ml-2 font-normal text-gray-500">
                            {relationshipLabel(guardian.relationship_type)}
                          </span>
                        </p>
                        {flags.length > 0 && (
                          <p className="mt-0.5 truncate text-xs text-gray-500">
                            {flags.join(" · ")}
                          </p>
                        )}
                      </div>
                      <button
                        type="button"
                        onClick={() => removeGuardian(index)}
                        disabled={saveLoading}
                        aria-label="Erziehungsberechtigte/n entfernen"
                        className="flex flex-shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-xs text-red-600 transition-colors hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </li>
                  );
                })}
              </ul>
            )}

            <button
              type="button"
              onClick={() => setGuardianModalOpen(true)}
              disabled={saveLoading}
              className="flex w-full items-center justify-center gap-2 rounded-xl border-2 border-dashed border-gray-300 bg-gray-50 py-2 text-xs font-medium text-gray-600 transition-all duration-200 hover:border-gray-400 hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm"
            >
              <Plus className="h-4 w-4" />
              Erziehungsberechtigte hinzufügen
            </button>
          </div>

          {/* Common Form Sections */}
          <StudentCommonFormSections
            formData={formData}
            errors={errors}
            onChange={handleChange}
          />

          {/* Bus Status */}
          <BusStatusSection
            value={formData.bus}
            onChange={(v) => handleChange("bus", v)}
          />

          {/* Pickup Status */}
          <PickupStatusSection
            value={formData.pickup_status}
            onChange={(v) => handleChange("pickup_status", v)}
          />

          {/* Action Buttons */}
          <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex gap-2 border-t border-gray-100 bg-white/95 px-4 py-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:py-4">
            <button
              type="button"
              onClick={onClose}
              disabled={saveLoading}
              className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-xs font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
            >
              Abbrechen
            </button>
            <button
              type="submit"
              disabled={saveLoading}
              className="flex-1 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
            >
              {saveLoading ? (
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
                    ></circle>
                    <path
                      className="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    ></path>
                  </svg>
                  Wird erstellt...
                </span>
              ) : (
                "Erstellen"
              )}
            </button>
          </div>
        </form>
      </Modal>

      {/* Reuse the existing guardian form: collect guardians without firing
          API calls — they are persisted atomically with the student. */}
      {guardianModalOpen && (
        <GuardianFormModal
          isOpen={guardianModalOpen}
          mode="create"
          onClose={() => setGuardianModalOpen(false)}
          onSubmit={handleGuardianSubmit}
        />
      )}
    </>
  );
}
