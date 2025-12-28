"use client";

import { useState, useEffect } from "react";
import type { CombinedGroup } from "@/lib/api";

interface CombinedGroupFormProps {
  initialData?: Partial<CombinedGroup>;
  onSubmitAction: (groupData: Partial<CombinedGroup>) => Promise<void>;
  onCancelAction: () => void;
  isLoading: boolean;
  formTitle: string;
  submitLabel: string;
}

export default function CombinedGroupForm({
  initialData,
  onSubmitAction,
  onCancelAction,
  isLoading,
  formTitle,
  submitLabel,
}: CombinedGroupFormProps) {
  const [formData, setFormData] = useState({
    name: "",
    is_active: true,
    access_policy: "manual" as "all" | "first" | "specific" | "manual",
    valid_until: "",
    specific_group_id: "",
  });

  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (initialData) {
      setFormData({
        name: initialData.name ?? "",
        is_active: initialData.is_active ?? true,
        access_policy: initialData.access_policy ?? "manual",
        valid_until: initialData.valid_until?.split("T")[0] ?? "", // Format as YYYY-MM-DD for input type="date"
        specific_group_id: initialData.specific_group_id ?? "",
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

    // Validate form
    if (!formData.name) {
      setError("Bitte geben Sie einen Namen für die Gruppenkombination ein.");
      return;
    }

    // Validate specific group is provided if access policy is 'specific'
    if (formData.access_policy === "specific" && !formData.specific_group_id) {
      setError(
        'Bitte wählen Sie eine spezifische Gruppe aus, wenn die Zugriffsmethode "Spezifische Gruppe" ist.',
      );
      return;
    }

    try {
      setError(null);

      // Call the provided submit function with form data
      await onSubmitAction(formData);
    } catch (err) {
      console.error("Error submitting form:", err);
      setError(
        "Fehler beim Speichern der Gruppenkombination. Bitte versuchen Sie es später erneut.",
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

        <form onSubmit={handleSubmit} noValidate className="space-y-6">
          <div className="mb-8 rounded-lg bg-blue-50 p-4">
            <h2 className="mb-4 text-lg font-medium text-blue-800">
              Grunddaten
            </h2>
            <div className="grid grid-cols-1 gap-4">
              {/* Group Name field */}
              <div>
                <label
                  htmlFor="name"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Name der Kombination*
                </label>
                <input
                  type="text"
                  id="name"
                  name="name"
                  value={formData.name}
                  onChange={handleChange}
                  required
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none"
                />
              </div>

              {/* Is Active field */}
              <div className="flex items-center">
                <input
                  type="checkbox"
                  id="is_active"
                  name="is_active"
                  checked={formData.is_active}
                  onChange={handleChange}
                  className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                />
                <label
                  htmlFor="is_active"
                  className="ml-2 block text-sm text-gray-700"
                >
                  Aktiv
                </label>
              </div>

              {/* Valid Until field */}
              <div>
                <label
                  htmlFor="valid_until"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Gültig bis
                </label>
                <input
                  type="date"
                  id="valid_until"
                  name="valid_until"
                  value={formData.valid_until}
                  onChange={handleChange}
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none"
                />
                <p className="mt-1 text-xs text-gray-500">
                  Lassen Sie dieses Feld leer, wenn die Kombination kein
                  Ablaufdatum haben soll
                </p>
              </div>
            </div>
          </div>

          <div className="mb-8 rounded-lg bg-purple-50 p-4">
            <h2 className="mb-4 text-lg font-medium text-purple-800">
              Zugriffseinstellungen
            </h2>
            <div className="grid grid-cols-1 gap-4">
              {/* Access Policy field */}
              <div>
                <label
                  htmlFor="access_policy"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Zugriffsmethode*
                </label>
                <select
                  id="access_policy"
                  name="access_policy"
                  value={formData.access_policy}
                  onChange={handleChange}
                  required
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-purple-500 focus:outline-none"
                >
                  <option value="all">Alle Gruppen</option>
                  <option value="first">Erste Gruppe</option>
                  <option value="specific">Spezifische Gruppe</option>
                  <option value="manual">Manuell</option>
                </select>
                <p className="mt-1 text-xs text-gray-500">
                  Bestimmt, wie Zugriffsberechtigungen für die kombinierten
                  Gruppen verwaltet werden
                </p>
              </div>

              {/* Specific Group ID field - only shown when access_policy is 'specific' */}
              {formData.access_policy === "specific" && (
                <div>
                  <label
                    htmlFor="specific_group_id"
                    className="mb-1 block text-sm font-medium text-gray-700"
                  >
                    Spezifische Gruppe*
                  </label>
                  <input
                    type="text"
                    id="specific_group_id"
                    name="specific_group_id"
                    value={formData.specific_group_id}
                    onChange={handleChange}
                    required
                    className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-purple-500 focus:outline-none"
                  />
                  <p className="mt-1 text-xs text-gray-500">
                    ID der Gruppe, deren Zugriffsberechtigungen verwendet werden
                    sollen
                  </p>
                </div>
              )}
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
