"use client";

import { useState, useEffect, useCallback } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Input } from "~/components/ui";
import { Alert } from "~/components/ui/alert";

// Student type based on what's seen in the code
interface Student {
  id: string;
  first_name: string;
  second_name: string;
  name?: string;
  school_class: string;
  group_id: string;
  group_name?: string;
  in_house: boolean;
  wc: boolean;
  school_yard: boolean;
  bus: boolean;
}

// Demo data for students - defined outside component to avoid dependency issues
const exampleStudents: Student[] = [
  {
    id: "1",
    first_name: "Emma",
    second_name: "Müller",
    school_class: "1a",
    group_id: "g1",
    group_name: "Bären",
    in_house: true,
    wc: false,
    school_yard: false,
    bus: false
  },
  {
    id: "2",
    first_name: "Max",
    second_name: "Schmidt",
    school_class: "1b",
    group_id: "g1",
    group_name: "Bären",
    in_house: false,
    wc: true,
    school_yard: false,
    bus: false
  },
  {
    id: "3",
    first_name: "Sophie",
    second_name: "Wagner",
    school_class: "2a",
    group_id: "g2",
    group_name: "Füchse",
    in_house: true,
    wc: false,
    school_yard: false,
    bus: false
  },
  {
    id: "4",
    first_name: "Leon",
    second_name: "Fischer",
    school_class: "2b",
    group_id: "g2",
    group_name: "Füchse",
    in_house: true,
    wc: false,
    school_yard: false,
    bus: false
  },
  {
    id: "5",
    first_name: "Mia",
    second_name: "Weber",
    school_class: "3a",
    group_id: "g3",
    group_name: "Eulen",
    in_house: true,
    wc: false,
    school_yard: false,
    bus: false
  },
  {
    id: "6",
    first_name: "Noah",
    second_name: "Becker",
    school_class: "3b",
    group_id: "g3",
    group_name: "Eulen",
    in_house: false,
    wc: false,
    school_yard: true,
    bus: false
  },
  {
    id: "7",
    first_name: "Lina",
    second_name: "Schulz",
    school_class: "4a",
    group_id: "g4",
    group_name: "Wölfe",
    in_house: false,
    wc: false,
    school_yard: false,
    bus: true
  },
  {
    id: "8",
    first_name: "Felix",
    second_name: "Hoffmann",
    school_class: "4b",
    group_id: "g4",
    group_name: "Wölfe",
    in_house: true,
    wc: false,
    school_yard: false,
    bus: false
  }
];

