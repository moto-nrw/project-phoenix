"use client";

import { useState, useEffect, useCallback } from "react";
import { FormModal } from "~/components/ui";
import { SimpleAlert } from "~/components/simple/SimpleAlert";
import * as activityService from "~/lib/activity-api";
import type { Activity, ActivitySchedule } from "~/lib/activity-helpers";

// Type for available timeframes returned by the API
type AvailableTimeframe = {
  id: string;
  start_time: string;
  end_time?: string;
  description?: string;
  display_name?: string;
};

// Weekday options for selection
const WEEKDAYS = [
  { value: "1", label: "Montag" },
  { value: "2", label: "Dienstag" },
  { value: "3", label: "Mittwoch" },
  { value: "4", label: "Donnerstag" },
  { value: "5", label: "Freitag" },
  { value: "6", label: "Samstag" },
  { value: "7", label: "Sonntag" },
];

interface TimeManagementModalProps {
  isOpen: boolean;
  onClose: () => void;
  activity: Activity;
  onUpdate: () => void;
}

export function TimeManagementModal({
  isOpen,
  onClose,
  activity,
  onUpdate,
}: TimeManagementModalProps) {
  const [showSuccessAlert, setShowSuccessAlert] = useState(false);
  const [successMessage, setSuccessMessage] = useState("");
  const [showErrorAlert, setShowErrorAlert] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  const [showWarningAlert, setShowWarningAlert] = useState(false);
  const [warningMessage, setWarningMessage] = useState("");
  const [schedules, setSchedules] = useState<ActivitySchedule[]>([]);
  const [timeframes, setTimeframes] = useState<AvailableTimeframe[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [activeTab, setActiveTab] = useState<"schedules" | "add">("schedules");

  const showSuccess = useCallback((message: string) => {
    setSuccessMessage(message);
    setShowSuccessAlert(true);
  }, []);

  const showError = useCallback((message: string) => {
    setErrorMessage(message);
    setShowErrorAlert(true);
  }, []);

  const showWarning = useCallback((message: string) => {
    setWarningMessage(message);
    setShowWarningAlert(true);
  }, []);
  
  // Form state for adding new schedule
  const [newSchedule, setNewSchedule] = useState({
    weekday: "",
    timeframe_id: "",
  });

  // Fetch current schedules
  const fetchSchedules = useCallback(async () => {
    try {
      const schedulesData = await activityService.getActivitySchedules(activity.id);
      setSchedules(schedulesData);
    } catch (error) {
      console.error("Error fetching schedules:", error);
      showError("Fehler beim Laden der Zeiten");
    }
  }, [activity.id, showError]);

  // Fetch available timeframes
  const fetchTimeframes = useCallback(async () => {
    try {
      const timeframesData = await activityService.getAvailableTimeframes();
      setTimeframes(timeframesData);
    } catch (error) {
      console.error("Error fetching timeframes:", error);
      showError("Fehler beim Laden der Zeitrahmen");
    }
  }, [showError]);

  useEffect(() => {
    if (isOpen) {
      setLoading(true);
      void Promise.all([fetchSchedules(), fetchTimeframes()])
        .finally(() => setLoading(false));
    }
  }, [isOpen, activity.id, fetchSchedules, fetchTimeframes]);

  const handleAddSchedule = async () => {
    if (!newSchedule.weekday) {
      showWarning("Bitte wählen Sie einen Wochentag aus");
      return;
    }

    // Check if schedule for this weekday already exists
    const existingSchedule = schedules.find(s => s.weekday === newSchedule.weekday);
    if (existingSchedule) {
      showError("Es existiert bereits ein Termin für diesen Wochentag");
      return;
    }

    try {
      setSaving(true);
      await activityService.createActivitySchedule(activity.id, {
        activity_id: activity.id,
        weekday: newSchedule.weekday,
        timeframe_id: newSchedule.timeframe_id || undefined,
        created_at: new Date(),
        updated_at: new Date(),
      });
      
      showSuccess("Termin erfolgreich hinzugefügt");
      setNewSchedule({ weekday: "", timeframe_id: "" });
      setActiveTab("schedules");
      await fetchSchedules();
      onUpdate();
    } catch (error) {
      console.error("Error adding schedule:", error);
      showError("Fehler beim Hinzufügen des Termins");
    } finally {
      setSaving(false);
    }
  };

  const handleDeleteSchedule = async (scheduleId: string) => {
    try {
      setSaving(true);
      await activityService.deleteActivitySchedule(activity.id, scheduleId);
      showSuccess("Termin erfolgreich entfernt");
      await fetchSchedules();
      onUpdate();
    } catch (error) {
      console.error("Error deleting schedule:", error);
      showError("Fehler beim Entfernen des Termins");
    } finally {
      setSaving(false);
    }
  };

  // Helper function to get weekday name
  const getWeekdayName = (weekday: string) => {
    const day = WEEKDAYS.find(w => w.value === weekday);
    return day?.label ?? `Tag ${weekday}`;
  };

  // Helper function to get timeframe display name
  const getTimeframeDisplay = (timeframeId?: string) => {
    if (!timeframeId) return "Keine Zeit zugewiesen";
    const timeframe = timeframes.find(tf => tf.id === timeframeId);
    return timeframe?.display_name ?? `Zeitrahmen ${timeframeId}`;
  };

  const footer = activeTab === "add" && (
    <button
      onClick={handleAddSchedule}
      disabled={saving || !newSchedule.weekday}
      className="rounded-lg bg-blue-600 px-6 py-2 text-white hover:bg-blue-700 disabled:opacity-50"
    >
      {saving ? "Wird gespeichert..." : "Termin hinzufügen"}
    </button>
  );

  return (
    <>
      <FormModal
        isOpen={isOpen}
        onClose={onClose}
        title={`Zeitmanagement - ${activity.name}`}
        size="xl"
        footer={footer}
      >
        <div className="space-y-4">
          {/* Stats */}
          <div className="rounded-lg bg-gray-50 p-4">
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-600">Termine:</span>
              <span className="font-semibold">
                {schedules.length} {schedules.length === 1 ? "Termin" : "Termine"}
              </span>
            </div>
          </div>

          {/* Tabs */}
          <div className="flex border-b border-gray-200">
            <button
              onClick={() => setActiveTab("schedules")}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab === "schedules"
                  ? "border-blue-500 text-blue-600"
                  : "border-transparent text-gray-500 hover:text-gray-700"
              }`}
            >
              Aktuelle Termine ({schedules.length})
            </button>
            <button
              onClick={() => setActiveTab("add")}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab === "add"
                  ? "border-blue-500 text-blue-600"
                  : "border-transparent text-gray-500 hover:text-gray-700"
              }`}
            >
              Termin hinzufügen
            </button>
          </div>

          {/* Content */}
          {loading ? (
            <div className="text-center py-8 text-gray-500">Laden...</div>
          ) : (
            <>
              {activeTab === "schedules" && (
                <div className="space-y-2 max-h-96 overflow-y-auto">
                  {schedules.length === 0 ? (
                    <p className="text-center py-8 text-gray-500">
                      Keine Termine konfiguriert
                    </p>
                  ) : (
                    schedules.map((schedule) => (
                      <div
                        key={schedule.id}
                        className="flex items-center justify-between p-3 bg-white rounded-lg border border-gray-200"
                      >
                        <div>
                          <div className="font-medium">
                            {getWeekdayName(schedule.weekday)}
                          </div>
                          <div className="text-sm text-gray-600">
                            {getTimeframeDisplay(schedule.timeframe_id)}
                          </div>
                        </div>
                        <button
                          onClick={() => void handleDeleteSchedule(schedule.id)}
                          disabled={saving}
                          className="text-red-600 hover:text-red-800 text-sm font-medium disabled:opacity-50"
                        >
                          Entfernen
                        </button>
                      </div>
                    ))
                  )}
                </div>
              )}

              {activeTab === "add" && (
                <div className="space-y-4">
                  {/* Weekday Selection */}
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Wochentag *
                    </label>
                    <select
                      value={newSchedule.weekday}
                      onChange={(e) => setNewSchedule(prev => ({ ...prev, weekday: e.target.value }))}
                      className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                    >
                      <option value="">Wochentag auswählen</option>
                      {WEEKDAYS.filter(day => !schedules.some(s => s.weekday === day.value)).map((day) => (
                        <option key={day.value} value={day.value}>
                          {day.label}
                        </option>
                      ))}
                    </select>
                  </div>

                  {/* Timeframe Selection */}
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Zeitrahmen (optional)
                    </label>
                    <select
                      value={newSchedule.timeframe_id}
                      onChange={(e) => setNewSchedule(prev => ({ ...prev, timeframe_id: e.target.value }))}
                      className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                    >
                      <option value="">Kein Zeitrahmen</option>
                      {timeframes.map((timeframe) => (
                        <option key={timeframe.id} value={timeframe.id}>
                          {timeframe.display_name}
                        </option>
                      ))}
                    </select>
                  </div>

                  {/* Help text */}
                  <div className="text-sm text-gray-600 bg-blue-50 p-3 rounded-lg">
                    <p className="font-medium mb-1">Hinweise:</p>
                    <ul className="list-disc list-inside space-y-1">
                      <li>Wählen Sie den Wochentag für die Aktivität aus</li>
                      <li>Der Zeitrahmen ist optional und kann später hinzugefügt werden</li>
                      <li>Pro Wochentag kann nur ein Termin erstellt werden</li>
                    </ul>
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      </FormModal>
      
      {/* Alert components */}
      {showSuccessAlert && (
        <SimpleAlert
          type="success"
          message={successMessage}
          onClose={() => setShowSuccessAlert(false)}
          autoClose
        />
      )}
      {showErrorAlert && (
        <SimpleAlert
          type="error"
          message={errorMessage}
          onClose={() => setShowErrorAlert(false)}
        />
      )}
      {showWarningAlert && (
        <SimpleAlert
          type="warning"
          message={warningMessage}
          onClose={() => setShowWarningAlert(false)}
        />
      )}
    </>
  );
}