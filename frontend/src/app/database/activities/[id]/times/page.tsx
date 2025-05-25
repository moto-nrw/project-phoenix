"use client";

import { useSession } from "next-auth/react";
import { redirect, useParams } from "next/navigation";
import { useState, useEffect, useCallback } from "react";
import { PageHeader, SectionTitle } from "@/components/dashboard";
import type { Activity, Timeframe } from "@/lib/activity-helpers";
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
  const [weekday, setWeekday] = useState("1");
  const [selectedTimeframeId, setSelectedTimeframeId] = useState<string>("");
  const [addingTime, setAddingTime] = useState(false);
  
  // Timeframes
  const [timeframes, setTimeframes] = useState<Timeframe[]>([]);
  const [loadingTimeframes, setLoadingTimeframes] = useState(true);

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Function to fetch timeframes
  const fetchTimeframes = useCallback(async () => {
    try {
      setLoadingTimeframes(true);
      const data = await activityService.getTimeframes();
      setTimeframes(data);
    } catch (err) {
      console.error("Error loading timeframes:", err);
    } finally {
      setLoadingTimeframes(false);
    }
  }, []);

  // Function to fetch the activity details
  const fetchActivity = useCallback(async () => {
    if (!id) return;

    try {
      setLoading(true);

      // Fetch from the API proxy
      const data = await activityService.getActivity(id as string);
      setActivity(data);
    } catch (err) {
      console.error("Error loading activity:", err);
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
    void fetchTimeframes();
  }, [fetchActivity, fetchTimeframes]);

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
    } catch (err) {
      console.error("Error deleting time slot:", err);
      setError(
        "Fehler beim Löschen des Zeitslots. Bitte versuchen Sie es später erneut.",
      );
    } finally {
      setDeleting(false);
    }
  };

  // Add a new time slot
  const handleAddTimeSlot = async () => {
    if (!selectedTimeframeId) {
      setError("Bitte wählen Sie eine Zeitrahmen aus");
      return;
    }

    try {
      setAddingTime(true);

      // Create a new schedule using the timeframe system
      const newSchedule = {
        weekday: weekday,
        timeframe_id: selectedTimeframeId,
      };

      await activityService.createActivitySchedule(id as string, newSchedule);

      // Clear form fields
      setSelectedTimeframeId("");

      // Refresh data to show changes
      void fetchActivity();
    } catch (err) {
      console.error("Error adding time slot:", err);
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
    { value: "1", label: "Montag" },
    { value: "2", label: "Dienstag" },
    { value: "3", label: "Mittwoch" },
    { value: "4", label: "Donnerstag" },
    { value: "5", label: "Freitag" },
    { value: "6", label: "Samstag" },
    { value: "7", label: "Sonntag" },
  ];

  const getWeekdayName = (weekday: string | number): string => {
    const weekdayMap: Record<string, string> = {
      "1": "Montag",
      "2": "Dienstag",
      "3": "Mittwoch",
      "4": "Donnerstag",
      "5": "Freitag",
      "6": "Samstag",
      "7": "Sonntag",
    };
    return weekdayMap[String(weekday)] ?? String(weekday);
  };

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
                    <span className="font-medium">{getWeekdayName(time.weekday)}</span>
                    {time.timeframe_id && (
                      <span className="ml-4">
                        {(() => {
                          const timeframe = timeframes.find(tf => tf.id === String(time.timeframe_id));
                          return timeframe ? timeframe.display_name ?? timeframe.description : `Timeframe ID: ${time.timeframe_id}`;
                        })()}
                      </span>
                    )}
                    {!time.timeframe_id && (
                      <span className="ml-4 text-gray-500">Keine Zeitrahmen zugewiesen</span>
                    )}
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
              {loadingTimeframes ? (
                <div className="w-full rounded-lg border border-gray-300 px-4 py-2 text-gray-500">
                  Laden...
                </div>
              ) : (
                <select
                  value={selectedTimeframeId}
                  onChange={(e) => setSelectedTimeframeId(e.target.value)}
                  className="w-full rounded-lg border border-gray-300 px-4 py-2"
                >
                  <option value="">Zeitrahmen auswählen</option>
                  {timeframes.map((timeframe) => (
                    <option key={timeframe.id} value={timeframe.id}>
                      {timeframe.display_name ?? timeframe.description}
                    </option>
                  ))}
                </select>
              )}
              {timeframes.length === 0 && !loadingTimeframes && (
                <p className="mt-1 text-sm text-red-600">
                  Keine Zeitrahmen verfügbar. Bitte erstellen Sie zuerst Zeitrahmen in den Systemeinstellungen.
                </p>
              )}
            </div>

            <button
              onClick={handleAddTimeSlot}
              disabled={addingTime || !selectedTimeframeId || loadingTimeframes}
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