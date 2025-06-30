"use client";

import { useState, useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header/types";
import { fetchActivities, getCategories } from "~/lib/activity-api";
import { 
    formatSupervisorList, 
    type Activity, 
    type ActivityCategory
} from "~/lib/activity-helpers";

// Kategorie-zu-Farbe Mapping (gleich wie bei Räumen)
const categoryColors: Record<string, string> = {
    "Gruppenraum": "#4F46E5", // Blau für Gruppenraum
    "Lernen": "#10B981",      // Grün für Lernen
    "Spielen": "#F59E0B",     // Orange für Spielen
    "Bewegen/Ruhe": "#EC4899", // Pink für Bewegen/Ruhe
    "Hauswirtschaft": "#EF4444", // Rot für Hauswirtschaft
    "Natur": "#22C55E",       // Grün für Natur
    "Kreatives/Musik": "#8B5CF6", // Lila für Kreatives/Musik
    "NW/Technik": "#06B6D4",  // Türkis für NW/Technik
};

export default function ActivitiesPage() {
    const router = useRouter();
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [activities, setActivities] = useState<Activity[]>([]);
    const [categories, setCategories] = useState<ActivityCategory[]>([]);
    const [searchTerm, setSearchTerm] = useState("");
    const [categoryFilter, setCategoryFilter] = useState("all");
    const [participationFilter, setParticipationFilter] = useState("all");
    const [filteredActivities, setFilteredActivities] = useState<Activity[]>([]);

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

        // Apply participation filter
        if (participationFilter !== "all") {
            const isOpen = participationFilter === "open";
            filtered = filtered.filter(activity => activity.is_open_ags === isOpen);
        }

        setFilteredActivities(filtered);
    }, [searchTerm, categoryFilter, participationFilter, activities]);

    // Handle activity selection
    const handleSelectActivity = (activity: Activity) => {
        router.push(`/activities/${activity.id}`);
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
        },
        {
            id: 'participation',
            label: 'Teilnahme',
            type: 'dropdown', 
            value: participationFilter,
            onChange: (value: string | string[]) => setParticipationFilter(value as string),
            options: [
                { value: "all", label: "Alle" },
                { value: "open", label: "Aktiv" },
                { value: "closed", label: "Geschlossen" }
            ]
        }
    ], [categoryFilter, participationFilter, categories]);

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
        
        if (participationFilter !== "all") {
            filters.push({
                id: 'participation',
                label: participationFilter === "open" ? "Aktiv" : "Geschlossen",
                onRemove: () => setParticipationFilter("all")
            });
        }
        
        return filters;
    }, [searchTerm, categoryFilter, participationFilter, categories]);

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
                        setParticipationFilter("all");
                    }}
                    className="mb-6"
                />

                {/* Error Alert */}
                {error && (
                    <div className="mb-6 rounded-lg bg-red-50 border border-red-200 p-4">
                        <p className="text-sm text-red-800">{error}</p>
                    </div>
                )}

                {/* Activity Cards Grid */}
                {filteredActivities.length > 0 ? (
                    <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
                        {filteredActivities.map((activity) => {
                            const categoryColor = categoryColors[activity.category_name ?? ""] ?? "#6B7280";
                            const participantPercentage = activity.max_participant > 0 
                                ? Math.min((activity.participant_count ?? 0) / activity.max_participant * 100, 100)
                                : 0;
                            
                            return (
                                <div
                                    key={activity.id}
                                    onClick={() => handleSelectActivity(activity)}
                                    className="group cursor-pointer relative bg-white rounded-2xl shadow-sm hover:shadow-xl transition-all duration-300 hover:scale-[1.02] active:scale-[0.98] overflow-hidden border border-gray-100"
                                >
                                    {/* Stack effect for document metaphor */}
                                    <div className="absolute -bottom-1 left-2 right-2 h-full bg-gray-100 rounded-2xl -z-10 transform translate-y-1"></div>
                                    <div className="absolute -bottom-2 left-4 right-4 h-full bg-gray-50 rounded-2xl -z-20 transform translate-y-2"></div>
                                    
                                    {/* Main card content */}
                                    <div className="relative bg-white rounded-2xl overflow-hidden">
                                        {/* Category color sidebar */}
                                        <div 
                                            className="absolute left-0 top-0 bottom-0 w-1 opacity-80 group-hover:opacity-100 transition-opacity"
                                            style={{ backgroundColor: categoryColor }}
                                        ></div>
                                        
                                        {/* Content */}
                                        <div className="p-6 pl-7">
                                            {/* Header */}
                                            <div className="mb-4">
                                                <h3 className="text-lg font-semibold text-gray-900 mb-1 group-hover:text-[#5080D8] transition-colors line-clamp-2">
                                                    {activity.name}
                                                </h3>
                                                <p className="text-sm text-gray-600">
                                                    {formatSupervisorList(activity.supervisors)}
                                                </p>
                                            </div>
                                            
                                            {/* Category badge */}
                                            <div className="flex items-center gap-2 mb-4">
                                                <span 
                                                    className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium"
                                                    style={{
                                                        backgroundColor: `${categoryColor}15`,
                                                        color: categoryColor
                                                    }}
                                                >
                                                    <span 
                                                        className="w-1.5 h-1.5 rounded-full mr-1.5"
                                                        style={{ backgroundColor: categoryColor }}
                                                    ></span>
                                                    {activity.category_name ?? "Keine Kategorie"}
                                                </span>
                                                
                                                {/* Open/Closed status */}
                                                <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                                                    activity.is_open_ags 
                                                        ? "bg-green-100 text-green-700" 
                                                        : "bg-red-100 text-red-700"
                                                }`}>
                                                    {activity.is_open_ags ? "Aktiv" : "Geschlossen"}
                                                </span>
                                            </div>
                                            
                                            {/* Participant count and progress */}
                                            {activity.max_participant > 0 && (
                                                <div className="mt-auto">
                                                    <div className="flex justify-between items-center mb-2">
                                                        <span className="text-sm text-gray-600">Teilnehmer</span>
                                                        <span className="text-sm font-medium text-gray-900">
                                                            {activity.participant_count ?? 0} / {activity.max_participant}
                                                        </span>
                                                    </div>
                                                    <div className="relative h-2 bg-gray-100 rounded-full overflow-hidden">
                                                        <div 
                                                            className="absolute left-0 top-0 h-full transition-all duration-500 ease-out rounded-full"
                                                            style={{
                                                                width: `${participantPercentage}%`,
                                                                backgroundColor: participantPercentage >= 90 ? "#EF4444" : participantPercentage >= 70 ? "#F59E0B" : "#83CD2D"
                                                            }}
                                                        ></div>
                                                    </div>
                                                </div>
                                            )}
                                            
                                            {/* Hover indicator */}
                                            <div className="absolute bottom-6 right-6 w-8 h-8 bg-gray-100 rounded-full flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity duration-300">
                                                <svg className="w-4 h-4 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                                </svg>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            );
                        })}
                    </div>
                ) : (
                    <div className="flex min-h-[300px] items-center justify-center">
                        <div className="text-center">
                            <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
                            </svg>
                            <h3 className="mt-4 text-lg font-medium text-gray-900">
                                {searchTerm || categoryFilter !== "all" || participationFilter !== "all"
                                    ? "Keine Aktivitäten gefunden"
                                    : "Keine Aktivitäten vorhanden"}
                            </h3>
                            <p className="mt-2 text-sm text-gray-600">
                                {searchTerm || categoryFilter !== "all" || participationFilter !== "all"
                                    ? "Versuchen Sie andere Suchkriterien oder Filter."
                                    : "Es wurden noch keine Aktivitäten erstellt."}
                            </p>
                        </div>
                    </div>
                )}
            </div>
        </ResponsiveLayout>
    );
}