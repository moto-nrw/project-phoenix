"use client";

import { useState, useEffect } from "react";
import type { Group, Room } from "@/lib/api";
import { roomService } from "@/lib/api";
import { teacherService } from "@/lib/teacher-api";
import type { Teacher } from "@/lib/teacher-api";

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
    representative_id: "",
    teacher_ids: [] as string[],
  });

  const [error, setError] = useState<string | null>(null);
  const [rooms, setRooms] = useState<Room[]>([]);
  const [teachers, setTeachers] = useState<Teacher[]>([]);
  const [loadingData, setLoadingData] = useState(true);

  useEffect(() => {
    if (initialData) {
      setFormData({
        name: initialData.name ?? "",
        room_id: initialData.room_id ?? "",
        representative_id: initialData.representative_id ?? "",
        teacher_ids: initialData.supervisors?.map(s => s.id) ?? [],
      });
    }
  }, [initialData]);

  // Fetch rooms and teachers on component mount
  useEffect(() => {
    const fetchData = async () => {
      try {
        setLoadingData(true);
        
        // Fetch rooms and teachers in parallel
        const [roomsData, teachersData] = await Promise.all([
          roomService.getRooms(),
          teacherService.getTeachers()
        ]);
        
        setRooms(roomsData);
        setTeachers(teachersData);
        setError(null);
      } catch (err) {
        console.error("Error fetching data:", err);
        setError("Fehler beim Laden der Daten. Bitte versuchen Sie es später erneut.");
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

  const handleTeacherToggle = (teacherId: string) => {
    setFormData((prev) => ({
      ...prev,
      teacher_ids: prev.teacher_ids.includes(teacherId)
        ? prev.teacher_ids.filter(id => id !== teacherId)
        : [...prev.teacher_ids, teacherId],
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
        representative_id: formData.representative_id || undefined,
        teacher_ids: formData.teacher_ids.length > 0 ? formData.teacher_ids : undefined,
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

              {/* Representative ID field */}
              <div>
                <label
                  htmlFor="representative_id"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Vertreter
                </label>
                <select
                  id="representative_id"
                  name="representative_id"
                  value={formData.representative_id}
                  onChange={handleChange}
                  disabled={loadingData}
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none disabled:bg-gray-100"
                >
                  <option value="">Lehrer auswählen</option>
                  {teachers.map((teacher) => (
                    <option key={teacher.id} value={teacher.id}>
                      {teacher.name}
                      {teacher.specialization && ` (${teacher.specialization})`}
                    </option>
                  ))}
                </select>
                <p className="mt-1 text-xs text-gray-500">
                  Legt den Hauptverantwortlichen für diese Gruppe fest
                </p>
              </div>

              {/* Teacher Multi-Select field */}
              <div>
                <label className="mb-1 block text-sm font-medium text-gray-700">
                  Lehrer/Aufsichtspersonen
                </label>
                <div className="space-y-2 max-h-48 overflow-y-auto border border-gray-300 rounded-lg p-3">
                  {loadingData ? (
                    <p className="text-sm text-gray-500">Lehrer werden geladen...</p>
                  ) : teachers.length === 0 ? (
                    <p className="text-sm text-gray-500">Keine Lehrer verfügbar</p>
                  ) : (
                    teachers.map((teacher) => (
                      <label
                        key={teacher.id}
                        className="flex items-center space-x-2 cursor-pointer hover:bg-gray-50 p-2 rounded"
                      >
                        <input
                          type="checkbox"
                          checked={formData.teacher_ids.includes(teacher.id)}
                          onChange={() => handleTeacherToggle(teacher.id)}
                          className="w-4 h-4 text-blue-600 rounded focus:ring-blue-500"
                        />
                        <span className="text-sm">
                          {teacher.name}
                          {teacher.specialization && ` (${teacher.specialization})`}
                        </span>
                      </label>
                    ))
                  )}
                </div>
                <p className="mt-1 text-xs text-gray-500">
                  Wählen Sie die Lehrer aus, die dieser Gruppe zugeordnet werden sollen
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
