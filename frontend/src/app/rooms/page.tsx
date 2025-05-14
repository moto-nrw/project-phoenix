"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Header } from "~/components/dashboard/header";
import { Sidebar } from "~/components/dashboard/sidebar";
import { Alert } from "~/components/ui/alert";

// Room interface
interface Room {
    id: string;
    name: string;
    building?: string;
    floor: number;
    capacity: number;
    category: string;
    isOccupied: boolean;
    groupName?: string;
    activityName?: string;
    supervisorName?: string;
    deviceId?: string;
    studentCount?: number;
}

// Kategorie-zu-Farbe Mapping
const categoryColors: Record<string, string> = {
    "Gruppenraum": "#4F46E5", // Blau für Gruppenraum (circle1)
    "Lernen": "#10B981",      // Grün für Lernen (circle2)
    "Spielen": "#F59E0B",     // Orange für Spielen (circle3)
    "Bewegen/Ruhe": "#EC4899", // Pink für Bewegen/Ruhe (circle4)
    "Hauswirtschaft": "#EF4444", // Rot für Hauswirtschaft (circle5)
    "Natur": "#22C55E",       // Grün für Natur (circle6)
    "Kreatives/Musik": "#8B5CF6", // Lila für Kreatives/Musik (circle7)
    "NW/Technik": "#06B6D4",  // Türkis für NW/Technik (circle8)
};

// Dummy data for rooms
const dummyRooms: Room[] = [
    {
        id: "1",
        name: "Klassenraum 101",
        building: "Hauptgebäude",
        floor: 1,
        capacity: 30,
        category: "Gruppenraum",
        isOccupied: true,
        groupName: "Klasse 5a",
        activityName: "Mathematik",
        supervisorName: "Fr. Schmidt",
        studentCount: 25
    },
    {
        id: "2",
        name: "Musiksaal",
        building: "Hauptgebäude",
        floor: 2,
        capacity: 40,
        category: "Kreatives/Musik",
        isOccupied: false
    },
    {
        id: "3",
        name: "Computerraum 1",
        building: "Nebengebäude",
        floor: 1,
        capacity: 25,
        category: "NW/Technik",
        isOccupied: true,
        groupName: "AG Informatik",
        activityName: "Programmierung",
        supervisorName: "Hr. Meyer",
        studentCount: 15
    },
    {
        id: "4",
        name: "Kunstraum",
        building: "Hauptgebäude",
        floor: 3,
        capacity: 30,
        category: "Kreatives/Musik",
        isOccupied: false
    },
    {
        id: "5",
        name: "Turnhalle",
        building: "Sportgebäude",
        floor: 0,
        capacity: 100,
        category: "Bewegen/Ruhe",
        isOccupied: true,
        groupName: "Klasse 7c",
        activityName: "Sportunterricht",
        supervisorName: "Hr. Müller",
        studentCount: 28
    },
    {
        id: "6",
        name: "Klassenraum 102",
        building: "Hauptgebäude",
        floor: 1,
        capacity: 30,
        category: "Gruppenraum",
        isOccupied: false
    },
    {
        id: "7",
        name: "Klassenraum 201",
        building: "Hauptgebäude",
        floor: 2,
        capacity: 30,
        category: "Lernen",
        isOccupied: true,
        groupName: "Klasse 6b",
        activityName: "Deutsch",
        supervisorName: "Hr. Weber",
        studentCount: 27
    },
    {
        id: "8",
        name: "Bibliothek",
        building: "Hauptgebäude",
        floor: 1,
        capacity: 50,
        category: "Lernen",
        isOccupied: false
    },
    {
        id: "9",
        name: "Naturwissenschaftsraum",
        building: "Nebengebäude",
        floor: 2,
        capacity: 35,
        category: "NW/Technik",
        isOccupied: true,
        groupName: "Klasse 8a",
        activityName: "Chemie",
        supervisorName: "Fr. Klein",
        studentCount: 30
    },
    {
        id: "10",
        name: "Besprechungsraum",
        building: "Verwaltung",
        floor: 1,
        capacity: 15,
        category: "Gruppenraum",
        isOccupied: false
    },
    {
        id: "11",
        name: "Mensa",
        building: "Hauptgebäude",
        floor: 0,
        capacity: 200,
        category: "Hauswirtschaft",
        isOccupied: true,
        groupName: "Alle Klassen",
        activityName: "Mittagessen",
        studentCount: 150
    },
    {
        id: "12",
        name: "Aula",
        building: "Hauptgebäude",
        floor: 0,
        capacity: 300,
        category: "Spielen",
        isOccupied: false
    }
];

