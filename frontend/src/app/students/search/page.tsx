"use client";

import { useState, useEffect, useCallback, useRef, Suspense } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { studentService, groupService } from "~/lib/api";
import type { Student, Group } from "~/lib/api";

function SearchPageContent() {
  const { status } = useSession();
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

      setStudents(fetchedStudents.students);
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

  // Apply additional client-side filtering for attendance statuses and year
  const filteredStudents = students.filter((student) => {
    // Apply attendance filter
    if (attendanceFilter === "all") {
      // No attendance filtering
    } else if (attendanceFilter === "anwesend" && student.current_location !== "Anwesend") {
      return false;
    } else if (attendanceFilter === "abwesend" && student.current_location !== "Abwesend") {
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

  // Helper function to get attendance status with enhanced design
  const getLocationStatus = (student: Student) => {
    if (student.current_location === "Anwesend") {
      return { 
        label: "Anwesend", 
        badgeColor: "text-white backdrop-blur-sm",
        cardGradient: "from-emerald-50/80 to-green-100/80",
        glowColor: "ring-emerald-200/50 shadow-emerald-100/50",
        customBgColor: "#83CD2D",
        customShadow: "0 8px 25px rgba(131, 205, 45, 0.4)"
      };
    }
    if (student.current_location === "Abwesend") {
      return { 
        label: "Abwesend", 
        badgeColor: "text-white backdrop-blur-sm",
        cardGradient: "from-amber-50/80 to-yellow-100/80",
        glowColor: "ring-amber-200/50 shadow-amber-100/50",
        customBgColor: "#F78C10",
        customShadow: "0 8px 25px rgba(247, 140, 16, 0.4)"
      };
    }
    return { 
      label: "Unbekannt", 
      badgeColor: "text-white backdrop-blur-sm",
      cardGradient: "from-gray-50/80 to-slate-100/80",
      glowColor: "ring-gray-200/50 shadow-gray-100/50",
      customBgColor: "#6B7280",
      customShadow: "0 8px 25px rgba(107, 114, 128, 0.4)"
    };
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
    <ResponsiveLayout>
      <div className="max-w-7xl mx-auto">
        {/* Simple Header */}
        <div className="mb-6 md:mb-8">
          <h1 className="text-3xl md:text-4xl font-bold text-gray-900">Schülersuche</h1>
        </div>

        {/* Mobile Search - Minimalistic Design */}
        <div className="mb-4 md:hidden">
          {/* Search Input */}
          <div className="relative mb-3">
            <svg className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <input
              type="text"
              placeholder="Schüler suchen..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full pl-10 pr-4 py-3 bg-white border border-gray-200 rounded-lg text-gray-900 placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-colors text-base"
            />
          </div>

          {/* Filter Toggle Button */}
          <div className="flex justify-between items-center mb-3">
            <button
              onClick={() => setIsMobileFiltersOpen(!isMobileFiltersOpen)}
              className="flex items-center gap-2 text-base text-blue-600 font-medium py-2"
            >
              <svg className={`h-5 w-5 transition-transform duration-200 ${isMobileFiltersOpen ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
              </svg>
              {isMobileFiltersOpen ? 'Filter ausblenden' : 'Filter anzeigen'}
            </button>
            {(selectedGroup || selectedYear !== "all" || attendanceFilter !== "all") && (
              <button
                onClick={() => {
                  setSelectedGroup("");
                  setSelectedYear("all");
                  setAttendanceFilter("all");
                }}
                className="text-sm text-gray-500"
              >
                Alle löschen
              </button>
            )}
          </div>

          {/* Collapsible Filter Dropdowns */}
          {isMobileFiltersOpen && (
            <div className="grid grid-cols-2 gap-3 mb-3">
              <div className="relative">
                <select
                  value={selectedGroup}
                  onChange={(e) => setSelectedGroup(e.target.value)}
                  className="w-full pl-3 pr-8 py-2.5 bg-white border border-gray-200 rounded-lg text-gray-900 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 text-base appearance-none"
                >
                  <option value="">Alle Gruppen</option>
                  {groups.map(group => (
                    <option key={group.id} value={group.id}>{group.name}</option>
                  ))}
                </select>
                <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2">
                  <svg className="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                  </svg>
                </div>
              </div>

              <div className="relative">
                <select
                  value={selectedYear}
                  onChange={(e) => setSelectedYear(e.target.value)}
                  className="w-full pl-3 pr-8 py-2.5 bg-white border border-gray-200 rounded-lg text-gray-900 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 text-base appearance-none"
                >
                  <option value="all">Alle Jahre</option>
                  <option value="1">Jahr 1</option>
                  <option value="2">Jahr 2</option>
                  <option value="3">Jahr 3</option>
                  <option value="4">Jahr 4</option>
                </select>
                <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2">
                  <svg className="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                  </svg>
                </div>
              </div>

              <div className="relative col-span-2">
                <select
                  value={attendanceFilter}
                  onChange={(e) => setAttendanceFilter(e.target.value)}
                  className="w-full pl-3 pr-8 py-2.5 bg-white border border-gray-200 rounded-lg text-gray-900 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 text-base appearance-none"
                >
                  <option value="all">Alle Status</option>
                  <option value="anwesend">Anwesend</option>
                  <option value="abwesend">Abwesend</option>
                </select>
                <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2">
                  <svg className="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                  </svg>
                </div>
              </div>
            </div>
          )}

          {/* Active Filters Bar */}
          {(searchTerm || selectedGroup || selectedYear !== "all" || attendanceFilter !== "all") && (
            <div className="flex items-center justify-between text-sm text-gray-600">
              <span>
                {(() => {
                  const activeFilters = [];
                  if (searchTerm) activeFilters.push(`Suche: "${searchTerm}"`);
                  if (selectedGroup) {
                    const groupName = groups.find(g => g.id === selectedGroup)?.name;
                    activeFilters.push(`Gruppe: ${groupName ?? selectedGroup}`);
                  }
                  if (selectedYear !== "all") activeFilters.push(`Jahr ${selectedYear}`);
                  if (attendanceFilter !== "all") {
                    const statusLabels: Record<string, string> = {
                      "anwesend": "Anwesend",
                      "abwesend": "Abwesend"
                    };
                    activeFilters.push(statusLabels[attendanceFilter] ?? attendanceFilter);
                  }
                  
                  if (activeFilters.length === 1) {
                    return `1 Filter aktiv: ${activeFilters[0]}`;
                  } else {
                    return `${activeFilters.length} Filter aktiv: ${activeFilters.join(", ")}`;
                  }
                })()}
              </span>
              <button
                onClick={() => {
                  setSearchTerm("");
                  setSelectedGroup("");
                  setSelectedYear("all");
                  setAttendanceFilter("all");
                }}
                className="text-blue-600 hover:text-blue-700 font-medium"
              >
                Zurücksetzen
              </button>
            </div>
          )}
        </div>

        {/* Desktop Search & Filter - Minimalistic */}
        <div className="hidden md:block mb-4">
          <div className="flex items-center gap-3">
            {/* Search Input */}
            <div className="flex-1">
              <div className="relative">
                <svg className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
                <input
                  type="text"
                  placeholder="Schüler suchen..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="w-full pl-10 pr-4 py-2 bg-white border border-gray-200 rounded-lg text-gray-900 placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-colors"
                />
              </div>
            </div>

            {/* Filter Dropdowns */}
            <div className="relative">
              <select
                value={selectedGroup}
                onChange={(e) => setSelectedGroup(e.target.value)}
                className="pl-3 pr-8 py-2 bg-white border border-gray-200 rounded-lg text-gray-900 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 text-sm min-w-[140px] appearance-none"
              >
                <option value="">Alle Gruppen</option>
                {groups.map(group => (
                  <option key={group.id} value={group.id}>{group.name}</option>
                ))}
              </select>
              <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2">
                <svg className="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                </svg>
              </div>
            </div>

            <div className="relative">
              <select
                value={selectedYear}
                onChange={(e) => setSelectedYear(e.target.value)}
                className="pl-3 pr-8 py-2 bg-white border border-gray-200 rounded-lg text-gray-900 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 text-sm min-w-[120px] appearance-none"
              >
                <option value="all">Alle Jahre</option>
                <option value="1">Jahr 1</option>
                <option value="2">Jahr 2</option>
                <option value="3">Jahr 3</option>
                <option value="4">Jahr 4</option>
              </select>
              <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2">
                <svg className="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                </svg>
              </div>
            </div>

            <div className="relative">
              <select
                value={attendanceFilter}
                onChange={(e) => setAttendanceFilter(e.target.value)}
                className="pl-3 pr-8 py-2 bg-white border border-gray-200 rounded-lg text-gray-900 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 text-sm min-w-[130px] appearance-none"
              >
                <option value="all">Alle Status</option>
                <option value="anwesend">Anwesend</option>
                <option value="abwesend">Abwesend</option>
              </select>
              <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2">
                <svg className="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                </svg>
              </div>
            </div>
          </div>

          {/* Active Filters Bar */}
          {(searchTerm || selectedGroup || selectedYear !== "all" || attendanceFilter !== "all") && (
            <div className="mt-3 flex items-center justify-between text-sm text-gray-600">
              <div className="flex items-center gap-2">
                <span>
                  {(() => {
                    const activeFilters = [];
                    if (searchTerm) activeFilters.push(`Suche: "${searchTerm}"`);
                    if (selectedGroup) {
                      const groupName = groups.find(g => g.id === selectedGroup)?.name;
                      activeFilters.push(`Gruppe: ${groupName ?? selectedGroup}`);
                    }
                    if (selectedYear !== "all") activeFilters.push(`Jahr ${selectedYear}`);
                    if (attendanceFilter !== "all") {
                      const statusLabels: Record<string, string> = {
                        "anwesend": "Anwesend",
                        "abwesend": "Abwesend"
                      };
                      activeFilters.push(statusLabels[attendanceFilter] ?? attendanceFilter);
                    }
                    
                    if (activeFilters.length === 1) {
                      return `1 Filter aktiv: ${activeFilters[0]}`;
                    } else {
                      return `${activeFilters.length} Filter aktiv: ${activeFilters.join(", ")}`;
                    }
                  })()}
                </span>
              </div>
              <button
                onClick={() => {
                  setSearchTerm("");
                  setSelectedGroup("");
                  setSelectedYear("all");
                  setAttendanceFilter("all");
                }}
                className="text-blue-600 hover:text-blue-700 font-medium"
              >
                Filter zurücksetzen
              </button>
            </div>
          )}
        </div>

        {/* Mobile Error Display */}
        {error && (
          <div className="mb-4 md:hidden">
            <Alert type="error" message={error} />
          </div>
        )}

        {/* Student Grid - Mobile Optimized with Playful Design */}
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
          <div>
            {/* Add floating animation keyframes */}
            <style jsx>{`
              @keyframes float {
                0%, 100% { transform: translateY(0px) rotate(var(--rotation)); }
                50% { transform: translateY(-4px) rotate(var(--rotation)); }
              }
            `}</style>
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-2 gap-6 max-w-5xl mx-auto">
              {filteredStudents.map((student, index) => {
                const locationStatus = getLocationStatus(student);

                return (
                  <div
                    key={student.id}
                    onClick={() => router.push(`/students/${student.id}?from=/students/search`)}
                    className={`group cursor-pointer relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.03] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-3 active:scale-[0.97] md:hover:border-blue-200/50`}
                    style={{
                      transform: `rotate(${(index % 3 - 1) * 0.8}deg)`,
                      animation: `float 8s ease-in-out infinite ${index * 0.7}s`
                    }}
                  >
                    {/* Modern gradient overlay */}
                    <div className={`absolute inset-0 bg-gradient-to-br ${locationStatus.cardGradient} opacity-[0.03] rounded-3xl`}></div>
                    {/* Subtle inner glow */}
                    <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                    {/* Modern border highlight */}
                    <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 md:group-hover:ring-blue-200/60 transition-all duration-300"></div>
                    

                    <div className="relative p-6">
                      {/* Header with student name */}
                      <div className="flex items-center justify-between mb-3">
                        {/* Student Name */}
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2">
                            <h3 className="text-lg font-bold text-gray-800 break-words md:group-hover:text-blue-600 transition-colors duration-300">
                              {student.first_name}
                            </h3>
                            {/* Subtle integrated arrow */}
                            <svg className="w-4 h-4 text-gray-300 md:group-hover:text-blue-500 md:group-hover:translate-x-1 transition-all duration-300 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                            </svg>
                          </div>
                          <p className="text-base font-semibold text-gray-700 break-words md:group-hover:text-blue-500 transition-colors duration-300">
                            {student.second_name}
                          </p>
                        </div>
                        
                        {/* Status Badge */}
                        <span 
                          className={`inline-flex items-center px-3 py-1.5 rounded-full text-xs font-bold ${locationStatus.badgeColor} ml-3`}
                          style={{ 
                            backgroundColor: locationStatus.customBgColor,
                            boxShadow: locationStatus.customShadow
                          }}
                        >
                          <span className="w-1.5 h-1.5 bg-white/80 rounded-full mr-2 animate-pulse"></span>
                          {locationStatus.label}
                        </span>
                      </div>

                      {/* Additional Info */}
                      <div className="space-y-1 mb-3">
                        <div className="flex items-center text-sm text-gray-600">
                          <span>Klasse {student.school_class}</span>
                        </div>
                        {student.group_name && (
                          <div className="flex items-center text-sm text-gray-600">
                            <svg className="h-4 w-4 mr-2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                            </svg>
                            Gruppe: {student.group_name}
                          </div>
                        )}
                      </div>

                      {/* Bottom row with click hint */}
                      <div className="flex justify-start">
                        <p className="text-xs text-gray-400 md:group-hover:text-blue-400 transition-colors duration-300">
                          Tippen für mehr Infos
                        </p>
                      </div>

                      {/* Decorative elements */}
                      <div className="absolute top-3 left-3 w-5 h-5 bg-white/20 rounded-full animate-ping"></div>
                      <div className="absolute bottom-3 right-3 w-3 h-3 bg-white/30 rounded-full"></div>
                    </div>

                    {/* Glowing border effect */}
                    <div className="absolute inset-0 rounded-3xl opacity-0 md:group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent via-blue-100/30 to-transparent"></div>
                  </div>
                );
              })}
            </div>
          </div>
        )}
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