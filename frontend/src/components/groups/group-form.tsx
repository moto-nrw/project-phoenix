"use client";

import { useState, useEffect } from "react";
import type { Group, Room } from "@/lib/api";
import { roomService } from "@/lib/api";
import { SupervisorMultiSelect } from "./supervisor-multi-select";

interface GroupFormProps {
  initialData?: Partial<Group>;
  onSubmitAction: (groupData: Partial<Group>) => Promise<void>;
  onCancelAction: () => void;
  isLoading: boolean;
  formTitle: string;
  submitLabel: string;
}

export default function GroupForm({
  initialData,
  onSubmitAction,
  onCancelAction,
  isLoading,
  formTitle,
  submitLabel,
}: GroupFormProps) {
  const [formData, setFormData] = useState({
    name: "",
    room_id: "",
    teacher_ids: [] as string[],
  });

  const [error, setError] = useState<string | null>(null);
  const [rooms, setRooms] = useState<Room[]>([]);
  const [loadingData, setLoadingData] = useState(true);

  useEffect(() => {
    if (initialData) {
      setFormData({
        name: initialData.name ?? "",
        room_id: initialData.room_id ?? "",
        teacher_ids: initialData.supervisors?.map((s) => s.id) ?? [],
      });
    }
  }, [initialData]);

  // Fetch rooms on component mount
  useEffect(() => {
    const fetchData = async () => {
      try {
        setLoadingData(true);

        // Fetch rooms
        const roomsData = await roomService.getRooms();

        setRooms(roomsData);
        setError(null);
      } catch (err) {
        console.error("Error fetching data:", err);
        setError(
          "Fehler beim Laden der Daten. Bitte versuchen Sie es später erneut.",
        );
      } finally {
        setLoadingData(false);
      }
    };

    void fetchData();
  }, []);

  const handleChange = (
    e: React.ChangeEvent<
      HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement
    >,
  ) => {
    const { name, value } = e.target;
    setFormData((prev) => ({
      ...prev,
      [name]: value,
    }));
  };

  const handleSupervisorChange = (teacherIds: string[]) => {
    setFormData((prev) => ({
      ...prev,
      teacher_ids: teacherIds,
    }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // Validate form
    if (!formData.name) {
      setError("Bitte geben Sie einen Gruppennamen ein.");
      return;
    }

    try {
      setError(null);

      // Prepare data - ensure we only include non-empty values
      const submitData: Partial<Group> = {
        name: formData.name,
        room_id: formData.room_id || undefined,
        teacher_ids:
          formData.teacher_ids.length > 0 ? formData.teacher_ids : undefined,
      };

      // Call the provided submit function with form data
      await onSubmitAction(submitData);
    } catch (err) {
      console.error("Error submitting form:", err);
      setError(
        "Fehler beim Speichern der Gruppendaten. Bitte versuchen Sie es später erneut.",
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
              Gruppendaten
            </h2>
            <div className="grid grid-cols-1 gap-4">
              {/* Group Name field */}
              <div>
                <label
                  htmlFor="name"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Gruppenname*
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

              {/* Room ID field */}
              <div>
                <label
                  htmlFor="room_id"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Raum
                </label>
                <select
                  id="room_id"
                  name="room_id"
                  value={formData.room_id}
                  onChange={handleChange}
                  disabled={loadingData}
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none disabled:bg-gray-100"
                >
                  <option value="">Raum auswählen</option>
                  {rooms.map((room) => (
                    <option key={room.id} value={room.id}>
                      {room.name}
                      {room.building && ` - ${room.building}`}
                      {room.floor !== undefined && ` (Etage ${room.floor})`}
                    </option>
                  ))}
                </select>
                <p className="mt-1 text-xs text-gray-500">
                  Verbindet diese Gruppe mit einem Raum
                </p>
              </div>

              {/* Supervisors Multi-Select field */}
              <div>
                <label className="mb-1 block text-sm font-medium text-gray-700">
                  Aufsichtspersonen *
                </label>
                <SupervisorMultiSelect
                  selectedSupervisors={formData.teacher_ids}
                  onSelectionChange={handleSupervisorChange}
                  placeholder="Aufsichtspersonen auswählen..."
                />
                <p className="mt-1 text-xs text-gray-500">
                  Wählen Sie eine oder mehrere Aufsichtspersonen für diese
                  Gruppe aus
                </p>
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
