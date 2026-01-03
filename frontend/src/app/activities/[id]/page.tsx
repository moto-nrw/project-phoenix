"use client";

import { useEffect, useState, Suspense } from "react";
import { useParams, useRouter } from "next/navigation";
import { PageHeader } from "@/components/dashboard";
import {
  fetchActivity,
  getEnrolledStudents,
  getTimeframes,
} from "@/lib/activity-api";
import {
  getActivityCategoryColor,
  getWeekdayFullName,
  type Activity,
  type ActivityStudent,
  type Timeframe,
} from "@/lib/activity-helpers";

function ActivityDetailContent() {
  const router = useRouter();
  const params = useParams();
  const activityId = params.id as string;

  const [activity, setActivity] = useState<Activity | null>(null);
  const [students, setStudents] = useState<ActivityStudent[]>([]);
  const [timeframes, setTimeframes] = useState<Timeframe[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const loadActivity = async () => {
      try {
        setLoading(true);
        // Load activity details
        const data = await fetchActivity(activityId);
        setActivity(data);

        // Load enrolled students separately
        try {
          const enrolledStudents = await getEnrolledStudents(activityId);
          console.log("Enrolled students:", enrolledStudents);
          setStudents(enrolledStudents);
        } catch (error_) {
          console.error("Error fetching enrolled students:", error_);
          // Don't fail the whole page if students can't be loaded
          setStudents([]);
        }

        // Load timeframes to map IDs to names
        try {
          const timeframeData = await getTimeframes();
          setTimeframes(timeframeData);
        } catch (error_) {
          console.error("Error fetching timeframes:", error_);
          // Don't fail the whole page if timeframes can't be loaded
          setTimeframes([]);
        }

        setError(null);
      } catch (err) {
        console.error("Error fetching activity details:", err);
        setError("Fehler beim Laden der Aktivitätsdaten.");
        setActivity(null);
        setStudents([]);
      } finally {
        setLoading(false);
      }
    };

    if (activityId) {
      void loadActivity();
    }
  }, [activityId]);

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-50">
        <div className="flex animate-pulse flex-col items-center">
          <div className="h-12 w-12 rounded-full bg-gradient-to-r from-teal-400 to-blue-500"></div>
          <p className="mt-4 text-gray-500">Daten werden geladen...</p>
        </div>
      </div>
    );
  }

  if (error && !activity) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 p-4">
        <div className="max-w-md rounded-lg bg-red-50 p-6 text-red-800 shadow-md">
          <h2 className="mb-3 text-lg font-semibold">Fehler</h2>
          <p className="mb-4">{error}</p>
          <button
            onClick={() => router.back()}
            className="rounded-lg bg-red-100 px-4 py-2 text-red-800 shadow-sm transition-colors hover:bg-red-200"
          >
            Zurück
          </button>
        </div>
      </div>
    );
  }

  if (!activity) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 p-4">
        <div className="max-w-md rounded-lg bg-yellow-50 p-6 text-yellow-800 shadow-md">
          <h2 className="mb-3 text-lg font-semibold">
            Aktivität nicht gefunden
          </h2>
          <p className="mb-4">
            Die angeforderte Aktivität konnte nicht gefunden werden.
          </p>
          <button
            onClick={() => router.push("/activities")}
            className="rounded-lg bg-yellow-100 px-4 py-2 text-yellow-800 shadow-sm transition-colors hover:bg-yellow-200"
          >
            Zurück zur Übersicht
          </button>
        </div>
      </div>
    );
  }

  const categoryColor = getActivityCategoryColor(activity.category_name);

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <PageHeader title="Aktivitätsdetails" backUrl="/activities" />

      {/* Main Content */}
      <main className="mx-auto max-w-4xl p-4">
        {/* Error Alert */}
        {error && activity && (
          <div
            className="mb-4 rounded border-l-4 border-red-500 bg-red-100 p-4 text-red-700 shadow-md"
            role="alert"
          >
            <div className="flex items-start">
              <div className="flex-shrink-0">
                <svg
                  className="mr-2 h-5 w-5 text-red-500"
                  xmlns="http://www.w3.org/2000/svg"
                  viewBox="0 0 20 20"
                  fill="currentColor"
                >
                  <path
                    fillRule="evenodd"
                    d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                    clipRule="evenodd"
                  />
                </svg>
              </div>
              <div className="flex-1">
                <p className="font-bold">Fehler</p>
                <p className="mt-1 text-sm">{error}</p>
              </div>
              <button
                className="ml-2 flex-shrink-0 text-red-500 transition-colors hover:text-red-700"
                onClick={() => setError(null)}
                aria-label="Schließen"
              >
                <span className="text-xl">&times;</span>
              </button>
            </div>
          </div>
        )}

        <div className="overflow-hidden rounded-lg bg-white shadow-md">
          {/* Activity card header */}
          <div
            className={`relative bg-gradient-to-r ${categoryColor} p-6 text-white`}
          >
            <div className="flex items-center">
              <div className="mr-5 flex h-20 w-20 items-center justify-center rounded-full bg-white/30 text-3xl font-bold">
                {activity.name?.[0] ?? "A"}
              </div>
              <div>
                <h1 className="text-2xl font-bold">{activity.name}</h1>
                {activity.category_name && (
                  <p className="opacity-90">
                    Kategorie: {activity.category_name}
                  </p>
                )}
                {activity.is_open_ags && (
                  <p className="text-sm opacity-75">Offen für Teilnahme</p>
                )}
              </div>
            </div>
          </div>

          {/* Content */}
          <div className="p-6">
            <div className="mb-6">
              <h2 className="text-xl font-medium text-gray-700">
                Aktivitätsdetails
              </h2>
            </div>

            {/* Activity Information (Top Section) */}
            <div className="mb-8">
              <div className="grid grid-cols-1 gap-8 md:grid-cols-2 lg:grid-cols-3">
                {/* Activity Details */}
                <div className="space-y-4">
                  <h3 className="border-b border-blue-200 pb-2 text-lg font-medium text-blue-800">
                    Aktivitätsdaten
                  </h3>

                  <div>
                    <div className="text-sm text-gray-500">Name</div>
                    <div className="text-base">{activity.name}</div>
                  </div>

                  <div>
                    <div className="text-sm text-gray-500">Kategorie</div>
                    <div className="text-base">
                      {activity.category_name ?? "Nicht zugewiesen"}
                    </div>
                  </div>

                  <div>
                    <div className="text-sm text-gray-500">Status</div>
                    <div className="text-base">
                      {activity.is_open_ags ? (
                        <span className="inline-flex items-center rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-800">
                          Offen
                        </span>
                      ) : (
                        <span className="inline-flex items-center rounded-full bg-red-100 px-2.5 py-0.5 text-xs font-medium text-red-800">
                          Geschlossen
                        </span>
                      )}
                    </div>
                  </div>

                  <div>
                    <div className="text-sm text-gray-500">Max. Teilnehmer</div>
                    <div className="text-base">{activity.max_participant}</div>
                  </div>

                  <div>
                    <div className="text-sm text-gray-500">
                      Aktuelle Teilnehmer
                    </div>
                    <div className="text-base">
                      {activity.participant_count ?? 0}
                    </div>
                  </div>
                </div>

                {/* Supervisors */}
                <div className="space-y-4">
                  <h3 className="border-b border-purple-200 pb-2 text-lg font-medium text-purple-800">
                    Betreuer
                  </h3>

                  {activity.supervisors && activity.supervisors.length > 0 ? (
                    <div className="space-y-2">
                      {activity.supervisors.map((supervisor) => (
                        <div
                          key={supervisor.id}
                          className="rounded-lg bg-purple-50 p-3 transition-colors hover:bg-purple-100"
                        >
                          <div className="flex items-center justify-between">
                            <span>
                              {supervisor.first_name} {supervisor.last_name}
                            </span>
                            {supervisor.is_primary && (
                              <span className="text-xs text-purple-600">
                                Hauptbetreuer
                              </span>
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="text-gray-500">Keine Betreuer zugewiesen.</p>
                  )}
                </div>

                {/* Schedule */}
                <div className="space-y-4">
                  <h3 className="border-b border-orange-200 pb-2 text-lg font-medium text-orange-800">
                    Zeitplan
                  </h3>

                  {activity.times && activity.times.length > 0 ? (
                    <div className="space-y-2">
                      {activity.times.map((schedule) => {
                        const timeframe = schedule.timeframe_id
                          ? timeframes.find(
                              (tf) => tf.id === schedule.timeframe_id,
                            )
                          : null;

                        return (
                          <div
                            key={schedule.id}
                            className="rounded-lg bg-orange-50 p-3"
                          >
                            <div className="text-sm font-medium">
                              {getWeekdayFullName(schedule.weekday)}
                            </div>
                            {timeframe && (
                              <div className="mt-1 text-xs text-gray-600">
                                {timeframe.display_name ??
                                  timeframe.description ??
                                  timeframe.name}{" "}
                                ({timeframe.start_time.slice(11, 16)} -{" "}
                                {timeframe.end_time.slice(11, 16)})
                              </div>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  ) : (
                    <p className="text-gray-500">Kein Zeitplan definiert.</p>
                  )}
                </div>
              </div>
            </div>

            {/* Students Section (Full Width) */}
            <div className="border-t border-gray-200 pt-6">
              <div className="mb-4 flex items-center justify-between">
                <h3 className="border-b border-green-200 pb-2 text-lg font-medium text-green-800">
                  Teilnehmende Schüler
                </h3>

                <div className="flex items-center gap-4">
                  <div className="text-sm text-gray-500">
                    {students.length > 0
                      ? `${students.length} von ${activity.max_participant} Teilnehmer`
                      : "Keine Teilnehmer"}
                  </div>

                  <button
                    onClick={() =>
                      router.push(
                        `/database/activities/${activityId}/add-students`,
                      )
                    }
                    className="flex items-center gap-1 rounded-md bg-green-50 px-3 py-1.5 text-green-600 transition-colors hover:bg-green-100"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      className="h-4 w-4"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M12 4v16m8-8H4"
                      />
                    </svg>
                    <span>Schüler hinzufügen</span>
                  </button>
                </div>
              </div>

              {students.length > 0 ? (
                <div className="grid gap-2 md:grid-cols-2 lg:grid-cols-3">
                  {students.map((student) => {
                    // Use the enrollment's student_id if available, otherwise try to extract from id
                    const studentId = student.student_id || student.id;
                    const handleStudentClick = () =>
                      router.push(
                        `/students/${studentId}?from=/activities/${activityId}`,
                      );
                    return (
                      <button
                        type="button"
                        key={student.id}
                        className="w-full cursor-pointer rounded-lg bg-gray-50 p-3 text-left transition-colors hover:bg-gray-100"
                        onClick={handleStudentClick}
                      >
                        <div className="font-medium">{student.name}</div>
                        {student.school_class && (
                          <div className="text-sm text-gray-600">
                            Klasse: {student.school_class}
                          </div>
                        )}
                      </button>
                    );
                  })}
                </div>
              ) : (
                <div className="rounded-lg bg-yellow-50 p-4 text-center text-yellow-800">
                  <p className="mb-2 font-semibold">
                    Keine Schüler eingeschrieben
                  </p>
                  <p className="text-sm">
                    Fügen Sie Schüler zu dieser Aktivität hinzu, um die
                    Teilnahme zu verfolgen.
                  </p>
                </div>
              )}
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}

export default function ActivityDetailPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center">
          <p>Lädt...</p>
        </div>
      }
    >
      <ActivityDetailContent />
    </Suspense>
  );
}
