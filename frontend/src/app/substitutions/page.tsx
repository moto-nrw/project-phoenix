"use client";

import { useState, useEffect, useCallback } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { Header } from "~/components/dashboard/header";
import { Sidebar } from "~/components/dashboard/sidebar";
import { Input } from "~/components/ui";
import { Alert } from "~/components/ui/alert";
import { BackgroundWrapper } from "~/components/background-wrapper";

// Teacher type based on DB schema seen in the migrations
interface Teacher {
    id: string;
    first_name: string;
    second_name: string;
    regular_group?: string;  // OGS-Gruppe, von der die Person Leiter/in ist
    role?: string;           // Position (e.g. "Gruppenleiter/in")
    in_substitution: boolean;
    current_group?: string;  // Wenn in Vertretung, für welche Gruppe
}

// Group type based on DB schema
interface Group {
    id: string;
    name: string;
    regular_staff_id?: string;
    regular_staff_name?: string;
    substitute_staff_id?: string;
    substitute_staff_name?: string;
}

// Demo data for teachers
const exampleTeachers: Teacher[] = [
    {
        id: "1",
        first_name: "Anna",
        second_name: "Lehmann",
        regular_group: "Bären",
        role: "Gruppenleiter/in",
        in_substitution: false
    },
    {
        id: "2",
        first_name: "Thomas",
        second_name: "Meyer",
        regular_group: "Wölfe",
        role: "Gruppenleiter/in",
        in_substitution: true,
        current_group: "Bären"
    },
    {
        id: "3",
        first_name: "Sarah",
        second_name: "Schneider",
        regular_group: "Füchse",
        role: "Gruppenleiter/in",
        in_substitution: false
    },
    {
        id: "4",
        first_name: "Michael",
        second_name: "Fischer",
        role: "Pädagogische Fachkraft",
        in_substitution: false
    },
    {
        id: "5",
        first_name: "Laura",
        second_name: "Weber",
        regular_group: "Eulen",
        role: "Gruppenleiter/in",
        in_substitution: true,
        current_group: "Eulen"
    },
    {
        id: "6",
        first_name: "Daniel",
        second_name: "Schmidt",
        role: "Pädagogische Fachkraft",
        in_substitution: false
    }
];

// Demo data for groups
const exampleGroups: Group[] = [
    {
        id: "g1",
        name: "Bären",
        regular_staff_id: "8",
        regular_staff_name: "Petra Müller",
        substitute_staff_id: "2",
        substitute_staff_name: "Thomas Meyer"
    },
    {
        id: "g2",
        name: "Füchse",
        regular_staff_id: "9",
        regular_staff_name: "Martin Wagner",
        substitute_staff_id: "",
        substitute_staff_name: ""
    },
    {
        id: "g3",
        name: "Eulen",
        regular_staff_id: "10",
        regular_staff_name: "Julia Hoffmann",
        substitute_staff_id: "5",
        substitute_staff_name: "Laura Weber"
    },
    {
        id: "g4",
        name: "Wölfe",
        regular_staff_id: "11",
        regular_staff_name: "Stefan Koch",
        substitute_staff_id: "",
        substitute_staff_name: ""
    }
];

