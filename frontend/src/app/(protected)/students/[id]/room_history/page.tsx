"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { Alert } from "~/components/ui/alert";
import { useSession } from "next-auth/react";
import { useStudentHistoryBreadcrumb } from "~/lib/breadcrumb-context";

import { Loading } from "~/components/ui/loading";
// Student type (reused from student page)
interface Student {
  id: string;
  first_name: string;
  second_name: string;
  name?: string;
  school_class: string;
  group_id: string;
  group_name?: string;
  bus: boolean;
  current_room?: string;
  current_location?: string;
  guardian_name: string;
  guardian_contact: string;
  guardian_phone?: string;
  birthday?: string;
  notes?: string;
  buskind?: boolean;
  attendance_rate?: number;
}

// Room History interface
interface RoomHistoryEntry {
  id: string;
  timestamp: string;
  room_name: string;
  duration_minutes?: number;
  entry_type: "entry" | "exit";
  reason?: string;
}

export default function StudentRoomHistoryPage() {
  const router = useRouter();
  const params = useParams();
  const searchParams = useSearchParams();
  const studentId = params.id as string;
  const referrer = searchParams.get("from") ?? "/students/search";
  useSession(); // Ensure session is active

  const [student, setStudent] = useState<Student | null>(null);
  const [roomHistory, setRoomHistory] = useState<RoomHistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [timeRange, setTimeRange] = useState<string>("7days"); // Default to last 7 days

  useStudentHistoryBreadcrumb({ studentName: student?.name, referrer });

  // Fetch student data and room history
  useEffect(() => {
    setLoading(true);
    setError(null);

    // Simulate API request with timeout
    const timer = setTimeout(() => {
      try {
        // Mock student data (same as in student detail page)
        const mockStudent: Student = {
          id: studentId,
          first_name: "Emma",
          second_name: "Müller",
          name: "Emma Müller",
          school_class: "3b",
          group_id: "g3",
          group_name: "Eulen",
          bus: false,
          current_room: "Raum 1.2",
          current_location: "Anwesend - Raum 1.2",
          guardian_name: "Maria Müller",
          guardian_contact: "muellers@example.com",
          guardian_phone: "+49 176 12345678",
          birthday: "2016-06-15",
          notes: "Nimmt an der Musik-AG teil. Liebt Kunst und Lesen.",
          buskind: true,
          attendance_rate: 92.5,
        };

        // Mock room history data
        const mockRoomHistory: RoomHistoryEntry[] = [
          {
            id: "1",
            timestamp: "2025-05-14T09:05:23",
            room_name: "Eulen Gruppenraum (Raum 1.2)",
            entry_type: "entry",
            duration_minutes: 45,
          },
          {
            id: "2",
            timestamp: "2025-05-14T09:50:12",
            room_name: "Mensa",
            entry_type: "entry",
            duration_minutes: 30,
            reason: "Mittagessen",
          },
          {
            id: "3",
            timestamp: "2025-05-14T10:20:45",
            room_name: "Eulen Gruppenraum (Raum 1.2)",
            entry_type: "entry",
            duration_minutes: 65,
          },
          {
            id: "4",
            timestamp: "2025-05-14T11:25:10",
            room_name: "Sporthalle",
            entry_type: "entry",
            duration_minutes: 60,
            reason: "Fußball AG",
          },
          {
            id: "5",
            timestamp: "2025-05-14T12:25:33",
            room_name: "Schulhof",
            entry_type: "entry",
            duration_minutes: 30,
          },
          {
            id: "6",
            timestamp: "2025-05-14T12:55:05",
            room_name: "Eulen Gruppenraum (Raum 1.2)",
            entry_type: "entry",
            duration_minutes: 90,
          },
          {
            id: "7",
            timestamp: "2025-05-13T09:00:12",
            room_name: "Eulen Gruppenraum (Raum 1.2)",
            entry_type: "entry",
            duration_minutes: 60,
          },
          {
            id: "8",
            timestamp: "2025-05-13T10:00:45",
            room_name: "Musikraum",
            entry_type: "entry",
            duration_minutes: 45,
            reason: "Musik AG",
          },
          {
            id: "9",
            timestamp: "2025-05-13T10:45:22",
            room_name: "Mensa",
            entry_type: "entry",
            duration_minutes: 30,
            reason: "Mittagessen",
          },
          {
            id: "10",
            timestamp: "2025-05-13T11:15:08",
            room_name: "Eulen Gruppenraum (Raum 1.2)",
            entry_type: "entry",
            duration_minutes: 120,
          },
        ];

        setStudent(mockStudent);
        setRoomHistory(mockRoomHistory);
        setLoading(false);
      } catch (err) {
        console.error("Error fetching data:", err);
        setError("Fehler beim Laden der Daten.");
        setLoading(false);
      }
    }, 800);

    return () => clearTimeout(timer);
  }, [studentId, timeRange]);

  // Get year from class
  const getYear = (schoolClass: string): number => {
    const yearMatch = /^(\d)/.exec(schoolClass);
    return yearMatch?.[1] ? Number.parseInt(yearMatch[1], 10) : 0;
  };

  // Determine color for year indicator
  const getYearColor = (year: number): string => {
    switch (year) {
      case 1:
        return "bg-blue-500";
      case 2:
        return "bg-green-500";
      case 3:
        return "bg-yellow-500";
      case 4:
        return "bg-purple-500";
      default:
        return "bg-gray-400";
    }
  };

  // Format date for display
  const formatDate = (dateString: string): string => {
    const date = new Date(dateString);
    return date.toLocaleDateString("de-DE", {
      weekday: "long",
      year: "numeric",
      month: "long",
      day: "numeric",
    });
  };

  // Format time for display
  const formatTime = (dateString: string): string => {
    const date = new Date(dateString);
    return date.toLocaleTimeString("de-DE", {
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  // Group room history by date
  const groupedRoomHistory = roomHistory.reduce(
    (groups, entry) => {
      const date = new Date(entry.timestamp).toLocaleDateString("de-DE", {
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
      });
      groups[date] ??= [];
      groups[date].push(entry);
      return groups;
    },
    {} as Record<string, RoomHistoryEntry[]>,
  );

  // Sort dates in descending order (most recent first)
  const sortedDates = Object.keys(groupedRoomHistory).sort((a, b) => {
    return new Date(b).getTime() - new Date(a).getTime();
  });

  const year = student ? getYear(student.school_class) : 0;
  const yearColor = getYearColor(year);

  if (loading) {
    return <Loading fullPage={false} />;
  }

  if (error || !student) {
    return (
      <div className="flex min-h-[80vh] flex-col items-center justify-center">
        <Alert type="error" message={error ?? "Schüler nicht gefunden"} />
        <button
          onClick={() => router.push(referrer)}
          className="mt-4 rounded bg-blue-100 px-4 py-2 text-blue-800 transition-colors hover:bg-blue-200"
        >
          Zurück
        </button>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl">
      {/* Back Button */}
      <div className="mb-6">
        <button
          onClick={() => router.push(`/students/${studentId}?from=${referrer}`)}
          className="flex items-center text-gray-600 transition-colors hover:text-blue-600"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            className="mr-1 h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M10 19l-7-7m0 0l7-7m-7 7h18"
            />
          </svg>
          Zurück zum Schülerprofil
        </button>
      </div>

      {/* Student Profile Header with Status */}
      <div className="relative mb-8 overflow-hidden rounded-xl bg-gradient-to-r from-teal-500 to-blue-600 p-6 text-white shadow-md">
        <div className="flex items-center">
          <div className="mr-6 flex h-24 w-24 items-center justify-center rounded-full bg-white/30 text-4xl font-bold">
            {student.first_name[0]}
            {student.second_name[0]}
          </div>
          <div>
            <h1 className="text-3xl font-bold">{student.name}</h1>
            <div className="mt-1 flex items-center">
              <span className="opacity-90">Klasse {student.school_class}</span>
              <span
                className={`ml-2 inline-block h-3 w-3 rounded-full ${yearColor}`}
                title={`Jahrgang ${year}`}
              ></span>
              <span className="mx-2">•</span>
              <span className="opacity-90">Gruppe: {student.group_name}</span>
            </div>
            <div className="mt-4">
              <h2 className="text-2xl font-semibold text-white">Raumverlauf</h2>
            </div>
          </div>
        </div>
      </div>

      {/* Time Range Filter */}
      <div className="mb-8">
        <div className="rounded-lg bg-white p-4 shadow-sm">
          <h2 className="mb-3 text-lg font-medium text-gray-800">
            Zeitraum auswählen
          </h2>
          <div className="flex flex-wrap gap-2">
            <button
              className={`rounded-lg px-4 py-2 transition-colors ${timeRange === "today" ? "bg-blue-500 text-white" : "bg-gray-100 text-gray-700 hover:bg-gray-200"}`}
              onClick={() => setTimeRange("today")}
            >
              Heute
            </button>
            <button
              className={`rounded-lg px-4 py-2 transition-colors ${timeRange === "week" ? "bg-blue-500 text-white" : "bg-gray-100 text-gray-700 hover:bg-gray-200"}`}
              onClick={() => setTimeRange("week")}
            >
              Diese Woche
            </button>
            <button
              className={`rounded-lg px-4 py-2 transition-colors ${timeRange === "7days" ? "bg-blue-500 text-white" : "bg-gray-100 text-gray-700 hover:bg-gray-200"}`}
              onClick={() => setTimeRange("7days")}
            >
              Letzte 7 Tage
            </button>
            <button
              className={`rounded-lg px-4 py-2 transition-colors ${timeRange === "month" ? "bg-blue-500 text-white" : "bg-gray-100 text-gray-700 hover:bg-gray-200"}`}
              onClick={() => setTimeRange("month")}
            >
              Diesen Monat
            </button>
          </div>
        </div>
      </div>

      {/* Room History Timeline */}
      <div className="space-y-6">
        {roomHistory.length === 0 ? (
          <div className="rounded-lg bg-white p-8 text-center shadow-sm">
            <p className="text-gray-500">
              Keine Raumhistorie für den ausgewählten Zeitraum verfügbar.
            </p>
          </div>
        ) : (
          sortedDates.map((date) => (
            <div
              key={date}
              className="overflow-hidden rounded-lg bg-white shadow-sm"
            >
              <div className="border-b border-blue-100 bg-blue-50 px-6 py-3">
                <h3 className="font-medium text-blue-800">
                  {groupedRoomHistory[date]?.[0]?.timestamp
                    ? formatDate(groupedRoomHistory[date][0].timestamp)
                    : ""}
                </h3>
              </div>
              <div className="px-6 py-2">
                <ul className="relative">
                  {groupedRoomHistory[date]
                    ?.sort(
                      (a, b) =>
                        new Date(b.timestamp).getTime() -
                        new Date(a.timestamp).getTime(),
                    )
                    .map((entry, index) => (
                      <li
                        key={entry.id}
                        className="relative border-l-2 border-gray-200 py-4 pl-8"
                      >
                        {/* Time dot */}
                        <div className="absolute -left-[5px] mt-1.5 h-3 w-3 rounded-full bg-blue-500"></div>

                        <div className="flex flex-col space-y-1">
                          <span className="text-sm text-gray-500">
                            {formatTime(entry.timestamp)}
                          </span>

                          <div className="flex items-start gap-2">
                            <span className="font-medium text-gray-900">
                              {entry.room_name}
                            </span>

                            {entry.reason && (
                              <span className="inline-flex items-center rounded-full bg-blue-100 px-2.5 py-0.5 text-xs font-medium text-blue-800">
                                {entry.reason}
                              </span>
                            )}
                          </div>

                          {entry.duration_minutes && (
                            <span className="text-sm text-gray-600">
                              Dauer:{" "}
                              {Math.floor(entry.duration_minutes / 60) > 0
                                ? `${Math.floor(entry.duration_minutes / 60)} Std. ${entry.duration_minutes % 60} Min.`
                                : `${entry.duration_minutes} Min.`}
                            </span>
                          )}
                        </div>

                        {/* Last item in the day doesn't need connecting line to next item */}
                        {groupedRoomHistory[date] &&
                          index !== groupedRoomHistory[date].length - 1 && (
                            <div className="absolute top-10 bottom-0 left-[-2px] w-0.5 bg-gray-200"></div>
                          )}
                      </li>
                    ))}
                </ul>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