export default function StudentSearchPage() {
  const router = useRouter();
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });

  // Mock Groups für die Gruppenauswahl
  const mockGroups = [
    { id: "g1", name: "Bären" },
    { id: "g2", name: "Füchse" },
    { id: "g3", name: "Eulen" },
    { id: "g4", name: "Wölfe" }
  ];

  // Search state
  const [searchTerm, setSearchTerm] = useState("");
  const [selectedGroup, setSelectedGroup] = useState("");
  const [selectedYear, setSelectedYear] = useState<string>("all");
  const [attendanceFilter, setAttendanceFilter] = useState<string>("all");
  const [isSearching, setIsSearching] = useState(false);
  const [students, setStudents] = useState<Student[]>([]);
  const [error, setError] = useState<string | null>(null);

  // Using useCallback to memoize the fetchStudents function
  const fetchStudents = useCallback(async (filters?: {
    search?: string;
    inHouse?: boolean;
    groupId?: string;
  }) => {
    setIsSearching(true);
    setError(null);

    try {
      // Simuliere eine API-Anfrage mit den Beispieldaten
      setTimeout(() => {
        let filteredExampleStudents = [...exampleStudents];

        // Filtern nach Suchbegriff
        if (filters?.search) {
          const searchLower = filters.search.toLowerCase();
          filteredExampleStudents = filteredExampleStudents.filter(student =>
              student.first_name.toLowerCase().includes(searchLower) ||
              student.second_name.toLowerCase().includes(searchLower)
          );
        }

        // Filtern nach Anwesenheit
        if (filters?.inHouse !== undefined) {
          filteredExampleStudents = filteredExampleStudents.filter(student =>
              student.in_house === filters.inHouse
          );
        }

        // Filtern nach Gruppe
        if (filters?.groupId) {
          filteredExampleStudents = filteredExampleStudents.filter(student =>
              student.group_id === filters.groupId
          );
        }

        setStudents(filteredExampleStudents);
        setIsSearching(false);
      }, 500); // Kleine Verzögerung für realistischeres Verhalten
    } catch (err) {
      console.error("Error fetching students:", err);
      setError("Fehler beim Laden der Schülerdaten.");
      setStudents([]);
      setIsSearching(false);
    }
  }, []);  // Empty dependency array as it doesn't depend on any state or props

  // Load initial data - now includes fetchStudents in the dependency array
  useEffect(() => {
    void fetchStudents();
  }, [fetchStudents]);

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
    setSelectedYear("all");
    setAttendanceFilter("all");
    void fetchStudents();
  };

  // Apply additional client-side filtering for attendance statuses and year
  const filteredStudents = students.filter((student) => {
    // Apply attendance filter
    if (attendanceFilter === "all") {
      // No attendance filtering
    } else if (attendanceFilter === "in_house" && !student.in_house) {
      return false;
    } else if (attendanceFilter === "wc" && !student.wc) {
      return false;
    } else if (attendanceFilter === "school_yard" && !student.school_yard) {
      return false;
    } else if (attendanceFilter === "bus" && !student.bus) {
      return false;
    }

    // Apply year filter - extract year from school_class (e.g., "1a" → year 1)
    if (selectedYear !== "all") {
      const yearMatch = /^(\d)/.exec(student.school_class);
      const studentYear = yearMatch ? yearMatch[1] : null;
      if (studentYear !== selectedYear) {
        return false;
      }
    }

    return true;
  });

  // Helper function to determine the school year
  const getSchoolYear = (schoolClass: string): number => {
    const yearMatch = /^(\d)/.exec(schoolClass);
    return yearMatch?.[1] ? parseInt(yearMatch[1], 10) : 0;
  };

  // Determine color for year dot
  const getYearColor = (year: number): string => {
    switch (year) {
      case 1: return "bg-blue-500";
      case 2: return "bg-green-500";
      case 3: return "bg-yellow-500";
      case 4: return "bg-purple-500";
      default: return "bg-gray-400";
    }
  };

  if (status === "loading") {
    return (
        <div className="flex min-h-screen items-center justify-center">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
    );
  }

  // Common class for all dropdowns to ensure consistent height
  const dropdownClass = "mt-1 block w-full rounded-lg border-0 px-4 py-3 h-12 shadow-sm ring-1 ring-gray-200 transition-all duration-200 hover:bg-gray-50/50 hover:ring-gray-300 focus:ring-2 focus:ring-teal-500 focus:outline-none appearance-none pr-8";

  return (
      <ResponsiveLayout userName={session?.user?.name ?? "Root"}>
        <div className="max-w-7xl mx-auto">
          <h1 className="mb-8 text-4xl font-bold text-gray-900">Schülersuche</h1>

                {/* Search Panel */}
                <div className="mb-8 overflow-hidden rounded-xl bg-white p-6 shadow-md">
                  <h2 className="mb-4 text-xl font-bold text-gray-800">Suchkriterien</h2>

                  <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-4">
                    {/* Name Search */}
                    <Input
                        label="Name"
                        name="searchTerm"
                        placeholder="Vor- oder Nachname"
                        value={searchTerm}
                        onChange={(e) => setSearchTerm(e.target.value)}
                        className="h-12" // Add fixed height to the Input component
                    />

                    {/* Group Filter */}
                    <div className="relative">
                      <label className="block text-sm font-medium text-gray-700">
                        Gruppe
                      </label>
                      <select
                          value={selectedGroup}
                          onChange={(e) => setSelectedGroup(e.target.value)}
                          className={dropdownClass}
                      >
                        <option value="">Alle Gruppen</option>
                        {mockGroups.map(group => (
                            <option key={group.id} value={group.id}>{group.name}</option>
                        ))}
                      </select>
                      <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                        <svg className="h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                          <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                        </svg>
                      </div>
                    </div>

                    {/* School Year Filter */}
                    <div className="relative">
                      <label className="block text-sm font-medium text-gray-700">
                        Jahrgangsstufe
                      </label>
                      <select
                          value={selectedYear}
                          onChange={(e) => setSelectedYear(e.target.value)}
                          className={dropdownClass}
                      >
                        <option value="all">Alle Jahrgänge</option>
                        <option value="1">Jahrgang 1</option>
                        <option value="2">Jahrgang 2</option>
                        <option value="3">Jahrgang 3</option>
                        <option value="4">Jahrgang 4</option>
                      </select>
                      <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                        <svg className="h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                          <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                        </svg>
                      </div>
                    </div>

                    {/* Attendance Status */}
                    <div className="relative">
                      <label className="block text-sm font-medium text-gray-700">
                        Anwesenheitsstatus
                      </label>
                      <select
                          value={attendanceFilter}
                          onChange={(e) => setAttendanceFilter(e.target.value)}
                          className={dropdownClass}
                      >
                        <option value="all">Alle</option>
                        <option value="in_house">In Räumen</option>
                        <option value="wc">Toilette</option>
                        <option value="school_yard">Schulhof</option>
                        <option value="bus">Zuhause</option>
                      </select>
                      <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                        <svg className="h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                          <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                        </svg>
                      </div>
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
                  <div className="flex justify-between items-center mb-6">
                    <h2 className="text-xl font-bold text-gray-800">Suchergebnisse</h2>
                    <div className="flex items-center space-x-6">
                      {/* Year legend */}
                      <div className="flex items-center space-x-4">
                        <div className="flex items-center">
                          <span className="inline-block h-3 w-3 rounded-full bg-blue-500 mr-1"></span>
                          <span className="text-xs text-gray-600">Jahr 1</span>
                        </div>
                        <div className="flex items-center">
                          <span className="inline-block h-3 w-3 rounded-full bg-green-500 mr-1"></span>
                          <span className="text-xs text-gray-600">Jahr 2</span>
                        </div>
                        <div className="flex items-center">
                          <span className="inline-block h-3 w-3 rounded-full bg-yellow-500 mr-1"></span>
                          <span className="text-xs text-gray-600">Jahr 3</span>
                        </div>
                        <div className="flex items-center">
                          <span className="inline-block h-3 w-3 rounded-full bg-purple-500 mr-1"></span>
                          <span className="text-xs text-gray-600">Jahr 4</span>
                        </div>
                      </div>
                    </div>
                  </div>

                  {error && (
                      <div className="mb-6">
                        <Alert type="error" message={error} />
                      </div>
                  )}

                  {isSearching ? (
                      <div className="py-8 text-center">
                        <div className="flex flex-col items-center gap-4">
                          <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
                          <p className="text-gray-500">Suche läuft...</p>
                        </div>
                      </div>
                  ) : (
                      <div className="space-y-2">
                        {filteredStudents.length > 0 ? (
                            filteredStudents.map((student) => {
                              const year = getSchoolYear(student.school_class);
                              const yearColor = getYearColor(year);

                              return (
                                  <div
                                      key={student.id}
                                      onClick={() => router.push(`/students/${student.id}`)}
                                      className="group cursor-pointer rounded-lg border border-gray-100 bg-white p-4 shadow-sm transition-all duration-200 hover:translate-y-[-1px] hover:border-blue-200 hover:shadow-md"
                                  >
                                    <div className="flex items-center justify-between">
                                      <div className="flex items-center space-x-3">
                                        <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-r from-blue-400 to-indigo-500 font-medium text-white">
                                          {(student.first_name?.charAt(0) || "S").toUpperCase()}
                                        </div>

                                        <div className="flex flex-col">
                                          <div className="flex items-center">
                                          <span className="font-medium text-gray-900 transition-colors group-hover:text-blue-600">
                                            {student.first_name} {student.second_name}
                                          </span>
                                            {/* Year indicator */}
                                            <span className={`ml-2 inline-block h-3 w-3 rounded-full ${yearColor}`} title={`Jahrgang ${year}`}></span>
                                          </div>
                                          <span className="text-sm text-gray-500">
                                          Klasse: {student.school_class}
                                            {student.group_name && ` | Gruppe: ${student.group_name}`}
                                        </span>
                                        </div>
                                      </div>

                                      <div className="flex items-center space-x-4">
                                        {/* Attendance indicator */}
                                        <div className="relative flex items-center">
                                          <div
                                              className={`h-4 w-4 rounded-full ${student.in_house ? "bg-green-500" : "bg-red-500"}`}
                                              title={student.in_house ? "Anwesend" : "Abwesend"}
                                          >
                                            {student.in_house && (
                                                <span className="absolute -top-0.5 -left-0.5 flex h-5 w-5">
                                                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-400 opacity-75"></span>
                                              </span>
                                            )}
                                          </div>
                                        </div>

                                        <svg
                                            xmlns="http://www.w3.org/2000/svg"
                                            className="h-5 w-5 text-gray-400 transition-all duration-200 group-hover:translate-x-1 group-hover:text-blue-500"
                                            fill="none"
                                            viewBox="0 0 24 24"
                                            stroke="currentColor"
                                        >
                                          <path
                                              strokeLinecap="round"
                                              strokeLinejoin="round"
                                              strokeWidth={2}
                                              d="M9 5l7 7-7 7"
                                          />
                                        </svg>
                                      </div>
                                    </div>
                                  </div>
                              );
                            })
                        ) : (
                            <div className="py-8 text-center">
                              <p className="text-gray-500">Keine Schüler gefunden. Bitte passen Sie Ihre Suchkriterien an.</p>
                            </div>
                        )}
                      </div>
                  )}
                </div>
        </div>
      </ResponsiveLayout>
  );
}