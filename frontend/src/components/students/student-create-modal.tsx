"use client";

import { useState, useEffect } from "react";
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

interface StudentCreateModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (data: Partial<Student>) => Promise<void>;
  readonly groups?: Array<{ readonly value: string; readonly label: string }>;
}

export function StudentCreateModal({
  isOpen,
  onClose,
  onCreate,
  groups = [],
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
    }
  }, [isOpen]);

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
    return handleStudentFormSubmit(
      e,
      formData,
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

        {/* Guardian Information - Note about managing after creation */}
        <div className="rounded-xl border border-purple-100 bg-purple-50/50 p-3 md:p-4">
          <div className="flex items-start gap-3">
            <div className="flex-shrink-0">
              <svg
                className="h-5 w-5 text-purple-600"
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
            </div>
            <div className="flex-1">
              <h3 className="mb-1 text-xs font-semibold text-gray-900 md:text-sm">
                Erziehungsberechtigte
              </h3>
              <p className="text-xs text-gray-600">
                Erziehungsberechtigte können nach der Erstellung des Schülers
                auf der Schülerdetailseite hinzugefügt und verwaltet werden.
                Dort stehen umfangreiche Optionen zur Verfügung (Kontaktdaten,
                Adressen, Abholberechtigungen, Notfallkontakte).
              </p>
            </div>
          </div>
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
  );
}
