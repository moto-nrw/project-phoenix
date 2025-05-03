"use client";

import { useState, useEffect } from "react";
import type { Room } from "@/lib/api";

interface RoomFormProps {
  initialData?: Partial<Room>;
  onSubmitAction: (roomData: Partial<Room>) => Promise<void>;
  onCancelAction: () => void;
  isLoading: boolean;
  formTitle: string;
  submitLabel: string;
}

export function RoomForm({
  initialData,
  onSubmitAction,
  onCancelAction,
  isLoading,
  formTitle,
  submitLabel,
}: RoomFormProps) {
  const [formData, setFormData] = useState({
    name: "",
    building: "",
    floor: 0,
    capacity: 0,
    category: "Klassenzimmer",
    color: "#4F46E5", // Default indigo color
    deviceId: "",
  });

  const [error, setError] = useState<string | null>(null);

  // Common room categories
  const categories = [
    "Klassenzimmer",
    "Büro",
    "Sporthalle",
    "Mensa",
    "Musikraum",
    "Kunstraum",
    "Bibliothek",
    "Labor",
    "Aula",
    "Konferenzraum",
    "Pausenraum",
    "Sonstige",
  ];

  // Common room colors
  const colorOptions = [
    { value: "#EF4444", name: "Rot" },
    { value: "#F97316", name: "Orange" },
    { value: "#FACC15", name: "Gelb" },
    { value: "#10B981", name: "Grün" },
    { value: "#06B6D4", name: "Cyan" },
    { value: "#3B82F6", name: "Blau" },
    { value: "#4F46E5", name: "Indigo" },
    { value: "#8B5CF6", name: "Violett" },
    { value: "#EC4899", name: "Pink" },
    { value: "#6B7280", name: "Grau" },
  ];

  useEffect(() => {
    if (initialData) {
      setFormData({
        name: initialData.name ?? "",
        building: initialData.building ?? "",
        floor: initialData.floor ?? 0,
        capacity: initialData.capacity ?? 0,
        category: initialData.category ?? "Klassenzimmer",
        color: initialData.color ?? "#4F46E5",
        deviceId: initialData.deviceId ?? "",
      });
    }
  }, [initialData]);

  const handleChange = (
    e: React.ChangeEvent<
      HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement
    >,
  ) => {
    const { name, value, type } = e.target;
    
    // Handle numeric inputs
    if (type === "number") {
      setFormData((prev) => ({
        ...prev,
        [name]: value === "" ? 0 : parseInt(value, 10),
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
      setError("Bitte geben Sie einen Raumnamen ein.");
      return;
    }

    if (formData.capacity <= 0) {
      setError("Die Kapazität muss größer als 0 sein.");
      return;
    }

    try {
      setError(null);

      // Call the provided submit function with form data
      await onSubmitAction(formData);
    } catch (err) {
      console.error("Error submitting form:", err);
      setError(
        "Fehler beim Speichern der Raumdaten. Bitte versuchen Sie es später erneut."
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
              Rauminformationen
            </h2>
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              {/* Room Name field */}
              <div>
                <label
                  htmlFor="name"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Raumname*
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

              {/* Category field */}
              <div>
                <label
                  htmlFor="category"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Kategorie*
                </label>
                <select
                  id="category"
                  name="category"
                  value={formData.category}
                  onChange={handleChange}
                  required
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none"
                >
                  {categories.map((category) => (
                    <option key={category} value={category}>
                      {category}
                    </option>
                  ))}
                </select>
              </div>

              {/* Building field */}
              <div>
                <label
                  htmlFor="building"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Gebäude
                </label>
                <input
                  type="text"
                  id="building"
                  name="building"
                  value={formData.building}
                  onChange={handleChange}
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none"
                />
              </div>

              {/* Floor field */}
              <div>
                <label
                  htmlFor="floor"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Etage*
                </label>
                <input
                  type="number"
                  id="floor"
                  name="floor"
                  value={formData.floor}
                  onChange={handleChange}
                  required
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none"
                />
              </div>

              {/* Capacity field */}
              <div>
                <label
                  htmlFor="capacity"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Kapazität*
                </label>
                <input
                  type="number"
                  id="capacity"
                  name="capacity"
                  value={formData.capacity}
                  onChange={handleChange}
                  required
                  min="1"
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none"
                />
              </div>

              {/* Device ID field */}
              <div>
                <label
                  htmlFor="deviceId"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Geräte-ID
                </label>
                <input
                  type="text"
                  id="deviceId"
                  name="deviceId"
                  value={formData.deviceId}
                  onChange={handleChange}
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none"
                />
                <p className="mt-1 text-xs text-gray-500">
                  ID des Tablets oder Geräts für die Raumverwaltung
                </p>
              </div>
            </div>
          </div>

          {/* Color Selection */}
          <div className="mb-8 rounded-lg bg-gray-50 p-4">
            <h2 className="mb-4 text-lg font-medium text-gray-800">
              Farbauswahl
            </h2>
            <div>
              <label
                htmlFor="color"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Raumfarbe
              </label>
              <div className="grid grid-cols-5 gap-2">
                {colorOptions.map((color) => (
                  <div
                    key={color.value}
                    className={`flex flex-col items-center rounded-lg p-2 transition-all ${
                      formData.color === color.value
                        ? "bg-gray-200 shadow-sm"
                        : "bg-white hover:bg-gray-100"
                    }`}
                    onClick={() =>
                      setFormData((prev) => ({ ...prev, color: color.value }))
                    }
                  >
                    <div
                      className="h-8 w-8 rounded-full"
                      style={{ backgroundColor: color.value }}
                    ></div>
                    <span className="mt-1 text-xs">{color.name}</span>
                  </div>
                ))}
              </div>
              <div className="mt-4 flex">
                <input
                  type="color"
                  id="color"
                  name="color"
                  value={formData.color}
                  onChange={handleChange}
                  className="h-10 w-12"
                />
                <input
                  type="text"
                  value={formData.color}
                  onChange={(e) =>
                    setFormData((prev) => ({ ...prev, color: e.target.value }))
                  }
                  className="ml-2 w-full rounded-lg border border-gray-300 px-4 py-2 text-sm transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none"
                />
              </div>
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