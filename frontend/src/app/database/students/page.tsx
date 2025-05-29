"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import { ResponsiveLayout } from "@/components/dashboard";
import type { Student } from "@/lib/api";
import { studentService } from "@/lib/api";
import { formatStudentName } from "@/lib/student-helpers";
import { GroupSelector } from "@/components/groups";
import Link from "next/link";

// Student list will be loaded from API

export default function StudentsPage() {
  const router = useRouter();
  const [students, setStudents] = useState<Student[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState("");
  const [groupFilter, setGroupFilter] = useState<string | null>(null);
  const [isMobileFiltersOpen, setIsMobileFiltersOpen] = useState(false);

  // const handleSearchInput = (value: string) => {
  //   setSearchFilter(value);
  // };

  // const handleFilterChange = (filterId: string, value: string | null) => {
  //   if (filterId === 'group') {
  //     setGroupFilter(value);
  //   }
  // };

  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Function to fetch students with optional filters
  const fetchStudents = async (search?: string, groupId?: string | null) => {
    try {
      setLoading(true);

      // Prepare filters for API call
      const filters = {
        search: search ?? undefined,
        groupId: groupId ?? undefined,
      };

      try {
        // Fetch from the real API using our student service
        const data = await studentService.getStudents(filters);
        console.log("Received data from studentService:", data);
        setStudents(data);
        setError(null);
      } catch (apiErr) {
        console.error("API error when fetching students:", apiErr);
        setError(
          "Fehler beim Laden der Schülerdaten. Bitte versuchen Sie es später erneut.",
        );
        setStudents([]);
      }
    } catch (err) {
      console.error("Error fetching students:", err);
      setError(
        "Fehler beim Laden der Schülerdaten. Bitte versuchen Sie es später erneut.",
      );
      setStudents([]);
    } finally {
      setLoading(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchStudents();
  }, []);

  // Handle search and group filter changes
  useEffect(() => {
    // Debounce search to avoid too many API calls
    const timer = setTimeout(() => {
      void fetchStudents(searchFilter, groupFilter);
    }, 300);

    return () => clearTimeout(timer);
  }, [searchFilter, groupFilter]);
  

  if (status === "loading" || loading) {
    return (
      <ResponsiveLayout userName={session?.user?.name ?? "Root"}>
        <div className="p-4 md:p-6 lg:p-8 pb-24 lg:pb-8">
          <div className="flex flex-col items-center justify-center py-12 md:py-16">
            <div className="flex flex-col items-center gap-4">
              <div className="h-10 w-10 md:h-12 md:w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
              <p className="text-sm md:text-base text-gray-600">Daten werden geladen...</p>
            </div>
          </div>
        </div>
      </ResponsiveLayout>
    );
  }

  const handleSelectStudent = (student: Student) => {
    router.push(`/database/students/${student.id}`);
  };

  // Custom renderer for student items
  const renderStudent = (student: Student) => {
    return (
    <>
      <div className="flex flex-col min-w-0 flex-1 transition-transform duration-200 group-hover:translate-x-1">
        <div className="flex flex-col sm:flex-row sm:items-center sm:gap-2">
          <span className="font-semibold text-gray-900 transition-colors duration-200 group-hover:text-blue-600 truncate">
            {formatStudentName(student)}
          </span>
          {student.in_house && (
            <span className="inline-flex mt-1 sm:mt-0 self-start sm:self-auto rounded-full bg-green-100 px-2 py-0.5 text-xs text-green-800">
              Anwesend
            </span>
          )}
        </div>
        <div className="text-xs sm:text-sm text-gray-500 mt-1">
          <span>Klasse: {student.school_class}</span>
          {student.group_name && student.group_id && (
            <>
              <span className="hidden sm:inline"> | </span>
              <span className="block sm:inline mt-0.5 sm:mt-0">
                Gruppe:{' '}
                <a
                  href={`/database/groups/${student.group_id}`}
                  className="text-blue-600 transition-colors hover:text-blue-800 hover:underline"
                  onClick={(e) => {
                    e.stopPropagation(); // Prevent triggering the parent click event
                  }}
                >
                  {student.group_name}
                </a>
              </span>
            </>
          )}
        </div>
      </div>
      <svg
        xmlns="http://www.w3.org/2000/svg"
        className="h-5 w-5 text-gray-400 transition-all duration-200 group-hover:translate-x-1 group-hover:transform group-hover:text-blue-500 flex-shrink-0"
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
    </>
  );
  };

  // Show error if loading failed
  if (error) {
    return (
      <ResponsiveLayout userName={session?.user?.name ?? "Root"}>
        <div className="p-4 md:p-6 lg:p-8 pb-24 lg:pb-8">
          <div className="flex flex-col items-center justify-center py-8 md:py-12">
            <div className="max-w-md w-full rounded-lg bg-red-50 p-4 md:p-6 text-red-800 shadow-md">
              <h2 className="mb-2 text-lg md:text-xl font-semibold">Fehler</h2>
              <p className="text-sm md:text-base">{error}</p>
              <button
                onClick={() => fetchStudents()}
                className="mt-4 w-full md:w-auto rounded-lg bg-red-100 px-4 py-2 text-sm md:text-base text-red-800 transition-colors hover:bg-red-200 active:scale-[0.98]"
              >
                Erneut versuchen
              </button>
            </div>
          </div>
        </div>
      </ResponsiveLayout>
    );
  }

  // Define the filters for the student list
  /*
  const filters = [
    {
      id: 'group',
      label: 'Gruppe',
      options: [
        { label: 'Alle Gruppen', value: null },
        // The actual options will be populated by the GroupSelector component
      ],
    },
  ];
  */

  return (
    <ResponsiveLayout userName={session?.user?.name ?? "Root"}>
      <div className="p-4 md:p-6 lg:p-8 pb-24 lg:pb-8">
        {/* Header Section - Mobile Responsive */}
        <div className="mb-4 md:mb-6 lg:mb-8">
          <h1 className="text-2xl md:text-3xl font-bold text-gray-900">Schüler auswählen</h1>
          <p className="mt-1 text-sm md:text-base text-gray-600">Verwalte Schülerdaten und Gruppenzuweisungen</p>
        </div>

        {/* Mobile Search Bar */}
        <div className="mb-4 md:hidden">
          <div className="relative">
            <input
              type="text"
              placeholder="Schüler suchen..."
              value={searchFilter}
              onChange={(e) => setSearchFilter(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-4 py-3 pl-10 text-base transition-all duration-200 hover:border-gray-400 focus:shadow-md focus:ring-2 focus:ring-blue-500 focus:outline-none"
            />
            <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                className="h-5 w-5 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                />
              </svg>
            </div>
          </div>
        </div>

        {/* Mobile Filter Toggle */}
        <div className="mb-4 md:hidden">
          <button
            onClick={() => setIsMobileFiltersOpen(!isMobileFiltersOpen)}
            className="flex w-full items-center justify-between rounded-lg bg-white px-4 py-3 shadow-sm ring-1 ring-gray-200 hover:ring-gray-300 focus:ring-2 focus:ring-blue-500 focus:outline-none transition-all duration-200"
          >
            <span className="text-sm font-medium text-gray-700">
              Filter & Optionen
            </span>
            <svg 
              className={`h-5 w-5 text-gray-400 transition-transform duration-200 ${isMobileFiltersOpen ? 'rotate-180' : ''}`} 
              fill="none" 
              viewBox="0 0 24 24" 
              stroke="currentColor"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>
        </div>

        {/* Desktop Search and Filters */}
        <div className={`mb-6 md:mb-8 ${isMobileFiltersOpen ? 'block' : 'hidden md:block'}`}>
          <div className="rounded-lg bg-white p-4 md:p-6 shadow-md border border-gray-100">
            <div className="flex flex-col gap-4">
              {/* Desktop Search and Add Button Row */}
              <div className="hidden md:flex items-center justify-between gap-4">
                <div className="relative max-w-md flex-1">
                  <input
                    type="text"
                    placeholder="Suchen..."
                    value={searchFilter}
                    onChange={(e) => setSearchFilter(e.target.value)}
                    className="w-full rounded-lg border border-gray-300 px-4 py-3 pl-10 transition-all duration-200 hover:border-gray-400 focus:shadow-md focus:ring-2 focus:ring-blue-500 focus:outline-none"
                  />
                  <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      className="h-5 w-5 text-gray-400"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                      />
                    </svg>
                  </div>
                </div>

                <Link href="/database/students/new">
                  <button className="group flex items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-blue-500 to-blue-600 px-6 py-3 text-sm font-medium text-white shadow-sm transition-all duration-200 hover:from-blue-600 hover:to-blue-700 hover:shadow-md">
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90"
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
                    <span>Neuen Schüler erstellen</span>
                  </button>
                </Link>
              </div>

              {/* Filter Section */}
              <div className="flex flex-col md:flex-row md:items-center gap-3 md:gap-4">
                <span className="text-sm font-medium text-gray-700">Filter:</span>
                <div className="flex-1 md:max-w-xs">
                  <GroupSelector
                    value={groupFilter ?? ""}
                    onChange={(value) =>
                      setGroupFilter(value === "" ? null : value)
                    }
                    includeEmpty={true}
                    emptyLabel="Alle Gruppen"
                    label=""
                  />
                </div>
              </div>

              {/* Mobile Add Button */}
              <Link href="/database/students/new" className="md:hidden">
                <button className="group flex w-full items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-blue-500 to-blue-600 px-4 py-3 text-sm font-medium text-white shadow-sm transition-all duration-200 hover:from-blue-600 hover:to-blue-700 hover:shadow-md active:scale-[0.98]">
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90"
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
                  <span>Neuen Schüler erstellen</span>
                </button>
              </Link>
            </div>
          </div>
        </div>

        {/* Student List */}
        <div className="rounded-lg bg-white shadow-md border border-gray-100">
          <div className="p-4 md:p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg md:text-xl font-semibold text-gray-900">
                Schülerliste
              </h2>
              <span className="text-sm text-gray-500">
                {students.length} {students.length === 1 ? 'Eintrag' : 'Einträge'}
              </span>
            </div>
            
            <div className="space-y-2 md:space-y-3">
              {students.length > 0 ? (
                students.map((student) => (
                  <div
                    key={student.id}
                    className="group flex cursor-pointer items-center justify-between rounded-lg border border-gray-200 bg-white p-3 md:p-4 transition-all duration-200 hover:border-blue-300 hover:shadow-md active:scale-[0.99] min-h-[60px] md:min-h-[72px]"
                    onClick={() => handleSelectStudent(student)}
                  >
                    {renderStudent(student)}
                  </div>
                ))
              ) : (
                <div className="py-8 md:py-12 text-center">
                  <div className="flex flex-col items-center gap-4">
                    <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                    </svg>
                    <div>
                      <h3 className="text-base md:text-lg font-medium text-gray-900 mb-1">
                        {searchFilter || groupFilter
                          ? "Keine Ergebnisse gefunden"
                          : "Keine Schüler vorhanden"}
                      </h3>
                      <p className="text-sm text-gray-600">
                        {searchFilter || groupFilter
                          ? "Versuche deine Suchkriterien anzupassen."
                          : "Erstelle deinen ersten Schüler über den Button oben."}
                      </p>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </ResponsiveLayout>
  );
}
