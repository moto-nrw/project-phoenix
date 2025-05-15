"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Header } from "~/components/dashboard/header";
import { Sidebar } from "~/components/dashboard/sidebar";
import { Alert } from "~/components/ui/alert";

// Activity interface
interface Activity {
    id: string;
    name: string;
    categoryId: string;
    category: string;
    maxParticipants: number;
    currentParticipants?: number;
    isOpen: boolean;
    supervisorName?: string;
    plannedRoomId?: string;
    plannedRoomName?: string;
    activeRoomId?: string;
    activeRoomName?: string;
    isActive: boolean;
    startTime?: string;
    endTime?: string;
    building?: string; // Gebäude zur Aktivität hinzugefügt
}

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

// Dummy data for activities with building information
const dummyActivities: Activity[] = [
    {
        id: "2",
        name: "Chor",
        categoryId: "7",
        category: "Kreatives/Musik",
        maxParticipants: 40,
        currentParticipants: 35,
        isOpen: true,
        supervisorName: "Hr. Wagner",
        plannedRoomId: "2",
        plannedRoomName: "Musiksaal",
        activeRoomId: "2",
        activeRoomName: "Musiksaal",
        isActive: true,
        startTime: "14:00",
        endTime: "15:30",
        building: "Hauptgebäude"
    },
    {
        id: "3",
        name: "Informatik AG",
        categoryId: "8",
        category: "NW/Technik",
        maxParticipants: 25,
        currentParticipants: 15,
        isOpen: true,
        supervisorName: "Hr. Meyer",
        plannedRoomId: "3",
        plannedRoomName: "Computerraum 1",
        activeRoomId: "3",
        activeRoomName: "Computerraum 1",
        isActive: true,
        startTime: "15:30",
        endTime: "17:00",
        building: "Technikgebäude"
    },
    {
        id: "5",
        name: "Basketball AG",
        categoryId: "4",
        category: "Bewegen/Ruhe",
        maxParticipants: 30,
        currentParticipants: 28,
        isOpen: false,
        supervisorName: "Hr. Müller",
        plannedRoomId: "5",
        plannedRoomName: "Turnhalle",
        activeRoomId: "5",
        activeRoomName: "Turnhalle",
        isActive: true,
        startTime: "10:00",
        endTime: "11:30",
        building: "Sporthalle"
    },
    {
        id: "6",
        name: "Gruppenraum",
        categoryId: "2",
        category: "Lernen",
        maxParticipants: 30,
        currentParticipants: 27,
        isOpen: false,
        supervisorName: "Hr. Weber",
        plannedRoomId: "7",
        plannedRoomName: "Klassenraum 201",
        activeRoomId: "7",
        activeRoomName: "Klassenraum 201",
        isActive: true,
        startTime: "08:00",
        endTime: "09:30",
        building: "Hauptgebäude"
    },
    {
        id: "7",
        name: "Lesegruppe",
        categoryId: "2",
        category: "Lernen",
        maxParticipants: 15,
        isOpen: true,
        supervisorName: "Fr. Schulz",
        plannedRoomId: "8",
        plannedRoomName: "Bibliothek",
        isActive: false,
        building: "Hauptgebäude"
    },
    {
        id: "10",
        name: "Leimen",
        categoryId: "5",
        category: "Hauswirtschaft",
        maxParticipants: 200,
        currentParticipants: 150,
        isOpen: true,
        supervisorName: "Hr. Wenger",
        plannedRoomId: "11",
        plannedRoomName: "Raum 12.3",
        activeRoomId: "11",
        activeRoomName: "Raum 12.3",
        isActive: true,
        startTime: "12:00",
        endTime: "13:30",
        building: "Nebengebäude"
    },
    {
        id: "11",
        name: "Theaterprojekt",
        categoryId: "3",
        category: "Spielen",
        maxParticipants: 40,
        isOpen: true,
        supervisorName: "Fr. Hoffmann",
        plannedRoomId: "12",
        plannedRoomName: "Aula",
        isActive: false,
        building: "Hauptgebäude"
    },
    {
        id: "12",
        name: "Garten AG",
        categoryId: "6",
        category: "Natur",
        maxParticipants: 20,
        isOpen: true,
        supervisorName: "Hr. Fischer",
        isActive: false,
        building: "Außenbereich"
    }
];

