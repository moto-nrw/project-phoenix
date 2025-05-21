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
  onEdit,
  parentActivityId,
  timeframes = [],
  availableTimeSlots = [],
}: {
  timeSlots: ActivitySchedule[];
  onAdd: (timeSlot: Omit<ActivitySchedule, "id" | "created_at" | "updated_at">) => void;
  onRemove: (index: number) => void;
  onEdit?: (index: number, timeSlot: Partial<ActivitySchedule>) => void;
  parentActivityId?: string;
  timeframes?: Array<{ id: string; start_time: string; end_time: string; name?: string }>;
  availableTimeSlots?: Array<any>;
}) => {
  const [weekday, setWeekday] = useState("monday");
  const [timeframeId, setTimeframeId] = useState("");
  const [isCreatingTimespan, setIsCreatingTimespan] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  
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

  // Check if a time slot is already in use by existing time slots
  const isTimeSlotConflict = (weekday: string, timeframeId: string, excludeIndex?: number): boolean => {
    // Skip timeframes with no ID (all-day slots)
    if (!timeframeId) return false;
    
    return timeSlots.some((slot, index) => 
      // Skip the slot we're currently editing
      (excludeIndex === undefined || index !== excludeIndex) &&
      // Check if weekday and timeframe match
      slot.weekday.toLowerCase() === weekday.toLowerCase() && 
      slot.timeframe_id === timeframeId
    );
  };

  // Check if a time slot is already in use based on available slots from API
  const isTimeSlotTaken = (weekday: string, timeframeId: string): boolean => {
    // First check existing time slots for conflicts
    if (isTimeSlotConflict(weekday, timeframeId)) {
      return true;
    }
    
    // If we don't have a list of available time slots from API, only check conflicts
    if (!availableTimeSlots || availableTimeSlots.length === 0) return false;
    
    // Check if this time slot is available in the available slots
    const isAvailable = availableTimeSlots.some(slot => 
      slot.weekday.toLowerCase() === weekday.toLowerCase() && 
      slot.timeframe_id === (timeframeId || undefined)
    );
    
    // Return true if not available (taken)
    return !isAvailable;
  };

  const handleAddTimeSlot = () => {
    if (!weekday) {
      setError("Bitte geben Sie einen Wochentag an.");
      return;
    }

    // Check for conflicts with existing timeslots
    if (timeframeId && isTimeSlotConflict(weekday, timeframeId)) {
      setError(`Dieser Zeitslot ist bereits belegt (${formatWeekday(weekday)}, ${getTimeframeLabel(timeframeId)}).`);
      return;
    }
    
    try {
      setIsCreatingTimespan(true);
      setError(null);

      // Add the time slot
      onAdd({
        weekday,
        timeframe_id: timeframeId || undefined,
        activity_id: activityId || "new",
      });

      // Reset form
      setTimeframeId("");
    } catch (error) {
      console.error("Error adding time slot:", error);
      setError("Fehler beim Hinzufügen des Zeitslots. Bitte versuchen Sie es später erneut.");
    } finally {
      setIsCreatingTimespan(false);
    }
  };

  const handleEditTimeSlot = (index: number) => {
    if (!onEdit) return;
    
    const slot = timeSlots[index];
    if (!slot) return;
    
    // Set form values to the slot being edited
    setWeekday(slot.weekday);
    setTimeframeId(slot.timeframe_id || "");
    setEditingIndex(index);
  };

  const handleSaveEdit = () => {
    if (!onEdit || editingIndex === null) return;
    
    // Check for conflicts with existing timeslots (excluding the one being edited)
    if (timeframeId && isTimeSlotConflict(weekday, timeframeId, editingIndex)) {
      setError(`Dieser Zeitslot ist bereits belegt (${formatWeekday(weekday)}, ${getTimeframeLabel(timeframeId)}).`);
      return;
    }
    
    try {
      setError(null);
      
      onEdit(editingIndex, {
        weekday,
        timeframe_id: timeframeId || undefined,
      });
      
      // Reset edit mode
      setEditingIndex(null);
      setTimeframeId("");
    } catch (error) {
      console.error("Error updating time slot:", error);
      setError("Fehler beim Aktualisieren des Zeitslots. Bitte versuchen Sie es später erneut.");
    }
  };

  const cancelEdit = () => {
    setEditingIndex(null);
    setTimeframeId("");
    setWeekday("monday");
    setError(null);
  };

  // Get a display label for a timeframe
  const getTimeframeLabel = (timeframeId: string): string => {
    const timeframe = timeframes.find(tf => tf.id === timeframeId);
    if (!timeframe) return "Unbekannter Zeitrahmen";
    
    return timeframe.name || `${timeframe.start_time}-${timeframe.end_time}`;
  };

  // Format a time slot for display
  const formatTimeSlot = (slot: ActivitySchedule): string => {
    const weekdayText = formatWeekday(slot.weekday);
    
    if (!slot.timeframe_id || !timeframes.length) {
      return weekdayText;
    }
    
    const timeframe = timeframes.find(tf => tf.id === slot.timeframe_id);
    if (!timeframe) return weekdayText;
    
    return `${weekdayText} (${timeframe.name || `${timeframe.start_time}-${timeframe.end_time}`})`;
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

  // Sort time slots by weekday
  const sortedTimeSlots = [...timeSlots].sort((a, b) => {
    const weekdayOrder: Record<string, number> = {
      monday: 1,
      tuesday: 2,
      wednesday: 3,
      thursday: 4,
      friday: 5,
      saturday: 6,
      sunday: 7,
    };
    return (weekdayOrder[a.weekday.toLowerCase()] || 99) - (weekdayOrder[b.weekday.toLowerCase()] || 99);
  });

  // Group time slots by weekday for better organization
  const timeSlotsByWeekday = sortedTimeSlots.reduce((acc, slot) => {
    const day = slot.weekday.toLowerCase();
    if (!acc[day]) {
      acc[day] = [];
    }
    acc[day].push(slot);
    return acc;
  }, {} as Record<string, ActivitySchedule[]>);

  return (
    <div className="space-y-4">
      <h3 className="text-md font-medium text-gray-700">Zeitslots</h3>

      {error && (
        <div className="rounded-lg bg-red-50 p-3 text-sm text-red-700">
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700">
            Wochentag
          </label>
          <select
            value={weekday}
            onChange={(e) => setWeekday(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-purple-500 focus:outline-none"
            aria-label="Wählen Sie einen Wochentag"
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
            Zeitrahmen
          </label>
          <select
            value={timeframeId}
            onChange={(e) => setTimeframeId(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-4 py-2 transition-all duration-200 focus:ring-2 focus:ring-purple-500 focus:outline-none"
            aria-label="Wählen Sie einen Zeitrahmen"
          >
            <option value="">Ganztägig</option>
            {timeframes.map((timeframe) => {
              const isTaken = isTimeSlotTaken(weekday, timeframe.id);
              const isConflict = isTimeSlotConflict(
                weekday, 
                timeframe.id, 
                editingIndex !== null ? editingIndex : undefined
              );
              
              // If we're editing, don't disable if it's the same as the current value
              const disabled = editingIndex !== null ? 
                isConflict : // When editing, only disable conflicts
                isTaken;    // When adding new, disable taken slots
              
              return (
                <option 
                  key={timeframe.id} 
                  value={timeframe.id}
                  disabled={disabled}
                  className={disabled ? "text-gray-400" : ""}
                >
                  {timeframe.name || `${timeframe.start_time}-${timeframe.end_time}`}
                  {disabled ? ' (Bereits belegt)' : ''}
                </option>
              );
            })}
          </select>
          <div className="mt-1 text-xs text-gray-500">
            {timeframeId ? (
              timeframes.find(tf => tf.id === timeframeId) ? (
                <span>Zeitrahmen: {getTimeframeLabel(timeframeId)}</span>
              ) : "Unbekannter Zeitrahmen"
            ) : "Bei 'Ganztägig' wird kein spezifischer Zeitrahmen festgelegt"}
          </div>
        </div>
      </div>

      <div className="flex justify-end gap-2">
        {editingIndex !== null ? (
          <>
            <button
              type="button"
              onClick={cancelEdit}
              className="rounded-lg border border-gray-300 bg-white px-4 py-2 text-gray-700 transition-colors hover:bg-gray-50"
            >
              Abbrechen
            </button>
            <button
              type="button"
              onClick={handleSaveEdit}
              className="rounded-lg bg-blue-600 px-4 py-2 text-white transition-colors hover:bg-blue-700"
            >
              Änderungen speichern
            </button>
          </>
        ) : (
          <button
            type="button"
            onClick={handleAddTimeSlot}
            disabled={isCreatingTimespan}
            className="rounded-lg bg-purple-600 px-4 py-2 text-white transition-colors hover:bg-purple-700 disabled:cursor-not-allowed disabled:bg-gray-400"
          >
            {isCreatingTimespan ? "Wird hinzugefügt..." : "Zeitslot hinzufügen"}
          </button>
        )}
      </div>

      {sortedTimeSlots.length > 0 && (
        <div className="mt-4">
          <h4 className="mb-2 text-sm font-medium text-gray-700">
            Vorhandene Zeitslots:
          </h4>
          
          {/* Group by weekday for better organization */}
          <div className="space-y-4">
            {Object.entries(timeSlotsByWeekday).map(([day, daySlots]) => (
              <div key={day} className="rounded-lg border border-gray-200 overflow-hidden">
                <div className="bg-gray-100 px-3 py-2 font-medium">{formatWeekday(day)}</div>
                <ul className="divide-y divide-gray-100">
                  {daySlots.map((slot, slotIndex) => {
                    const index = timeSlots.findIndex(s => s.id === slot.id);
                    const timeframe = slot.timeframe_id ? 
                      timeframes.find(tf => tf.id === slot.timeframe_id) : null;
                      
                    return (
                      <li
                        key={slot.id || slotIndex}
                        className="flex items-center justify-between px-3 py-2 hover:bg-gray-50"
                      >
                        <span className="flex items-center">
                          <div className={`h-3 w-3 rounded-full mr-2 ${timeframe ? 'bg-blue-500' : 'bg-green-500'}`} />
                          {timeframe ? (
                            <span>
                              {timeframe.name || `${timeframe.start_time}-${timeframe.end_time}`}
                            </span>
                          ) : (
                            <span className="text-gray-500">Ganztägig</span>
                          )}
                        </span>
                        <div className="flex space-x-2">
                          {onEdit && (
                            <button
                              type="button"
                              onClick={() => handleEditTimeSlot(index)}
                              className="text-blue-500 hover:text-blue-700"
                              aria-label="Zeitslot bearbeiten"
                            >
                              <svg 
                                xmlns="http://www.w3.org/2000/svg" 
                                viewBox="0 0 20 20"
                                fill="currentColor"
                                className="h-5 w-5"
                              >
                                <path d="M5.433 13.917l1.262-3.155A4 4 0 017.58 9.42l6.92-6.918a2.121 2.121 0 013 3l-6.92 6.918c-.383.383-.84.685-1.343.886l-3.154 1.262a.5.5 0 01-.65-.65z" />
                                <path d="M3.5 5.75c0-.69.56-1.25 1.25-1.25H10A.75.75 0 0010 3H4.75A2.75 2.75 0 002 5.75v9.5A2.75 2.75 0 004.75 18h9.5A2.75 2.75 0 0017 15.25V10a.75.75 0 00-1.5 0v5.25c0 .69-.56 1.25-1.25 1.25h-9.5c-.69 0-1.25-.56-1.25-1.25v-9.5z" />
                              </svg>
                            </button>
                          )}
                          <button
                            type="button"
                            onClick={() => onRemove(index)}
                            className="text-red-500 hover:text-red-700"
                            aria-label="Zeitslot entfernen"
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
                        </div>
                      </li>
                    );
                  })}
                </ul>
              </div>
            ))}
          </div>
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
  const [timeframes, setTimeframes] = useState<Array<{ id: string; start_time: string; end_time: string; name?: string }>>([]);
  const [availableTimeSlots, setAvailableTimeSlots] = useState<Array<any>>([]);
  const [isLoadingTimeframes, setIsLoadingTimeframes] = useState(false);

  // Load initial data
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
  
  // Fetch timeframes and available time slots
  useEffect(() => {
    const fetchTimeframesData = async () => {
      try {
        setIsLoadingTimeframes(true);
        
        // Fetch timeframes from the API
        const response = await fetch('/api/activities/timespans');
        
        if (!response.ok) {
          throw new Error(`Failed to fetch timeframes: ${response.status} ${response.statusText}`);
        }
        
        const timeframesData = await response.json();
        
        // Check if we got valid timeframe data
        if (Array.isArray(timeframesData) && timeframesData.length > 0) {
          setTimeframes(timeframesData);
        } else if (timeframesData && 'data' in timeframesData && Array.isArray(timeframesData.data)) {
          // Handle wrapped response format
          setTimeframes(timeframesData.data);
        } else {
          console.error("Unexpected timeframes response format:", timeframesData);
          
          // Fallback to default timeframes if API fails to provide valid data
          const fallbackTimeframes = [
            { id: "1", start_time: "08:00", end_time: "09:30", name: "1. Stunde" },
            { id: "2", start_time: "09:45", end_time: "11:15", name: "2. Stunde" },
            { id: "3", start_time: "11:30", end_time: "13:00", name: "3. Stunde" },
            { id: "4", start_time: "13:30", end_time: "15:00", name: "4. Stunde" },
            { id: "5", start_time: "15:15", end_time: "16:45", name: "5. Stunde" },
          ];
          setTimeframes(fallbackTimeframes);
        }
        
        // Fetch available timeslots if we have an activity ID
        let availableSlots: any[] = [];
        
        if (formData.id) {
          try {
            // Try to fetch available time slots from API
            const slotsResponse = await fetch(`/api/activities/${formData.id}/schedules/available`);
            
            if (slotsResponse.ok) {
              const slotsData = await slotsResponse.json();
              if (Array.isArray(slotsData)) {
                availableSlots = slotsData;
              } else if (slotsData && 'data' in slotsData && Array.isArray(slotsData.data)) {
                availableSlots = slotsData.data;
              }
            }
          } catch (slotsError) {
            console.error("Error fetching available time slots:", slotsError);
          }
        }
        
        // If we couldn't get slots from API or it's a new activity, generate all possible slots
        if (availableSlots.length === 0) {
          availableSlots = timeframes.flatMap(tf => 
            ["monday", "tuesday", "wednesday", "thursday", "friday"].map(day => ({
              weekday: day,
              timeframe_id: tf.id
            }))
          );
        }
        
        setAvailableTimeSlots(availableSlots);
      } catch (error) {
        console.error("Error fetching timeframes:", error);
        // Fallback to default timeframes if API fails
        const fallbackTimeframes = [
          { id: "1", start_time: "08:00", end_time: "09:30", name: "1. Stunde" },
          { id: "2", start_time: "09:45", end_time: "11:15", name: "2. Stunde" },
          { id: "3", start_time: "11:30", end_time: "13:00", name: "3. Stunde" },
          { id: "4", start_time: "13:30", end_time: "15:00", name: "4. Stunde" },
          { id: "5", start_time: "15:15", end_time: "16:45", name: "5. Stunde" },
        ];
        setTimeframes(fallbackTimeframes);
        
        // Generate all possible slots with fallback timeframes
        const fallbackSlots = fallbackTimeframes.flatMap(tf => 
          ["monday", "tuesday", "wednesday", "thursday", "friday"].map(day => ({
            weekday: day,
            timeframe_id: tf.id
          }))
        );
        setAvailableTimeSlots(fallbackSlots);
      } finally {
        setIsLoadingTimeframes(false);
      }
    };
    
    fetchTimeframesData();
  }, [formData.id]);

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
      // Add schedules for backend - handle both temp and real IDs
      schedules: timeSlots.map(slot => ({
        id: slot.id && !slot.id.startsWith('temp') ? parseInt(slot.id, 10) : undefined,
        weekday: slot.weekday.toUpperCase(),
        timeframe_id: slot.timeframe_id ? parseInt(slot.timeframe_id, 10) : null
      }))
    };

    console.log("Preparing data for submission:", submissionData);
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
  
  const handleEditTimeSlot = (index: number, updatedSlot: Partial<ActivitySchedule>) => {
    setTimeSlots((prev) => {
      const newTimeSlots = [...prev];
      if (newTimeSlots[index]) {
        newTimeSlots[index] = {
          ...newTimeSlots[index],
          ...updatedSlot,
          // If this is a backend item with no changes yet, mark it as temp for UI
          id: newTimeSlots[index].id.startsWith('temp-') 
            ? newTimeSlots[index].id 
            : `temp-edit-${newTimeSlots[index].id}`,
          updated_at: new Date()
        };
      }
      return newTimeSlots;
    });
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
              onEdit={handleEditTimeSlot}
              parentActivityId={formData.id}
              timeframes={timeframes}
              availableTimeSlots={availableTimeSlots}
            />
            {isLoadingTimeframes && (
              <div className="mt-2 text-sm text-gray-500 italic">
                Zeitrahmen werden geladen...
              </div>
            )}
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