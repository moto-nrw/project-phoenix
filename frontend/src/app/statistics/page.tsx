// app/statistics/page.tsx
"use client";

import { useState, useEffect } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { PageHeaderWithSearch } from "~/components/ui/page-header";

import { Loading } from "~/components/ui/loading";

// Type definitions for statistics
interface SchoolStats {
  totalStudents: number;
  presentStudents: number;
  absentStudents: number;
  schoolyard: number;
  bus: number;
  sickReported: number;
  activeGroups: number;
  totalCapacity: number;
  occupancyRate: number;
}

interface AttendanceData {
  date: string;
  present: number;
  absent: number;
}

interface RoomUtilization {
  room: string;
  usagePercent: number;
  category: string;
}

interface ActivityParticipation {
  activity: string;
  students: number;
  totalSlots: number;
  category: string;
}

interface FeedbackSummary {
  positive: number;
  neutral: number;
  negative: number;
  total: number;
}

export default function StatisticsPage() {
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // State variables
  const [schoolStats, setSchoolStats] = useState<SchoolStats | null>(null);
  const [attendanceData, setAttendanceData] = useState<AttendanceData[]>([]);
  const [roomUtilization, setRoomUtilization] = useState<RoomUtilization[]>([]);
  const [activityParticipation, setActivityParticipation] = useState<
    ActivityParticipation[]
  >([]);
  const [feedbackSummary, setFeedbackSummary] =
    useState<FeedbackSummary | null>(null);
  const [timeRange, setTimeRange] = useState<string>("7days");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Category-to-color mapping using batch colors
  const categoryColors: Record<string, string> = {
    Gruppenraum: "#5080d8", // Blue
    Lernen: "#83CD2D", // Green
    Spielen: "#F78C10", // Orange
    "Bewegen/Ruhe": "#5080d8", // Blue
    Hauswirtschaft: "#F78C10", // Orange
    Natur: "#83CD2D", // Green
    "Kreatives/Musik": "#5080d8", // Blue
    "NW/Technik": "#83CD2D", // Green
  };

  // Fetch statistics data
  useEffect(() => {
    setLoading(true);
    setError(null);

    // Simulate API request with timeout
    const timer = setTimeout(() => {
      try {
        // Mock school stats
        const mockSchoolStats: SchoolStats = {
          totalStudents: 150,
          presentStudents: 127,
          absentStudents: 23,
          schoolyard: 32,
          bus: 10,
          sickReported: 7,
          activeGroups: 8,
          totalCapacity: 180,
          occupancyRate: 70.5,
        };

        // Mock attendance data for the last 7 days
        const mockAttendanceData: AttendanceData[] = [
          { date: "2025-05-09", present: 130, absent: 20 },
          { date: "2025-05-10", present: 125, absent: 25 },
          { date: "2025-05-11", present: 128, absent: 22 },
          { date: "2025-05-12", present: 131, absent: 19 },
          { date: "2025-05-13", present: 124, absent: 26 },
          { date: "2025-05-14", present: 127, absent: 23 },
          { date: "2025-05-15", present: 129, absent: 21 },
        ];

        // Mock room utilization data
        const mockRoomUtilization: RoomUtilization[] = [
          {
            room: "Klassenraum 101",
            usagePercent: 85,
            category: "Gruppenraum",
          },
          { room: "Musiksaal", usagePercent: 60, category: "Kreatives/Musik" },
          { room: "Computerraum 1", usagePercent: 75, category: "NW/Technik" },
          { room: "Kunstraum", usagePercent: 55, category: "Kreatives/Musik" },
          { room: "Turnhalle", usagePercent: 90, category: "Bewegen/Ruhe" },
          {
            room: "Klassenraum 102",
            usagePercent: 70,
            category: "Gruppenraum",
          },
          { room: "Klassenraum 201", usagePercent: 80, category: "Lernen" },
          { room: "Bibliothek", usagePercent: 45, category: "Lernen" },
        ];

        // Mock activity participation data
        const mockActivityParticipation: ActivityParticipation[] = [
          {
            activity: "Fußball AG",
            students: 12,
            totalSlots: 15,
            category: "Bewegen/Ruhe",
          },
          {
            activity: "Coding Club",
            students: 8,
            totalSlots: 10,
            category: "NW/Technik",
          },
          {
            activity: "Musikgruppe",
            students: 14,
            totalSlots: 15,
            category: "Kreatives/Musik",
          },
          {
            activity: "Lesezirkel",
            students: 7,
            totalSlots: 10,
            category: "Lernen",
          },
          {
            activity: "Kochgruppe",
            students: 9,
            totalSlots: 12,
            category: "Hauswirtschaft",
          },
          {
            activity: "Naturerkundung",
            students: 11,
            totalSlots: 15,
            category: "Natur",
          },
          {
            activity: "Schach AG",
            students: 6,
            totalSlots: 8,
            category: "Spielen",
          },
          {
            activity: "Kunstwerkstatt",
            students: 10,
            totalSlots: 12,
            category: "Sonstiges",
          },
        ];

        // Mock feedback summary data
        const mockFeedbackSummary: FeedbackSummary = {
          positive: 75,
          neutral: 20,
          negative: 5,
          total: 100,
        };

        setSchoolStats(mockSchoolStats);
        setAttendanceData(mockAttendanceData);
        setRoomUtilization(mockRoomUtilization);
        setActivityParticipation(mockActivityParticipation);
        setFeedbackSummary(mockFeedbackSummary);
        setLoading(false);
      } catch (err) {
        console.error("Error fetching statistics:", err);
        setError("Fehler beim Laden der Statistikdaten.");
        setLoading(false);
      }
    }, 800);

    return () => clearTimeout(timer);
  }, [timeRange]);

  if (status === "loading" || loading) {
    return <Loading fullPage={false} />;
  }

  if (error) {
    return (
      <ResponsiveLayout>
        <div className="flex min-h-[80vh] flex-col items-center justify-center">
          <Alert type="error" message={error} />
        </div>
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout>
      <div className="space-y-8">
        {/* Page Header with Export Button */}
        <div className="mb-8">
          <div className="flex items-center justify-between gap-4">
            <PageHeaderWithSearch
              title="Statistiken"
              badge={{
                icon: (
                  <svg
                    className="h-5 w-5 text-gray-600"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
                    />
                  </svg>
                ),
                count: schoolStats?.totalStudents ?? 0,
                label: "Schüler",
              }}
            />
            {/* Export Button in Filter Style */}
            <button className="flex h-10 items-center gap-2 rounded-xl bg-white px-3 py-2 text-sm font-medium shadow-sm transition-all hover:bg-gray-50">
              <svg
                className="h-4 w-4 text-gray-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
                />
              </svg>
              <span className="text-gray-600">Export</span>
            </button>
          </div>
        </div>

        {/* Key Metrics - Modern Glassmorphism Cards */}
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-4">
          <div className="group relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 hover:-translate-y-2 hover:scale-[1.03] hover:bg-white hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]">
            {/* Gradient overlay */}
            <div className="absolute inset-0 bg-gradient-to-br from-[#5080d8]/5 to-blue-100/5 opacity-30"></div>
            {/* Inner glow */}
            <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
            {/* Border highlight */}
            <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 group-hover:ring-[#5080d8]/30"></div>

            <div className="relative p-6">
              <div className="mb-4 flex items-center justify-between">
                <div className="relative">
                  <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-[#5080d8] to-[#4070c8] shadow-[0_8px_25px_rgba(80,128,216,0.4)] transition-transform duration-300 group-hover:scale-110">
                    <svg
                      className="h-7 w-7 text-white"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
                      />
                    </svg>
                  </div>
                  <div className="absolute -inset-1 rounded-2xl bg-gradient-to-br from-[#5080d8] to-[#4070c8] opacity-25 blur-lg transition-opacity duration-300 group-hover:opacity-40"></div>
                </div>
                <span className="bg-gradient-to-br from-gray-900 to-gray-700 bg-clip-text text-3xl font-bold text-transparent">
                  {schoolStats?.totalStudents}
                </span>
              </div>
              <p className="text-sm font-medium text-gray-600">
                Gesamt Schüler
              </p>
              <div className="mt-3 flex items-center text-xs text-gray-500">
                <span className="mr-2 h-2 w-2 animate-pulse rounded-full bg-[#5080d8]"></span>
                Aktiv im System
              </div>
            </div>
          </div>

          <div className="group relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 hover:-translate-y-2 hover:scale-[1.03] hover:bg-white hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]">
            {/* Gradient overlay */}
            <div className="absolute inset-0 bg-gradient-to-br from-[#83CD2D]/5 to-green-100/5 opacity-30"></div>
            {/* Inner glow */}
            <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
            {/* Border highlight */}
            <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 group-hover:ring-[#83CD2D]/30"></div>

            <div className="relative p-6">
              <div className="mb-4 flex items-center justify-between">
                <div className="relative">
                  <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-[#83CD2D] to-[#73BD1D] shadow-[0_8px_25px_rgba(131,205,45,0.4)] transition-transform duration-300 group-hover:scale-110">
                    <svg
                      className="h-7 w-7 text-white"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                      />
                    </svg>
                  </div>
                  <div className="absolute -inset-1 rounded-2xl bg-gradient-to-br from-[#83CD2D] to-[#73BD1D] opacity-25 blur-lg transition-opacity duration-300 group-hover:opacity-40"></div>
                </div>
                <span className="bg-gradient-to-br from-gray-900 to-gray-700 bg-clip-text text-3xl font-bold text-transparent">
                  {schoolStats?.presentStudents}
                </span>
              </div>
              <p className="text-sm font-medium text-gray-600">
                Anwesend heute
              </p>
              <div className="mt-3 flex items-center text-xs text-gray-500">
                <span className="mr-2 h-2 w-2 animate-pulse rounded-full bg-[#83CD2D]"></span>
                {(
                  ((schoolStats?.presentStudents ?? 0) /
                    (schoolStats?.totalStudents ?? 1)) *
                  100
                ).toFixed(0)}
                % Anwesenheit
              </div>
            </div>
          </div>

          <div className="group relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 hover:-translate-y-2 hover:scale-[1.03] hover:bg-white hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]">
            {/* Gradient overlay */}
            <div className="absolute inset-0 bg-gradient-to-br from-[#F78C10]/5 to-orange-100/5 opacity-30"></div>
            {/* Inner glow */}
            <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
            {/* Border highlight */}
            <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 group-hover:ring-[#F78C10]/30"></div>

            <div className="relative p-6">
              <div className="mb-4 flex items-center justify-between">
                <div className="relative">
                  <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-[#F78C10] to-[#E77C00] shadow-[0_8px_25px_rgba(247,140,16,0.4)] transition-transform duration-300 group-hover:scale-110">
                    <svg
                      className="h-7 w-7 text-white"
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
                  </div>
                  <div className="absolute -inset-1 rounded-2xl bg-gradient-to-br from-[#F78C10] to-[#E77C00] opacity-25 blur-lg transition-opacity duration-300 group-hover:opacity-40"></div>
                </div>
                <span className="bg-gradient-to-br from-gray-900 to-gray-700 bg-clip-text text-3xl font-bold text-transparent">
                  {(schoolStats?.occupancyRate ?? 0).toFixed(0)}%
                </span>
              </div>
              <p className="text-sm font-medium text-gray-600">
                Raumauslastung
              </p>
              <div className="mt-3 flex items-center text-xs text-gray-500">
                <span className="mr-2 h-2 w-2 animate-pulse rounded-full bg-[#F78C10]"></span>
                Durchschnittlich
              </div>
            </div>
          </div>

          <div className="group relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 hover:-translate-y-2 hover:scale-[1.03] hover:bg-white hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]">
            {/* Gradient overlay */}
            <div className="absolute inset-0 bg-gradient-to-br from-[#5080d8]/5 to-purple-100/5 opacity-30"></div>
            {/* Inner glow */}
            <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
            {/* Border highlight */}
            <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 group-hover:ring-[#5080d8]/30"></div>

            <div className="relative p-6">
              <div className="mb-4 flex items-center justify-between">
                <div className="relative">
                  <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-[#5080d8] to-[#4070c8] shadow-[0_8px_25px_rgba(80,128,216,0.4)] transition-transform duration-300 group-hover:scale-110">
                    <svg
                      className="h-7 w-7 text-white"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
                      />
                    </svg>
                  </div>
                  <div className="absolute -inset-1 rounded-2xl bg-gradient-to-br from-[#5080d8] to-[#4070c8] opacity-25 blur-lg transition-opacity duration-300 group-hover:opacity-40"></div>
                </div>
                <span className="bg-gradient-to-br from-gray-900 to-gray-700 bg-clip-text text-3xl font-bold text-transparent">
                  {schoolStats?.activeGroups}
                </span>
              </div>
              <p className="text-sm font-medium text-gray-600">
                Aktive Gruppen
              </p>
              <div className="mt-3 flex items-center text-xs text-gray-500">
                <span className="mr-2 h-2 w-2 animate-pulse rounded-full bg-[#5080d8]"></span>
                In Betreuung
              </div>
            </div>
          </div>
        </div>

        {/* Location Distribution - Modern Glassmorphism Card */}
        <div className="relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md">
          {/* Gradient overlay */}
          <div className="absolute inset-0 bg-gradient-to-br from-gray-50/50 to-white/30 opacity-50"></div>
          {/* Inner glow */}
          <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>

          <div className="relative p-8">
            <h2 className="mb-8 text-xl font-bold text-gray-900">
              Aktuelle Verteilung
            </h2>
            <div className="grid grid-cols-2 gap-6 md:grid-cols-5">
              <div className="group text-center transition-all duration-300 hover:scale-105">
                <div className="relative mb-4 inline-flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-br from-[#83CD2D] to-[#73BD1D] shadow-[0_8px_25px_rgba(131,205,45,0.4)] transition-all duration-300 group-hover:shadow-[0_12px_30px_rgba(131,205,45,0.5)]">
                  <span className="text-2xl font-bold text-white">
                    {schoolStats?.presentStudents}
                  </span>
                  <div className="absolute inset-0 rounded-2xl bg-white/10 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                </div>
                <p className="text-sm font-medium text-gray-700">In Räumen</p>
              </div>

              <div className="group text-center transition-all duration-300 hover:scale-105">
                <div className="relative mb-4 inline-flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-br from-[#5080d8] to-[#4070c8] shadow-[0_8px_25px_rgba(80,128,216,0.4)] transition-all duration-300 group-hover:shadow-[0_12px_30px_rgba(80,128,216,0.5)]">
                  <span className="text-2xl font-bold text-white">
                    {schoolStats?.schoolyard}
                  </span>
                  <div className="absolute inset-0 rounded-2xl bg-white/10 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                </div>
                <p className="text-sm font-medium text-gray-700">Schulhof</p>
              </div>

              <div className="group text-center transition-all duration-300 hover:scale-105">
                <div className="relative mb-4 inline-flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-br from-[#F78C10] to-[#E77C00] shadow-[0_8px_25px_rgba(247,140,16,0.4)] transition-all duration-300 group-hover:shadow-[0_12px_30px_rgba(247,140,16,0.5)]">
                  <span className="text-2xl font-bold text-white">
                    {schoolStats?.bus}
                  </span>
                  <div className="absolute inset-0 rounded-2xl bg-white/10 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                </div>
                <p className="text-sm font-medium text-gray-700">Unterwegs</p>
              </div>

              <div className="group text-center transition-all duration-300 hover:scale-105">
                <div className="relative mb-4 inline-flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-br from-[#D946EF] to-[#C936DF] shadow-[0_8px_25px_rgba(217,70,239,0.4)] transition-all duration-300 group-hover:shadow-[0_12px_30px_rgba(217,70,239,0.5)]">
                  <span className="text-2xl font-bold text-white">
                    {schoolStats?.sickReported}
                  </span>
                  <div className="absolute inset-0 rounded-2xl bg-white/10 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                </div>
                <p className="text-sm font-medium text-gray-700">
                  Krankgemeldet
                </p>
              </div>

              <div className="group text-center transition-all duration-300 hover:scale-105">
                <div className="relative mb-4 inline-flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-br from-[#FF3130] to-[#EF2120] shadow-[0_8px_25px_rgba(255,49,48,0.4)] transition-all duration-300 group-hover:shadow-[0_12px_30px_rgba(255,49,48,0.5)]">
                  <span className="text-2xl font-bold text-white">
                    {schoolStats?.absentStudents}
                  </span>
                  <div className="absolute inset-0 rounded-2xl bg-white/10 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                </div>
                <p className="text-sm font-medium text-gray-700">Zuhause</p>
              </div>
            </div>
          </div>
        </div>

        {/* Time Range Filter - OGS Group Style */}
        <div className="flex h-10 w-fit rounded-xl bg-white p-1 shadow-sm">
          {[
            { value: "today", label: "Heute" },
            { value: "week", label: "Diese Woche" },
            { value: "7days", label: "7 Tage" },
            { value: "month", label: "Monat" },
            { value: "year", label: "Jahr" },
          ].map((option) => (
            <button
              key={option.value}
              type="button"
              onClick={() => setTimeRange(option.value)}
              className={`rounded-lg px-3 text-sm font-medium transition-all ${
                timeRange === option.value
                  ? "bg-gray-900 text-white"
                  : "text-gray-600 hover:text-gray-900"
              } `}
            >
              {option.label}
            </button>
          ))}
        </div>

        {/* Attendance Chart - Modern Glassmorphism Design */}
        <div className="relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md">
          {/* Gradient overlay */}
          <div className="absolute inset-0 bg-gradient-to-br from-gray-50/50 to-white/30 opacity-50"></div>
          {/* Inner glow */}
          <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>

          <div className="relative p-8">
            <h2 className="mb-8 text-xl font-bold text-gray-900">
              Anwesenheitsverlauf
            </h2>
            <div className="relative h-64">
              <svg
                width="100%"
                height="100%"
                viewBox="0 0 800 250"
                className="overflow-visible"
              >
                {/* Grid lines */}
                {[0, 50, 100, 150].map((y) => (
                  <line
                    key={y}
                    x1="50"
                    y1={200 - y * 0.8}
                    x2="750"
                    y2={200 - y * 0.8}
                    stroke="#F3F4F6"
                    strokeWidth="1"
                  />
                ))}

                {/* Data lines */}
                {attendanceData.map((entry, index) => {
                  const x = 100 + index * 100;
                  const presentY = 200 - (entry.present / 200) * 160;
                  const absentY = 200 - (entry.absent / 200) * 160;

                  return (
                    <g key={index}>
                      {/* Present line */}
                      {index > 0 && (
                        <line
                          x1={100 + (index - 1) * 100}
                          y1={
                            200 -
                            ((attendanceData[index - 1]?.present ?? 0) / 200) *
                              160
                          }
                          x2={x}
                          y2={presentY}
                          stroke="#83CD2D"
                          strokeWidth="3"
                        />
                      )}
                      <circle
                        cx={x}
                        cy={presentY}
                        r="5"
                        fill="#83CD2D"
                        className="hover:r-7 transition-all duration-300"
                      />

                      {/* Absent line */}
                      {index > 0 && (
                        <line
                          x1={100 + (index - 1) * 100}
                          y1={
                            200 -
                            ((attendanceData[index - 1]?.absent ?? 0) / 200) *
                              160
                          }
                          x2={x}
                          y2={absentY}
                          stroke="#EF4444"
                          strokeWidth="3"
                        />
                      )}
                      <circle
                        cx={x}
                        cy={absentY}
                        r="5"
                        fill="#EF4444"
                        className="hover:r-7 transition-all duration-300"
                      />

                      {/* X-axis label */}
                      <text
                        x={x}
                        y="230"
                        textAnchor="middle"
                        fill="#9CA3AF"
                        fontSize="12"
                      >
                        {new Date(entry.date).getDate()}.
                        {new Date(entry.date).getMonth() + 1}
                      </text>
                    </g>
                  );
                })}

                {/* Y-axis labels */}
                {[0, 50, 100, 150].map((y) => (
                  <text
                    key={y}
                    x="30"
                    y={205 - y * 0.8}
                    textAnchor="end"
                    fill="#9CA3AF"
                    fontSize="12"
                  >
                    {y}
                  </text>
                ))}
              </svg>
            </div>

            {/* Legend with Modern Style */}
            <div className="mt-6 flex justify-center gap-8">
              <div className="flex items-center gap-3">
                <div className="relative">
                  <div className="h-4 w-4 rounded-full bg-gradient-to-br from-[#83CD2D] to-[#73BD1D] shadow-[0_4px_10px_rgba(131,205,45,0.4)]"></div>
                  <div className="absolute inset-0 animate-ping rounded-full bg-[#83CD2D] opacity-30"></div>
                </div>
                <span className="text-sm font-medium text-gray-700">
                  Anwesend
                </span>
              </div>
              <div className="flex items-center gap-3">
                <div className="relative">
                  <div className="h-4 w-4 rounded-full bg-gradient-to-br from-[#EF4444] to-[#DC2626] shadow-[0_4px_10px_rgba(239,68,68,0.4)]"></div>
                  <div className="absolute inset-0 animate-ping rounded-full bg-[#EF4444] opacity-30"></div>
                </div>
                <span className="text-sm font-medium text-gray-700">
                  Abwesend
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Room & Activity Stats - Modern Glassmorphism Cards */}
        <div className="grid grid-cols-1 gap-8 lg:grid-cols-2">
          {/* Room Utilization */}
          <div className="relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md">
            {/* Gradient overlay */}
            <div className="absolute inset-0 bg-gradient-to-br from-[#5080d8]/5 to-blue-100/5 opacity-30"></div>
            {/* Inner glow */}
            <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>

            <div className="relative p-8">
              <h2 className="mb-8 text-xl font-bold text-gray-900">
                Raumauslastung
              </h2>
              <div className="space-y-6">
                {roomUtilization.slice(0, 5).map((room, index) => (
                  <div key={index} className="group">
                    <div className="mb-3 flex items-center justify-between">
                      <span className="text-sm font-semibold text-gray-800 transition-colors duration-300 group-hover:text-[#5080d8]">
                        {room.room}
                      </span>
                      <span className="text-sm font-bold text-gray-700">
                        {room.usagePercent}%
                      </span>
                    </div>
                    <div className="relative h-3 w-full overflow-hidden rounded-full bg-gray-100/50 backdrop-blur-sm">
                      <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/20 to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                      <div
                        className="relative h-3 rounded-full shadow-sm transition-all duration-700 ease-out"
                        style={{
                          width: `${room.usagePercent}%`,
                          background: `linear-gradient(90deg, ${categoryColors[room.category] ?? "#6B7280"} 0%, ${categoryColors[room.category] ?? "#6B7280"}dd 100%)`,
                          boxShadow: `0 2px 8px ${categoryColors[room.category] ?? "#6B7280"}40`,
                        }}
                      >
                        <div className="absolute inset-0 rounded-full bg-white/20 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* Activity Categories */}
          <div className="relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md">
            {/* Gradient overlay */}
            <div className="absolute inset-0 bg-gradient-to-br from-[#83CD2D]/5 to-green-100/5 opacity-30"></div>
            {/* Inner glow */}
            <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>

            <div className="relative p-8">
              <h2 className="mb-8 text-xl font-bold text-gray-900">
                Aktivitäten nach Kategorie
              </h2>
              <div className="space-y-6">
                {Object.entries(
                  activityParticipation.reduce(
                    (acc, activity) => {
                      acc[activity.category] ??= { students: 0, total: 0 };
                      const categoryData = acc[activity.category];
                      if (categoryData) {
                        categoryData.students += activity.students;
                        categoryData.total += activity.totalSlots;
                      }
                      return acc;
                    },
                    {} as Record<string, { students: number; total: number }>,
                  ),
                )
                  .slice(0, 5)
                  .map(([category, data], index) => (
                    <div key={index} className="group">
                      <div className="mb-3 flex items-center justify-between">
                        <span className="text-sm font-semibold text-gray-800 transition-colors duration-300 group-hover:text-[#83CD2D]">
                          {category}
                        </span>
                        <span className="text-sm font-bold text-gray-700">
                          {data.students}/{data.total}
                        </span>
                      </div>
                      <div className="relative h-3 w-full overflow-hidden rounded-full bg-gray-100/50 backdrop-blur-sm">
                        <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/20 to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                        <div
                          className="relative h-3 rounded-full shadow-sm transition-all duration-700 ease-out"
                          style={{
                            width: `${(data.students / data.total) * 100}%`,
                            background: `linear-gradient(90deg, ${categoryColors[category] ?? "#6B7280"} 0%, ${categoryColors[category] ?? "#6B7280"}dd 100%)`,
                            boxShadow: `0 2px 8px ${categoryColors[category] ?? "#6B7280"}40`,
                          }}
                        >
                          <div className="absolute inset-0 rounded-full bg-white/20 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                        </div>
                      </div>
                    </div>
                  ))}
              </div>
            </div>
          </div>
        </div>

        {/* Feedback Summary - Modern Glassmorphism Design */}
        <div className="relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md">
          {/* Gradient overlay */}
          <div className="absolute inset-0 bg-gradient-to-br from-gray-50/50 to-white/30 opacity-50"></div>
          {/* Inner glow */}
          <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>

          <div className="relative p-8">
            <h2 className="mb-8 text-xl font-bold text-gray-900">
              Feedback Übersicht
            </h2>
            <div className="grid grid-cols-1 gap-8 md:grid-cols-3">
              {/* Pie Chart - Modern with Glow */}
              <div className="flex items-center justify-center">
                <div className="group relative">
                  <svg width="200" height="200" viewBox="0 0 200 200">
                    <g transform="translate(100, 100)">
                      {/* Positive */}
                      <path
                        d={`M 0 -80 A 80 80 0 ${(feedbackSummary?.positive ?? 0) > 50 ? 1 : 0} 1 ${
                          Math.cos(
                            ((feedbackSummary?.positive ?? 0) / 100) *
                              2 *
                              Math.PI -
                              Math.PI / 2,
                          ) * 80
                        } ${
                          Math.sin(
                            ((feedbackSummary?.positive ?? 0) / 100) *
                              2 *
                              Math.PI -
                              Math.PI / 2,
                          ) * 80
                        } L 0 0`}
                        fill="#83CD2D"
                        className="transition-opacity duration-300 hover:opacity-90"
                      />
                      {/* Neutral */}
                      <path
                        d={`M ${
                          Math.cos(
                            ((feedbackSummary?.positive ?? 0) / 100) *
                              2 *
                              Math.PI -
                              Math.PI / 2,
                          ) * 80
                        } ${
                          Math.sin(
                            ((feedbackSummary?.positive ?? 0) / 100) *
                              2 *
                              Math.PI -
                              Math.PI / 2,
                          ) * 80
                        } A 80 80 0 ${(feedbackSummary?.neutral ?? 0) > 50 ? 1 : 0} 1 ${
                          Math.cos(
                            (((feedbackSummary?.positive ?? 0) +
                              (feedbackSummary?.neutral ?? 0)) /
                              100) *
                              2 *
                              Math.PI -
                              Math.PI / 2,
                          ) * 80
                        } ${
                          Math.sin(
                            (((feedbackSummary?.positive ?? 0) +
                              (feedbackSummary?.neutral ?? 0)) /
                              100) *
                              2 *
                              Math.PI -
                              Math.PI / 2,
                          ) * 80
                        } L 0 0`}
                        fill="#F78C10"
                        className="transition-opacity duration-300 hover:opacity-90"
                      />
                      {/* Negative */}
                      <path
                        d={`M ${
                          Math.cos(
                            (((feedbackSummary?.positive ?? 0) +
                              (feedbackSummary?.neutral ?? 0)) /
                              100) *
                              2 *
                              Math.PI -
                              Math.PI / 2,
                          ) * 80
                        } ${
                          Math.sin(
                            (((feedbackSummary?.positive ?? 0) +
                              (feedbackSummary?.neutral ?? 0)) /
                              100) *
                              2 *
                              Math.PI -
                              Math.PI / 2,
                          ) * 80
                        } A 80 80 0 ${(feedbackSummary?.negative ?? 0) > 50 ? 1 : 0} 1 0 -80 L 0 0`}
                        fill="#EF4444"
                        className="transition-opacity duration-300 hover:opacity-90"
                      />
                      {/* Center circle with glassmorphism effect */}
                      <circle
                        cx="0"
                        cy="0"
                        r="50"
                        fill="white"
                        fillOpacity="0.9"
                      />
                      <circle
                        cx="0"
                        cy="0"
                        r="50"
                        fill="none"
                        stroke="#E5E7EB"
                        strokeWidth="1"
                      />

                      {/* Center text */}
                      <text
                        y="-5"
                        textAnchor="middle"
                        className="fill-gray-900 text-3xl font-bold"
                      >
                        {feedbackSummary?.total}
                      </text>
                      <text
                        y="15"
                        textAnchor="middle"
                        className="fill-gray-500 text-sm"
                      >
                        Total
                      </text>
                    </g>

                    {/* Animated glow effect */}
                    <defs>
                      <filter id="glow">
                        <feGaussianBlur stdDeviation="4" result="coloredBlur" />
                        <feMerge>
                          <feMergeNode in="coloredBlur" />
                          <feMergeNode in="SourceGraphic" />
                        </feMerge>
                      </filter>
                    </defs>
                  </svg>
                </div>
              </div>

              {/* Stats Grid - Modern Cards */}
              <div className="space-y-6 md:col-span-2">
                <div className="grid grid-cols-3 gap-4">
                  <div className="group relative overflow-hidden rounded-2xl bg-gradient-to-br from-[#83CD2D]/10 to-[#83CD2D]/5 p-6 text-center transition-all duration-300 hover:scale-105 hover:shadow-[0_12px_25px_rgba(131,205,45,0.3)]">
                    <div className="absolute inset-0 bg-gradient-to-br from-[#83CD2D]/0 to-[#83CD2D]/10 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                    <p className="relative mb-2 text-3xl font-bold text-[#83CD2D]">
                      {feedbackSummary?.positive}%
                    </p>
                    <p className="relative text-sm font-medium text-gray-700">
                      Positiv
                    </p>
                    <div className="absolute top-2 right-2 h-2 w-2 animate-pulse rounded-full bg-[#83CD2D]"></div>
                  </div>
                  <div className="group relative overflow-hidden rounded-2xl bg-gradient-to-br from-[#F78C10]/10 to-[#F78C10]/5 p-6 text-center transition-all duration-300 hover:scale-105 hover:shadow-[0_12px_25px_rgba(247,140,16,0.3)]">
                    <div className="absolute inset-0 bg-gradient-to-br from-[#F78C10]/0 to-[#F78C10]/10 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                    <p className="relative mb-2 text-3xl font-bold text-[#F78C10]">
                      {feedbackSummary?.neutral}%
                    </p>
                    <p className="relative text-sm font-medium text-gray-700">
                      Neutral
                    </p>
                    <div className="absolute top-2 right-2 h-2 w-2 animate-pulse rounded-full bg-[#F78C10]"></div>
                  </div>
                  <div className="group relative overflow-hidden rounded-2xl bg-gradient-to-br from-red-50 to-red-50/50 p-6 text-center transition-all duration-300 hover:scale-105 hover:shadow-[0_12px_25px_rgba(239,68,68,0.3)]">
                    <div className="absolute inset-0 bg-gradient-to-br from-red-50/0 to-red-100/50 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                    <p className="relative mb-2 text-3xl font-bold text-red-600">
                      {feedbackSummary?.negative}%
                    </p>
                    <p className="relative text-sm font-medium text-gray-700">
                      Negativ
                    </p>
                    <div className="absolute top-2 right-2 h-2 w-2 animate-pulse rounded-full bg-red-600"></div>
                  </div>
                </div>
                <div className="flex items-center justify-center">
                  <p className="rounded-2xl border border-gray-200/50 bg-gray-50/50 px-6 py-3 text-sm text-gray-600 backdrop-blur-sm">
                    Basierend auf{" "}
                    <span className="font-bold text-gray-900">
                      {feedbackSummary?.total}
                    </span>{" "}
                    Rückmeldungen im ausgewählten Zeitraum
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </ResponsiveLayout>
  );
}
