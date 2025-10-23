"use client";

import { useState, useEffect, useCallback, useRef, Suspense, useMemo } from "react";
import clsx from "clsx";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header";
import { studentService, groupService } from "~/lib/api";
import type { Student, Group } from "~/lib/api";
import { useSSE } from "~/lib/hooks/use-sse";
import type { SSEEvent } from "~/lib/sse-types";
import { LocationBadge } from "~/components/simple/student/LocationBadge";
import { mapLocationStatus } from "~/lib/student-location-helpers";
import { getLocationCardVisuals } from "~/lib/student-location-visuals";

function SearchPageContent() {
  const { status } = useSession();
  const router = useRouter();
  const searchTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const fallbackPollRef = useRef<NodeJS.Timeout | null>(null);

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

  const fetchStudentsData = useCallback(
    async (filters?: { search?: string; groupId?: string }) => {
      try {
        setIsSearching(true);
        setError(null);

        const fetchedStudents = await studentService.getStudents({
          search: filters?.search ?? searchTerm,
          groupId: filters?.groupId ?? selectedGroup,
        });

        setStudents(fetchedStudents.students);
      } catch {
        setError("Fehler beim Laden der Schülerdaten.");
      } finally {
        setIsSearching(false);
      }
    },
    [searchTerm, selectedGroup],
  );

  // Load groups and user's OGS groups on mount
  useEffect(() => {
    const loadInitialData = async () => {
      try {
        const fetchedGroups = await groupService.getGroups();
        setGroups(fetchedGroups);
      } catch (error) {
        console.error("Error loading groups:", error);
      }
    };

    void loadInitialData();
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

  const handleSSEEvent = useCallback(
    (event: SSEEvent) => {
      if (
        (event.type !== "student_checkin" && event.type !== "student_checkout") ||
        !event.data?.student_id
      ) {
        return;
      }

      const mappedStatus = mapLocationStatus(event.data.location_status);
      const studentId = event.data.student_id;

      setStudents((prev) => {
        if (!Array.isArray(prev) || prev.length === 0) {
          return prev;
        }

        let didUpdate = false;
        const next = prev.map((student) => {
          if (student.id !== studentId) {
            return student;
          }

          didUpdate = true;
          return {
            ...student,
            location_status: mappedStatus ?? undefined,
            in_house: mappedStatus ? mappedStatus.state !== "HOME" : student.in_house,
            school_yard: mappedStatus
              ? mappedStatus.state === "SCHOOLYARD"
              : student.school_yard,
          };
        });

        return didUpdate ? next : prev;
      });
    },
    [],
  );

  const { status: sseStatus } = useSSE("/api/sse/events", {
    onMessage: handleSSEEvent,
  });

  useEffect(() => {
    if (sseStatus === "connected") {
      if (fallbackPollRef.current) {
        clearInterval(fallbackPollRef.current);
        fallbackPollRef.current = null;
      }
      return;
    }

    if (sseStatus === "reconnecting" || sseStatus === "failed") {
      fallbackPollRef.current ??= setInterval(() => {
        void fetchStudentsData();
      }, 30_000);
      void fetchStudentsData();
    }

    return () => {
      if (fallbackPollRef.current) {
        clearInterval(fallbackPollRef.current);
        fallbackPollRef.current = null;
      }
    };
  }, [sseStatus, fetchStudentsData]);

  useEffect(
    () => () => {
      if (fallbackPollRef.current) {
        clearInterval(fallbackPollRef.current);
        fallbackPollRef.current = null;
      }
    },
    [],
  );

  // Prepare filter configurations for PageHeaderWithSearch
  const filterConfigs: FilterConfig[] = useMemo(() => [
    {
      id: 'year',
      label: 'Klassenstufe',
      type: 'buttons',
      value: selectedYear,
      onChange: (value) => setSelectedYear(value as string),
      options: [
        { value: 'all', label: 'Alle' },
        { value: '1', label: '1' },
        { value: '2', label: '2' },
        { value: '3', label: '3' },
        { value: '4', label: '4' }
      ]
    },
    {
      id: 'group',
      label: 'Gruppe',
      type: 'dropdown',
      value: selectedGroup,
      onChange: (value) => setSelectedGroup(value as string),
      options: [
        { value: '', label: 'Alle Gruppen' },
        ...groups.map(group => ({ value: group.id, label: group.name }))
      ]
    },
    {
      id: 'attendance',
      label: 'Anwesenheit',
      type: 'dropdown',
      value: attendanceFilter,
      onChange: (value) => setAttendanceFilter(value as string),
      options: [
        { value: 'all', label: 'Alle Status' },
        { value: 'anwesend', label: 'Anwesend' },
        { value: 'abwesend', label: 'Zuhause' }
      ]
    }
  ], [selectedYear, selectedGroup, attendanceFilter, groups]);

  // Prepare active filters for display
  const activeFilters: ActiveFilter[] = useMemo(() => {
    const filters: ActiveFilter[] = [];
    
    if (searchTerm) {
      filters.push({
        id: 'search',
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm("")
      });
    }
    
    if (selectedYear !== "all") {
      filters.push({
        id: 'year',
        label: `Jahr ${selectedYear}`,
        onRemove: () => setSelectedYear("all")
      });
    }
    
    if (selectedGroup) {
      const groupName = groups.find(g => g.id === selectedGroup)?.name ?? "Gruppe";
      filters.push({
        id: 'group',
        label: groupName,
        onRemove: () => setSelectedGroup("")
      });
    }
    
    if (attendanceFilter !== "all") {
      const statusLabels: Record<string, string> = {
        "anwesend": "Anwesend",
        "abwesend": "Zuhause"
      };
      filters.push({
        id: 'attendance',
        label: statusLabels[attendanceFilter] ?? attendanceFilter,
        onRemove: () => setAttendanceFilter("all")
      });
    }
    
    return filters;
  }, [searchTerm, selectedYear, selectedGroup, attendanceFilter, groups]);

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
      <div className="w-full -mt-1.5">
        {/* PageHeaderWithSearch - No title */}
        <PageHeaderWithSearch
          title=""
          badge={{
            icon: (
              <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                      d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
              </svg>
            ),
            count: filteredStudents.length
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Name suchen..."
          }}
          filters={filterConfigs}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
            setSelectedGroup("");
            setSelectedYear("all");
            setAttendanceFilter("all");
          }}
        />

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
                const locationVisuals = getLocationCardVisuals(
                  student.location_status ?? null,
                );

                return (
                  <div
                    key={student.id}
                    onClick={() => router.push(`/students/${student.id}?from=/students/search`)}
                    className={clsx(
                      "group cursor-pointer relative overflow-hidden rounded-2xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.03] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-3 active:scale-[0.97]",
                      locationVisuals.hoverBorderClass,
                    )}
                    style={{
                      transform: `rotate(${(index % 3 - 1) * 0.8}deg)`,
                      animation: `float 8s ease-in-out infinite ${index * 0.7}s`
                    }}
                  >
                    {/* Ambient gradient overlay */}
                    <div
                      className="absolute inset-0 rounded-2xl opacity-[0.07]"
                      style={locationVisuals.overlayStyle}
                    ></div>
                    {/* Subtle inner glow */}
                    <div className="absolute inset-px rounded-2xl bg-gradient-to-br from-white/80 to-white/20"></div>
                    {/* Modern border highlight */}
                    <div
                      className={clsx(
                        "absolute inset-0 rounded-2xl ring-1 ring-white/20 transition-all duration-300",
                        locationVisuals.hoverRingClass,
                      )}
                    ></div>
                    

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

                        {/* Location Status Badge */}
                        <div className="ml-3 flex-shrink-0">
                          <LocationBadge
                            locationStatus={student.location_status ?? null}
                            className={locationVisuals.badgeClassName}
                            style={locationVisuals.badgeStyle}
                          />
                        </div>
                      </div>

                      {/* Additional Info */}
                      <div className="space-y-1 mb-3">
                        <div className="flex items-center text-sm text-gray-600">
                          <svg className="h-4 w-4 mr-2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
                          </svg>
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
                    <div
                      className={clsx(
                        "absolute inset-0 rounded-2xl opacity-0 md:group-hover:opacity-100 transition-opacity duration-300",
                        locationVisuals.glowClassName,
                      )}
                    ></div>
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
