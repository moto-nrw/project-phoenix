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
    } else if (attendanceFilter === "abwesend" && student.current_location !== "Zuhause") {
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
    if (student.current_location === "Zuhause") {
      return { 
        label: "Zuhause", 
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
      <div className="w-full">
        {/* Modern Header with Clean Navigation */}
        <div className="mb-6">
          {/* Title Section */}
          <div className="mb-4">
            <div className="flex items-center justify-between gap-4">
              <h1 className="text-[1.625rem] md:text-3xl font-bold text-gray-900">
                Schülersuche
              </h1>
              {/* Result Count Badge */}
              <div className="flex items-center gap-2 px-3 py-1.5 bg-gray-100 rounded-full">
                <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                        d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                </svg>
                <span className="text-sm font-medium text-gray-700">
                  {filteredStudents.length}
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Mobile Search & Filters - Modern Minimal Design */}
        <div className="mb-6 md:hidden">
          {/* Search Input with Integrated Filter Button */}
          <div className="flex gap-2 mb-3">
            <div className="relative flex-1">
              <svg className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
              <input
                type="text"
                placeholder="Name suchen..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="w-full pl-9 pr-3 py-2.5 bg-white border border-gray-200 rounded-2xl text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-all duration-200 text-sm"
              />
              {searchTerm && (
                <button
                  onClick={() => setSearchTerm("")}
                  className="absolute right-2 top-1/2 transform -translate-y-1/2 p-1 hover:bg-gray-100 rounded-full transition-colors"
                >
                  <svg className="h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              )}
            </div>
            <button
              onClick={() => setIsMobileFiltersOpen(!isMobileFiltersOpen)}
              className={`
                p-2.5 rounded-2xl transition-all duration-200
                ${isMobileFiltersOpen 
                  ? 'bg-blue-500 text-white' 
                  : 'bg-white border border-gray-200 text-gray-600 hover:bg-gray-50'
                }
                ${(selectedGroup || selectedYear !== "all" || attendanceFilter !== "all") ? 'ring-2 ring-blue-500 ring-offset-1' : ''}
              `}
            >
              <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
            </svg>
            </button>
          </div>

          {/* Active Filter Chips */}
          {(selectedGroup || selectedYear !== "all" || attendanceFilter !== "all") && (
            <div className="flex gap-2 mb-3 flex-wrap">
              {selectedGroup && (
                <button
                  onClick={() => setSelectedGroup("")}
                  className="inline-flex items-center gap-1 px-2.5 py-1 bg-blue-100 text-blue-700 rounded-full text-xs font-medium"
                >
                  {groups.find(g => g.id === selectedGroup)?.name || selectedGroup}
                  <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              )}
              {selectedYear !== "all" && (
                <button
                  onClick={() => setSelectedYear("all")}
                  className="inline-flex items-center gap-1 px-2.5 py-1 bg-blue-100 text-blue-700 rounded-full text-xs font-medium"
                >
                  Jahr {selectedYear}
                  <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              )}
              {attendanceFilter !== "all" && (
                <button
                  onClick={() => setAttendanceFilter("all")}
                  className="inline-flex items-center gap-1 px-2.5 py-1 bg-blue-100 text-blue-700 rounded-full text-xs font-medium"
                >
                  {attendanceFilter === "anwesend" ? "Anwesend" : "Zuhause"}
                  <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              )}
            </div>
          )}

          {/* Expandable Filter Panel */}
          {isMobileFiltersOpen && (
            <div className="bg-white rounded-2xl border border-gray-200 p-4 mb-3 shadow-sm">
              <div className="space-y-3">
                {/* Year Filter */}
                <div>
                  <label className="text-xs font-medium text-gray-600 mb-1.5 block">Klassenstufe</label>
                  <div className="grid grid-cols-5 gap-1.5">
                    <button
                      onClick={() => setSelectedYear("all")}
                      className={`py-2 px-3 rounded-lg text-sm font-medium transition-all ${
                        selectedYear === "all" 
                          ? 'bg-gray-900 text-white' 
                          : 'bg-gray-50 text-gray-600 hover:bg-gray-100'
                      }`}
                    >
                      Alle
                    </button>
                    {['1', '2', '3', '4'].map((year) => (
                      <button
                        key={year}
                        onClick={() => setSelectedYear(year)}
                        className={`py-2 px-3 rounded-lg text-sm font-medium transition-all ${
                          selectedYear === year 
                            ? 'bg-gray-900 text-white' 
                            : 'bg-gray-50 text-gray-600 hover:bg-gray-100'
                        }`}
                      >
                        {year}
                      </button>
                    ))}
                  </div>
                </div>

                {/* Group Filter */}
                <div>
                  <label className="text-xs font-medium text-gray-600 mb-1.5 block">Gruppe</label>
                  <div className="grid grid-cols-2 gap-2">
                    <button
                      onClick={() => setSelectedGroup("")}
                      className={`py-2 px-3 rounded-lg text-sm font-medium transition-all ${
                        selectedGroup === "" 
                          ? 'bg-gray-900 text-white' 
                          : 'bg-gray-50 text-gray-600 hover:bg-gray-100'
                      }`}
                    >
                      Alle Gruppen
                    </button>
                    {groups.slice(0, 5).map((group) => (
                      <button
                        key={group.id}
                        onClick={() => setSelectedGroup(group.id)}
                        className={`py-2 px-3 rounded-lg text-sm font-medium transition-all text-left ${
                          selectedGroup === group.id 
                            ? 'bg-gray-900 text-white' 
                            : 'bg-gray-50 text-gray-600 hover:bg-gray-100'
                        }`}
                      >
                        {group.name}
                      </button>
                    ))}
                    {groups.length > 5 && (
                      <div className="col-span-2 text-center text-xs text-gray-500 py-1">
                        +{groups.length - 5} weitere Gruppen
                      </div>
                    )}
                  </div>
                </div>

                {/* Status Filter */}
                <div>
                  <label className="text-xs font-medium text-gray-600 mb-1.5 block">Anwesenheit</label>
                  <div className="grid grid-cols-3 gap-2">
                    <button
                      onClick={() => setAttendanceFilter("all")}
                      className={`py-2 px-3 rounded-lg text-sm font-medium transition-all ${
                        attendanceFilter === "all" 
                          ? 'bg-gray-900 text-white' 
                          : 'bg-gray-50 text-gray-600 hover:bg-gray-100'
                      }`}
                    >
                      Alle
                    </button>
                    <button
                      onClick={() => setAttendanceFilter("anwesend")}
                      className={`py-2 px-3 rounded-lg text-sm font-medium transition-all ${
                        attendanceFilter === "anwesend" 
                          ? 'bg-gray-900 text-white' 
                          : 'bg-gray-50 text-gray-600 hover:bg-gray-100'
                      }`}
                    >
                      Anwesend
                    </button>
                    <button
                      onClick={() => setAttendanceFilter("abwesend")}
                      className={`py-2 px-3 rounded-lg text-sm font-medium transition-all ${
                        attendanceFilter === "abwesend" 
                          ? 'bg-gray-900 text-white' 
                          : 'bg-gray-50 text-gray-600 hover:bg-gray-100'
                      }`}
                    >
                      Zuhause
                    </button>
                  </div>
                </div>
              </div>

              {/* Filter Actions */}
              <div className="flex gap-2 mt-4 pt-3 border-t border-gray-100">
                <button
                  onClick={() => {
                    setSelectedGroup("");
                    setSelectedYear("all");
                    setAttendanceFilter("all");
                  }}
                  className="flex-1 py-2 text-sm font-medium text-gray-600 hover:text-gray-900 transition-colors"
                >
                  Zurücksetzen
                </button>
                <button
                  onClick={() => setIsMobileFiltersOpen(false)}
                  className="flex-1 py-2 bg-gray-900 text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors"
                >
                  Fertig
                </button>
              </div>
            </div>
          )}

        </div>

        {/* Desktop Search & Filter - Modern Minimal Design */}
        <div className="hidden md:block mb-6">
          <div className="flex items-center gap-3 mb-3">
            {/* Search Input */}
            <div className="flex-1 relative">
              <svg className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
              <input
                type="text"
                placeholder="Name suchen..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="w-full pl-10 pr-10 py-2.5 bg-white border border-gray-200 rounded-2xl text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-all duration-200"
              />
              {searchTerm && (
                <button
                  onClick={() => setSearchTerm("")}
                  className="absolute right-3 top-1/2 transform -translate-y-1/2 p-1 hover:bg-gray-100 rounded-full transition-colors"
                >
                  <svg className="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              )}
            </div>

            {/* Filter Buttons */}
            <div className="flex gap-2">
              {/* Year Filter */}
              <div className="flex bg-white rounded-xl p-1 shadow-sm h-10">
                {['all', '1', '2', '3', '4'].map((year) => (
                  <button
                    key={year}
                    onClick={() => setSelectedYear(year)}
                    className={`
                      px-3 rounded-lg text-sm font-medium transition-all
                      ${selectedYear === year 
                        ? 'bg-gray-900 text-white' 
                        : 'text-gray-600 hover:text-gray-900'
                      }
                    `}
                  >
                    {year === 'all' ? 'Alle' : year}
                  </button>
                ))}
              </div>

              {/* Group Filter Dropdown */}
              <div className="relative">
                <button
                  onClick={() => setIsMobileFiltersOpen(!isMobileFiltersOpen)}
                  className={`
                    flex items-center gap-2 px-4 h-10 rounded-xl transition-all shadow-sm
                    ${selectedGroup !== "" 
                      ? 'bg-gray-900 text-white' 
                      : 'bg-white text-gray-700 hover:bg-gray-50'
                    }
                  `}
                >
                  <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                  </svg>
                  <span className="text-sm font-medium">
                    {selectedGroup ? groups.find(g => g.id === selectedGroup)?.name || "Gruppe" : "Alle Gruppen"}
                  </span>
                  <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                  </svg>
                </button>
                
                {/* Dropdown Menu */}
                {isMobileFiltersOpen && (
                  <div className="absolute right-0 mt-2 w-48 bg-white rounded-xl shadow-lg border border-gray-200 py-1 z-10 max-h-80 overflow-y-auto">
                    <button
                      onClick={() => {
                        setSelectedGroup("");
                        setIsMobileFiltersOpen(false);
                      }}
                      className={`
                        w-full text-left px-4 py-2 text-sm transition-colors
                        ${selectedGroup === "" 
                          ? 'bg-gray-100 text-gray-900 font-medium' 
                          : 'text-gray-700 hover:bg-gray-50'
                        }
                      `}
                    >
                      Alle Gruppen
                    </button>
                    {groups.map(group => (
                      <button
                        key={group.id}
                        onClick={() => {
                          setSelectedGroup(group.id);
                          setIsMobileFiltersOpen(false);
                        }}
                        className={`
                          w-full text-left px-4 py-2 text-sm transition-colors
                          ${selectedGroup === group.id 
                            ? 'bg-gray-100 text-gray-900 font-medium' 
                            : 'text-gray-700 hover:bg-gray-50'
                          }
                        `}
                      >
                        {group.name}
                      </button>
                    ))}
                  </div>
                )}
              </div>

              {/* Status Filter Dropdown */}
              <div className="relative">
                <button
                  onClick={() => {
                    // Toggle a separate state for attendance dropdown
                    const element = document.getElementById('attendance-dropdown');
                    if (element) {
                      element.classList.toggle('hidden');
                    }
                  }}
                  className={`
                    flex items-center gap-2 px-4 h-10 rounded-xl transition-all shadow-sm
                    ${attendanceFilter !== "all" 
                      ? 'bg-gray-900 text-white' 
                      : 'bg-white text-gray-700 hover:bg-gray-50'
                    }
                  `}
                >
                  <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <span className="text-sm font-medium">
                    {attendanceFilter === "all" && "Alle Status"}
                    {attendanceFilter === "anwesend" && "Anwesend"}
                    {attendanceFilter === "abwesend" && "Zuhause"}
                  </span>
                  <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                  </svg>
                </button>
                
                {/* Dropdown Menu */}
                <div id="attendance-dropdown" className="hidden absolute right-0 mt-2 w-48 bg-white rounded-xl shadow-lg border border-gray-200 py-1 z-10">
                  {[
                    { value: "all", label: "Alle Status" },
                    { value: "anwesend", label: "Anwesend" },
                    { value: "abwesend", label: "Zuhause" }
                  ].map((status) => (
                    <button
                      key={status.value}
                      onClick={() => {
                        setAttendanceFilter(status.value);
                        document.getElementById('attendance-dropdown')?.classList.add('hidden');
                      }}
                      className={`
                        w-full text-left px-4 py-2 text-sm transition-colors
                        ${attendanceFilter === status.value 
                          ? 'bg-gray-100 text-gray-900 font-medium' 
                          : 'text-gray-700 hover:bg-gray-50'
                        }
                      `}
                    >
                      {status.label}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          </div>

          {/* Active Filter Chips */}
          {(searchTerm || selectedGroup || selectedYear !== "all" || attendanceFilter !== "all") && (
            <div className="flex items-center justify-between">
              <div className="flex gap-2 flex-wrap">
                {searchTerm && (
                  <span className="inline-flex items-center gap-1 px-3 py-1 bg-blue-100 text-blue-700 rounded-full text-xs font-medium">
                    "{searchTerm}"
                    <button onClick={() => setSearchTerm("")} className="hover:text-blue-900">
                      <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  </span>
                )}
                {selectedGroup && (
                  <span className="inline-flex items-center gap-1 px-3 py-1 bg-blue-100 text-blue-700 rounded-full text-xs font-medium">
                    {groups.find(g => g.id === selectedGroup)?.name || "Gruppe"}
                    <button onClick={() => setSelectedGroup("")} className="hover:text-blue-900">
                      <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  </span>
                )}
                {selectedYear !== "all" && (
                  <span className="inline-flex items-center gap-1 px-3 py-1 bg-blue-100 text-blue-700 rounded-full text-xs font-medium">
                    Jahr {selectedYear}
                    <button onClick={() => setSelectedYear("all")} className="hover:text-blue-900">
                      <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  </span>
                )}
                {attendanceFilter !== "all" && (
                  <span className="inline-flex items-center gap-1 px-3 py-1 bg-blue-100 text-blue-700 rounded-full text-xs font-medium">
                    {attendanceFilter === "anwesend" ? "Anwesend" : "Zuhause"}
                    <button onClick={() => setAttendanceFilter("all")} className="hover:text-blue-900">
                      <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  </span>
                )}
              </div>
              <button
                onClick={() => {
                  setSearchTerm("");
                  setSelectedGroup("");
                  setSelectedYear("all");
                  setAttendanceFilter("all");
                }}
                className="text-sm text-gray-500 hover:text-gray-700 font-medium transition-colors"
              >
                Alle zurücksetzen
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
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3 gap-6">
              {filteredStudents.map((student, index) => {
                const locationStatus = getLocationStatus(student);

                return (
                  <div
                    key={student.id}
                    onClick={() => router.push(`/students/${student.id}?from=/students/search`)}
                    className={`group cursor-pointer relative overflow-hidden rounded-2xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.03] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-3 active:scale-[0.97] md:hover:border-blue-200/50`}
                    style={{
                      transform: `rotate(${(index % 3 - 1) * 0.8}deg)`,
                      animation: `float 8s ease-in-out infinite ${index * 0.7}s`
                    }}
                  >
                    {/* Modern gradient overlay */}
                    <div className={`absolute inset-0 bg-gradient-to-br ${locationStatus.cardGradient} opacity-[0.03] rounded-2xl`}></div>
                    {/* Subtle inner glow */}
                    <div className="absolute inset-px rounded-2xl bg-gradient-to-br from-white/80 to-white/20"></div>
                    {/* Modern border highlight */}
                    <div className="absolute inset-0 rounded-2xl ring-1 ring-white/20 md:group-hover:ring-blue-200/60 transition-all duration-300"></div>
                    

                    <div className="relative p-6">
                      {/* Header with student name */}
                      <div className="flex items-center justify-between mb-3">
                        {/* Student Name */}
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2">
                            <h3 className="text-lg font-bold text-gray-800 whitespace-nowrap overflow-hidden text-ellipsis md:group-hover:text-blue-600 transition-colors duration-300">
                              {student.first_name}
                            </h3>
                            {/* Subtle integrated arrow */}
                            <svg className="w-4 h-4 text-gray-300 md:group-hover:text-blue-500 md:group-hover:translate-x-1 transition-all duration-300 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                            </svg>
                          </div>
                          <p className="text-base font-semibold text-gray-700 whitespace-nowrap overflow-hidden text-ellipsis md:group-hover:text-blue-500 transition-colors duration-300">
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
                    <div className="absolute inset-0 rounded-2xl opacity-0 md:group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent via-blue-100/30 to-transparent"></div>
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