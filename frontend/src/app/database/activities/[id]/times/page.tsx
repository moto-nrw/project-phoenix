"use client";

import { useSession } from "next-auth/react";
import { redirect, useParams } from "next/navigation";
import { useState, useEffect, useCallback } from "react";
import { PageHeader, SectionTitle } from "@/components/dashboard";
import type { Activity } from "@/lib/activity-helpers";
import { activityService } from "@/lib/activity-service";
import Link from "next/link";

export default function ActivityTimesPage() {
  // const router = useRouter();
  const params = useParams();
  const { id } = params;
  const [activity, setActivity] = useState<Activity | null>(null);
  const [loading, setLoading] = useState(true);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // New time slot form
  const [weekday, setWeekday] = useState("Monday");
  const [startTime, setStartTime] = useState("");
  const [endTime, setEndTime] = useState("");
  const [addingTime, setAddingTime] = useState(false);
  // const [timeSpanId, setTimeSpanId] = useState('');

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Function to fetch the activity details
  const fetchActivity = useCallback(async () => {
    if (!id) return;

    try {
      setLoading(true);

      // Fetch from the API proxy
      const data = await activityService.getActivity(id as string);
      setActivity(data);
    } catch (_err) {
      setError(
        "Fehler beim Laden der Aktivität. Bitte versuchen Sie es später erneut.",
      );
    } finally {
      setLoading(false);
    }
  }, [id]);

  // Initial data load
  useEffect(() => {
    void fetchActivity();
  }, [fetchActivity]);

  // Loading state
  if (status === "loading" || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  // Delete a time slot
  const handleDeleteTimeSlot = async (timeId: string) => {
    try {
      setDeleting(true);

      // Delete the time slot using the service
      await activityService.deleteTimeSlot(id as string, timeId);

      // Refresh data to show changes
      void fetchActivity();
    } catch (_err) {
      setError(
        "Fehler beim Löschen des Zeitslots. Bitte versuchen Sie es später erneut.",
      );
    } finally {
      setDeleting(false);
    }
  };

  // Add a new time slot
  const handleAddTimeSlot = async () => {
    if (!startTime) {
      setError("Bitte geben Sie eine Startzeit ein");
      return;
    }

    try {
      setAddingTime(true);

      // Create a new time slot with start/end times
      const newTimeSlot = {
        weekday,
        startTime: startTime,
        endTime: endTime ?? '',
      };

      await activityService.addTimeSlot(id as string, newTimeSlot);

      // Clear form fields
      setStartTime("");
      setEndTime("");
      // This field is now commented out and no longer needed
      // setTimeSpanId(''); // Clear this field even though it's no longer visible (for backward compatibility)

      // Refresh data to show changes
      void fetchActivity();
    } catch (_err) {
      setError(
        "Fehler beim Hinzufügen des Zeitslots. Bitte versuchen Sie es später erneut.",
      );
    } finally {
      setAddingTime(false);
    }
  };

  // Show error if loading failed
  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="max-w-md rounded-lg bg-red-50 p-4 text-red-800">
          <h2 className="mb-2 font-semibold">Fehler</h2>
          <p>{error}</p>
          <button
            onClick={() => {
              setError(null);
              void fetchActivity();
            }}
            className="mt-4 rounded bg-red-100 px-4 py-2 text-red-800 transition-colors hover:bg-red-200"
          >
            Erneut versuchen
          </button>
        </div>
      </div>
    );
  }

  if (!activity) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Aktivität nicht gefunden</p>
      </div>
    );
  }

  const weekdays = [
    "Monday",
    "Tuesday",
    "Wednesday",
    "Thursday",
    "Friday",
    "Saturday",
    "Sunday",
  ];

  return (
    <div className="min-h-screen">
      {/* Header */}
      <PageHeader
        title={`Zeiten - ${activity.name}`}
        backUrl={`/database/activities/${id as string}`}
      />

      {/* Main Content */}
      <main className="mx-auto max-w-4xl p-4">
        {/* Title Section */}
        <div className="mb-8">
          <SectionTitle title="Zeiten verwalten" />
          <p className="text-gray-600">
            Bearbeiten Sie die Zeiten für {activity.name}
          </p>
        </div>

        {/* Actions */}
        <div className="mb-4 flex gap-4">
          <Link href={`/database/activities/${id as string}`}>
            <button className="rounded-lg bg-gray-500 px-4 py-2 text-white transition-colors hover:bg-gray-600">
              Zurück zu Details
            </button>
          </Link>
        </div>

        {/* Current Times List */}
        <div className="mb-8">
          <h3 className="mb-4 text-lg font-semibold">Aktuelle Zeiten</h3>
          {activity.times?.length === 0 ? (
            <p className="text-gray-500">Keine Zeiten definiert</p>
          ) : (
            <div className="space-y-2">
              {activity.times?.map((time) => (
                <div
                  key={time.id}
                  className="flex items-center justify-between rounded-lg border border-gray-200 bg-white p-4"
                >
                  <div>
                    <span className="font-medium">{time.weekday}</span>
                    <span className="ml-4">
                      {time.timespan.start_time} - {time.timespan.end_time}
                    </span>
                  </div>
                  <button
                    onClick={() => handleDeleteTimeSlot(time.id)}
                    disabled={deleting}
                    className="rounded-lg bg-red-500 px-3 py-1 text-white transition-colors hover:bg-red-600 disabled:opacity-50"
                  >
                    Löschen
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Add Time Form */}
        <div className="rounded-lg border border-gray-200 bg-white p-6">
          <h3 className="mb-4 text-lg font-semibold">Neue Zeit hinzufügen</h3>
          <div className="space-y-4">
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">
                Wochentag
              </label>
              <select
                value={weekday}
                onChange={(e) => setWeekday(e.target.value)}
                className="w-full rounded-lg border border-gray-300 px-4 py-2"
              >
                {weekdays.map((day) => (
                  <option key={day} value={day}>
                    {day}
                  </option>
                ))}
              </select>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="mb-1 block text-sm font-medium text-gray-700">
                  Startzeit
                </label>
                <input
                  type="time"
                  value={startTime}
                  onChange={(e) => setStartTime(e.target.value)}
                  className="w-full rounded-lg border border-gray-300 px-4 py-2"
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
                  className="w-full rounded-lg border border-gray-300 px-4 py-2"
                />
              </div>
            </div>

            <button
              onClick={handleAddTimeSlot}
              disabled={addingTime || !startTime}
              className="mt-4 w-full rounded-lg bg-green-500 px-4 py-2 text-white transition-colors hover:bg-green-600 disabled:opacity-50"
            >
              {addingTime ? "Hinzufügen..." : "Zeit hinzufügen"}
            </button>
          </div>
        </div>

        {/* Navigation Buttons */}
        <div className="mt-8 flex justify-between">
          <Link href={`/database/activities/${id as string}/edit`}>
            <button className="rounded-lg bg-blue-500 px-4 py-2 text-white transition-colors hover:bg-blue-600">
              Grunddaten bearbeiten
            </button>
          </Link>
          <Link href={`/database/activities/${id as string}/students`}>
            <button className="rounded-lg bg-blue-500 px-4 py-2 text-white transition-colors hover:bg-blue-600">
              Schüler verwalten
            </button>
          </Link>
        </div>
      </main>
    </div>
  );
}