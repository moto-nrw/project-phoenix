"use client";

import { useState, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import type { Student } from "@/lib/api";

interface StudentCreateModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreate: (data: Partial<Student>) => Promise<void>;
  groups?: Array<{ value: string; label: string }>;
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
      });
      setErrors({});
    }
  }, [isOpen]);

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.first_name?.trim()) {
      newErrors.first_name = "Vorname ist erforderlich";
    }
    if (!formData.second_name?.trim()) {
      newErrors.second_name = "Nachname ist erforderlich";
    }
    if (!formData.school_class?.trim()) {
      newErrors.school_class = "Klasse ist erforderlich";
    }
    if (
      formData.data_retention_days &&
      (formData.data_retention_days < 1 || formData.data_retention_days > 31)
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

      await onCreate(formData);
      // Modal is closed by parent component after successful creation
    } catch (error) {
      console.error("Error creating student:", error);
      setErrors({
        submit: "Fehler beim Erstellen. Bitte versuchen Sie es erneut.",
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

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Neuer Schüler">
      <form onSubmit={handleSubmit} className="space-y-4 md:space-y-6">
        {/* Submit Error */}
        {errors.submit && (
          <div className="rounded-lg border border-red-200 bg-red-50 p-2 md:p-3">
            <p className="text-xs text-red-800 md:text-sm">{errors.submit}</p>
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
                value={formData.first_name ?? ""}
                onChange={(e) => handleChange("first_name", e.target.value)}
                className={`block w-full rounded-lg border px-3 py-2 text-sm transition-colors ${
                  errors.first_name
                    ? "border-red-300 bg-red-50"
                    : "border-gray-200 bg-white focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                }`}
                placeholder="Max"
              />
              {errors.first_name && (
                <p className="mt-1 text-xs text-red-600">{errors.first_name}</p>
              )}
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700">
                Nachname <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={formData.second_name ?? ""}
                onChange={(e) => handleChange("second_name", e.target.value)}
                className={`block w-full rounded-lg border px-3 py-2 text-sm transition-colors ${
                  errors.second_name
                    ? "border-red-300 bg-red-50"
                    : "border-gray-200 bg-white focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                }`}
                placeholder="Mustermann"
              />
              {errors.second_name && (
                <p className="mt-1 text-xs text-red-600">
                  {errors.second_name}
                </p>
              )}
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700">
                Klasse <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={formData.school_class ?? ""}
                onChange={(e) => handleChange("school_class", e.target.value)}
                className={`block w-full rounded-lg border px-3 py-2 text-sm transition-colors ${
                  errors.school_class
                    ? "border-red-300 bg-red-50"
                    : "border-gray-200 bg-white focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                }`}
                placeholder="5A"
              />
              {errors.school_class && (
                <p className="mt-1 text-xs text-red-600">
                  {errors.school_class}
                </p>
              )}
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700">
                Gruppe
              </label>
              <div className="relative">
                <select
                  value={formData.group_id ?? ""}
                  onChange={(e) =>
                    handleChange("group_id", e.target.value || null)
                  }
                  className="block w-full appearance-none rounded-lg border border-gray-200 bg-white px-3 py-2 pr-10 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                >
                  <option value="">Keine Gruppe</option>
                  {groups.map((group) => (
                    <option key={group.value} value={group.value}>
                      {group.label}
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
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700">
                Geburtstag
              </label>
              <input
                type="date"
                value={formData.birthday ?? ""}
                onChange={(e) => handleChange("birthday", e.target.value)}
                className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
              />
            </div>
          </div>
        </div>

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

        {/* Health Information */}
        <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
          <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
            <svg
              className="h-3.5 w-3.5 text-red-600 md:h-4 md:w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"
              />
            </svg>
            Gesundheitsinformationen
          </h3>
          <textarea
            value={formData.health_info ?? ""}
            onChange={(e) => handleChange("health_info", e.target.value)}
            rows={3}
            className="block w-full resize-none rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] md:text-sm"
            placeholder="Allergien, Medikamente, gesundheitliche Einschränkungen..."
          />
        </div>

        {/* Supervisor Notes */}
        <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
          <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
            <svg
              className="h-3.5 w-3.5 text-amber-600 md:h-4 md:w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
              />
            </svg>
            Betreuernotizen
          </h3>
          <textarea
            value={formData.supervisor_notes ?? ""}
            onChange={(e) => handleChange("supervisor_notes", e.target.value)}
            rows={3}
            className="block w-full resize-none rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] md:text-sm"
            placeholder="Interne Notizen für Betreuer..."
          />
        </div>

        {/* Additional Information */}
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
                d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
              />
            </svg>
            Elternnotizen
          </h3>
          <textarea
            value={formData.extra_info ?? ""}
            onChange={(e) => handleChange("extra_info", e.target.value)}
            rows={3}
            className="block w-full resize-none rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] md:text-sm"
            placeholder="Weitere Informationen über den Schüler..."
          />
        </div>

        {/* Privacy & Data Retention */}
        <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
          <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
            <svg
              className="h-3.5 w-3.5 text-gray-600 md:h-4 md:w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
              />
            </svg>
            Datenschutz
          </h3>
          <div className="space-y-4">
            <label className="group flex cursor-pointer items-start gap-3">
              <input
                type="checkbox"
                checked={formData.privacy_consent_accepted ?? false}
                onChange={(e) =>
                  handleChange("privacy_consent_accepted", e.target.checked)
                }
                className="mt-0.5 h-4 w-4 rounded border-gray-300 text-[#5080D8] focus:ring-[#5080D8]"
              />
              <span className="text-sm text-gray-700 transition-colors group-hover:text-gray-900">
                Einwilligung zur Datenverarbeitung erteilt
              </span>
            </label>
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700">
                Aufbewahrungsdauer (Tage)
              </label>
              <input
                type="number"
                min="1"
                max="31"
                value={formData.data_retention_days ?? 30}
                onChange={(e) => {
                  const v = parseInt(e.target.value, 10);
                  handleChange("data_retention_days", Number.isNaN(v) ? 30 : v);
                }}
                className={`block w-full rounded-lg border px-3 py-2 text-sm transition-colors ${
                  errors.data_retention_days
                    ? "border-red-300 bg-red-50"
                    : "border-gray-200 bg-white focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                }`}
              />
              {errors.data_retention_days && (
                <p className="mt-1 text-xs text-red-600">
                  {errors.data_retention_days}
                </p>
              )}
              <p className="mt-1 text-xs text-gray-500">
                Daten werden nach dieser Zeit automatisch gelöscht (1-31 Tage)
              </p>
            </div>
          </div>
        </div>

        {/* Bus Status */}
        <div className="rounded-xl border border-orange-200 bg-orange-50 p-4">
          <label className="group flex cursor-pointer items-center gap-3">
            <input
              type="checkbox"
              checked={formData.bus ?? false}
              onChange={(e) => handleChange("bus", e.target.checked)}
              className="h-4 w-4 rounded border-gray-300 text-orange-600 focus:ring-orange-600"
            />
            <div className="flex items-center gap-2">
              <svg
                className="h-5 w-5 text-orange-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
                />
              </svg>
              <span className="text-sm font-medium text-orange-900">
                Fährt mit dem Bus
              </span>
            </div>
          </label>
        </div>

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
