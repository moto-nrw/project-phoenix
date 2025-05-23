"use client";

import { useState, useEffect } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Input } from "~/components/ui";
import { Alert } from "~/components/ui/alert";

// Define types based on existing Student type
interface Student {
    id: string;
    first_name: string;
    second_name: string;
    name?: string;
    school_class: string;
    group_id: string;
    group_name?: string;
    in_house: boolean;
    wc: boolean;
    school_yard: boolean;
    bus: boolean;
}

interface OGSGroup {
    id: string;
    name: string;
    room_name?: string;
    room_id?: string;
    student_count?: number;
    supervisor_name?: string;
    students?: Student[];
}

export default function OGSGroupPage() {
    const { data: session, status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/");
        },
    });

    // State variables
    const [ogsGroup, setOGSGroup] = useState<OGSGroup | null>(null);
    const [students, setStudents] = useState<Student[]>([]);
    const [searchTerm, setSearchTerm] = useState("");
    const [selectedYear, setSelectedYear] = useState<string>("all");
    const [attendanceFilter, setAttendanceFilter] = useState<string>("all");
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    // Statistics
    const [stats, setStats] = useState({
        totalStudents: 0,
        presentStudents: 0,
        absentStudents: 0,
        schoolyard: 0,
        bathroom: 0,
        bus: 0,
    });

    // Mock data for OGS group - replace with actual API call in production
    useEffect(() => {
        const fetchOGSGroupData = async () => {
            try {
                setIsLoading(true);

                // Simulate API fetch delay
                await new Promise(resolve => setTimeout(resolve, 800));

                // Mock OGS Group data - this would be replaced with actual API call
                const mockOgsGroup: OGSGroup = {
                    id: "g1",
                    name: "Sonnenschein",
                    room_name: "Raum 103",
                    room_id: "r103",
                    student_count: 24,
                    supervisor_name: "Frau Meyer"
                };

                // Mock Students data
                const mockStudents: Student[] = [
                    {
                        id: "1",
                        first_name: "Emma",
                        second_name: "Müller",
                        name: "Emma Müller",
                        school_class: "1a",
                        group_id: "g1",
                        group_name: "Sonnenschein",
                        in_house: true,
                        wc: false,
                        school_yard: false,
                        bus: false
                    },
                    {
                        id: "2",
                        first_name: "Max",
                        second_name: "Schmidt",
                        name: "Max Schmidt",
                        school_class: "1b",
                        group_id: "g1",
                        group_name: "Sonnenschein",
                        in_house: false,
                        wc: true,
                        school_yard: false,
                        bus: false
                    },
                    {
                        id: "3",
                        first_name: "Sophie",
                        second_name: "Wagner",
                        name: "Sophie Wagner",
                        school_class: "2a",
                        group_id: "g1",
                        group_name: "Sonnenschein",
                        in_house: true,
                        wc: false,
                        school_yard: false,
                        bus: false
                    },
                    {
                        id: "4",
                        first_name: "Leon",
                        second_name: "Fischer",
                        name: "Leon Fischer",
                        school_class: "2b",
                        group_id: "g1",
                        group_name: "Sonnenschein",
                        in_house: true,
                        wc: false,
                        school_yard: false,
                        bus: false
                    },
                    {
                        id: "5",
                        first_name: "Mia",
                        second_name: "Weber",
                        name: "Mia Weber",
                        school_class: "3a",
                        group_id: "g1",
                        group_name: "Sonnenschein",
                        in_house: true,
                        wc: false,
                        school_yard: false,
                        bus: false
                    },
                    {
                        id: "6",
                        first_name: "Noah",
                        second_name: "Becker",
                        name: "Noah Becker",
                        school_class: "3b",
                        group_id: "g1",
                        group_name: "Sonnenschein",
                        in_house: false,
                        wc: false,
                        school_yard: true,
                        bus: false
                    },
                    {
                        id: "7",
                        first_name: "Lina",
                        second_name: "Schulz",
                        name: "Lina Schulz",
                        school_class: "4a",
                        group_id: "g1",
                        group_name: "Sonnenschein",
                        in_house: false,
                        wc: false,
                        school_yard: false,
                        bus: true
                    },
                    {
                        id: "8",
                        first_name: "Felix",
                        second_name: "Hoffmann",
                        name: "Felix Hoffmann",
                        school_class: "4b",
                        group_id: "g1",
                        group_name: "Sonnenschein",
                        in_house: true,
                        wc: false,
                        school_yard: false,
                        bus: false
                    }
                ];

                setOGSGroup(mockOgsGroup);
                setStudents(mockStudents);

                // Calculate statistics
                const presentCount = mockStudents.filter(s => s.in_house).length;
                const schoolyardCount = mockStudents.filter(s => s.school_yard).length;
                const bathroomCount = mockStudents.filter(s => s.wc).length;
                const busCount = mockStudents.filter(s => s.bus).length;

                setStats({
                    totalStudents: mockStudents.length,
                    presentStudents: presentCount,
                    absentStudents: mockStudents.length - presentCount - schoolyardCount - bathroomCount - busCount,
                    schoolyard: schoolyardCount,
                    bathroom: bathroomCount,
                    bus: busCount
                });

                setError(null);
            } catch (err) {
                console.error("Error fetching OGS group data:", err);
                setError("Fehler beim Laden der OGS-Gruppendaten.");
            } finally {
                setIsLoading(false);
            }
        };

        void fetchOGSGroupData();
    }, []);

    // Apply filters to students
    const filteredStudents = students.filter((student) => {
        // Apply search filter
        if (searchTerm && !student.name?.toLowerCase().includes(searchTerm.toLowerCase()) &&
            !student.school_class?.toLowerCase().includes(searchTerm.toLowerCase())) {
            return false;
        }

        // Apply attendance filter
        if (attendanceFilter === "in_house" && !student.in_house) return false;
        if (attendanceFilter === "wc" && !student.wc) return false;
        if (attendanceFilter === "school_yard" && !student.school_yard) return false;
        if (attendanceFilter === "bus" && !student.bus) return false;

        // Apply year filter
        if (selectedYear !== "all") {
            const yearMatch = /^(\d)/.exec(student.school_class);
            const studentYear = yearMatch ? yearMatch[1] : null;
            if (studentYear !== selectedYear) {
                return false;
            }
        }

        return true;
    });

    // Helper function to determine the school year
    const getSchoolYear = (schoolClass: string): number => {
        const yearMatch = /^(\d)/.exec(schoolClass);
        return yearMatch?.[1] ? parseInt(yearMatch[1], 10) : 0;
    };

    // Determine color for year dot
    const getYearColor = (year: number): string => {
        switch (year) {
            case 1: return "bg-blue-500";
            case 2: return "bg-green-500";
            case 3: return "bg-yellow-500";
            case 4: return "bg-purple-500";
            default: return "bg-gray-400";
        }
    };

    // Common class for all dropdowns to ensure consistent height
    const dropdownClass = "mt-1 block w-full rounded-lg border-0 px-4 py-3 h-12 shadow-sm ring-1 ring-gray-200 transition-all duration-200 hover:bg-gray-50/50 hover:ring-gray-300 focus:ring-2 focus:ring-teal-500 focus:outline-none appearance-none pr-8";

    if (status === "loading" || isLoading) {
        return (
            <div className="flex min-h-screen items-center justify-center">
                <div className="flex flex-col items-center gap-4">
                    <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
                    <p className="text-gray-600">Daten werden geladen...</p>
                </div>
            </div>
        );
    }

    return (
        <ResponsiveLayout userName={session?.user?.name ?? "Root"}>
            <div className="max-w-7xl mx-auto">
                            {/* OGS Group Header with Gradient */}
                            <div className="mb-8 overflow-hidden rounded-xl bg-gradient-to-r from-blue-600 to-teal-500 shadow-lg">
                                <div className="px-8 py-6 text-white">
                                    <div className="flex items-center justify-between">
                                        <div>
                                            <h1 className="text-3xl font-bold">OGS-Gruppe: {ogsGroup?.name}</h1>
                                            <p className="mt-1 text-white/80">
                                                {ogsGroup?.room_name && `Raum: ${ogsGroup.room_name}`}
                                                {ogsGroup?.supervisor_name && ` • Betreuer: ${ogsGroup.supervisor_name}`}
                                            </p>
                                        </div>
                                        <div className="rounded-full bg-white/20 px-4 py-2 text-center backdrop-blur-sm">
                                            <span className="text-xl font-bold">{ogsGroup?.student_count ?? 0}</span>
                                            <p className="text-xs font-medium">Schüler</p>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            {/* Stats Overview Cards */}
                            <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-3 lg:grid-cols-5">
                                {/* Present Students Card */}
                                <div className="overflow-hidden rounded-lg bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md">
                                    <div className="flex items-center">
                                        <div className="mr-4 rounded-full bg-green-100 p-3">
                                            <svg className="h-6 w-6 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                                            </svg>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-gray-600">Im Gruppenraum</p>
                                            <p className="text-2xl font-bold text-gray-900">{stats.presentStudents}</p>
                                        </div>
                                    </div>
                                </div>

                                {/* Schoolyard Card - Updated with playground icon */}
                                <div className="overflow-hidden rounded-lg bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md">
                                    <div className="flex items-center">
                                        <div className="mr-4 rounded-full bg-blue-100 p-3">
                                            <svg className="h-6 w-6 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 5l-4-4-4 4M8 9v10M16 19V9M12 1v18M3 19h18" />
                                            </svg>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-gray-600">Schulhof</p>
                                            <p className="text-2xl font-bold text-gray-900">{stats.schoolyard}</p>
                                        </div>
                                    </div>
                                </div>

                                {/* Bathroom Card - Updated with toilet icon */}
                                <div className="overflow-hidden rounded-lg bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md">
                                    <div className="flex items-center">
                                        <div className="mr-4 rounded-full bg-yellow-100 p-3">
                                            <span className="flex h-6 w-6 items-center justify-center font-bold text-yellow-600">
                                                WC
                                            </span>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-gray-600">Toilette</p>
                                            <p className="text-2xl font-bold text-gray-900">{stats.bathroom}</p>
                                        </div>
                                    </div>
                                </div>

                                {/* Bus Card - Updated with arrow/exit icon */}
                                <div className="overflow-hidden rounded-lg bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md">
                                    <div className="flex items-center">
                                        <div className="mr-4 rounded-full bg-purple-100 p-3">
                                            <svg className="h-6 w-6 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 5l7 7m0 0l-7 7m7-7H3" />
                                            </svg>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-gray-600">Unterwegs</p>
                                            <p className="text-2xl font-bold text-gray-900">{stats.bus}</p>
                                        </div>
                                    </div>
                                </div>

                                {/* Home Card */}
                                <div className="overflow-hidden rounded-lg bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md">
                                    <div className="flex items-center">
                                        <div className="mr-4 rounded-full bg-red-100 p-3">
                                            <svg className="h-6 w-6 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
                                            </svg>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-gray-600">Zuhause</p>
                                            <p className="text-2xl font-bold text-gray-900">{stats.absentStudents}</p>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            {/* Search Panel */}
                            <div className="mb-8 overflow-hidden rounded-xl bg-white p-6 shadow-md">
                                <h2 className="mb-4 text-xl font-bold text-gray-800">Suchkriterien</h2>

                                <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
                                    {/* Name Search */}
                                    <Input
                                        label="Name"
                                        name="searchTerm"
                                        placeholder="Vor- oder Nachname"
                                        value={searchTerm}
                                        onChange={(e) => setSearchTerm(e.target.value)}
                                        className="h-12" // Add fixed height to the Input component
                                    />

                                    {/* School Year Filter */}
                                    <div className="relative">
                                        <label className="block text-sm font-medium text-gray-700">
                                            Jahrgangsstufe
                                        </label>
                                        <select
                                            value={selectedYear}
                                            onChange={(e) => setSelectedYear(e.target.value)}
                                            className={dropdownClass}
                                        >
                                            <option value="all">Alle Jahrgänge</option>
                                            <option value="1">Jahrgang 1</option>
                                            <option value="2">Jahrgang 2</option>
                                            <option value="3">Jahrgang 3</option>
                                            <option value="4">Jahrgang 4</option>
                                        </select>
                                        <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                                            <svg className="h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                                                <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                            </svg>
                                        </div>
                                    </div>

                                    {/* Attendance Status */}
                                    <div className="relative">
                                        <label className="block text-sm font-medium text-gray-700">
                                            Anwesenheitsstatus
                                        </label>
                                        <select
                                            value={attendanceFilter}
                                            onChange={(e) => setAttendanceFilter(e.target.value)}
                                            className={dropdownClass}
                                        >
                                            <option value="all">Alle</option>
                                            <option value="in_house">Im Gruppenraum</option>
                                            <option value="wc">Toilette</option>
                                            <option value="school_yard">Schulhof</option>
                                            <option value="bus">Unterwegs</option>
                                        </select>
                                        <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                                            <svg className="h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                                                <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                            </svg>
                                        </div>
                                    </div>
                                </div>

                                {/* Search Actions */}
                                <div className="mt-6 flex flex-wrap justify-end gap-3">
                                    <button
                                        onClick={() => {
                                            setSearchTerm("");
                                            setSelectedYear("all");
                                            setAttendanceFilter("all");
                                        }}
                                        className="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50"
                                    >
                                        Zurücksetzen
                                    </button>
                                </div>
                            </div>

                            {/* Results Section */}
                            <div className="overflow-hidden rounded-xl bg-white p-6 shadow-md">
                                <div className="flex justify-between items-center mb-6">
                                    <h2 className="text-xl font-bold text-gray-800">Schüler in dieser Gruppe</h2>
                                    <div className="flex items-center space-x-6">
                                        {/* Year legend */}
                                        <div className="flex items-center space-x-4">
                                            <div className="flex items-center">
                                                <span className="inline-block h-3 w-3 rounded-full bg-blue-500 mr-1"></span>
                                                <span className="text-xs text-gray-600">Jahr 1</span>
                                            </div>
                                            <div className="flex items-center">
                                                <span className="inline-block h-3 w-3 rounded-full bg-green-500 mr-1"></span>
                                                <span className="text-xs text-gray-600">Jahr 2</span>
                                            </div>
                                            <div className="flex items-center">
                                                <span className="inline-block h-3 w-3 rounded-full bg-yellow-500 mr-1"></span>
                                                <span className="text-xs text-gray-600">Jahr 3</span>
                                            </div>
                                            <div className="flex items-center">
                                                <span className="inline-block h-3 w-3 rounded-full bg-purple-500 mr-1"></span>
                                                <span className="text-xs text-gray-600">Jahr 4</span>
                                            </div>
                                        </div>
                                    </div>
                                </div>

                                {error && (
                                    <div className="mb-6">
                                        <Alert type="error" message={error} />
                                    </div>
                                )}

                                <div className="space-y-2">
                                    {filteredStudents.length > 0 ? (
                                        filteredStudents.map((student) => {
                                            const year = getSchoolYear(student.school_class);
                                            const yearColor = getYearColor(year);

                                            return (
                                                <div
                                                    key={student.id}
                                                    onClick={() => {/* Navigate to student detail */}}
                                                    className="group cursor-pointer rounded-lg border border-gray-100 bg-white p-4 shadow-sm transition-all duration-200 hover:translate-y-[-1px] hover:border-blue-200 hover:shadow-md"
                                                >
                                                    <div className="flex items-center justify-between">
                                                        <div className="flex items-center space-x-3">
                                                            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-r from-blue-400 to-indigo-500 font-medium text-white">
                                                                {student.first_name?.charAt(0).toUpperCase() || "S"}
                                                            </div>

                                                            <div className="flex flex-col">
                                                                <div className="flex items-center">
                                                                    <span className="font-medium text-gray-900 transition-colors group-hover:text-blue-600">
                                                                      {student.first_name} {student.second_name}
                                                                    </span>
                                                                    {/* Year indicator */}
                                                                    <span className={`ml-2 inline-block h-3 w-3 rounded-full ${yearColor}`} title={`Jahrgang ${year}`}></span>
                                                                </div>
                                                                <span className="text-sm text-gray-500">
                                                                    Klasse: {student.school_class}
                                                                </span>
                                                            </div>
                                                        </div>

                                                        <div className="flex items-center space-x-4">
                                                            {/* Status indicators - UPDATED TO MATCH CARD COLORS */}
                                                            <div className="flex space-x-2">
                                                                {student.in_house && (
                                                                    <div className="flex h-7 items-center rounded-full bg-green-100 px-2 text-xs font-medium text-green-600" title="Im Gruppenraum">
                                                                        <span className="mr-1 h-2 w-2 rounded-full bg-green-600"></span>
                                                                        <span>Im Gruppenraum</span>
                                                                    </div>
                                                                )}
                                                                {student.wc && (
                                                                    <div className="flex h-7 items-center rounded-full bg-yellow-100 px-2 text-xs font-medium text-yellow-600" title="Toilette">
                                                                        <span className="mr-1 h-2 w-2 rounded-full bg-yellow-600"></span>
                                                                        <span>WC</span>
                                                                    </div>
                                                                )}
                                                                {student.school_yard && (
                                                                    <div className="flex h-7 items-center rounded-full bg-blue-100 px-2 text-xs font-medium text-blue-600" title="Schulhof">
                                                                        <span className="mr-1 h-2 w-2 rounded-full bg-blue-600"></span>
                                                                        <span>Schulhof</span>
                                                                    </div>
                                                                )}
                                                                {student.bus && (
                                                                    <div className="flex h-7 items-center rounded-full bg-purple-100 px-2 text-xs font-medium text-purple-600" title="Unterwegs">
                                                                        <span className="mr-1 h-2 w-2 rounded-full bg-purple-600"></span>
                                                                        <span>Unterwegs</span>
                                                                    </div>
                                                                )}
                                                                {!student.in_house && !student.wc && !student.school_yard && !student.bus && (
                                                                    <div className="flex h-7 items-center rounded-full bg-red-100 px-2 text-xs font-medium text-red-600" title="Nicht anwesend">
                                                                        <span className="mr-1 h-2 w-2 rounded-full bg-red-600"></span>
                                                                        <span>Abwesend</span>
                                                                    </div>
                                                                )}
                                                            </div>

                                                            <svg
                                                                xmlns="http://www.w3.org/2000/svg"
                                                                className="h-5 w-5 text-gray-400 transition-all duration-200 group-hover:translate-x-1 group-hover:text-blue-500"
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
                                                        </div>
                                                    </div>
                                                </div>
                                            );
                                        })
                                    ) : (
                                        <div className="py-8 text-center">
                                            <p className="text-gray-500">
                                                {searchTerm || selectedYear !== "all" || attendanceFilter !== "all"
                                                    ? "Keine Ergebnisse gefunden. Bitte passen Sie Ihre Suchkriterien an."
                                                    : "Keine Schüler in dieser Gruppe gefunden."}
                                            </p>
                                        </div>
                                    )}
                                </div>
                            </div>
            </div>
        </ResponsiveLayout>
    );
}