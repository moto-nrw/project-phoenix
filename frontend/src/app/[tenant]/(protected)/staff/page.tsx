"use client";

import { useState, useEffect, useMemo } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import { staffService } from "~/lib/staff-api";
import type { Staff } from "~/lib/staff-api";
import {
  getStaffDisplayType,
  getStaffCardInfo,
  formatStaffNotes,
  sortStaff,
  getStaffLocationStatus,
} from "~/lib/staff-helpers";
import { useSWRAuth } from "~/lib/swr";
import { useTenantRouter } from "~/lib/tenant-router";
import { isAdmin } from "~/lib/auth-utils";
import {
  StaffPendingInbox,
  useStaffPendingInbox,
} from "~/components/staff/staff-pending-inbox";
import { StaffPageSkeleton } from "./page-skeleton";

function StaffPageContent() {
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const router = useTenantRouter();
  const userIsAdmin = isAdmin(session);

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
  // Global SSE in TenantAuthWrapper handles cache invalidation automatically
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

  // Open absence requests (#1419): feeds the inbox above the grid and the
  // per-card pending indicators. Fetches only with vacation:approve.
  const {
    rows: pendingAbsences,
    canReview: canReviewAbsences,
    refresh: refreshPendingAbsences,
  } = useStaffPendingInbox();
  const pendingByStaff = useMemo(() => {
    const map = new Map<number, number>();
    for (const row of pendingAbsences) {
      map.set(row.staff_id, (map.get(row.staff_id) ?? 0) + 1);
    }
    return map;
  }, [pendingAbsences]);

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

  // All absence location labels for the unified "Abwesend" filter
  const absenceLocations = new Set([
    "Krank",
    "Urlaub",
    "Fortbildung",
    "Abwesend",
  ]);

  // Helper to check if location matches filter
  const matchesLocationFilter = (location: string, filter: string): boolean => {
    if (filter === "all") return true;
    // "abwesend_nicht_eingecheckt" - not clocked in (Zuhause or Abwesend)
    if (filter === "abwesend_nicht_eingecheckt")
      return location === "Zuhause" || location === "Abwesend";
    if (filter === "anwesend") return location === "Anwesend";
    if (filter === "homeoffice") return location === "Homeoffice";
    // "abwesend" - all absence types (Krank, Urlaub, Fortbildung)
    if (filter === "abwesend") return absenceLocations.has(location);
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
    const location = staffMember.currentLocation ?? "Abwesend";
    return matchesLocationFilter(location, locationFilter);
  });

  // Prepare filter configurations for PageHeaderWithSearch
  const filterConfigs: FilterConfig[] = useMemo(
    () => [
      {
        id: "location",
        label: "Status",
        type: "grid",
        value: locationFilter,
        onChange: (value) => setLocationFilter(value as string),
        options: [
          { value: "all", label: "Alle", icon: "M4 6h16M4 12h16M4 18h16" },
          {
            value: "anwesend",
            label: "Anwesend",
            icon: "M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z",
          },
          {
            value: "abwesend_nicht_eingecheckt",
            label: "Abwesend",
            icon: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6",
          },
          {
            value: "im_raum",
            label: "Im Raum",
            icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4",
          },
          {
            value: "homeoffice",
            label: "Homeoffice",
            icon: "M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z",
          },
          {
            value: "abwesend",
            label: "Krank/Urlaub",
            icon: "M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636",
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
        anwesend: "Anwesend",
        abwesend_nicht_eingecheckt: "Abwesend",
        im_raum: "Im Raum",
        homeoffice: "Homeoffice",
        abwesend: "Krank/Urlaub",
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
    return <StaffPageSkeleton />;
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

      {/* Eingehende Anfragen (#1419) — only for approvers */}
      {canReviewAbsences && (
        <StaffPendingInbox
          rows={pendingAbsences}
          staffList={staff}
          onRefresh={() => {
            refreshPendingAbsences();
          }}
        />
      )}

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
              const supervisions = staffMember.supervisions ?? [];

              const cardClassName = `group moto-content-surface moto-hover-elevated relative w-full overflow-hidden rounded-2xl border border-gray-200 bg-white text-left shadow-[0_1px_2px_rgba(15,23,42,0.04),0_0_0_1px_rgba(15,23,42,0.02)] focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:ring-offset-2 focus-visible:outline-none active:shadow-[0_10px_26px_rgba(15,23,42,0.1)] ${userIsAdmin ? "cursor-pointer" : ""}`;
              const navigateToStaff = () =>
                router.push(`/staff/${staffMember.id}`);

              return (
                <div
                  key={staffMember.id}
                  {...(userIsAdmin
                    ? {
                        role: "button" as const,
                        tabIndex: 0,
                        onClick: navigateToStaff,
                        onKeyDown: (e: React.KeyboardEvent) => {
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            navigateToStaff();
                          }
                        },
                      }
                    : {})}
                  className={cardClassName}
                >
                  <div className="relative p-5 pb-4">
                    <div className="pointer-events-none absolute inset-0 rounded-2xl ring-1 ring-transparent transition-[box-shadow] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] md:group-hover:shadow-[inset_0_1px_0_rgba(255,255,255,0.9)]" />

                    <div className="relative flex min-h-[112px] flex-col">
                      <div className="mb-3 flex items-start justify-between gap-3">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <h3 className="inline-block origin-left overflow-hidden text-lg font-bold text-ellipsis whitespace-nowrap text-gray-800 transition-[color,transform] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] motion-reduce:transition-none md:group-hover:scale-[1.025] md:group-hover:text-gray-950 motion-reduce:md:group-hover:scale-100">
                              {staffMember.firstName}
                            </h3>
                            {userIsAdmin && (
                              <svg
                                className="h-4 w-4 flex-shrink-0 translate-x-0 text-gray-300 opacity-70 transition-[color,opacity,transform] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] motion-reduce:transition-none md:group-hover:translate-x-0.5 md:group-hover:text-gray-600 md:group-hover:opacity-100 motion-reduce:md:group-hover:translate-x-0"
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
                            )}
                          </div>
                          <p className="overflow-hidden text-base font-semibold text-ellipsis whitespace-nowrap text-gray-700 transition-colors duration-300 md:group-hover:text-gray-800">
                            {staffMember.lastName}
                          </p>
                          <p className="mt-1 text-xs text-gray-400 transition-colors duration-300 md:group-hover:text-gray-500">
                            {staffMember.specialization ?? displayType}
                          </p>
                        </div>

                        <span className="flex flex-shrink-0 flex-col items-end gap-1.5">
                          <span
                            className={`inline-flex items-center rounded-full px-3 py-1.5 text-xs font-bold ${locationStatus.badgeColor}`}
                            style={{
                              backgroundColor: locationStatus.customBgColor,
                              boxShadow: locationStatus.customShadow,
                            }}
                          >
                            <span className="mr-2 h-1.5 w-1.5 animate-pulse rounded-full bg-white/80"></span>
                            {locationStatus.label}
                          </span>
                          {(pendingByStaff.get(Number(staffMember.id)) ?? 0) >
                            0 && (
                            <span className="inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-800">
                              {pendingByStaff.get(Number(staffMember.id))}{" "}
                              {pendingByStaff.get(Number(staffMember.id)) === 1
                                ? "Anfrage"
                                : "Anfragen"}
                            </span>
                          )}
                        </span>
                      </div>

                      <div className="flex-1 space-y-2">
                        {supervisions.length > 0 && (
                          <div className="text-sm text-gray-600">
                            <span className="font-medium">
                              Aktuelle Aufsicht:
                            </span>
                            <ul className="mt-0.5 list-inside">
                              {supervisions.map((supervision) => (
                                <li
                                  key={`${supervision.roomId}-${supervision.activeGroupId}`}
                                >
                                  • {supervision.roomName}
                                </li>
                              ))}
                            </ul>
                          </div>
                        )}

                        {cardInfo.length > 0 && (
                          <div className="flex flex-wrap gap-2">
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

                        {notes && (
                          <p className="text-xs text-gray-500 italic transition-colors duration-300 md:group-hover:text-gray-600">
                            {notes}
                          </p>
                        )}
                      </div>

                      {userIsAdmin && (
                        <p className="mt-2 text-xs text-gray-400 transition-colors duration-150 md:group-hover:text-gray-500">
                          Tippen für mehr Infos
                        </p>
                      )}

                      <div className="absolute right-3 bottom-3 h-3 w-3 rounded-full bg-white/30"></div>
                    </div>
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

export default function StaffPage() {
  return <StaffPageContent />;
}
