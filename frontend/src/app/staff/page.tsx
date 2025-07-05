"use client";

import { useState, useEffect, Suspense, useMemo } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header";
import { staffService } from "~/lib/staff-api";
import type { Staff, StaffFilters } from "~/lib/staff-api";
import { 
  getStaffLocationStatus, 
  getStaffDisplayType, 
  getStaffCardInfo,
  formatStaffNotes,
  sortStaff 
} from "~/lib/staff-helpers";

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
  const [typeFilter, setTypeFilter] = useState("all");
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Fetch staff data
  useEffect(() => {
    const fetchStaffData = async () => {
      try {
        setIsLoading(true);

        const filters: StaffFilters = {
          search: searchTerm || undefined,
          type: typeFilter as StaffFilters['type'],
        };

        const staffData = await staffService.getAllStaff(filters);
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
  }, [session?.user?.token, searchTerm, typeFilter]);

  // Apply client-side search filter (in case backend doesn't support search)
  const filteredStaff = staff.filter((staffMember) => {
    if (searchTerm) {
      const searchLower = searchTerm.toLowerCase();
      return (
        staffMember.firstName.toLowerCase().includes(searchLower) ||
        staffMember.lastName.toLowerCase().includes(searchLower) ||
        staffMember.name.toLowerCase().includes(searchLower)
      );
    }
    return true;
  });

  // Prepare filter configurations for PageHeaderWithSearch
  const filterConfigs: FilterConfig[] = useMemo(() => [
    {
      id: 'type',
      label: 'Typ',
      type: 'buttons',
      value: typeFilter,
      onChange: (value) => setTypeFilter(value as string),
      options: [
        { value: 'all', label: 'Alle' },
        { value: 'teachers', label: 'Lehrer' },
        { value: 'staff', label: 'Betreuer' }
      ]
    }
  ], [typeFilter]);

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
    
    if (typeFilter !== "all") {
      const typeLabels: Record<string, string> = {
        "teachers": "Lehrer",
        "staff": "Betreuer"
      };
      filters.push({
        id: 'type',
        label: typeLabels[typeFilter] ?? typeFilter,
        onRemove: () => setTypeFilter("all")
      });
    }
    
    return filters;
  }, [searchTerm, typeFilter]);

  if (status === "loading" || isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
          <p className="text-gray-600">Personal wird geladen...</p>
        </div>
      </div>
    );
  }

  return (
    <ResponsiveLayout>
      <div className="w-full">
        {/* Modern Header with PageHeaderWithSearch component */}
        <PageHeaderWithSearch
          title="Personal"
          badge={{
            icon: (
              <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                      d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
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
            setTypeFilter("all");
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
                      d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
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
            {/* Add floating animation keyframes */}
            <style jsx>{`
              @keyframes float {
                0%, 100% { transform: translateY(0px) rotate(var(--rotation)); }
                50% { transform: translateY(-4px) rotate(var(--rotation)); }
              }
            `}</style>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3 gap-6">
              {filteredStaff.map((staffMember, index) => {
                const locationStatus = getStaffLocationStatus(staffMember);
                const displayType = getStaffDisplayType(staffMember);
                const cardInfo = getStaffCardInfo(staffMember);
                const notes = formatStaffNotes(staffMember.staffNotes, 80);

                return (
                  <div
                    key={staffMember.id}
                    className={`group relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500`}
                    style={{
                      transform: `rotate(${(index % 3 - 1) * 0.8}deg)`,
                      animation: `float 8s ease-in-out infinite ${index * 0.7}s`,
                      '--rotation': `${(index % 3 - 1) * 0.8}deg`
                    } as React.CSSProperties}
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

                      {/* Display Type */}
                      <div className="mb-2">
                        <p className="text-sm font-medium text-gray-600">
                          {displayType}
                        </p>
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
                      
                      {/* Info text - matching ogs_groups style */}
                      <div className="flex justify-start mt-3">
                        <p className="text-xs text-gray-400">
                          Nur zur Information
                        </p>
                      </div>

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
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
      </div>
    }>
      <StaffPageContent />
    </Suspense>
  );
}