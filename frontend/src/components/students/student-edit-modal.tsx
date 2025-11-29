"use client";

import { useState, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import type { Student } from "@/lib/api";
import {
  PersonalInfoSection,
  HealthInfoSection,
  SupervisorNotesSection,
  AdditionalInfoSection,
  PrivacyConsentSection,
  BusStatusSection,
  PickupStatusSection,
} from "./student-form-fields";

interface StudentEditModalProps {
  isOpen: boolean;
  onClose: () => void;
  student: Student | null;
  onSave: (data: Partial<Student>) => Promise<void>;
  loading?: boolean;
  groups?: Array<{ value: string; label: string }>;
}

export function StudentEditModal({
  isOpen,
  onClose,
  student,
  onSave,
  loading = false,
  groups = [],
}: StudentEditModalProps) {
  const [formData, setFormData] = useState<Partial<Student>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saveLoading, setSaveLoading] = useState(false);

  // Initialize form data when student changes
  useEffect(() => {
    if (student) {
      setFormData({
        first_name: student.first_name ?? "",
        second_name: student.second_name ?? "",
        school_class: student.school_class ?? "",
        group_id: student.group_id ?? "",
        birthday: student.birthday ?? "",
        health_info: student.health_info ?? "",
        supervisor_notes: student.supervisor_notes ?? "",
        extra_info: student.extra_info ?? "",
        privacy_consent_accepted: student.privacy_consent_accepted ?? false,
        data_retention_days: student.data_retention_days ?? 30,
        bus: student.bus ?? false,
        pickup_status: student.pickup_status ?? "",
      });
      setErrors({});
    }
  }, [student]);

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.first_name?.trim()) {
      newErrors.first_name = "Vorname ist erforderlich";
    }
    if (!formData.second_name?.trim()) {
      newErrors.second_name = "Nachname ist erforderlich";
    }
    // Validate data retention days - must be set and in valid range
    if (
      formData.data_retention_days === null ||
      formData.data_retention_days === undefined
    ) {
      newErrors.data_retention_days =
        "Aufbewahrungsdauer ist erforderlich (1-31 Tage)";
    } else if (
      formData.data_retention_days < 1 ||
      formData.data_retention_days > 31
    ) {
      newErrors.data_retention_days =
        "Aufbewahrungsdauer muss zwischen 1 und 31 Tagen liegen";
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm()) {
      return;
    }

    try {
      setSaveLoading(true);

      await onSave(formData);
    } catch (error) {
      console.error("Error saving student:", error);
      setErrors({
        submit: "Fehler beim Speichern. Bitte versuchen Sie es erneut.",
      });
    } finally {
      setSaveLoading(false);
    }
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

  if (!student) return null;

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Schüler bearbeiten">
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#5080D8]"></div>
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : (
        <form onSubmit={handleSubmit} className="space-y-4 md:space-y-6">
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
            requiredFields={{ firstName: true, lastName: true, schoolClass: false }}
          />

          {/* Guardian Information - Link to Student Detail Page */}
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
                  Erziehungsberechtigte verwalten
                </h3>
                <p className="mb-3 text-xs text-gray-600">
                  Umfangreiche Verwaltung von Erziehungsberechtigten (inkl.
                  Kontaktdaten, Adressen, Abholberechtigungen und
                  Notfallkontakte) ist auf der Schülerdetailseite verfügbar.
                </p>
                <a
                  href={`/students/${student.id}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 rounded-lg bg-purple-100 px-3 py-1.5 text-xs font-medium text-purple-700 transition-colors hover:bg-purple-200"
                >
                  Zur Schülerdetailseite
                  <svg
                    className="h-3 w-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                    />
                  </svg>
                </a>
              </div>
            </div>
          </div>

          {/* Health Information */}
          <HealthInfoSection
            value={formData.health_info}
            onChange={(v) => handleChange("health_info", v)}
          />

          {/* Supervisor Notes */}
          <SupervisorNotesSection
            value={formData.supervisor_notes}
            onChange={(v) => handleChange("supervisor_notes", v)}
          />

          {/* Additional Information */}
          <AdditionalInfoSection
            value={formData.extra_info}
            onChange={(v) => handleChange("extra_info", v)}
          />

          {/* Privacy & Data Retention */}
          <PrivacyConsentSection
            formData={formData}
            onChange={handleChange}
            errors={errors}
          />

          {/* Pickup Status */}
          <PickupStatusSection
            value={formData.pickup_status}
            onChange={(v) => handleChange("pickup_status", v)}
          />

          {/* Bus Status */}
          <BusStatusSection
            value={formData.bus}
            onChange={(v) => handleChange("bus", v)}
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
                  Wird gespeichert...
                </span>
              ) : (
                "Speichern"
              )}
            </button>
          </div>
        </form>
      )}
    </Modal>
  );
}
