"use client";

import { useState, useEffect } from "react";
import type {
  Activity,
  ActivityCategory,
  ActivitySchedule,
  CreateActivityRequest,
} from "~/lib/activity-helpers";

// Helper component for selecting a supervisor
const SupervisorSelector = ({
  value,
  onChange,
  label,
  supervisors = [],
}: {
  value: string;
  onChange: (value: string) => void;
  label: string;
  supervisors?: Array<{ id: string; name: string }>;
}) => {
  return (
    <div>
      <label className="mb-1 block text-sm font-medium text-gray-700">
        {label}
      </label>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-purple-500 focus:outline-none"
      >
        <option value="">Supervisor auswählen</option>
        {supervisors.map((supervisor) => (
          <option key={supervisor.id} value={supervisor.id}>
            {supervisor.name}
          </option>
        ))}
      </select>
    </div>
  );
};

// Helper component for selecting a category
const CategorySelector = ({
  value,
  onChange,
  label,
  categories = [],
}: {
  value: string;
  onChange: (value: string) => void;
  label: string;
  categories?: ActivityCategory[];
}) => {
  return (
    <div>
      <label className="mb-1 block text-sm font-medium text-gray-700">
        {label}
      </label>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-purple-500 focus:outline-none"
      >
        <option value="">Kategorie auswählen</option>
        {categories.map((category) => (
          <option key={category.id} value={category.id}>
            {category.name}
          </option>
        ))}
      </select>
    </div>
  );
};

// Helper component for room selection
const RoomSelector = ({
  value,
  onChange,
  label,
  rooms = [],
}: {
  value: string;
  onChange: (value: string) => void;
  label: string;
  rooms?: Array<{ id: string; name: string }>;
}) => {
  return (
    <div>
      <label className="mb-1 block text-sm font-medium text-gray-700">
        {label}
      </label>
      <select
        value={value || ""}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-purple-500 focus:outline-none"
      >
        <option value="">Keinen Raum zuweisen</option>
        {rooms.map((room) => (
          <option key={room.id} value={room.id}>
            {room.name}
          </option>
        ))}
      </select>
    </div>
  );
};

