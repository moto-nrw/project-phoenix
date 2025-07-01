"use client";

import { useState, useEffect, useMemo } from "react";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header/types";
import { fetchActivities, getCategories } from "~/lib/activity-api";
import { 
    formatSupervisorList, 
    type Activity, 
    type ActivityCategory
} from "~/lib/activity-helpers";
import { ActivityManagementModal } from "~/components/activities/activity-management-modal";


export default function ActivitiesPage() {
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [activities, setActivities] = useState<Activity[]>([]);
    const [categories, setCategories] = useState<ActivityCategory[]>([]);
    const [searchTerm, setSearchTerm] = useState("");
    const [categoryFilter, setCategoryFilter] = useState("all");
    const [filteredActivities, setFilteredActivities] = useState<Activity[]>([]);
    const [selectedActivity, setSelectedActivity] = useState<Activity | null>(null);
    const [isManagementModalOpen, setIsManagementModalOpen] = useState(false);

    // Load activities and categories on mount
    useEffect(() => {
        const loadData = async () => {
            try {
                setLoading(true);
                const [activitiesData, categoriesData] = await Promise.all([
                    fetchActivities(),
                    getCategories()
                ]);
                setActivities(activitiesData);
                setCategories(categoriesData);
                setFilteredActivities(activitiesData);
                setError(null);
            } catch (err) {
                setError("Fehler beim Laden der Aktivitäten");
                console.error("Error loading activities:", err);
            } finally {
                setLoading(false);
            }
        };

        void loadData();
    }, []);

    // Apply filters
    useEffect(() => {
        // Ensure activities is an array before filtering
        const activityList = Array.isArray(activities) ? activities : [];
        let filtered = [...activityList];

        // Apply search filter
        if (searchTerm) {
            const searchLower = searchTerm.toLowerCase();
            filtered = filtered.filter(activity =>
                activity.name.toLowerCase().includes(searchLower) ||
                formatSupervisorList(activity.supervisors).toLowerCase().includes(searchLower) ||
                (activity.category_name?.toLowerCase().includes(searchLower) ?? false)
            );
        }

        // Apply category filter
        if (categoryFilter !== "all") {
            filtered = filtered.filter(activity => activity.ag_category_id === categoryFilter);
        }


        setFilteredActivities(filtered);
    }, [searchTerm, categoryFilter, activities]);

    // Handle activity selection - open management modal
    const handleSelectActivity = (activity: Activity) => {
        setSelectedActivity(activity);
        setIsManagementModalOpen(true);
    };

    // Handle edit button click - same as selecting activity
    const handleEditActivity = (e: React.MouseEvent, activity: Activity) => {
        e.stopPropagation(); // Prevent duplicate modal opening
        handleSelectActivity(activity);
    };

    // Handle successful management actions (edit/delete)
    const handleManagementSuccess = async () => {
        // Reload activities to show updated data
        try {
            const activitiesData = await fetchActivities();
            setActivities(activitiesData);
            setFilteredActivities(activitiesData);
        } catch (err) {
            console.error("Error reloading activities:", err);
        }
    };

    // Prepare filters for PageHeaderWithSearch
    const filters: FilterConfig[] = useMemo(() => [
        {
            id: 'category',
            label: 'Kategorie',
            type: 'dropdown',
            value: categoryFilter,
            onChange: (value: string | string[]) => setCategoryFilter(value as string),
            options: [
                { value: "all", label: "Alle Kategorien" },
                ...categories.map(cat => ({
                    value: cat.id.toString(),
                    label: cat.name
                }))
            ]
        }
    ], [categoryFilter, categories]);

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
        
        if (categoryFilter !== "all") {
            const category = categories.find(cat => cat.id.toString() === categoryFilter);
            filters.push({
                id: 'category',
                label: category?.name ?? 'Kategorie',
                onRemove: () => setCategoryFilter("all")
            });
        }
        
        
        return filters;
    }, [searchTerm, categoryFilter, categories]);

    if (loading) {
        return (
            <ResponsiveLayout>
                <div className="flex min-h-[50vh] items-center justify-center">
                    <div className="flex flex-col items-center gap-4">
                        <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#5080D8]"></div>
                        <p className="text-gray-600">Aktivitäten werden geladen...</p>
                    </div>
                </div>
            </ResponsiveLayout>
        );
    }

    return (
        <ResponsiveLayout>
            <div className="w-full">
                {/* Page Header with Search and Filters */}
                <PageHeaderWithSearch
                    title="Aktivitäten"
                    badge={activities.length > 0 ? { count: activities.length, label: activities.length === 1 ? "Aktivität" : "Aktivitäten" } : undefined}
                    search={{
                        value: searchTerm,
                        onChange: setSearchTerm,
                        placeholder: "Aktivität, Betreuer oder Kategorie suchen..."
                    }}
                    filters={filters}
                    activeFilters={activeFilters}
                    onClearAllFilters={() => {
                        setSearchTerm("");
                        setCategoryFilter("all");
                    }}
                    className="mb-6"
                />

                {/* Error Alert */}
                {error && (
                    <div className="mb-6 rounded-lg bg-red-50 border border-red-200 p-4">
                        <p className="text-sm text-red-800">{error}</p>
                    </div>
                )}

                {/* Activity List - Modern Design */}
                {filteredActivities.length > 0 ? (
                    <div className="space-y-3">
                        {filteredActivities.map((activity, index) => {
                            const isGruppenraum = activity.category_name === "Gruppenraum";
                            
                            return (
                                <div
                                    key={activity.id}
                                    onClick={() => handleSelectActivity(activity)}
                                    className="group cursor-pointer relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.01] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-1 active:scale-[0.99] md:hover:border-blue-200/50"
                                    style={{
                                        animationDelay: `${index * 0.05}s`,
                                        animation: 'fadeInUp 0.5s ease-out forwards',
                                        opacity: 0
                                    }}
                                >
                                    {/* Modern gradient overlay */}
                                    <div className="absolute inset-0 bg-gradient-to-br from-blue-50/80 to-cyan-100/80 opacity-[0.03] rounded-3xl"></div>
                                    {/* Subtle inner glow */}
                                    <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                                    {/* Modern border highlight */}
                                    <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 md:group-hover:ring-blue-200/60 transition-all duration-300"></div>
                                    
                                    {/* Gruppenraum indicator - accent bar */}
                                    {isGruppenraum && (
                                        <div 
                                            className="absolute left-0 top-0 bottom-0 w-1 rounded-l-3xl"
                                            style={{ backgroundColor: "#83CD2D" }}
                                        ></div>
                                    )}
                                    
                                    <div className={`relative flex items-center justify-between p-5 ${isGruppenraum ? 'pl-6' : ''}`}>
                                        {/* Left content */}
                                        <div className="flex-1 min-w-0">
                                            {/* Activity Name */}
                                            <h3 className="text-lg font-semibold text-gray-900 md:group-hover:text-blue-600 transition-colors duration-300">
                                                {activity.name}
                                            </h3>
                                            
                                            {/* Meta info row */}
                                            <div className="flex items-center gap-4 mt-1">
                                                {/* Creator info */}
                                                <p className="text-sm text-gray-500">
                                                    <span className="text-gray-400">Erstellt von:</span> {formatSupervisorList(activity.supervisors)}
                                                </p>
                                                
                                                {/* Gruppenraum indicator */}
                                                {isGruppenraum && (
                                                    <div className="flex items-center gap-1.5">
                                                        <div className="w-2 h-2 rounded-full bg-[#83CD2D] animate-pulse"></div>
                                                        <span className="text-sm font-medium text-[#83CD2D]">
                                                            Gruppenraum
                                                        </span>
                                                    </div>
                                                )}
                                            </div>
                                        </div>
                                        
                                        {/* Right content - Edit button */}
                                        <div className="flex items-center gap-3 ml-4">
                                            {/* Desktop hint */}
                                            <span className="hidden lg:block text-xs text-gray-400 group-hover:text-gray-600 transition-colors">
                                                Bearbeiten
                                            </span>
                                            
                                            {/* Edit icon button */}
                                            <button
                                                onClick={(e) => handleEditActivity(e, activity)}
                                                className="relative"
                                                aria-label="Aktivität bearbeiten"
                                            >
                                                <div className="w-10 h-10 rounded-full bg-gray-100 md:group-hover:bg-blue-100 flex items-center justify-center transition-all duration-200 md:group-hover:scale-110">
                                                    <svg 
                                                        className="w-5 h-5 text-gray-600 md:group-hover:text-blue-600 transition-colors" 
                                                        fill="none" 
                                                        viewBox="0 0 24 24" 
                                                        stroke="currentColor"
                                                    >
                                                        <path 
                                                            strokeLinecap="round" 
                                                            strokeLinejoin="round" 
                                                            strokeWidth={2} 
                                                            d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" 
                                                        />
                                                    </svg>
                                                </div>
                                                
                                                {/* Ripple effect on hover */}
                                                <div className="absolute inset-0 rounded-full bg-blue-200/20 scale-0 md:group-hover:scale-100 transition-transform duration-300"></div>
                                            </button>
                                        </div>
                                    </div>

                                    {/* Glowing border effect */}
                                    <div className="absolute inset-0 rounded-3xl opacity-0 md:group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent via-blue-100/30 to-transparent"></div>
                                </div>
                            );
                        })}
                        
                        {/* Add fadeInUp animation */}
                        <style jsx>{`
                            @keyframes fadeInUp {
                                from {
                                    opacity: 0;
                                    transform: translateY(20px);
                                }
                                to {
                                    opacity: 1;
                                    transform: translateY(0);
                                }
                            }
                        `}</style>
                    </div>
                ) : (
                    <div className="flex min-h-[300px] items-center justify-center">
                        <div className="text-center">
                            <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
                            </svg>
                            <h3 className="mt-4 text-lg font-medium text-gray-900">
                                {searchTerm || categoryFilter !== "all"
                                    ? "Keine Aktivitäten gefunden"
                                    : "Keine Aktivitäten vorhanden"}
                            </h3>
                            <p className="mt-2 text-sm text-gray-600">
                                {searchTerm || categoryFilter !== "all"
                                    ? "Versuchen Sie andere Suchkriterien oder Filter."
                                    : "Es wurden noch keine Aktivitäten erstellt."}
                            </p>
                        </div>
                    </div>
                )}
            </div>
            
            {/* Activity Management Modal */}
            {selectedActivity && (
                <ActivityManagementModal
                    isOpen={isManagementModalOpen}
                    onClose={() => {
                        setIsManagementModalOpen(false);
                        setSelectedActivity(null);
                    }}
                    onSuccess={handleManagementSuccess}
                    activity={selectedActivity}
                />
            )}
        </ResponsiveLayout>
    );
}