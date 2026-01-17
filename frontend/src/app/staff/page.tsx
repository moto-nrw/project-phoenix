"use client";

import { useState, useEffect, Suspense, useMemo } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
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

  // Helper to check if location matches filter
  const matchesLocationFilter = (location: string, filter: string): boolean => {
    if (filter === "all") return true;
    if (filter === "zuhause") return location === "Zuhause";
    if (filter === "anwesend") return location === "Anwesend";
    if (filter === "schulhof") return location === "Schulhof";
    if (filter === "unterwegs") return location === "Unterwegs";
    if (filter === "im_raum") {
      // Staff actively supervising in a room (not Zuhause, Anwesend, Schulhof, or Unterwegs)
      return (
        location !== "Zuhause" &&
        location !== "Anwesend" &&
        location !== "Schulhof" &&
        location !== "Unterwegs"
      );
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
                    className={`group relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500`}
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
    </ResponsiveLayout>
  );
}

// Main component with Suspense wrapper
export default function StaffPage() {
  return (
    <Suspense
      fallback={
        <ResponsiveLayout>
          <Loading fullPage={false} />
        </ResponsiveLayout>
      }
    >
      <StaffPageContent />
    </Suspense>
  );
}