export default function ActivitiesPage() {
    const router = useRouter();
    const [loading, setLoading] = useState(false);
    const [error] = useState<string | null>(null);
    const [searchFilter, setSearchFilter] = useState("");
    const [categoryFilter, setCategoryFilter] = useState<string | null>(null);
    const [statusFilter, setStatusFilter] = useState<string | null>(null);
    const [openFilter, setOpenFilter] = useState<string | null>(null);
    const [buildingFilter, setBuildingFilter] = useState<string | null>(null);
    const [filteredActivities, setFilteredActivities] = useState<Activity[]>(dummyActivities);

    // Apply filters function
    const applyFilters = () => {
        setLoading(true);

        let filtered = [...dummyActivities];

        // Apply search filter
        if (searchFilter) {
            const searchLower = searchFilter.toLowerCase();
            filtered = filtered.filter(activity =>
                activity.name.toLowerCase().includes(searchLower) ||
                (activity.supervisorName?.toLowerCase().includes(searchLower) ?? false) ||
                (activity.plannedRoomName?.toLowerCase().includes(searchLower) ?? false)
            );
        }

        // Apply category filter
        if (categoryFilter) {
            filtered = filtered.filter(activity => activity.category === categoryFilter);
        }

        // Apply status filter
        if (statusFilter) {
            const isActive = statusFilter === "active";
            filtered = filtered.filter(activity => activity.isActive === isActive);
        }

        // Apply open filter
        if (openFilter) {
            const isOpen = openFilter === "open";
            filtered = filtered.filter(activity => activity.isOpen === isOpen);
        }

        // Apply building filter
        if (buildingFilter) {
            filtered = filtered.filter(activity => activity.building === buildingFilter);
        }

        setFilteredActivities(filtered);
        setLoading(false);
    };

    // Effect to apply filters when filter values change
    useEffect(() => {
        const timer = setTimeout(() => {
            applyFilters();
        }, 300);

        return () => clearTimeout(timer);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [searchFilter, categoryFilter, statusFilter, openFilter, buildingFilter]);

    // Handle activity selection
    const handleSelectActivity = (activity: Activity) => {
        router.push(`/activities/${activity.id}`);
    };

    // Get all unique categories for filter dropdown
    const uniqueCategories = Array.from(
        new Set(dummyActivities.map((activity) => activity.category))
    );

    // Get all unique buildings for filter dropdown
    const uniqueBuildings = Array.from(
        new Set(dummyActivities.map((activity) => activity.building).filter(Boolean))
    );

    // Reset all filters
    const resetFilters = () => {
        setSearchFilter("");
        setCategoryFilter(null);
        setStatusFilter(null);
        setOpenFilter(null);
        setBuildingFilter(null);
    };

    if (loading) {
        return (
            <div className="flex min-h-screen items-center justify-center">
                <p>Loading...</p>
            </div>
        );
    }

    // Common class for all dropdowns to ensure consistent height
    const dropdownClass = "mt-1 block w-full rounded-lg border-0 px-4 py-3 h-12 shadow-sm ring-1 ring-gray-200 transition-all duration-200 hover:bg-gray-50/50 hover:ring-gray-300 focus:ring-2 focus:ring-blue-500 focus:outline-none appearance-none pr-8";

    return (
        <div className="min-h-screen bg-gray-50">
            {/* Header */}
            <Header userName="Benutzer" />

            <div className="flex">
                {/* Sidebar */}
                <Sidebar />

                {/* Main Content */}
                <main className="flex-1 p-8">
                    <div className="mx-auto max-w-7xl">
                        <h1 className="mb-8 text-4xl font-bold text-gray-900">Aktivitätenübersicht</h1>

                        {/* Search and Filter Panel */}
                        <div className="mb-8 overflow-hidden rounded-xl bg-white p-6 shadow-md">
                            <h2 className="mb-4 text-xl font-bold text-gray-800">Filter</h2>

                            <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-5">
                                {/* Search input */}
                                <div>
                                    <label className="block text-sm font-medium text-gray-700">
                                        Suche
                                    </label>
                                    <div className="relative mt-1">
                                        <input
                                            type="text"
                                            placeholder="Aktivität, Lehrer, Raum..."
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
                                        {uniqueCategories.map((category) => (
                                            <option key={category} value={category}>
                                                {category}
                                            </option>
                                        ))}
                                    </select>
                                    <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                                        <svg className="h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                                            <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                        </svg>
                                    </div>
                                </div>

                                {/* Activity Status Filter */}
                                <div className="relative">
                                    <label className="block text-sm font-medium text-gray-700">
                                        Status
                                    </label>
                                    <select
                                        value={statusFilter ?? ""}
                                        onChange={(e) => setStatusFilter(e.target.value || null)}
                                        className={dropdownClass}
                                    >
                                        <option value="">Alle</option>
                                        <option value="active">Aktiv</option>
                                        <option value="inactive">Inaktiv</option>
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

                                {/* Building Filter (ersetzt Room Filter) */}
                                <div className="relative">
                                    <label className="block text-sm font-medium text-gray-700">
                                        Gebäude
                                    </label>
                                    <select
                                        value={buildingFilter ?? ""}
                                        onChange={(e) => setBuildingFilter(e.target.value || null)}
                                        className={dropdownClass}
                                    >
                                        <option value="">Alle Gebäude</option>
                                        {uniqueBuildings.map((building) => (
                                            <option key={building} value={building}>
                                                {building}
                                            </option>
                                        ))}
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
                                        className="overflow-hidden rounded-lg bg-white shadow-md"
                                    >
                                        {/* Top colored bar based on activity category */}
                                        <div
                                            className="h-2"
                                            style={{ backgroundColor: categoryColors[activity.category] ?? "#6B7280" }}
                                        />
                                        <div className="p-4">
                                            <div className="mb-2 flex items-center justify-between">
                                                <h3 className="text-lg font-semibold text-gray-800">{activity.name}</h3>
                                                <span
                                                    className={`inline-flex h-3 w-3 rounded-full ${
                                                        activity.isActive ? "bg-green-500" : "bg-gray-400"
                                                    }`}
                                                    title={activity.isActive ? "Aktiv" : "Inaktiv"}
                                                ></span>
                                            </div>
                                            <div className="text-sm text-gray-600">
                                                {activity.supervisorName && (
                                                    <p className="mb-1">Leitung: {activity.supervisorName}</p>
                                                )}
                                                {activity.plannedRoomName && (
                                                    <p className="mb-1">
                                                        Raum: {activity.isActive && activity.activeRoomName ? activity.activeRoomName : activity.plannedRoomName}
                                                        {activity.isActive && activity.activeRoomName !== activity.plannedRoomName && activity.activeRoomName && (
                                                            <span className="text-xs text-orange-500 ml-1">(geändert)</span>
                                                        )}
                                                    </p>
                                                )}
                                                {activity.building && (
                                                    <p className="mb-1">Gebäude: {activity.building}</p>
                                                )}
                                            </div>

                                            <div className="mt-2 border-t border-gray-100 pt-2 text-sm font-medium text-gray-700">
                                                <div className="flex items-center">
                                                    <span
                                                        className="inline-block h-3 w-3 rounded-full mr-1.5"
                                                        style={{ backgroundColor: categoryColors[activity.category] ?? "#6B7280" }}
                                                    ></span>
                                                    Kategorie: {activity.category}
                                                </div>

                                                <div className="mt-1 flex items-center">
                                                    <span className={`inline-block h-2 w-2 rounded-full mr-2 ${activity.isOpen ? "bg-green-500" : "bg-red-500"}`}></span>
                                                    {activity.isOpen ? "Offen für Teilnahme" : "Geschlossene Gruppe"}
                                                </div>

                                                {activity.currentParticipants !== undefined && activity.maxParticipants > 0 && (
                                                    <div className="mt-1">
                                                        <div>Teilnehmer: {activity.currentParticipants} / {activity.maxParticipants}</div>
                                                        <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200">
                                                            <div
                                                                className="h-full bg-blue-600"
                                                                style={{
                                                                    width: `${Math.min(
                                                                        (activity.currentParticipants / activity.maxParticipants) * 100,
                                                                        100
                                                                    )}%`,
                                                                }}
                                                            ></div>
                                                        </div>
                                                    </div>
                                                )}

                                                {/* Zeit-Felder entfernt */}
                                            </div>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        ) : (
                            <div className="flex h-40 items-center justify-center rounded-lg border-2 border-dashed border-gray-300 bg-white p-6">
                                <div className="text-center">
                                    <p className="text-sm font-medium text-gray-500">
                                        {searchFilter || categoryFilter || statusFilter || openFilter || buildingFilter
                                            ? "Keine Aktivitäten gefunden, die Ihren Filterkriterien entsprechen."
                                            : "Keine Aktivitäten verfügbar."}
                                    </p>
                                </div>
                            </div>
                        )}
                    </div>
                </main>
            </div>
        </div>
    );
}