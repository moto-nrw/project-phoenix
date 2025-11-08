"use client";

import { Modal } from "~/components/ui/modal";
import type { Room } from "@/lib/room-helpers";
import { useState } from "react";

interface RoomCreateModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreate: (data: Partial<Room>) => Promise<void>;
  loading?: boolean;
}

export function RoomCreateModal({
  isOpen,
  onClose,
  onCreate,
  loading = false,
}: RoomCreateModalProps) {
  const [form, setForm] = useState<Partial<Room>>({
    name: "",
    category: "Klassenzimmer",
    building: "",
    floor: 0,
    capacity: 30,
    color: "#4F46E5",
    isOccupied: false,
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  const handleChange = (field: keyof Room, value: string | number) => {
    setForm((prev) => ({ ...prev, [field]: value }));
    if (errors[field as string]) {
      setErrors((prev) => {
        const n = { ...prev };
        delete n[field as string];
        return n;
      });
    }
  };

  const validate = () => {
    const e: Record<string, string> = {};
    if (!form.name || !String(form.name).trim())
      e.name = "Raumname ist erforderlich";
    if (form.floor === undefined || isNaN(Number(form.floor)))
      e.floor = "Etage ist erforderlich";
    if (form.capacity === undefined || Number(form.capacity) <= 0)
      e.capacity = "Kapazität muss > 0 sein";
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    try {
      setSaving(true);
      await onCreate({
        name: String(form.name).trim(),
        category: form.category ?? "Klassenzimmer",
        building: form.building ?? "",
        floor: Number(form.floor ?? 0),
        capacity: Number(form.capacity ?? 0),
        color: form.color ?? "#4F46E5",
      });
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Neuer Raum">
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-indigo-500" />
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : (
        <form onSubmit={handleSubmit} className="space-y-4 md:space-y-6">
          {/* Room details */}
          <div className="rounded-xl border border-gray-100 bg-indigo-50/30 p-3 md:p-4">
            <h4 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
              <svg
                className="h-3.5 w-3.5 text-indigo-600 md:h-4 md:w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
                />
              </svg>
              Raumdaten
            </h4>
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-700">
                  Raumname <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  value={form.name ?? ""}
                  onChange={(e) => handleChange("name", e.target.value)}
                  className={`w-full rounded-lg border ${errors.name ? "border-red-300 bg-red-50" : "border-gray-200 bg-white focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"} px-3 py-2 text-sm transition-colors`}
                />
                {errors.name && (
                  <p className="mt-1 text-xs text-red-600">{errors.name}</p>
                )}
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-700">
                  Kategorie
                </label>
                <div className="relative">
                  <select
                    value={form.category ?? "Klassenzimmer"}
                    onChange={(e) => handleChange("category", e.target.value)}
                    className="w-full appearance-none rounded-lg border border-gray-200 bg-white py-2 pr-10 pl-3 text-sm transition-colors focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                  >
                    {[
                      "Klassenzimmer",
                      "Labor",
                      "Sport",
                      "Kunst",
                      "Musik",
                      "Computer",
                      "Bibliothek",
                      "Lernraum",
                      "Speiseraum",
                      "Versammlung",
                      "Medizin",
                      "Büro",
                      "Besprechung",
                    ].map((c) => (
                      <option key={c} value={c}>
                        {c}
                      </option>
                    ))}
                  </select>
                  <svg
                    className="pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 text-gray-400"
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
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-700">
                  Gebäude
                </label>
                <input
                  type="text"
                  value={form.building ?? ""}
                  onChange={(e) => handleChange("building", e.target.value)}
                  className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-700">
                  Etage <span className="text-red-500">*</span>
                </label>
                <input
                  type="number"
                  value={Number(form.floor ?? 0)}
                  onChange={(e) =>
                    handleChange("floor", Number(e.target.value))
                  }
                  className={`w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 ${errors.floor ? "border-red-300 bg-red-50" : ""}`}
                />
                {errors.floor && (
                  <p className="mt-1 text-xs text-red-600">{errors.floor}</p>
                )}
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-700">
                  Kapazität <span className="text-red-500">*</span>
                </label>
                <input
                  type="number"
                  min={1}
                  value={Number(form.capacity ?? 30)}
                  onChange={(e) =>
                    handleChange("capacity", Number(e.target.value))
                  }
                  className={`w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 ${errors.capacity ? "border-red-300 bg-red-50" : ""}`}
                />
                {errors.capacity && (
                  <p className="mt-1 text-xs text-red-600">{errors.capacity}</p>
                )}
              </div>
            </div>
          </div>

          {/* Sticky actions */}
          <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex gap-2 border-t border-gray-100 bg-white/95 px-4 pt-3 pb-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:pt-4 md:pb-4">
            <button
              type="button"
              onClick={onClose}
              disabled={saving || loading}
              className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-xs font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
            >
              Abbrechen
            </button>
            <button
              type="submit"
              disabled={saving || loading}
              className="flex-1 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
            >
              {saving || loading ? "Wird gespeichert..." : "Erstellen"}
            </button>
          </div>
        </form>
      )}
    </Modal>
  );
}
