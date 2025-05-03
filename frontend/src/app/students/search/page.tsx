"use client";

import { useState, useEffect } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { StudentList } from "@/components/students";
import { Input } from "@/components/ui";
import { studentService } from "@/lib/api";
import type { Student } from "@/lib/api";
import { GroupSelector } from "@/components/groups";

export default function StudentSearchPage() {
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/login");
    },
  });

  // Search state
  const [searchTerm, setSearchTerm] = useState("");
  const [selectedGroup, setSelectedGroup] = useState("");
  const [attendanceFilter, setAttendanceFilter] = useState<string>("all");
  const [isSearching, setIsSearching] = useState(false);
  const [students, setStudents] = useState<Student[]>([]);
  const [error, setError] = useState<string | null>(null);

  // Load initial data
  useEffect(() => {
    void fetchStudents();
  }, []);

  async function fetchStudents(filters?: {
    search?: string;
    inHouse?: boolean;
    groupId?: string;
  }) {
    setIsSearching(true);
    setError(null);

    try {
      const fetchedStudents = await studentService.getStudents(filters);
      setStudents(fetchedStudents);
    } catch (err) {
      console.error("Error fetching students:", err);
      setError("Fehler beim Laden der Schülerdaten.");
      setStudents([]);
    } finally {
      setIsSearching(false);
    }
  }

  const handleSearch = async () => {
    const filters: {
      search?: string;
      inHouse?: boolean;
      groupId?: string;
    } = {};

    if (searchTerm.trim()) {
      filters.search = searchTerm.trim();
    }

    if (selectedGroup) {
      filters.groupId = selectedGroup;
    }

    if (attendanceFilter === "in_house") {
      filters.inHouse = true;
    }

    void fetchStudents(filters);
  };

  const handleFilterReset = () => {
    setSearchTerm("");
    setSelectedGroup("");
    setAttendanceFilter("all");
    void fetchStudents();
  };

  // Apply additional client-side filtering for attendance statuses that the API doesn't support
  const filteredStudents = students.filter((student) => {
    if (attendanceFilter === "all") return true;
    if (attendanceFilter === "in_house") return student.in_house === true;
    if (attendanceFilter === "wc") return student.wc === true;
    if (attendanceFilter === "school_yard") return student.school_yard === true;
    if (attendanceFilter === "bus") return student.bus === true;
    return true;
  });

  if (status === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 pb-12">
      {/* Header */}
      <div className="bg-gradient-to-r from-teal-500 to-blue-600 py-6 text-white">
        <div className="container mx-auto px-4">
          <div className="flex flex-col items-start justify-between gap-4 md:flex-row md:items-center">
            <div>
              <h1 className="text-3xl font-bold">Suche Kind</h1>
              <p className="mt-1 text-teal-100">
                Informationen zu Kindern finden
              </p>
            </div>
            <button
              onClick={() => { void redirect("/dashboard"); }}
              className="rounded-lg bg-white/20 px-4 py-2 text-sm font-medium text-white backdrop-blur-sm transition-colors hover:bg-white/30"
            >
              Zurück zum Dashboard
            </button>
          </div>
        </div>
      </div>

      {/* Main Content */}
      <div className="container mx-auto px-4 py-8">
        {/* Search Panel */}
        <div className="mb-8 overflow-hidden rounded-xl bg-white p-6 shadow-md">
          <h2 className="mb-4 text-xl font-bold text-gray-800">Suchkriterien</h2>

          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
            {/* Name Search */}
            <Input
              label="Name"
              name="searchTerm"
              placeholder="Vor- oder Nachname"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
            />

            {/* Group Filter */}
            <div>
              <GroupSelector
                label="Gruppe"
                value={selectedGroup}
                onChange={setSelectedGroup}
                includeEmpty
              />
            </div>

            {/* Attendance Status */}
            <div>
              <label className="block text-sm font-medium text-gray-700">
                Anwesenheitsstatus
              </label>
              <select
                value={attendanceFilter}
                onChange={(e) => setAttendanceFilter(e.target.value)}
                className="mt-1 block w-full rounded-lg border-0 px-4 py-3 shadow-sm ring-1 ring-gray-200 transition-all duration-200 hover:bg-gray-50/50 hover:ring-gray-300 focus:ring-2 focus:ring-teal-500 focus:outline-none"
              >
                <option value="all">Alle</option>
                <option value="in_house">Im Haus</option>
                <option value="wc">Toilette</option>
                <option value="school_yard">Schulhof</option>
                <option value="bus">Bus</option>
              </select>
            </div>
          </div>

          {/* Search Actions */}
          <div className="mt-6 flex flex-wrap justify-end gap-3">
            <button
              onClick={handleFilterReset}
              className="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50"
            >
              Zurücksetzen
            </button>
            <button
              onClick={handleSearch}
              disabled={isSearching}
              className="rounded-lg bg-gradient-to-r from-teal-500 to-blue-600 px-6 py-2 text-sm font-medium text-white shadow-sm transition-all hover:from-teal-600 hover:to-blue-700 hover:shadow-md disabled:opacity-70"
            >
              {isSearching ? "Suche läuft..." : "Suchen"}
            </button>
          </div>
        </div>

        {/* Results Section */}
        <div className="overflow-hidden rounded-xl bg-white p-6 shadow-md">
          <h2 className="mb-6 text-xl font-bold text-gray-800">Suchergebnisse</h2>

          {error && (
            <div className="mb-6 rounded-lg bg-red-50 p-4 text-red-800">
              <p>{error}</p>
            </div>
          )}

          {isSearching ? (
            <div className="py-8 text-center">
              <p className="text-gray-500">Suche läuft...</p>
            </div>
          ) : (
            <StudentList
              students={filteredStudents}
              emptyMessage="Keine Schüler gefunden. Bitte passen Sie Ihre Suchkriterien an."
            />
          )}
        </div>
      </div>
    </div>
  );
}