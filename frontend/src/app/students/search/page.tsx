"use client";

import { useState, useEffect, useCallback, useRef, Suspense } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Input } from "~/components/ui";
import { Alert } from "~/components/ui/alert";
import { studentService, groupService } from "~/lib/api";
import type { Student, Group } from "~/lib/api";


function SearchPageContent() {
  const { data: session, status } = useSession();
  const router = useRouter();
  const searchTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  // Search and filter state
  const [searchTerm, setSearchTerm] = useState("");
  const [selectedGroup, setSelectedGroup] = useState("");
  const [selectedYear, setSelectedYear] = useState("all");
  const [attendanceFilter, setAttendanceFilter] = useState("all");

  // Data state
  const [students, setStudents] = useState<Student[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Mobile-specific state
  const [isMobileFiltersOpen, setIsMobileFiltersOpen] = useState(false);

  const fetchStudentsData = useCallback(async (filters?: {
    search?: string;
    groupId?: string;
  }) => {
    try {
      setIsSearching(true);
      setError(null);

      // Fetch students from API
      const fetchedStudents = await studentService.getStudents({
        search: filters?.search ?? searchTerm,
        groupId: filters?.groupId ?? selectedGroup
      });

      setStudents(fetchedStudents);
    } catch {
      // Error fetching students - handle gracefully
      setError("Fehler beim Laden der Schülerdaten.");
    } finally {
      setIsSearching(false);
    }
  }, [searchTerm, selectedGroup]);

  // Load groups on mount
  useEffect(() => {
    const loadGroups = async () => {
      try {
        const fetchedGroups = await groupService.getGroups();
        setGroups(fetchedGroups);
      } catch (error) {
        console.error("Error loading groups:", error);
      }
    };

    void loadGroups();
  }, []);

  // Load initial students on mount
  useEffect(() => {
    void fetchStudentsData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Debounced search effect
  useEffect(() => {
    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }

    searchTimeoutRef.current = setTimeout(() => {
      if (searchTerm.length >= 2 || searchTerm.length === 0) {
        void fetchStudentsData();
      }
    }, 300);

    return () => {
      if (searchTimeoutRef.current) {
        clearTimeout(searchTimeoutRef.current);
      }
    };
  }, [searchTerm, fetchStudentsData]);

  // Re-fetch when group filter changes
  useEffect(() => {
    void fetchStudentsData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedGroup]);

  const handleSearch = () => {
    const filters = {
      search: searchTerm,
      groupId: selectedGroup,
    };

    void fetchStudentsData(filters);
  };

  const handleFilterReset = () => {
    setSearchTerm("");
    setSelectedGroup("");
    setSelectedYear("all");
    setAttendanceFilter("all");
    setIsMobileFiltersOpen(false);
    
    // Clear timeout to prevent pending searches
    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }
    void fetchStudentsData();
  };

  // Apply additional client-side filtering for attendance statuses and year
  const filteredStudents = students.filter((student) => {
    // Apply attendance filter
    if (attendanceFilter === "all") {
      // No attendance filtering
    } else if (attendanceFilter === "in_house" && !student.in_house) {
      return false;
    } else if (attendanceFilter === "wc" && student.wc !== true) {
      return false;
    } else if (attendanceFilter === "school_yard" && student.school_yard !== true) {
      return false;
    } else if (attendanceFilter === "bus" && student.bus !== true) {
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

  // Helper function to get location status
  const getLocationStatus = (student: Student) => {
    if (student.in_house === true) return { label: "Im Haus", color: "bg-green-500 text-green-50" };
    if (student.wc === true) return { label: "Toilette", color: "bg-blue-500 text-blue-50" };
    if (student.school_yard === true) return { label: "Schulhof", color: "bg-yellow-500 text-yellow-50" };
    // Student is at home when current_location is "Home" or all location flags are false
    if (student.current_location === "Home" || (!student.in_house && !student.wc && !student.school_yard)) {
      return { label: "Zuhause", color: "bg-orange-500 text-orange-50" };
    }
    if (student.current_location === "Bus") return { label: "Unterwegs", color: "bg-purple-500 text-purple-50" };
    return { label: "Unbekannt", color: "bg-gray-500 text-gray-50" };
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

  return (
    <ResponsiveLayout userName={session?.user?.name ?? "Root"}>
      <div className="max-w-7xl mx-auto">
        {/* Mobile-optimized Header */}
        <div className="mb-4 md:mb-8">
          <h1 className="text-2xl md:text-4xl font-bold text-gray-900">Schülersuche</h1>
          <p className="mt-1 text-sm md:text-base text-gray-600">Finde Schüler nach Namen, Gruppe oder Status</p>
        </div>

        {/* Mobile Search Bar - Always Visible */}
        <div className="mb-4 md:hidden">
          <Input
            label="Schnellsuche"
            name="searchTerm"
            placeholder="Schüler suchen..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="text-base" // Prevent iOS zoom
          />
        </div>

        {/* Mobile Filter Toggle */}
        <div className="mb-4 md:hidden">
          <button
            onClick={() => setIsMobileFiltersOpen(!isMobileFiltersOpen)}
            className="flex w-full items-center justify-between rounded-lg bg-white px-4 py-3 shadow-sm ring-1 ring-gray-200 hover:ring-gray-300 focus:ring-2 focus:ring-blue-500 focus:outline-none"
          >
            <span className="text-sm font-medium text-gray-700">
              Filter & Erweiterte Suche
            </span>
            <svg 
              className={`h-5 w-5 text-gray-400 transition-transform ${isMobileFiltersOpen ? 'rotate-180' : ''}`} 
              fill="none" 
              viewBox="0 0 24 24" 
              stroke="currentColor"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>
        </div>

        {/* Search Panel - Desktop always visible, Mobile collapsible */}
        <div className={`mb-6 overflow-hidden rounded-xl bg-white shadow-md transition-all duration-300 ${
          isMobileFiltersOpen ? 'block' : 'hidden md:block'
        }`}>
          <div className="p-4 md:p-6">
            <h2 className="mb-4 text-lg md:text-xl font-bold text-gray-800">Suchkriterien</h2>

            <div className="grid grid-cols-1 gap-4 md:gap-6 md:grid-cols-2 lg:grid-cols-4">
              {/* Name Search - Desktop only (mobile has quick search above) */}
              <div className="hidden md:block">
                <Input
                  label="Name"
                  name="searchTerm"
                  placeholder="Vor- oder Nachname"
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="h-12 text-base"
                />
              </div>

              {/* Group Filter */}
              <div className="relative">
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Gruppe
                </label>
                <select
                  value={selectedGroup}
                  onChange={(e) => setSelectedGroup(e.target.value)}
                  className="mt-1 block w-full rounded-lg border-0 px-4 py-3 h-12 text-base shadow-sm ring-1 ring-gray-200 transition-all duration-200 hover:bg-gray-50/50 hover:ring-gray-300 focus:ring-2 focus:ring-blue-500 focus:outline-none appearance-none pr-8"
                >
                  <option value="">Alle Gruppen</option>
                  {groups.map(group => (
                    <option key={group.id} value={group.id}>{group.name}</option>
                  ))}
                </select>
                <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                  <svg className="h-5 w-5 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                  </svg>
                </div>
              </div>

              {/* School Year Filter */}
              <div className="relative">
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Jahrgangsstufe
                </label>
                <select
                  value={selectedYear}
                  onChange={(e) => setSelectedYear(e.target.value)}
                  className="mt-1 block w-full rounded-lg border-0 px-4 py-3 h-12 text-base shadow-sm ring-1 ring-gray-200 transition-all duration-200 hover:bg-gray-50/50 hover:ring-gray-300 focus:ring-2 focus:ring-blue-500 focus:outline-none appearance-none pr-8"
                >
                  <option value="all">Alle Jahrgänge</option>
                  <option value="1">Jahrgang 1</option>
                  <option value="2">Jahrgang 2</option>
                  <option value="3">Jahrgang 3</option>
                  <option value="4">Jahrgang 4</option>
                </select>
                <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                  <svg className="h-5 w-5 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                  </svg>
                </div>
              </div>

              {/* Attendance Status */}
              <div className="relative">
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Anwesenheitsstatus
                </label>
                <select
                  value={attendanceFilter}
                  onChange={(e) => setAttendanceFilter(e.target.value)}
                  className="mt-1 block w-full rounded-lg border-0 px-4 py-3 h-12 text-base shadow-sm ring-1 ring-gray-200 transition-all duration-200 hover:bg-gray-50/50 hover:ring-gray-300 focus:ring-2 focus:ring-blue-500 focus:outline-none appearance-none pr-8"
                >
                  <option value="all">Alle</option>
                  <option value="in_house">In Räumen</option>
                  <option value="wc">Toilette</option>
                  <option value="school_yard">Schulhof</option>
                  <option value="bus">Zuhause</option>
                </select>
                <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                  <svg className="h-5 w-5 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                  </svg>
                </div>
              </div>
            </div>

            {/* Search Actions */}
            <div className="mt-6 flex flex-col sm:flex-row gap-3 sm:justify-end">
              <button
                onClick={handleFilterReset}
                className="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 order-2 sm:order-1"
              >
                Zurücksetzen
              </button>
              <button
                onClick={handleSearch}
                disabled={isSearching}
                className="rounded-lg bg-gradient-to-r from-blue-500 to-blue-600 px-6 py-2 text-sm font-medium text-white shadow-sm transition-all hover:from-blue-600 hover:to-blue-700 hover:shadow-md disabled:opacity-70 order-1 sm:order-2"
              >
                {isSearching ? "Suche läuft..." : "Suchen"}
              </button>
            </div>
          </div>
        </div>

        {/* Results Section */}
        <div className="rounded-xl bg-white shadow-md overflow-hidden">
          <div className="p-4 md:p-6">
            {/* Results Header - Mobile Optimized */}
            <div className="flex flex-col md:flex-row md:justify-between md:items-center mb-4 md:mb-6 gap-4">
              <div>
                <h2 className="text-lg md:text-xl font-bold text-gray-800">
                  Suchergebnisse
                </h2>
                <p className="text-sm text-gray-600 mt-1">
                  {filteredStudents.length} {filteredStudents.length === 1 ? 'Schüler gefunden' : 'Schüler gefunden'}
                </p>
              </div>
              
              {/* Year Legend - Hidden on mobile, shown on tablet+ */}
              <div className="hidden md:flex items-center space-x-4">
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

            {error && (
              <div className="mb-6">
                <Alert type="error" message={error} />
              </div>
            )}

            {isSearching ? (
              <div className="py-12 text-center">
                <div className="flex flex-col items-center gap-4">
                  <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
                  <p className="text-gray-600">Suche läuft...</p>
                </div>
              </div>
            ) : filteredStudents.length === 0 ? (
              <div className="py-12 text-center">
                <div className="flex flex-col items-center gap-4">
                  <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                  </svg>
                  <div>
                    <h3 className="text-lg font-medium text-gray-900">Keine Schüler gefunden</h3>
                    <p className="text-gray-600">Versuche deine Suchkriterien anzupassen.</p>
                  </div>
                </div>
              </div>
            ) : (
              /* Student Grid - Mobile Optimized */
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {filteredStudents.map((student) => {
                  const year = getSchoolYear(student.school_class);
                  const yearColor = getYearColor(year);
                  const locationStatus = getLocationStatus(student);

                  return (
                    <div
                      key={student.id}
                      onClick={() => router.push(`/students/${student.id}?from=/students/search`)}
                      className="group cursor-pointer rounded-lg border border-gray-200 bg-white p-4 shadow-sm transition-all duration-200 hover:border-blue-300 hover:shadow-md active:scale-[0.98]"
                    >
                      <div className="flex items-start justify-between mb-3">
                        <div className="flex-1 min-w-0">
                          <h3 className="font-semibold text-gray-900 truncate group-hover:text-blue-600 transition-colors">
                            {student.first_name} {student.second_name}
                          </h3>
                          <div className="flex items-center mt-1 gap-2">
                            <span className="text-sm text-gray-500">
                              Klasse {student.school_class}
                            </span>
                            <span className={`inline-block h-2 w-2 rounded-full ${yearColor}`} />
                          </div>
                        </div>
                        <svg className="h-5 w-5 text-gray-400 group-hover:text-blue-500 transition-colors flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                        </svg>
                      </div>

                      <div className="space-y-2">
                        {student.group_name && (
                          <div className="flex items-center text-sm text-gray-600">
                            <svg className="h-4 w-4 mr-2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                            </svg>
                            Gruppe: {student.group_name}
                          </div>
                        )}

                        <div className="flex items-center justify-between">
                          <span className="text-sm text-gray-500">Status:</span>
                          <span className={`inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ${locationStatus.color}`}>
                            {locationStatus.label}
                          </span>
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </div>
    </ResponsiveLayout>
  );
}

// Main component with Suspense wrapper
export default function StudentSearchPage() {
  return (
    <Suspense fallback={
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
      </div>
    }>
      <SearchPageContent />
    </Suspense>
  );
}