// Helper component for time slots
const TimeSlotEditor = ({
  timeSlots,
  onAdd,
  onRemove,
  parentActivityId,
}: {
  timeSlots: ActivitySchedule[];
  onAdd: (timeSlot: Omit<ActivitySchedule, "id" | "created_at" | "updated_at">) => void;
  onRemove: (index: number) => void;
  parentActivityId?: string;
}) => {
  const [weekday, setWeekday] = useState("monday");
  const [startTime, setStartTime] = useState("");
  const [endTime, setEndTime] = useState("");
  const [isCreatingTimespan, setIsCreatingTimespan] = useState(false);
  // Use parent activity ID if provided, or default to a temporary one
  const activityId = parentActivityId ?? "temp";

  const weekdays = [
    { value: "monday", label: "Montag" },
    { value: "tuesday", label: "Dienstag" },
    { value: "wednesday", label: "Mittwoch" },
    { value: "thursday", label: "Donnerstag" },
    { value: "friday", label: "Freitag" },
    { value: "saturday", label: "Samstag" },
    { value: "sunday", label: "Sonntag" },
  ];

  const handleAddTimeSlot = () => {
    if (!weekday || !startTime || !endTime) {
      alert("Bitte geben Sie Wochentag, Startzeit und Endzeit an.");
      return;
    }

    try {
      setIsCreatingTimespan(true);

      // Add the time slot
      onAdd({
        weekday,
        timeframe_id: undefined,
        activity_id: activityId || "new",
      });

      // Reset form
      setStartTime("");
      setEndTime("");
    } catch (error) {
      console.error("Error adding time slot:", error);
      alert(
        "Fehler beim Hinzufügen des Zeitslots. Bitte versuchen Sie es später erneut.",
      );
    } finally {
      setIsCreatingTimespan(false);
    }
  };

  const formatWeekday = (day: string): string => {
    const weekdayMap: Record<string, string> = {
      monday: "Montag",
      tuesday: "Dienstag",
      wednesday: "Mittwoch",
      thursday: "Donnerstag",
      friday: "Freitag",
      saturday: "Samstag",
      sunday: "Sonntag",
    };
    return weekdayMap[day.toLowerCase()] || day;
  };

  return (
    <div className="space-y-4">
      <h3 className="text-md font-medium text-gray-700">Zeitslots</h3>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700">
            Wochentag
          </label>
          <select
            value={weekday}
            onChange={(e) => setWeekday(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-purple-500 focus:outline-none"
          >
            {weekdays.map((day) => (
              <option key={day.value} value={day.value}>
                {day.label}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700">
            Startzeit
          </label>
          <input
            type="time"
            value={startTime}
            onChange={(e) => setStartTime(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-purple-500 focus:outline-none"
          />
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700">
            Endzeit
          </label>
          <input
            type="time"
            value={endTime}
            onChange={(e) => setEndTime(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-purple-500 focus:outline-none"
          />
        </div>
      </div>

      <div className="flex justify-end">
        <button
          type="button"
          onClick={handleAddTimeSlot}
          disabled={isCreatingTimespan}
          className="rounded-lg bg-purple-600 px-4 py-2 text-white transition-colors hover:bg-purple-700 disabled:cursor-not-allowed disabled:bg-gray-400"
        >
          {isCreatingTimespan ? "Wird hinzugefügt..." : "Zeitslot hinzufügen"}
        </button>
      </div>

      {timeSlots.length > 0 && (
        <div className="mt-4">
          <h4 className="mb-2 text-sm font-medium text-gray-700">
            Vorhandene Zeitslots:
          </h4>
          <ul className="space-y-2">
            {timeSlots.map((slot, index) => (
              <li
                key={slot.id || index}
                className="flex items-center justify-between rounded border border-gray-200 bg-gray-50 p-2"
              >
                <span>
                  {formatWeekday(slot.weekday)}
                </span>
                <button
                  type="button"
                  onClick={() => onRemove(index)}
                  className="text-red-500 hover:text-red-700"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    className="h-5 w-5"
                    viewBox="0 0 20 20"
                    fill="currentColor"
                  >
                    <path
                      fillRule="evenodd"
                      d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z"
                      clipRule="evenodd"
                    />
                  </svg>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
};

interface ActivityFormProps {
  initialData?: Partial<Activity>;
  onSubmitAction: (activityData: Partial<Activity>) => Promise<void>;
  onCancelAction: () => void;
  isLoading: boolean;
  formTitle: string;
  submitLabel: string;
  categories?: ActivityCategory[];
  supervisors?: Array<{ id: string; name: string }>;
  rooms?: Array<{ id: string; name: string }>;
}

export default function ActivityForm({
  initialData,
  onSubmitAction,
  onCancelAction,
  isLoading,
  formTitle,
  submitLabel,
  categories = [],
  supervisors = [],
  rooms = [],
}: ActivityFormProps) {
  const [formData, setFormData] = useState<Partial<Activity>>({
    name: "",
    max_participant: 0,
    is_open_ags: false,
    supervisor_id: "",
    ag_category_id: "",
    planned_room_id: "",
  });

  const [timeSlots, setTimeSlots] = useState<ActivitySchedule[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (initialData) {
      setFormData({
        id: initialData.id,
        name: initialData.name ?? "",
        max_participant: initialData.max_participant ?? 0,
        is_open_ags: initialData.is_open_ags ?? false,
        supervisor_id: initialData.supervisor_id ?? "",
        ag_category_id: initialData.ag_category_id ?? "",
        planned_room_id: initialData.planned_room_id ?? "",
      });

      if (initialData.times) {
        setTimeSlots(initialData.times);
      }
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
    } else if (type === "number") {
      setFormData((prev) => ({
        ...prev,
        [name]: parseInt(value) || 0,
      }));
    } else {
      setFormData((prev) => ({
        ...prev,
        [name]: value,
      }));
    }
  };

  const prepareDataForSubmission = (): Partial<Activity> & { schedules?: any[] } => {
    // Convert form data to what the backend API expects
    const submissionData = {
      ...formData,
      // Add schedules for backend
      schedules: timeSlots.map(slot => ({
        weekday: slot.weekday.toUpperCase(),
        timeframe_id: slot.timeframe_id ? parseInt(slot.timeframe_id, 10) : null
      }))
    };

    return submissionData;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // Validate form
    if (
      !formData.name ||
      !formData.max_participant ||
      !formData.ag_category_id
    ) {
      setError("Bitte füllen Sie alle Pflichtfelder aus.");
      return;
    }

    try {
      setError(null);

      // Prepare data for submission
      const submissionData = prepareDataForSubmission();

      // Debug log to see what we're submitting
      console.log(
        "Submitting form data:",
        JSON.stringify(submissionData, null, 2),
      );

      // Call the provided submit function with form data
      await onSubmitAction(submissionData);
    } catch (err) {
      console.error("Error submitting form:", err);
      setError(
        "Fehler beim Speichern der Aktivitätsdaten. Bitte versuchen Sie es später erneut.",
      );
    }
  };

  const handleAddTimeSlot = (
    newTimeSlot: Omit<ActivitySchedule, "id" | "created_at" | "updated_at">,
  ) => {
    // Generate a temporary ID for UI purposes
    const tempTimeSlot: ActivitySchedule = {
      id: `temp-${Date.now()}`,
      created_at: new Date(),
      updated_at: new Date(),
      // Use spread to get properties from newTimeSlot, but avoid duplicates
      ...newTimeSlot,
    };

    setTimeSlots((prev) => [...prev, tempTimeSlot]);
  };

  const handleRemoveTimeSlot = (index: number) => {
    setTimeSlots((prev) => prev.filter((_, i) => i !== index));
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
          <div className="mb-8 rounded-lg bg-purple-50 p-4">
            <h2 className="mb-4 text-lg font-medium text-purple-800">
              Grundlegende Informationen
            </h2>
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              {/* Name field */}
              <div>
                <label
                  htmlFor="name"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Name*
                </label>
                <input
                  type="text"
                  id="name"
                  name="name"
                  value={formData.name}
                  onChange={handleChange}
                  required
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-purple-500 focus:outline-none"
                />
              </div>

              {/* Max Participants field */}
              <div>
                <label
                  htmlFor="max_participant"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Maximale Teilnehmer*
                </label>
                <input
                  type="number"
                  id="max_participant"
                  name="max_participant"
                  value={formData.max_participant}
                  onChange={handleChange}
                  min="1"
                  required
                  className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-purple-500 focus:outline-none"
                />
              </div>

              {/* Category selector */}
              <CategorySelector
                value={formData.ag_category_id ?? ""}
                onChange={(value) => {
                  setFormData((prev) => ({
                    ...prev,
                    ag_category_id: value,
                  }));
                }}
                label="Kategorie*"
                categories={categories}
              />

              {/* Supervisor selector */}
              <SupervisorSelector
                value={formData.supervisor_id ?? ""}
                onChange={(value) => {
                  setFormData((prev) => ({
                    ...prev,
                    supervisor_id: value,
                  }));
                }}
                label="Leitung"
                supervisors={supervisors}
              />

              {/* Room selector */}
              <RoomSelector
                value={formData.planned_room_id ?? ""}
                onChange={(value) => {
                  setFormData((prev) => ({
                    ...prev,
                    planned_room_id: value,
                  }));
                }}
                label="Geplanter Raum"
                rooms={rooms}
              />

              {/* Is Open checkbox */}
              <div className="mt-2 flex items-center">
                <input
                  type="checkbox"
                  id="is_open_ags"
                  name="is_open_ags"
                  checked={formData.is_open_ags}
                  onChange={handleChange}
                  className="h-4 w-4 rounded border-gray-300 text-purple-600 focus:ring-purple-500"
                />
                <label
                  htmlFor="is_open_ags"
                  className="ml-2 block text-sm text-gray-700"
                >
                  Offen für Anmeldungen
                </label>
              </div>
            </div>
          </div>

          <div className="mb-8 rounded-lg bg-blue-50 p-4">
            <TimeSlotEditor
              timeSlots={timeSlots}
              onAdd={handleAddTimeSlot}
              onRemove={handleRemoveTimeSlot}
              parentActivityId={formData.id}
            />
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
              className="rounded-lg bg-gradient-to-r from-purple-500 to-indigo-600 px-6 py-2 text-white transition-all duration-200 hover:from-purple-600 hover:to-indigo-700 hover:shadow-lg"
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