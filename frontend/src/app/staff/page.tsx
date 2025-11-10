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
  sortStaff 
} from "~/lib/staff-helpers";

import { Loading } from "~/components/ui/loading";
function StaffPageContent() {
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // State variables
  const [staff, setStaff] = useState<Staff[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [locationFilter, setLocationFilter] = useState("all");
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isMobile, setIsMobile] = useState(false);

  // Handle mobile detection
  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
    window.addEventListener('resize', checkMobile);
    return () => window.removeEventListener('resize', checkMobile);
  }, []);

  // Fetch staff data once on mount
  useEffect(() => {
    const fetchStaffData = async () => {
      try {
        setIsLoading(true);

        // Load all staff without search filter (client-side filtering below)
        const staffData = await staffService.getAllStaff({});
        const sortedStaff = sortStaff(staffData);
        setStaff(sortedStaff);
        setError(null);
      } catch (err) {
        console.error("Error fetching staff data:", err);
        setError("Fehler beim Laden der Personaldaten.");
      } finally {
        setIsLoading(false);
      }
    };

    if (session?.user?.token) {
      void fetchStaffData();
    }
  }, [session?.user?.token]); // Only fetch once on mount

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
    if (locationFilter !== "all") {
      const location = staffMember.currentLocation ?? "Zuhause";
      
      switch (locationFilter) {
        case "zuhause":
          if (location !== "Zuhause") return false;
          break;
        case "im_raum":
          if (!location || location === "Zuhause" || location === "Schulhof" || location === "Unterwegs") return false;
          break;
        case "schulhof":
          if (location !== "Schulhof") return false;
          break;
        case "unterwegs":
          if (location !== "Unterwegs") return false;
          break;
      }
    }
    
    return true;
  });

  // Prepare filter configurations for PageHeaderWithSearch
  const filterConfigs: FilterConfig[] = useMemo(() => [
    {
      id: 'location',
      label: 'Aufenthaltsort',
      type: 'grid',
      value: locationFilter,
      onChange: (value) => setLocationFilter(value as string),
      options: [
        { value: "all", label: "Alle Orte", icon: "M4 6h16M4 12h16M4 18h16" },
        { value: "zuhause", label: "Zuhause", icon: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" },
        { value: "im_raum", label: "Im Raum", icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" },
        { value: "schulhof", label: "Schulhof", icon: "M21 12a9 9 0 11-18 0 9 9 0 0118 0zM12 12a8 8 0 008 4M7.5 13.5a12 12 0 008.5 6.5M12 12a8 8 0 00-7.464 4.928M12.951 7.353a12 12 0 00-9.88 4.111M12 12a8 8 0 00-.536-8.928M15.549 15.147a12 12 0 001.38-10.611" },
        { value: "unterwegs", label: "Unterwegs", icon: "M13 10V3L4 14h7v7l9-11h-7z" }
      ]
    }
  ], [locationFilter]);

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
    
    if (locationFilter !== "all") {
      const locationLabels: Record<string, string> = {
        "zuhause": "Zuhause",
        "im_raum": "Im Raum",
        "schulhof": "Schulhof",
        "unterwegs": "Unterwegs"
      };
      filters.push({
        id: 'location',
        label: locationLabels[locationFilter] ?? locationFilter,
        onRemove: () => setLocationFilter("all")
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
      <div className="w-full -mt-1.5">
        {/* PageHeaderWithSearch - Title only on mobile */}
        <PageHeaderWithSearch
          title={isMobile ? "Mitarbeiter" : ""}
          badge={{
            icon: (
              <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                      d="M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2" />
              </svg>
            ),
            count: filteredStaff.length
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
            setLocationFilter("all");
          }}
        />

        {/* Error Display */}
        {error && (
          <div className="mb-4 p-4 bg-red-50 border border-red-200 rounded-lg text-red-800">
            {error}
          </div>
        )}

        {/* Staff Grid */}
        {filteredStaff.length === 0 ? (
          <div className="py-12 text-center">
            <div className="flex flex-col items-center gap-4">
              <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                      d="M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2" />
              </svg>
              <div>
                <h3 className="text-lg font-medium text-gray-900">Kein Personal gefunden</h3>
                <p className="text-gray-600">
                  Versuchen Sie Ihre Suchkriterien anzupassen.
                </p>
              </div>
            </div>
          </div>
        ) : (
          <div>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3 gap-6">
              {filteredStaff.map((staffMember) => {
                const locationStatus = getStaffLocationStatus(staffMember);
                const displayType = getStaffDisplayType(staffMember);
                const cardInfo = getStaffCardInfo(staffMember);
                const notes = formatStaffNotes(staffMember.staffNotes, 80);

                return (
                  <div
                    key={staffMember.id}
                    className={`group relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500`}
                  >
                    {/* Modern gradient overlay */}
                    <div className={`absolute inset-0 bg-gradient-to-br ${locationStatus.cardGradient} opacity-[0.03] rounded-3xl`}></div>
                    {/* Subtle inner glow */}
                    <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                    {/* Modern border highlight */}
                    <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20"></div>
                    
                    <div className="relative p-6">
                      {/* Header with staff name */}
                      <div className="flex items-center justify-between mb-2">
                        {/* Staff Name */}
                        <div className="flex-1 min-w-0">
                          <h3 className="text-lg font-bold text-gray-800 whitespace-nowrap overflow-hidden text-ellipsis">
                            {staffMember.firstName}
                          </h3>
                          <p className="text-base font-semibold text-gray-700 whitespace-nowrap overflow-hidden text-ellipsis">
                            {staffMember.lastName}
                          </p>
                          {/* Role/Specialization in same style as "Nur zur Information" */}
                          <p className="text-xs text-gray-400 mt-1">
                            {staffMember.specialization ?? displayType}
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
                      {cardInfo.length > 0 && (
                        <div className="flex flex-wrap gap-2 mb-2">
                          {cardInfo.map((info, idx) => (
                            <span key={idx} className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-700">
                              {info}
                            </span>
                          ))}
                        </div>
                      )}

                      {/* Staff Notes (if available) */}
                      {notes && (
                        <p className="text-xs text-gray-500 italic mt-2">
                          {notes}
                        </p>
                      )}
                      

                      {/* Decorative elements */}
                      <div className="absolute top-3 left-3 w-5 h-5 bg-white/20 rounded-full animate-ping"></div>
                      <div className="absolute bottom-3 right-3 w-3 h-3 bg-white/30 rounded-full"></div>
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
    <Suspense fallback={
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    }>
      <StaffPageContent />
    </Suspense>
  );
}