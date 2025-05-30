"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
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
    const [searchFilter, setSearchFilter] = useState("");
    const [categoryFilter, setCategoryFilter] = useState<string | null>(null);
    const [openFilter, setOpenFilter] = useState<string | null>(null);
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

    // Apply filters function
    const applyFilters = () => {
        let filtered = [...activities];

        // Apply search filter
        if (searchFilter) {
            const searchLower = searchFilter.toLowerCase();
            filtered = filtered.filter(activity =>
                activity.name.toLowerCase().includes(searchLower) ||
                formatSupervisorList(activity.supervisors).toLowerCase().includes(searchLower) ||
                (activity.category_name?.toLowerCase().includes(searchLower) ?? false)
            );
        }

        // Apply category filter
        if (categoryFilter) {
            filtered = filtered.filter(activity => activity.ag_category_id === categoryFilter);
        }

        // Apply open filter
        if (openFilter) {
            const isOpen = openFilter === "open";
            filtered = filtered.filter(activity => activity.is_open_ags === isOpen);
        }

        setFilteredActivities(filtered);
    };

    // Effect to apply filters when filter values change
    useEffect(() => {
        const timer = setTimeout(() => {
            applyFilters();
        }, 300);

        return () => clearTimeout(timer);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [searchFilter, categoryFilter, openFilter, activities]);

    // Handle activity selection
    const handleSelectActivity = (activity: Activity) => {
        router.push(`/activities/${activity.id}`);
    };

    // Reset all filters
    const resetFilters = () => {
        setSearchFilter("");
        setCategoryFilter(null);
        setOpenFilter(null);
    };

    if (loading) {
        return (
            <ResponsiveLayout>
                <div className="flex min-h-screen items-center justify-center">
                    <div className="flex flex-col items-center gap-4">
                        <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
                        <p className="text-gray-600">Daten werden geladen...</p>
                    </div>
                </div>
            </ResponsiveLayout>
        );
    }

    // Common class for all dropdowns to ensure consistent height
    const dropdownClass = "mt-1 block w-full rounded-lg border-0 px-4 py-3 h-12 shadow-sm ring-1 ring-gray-200 transition-all duration-200 hover:bg-gray-50/50 hover:ring-gray-300 focus:ring-2 focus:ring-blue-500 focus:outline-none appearance-none pr-8";

    return (
        <ResponsiveLayout>
            <div className="mx-auto max-w-7xl">
                <h1 className="mb-8 text-4xl font-bold text-gray-900">Aktivitätenübersicht</h1>

                {/* Search and Filter Panel */}
                <div className="mb-8 overflow-hidden rounded-xl bg-white p-6 shadow-md">
                    <h2 className="mb-4 text-xl font-bold text-gray-800">Filter</h2>

                    <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
                        {/* Search input */}
                        <div>
                            <label className="block text-sm font-medium text-gray-700">
                                Suche
                            </label>
                            <div className="relative mt-1">
                                <input
                                    type="text"
                                    placeholder="Aktivität, Betreuer, Kategorie..."
                                    value={searchFilter}
                                    onChange={(e) => setSearchFilter(e.target.value)}
                                    className="block w-full rounded-lg border-0 px-4 py-3 pl-10 h-12 shadow-sm ring-1 ring-gray-200 transition-all duration-200 hover:bg-gray-50/50 hover:ring-gray-300 focus:ring-2 focus:ring-blue-500 focus:outline-none"
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

                        {/* Category Filter */}
                        <div className="relative">
                            <label className="block text-sm font-medium text-gray-700">
                                Kategorie
                            </label>
                            <select
                                value={categoryFilter ?? ""}
                                onChange={(e) => setCategoryFilter(e.target.value || null)}
                                className={dropdownClass}
                            >
                                <option value="">Alle Kategorien</option>
                                {categories.map((category) => (
                                    <option key={category.id} value={category.id}>
                                        {category.name}
                                    </option>
                                ))}
                            </select>
                            <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                                <svg className="h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                                    <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                </svg>
                            </div>
                        </div>

                        {/* Open/Closed Filter */}
                        <div className="relative">
                            <label className="block text-sm font-medium text-gray-700">
                                Teilnahme
                            </label>
                            <select
                                value={openFilter ?? ""}
                                onChange={(e) => setOpenFilter(e.target.value || null)}
                                className={dropdownClass}
                            >
                                <option value="">Alle</option>
                                <option value="open">Offen</option>
                                <option value="closed">Geschlossen</option>
                            </select>
                            <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                                <svg className="h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                                    <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                </svg>
                            </div>
                        </div>
                    </div>

                    {/* Filter Actions */}
                    <div className="mt-6 flex flex-wrap justify-end gap-3">
                        <button
                            onClick={resetFilters}
                            className="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50"
                        >
                            Zurücksetzen
                        </button>
                        <button
                            onClick={applyFilters}
                            className="rounded-lg bg-gradient-to-r from-teal-500 to-blue-600 px-6 py-2 text-sm font-medium text-white shadow-sm transition-all hover:from-teal-600 hover:to-blue-700 hover:shadow-md"
                        >
                            Filtern
                        </button>
                    </div>
                </div>

                {/* Error Alert */}
                {error && <Alert type="error" message={error} />}

                {/* Activity Cards Grid */}
                {filteredActivities.length > 0 ? (
                    <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                        {filteredActivities.map((activity) => (
                            <div
                                key={activity.id}
                                onClick={() => handleSelectActivity(activity)}
                                className="cursor-pointer overflow-hidden rounded-lg bg-white shadow-md transition-all duration-200 hover:translate-y-[-2px] hover:shadow-lg"
                            >
                                {/* Top colored bar based on activity category */}
                                <div
                                    className="h-2"
                                    style={{ backgroundColor: categoryColors[activity.category_name ?? ""] ?? "#6B7280" }}
                                />
                                <div className="p-4">
                                    <div className="mb-2 flex items-center justify-between">
                                        <h3 className="text-lg font-semibold text-gray-800">{activity.name}</h3>
                                    </div>
                                    <div className="text-sm text-gray-600">
                                        <p className="mb-1">Leitung: {formatSupervisorList(activity.supervisors)}</p>
                                    </div>

                                    <div className="mt-2 border-t border-gray-100 pt-2 text-sm font-medium text-gray-700">
                                        <div className="flex items-center">
                                            <span
                                                className="inline-block h-3 w-3 rounded-full mr-1.5"
                                                style={{ backgroundColor: categoryColors[activity.category_name ?? ""] ?? "#6B7280" }}
                                            ></span>
                                            Kategorie: {activity.category_name ?? "Unbekannt"}
                                        </div>

                                        <div className="mt-1 flex items-center">
                                            <span className={`inline-block h-2 w-2 rounded-full mr-2 ${activity.is_open_ags ? "bg-green-500" : "bg-red-500"}`}></span>
                                            {activity.is_open_ags ? "Offen für Teilnahme" : "Geschlossene Gruppe"}
                                        </div>

                                        {activity.participant_count !== undefined && activity.max_participant > 0 && (
                                            <div className="mt-1">
                                                <div>Teilnehmer: {activity.participant_count} / {activity.max_participant}</div>
                                                <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200">
                                                    <div
                                                        className="h-full bg-blue-600"
                                                        style={{
                                                            width: `${Math.min(
                                                                (activity.participant_count / activity.max_participant) * 100,
                                                                100
                                                            )}%`,
                                                        }}
                                                    ></div>
                                                </div>
                                            </div>
                                        )}
                                    </div>
                                </div>
                            </div>
                        ))}
                    </div>
                ) : (
                    <div className="flex h-40 items-center justify-center rounded-lg border-2 border-dashed border-gray-300 bg-white p-6">
                        <div className="text-center">
                            <p className="text-sm font-medium text-gray-500">
                                {searchFilter || categoryFilter || openFilter
                                    ? "Keine Aktivitäten gefunden, die Ihren Filterkriterien entsprechen."
                                    : "Keine Aktivitäten verfügbar."}
                            </p>
                        </div>
                    </div>
                )}
            </div>
        </ResponsiveLayout>
    );
}