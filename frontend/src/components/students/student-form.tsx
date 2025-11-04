"use client";

import { useState, useEffect } from "react";
import type { Student } from "@/lib/api";
import { GroupSelector } from "@/components/groups";

interface StudentFormProps {
  initialData?: Partial<Student>;
  onSubmitAction: (studentData: Partial<Student>) => Promise<void>;
  onCancelAction: () => void;
  isLoading: boolean;
  formTitle: string;
  submitLabel: string;
}

export default function StudentForm({
  initialData,
  onSubmitAction,
  onCancelAction,
  isLoading,
  formTitle,
  submitLabel,
}: StudentFormProps) {
  const [formData, setFormData] = useState({
    first_name: "",
    second_name: "",
    school_class: "",
    name_lg: "",
    contact_lg: "",
    group_id: "",
    bus: false,
    in_house: false,
    wc: false,
    school_yard: false,
  });

  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (initialData) {
      setFormData({
        first_name: initialData.first_name ?? "",
        second_name: initialData.second_name ?? "",
        school_class: initialData.school_class ?? "",
        name_lg: initialData.name_lg ?? "",
        contact_lg: initialData.contact_lg ?? "",
        group_id: initialData.group_id ?? "",
        bus: initialData.bus ?? false,
        in_house: initialData.in_house ?? false,
        wc: initialData.wc ?? false,
        school_yard: initialData.school_yard ?? false,
      });
    }
  }, [initialData]);

  const handleChange = (
    e: React.ChangeEvent<
      HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement
    >,
  ) => {
    const { name, value, type } = e.target as HTMLInputElement;

    if (type === "checkbox") {
      const { checked } = e.target as HTMLInputElement;
      setFormData((prev) => ({
        ...prev,
        [name]: checked,
      }));
    } else {
      setFormData((prev) => ({
        ...prev,
        [name]: value,
      }));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // Validate required fields according to backend API
    if (!formData.first_name?.trim()) {
      setError("Vorname ist erforderlich.");
      return;
    }
    
    if (!formData.second_name?.trim()) {
      setError("Nachname ist erforderlich.");
      return;
    }
    
    if (!formData.school_class?.trim()) {
      setError("Klasse ist erforderlich.");
      return;
    }
    
    if (!formData.name_lg?.trim()) {
      setError("Name des Erziehungsberechtigten ist erforderlich.");
      return;
    }
    
    if (!formData.contact_lg?.trim()) {
      setError("Kontakt des Erziehungsberechtigten ist erforderlich.");
      return;
    }

    try {
      setError(null);

      // Call the provided submit function with form data
      await onSubmitAction(formData);
    } catch (err) {
      console.error("Error submitting form:", err);
      setError(
        "Fehler beim Speichern der Schülerdaten. Bitte versuchen Sie es später erneut.",
      );
    }
  };

  return (
    <div className="overflow-hidden rounded-lg bg-white shadow-md">
      <div className="p-6">
        <h2 className="mb-6 text-xl font-bold text-gray-800">{formTitle}</h2>

        {error && (
          <div className="mb-6 rounded-lg bg-red-50 p-4 text-red-800">
            <p>{error}</p>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-6">
          <div className="mb-8 rounded-lg bg-blue-50 p-4">
            <h2 className="mb-4 text-lg font-medium text-blue-800">
              Persönliche Daten
            </h2>
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              {/* First Name field */}
              <div>
                <label
                  htmlFor="first_name"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Vorname*
                </label>
                <input
                  type="text"
                  id="first_name"
                  name="first_name"
                  value={formData.first_name}
                  onChange={handleChange}
                  required
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none"
                />
              </div>

              {/* Last Name field */}
              <div>
                <label
                  htmlFor="second_name"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Nachname*
                </label>
                <input
                  type="text"
                  id="second_name"
                  name="second_name"
                  value={formData.second_name}
                  onChange={handleChange}
                  required
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none"
                />
              </div>

              {/* School Class field */}
              <div>
                <label
                  htmlFor="school_class"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Klasse*
                </label>
                <input
                  type="text"
                  id="school_class"
                  name="school_class"
                  value={formData.school_class}
                  onChange={handleChange}
                  required
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none"
                />
              </div>

              {/* Group Selector */}
              <div>
                <GroupSelector
                  value={formData.group_id}
                  onChange={(groupId) => {
                    setFormData((prev) => ({
                      ...prev,
                      group_id: groupId,
                    }));
                  }}
                  label="OGS Gruppe"
                  required={true}
                  includeEmpty={true}
                  emptyLabel="Bitte wählen Sie eine Gruppe"
                />
              </div>
            </div>
          </div>

          <div className="mb-8 rounded-lg bg-purple-50 p-4">
            <h2 className="mb-4 text-lg font-medium text-purple-800">
              Erziehungsberechtigte
            </h2>
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              {/* Legal Guardian Name field */}
              <div>
                <label
                  htmlFor="name_lg"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Name des Erziehungsberechtigten*
                </label>
                <input
                  type="text"
                  id="name_lg"
                  name="name_lg"
                  value={formData.name_lg}
                  onChange={handleChange}
                  required
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-purple-500 focus:outline-none"
                />
              </div>

              {/* Legal Guardian Contact field */}
              <div>
                <label
                  htmlFor="contact_lg"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Kontakt des Erziehungsberechtigten*
                </label>
                <input
                  type="text"
                  id="contact_lg"
                  name="contact_lg"
                  value={formData.contact_lg}
                  onChange={handleChange}
                  required
                  placeholder="E-Mail oder Telefonnummer"
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-purple-500 focus:outline-none"
                />
                <p className="mt-1 text-xs text-gray-500">
                  Bitte E-Mail-Adresse oder Telefonnummer eingeben
                </p>
              </div>
            </div>
          </div>

          <div className="mb-8 rounded-lg bg-green-50 p-4">
            <h2 className="mb-4 text-lg font-medium text-green-800">Busfahrer</h2>
            <div className="flex items-center">
              <input
                type="checkbox"
                id="bus"
                name="bus"
                checked={formData.bus}
                onChange={handleChange}
                className="h-4 w-4 rounded border-gray-300 text-green-600 focus:ring-green-500"
              />
              <label
                htmlFor="bus"
                className="ml-2 block text-sm text-gray-700"
              >
                Fährt mit dem Bus
              </label>
              <p className="ml-2 text-xs text-gray-500">
                (Aktivieren, wenn der Schüler mit dem Bus fährt)
              </p>
            </div>
          </div>

          {/* Form actions */}
          <div className="flex justify-end pt-4">
            <button
              type="button"
              onClick={onCancelAction}
              className="mr-2 rounded-lg px-4 py-2 text-gray-700 shadow-sm transition-colors hover:bg-gray-100"
              disabled={isLoading}
            >
              Abbrechen
            </button>
            <button
              type="submit"
              className="rounded-lg bg-gradient-to-r from-teal-500 to-blue-600 px-6 py-2 text-white transition-all duration-200 hover:from-teal-600 hover:to-blue-700 hover:shadow-lg"
              disabled={isLoading}
            >
              {isLoading ? "Wird gespeichert..." : submitLabel}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