export default function RoomsPage() {
    const router = useRouter();
    const [rooms, setRooms] = useState<Room[]>(dummyRooms);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [searchFilter, setSearchFilter] = useState("");
    const [buildingFilter, setBuildingFilter] = useState<string | null>(null);
    const [floorFilter, setFloorFilter] = useState<string | null>(null);
    const [categoryFilter, setCategoryFilter] = useState<string | null>(null);
    const [occupiedFilter, setOccupiedFilter] = useState<string | null>(null);
    const [filteredRooms, setFilteredRooms] = useState<Room[]>(dummyRooms);

    // Apply filters function
    const applyFilters = () => {
        setLoading(true);

        let filtered = [...dummyRooms];

        // Apply search filter
        if (searchFilter) {
            const searchLower = searchFilter.toLowerCase();
            filtered = filtered.filter(room =>
                room.name.toLowerCase().includes(searchLower) ??
                (room.groupName && room.groupName.toLowerCase().includes(searchLower)) ??
                (room.activityName && room.activityName.toLowerCase().includes(searchLower))
            );
        }

        // Apply building filter
        if (buildingFilter) {
            filtered = filtered.filter(room => room.building === buildingFilter);
        }

        // Apply floor filter
        if (floorFilter) {
            filtered = filtered.filter(room => room.floor === parseInt(floorFilter));
        }

        // Apply category filter
        if (categoryFilter) {
            filtered = filtered.filter(room => room.category === categoryFilter);
        }

        // Apply occupied filter
        if (occupiedFilter) {
            const isOccupied = occupiedFilter === "true";
            filtered = filtered.filter(room => room.isOccupied === isOccupied);
        }

        setFilteredRooms(filtered);
        setLoading(false);
    };

    // Effect to apply filters when filter values change
    useEffect(() => {
        const timer = setTimeout(() => {
            applyFilters();
        }, 300);

        return () => clearTimeout(timer);
    }, [searchFilter, buildingFilter, floorFilter, categoryFilter, occupiedFilter]);

    // Handle room selection
    const handleSelectRoom = (room: Room) => {
        router.push(`/rooms/${room.id}`);
    };

    // Get all unique buildings for filter dropdown
    const uniqueBuildings = Array.from(
        new Set(dummyRooms.map((room) => room.building).filter(Boolean))
    );

    // Get all unique floors for filter dropdown
    const uniqueFloors = Array.from(
        new Set(dummyRooms.map((room) => room.floor.toString()))
    ).sort((a, b) => parseInt(a) - parseInt(b));

    // Get all unique categories for filter dropdown
    const uniqueCategories = Array.from(
        new Set(dummyRooms.map((room) => room.category))
    );

    // Reset all filters
    const resetFilters = () => {
        setSearchFilter("");
        setBuildingFilter(null);
        setFloorFilter(null);
        setCategoryFilter(null);
        setOccupiedFilter(null);
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
                        <h1 className="mb-8 text-4xl font-bold text-gray-900">Raumübersicht</h1>

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
                                            placeholder="Raumname"
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

                                {/* Building Filter */}
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
                                            <option key={building} value={building as string}>
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

                                {/* Floor Filter */}
                                <div className="relative">
                                    <label className="block text-sm font-medium text-gray-700">
                                        Etage
                                    </label>
                                    <select
                                        value={floorFilter ?? ""}
                                        onChange={(e) => setFloorFilter(e.target.value || null)}
                                        className={dropdownClass}
                                    >
                                        <option value="">Alle Etagen</option>
                                        {uniqueFloors.map((floor) => (
                                            <option key={floor} value={floor}>
                                                {floor}
                                            </option>
                                        ))}
                                    </select>
                                    <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                                        <svg className="h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                                            <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                        </svg>
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

                                {/* Occupied Filter */}
                                <div className="relative">
                                    <label className="block text-sm font-medium text-gray-700">
                                        Status
                                    </label>
                                    <select
                                        value={occupiedFilter ?? ""}
                                        onChange={(e) => setOccupiedFilter(e.target.value || null)}
                                        className={dropdownClass}
                                    >
                                        <option value="">Alle</option>
                                        <option value="true">Belegt</option>
                                        <option value="false">Frei</option>
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

                        {/* Room Cards Grid */}
                        {filteredRooms.length > 0 ? (
                            <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                                {filteredRooms.map((room) => (
                                    <div
                                        key={room.id}
                                        onClick={() => handleSelectRoom(room)}
                                        className="cursor-pointer overflow-hidden rounded-lg bg-white shadow-md transition-all duration-200 hover:translate-y-[-2px] hover:shadow-lg"
                                    >
                                        {/* Top colored bar based on room category */}
                                        <div
                                            className="h-2"
                                            style={{ backgroundColor: categoryColors[room.category] || "#6B7280" }}
                                        />
                                        <div className="p-4">
                                            <div className="mb-2 flex items-center justify-between">
                                                <h3 className="text-lg font-semibold text-gray-800">{room.name}</h3>
                                                <span
                                                    className={`inline-flex h-3 w-3 rounded-full ${
                                                        room.isOccupied ? "bg-red-500" : "bg-green-500"
                                                    }`}
                                                    title={room.isOccupied ? "Belegt" : "Frei"}
                                                ></span>
                                            </div>
                                            <div className="text-sm text-gray-600">
                                                <p className="mb-1">Gebäude: {room.building || "Unbekannt"}, Etage {room.floor}</p>
                                                <p>Kapazität: {room.capacity} Personen</p>
                                            </div>
                                            {/* Zeige Kategorie, Aktivität etc. für alle Räume an, nicht nur für belegte */}
                                            <div className="mt-2 border-t border-gray-100 pt-2">
                                                <p className="text-sm font-medium text-gray-700">
                                                    <span className="block flex items-center">
                                                        <span
                                                            className="inline-block h-3 w-3 rounded-full mr-1.5"
                                                            style={{ backgroundColor: categoryColors[room.category] || "#6B7280" }}
                                                        ></span>
                                                        Kategorie: {room.category}
                                                    </span>
                                                    {room.isOccupied && (
                                                        <>
                                                            {room.groupName && (
                                                                <span className="block mt-1">Gruppe: {room.groupName}</span>
                                                            )}
                                                            {room.activityName && (
                                                                <span className="block mt-1">Aktivität: {room.activityName}</span>
                                                            )}
                                                            {room.studentCount !== undefined && room.capacity > 0 && (
                                                                <span className="block mt-1">
                                                                    Belegung: {room.studentCount} / {room.capacity}
                                                                    <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200">
                                                                        <div
                                                                            className="h-full bg-gray-600"
                                                                            style={{
                                                                                width: `${Math.min(
                                                                                    (room.studentCount / room.capacity) * 100,
                                                                                    100
                                                                                )}%`,
                                                                            }}
                                                                        ></div>
                                                                    </div>
                                                                </span>
                                                            )}
                                                        </>
                                                    )}
                                                </p>
                                            </div>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        ) : (
                            <div className="flex h-40 items-center justify-center rounded-lg border-2 border-dashed border-gray-300 bg-white p-6">
                                <div className="text-center">
                                    <p className="text-sm font-medium text-gray-500">
                                        {searchFilter || buildingFilter || floorFilter || categoryFilter || occupiedFilter
                                            ? "Keine Räume gefunden, die Ihren Filterkriterien entsprechen."
                                            : "Keine Räume verfügbar."}
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