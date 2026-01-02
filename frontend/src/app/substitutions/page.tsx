"use client";

import { useState, useEffect, useCallback, useMemo, Suspense } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { ResponsiveLayout } from "@/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header";
import { Modal, ConfirmationModal } from "~/components/ui/modal";
import { Alert } from "~/components/ui/alert";
import { substitutionService } from "~/lib/substitution-api";
import { groupService } from "~/lib/api";
import type { Group } from "~/lib/api";
import type {
  Substitution,
  TeacherAvailability,
} from "~/lib/substitution-helpers";
import {
  formatTeacherName,
  getTeacherStatus,
  getSubstitutionCounts,
} from "~/lib/substitution-helpers";

import { Loading } from "~/components/ui/loading";
import { useToast } from "~/contexts/ToastContext";

// Helper function to resolve substitute teacher name
function getSubstituteName(
  teachers: TeacherAvailability[],
  substitution: Substitution,
): string {
  const substituteTeacher = teachers.find(
    (t) => t.id === substitution.substituteStaffId,
  );
  return substituteTeacher
    ? formatTeacherName(substituteTeacher)
    : (substitution.substituteStaffName ?? "Unbekannt");
}

// Helper component for rendering substitution count badges
function SubstitutionBadges({
  teacher,
}: Readonly<{ teacher: TeacherAvailability }>) {
  const counts = getSubstitutionCounts(teacher);
  const hasBoth = counts.transfers > 0 && counts.substitutions > 0;

  return (
    <>
      {counts.transfers > 0 && (
        <span
          className={`absolute flex h-5 w-5 items-center justify-center rounded-full bg-orange-500 text-xs font-bold text-white shadow-sm ${hasBoth ? "-top-2 right-2.5 z-10" : "-top-1 -right-1"}`}
        >
          {counts.transfers}
        </span>
      )}
      {counts.substitutions > 0 && (
        <span className="absolute -top-1 -right-1 z-20 flex h-5 w-5 items-center justify-center rounded-full bg-purple-500 text-xs font-bold text-white shadow-sm">
          {counts.substitutions}
        </span>
      )}
    </>
  );
}

// Helper component for rendering status indicators
function StatusIndicator({
  teacher,
  size = "default",
}: Readonly<{
  teacher: TeacherAvailability;
  size?: "default" | "large";
}>) {
  const counts = getSubstitutionCounts(teacher);
  const dotSize = size === "large" ? "h-2.5 w-2.5" : "h-2 w-2";

  if (counts.transfers > 0 && counts.substitutions > 0) {
    return (
      <div className="flex gap-0.5">
        <span
          className={`${dotSize} animate-pulse rounded-full bg-orange-500`}
        ></span>
        <span
          className={`${dotSize} animate-pulse rounded-full bg-purple-500`}
        ></span>
      </div>
    );
  } else if (counts.transfers > 0) {
    return (
      <span
        className={`${dotSize} animate-pulse rounded-full bg-orange-500`}
      ></span>
    );
  } else if (counts.substitutions > 0) {
    return (
      <span
        className={`${dotSize} animate-pulse rounded-full bg-purple-500`}
      ></span>
    );
  }
  return <span className={`${dotSize} rounded-full bg-[#83CD2D]`}></span>;
}

