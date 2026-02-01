"use client";

import { useState, useEffect, Suspense, useMemo } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header";
import { staffService } from "~/lib/staff-api";
import type { Staff } from "~/lib/staff-api";
import {
  getStaffLocationStatus,
  getStaffDisplayType,
  getStaffCardInfo,
  formatStaffNotes,
  sortStaff,
} from "~/lib/staff-helpers";
import { useSWRAuth } from "~/lib/swr";

import { Loading } from "~/components/ui/loading";
function StaffPageContent() {
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // State variables for filters
  const [searchTerm, setSearchTerm] = useState("");
  const [locationFilter, setLocationFilter] = useState("all");
  const [isMobile, setIsMobile] = useState(false);

  // Handle mobile detection
  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
    window.addEventListener("resize", checkMobile);
    return () => window.removeEventListener("resize", checkMobile);
  }, []);

  // Fetch staff data with SWR (automatic caching, deduplication, revalidation)
  // Global SSE in AuthWrapper handles cache invalidation automatically
  const {
    data: staffData,
    isLoading,
    error: staffError,
  } = useSWRAuth<Staff[]>(
    "staff-list",
    async () => {
      const staffData = await staffService.getAllStaff({});
      return sortStaff(staffData);
    },
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
    },
  );

  const staff = staffData ?? [];
  const error = staffError ? "Fehler beim Laden der Personaldaten." : null;

  // Known non-room locations for the "Im Raum" filter
  const nonRoomLocations = new Set([
    "Zuhause",
    "Anwesend",
    "Schulhof",
    "Unterwegs",
    "Homeoffice",
    "Krank",
    "Urlaub",
    "Fortbildung",
    "Abwesend",
  ]);

  // Helper to check if location matches filter
  const matchesLocationFilter = (location: string, filter: string): boolean => {
    if (filter === "all") return true;
    if (filter === "zuhause") return location === "Zuhause";
    if (filter === "anwesend") return location === "Anwesend";
    if (filter === "schulhof") return location === "Schulhof";
    if (filter === "unterwegs") return location === "Unterwegs";
    if (filter === "homeoffice") return location === "Homeoffice";
    if (filter === "krank") return location === "Krank";
    if (filter === "urlaub") return location === "Urlaub";
    if (filter === "fortbildung") return location === "Fortbildung";
    if (filter === "im_raum") {
      // Staff actively supervising in a room
      return !nonRoomLocations.has(location);
    }
    return true;
  };

  // Apply client-side filters
  const filteredStaff = staff.filter((staffMember) => {
    // Search filter
    if (searchTerm) {
      const searchLower = searchTerm.toLowerCase();
      const matchesSearch =
        staffMember.firstName.toLowerCase().includes(searchLower) ||
        staffMember.lastName.toLowerCase().includes(searchLower) ||
        staffMember.name.toLowerCase().includes(searchLower);

      if (!matchesSearch) return false;
    }

    // Location filter
    const location = staffMember.currentLocation ?? "Zuhause";
    return matchesLocationFilter(location, locationFilter);
  });

  // Prepare filter configurations for PageHeaderWithSearch
  const filterConfigs: FilterConfig[] = useMemo(
    () => [
      {
        id: "location",
        label: "Aufenthaltsort",
        type: "grid",
        value: locationFilter,
        onChange: (value) => setLocationFilter(value as string),
        options: [
          { value: "all", label: "Alle Orte", icon: "M4 6h16M4 12h16M4 18h16" },
          {
            value: "zuhause",
            label: "Zuhause",
            icon: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6",
          },
          {
            value: "anwesend",
            label: "Anwesend",
            icon: "M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z",
          },
          {
            value: "im_raum",
            label: "Im Raum",
            icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4",
          },
          {
            value: "schulhof",
            label: "Schulhof",
            icon: "M21 12a9 9 0 11-18 0 9 9 0 0118 0zM12 12a8 8 0 008 4M7.5 13.5a12 12 0 008.5 6.5M12 12a8 8 0 00-7.464 4.928M12.951 7.353a12 12 0 00-9.88 4.111M12 12a8 8 0 00-.536-8.928M15.549 15.147a12 12 0 001.38-10.611",
          },
          {
            value: "unterwegs",
            label: "Unterwegs",
            icon: "M13 10V3L4 14h7v7l9-11h-7z",
          },
          {
            value: "homeoffice",
            label: "Homeoffice",
            icon: "M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z",
          },
          {
            value: "krank",
            label: "Krank",
            icon: "M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z",
          },
          {
            value: "urlaub",
            label: "Urlaub",
            icon: "M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z",
          },
          {
            value: "fortbildung",
            label: "Fortbildung",
            icon: "M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253",
          },
        ],
      },
    ],
    [locationFilter],
  );

  // Prepare active filters for display
  const activeFilters: ActiveFilter[] = useMemo(() => {
    const filters: ActiveFilter[] = [];

    if (searchTerm) {
      filters.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    }

    if (locationFilter !== "all") {
      const locationLabels: Record<string, string> = {
        zuhause: "Zuhause",
        anwesend: "Anwesend",
        im_raum: "Im Raum",
        schulhof: "Schulhof",
        unterwegs: "Unterwegs",
        homeoffice: "Homeoffice",
        krank: "Krank",
        urlaub: "Urlaub",
        fortbildung: "Fortbildung",
      };
      filters.push({
        id: "location",
        label: locationLabels[locationFilter] ?? locationFilter,
        onRemove: () => setLocationFilter("all"),
      });
    }

    return filters;
  }, [searchTerm, locationFilter]);

  if (status === "loading" || isLoading) {
    return <Loading fullPage={false} />;
  }

  return (
    <div className="-mt-1.5 w-full">
      {/* PageHeaderWithSearch - Title only on mobile */}
      <PageHeaderWithSearch
        title={isMobile ? "Mitarbeiter" : ""}
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
                d="M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2"
              />
            </svg>
          ),
          count: filteredStaff.length,
        }}
        search={{
          value: searchTerm,
          onChange: setSearchTerm,
          placeholder: "Name suchen...",
        }}
        filters={filterConfigs}
        activeFilters={activeFilters}
        onClearAllFilters={() => {
          setSearchTerm("");
          setLocationFilter("all");
        }}
      />

      {/* Error Display */}
      {error && (
        <div className="mb-4 rounded-lg border border-red-200 bg-red-50 p-4 text-red-800">
          {error}
        </div>
      )}

      {/* Staff Grid */}
      {filteredStaff.length === 0 ? (
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
                d="M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2"
              />
            </svg>
            <div>
              <h3 className="text-lg font-medium text-gray-900">
                Kein Personal gefunden
              </h3>
              <p className="text-gray-600">
                Versuchen Sie Ihre Suchkriterien anzupassen.
              </p>
            </div>
          </div>
        </div>
      ) : (
        <div>
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3">
            {filteredStaff.map((staffMember) => {
              const locationStatus = getStaffLocationStatus(staffMember);
              const displayType = getStaffDisplayType(staffMember);
              const cardInfo = getStaffCardInfo(staffMember);
              const notes = formatStaffNotes(staffMember.staffNotes, 80);

              return (
                <div
                  key={staffMember.id}
                  className={`group relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-150`}
                >
                  {/* Modern gradient overlay */}
                  <div
                    className={`absolute inset-0 bg-gradient-to-br ${locationStatus.cardGradient} rounded-3xl opacity-[0.03]`}
                  ></div>
                  {/* Subtle inner glow */}
                  <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                  {/* Modern border highlight */}
                  <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20"></div>

                  <div className="relative p-6">
                    {/* Header with staff name */}
                    <div className="mb-2 flex items-center justify-between">
                      {/* Staff Name */}
                      <div className="min-w-0 flex-1">
                        <h3 className="overflow-hidden text-lg font-bold text-ellipsis whitespace-nowrap text-gray-800">
                          {staffMember.firstName}
                        </h3>
                        <p className="overflow-hidden text-base font-semibold text-ellipsis whitespace-nowrap text-gray-700">
                          {staffMember.lastName}
                        </p>
                        {/* Role/Specialization in same style as "Nur zur Information" */}
                        <p className="mt-1 text-xs text-gray-400">
                          {staffMember.specialization ?? displayType}
                        </p>
                      </div>

                      {/* Status Badge */}
                      <span
                        className={`inline-flex items-center rounded-full px-3 py-1.5 text-xs font-bold ${locationStatus.badgeColor} ml-3`}
                        style={{
                          backgroundColor: locationStatus.customBgColor,
                          boxShadow: locationStatus.customShadow,
                        }}
                      >
                        <span className="mr-2 h-1.5 w-1.5 animate-pulse rounded-full bg-white/80"></span>
                        {locationStatus.label}
                      </span>
                    </div>

                    {/* Additional Info */}
                    {cardInfo.length > 0 && (
                      <div className="mb-2 flex flex-wrap gap-2">
                        {cardInfo.map((info) => (
                          <span
                            key={info}
                            className="inline-flex items-center rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700"
                          >
                            {info}
                          </span>
                        ))}
                      </div>
                    )}

                    {/* Staff Notes (if available) */}
                    {notes && (
                      <p className="mt-2 text-xs text-gray-500 italic">
                        {notes}
                      </p>
                    )}

                    {/* Decorative elements */}
                    <div className="absolute top-3 left-3 h-5 w-5 animate-ping rounded-full bg-white/20"></div>
                    <div className="absolute right-3 bottom-3 h-3 w-3 rounded-full bg-white/30"></div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}

// Main component with Suspense wrapper
export default function StaffPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <StaffPageContent />
    </Suspense>
  );
}
