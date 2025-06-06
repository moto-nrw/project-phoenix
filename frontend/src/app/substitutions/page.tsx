"use client";

import { useState, useEffect, useCallback } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { ResponsiveLayout } from "@/components/dashboard";
import { Input } from "~/components/ui";
import { Alert } from "~/components/ui/alert";
import { substitutionService } from "~/lib/substitution-api";
import { groupService } from "~/lib/api";
import type { Group } from "~/lib/api";
import type { Substitution, TeacherAvailability } from "~/lib/substitution-helpers";
import { formatTeacherName, getTeacherStatus } from "~/lib/substitution-helpers";


export default function SubstitutionPage() {
    const router = useRouter();
    const { status } = useSession({
        required: true,
        onUnauthenticated() {
            router.push("/");
        },
    });

    // States
    const [teachers, setTeachers] = useState<TeacherAvailability[]>([]);
    const [groups, setGroups] = useState<Group[]>([]);
    const [activeSubstitutions, setActiveSubstitutions] = useState<Substitution[]>([]);
    const [searchTerm, setSearchTerm] = useState("");
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);


    // Popup states
    const [showPopup, setShowPopup] = useState(false);
    const [selectedTeacher, setSelectedTeacher] = useState<TeacherAvailability | null>(null);
    const [selectedGroup, setSelectedGroup] = useState("");
    const [substitutionDays, setSubstitutionDays] = useState(1);

    // Fetch teachers data
    const fetchTeachers = useCallback(async (filters?: {
        search?: string;
    }) => {
        setIsLoading(true);
        setError(null);

        try {
            const availableTeachers = await substitutionService.fetchAvailableTeachers(
                new Date(), // Current date
                filters?.search
            );
            setTeachers(availableTeachers);
        } catch (err) {
            console.error("Error fetching teachers:", err);
            setError("Fehler beim Laden der Lehrerdaten.");
            setTeachers([]);
        } finally {
            setIsLoading(false);
        }
    }, []);

    // Fetch groups data
    const fetchGroups = useCallback(async () => {
        try {
            const allGroups = await groupService.getGroups();
            setGroups(allGroups);
        } catch (err) {
            console.error("Error fetching groups:", err);
            setError("Fehler beim Laden der Gruppendaten.");
            setGroups([]);
        }
    }, []);

    // Fetch active substitutions
    const fetchActiveSubstitutions = useCallback(async () => {
        try {
            const substitutions = await substitutionService.fetchActiveSubstitutions(new Date());
            setActiveSubstitutions(substitutions);
        } catch (err) {
            console.error("Error fetching active substitutions:", err);
            // Don't set error for substitutions, just log it
        }
    }, []);

    // Load initial data
    useEffect(() => {
        void fetchTeachers();
        void fetchGroups();
        void fetchActiveSubstitutions();
    }, [fetchTeachers, fetchGroups, fetchActiveSubstitutions]);

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
    const openSubstitutionPopup = (teacher: TeacherAvailability) => {
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
    const handleAssignSubstitution = async () => {
        if (!selectedTeacher || !selectedGroup) {
            setError("Bitte wählen Sie eine Gruppe aus.");
            return;
        }

        try {
            setIsLoading(true);
            setError(null);

            // Find the selected group to get its ID and representative
            const group = groups.find(g => g.name === selectedGroup);
            if (!group) {
                setError("Gruppe nicht gefunden.");
                return;
            }

            // For general group coverage, we don't need to specify who is being replaced
            // Pass null for regularStaffId to indicate this is general coverage, not a specific replacement
            const regularStaffId = null;

            // Calculate end date based on substitution days
            const startDate = new Date();
            const endDate = new Date();
            endDate.setDate(endDate.getDate() + substitutionDays - 1);

            // Create the substitution
            await substitutionService.createSubstitution(
                group.id,
                regularStaffId,
                selectedTeacher.id,
                startDate,
                endDate,
                "Vertretung", // reason
                `Vertretung für ${substitutionDays} Tag(e)` // notes
            );

            // Refresh data
            await Promise.all([
                fetchTeachers(),
                fetchActiveSubstitutions()
            ]);

            closePopup();
        } catch (err) {
            console.error("Error creating substitution:", err);
            setError("Fehler beim Zuweisen der Vertretung.");
        } finally {
            setIsLoading(false);
        }
    };

    if (status === "loading") {
        return (
            <div className="flex min-h-screen items-center justify-center">
                <p>Loading...</p>
            </div>
        );
    }


    return (
        <ResponsiveLayout>
            <div className="max-w-7xl mx-auto">
                {/* Mobile-optimized Header */}
                <div className="mb-4 md:mb-8">
                    <h1 className="text-2xl md:text-4xl font-bold text-gray-900">Vertretungsverwaltung</h1>
                    <p className="mt-1 text-sm md:text-base text-gray-600">Verwalte Lehrkräfte und Vertretungszuweisungen</p>
                </div>

                {/* Mobile Search Bar - Always Visible */}
                <div className="mb-4 md:hidden">
                    <Input
                        label="Schnellsuche"
                        name="searchTerm"
                        placeholder="Lehrkraft suchen..."
                        value={searchTerm}
                        onChange={(e) => setSearchTerm(e.target.value)}
                        className="text-base" // Prevent iOS zoom
                    />
                </div>

                {/* Search Panel - Desktop always visible, Mobile shows basic search only */}
                <div className="mb-6 md:mb-8 overflow-hidden rounded-xl bg-white/90 shadow-md backdrop-blur-sm hidden md:block">
                    <div className="p-4 md:p-6">
                        <h2 className="mb-4 text-lg md:text-xl font-bold text-gray-800">Suchkriterien</h2>

                        <div className="grid grid-cols-1 gap-4 md:gap-6 md:grid-cols-2">
                            {/* Name Search - Desktop only */}
                            <div>
                                <Input
                                    label="Name"
                                    name="searchTerm"
                                    placeholder="Vor- oder Nachname"
                                    value={searchTerm}
                                    onChange={(e) => setSearchTerm(e.target.value)}
                                    className="h-12 text-base"
                                />
                            </div>

                            {/* Search Actions */}
                            <div className="flex flex-col md:flex-row md:items-end gap-3 md:space-x-3">
                                <button
                                    onClick={handleFilterReset}
                                    className="rounded-lg border border-gray-300 bg-white px-4 py-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 order-2 md:order-1"
                                >
                                    Zurücksetzen
                                </button>
                                <button
                                    onClick={handleSearch}
                                    disabled={isLoading}
                                    className="rounded-lg bg-gradient-to-r from-teal-500 to-blue-600 px-6 py-3 text-sm font-medium text-white shadow-sm transition-all hover:from-teal-600 hover:to-blue-700 hover:shadow-md disabled:opacity-70 order-1 md:order-2"
                                >
                                    {isLoading ? "Suche läuft..." : "Suchen"}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>

                {/* Mobile Search Actions */}
                <div className="mb-6 md:hidden flex gap-3">
                    <button
                        onClick={handleFilterReset}
                        className="flex-1 rounded-lg border border-gray-300 bg-white px-4 py-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50"
                    >
                        Zurücksetzen
                    </button>
                    <button
                        onClick={handleSearch}
                        disabled={isLoading}
                        className="flex-1 rounded-lg bg-gradient-to-r from-teal-500 to-blue-600 px-6 py-3 text-sm font-medium text-white shadow-sm transition-all hover:from-teal-600 hover:to-blue-700 hover:shadow-md disabled:opacity-70"
                    >
                        {isLoading ? "Suche läuft..." : "Suchen"}
                    </button>
                </div>

                {/* Teachers Results Section */}
                <div className="mb-6 md:mb-8 overflow-hidden rounded-xl bg-white/90 shadow-md backdrop-blur-sm">
                    <div className="p-4 md:p-6">
                        <h2 className="mb-4 md:mb-6 text-lg md:text-xl font-bold text-gray-800">Verfügbare Lehrkräfte</h2>

                        {error && (
                            <div className="mb-4 md:mb-6">
                                <Alert type="error" message={error} />
                            </div>
                        )}

                        {isLoading ? (
                            <div className="py-8 text-center">
                                <div className="flex flex-col items-center gap-4">
                                    <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-t-2 border-teal-500"></div>
                                    <p className="text-gray-600">Suche läuft...</p>
                                </div>
                            </div>
                        ) : (
                            <div className="space-y-3">
                                {teachers.length > 0 ? (
                                    teachers.map((teacher) => (
                                        <div
                                            key={teacher.id}
                                            className="group rounded-lg border border-gray-100 bg-white p-4 shadow-sm transition-all duration-200 hover:border-blue-200 hover:shadow-md"
                                        >
                                            {/* Mobile Layout - Stacked */}
                                            <div className="md:hidden">
                                                <div className="flex items-start justify-between mb-3">
                                                    <div className="flex items-center space-x-3 flex-1 min-w-0">
                                                        <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-r from-teal-400 to-blue-500 font-medium text-white flex-shrink-0">
                                                            {(teacher.firstName?.charAt(0) || "L").toUpperCase()}
                                                        </div>
                                                        <div className="flex flex-col min-w-0 flex-1">
                                                            <span className="font-medium text-gray-900 truncate">
                                                                {formatTeacherName(teacher)}
                                                            </span>
                                                            <span className="text-sm text-gray-500 truncate">
                                                                {teacher.role}
                                                            </span>
                                                            {teacher.regularGroup && (
                                                                <span className="text-sm text-gray-500 truncate">
                                                                    OGS-Gruppe: {teacher.regularGroup}
                                                                </span>
                                                            )}
                                                        </div>
                                                    </div>
                                                </div>

                                                <div className="flex items-center justify-between">
                                                    <div className="flex items-center">
                                                        <span className={`h-3 w-3 rounded-full ${teacher.inSubstitution ? "bg-orange-500" : "bg-green-500"} mr-2`}></span>
                                                        <span className="text-sm text-gray-600">
                                                            {getTeacherStatus(teacher)}
                                                        </span>
                                                    </div>
                                                </div>

                                                <div className="mt-3">
                                                    <button
                                                        onClick={() => openSubstitutionPopup(teacher)}
                                                        disabled={teacher.inSubstitution}
                                                        className={`w-full rounded-lg px-4 py-3 text-sm font-medium shadow-sm transition-all ${
                                                            teacher.inSubstitution
                                                                ? "bg-gray-200 text-gray-500 cursor-not-allowed"
                                                                : "bg-gradient-to-r from-teal-500 to-blue-600 text-white hover:from-teal-600 hover:to-blue-700 hover:shadow-md active:scale-[0.98]"
                                                        }`}
                                                    >
                                                        {teacher.inSubstitution ? "In Vertretung" : "Vertretung zuweisen"}
                                                    </button>
                                                </div>
                                            </div>

                                            {/* Desktop Layout - Horizontal */}
                                            <div className="hidden md:flex items-center justify-between">
                                                <div className="flex items-center space-x-3">
                                                    <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-r from-teal-400 to-blue-500 font-medium text-white">
                                                        {(teacher.firstName?.charAt(0) || "L").toUpperCase()}
                                                    </div>

                                                    <div className="flex flex-col">
                                                        <span className="font-medium text-gray-900">
                                                            {formatTeacherName(teacher)}
                                                        </span>
                                                        <span className="text-sm text-gray-500">
                                                            {teacher.role}{teacher.regularGroup ? ` | OGS-Gruppe: ${teacher.regularGroup}` : ''}
                                                        </span>
                                                    </div>
                                                </div>

                                                <div className="flex items-center space-x-4">
                                                    {/* Status indicator */}
                                                    <div className="flex items-center">
                                                        <span className={`h-3 w-3 rounded-full ${teacher.inSubstitution ? "bg-orange-500" : "bg-green-500"} mr-2`}></span>
                                                        <span className="text-sm text-gray-600">
                                                            {getTeacherStatus(teacher)}
                                                        </span>
                                                    </div>

                                                    <button
                                                        onClick={() => openSubstitutionPopup(teacher)}
                                                        disabled={teacher.inSubstitution}
                                                        className={`rounded-lg px-4 py-2 text-sm font-medium shadow-sm transition-all ${
                                                            teacher.inSubstitution
                                                                ? "bg-gray-200 text-gray-500 cursor-not-allowed"
                                                                : "bg-gradient-to-r from-teal-500 to-blue-600 text-white hover:from-teal-600 hover:to-blue-700 hover:shadow-md"
                                                        }`}
                                                    >
                                                        {teacher.inSubstitution ? "In Vertretung" : "Vertretung zuweisen"}
                                                    </button>
                                                </div>
                                            </div>
                                        </div>
                                    ))
                                ) : (
                                    <div className="py-8 text-center">
                                        <div className="flex flex-col items-center gap-4">
                                            <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                            </svg>
                                            <div>
                                                <h3 className="text-lg font-medium text-gray-900">Keine Lehrkräfte gefunden</h3>
                                                <p className="text-gray-600">Versuche deine Suchkriterien anzupassen.</p>
                                            </div>
                                        </div>
                                    </div>
                                )}
                            </div>
                        )}
                    </div>
                </div>

                {/* Current Substitutions Section */}
                <div className="overflow-hidden rounded-xl bg-white/90 shadow-md backdrop-blur-sm">
                    <div className="p-4 md:p-6">
                        <h2 className="mb-4 md:mb-6 text-lg md:text-xl font-bold text-gray-800">Aktuelle Vertretungen</h2>

                        <div className="space-y-3">
                            {activeSubstitutions.length > 0 ? (
                                activeSubstitutions.map((substitution) => {
                                    // Find the group for this substitution
                                    const group = groups.find(g => g.id === substitution.groupId);
                                    if (!group) return null;
                                    
                                    return (
                                        <div
                                            key={substitution.id}
                                            className="rounded-lg border border-gray-100 bg-white p-4 shadow-sm"
                                        >
                                            {/* Mobile Layout - Stacked */}
                                            <div className="md:hidden">
                                                <div className="flex items-center space-x-3 mb-3">
                                                    <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-r from-purple-400 to-indigo-500 font-medium text-white flex-shrink-0">
                                                        {(group.name?.charAt(0) || "G").toUpperCase()}
                                                    </div>
                                                    <div className="flex flex-col min-w-0 flex-1">
                                                        <span className="font-medium text-gray-900">
                                                            Gruppe: {group.name}
                                                        </span>
                                                        <span className="text-sm text-gray-500">
                                                            {substitution.groupName ?? group.name}
                                                        </span>
                                                    </div>
                                                </div>

                                                <div className="mb-3">
                                                    <span className="text-sm text-gray-600">Vertretung durch:</span>
                                                    <span className="block font-medium text-gray-900">
                                                        {substitution.substituteStaffName}
                                                    </span>
                                                </div>

                                                <button
                                                    onClick={async () => {
                                                        try {
                                                            setIsLoading(true);
                                                            await substitutionService.deleteSubstitution(substitution.id);
                                                            await Promise.all([
                                                                fetchTeachers(),
                                                                fetchActiveSubstitutions()
                                                            ]);
                                                        } catch (err) {
                                                            console.error("Error ending substitution:", err);
                                                            setError("Fehler beim Beenden der Vertretung.");
                                                        } finally {
                                                            setIsLoading(false);
                                                        }
                                                    }}
                                                    className="w-full rounded-lg bg-red-100 px-4 py-2 text-sm font-medium text-red-600 hover:bg-red-200 transition-colors active:scale-[0.98]"
                                                    disabled={isLoading}
                                                >
                                                    Vertretung beenden
                                                </button>
                                            </div>

                                            {/* Desktop Layout - Horizontal */}
                                            <div className="hidden md:flex items-center justify-between">
                                                <div className="flex items-center space-x-3">
                                                    <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-r from-purple-400 to-indigo-500 font-medium text-white">
                                                        {(group.name?.charAt(0) || "G").toUpperCase()}
                                                    </div>

                                                    <div className="flex flex-col">
                                                        <span className="font-medium text-gray-900">
                                                            Gruppe: {group.name}
                                                        </span>
                                                        <span className="text-sm text-gray-500">
                                                            {substitution.groupName ?? group.name}
                                                        </span>
                                                    </div>
                                                </div>

                                                <div className="flex flex-col">
                                                    <span className="font-medium text-gray-900">
                                                        Vertretung: {substitution.substituteStaffName}
                                                    </span>
                                                    <button
                                                        onClick={async () => {
                                                            try {
                                                                setIsLoading(true);
                                                                await substitutionService.deleteSubstitution(substitution.id);
                                                                await Promise.all([
                                                                    fetchTeachers(),
                                                                    fetchActiveSubstitutions()
                                                                ]);
                                                            } catch (err) {
                                                                console.error("Error ending substitution:", err);
                                                                setError("Fehler beim Beenden der Vertretung.");
                                                            } finally {
                                                                setIsLoading(false);
                                                            }
                                                        }}
                                                        className="mt-2 rounded-lg bg-red-100 px-4 py-1 text-sm font-medium text-red-600 hover:bg-red-200 transition-colors"
                                                        disabled={isLoading}
                                                    >
                                                        Vertretung beenden
                                                    </button>
                                                </div>
                                            </div>
                                        </div>
                                    );
                                })
                            ) : (
                                <div className="py-8 text-center">
                                    <div className="flex flex-col items-center gap-4">
                                        <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                                        </svg>
                                        <div>
                                            <h3 className="text-lg font-medium text-gray-900">Keine aktiven Vertretungen</h3>
                                            <p className="text-gray-600">Aktuell sind keine Vertretungen zugewiesen.</p>
                                        </div>
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            </div>

            {/* Substitution Assignment Popup */}
            {showPopup && selectedTeacher && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 backdrop-blur-sm p-4">
                    <div className="w-full max-w-md rounded-xl bg-white border border-gray-200 shadow-2xl">
                        <div className="p-4 md:p-6">
                            <h2 className="mb-4 text-lg md:text-xl font-bold text-gray-800">Vertretung zuweisen</h2>

                            <div className="mb-6">
                                <p className="mb-2 text-sm font-medium text-gray-700">Lehrkraft:</p>
                                <p className="font-medium text-gray-900">{formatTeacherName(selectedTeacher)}</p>
                            </div>

                            {/* Group selection */}
                            <div className="relative mb-4">
                                <label className="block text-sm font-medium text-gray-700 mb-1">
                                    OGS-Gruppe auswählen
                                </label>
                                <select
                                    value={selectedGroup}
                                    onChange={(e) => setSelectedGroup(e.target.value)}
                                    className="mt-1 block w-full rounded-lg border-0 px-4 py-3 h-12 text-base shadow-sm ring-1 ring-gray-200 transition-all duration-200 hover:bg-gray-50/50 hover:ring-gray-300 focus:ring-2 focus:ring-teal-500 focus:outline-none appearance-none pr-8"
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
                                    className="mt-1 block w-full rounded-lg border-0 px-4 py-3 h-12 text-base shadow-sm ring-1 ring-gray-200 focus:ring-2 focus:ring-teal-500 focus:outline-none"
                                />
                            </div>

                            {/* Actions */}
                            <div className="flex flex-col md:flex-row gap-3 md:justify-end md:space-x-3">
                                <button
                                    onClick={closePopup}
                                    className="order-2 md:order-1 rounded-lg border border-gray-300 bg-white px-4 py-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50"
                                >
                                    Abbrechen
                                </button>
                                <button
                                    onClick={handleAssignSubstitution}
                                    className="order-1 md:order-2 rounded-lg bg-gradient-to-r from-teal-500 to-blue-600 px-6 py-3 text-sm font-medium text-white shadow-sm transition-all hover:from-teal-600 hover:to-blue-700 hover:shadow-md active:scale-[0.98]"
                                >
                                    Zuweisen
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </ResponsiveLayout>
    );
}