export default function SubstitutionPage() {
    const router = useRouter();
    const { data: session, status } = useSession({
        required: true,
        onUnauthenticated() {
            router.push("/login");
        },
    });

    // States
    const [teachers, setTeachers] = useState<Teacher[]>([]);
    const [groups, setGroups] = useState<Group[]>([]);
    const [searchTerm, setSearchTerm] = useState("");
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    // Popup states
    const [showPopup, setShowPopup] = useState(false);
    const [selectedTeacher, setSelectedTeacher] = useState<Teacher | null>(null);
    const [selectedGroup, setSelectedGroup] = useState("");
    const [substitutionDays, setSubstitutionDays] = useState(1);

    // Fetch teachers data
    const fetchTeachers = useCallback(async (filters?: {
        search?: string;
    }) => {
        setIsLoading(true);
        setError(null);

        try {
            // Simulate API request with example data
            setTimeout(() => {
                let filteredTeachers = [...exampleTeachers];

                // Filter by search term
                if (filters?.search) {
                    const searchLower = filters.search.toLowerCase();
                    filteredTeachers = filteredTeachers.filter(teacher =>
                        teacher.first_name.toLowerCase().includes(searchLower) ||
                        teacher.second_name.toLowerCase().includes(searchLower)
                    );
                }

                setTeachers(filteredTeachers);
                setIsLoading(false);
            }, 500);
        } catch (err) {
            console.error("Error fetching teachers:", err);
            setError("Fehler beim Laden der Lehrerdaten.");
            setTeachers([]);
            setIsLoading(false);
        }
    }, []);

    // Fetch groups data
    const fetchGroups = useCallback(async () => {
        setIsLoading(true);
        setError(null);

        try {
            // Simulate API request with example data
            setTimeout(() => {
                setGroups(exampleGroups);
                setIsLoading(false);
            }, 500);
        } catch (err) {
            console.error("Error fetching groups:", err);
            setError("Fehler beim Laden der Gruppendaten.");
            setGroups([]);
            setIsLoading(false);
        }
    }, []);

    // Load initial data
    useEffect(() => {
        void fetchTeachers();
        void fetchGroups();
    }, [fetchTeachers, fetchGroups]);

    // Handle search
    const handleSearch = () => {
        const filters: {
            search?: string;
        } = {};

        if (searchTerm.trim()) {
            filters.search = searchTerm.trim();
        }

        void fetchTeachers(filters);
    };

    // Handle filter reset
    const handleFilterReset = () => {
        setSearchTerm("");
        void fetchTeachers();
    };

    // Open popup for substitution assignment
    const openSubstitutionPopup = (teacher: Teacher) => {
        setSelectedTeacher(teacher);
        setSelectedGroup("");
        setSubstitutionDays(1);
        setShowPopup(true);
    };

    // Close popup
    const closePopup = () => {
        setShowPopup(false);
        setSelectedTeacher(null);
    };

    // Handle substitution assignment
    const handleAssignSubstitution = () => {
        if (!selectedTeacher || !selectedGroup) {
            setError("Bitte wählen Sie eine Gruppe aus.");
            return;
        }

        // Simulate substitution assignment
        console.log(`Assigning ${selectedTeacher.first_name} ${selectedTeacher.second_name} to ${selectedGroup} for ${substitutionDays} days`);

        // Update teacher's substitution status
        const updatedTeachers = teachers.map(teacher => {
            if (teacher.id === selectedTeacher.id) {
                return {
                    ...teacher,
                    in_substitution: true,
                    current_group: selectedGroup
                };
            }
            return teacher;
        });

        // Update group's substitute data
        const updatedGroups = groups.map(group => {
            if (group.name === selectedGroup) {
                return {
                    ...group,
                    substitute_staff_id: selectedTeacher.id,
                    substitute_staff_name: `${selectedTeacher.first_name} ${selectedTeacher.second_name}`
                };
            }
            return group;
        });

        setTeachers(updatedTeachers);
        setGroups(updatedGroups);
        closePopup();
    };

    if (status === "loading") {
        return (
            <div className="flex min-h-screen items-center justify-center">
                <p>Loading...</p>
            </div>
        );
    }

    // Common class for all dropdowns to ensure consistent height
    const dropdownClass = "mt-1 block w-full rounded-lg border-0 px-4 py-3 h-12 shadow-sm ring-1 ring-gray-200 transition-all duration-200 hover:bg-gray-50/50 hover:ring-gray-300 focus:ring-2 focus:ring-teal-500 focus:outline-none appearance-none pr-8";

    return (
        <BackgroundWrapper>
            <div className="min-h-screen">
                {/* Header */}
                <Header userName={session?.user?.name ?? "Benutzer"} />

                <div className="flex">
                    {/* Sidebar */}
                    <Sidebar />

                    {/* Main Content */}
                    <main className="flex-1 p-8">
                        <div className="mx-auto max-w-7xl">
                            <h1 className="mb-8 text-4xl font-bold text-gray-900">Vertretungsverwaltung</h1>

                            {/* Search Panel */}
                            <div className="mb-8 overflow-hidden rounded-xl bg-white/90 p-6 shadow-md backdrop-blur-sm">
                                <h2 className="mb-4 text-xl font-bold text-gray-800">Suchkriterien</h2>

                                <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
                                    {/* Name Search */}
                                    <Input
                                        label="Name"
                                        name="searchTerm"
                                        placeholder="Vor- oder Nachname"
                                        value={searchTerm}
                                        onChange={(e) => setSearchTerm(e.target.value)}
                                        className="h-12"
                                    />

                                    {/* Search Actions */}
                                    <div className="flex items-end space-x-3">
                                        <button
                                            onClick={handleFilterReset}
                                            className="rounded-lg border border-gray-300 bg-white px-4 py-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50"
                                        >
                                            Zurücksetzen
                                        </button>
                                        <button
                                            onClick={handleSearch}
                                            disabled={isLoading}
                                            className="rounded-lg bg-gradient-to-r from-teal-500 to-blue-600 px-6 py-3 text-sm font-medium text-white shadow-sm transition-all hover:from-teal-600 hover:to-blue-700 hover:shadow-md disabled:opacity-70"
                                        >
                                            {isLoading ? "Suche läuft..." : "Suchen"}
                                        </button>
                                    </div>
                                </div>
                            </div>

                            {/* Teachers Results Section */}
                            <div className="mb-8 overflow-hidden rounded-xl bg-white/90 p-6 shadow-md backdrop-blur-sm">
                                <h2 className="mb-6 text-xl font-bold text-gray-800">Verfügbare Lehrkräfte</h2>

                                {error && (
                                    <div className="mb-6">
                                        <Alert type="error" message={error} />
                                    </div>
                                )}

                                {isLoading ? (
                                    <div className="py-8 text-center">
                                        <p className="text-gray-500">Suche läuft...</p>
                                    </div>
                                ) : (
                                    <div className="space-y-2">
                                        {teachers.length > 0 ? (
                                            teachers.map((teacher) => (
                                                <div
                                                    key={teacher.id}
                                                    className="group rounded-lg border border-gray-100 bg-white p-4 shadow-sm transition-all duration-200 hover:border-blue-200 hover:shadow-md"
                                                >
                                                    <div className="flex items-center justify-between">
                                                        <div className="flex items-center space-x-3">
                                                            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-r from-teal-400 to-blue-500 font-medium text-white">
                                                                {(teacher.first_name?.charAt(0) || "L").toUpperCase()}
                                                            </div>

                                                            <div className="flex flex-col">
                                                                <span className="font-medium text-gray-900">
                                                                    {teacher.first_name} {teacher.second_name}
                                                                </span>
                                                                <span className="text-sm text-gray-500">
                                                                    {teacher.role}{teacher.regular_group ? ` | OGS-Gruppe: ${teacher.regular_group}` : ''}
                                                                </span>
                                                            </div>
                                                        </div>

                                                        <div className="flex items-center space-x-4">
                                                            {/* Status indicator */}
                                                            <div className="flex items-center">
                                                                <span className={`h-3 w-3 rounded-full ${teacher.in_substitution ? "bg-orange-500" : "bg-green-500"} mr-2`}></span>
                                                                <span className="text-sm text-gray-600">
                                                                    {teacher.in_substitution
                                                                        ? `In Vertretung: ${teacher.current_group}`
                                                                        : "Verfügbar"}
                                                                </span>
                                                            </div>

                                                            <button
                                                                onClick={() => openSubstitutionPopup(teacher)}
                                                                disabled={teacher.in_substitution}
                                                                className={`rounded-lg px-4 py-2 text-sm font-medium shadow-sm transition-all ${
                                                                    teacher.in_substitution
                                                                        ? "bg-gray-200 text-gray-500 cursor-not-allowed"
                                                                        : "bg-gradient-to-r from-teal-500 to-blue-600 text-white hover:from-teal-600 hover:to-blue-700 hover:shadow-md"
                                                                }`}
                                                            >
                                                                {teacher.in_substitution ? "In Vertretung" : "Vertretung zuweisen"}
                                                            </button>
                                                        </div>
                                                    </div>
                                                </div>
                                            ))
                                        ) : (
                                            <div className="py-8 text-center">
                                                <p className="text-gray-500">Keine Lehrkräfte gefunden. Bitte passen Sie Ihre Suchkriterien an.</p>
                                            </div>
                                        )}
                                    </div>
                                )}
                            </div>

                            {/* Current Substitutions Section */}
                            <div className="overflow-hidden rounded-xl bg-white/90 p-6 shadow-md backdrop-blur-sm">
                                <h2 className="mb-6 text-xl font-bold text-gray-800">Aktuelle Vertretungen</h2>

                                <div className="space-y-2">
                                    {groups.some(group => group.substitute_staff_id) ? (
                                        groups
                                            .filter(group => group.substitute_staff_id)
                                            .map((group) => (
                                                <div
                                                    key={group.id}
                                                    className="rounded-lg border border-gray-100 bg-white p-4 shadow-sm"
                                                >
                                                    <div className="flex items-center justify-between">
                                                        <div className="flex items-center space-x-3">
                                                            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-r from-purple-400 to-indigo-500 font-medium text-white">
                                                                {(group.name?.charAt(0) || "G").toUpperCase()}
                                                            </div>

                                                            <div className="flex flex-col">
                                                                <span className="font-medium text-gray-900">
                                                                    Gruppe: {group.name}
                                                                </span>
                                                                <span className="text-sm text-gray-500">
                                                                    Reguläre Lehrkraft: {group.regular_staff_name}
                                                                </span>
                                                            </div>
                                                        </div>

                                                        <div className="flex flex-col">
                                                            <span className="font-medium text-gray-900">
                                                                Vertretung: {group.substitute_staff_name}
                                                            </span>
                                                            <button
                                                                onClick={() => {
                                                                    // Simulate ending substitution
                                                                    const updatedGroups = groups.map(g => {
                                                                        if (g.id === group.id) {
                                                                            return {
                                                                                ...g,
                                                                                substitute_staff_id: "",
                                                                                substitute_staff_name: ""
                                                                            };
                                                                        }
                                                                        return g;
                                                                    });

                                                                    const updatedTeachers = teachers.map(teacher => {
                                                                        if (teacher.id === group.substitute_staff_id) {
                                                                            return {
                                                                                ...teacher,
                                                                                in_substitution: false,
                                                                                current_group: undefined
                                                                            };
                                                                        }
                                                                        return teacher;
                                                                    });

                                                                    setGroups(updatedGroups);
                                                                    setTeachers(updatedTeachers);
                                                                }}
                                                                className="mt-2 rounded-lg bg-red-100 px-4 py-1 text-sm font-medium text-red-600 hover:bg-red-200 transition-colors"
                                                            >
                                                                Vertretung beenden
                                                            </button>
                                                        </div>
                                                    </div>
                                                </div>
                                            ))
                                    ) : (
                                        <div className="py-8 text-center">
                                            <p className="text-gray-500">Keine aktiven Vertretungen vorhanden.</p>
                                        </div>
                                    )}
                                </div>
                            </div>
                        </div>
                    </main>
                </div>

                {/* Substitution Assignment Popup */}
                {showPopup && selectedTeacher && (
                    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 backdrop-blur-sm">
                        <div className="w-full max-w-md rounded-xl bg-white p-6 border border-gray-200 shadow-2xl">
                            <h2 className="mb-4 text-xl font-bold text-gray-800">Vertretung zuweisen</h2>

                            <div className="mb-6">
                                <p className="mb-2 text-sm font-medium text-gray-700">Lehrkraft:</p>
                                <p className="font-medium text-gray-900">{selectedTeacher.first_name} {selectedTeacher.second_name}</p>
                            </div>

                            {/* Group selection */}
                            <div className="relative">
                                <label className="block text-sm font-medium text-gray-700 mb-1">
                                    OGS-Gruppe auswählen
                                </label>
                                <select
                                    value={selectedGroup}
                                    onChange={(e) => setSelectedGroup(e.target.value)}
                                    className={dropdownClass}
                                >
                                    <option value="">Gruppe auswählen...</option>
                                    {groups.map((group) => (
                                        <option key={group.id} value={group.name}>{group.name}</option>
                                    ))}
                                </select>
                                <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                                    <svg className="h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                                        <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                    </svg>
                                </div>
                            </div>

                            {/* Days selection */}
                            <div className="mb-6">
                                <label className="block text-sm font-medium text-gray-700 mb-1">
                                    Anzahl der Tage
                                </label>
                                <input
                                    type="number"
                                    min="1"
                                    max="30"
                                    value={substitutionDays}
                                    onChange={(e) => setSubstitutionDays(parseInt(e.target.value) || 1)}
                                    className="mt-1 block w-full rounded-lg border-0 px-4 py-3 shadow-sm ring-1 ring-gray-200 focus:ring-2 focus:ring-teal-500 focus:outline-none"
                                />
                            </div>

                            {/* Actions */}
                            <div className="flex justify-end space-x-3">
                                <button
                                    onClick={closePopup}
                                    className="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50"
                                >
                                    Abbrechen
                                </button>
                                <button
                                    onClick={handleAssignSubstitution}
                                    className="rounded-lg bg-gradient-to-r from-teal-500 to-blue-600 px-6 py-2 text-sm font-medium text-white shadow-sm transition-all hover:from-teal-600 hover:to-blue-700 hover:shadow-md"
                                >
                                    Zuweisen
                                </button>
                            </div>
                        </div>
                    </div>
                )}
            </div>
        </BackgroundWrapper>
    );
}