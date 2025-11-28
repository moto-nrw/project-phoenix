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
} from "~/lib/substitution-helpers";

import { Loading } from "~/components/ui/loading";
import { useToast } from "~/contexts/ToastContext";

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
                        {/* Teacher initial circle */}
                        <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-gray-600 text-base font-semibold text-white shadow-md">
                          {(teacher.firstName?.charAt(0) || "L").toUpperCase()}
                        </div>

                        {/* Teacher info */}
                        <div className="min-w-0 flex-1">
                          <h3 className="truncate text-base font-semibold text-gray-900">
                            {formatTeacherName(teacher)}
                          </h3>
                          <p className="mt-0.5 truncate text-sm text-gray-500">
                            {teacher.role}
                          </p>
                          {teacher.regularGroup && (
                            <p className="mt-0.5 truncate text-xs text-gray-400">
                              {teacher.regularGroup}
                            </p>
                          )}
                          {/* Mobile status indicator */}
                          <div className="mt-1.5 flex items-center gap-1.5">
                            <span
                              className={`h-2 w-2 rounded-full ${teacher.inSubstitution ? "animate-pulse bg-orange-500" : "bg-[#83CD2D]"}`}
                            ></span>
                            <span className="text-xs text-gray-600">
                              {getTeacherStatus(teacher)}
                            </span>
                          </div>
                        </div>
                      </div>

                      {/* Mobile action button */}
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          openSubstitutionPopup(teacher);
                        }}
                        disabled={teacher.inSubstitution}
                        className={`w-full rounded-xl px-4 py-2.5 text-sm font-medium shadow-sm transition-all duration-200 ${
                          teacher.inSubstitution
                            ? "cursor-not-allowed bg-gray-100 text-gray-500"
                            : "border-2 border-gray-400 bg-white text-gray-700 hover:border-gray-500 hover:bg-gray-50 hover:shadow-md active:scale-95"
                        }`}
                      >
                        {teacher.inSubstitution ? "In Vertretung" : "Zuweisen"}
                      </button>
                    </div>

                    {/* Desktop layout - horizontal */}
                    <div className="hidden items-center justify-between md:flex">
                      {/* Left content */}
                      <div className="flex min-w-0 flex-1 items-center gap-4">
                        {/* Teacher initial circle */}
                        <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-full bg-gray-600 text-lg font-semibold text-white shadow-md">
                          {(teacher.firstName?.charAt(0) || "L").toUpperCase()}
                        </div>

                        {/* Teacher info */}
                        <div className="min-w-0 flex-1">
                          <h3 className="truncate text-lg font-semibold text-gray-900 transition-colors duration-300 md:group-hover:text-blue-600">
                            {formatTeacherName(teacher)}
                          </h3>
                          <div className="mt-1 flex items-center gap-3">
                            <p className="truncate text-sm text-gray-500">
                              {teacher.role}
                            </p>
                            {teacher.regularGroup && (
                              <>
                                <span className="text-gray-300">•</span>
                                <p className="truncate text-sm text-gray-500">
                                  {teacher.regularGroup}
                                </p>
                              </>
                            )}
                          </div>
                        </div>
                      </div>

                      {/* Right content - Status and button */}
                      <div className="ml-4 flex items-center gap-4">
                        {/* Status indicator */}
                        <div className="flex items-center gap-2">
                          <span
                            className={`h-2.5 w-2.5 rounded-full ${teacher.inSubstitution ? "animate-pulse bg-orange-500" : "bg-[#83CD2D]"}`}
                          ></span>
                          <span className="text-sm whitespace-nowrap text-gray-600">
                            {getTeacherStatus(teacher)}
                          </span>
                        </div>

                        {/* Action button */}
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            openSubstitutionPopup(teacher);
                          }}
                          disabled={teacher.inSubstitution}
                          className={`rounded-xl px-4 py-2 text-sm font-medium whitespace-nowrap shadow-sm transition-all duration-200 ${
                            teacher.inSubstitution
                              ? "cursor-not-allowed bg-gray-100 text-gray-500"
                              : "border-2 border-gray-400 bg-white text-gray-700 hover:border-gray-500 hover:bg-gray-50 hover:shadow-md active:scale-95"
                          }`}
                        >
                          {teacher.inSubstitution
                            ? "In Vertretung"
                            : "Zuweisen"}
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

        {/* Current Substitutions Section */}
        <div>
          <h2 className="mb-3 text-base font-semibold text-gray-900 md:mb-4 md:text-lg">
            Aktuelle Vertretungen
          </h2>

          {activeSubstitutions.length > 0 ? (
            <div className="space-y-3">
              {activeSubstitutions.map((substitution) => {
                // Find the group for this substitution
                const group = groups.find((g) => g.id === substitution.groupId);
                if (!group) return null;

                // Find the substitute teacher name from the teachers list
                const substituteTeacher = teachers.find(
                  (t) => t.id === substitution.substituteStaffId,
                );
                const substituteName = substituteTeacher
                  ? formatTeacherName(substituteTeacher)
                  : (substitution.substituteStaffName ?? "Unbekannt");

                return (
                  <div
                    key={substitution.id}
                    className="group relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500"
                  >
                    {/* Modern gradient overlay */}
                    <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-br from-purple-50/80 to-pink-100/80 opacity-[0.03]"></div>
                    {/* Subtle inner glow */}
                    <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                    {/* Modern border highlight */}
                    <div className="pointer-events-none absolute inset-0 rounded-3xl ring-1 ring-white/20"></div>

                    <div className="relative p-4 md:p-5">
                      {/* Mobile layout */}
                      <div className="md:hidden">
                        <div className="mb-3 flex items-start gap-3">
                          {/* Group initial circle */}
                          <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-[#8B5CF6] text-base font-semibold text-white shadow-md">
                            {(group.name?.charAt(0) || "G").toUpperCase()}
                          </div>

                          {/* Substitution info */}
                          <div className="min-w-0 flex-1">
                            <h3 className="truncate text-base font-semibold text-gray-900">
                              {group.name}
                            </h3>
                            <p className="mt-1 text-sm text-gray-500">
                              <span className="text-gray-400">durch:</span>{" "}
                              <span className="font-medium text-gray-700">
                                {substituteName}
                              </span>
                            </p>
                          </div>
                        </div>

                        {/* Mobile action button */}
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
                        {/* Left content */}
                        <div className="flex min-w-0 flex-1 items-center gap-4">
                          {/* Group initial circle */}
                          <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-full bg-[#8B5CF6] text-lg font-semibold text-white shadow-md">
                            {(group.name?.charAt(0) || "G").toUpperCase()}
                          </div>

                          {/* Substitution info */}
                          <div className="min-w-0 flex-1">
                            <h3 className="truncate text-lg font-semibold text-gray-900">
                              {group.name}
                            </h3>
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

                        {/* Right content - End button */}
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
                    d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                  />
                </svg>
                <div>
                  <h3 className="text-lg font-medium text-gray-900">
                    Keine aktiven Vertretungen
                  </h3>
                  <p className="text-gray-600">
                    Aktuell sind keine Vertretungen zugewiesen.
                  </p>
                </div>
              </div>
            </div>
          )}
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
            <label className="mb-2 block text-sm font-medium text-gray-700">
              OGS-Gruppe auswählen
            </label>
            <div className="relative">
              <select
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

          {/* Days selection */}
          <div>
            <label className="mb-2 block text-sm font-medium text-gray-700">
              Anzahl der Tage
            </label>
            <input
              type="number"
              min="1"
              max="30"
              value={substitutionDays}
              onChange={(e) =>
                setSubstitutionDays(parseInt(e.target.value) || 1)
              }
              className="block w-full rounded-lg border border-gray-200 bg-white px-4 py-3 text-base text-gray-900 transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
            />
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