function SubstitutionPageContent() {
  const router = useRouter();
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });

  const { success: showSuccessToast } = useToast();

  // States
  const [teachers, setTeachers] = useState<TeacherAvailability[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [activeSubstitutions, setActiveSubstitutions] = useState<
    Substitution[]
  >([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isMobile, setIsMobile] = useState(false);

  // Popup states
  const [showPopup, setShowPopup] = useState(false);
  const [selectedTeacher, setSelectedTeacher] =
    useState<TeacherAvailability | null>(null);
  const [selectedGroup, setSelectedGroup] = useState("");
  const [substitutionDays, setSubstitutionDays] = useState(1);

  // Confirmation modal states
  const [showEndConfirmation, setShowEndConfirmation] = useState(false);
  const [substitutionToEnd, setSubstitutionToEnd] = useState<{
    id: string;
    groupName: string;
    teacherName: string;
  } | null>(null);

  // Handle mobile detection
  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
    window.addEventListener("resize", checkMobile);
    return () => window.removeEventListener("resize", checkMobile);
  }, []);

  // Fetch teachers data
  const fetchTeachers = useCallback(async (filters?: { search?: string }) => {
    setIsLoading(true);
    setError(null);

    try {
      const availableTeachers =
        await substitutionService.fetchAvailableTeachers(
          new Date(), // Current date
          filters?.search,
        );
      setTeachers(availableTeachers);
    } catch (err) {
      console.error("Error fetching teachers:", err);
      setError("Fehler beim Laden der Lehrerdaten.");
      setTeachers([]);
    } finally {
      setIsLoading(false);
    }
  }, []);

  // Fetch groups data
  const fetchGroups = useCallback(async () => {
    try {
      const allGroups = await groupService.getGroups();
      setGroups(allGroups);
    } catch (err) {
      console.error("Error fetching groups:", err);
      setError("Fehler beim Laden der Gruppendaten.");
      setGroups([]);
    }
  }, []);

  // Fetch active substitutions
  const fetchActiveSubstitutions = useCallback(async () => {
    try {
      const substitutions = await substitutionService.fetchActiveSubstitutions(
        new Date(),
      );
      setActiveSubstitutions(substitutions);
    } catch (err) {
      console.error("Error fetching active substitutions:", err);
      // Don't set error for substitutions, just log it
    }
  }, []);

  // Load initial data
  useEffect(() => {
    void fetchTeachers();
    void fetchGroups();
    void fetchActiveSubstitutions();
  }, [fetchTeachers, fetchGroups, fetchActiveSubstitutions]);

  // Apply filters to teachers
  const filteredTeachers = useMemo(() => {
    let filtered = [...teachers];

    // Apply search filter
    if (searchTerm) {
      const searchLower = searchTerm.toLowerCase();
      filtered = filtered.filter((teacher) => {
        const checks = [
          formatTeacherName(teacher).toLowerCase().includes(searchLower),
          teacher.role?.toLowerCase().includes(searchLower),
          teacher.regularGroup?.toLowerCase().includes(searchLower),
        ];
        return checks.some(Boolean);
      });
    }

    // Apply status filter
    if (statusFilter !== "all") {
      const isInSubstitution = statusFilter === "substitution";
      filtered = filtered.filter(
        (teacher) => teacher.inSubstitution === isInSubstitution,
      );
    }

    return filtered;
  }, [teachers, searchTerm, statusFilter]);

  // Open popup for substitution assignment
  const openSubstitutionPopup = (teacher: TeacherAvailability) => {
    setSelectedTeacher(teacher);
    setSelectedGroup("");
    setSubstitutionDays(1);
    setShowPopup(true);
  };

  // Close popup
  const closePopup = () => {
    setShowPopup(false);
    setSelectedTeacher(null);
  };

  // Handle substitution assignment
  const handleAssignSubstitution = async () => {
    if (!selectedTeacher || !selectedGroup) {
      setError("Bitte wählen Sie eine Gruppe aus.");
      return;
    }

    try {
      setIsLoading(true);
      setError(null);

      // Find the selected group to get its ID
      const group = groups.find((g) => g.name === selectedGroup);
      if (!group) {
        setError("Gruppe nicht gefunden.");
        return;
      }

      // For general group coverage, we don't need to specify who is being replaced
      const regularStaffId = null;

      // Calculate end date based on substitution days
      const startDate = new Date();
      const endDate = new Date();
      endDate.setDate(endDate.getDate() + substitutionDays - 1);

      // Create the substitution
      await substitutionService.createSubstitution(
        group.id,
        regularStaffId,
        selectedTeacher.id,
        startDate,
        endDate,
        "Vertretung", // reason
        `Vertretung für ${substitutionDays} Tag(e)`, // notes
      );

      // Refresh data
      await Promise.all([fetchTeachers(), fetchActiveSubstitutions()]);

      // Show success message (use group.name from the found group)
      const teacherName = formatTeacherName(selectedTeacher);
      const days = substitutionDays > 1 ? `${substitutionDays} Tage` : "1 Tag";
      showSuccessToast(
        `Vertretung für "${group.name}" an ${teacherName} zugewiesen (${days})`,
      );

      closePopup();
    } catch (err) {
      console.error("Error creating substitution:", err);
      setError("Fehler beim Zuweisen der Vertretung.");
    } finally {
      setIsLoading(false);
    }
  };

  // Handle ending substitution - show confirmation first
  const handleEndSubstitutionClick = (
    substitutionId: string,
    groupName: string,
    teacherName: string,
  ) => {
    setSubstitutionToEnd({ id: substitutionId, groupName, teacherName });
    setShowEndConfirmation(true);
  };

  // Confirm and execute ending substitution
  const confirmEndSubstitution = async () => {
    if (!substitutionToEnd) return;

    try {
      setIsLoading(true);
      await substitutionService.deleteSubstitution(substitutionToEnd.id);
      await Promise.all([fetchTeachers(), fetchActiveSubstitutions()]);

      // Show success message
      showSuccessToast(
        `Vertretung für "${substitutionToEnd.groupName}" beendet`,
      );

      setShowEndConfirmation(false);
      setSubstitutionToEnd(null);
    } catch (err) {
      console.error("Error ending substitution:", err);
      setError("Fehler beim Beenden der Vertretung.");
    } finally {
      setIsLoading(false);
    }
  };

  // Prepare filter configurations
  const filterConfigs: FilterConfig[] = useMemo(
    () => [
      {
        id: "status",
        label: "Status",
        type: "buttons",
        value: statusFilter,
        onChange: (value) => setStatusFilter(value as string),
        options: [
          { value: "all", label: "Alle" },
          { value: "available", label: "Verfügbar" },
          { value: "substitution", label: "In Vertretung" },
        ],
      },
    ],
    [statusFilter],
  );

  // Prepare active filters
  const activeFilters: ActiveFilter[] = useMemo(() => {
    const filters: ActiveFilter[] = [];

    if (searchTerm) {
      filters.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    }

    if (statusFilter !== "all") {
      const statusLabels = {
        available: "Verfügbar",
        substitution: "In Vertretung",
      };
      filters.push({
        id: "status",
        label:
          statusLabels[statusFilter as keyof typeof statusLabels] ??
          statusFilter,
        onRemove: () => setStatusFilter("all"),
      });
    }

    return filters;
  }, [searchTerm, statusFilter]);

  if (status === "loading") {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout>
      <div className="-mt-1.5 w-full">
        {/* PageHeaderWithSearch - Title only on mobile */}
        <PageHeaderWithSearch
          title={isMobile ? "Vertretungen" : ""}
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
                  d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
                />
              </svg>
            ),
            count: filteredTeachers.length,
            label: "Fachkräfte",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Fachkraft suchen...",
          }}
          filters={filterConfigs}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
            setStatusFilter("all");
          }}
        />

        {/* Error Alert */}
        {error && (
          <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4">
            <p className="text-sm text-red-800">{error}</p>
          </div>
        )}

        {/* Available Teachers Section */}
        <div className="mb-6">
          <h2 className="mb-3 text-base font-semibold text-gray-900 md:mb-4 md:text-lg">
            Verfügbare pädagogische Fachkräfte
          </h2>

          {isLoading ? (
            <Loading fullPage={false} />
          ) : filteredTeachers.length > 0 ? (
            <div className="space-y-3">
              {filteredTeachers.map((teacher) => (
                <div
                  key={teacher.id}
                  className="group relative cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.99] md:hover:-translate-y-1 md:hover:scale-[1.01] md:hover:border-blue-200/50 md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]"
                >
                  {/* Modern gradient overlay */}
                  <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-br from-blue-50/80 to-cyan-100/80 opacity-[0.03]"></div>
                  {/* Subtle inner glow */}
                  <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                  {/* Modern border highlight */}
                  <div className="pointer-events-none absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-blue-200/60"></div>

                  <div className="relative p-4 md:p-5">
                    {/* Mobile layout - vertical */}
                    <div className="md:hidden">
                      <div className="mb-3 flex items-start gap-3">
                        {/* Teacher initial circle with count badge */}
                        <div className="relative">
                          <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-gray-600 text-base font-semibold text-white shadow-md">
                            {(
                              teacher.firstName?.charAt(0) || "L"
                            ).toUpperCase()}
                          </div>
                          {/* Dual badges: Orange for Tagesübergaben, Purple for Vertretungen - overlapping at top */}
                          <SubstitutionBadges teacher={teacher} />
                        </div>

                        {/* Teacher info */}
                        <div className="min-w-0 flex-1">
                          <h3 className="truncate text-base font-semibold text-gray-900">
                            {formatTeacherName(teacher)}
                          </h3>
                          {teacher.regularGroup && (
                            <div className="mt-0.5 flex items-center text-sm text-gray-500">
                              <svg
                                className="mr-1.5 h-4 w-4 text-gray-400"
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
                              <span className="truncate">
                                {teacher.regularGroup}
                              </span>
                            </div>
                          )}
                          {/* Mobile status indicator - shows both colors if both types */}
                          <div className="mt-1.5 flex items-center gap-1.5">
                            <StatusIndicator teacher={teacher} />
                            <span className="text-xs text-gray-600">
                              {getTeacherStatus(teacher)}
                            </span>
                          </div>
                        </div>
                      </div>

                      {/* Mobile action button - always enabled for multiple assignments */}
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          openSubstitutionPopup(teacher);
                        }}
                        className="w-full rounded-xl border-2 border-gray-400 bg-white px-4 py-2.5 text-sm font-medium text-gray-700 shadow-sm transition-all duration-200 hover:border-gray-500 hover:bg-gray-50 hover:shadow-md active:scale-95"
                      >
                        Zuweisen
                      </button>
                    </div>

                    {/* Desktop layout - horizontal */}
                    <div className="hidden items-center justify-between md:flex">
                      {/* Left content */}
                      <div className="flex min-w-0 flex-1 items-center gap-4">
                        {/* Teacher initial circle with count badge */}
                        <div className="relative">
                          <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-full bg-gray-600 text-lg font-semibold text-white shadow-md">
                            {(
                              teacher.firstName?.charAt(0) || "L"
                            ).toUpperCase()}
                          </div>
                          {/* Dual badges: Orange for Tagesübergaben, Purple for Vertretungen - overlapping at top */}
                          <SubstitutionBadges teacher={teacher} />
                        </div>

                        {/* Teacher info */}
                        <div className="min-w-0 flex-1">
                          <h3 className="truncate text-lg font-semibold text-gray-900 transition-colors duration-300 md:group-hover:text-blue-600">
                            {formatTeacherName(teacher)}
                          </h3>
                          {teacher.regularGroup && (
                            <div className="mt-1 flex items-center text-sm text-gray-500">
                              <svg
                                className="mr-1.5 h-4 w-4 text-gray-400"
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
                              <span className="truncate">
                                {teacher.regularGroup}
                              </span>
                            </div>
                          )}
                        </div>
                      </div>

                      {/* Right content - Status and button */}
                      <div className="ml-4 flex items-center gap-4">
                        {/* Status indicator - shows both colors if both types */}
                        <div className="flex items-center gap-2">
                          <StatusIndicator teacher={teacher} size="large" />
                          <span className="text-sm whitespace-nowrap text-gray-600">
                            {getTeacherStatus(teacher)}
                          </span>
                        </div>

                        {/* Action button - always enabled for multiple assignments */}
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            openSubstitutionPopup(teacher);
                          }}
                          className="rounded-xl border-2 border-gray-400 bg-white px-4 py-2 text-sm font-medium whitespace-nowrap text-gray-700 shadow-sm transition-all duration-200 hover:border-gray-500 hover:bg-gray-50 hover:shadow-md active:scale-95"
                        >
                          Zuweisen
                        </button>
                      </div>
                    </div>
                  </div>

                  {/* Glowing border effect */}
                  <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-blue-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
                </div>
              ))}
            </div>
          ) : (
            <div className="py-12 text-center">
              <div className="flex flex-col items-center gap-4">
                <svg
                  className="h-12 w-12 text-gray-400"
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
                <div>
                  <h3 className="text-lg font-medium text-gray-900">
                    Keine Fachkräfte gefunden
                  </h3>
                  <p className="text-gray-600">
                    Versuche deine Suchkriterien anzupassen.
                  </p>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Active Assignments Section - Split by Type */}
        <div className="space-y-6">
          {/* Day Transfers Section (Tagesübergaben) */}
          {(() => {
            const transfers = activeSubstitutions.filter((s) => s.isTransfer);
            return (
              <div>
                <div className="mb-3 flex items-center gap-2 md:mb-4">
                  <svg
                    className="h-5 w-5 text-orange-500"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  <h2 className="text-base font-semibold text-gray-900 md:text-lg">
                    Tagesübergaben
                  </h2>
                  <span className="rounded-full bg-orange-100 px-2 py-0.5 text-xs font-medium text-orange-700">
                    {transfers.length}
                  </span>
                  <span className="text-xs text-gray-400">
                    (enden heute 23:59)
                  </span>
                </div>

                {transfers.length > 0 ? (
                  <div className="space-y-3">
                    {transfers.map((substitution) => {
                      const group = groups.find(
                        (g) => g.id === substitution.groupId,
                      );
                      if (!group) return null;

                      const substituteName = getSubstituteName(
                        teachers,
                        substitution,
                      );

                      return (
                        <div
                          key={substitution.id}
                          className="group relative overflow-hidden rounded-2xl border border-orange-200/50 bg-gradient-to-br from-orange-50/80 to-amber-50/50 shadow-sm transition-all duration-300 hover:shadow-md"
                        >
                          <div className="relative p-4 md:p-5">
                            {/* Mobile layout */}
                            <div className="md:hidden">
                              <div className="mb-3 flex items-start gap-3">
                                <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-orange-500 text-base font-semibold text-white shadow-md">
                                  {(group.name?.charAt(0) || "G").toUpperCase()}
                                </div>
                                <div className="min-w-0 flex-1">
                                  <h3 className="truncate text-base font-semibold text-gray-900">
                                    {group.name}
                                  </h3>
                                  <p className="mt-1 text-sm text-gray-500">
                                    <span className="text-gray-400">an:</span>{" "}
                                    <span className="font-medium text-gray-700">
                                      {substituteName}
                                    </span>
                                  </p>
                                </div>
                              </div>
                              <button
                                onClick={() =>
                                  handleEndSubstitutionClick(
                                    substitution.id,
                                    group.name,
                                    substituteName,
                                  )
                                }
                                disabled={isLoading}
                                className="w-full rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/10 px-4 py-2.5 text-sm font-medium text-[#FF3130] transition-all duration-200 hover:border-[#FF3130]/30 hover:bg-[#FF3130]/20 active:scale-95"
                              >
                                Beenden
                              </button>
                            </div>

                            {/* Desktop layout */}
                            <div className="hidden items-center justify-between md:flex">
                              <div className="flex min-w-0 flex-1 items-center gap-4">
                                <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-full bg-orange-500 text-lg font-semibold text-white shadow-md">
                                  {(group.name?.charAt(0) || "G").toUpperCase()}
                                </div>
                                <div className="min-w-0 flex-1">
                                  <h3 className="truncate text-lg font-semibold text-gray-900">
                                    {group.name}
                                  </h3>
                                  <p className="mt-1 text-sm text-gray-500">
                                    <span className="text-gray-400">
                                      Übergeben an:
                                    </span>{" "}
                                    <span className="font-medium text-gray-700">
                                      {substituteName}
                                    </span>
                                  </p>
                                </div>
                              </div>
                              <button
                                onClick={() =>
                                  handleEndSubstitutionClick(
                                    substitution.id,
                                    group.name,
                                    substituteName,
                                  )
                                }
                                disabled={isLoading}
                                className="ml-4 rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/10 px-4 py-2 text-sm font-medium whitespace-nowrap text-[#FF3130] transition-all duration-200 hover:border-[#FF3130]/30 hover:bg-[#FF3130]/20 active:scale-95"
                              >
                                Beenden
                              </button>
                            </div>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                ) : (
                  <div className="rounded-xl border border-dashed border-gray-200 bg-gray-50/50 py-6 text-center">
                    <p className="text-sm text-gray-500">
                      Keine aktiven Tagesübergaben
                    </p>
                  </div>
                )}
              </div>
            );
          })()}

          {/* Regular Substitutions Section (Vertretungen) */}
          {(() => {
            const regularSubs = activeSubstitutions.filter(
              (s) => !s.isTransfer,
            );
            return (
              <div>
                <div className="mb-3 flex items-center gap-2 md:mb-4">
                  <svg
                    className="h-5 w-5 text-purple-500"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
                    />
                  </svg>
                  <h2 className="text-base font-semibold text-gray-900 md:text-lg">
                    Vertretungen
                  </h2>
                  <span className="rounded-full bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-700">
                    {regularSubs.length}
                  </span>
                  <span className="text-xs text-gray-400">(mehrtägig)</span>
                </div>

                {regularSubs.length > 0 ? (
                  <div className="space-y-3">
                    {regularSubs.map((substitution) => {
                      const group = groups.find(
                        (g) => g.id === substitution.groupId,
                      );
                      if (!group) return null;

                      const substituteName = getSubstituteName(
                        teachers,
                        substitution,
                      );

                      // Format end date
                      const endDateStr =
                        substitution.endDate.toLocaleDateString("de-DE", {
                          day: "2-digit",
                          month: "2-digit",
                        });

                      return (
                        <div
                          key={substitution.id}
                          className="group relative overflow-hidden rounded-2xl border border-purple-200/50 bg-gradient-to-br from-purple-50/80 to-pink-50/50 shadow-sm transition-all duration-300 hover:shadow-md"
                        >
                          <div className="relative p-4 md:p-5">
                            {/* Mobile layout */}
                            <div className="md:hidden">
                              <div className="mb-3 flex items-start gap-3">
                                <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-purple-500 text-base font-semibold text-white shadow-md">
                                  {(group.name?.charAt(0) || "G").toUpperCase()}
                                </div>
                                <div className="min-w-0 flex-1">
                                  <h3 className="truncate text-base font-semibold text-gray-900">
                                    {group.name}
                                  </h3>
                                  <p className="mt-1 text-sm text-gray-500">
                                    <span className="text-gray-400">
                                      durch:
                                    </span>{" "}
                                    <span className="font-medium text-gray-700">
                                      {substituteName}
                                    </span>
                                  </p>
                                  <p className="mt-0.5 text-xs text-purple-600">
                                    bis {endDateStr}
                                  </p>
                                </div>
                              </div>
                              <button
                                onClick={() =>
                                  handleEndSubstitutionClick(
                                    substitution.id,
                                    group.name,
                                    substituteName,
                                  )
                                }
                                disabled={isLoading}
                                className="w-full rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/10 px-4 py-2.5 text-sm font-medium text-[#FF3130] transition-all duration-200 hover:border-[#FF3130]/30 hover:bg-[#FF3130]/20 active:scale-95"
                              >
                                Beenden
                              </button>
                            </div>

                            {/* Desktop layout */}
                            <div className="hidden items-center justify-between md:flex">
                              <div className="flex min-w-0 flex-1 items-center gap-4">
                                <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-full bg-purple-500 text-lg font-semibold text-white shadow-md">
                                  {(group.name?.charAt(0) || "G").toUpperCase()}
                                </div>
                                <div className="min-w-0 flex-1">
                                  <div className="flex items-center gap-2">
                                    <h3 className="truncate text-lg font-semibold text-gray-900">
                                      {group.name}
                                    </h3>
                                    <span className="rounded-full bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-700">
                                      bis {endDateStr}
                                    </span>
                                  </div>
                                  <p className="mt-1 text-sm text-gray-500">
                                    <span className="text-gray-400">
                                      Vertretung durch:
                                    </span>{" "}
                                    <span className="font-medium text-gray-700">
                                      {substituteName}
                                    </span>
                                  </p>
                                </div>
                              </div>
                              <button
                                onClick={() =>
                                  handleEndSubstitutionClick(
                                    substitution.id,
                                    group.name,
                                    substituteName,
                                  )
                                }
                                disabled={isLoading}
                                className="ml-4 rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/10 px-4 py-2 text-sm font-medium whitespace-nowrap text-[#FF3130] transition-all duration-200 hover:border-[#FF3130]/30 hover:bg-[#FF3130]/20 active:scale-95"
                              >
                                Beenden
                              </button>
                            </div>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                ) : (
                  <div className="rounded-xl border border-dashed border-gray-200 bg-gray-50/50 py-6 text-center">
                    <p className="text-sm text-gray-500">
                      Keine aktiven Vertretungen
                    </p>
                  </div>
                )}
              </div>
            );
          })()}
        </div>
      </div>

      {/* Substitution Assignment Modal */}
      <Modal
        isOpen={showPopup}
        onClose={closePopup}
        title="Vertretung zuweisen"
      >
        {error && <Alert type="error" message={error} />}

        <div className="space-y-4">
          <div>
            <p className="mb-2 text-sm font-medium text-gray-700">
              Pädagogische Fachkraft:
            </p>
            <p className="font-semibold text-gray-900">
              {selectedTeacher ? formatTeacherName(selectedTeacher) : ""}
            </p>
          </div>

          {/* Group selection */}
          <div>
            <label
              htmlFor="substitution-group-select"
              className="mb-2 block text-sm font-medium text-gray-700"
            >
              OGS-Gruppe auswählen
            </label>
            <div className="relative">
              <select
                id="substitution-group-select"
                value={selectedGroup}
                onChange={(e) => setSelectedGroup(e.target.value)}
                className="block w-full cursor-pointer appearance-none rounded-lg border border-gray-200 bg-white py-3 pr-10 pl-4 text-lg text-gray-900 transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
              >
                <option value="">Gruppe auswählen...</option>
                {groups.map((group) => (
                  <option key={group.id} value={group.name}>
                    {group.name}
                  </option>
                ))}
              </select>
              {/* Custom dropdown arrow */}
              <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
                <svg
                  className="h-5 w-5 text-gray-400"
                  viewBox="0 0 20 20"
                  fill="currentColor"
                >
                  <path
                    fillRule="evenodd"
                    d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
                    clipRule="evenodd"
                  />
                </svg>
              </div>
            </div>
          </div>

          {/* Days selection with stepper */}
          <div>
            <label
              htmlFor="substitution-days-input"
              className="mb-2 block text-sm font-medium text-gray-700"
            >
              Anzahl der Tage
            </label>
            <div className="flex items-center justify-center gap-3">
              {/* Minus button */}
              <button
                type="button"
                onClick={() =>
                  setSubstitutionDays((prev) => Math.max(1, prev - 1))
                }
                disabled={substitutionDays <= 1}
                className="flex h-12 w-12 items-center justify-center rounded-xl border-2 border-gray-300 bg-white text-gray-600 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 active:scale-95 disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:border-gray-300 disabled:hover:bg-white"
                aria-label="Tage verringern"
              >
                <svg
                  className="h-5 w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2.5}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M20 12H4"
                  />
                </svg>
              </button>

              {/* Editable input */}
              <input
                id="substitution-days-input"
                type="text"
                inputMode="numeric"
                pattern="[0-9]*"
                value={substitutionDays}
                onChange={(e) => {
                  const val = e.target.value.replaceAll(/\D/g, "");
                  if (val === "") {
                    setSubstitutionDays(1);
                  } else {
                    const num = Number.parseInt(val, 10);
                    if (num >= 1 && num <= 365) {
                      setSubstitutionDays(num);
                    }
                  }
                }}
                onBlur={() => {
                  if (substitutionDays < 1) {
                    setSubstitutionDays(1);
                  }
                }}
                className="w-20 rounded-xl border-2 border-gray-200 bg-white py-3 text-center text-xl font-semibold text-gray-900 transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] focus:outline-none"
              />

              {/* Plus button */}
              <button
                type="button"
                onClick={() =>
                  setSubstitutionDays((prev) => Math.min(365, prev + 1))
                }
                disabled={substitutionDays >= 365}
                className="flex h-12 w-12 items-center justify-center rounded-xl border-2 border-gray-300 bg-white text-gray-600 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 active:scale-95 disabled:cursor-not-allowed disabled:opacity-40"
                aria-label="Tage erhöhen"
              >
                <svg
                  className="h-5 w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2.5}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M12 4v16m8-8H4"
                  />
                </svg>
              </button>
            </div>
            <p className="mt-2 text-center text-xs text-gray-500">
              {substitutionDays === 1
                ? "Vertretung für heute"
                : `Vertretung für ${substitutionDays} Tage`}
            </p>
          </div>

          {/* Action Buttons */}
          <div className="flex gap-3 pt-4">
            <button
              type="button"
              onClick={closePopup}
              className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100"
            >
              Abbrechen
            </button>

            <button
              type="button"
              onClick={handleAssignSubstitution}
              disabled={isLoading}
              className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-gray-700 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:scale-100"
            >
              {isLoading ? (
                <span className="flex items-center justify-center gap-2">
                  <svg
                    className="h-4 w-4 animate-spin text-white"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <circle
                      className="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      strokeWidth="4"
                    ></circle>
                    <path
                      className="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    ></path>
                  </svg>
                  Wird zugewiesen...
                </span>
              ) : (
                "Zuweisen"
              )}
            </button>
          </div>
        </div>
      </Modal>

      {/* End Substitution Confirmation Modal */}
      <ConfirmationModal
        isOpen={showEndConfirmation}
        onClose={() => {
          setShowEndConfirmation(false);
          setSubstitutionToEnd(null);
        }}
        onConfirm={confirmEndSubstitution}
        title="Vertretung beenden?"
        confirmText="Beenden"
        cancelText="Abbrechen"
        isConfirmLoading={isLoading}
        confirmButtonClass="bg-[#FF3130] hover:bg-[#FF3130]/90"
      >
        {substitutionToEnd && (
          <div className="space-y-2">
            <p className="text-gray-700">
              Möchtest du die Vertretung wirklich beenden?
            </p>
            <div className="mt-4 rounded-lg border border-gray-200 bg-gray-50 p-4">
              <p className="mb-1 text-sm text-gray-600">
                <span className="font-medium text-gray-900">Gruppe:</span>{" "}
                {substitutionToEnd.groupName}
              </p>
              <p className="text-sm text-gray-600">
                <span className="font-medium text-gray-900">
                  Vertretung durch:
                </span>{" "}
                {substitutionToEnd.teacherName}
              </p>
            </div>
          </div>
        )}
      </ConfirmationModal>
    </ResponsiveLayout>
  );
}

// Main component with Suspense wrapper
export default function SubstitutionPage() {
  return (
    <Suspense
      fallback={
        <ResponsiveLayout>
          <Loading fullPage={false} />
        </ResponsiveLayout>
      }
    >
      <SubstitutionPageContent />
    </Suspense>
  );
}
