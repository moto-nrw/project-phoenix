"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import { PageHeader, SectionTitle } from "@/components/dashboard";
import { teacherService, type Teacher } from "@/lib/teacher-api";
import Link from "next/link";

export default function TeachersPage() {
    const router = useRouter();
    const [teachers, setTeachers] = useState<Teacher[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [searchFilter, setSearchFilter] = useState("");

    const { status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/login");
        },
    });

    // Function to fetch teachers with optional filters
    const fetchTeachers = async (search?: string) => {
        try {
            setLoading(true);

            // Prepare filters for API call
            const filters = {
                search: search ?? undefined,
            };

            try {
                // Fetch from the real API using our teacher service
                const data = await teacherService.getTeachers(filters);

                if (data.length === 0 && !search) {
                    console.log("No teachers returned from API, checking connection");
                }

                setTeachers(data);
                setError(null);
            } catch (apiErr) {
                console.error("API error when fetching teachers:", apiErr);
                setError(
                    "Fehler beim Laden der Lehrerdaten. Bitte versuchen Sie es später erneut.",
                );
                setTeachers([]);
            }
        } catch (err) {
            console.error("Error fetching teachers:", err);
            setError(
                "Fehler beim Laden der Lehrerdaten. Bitte versuchen Sie es später erneut.",
            );
            setTeachers([]);
        } finally {
            setLoading(false);
        }
    };

    // Initial data load
    useEffect(() => {
        void fetchTeachers();
    }, []);

    // Handle search filter changes
    useEffect(() => {
        // Debounce search to avoid too many API calls
        const timer = setTimeout(() => {
            void fetchTeachers(searchFilter);
        }, 300);

        return () => clearTimeout(timer);
    }, [searchFilter]);

    if (status === "loading" || loading) {
        return (
            <div className="flex min-h-screen items-center justify-center">
                <p>Loading...</p>
            </div>
        );
    }

    const handleSelectTeacher = (teacher: Teacher) => {
        router.push(`/database/teachers/${teacher.id}`);
    };

    // Show error if loading failed
    if (error) {
        return (
            <div className="flex min-h-screen flex-col items-center justify-center p-4">
                <div className="max-w-md rounded-lg bg-red-50 p-4 text-red-800">
                    <h2 className="mb-2 font-semibold">Fehler</h2>
                    <p>{error}</p>
                    <button
                        onClick={() => fetchTeachers()}
                        className="mt-4 rounded bg-red-100 px-4 py-2 text-red-800 transition-colors hover:bg-red-200"
                    >
                        Erneut versuchen
                    </button>
                </div>
            </div>
        );
    }

    // Filter teachers based on search
    const filteredTeachers = teachers.filter(
        (teacher) =>
            teacher.name.toLowerCase().includes(searchFilter.toLowerCase()) ||
            teacher.specialization.toLowerCase().includes(searchFilter.toLowerCase()) ||
            teacher.role?.toLowerCase().includes(searchFilter.toLowerCase())
    );

    return (
        <div className="min-h-screen">
            {/* Header */}
            <PageHeader title="Lehrerauswahl" backUrl="/database" />

            {/* Main Content */}
            <main className="mx-auto max-w-4xl p-4">
                {/* Title Section */}
                <div className="mb-8">
                    <SectionTitle title="Lehrer auswählen" />
                </div>

                {/* Search and Add Section */}
                <div className="mb-8 flex flex-col items-center justify-between gap-4 sm:flex-row">
                    <div className="relative w-full sm:max-w-md">
                        <input
                            type="text"
                            placeholder="Suchen..."
                            value={searchFilter}
                            onChange={(e) => setSearchFilter(e.target.value)}
                            className="w-full rounded-lg border border-gray-300 px-4 py-3 pl-10 transition-all duration-200 hover:border-gray-400 focus:shadow-md focus:ring-2 focus:ring-blue-500 focus:outline-none"
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

                    <Link href="/database/teachers/new" className="w-full sm:w-auto">
                        <button className="group flex w-full items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-teal-500 to-blue-600 px-4 py-3 text-white transition-all duration-200 hover:scale-[1.02] hover:from-teal-600 hover:to-blue-700 hover:shadow-lg sm:w-auto sm:justify-start">
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                            >
                                <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    strokeWidth={2}
                                    d="M12 4v16m8-8H4"
                                />
                            </svg>
                            <span>Neuen Lehrer erstellen</span>
                        </button>
                    </Link>
                </div>

                {/* Teacher List */}
                <div className="rounded-lg border border-gray-100 bg-white p-6 shadow-sm">
                    {filteredTeachers.length === 0 ? (
                        <div className="flex flex-col items-center justify-center py-12">
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                className="mb-4 h-16 w-16 text-gray-300"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                            >
                                <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    strokeWidth={1.5}
                                    d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
                                />
                            </svg>
                            <p className="mb-2 text-lg font-medium text-gray-600">
                                {searchFilter
                                    ? `Keine Ergebnisse für "${searchFilter}"`
                                    : "Keine Lehrer vorhanden."}
                            </p>
                            <p className="text-sm text-gray-500">
                                {searchFilter
                                    ? "Versuchen Sie einen anderen Suchbegriff."
                                    : "Fügen Sie einen neuen Lehrer hinzu, um zu beginnen."}
                            </p>
                        </div>
                    ) : (
                        <div className="space-y-4">
                            {filteredTeachers.map((teacher) => (
                                <div
                                    key={teacher.id}
                                    onClick={() => handleSelectTeacher(teacher)}
                                    className="group cursor-pointer rounded-lg border border-gray-100 p-4 transition-all hover:border-blue-200 hover:bg-blue-50 hover:shadow-sm"
                                >
                                    <div className="flex items-center justify-between">
                                        <div className="flex items-center space-x-4">
                                            {/* Avatar placeholder */}
                                            <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-full bg-gradient-to-r from-blue-400 to-indigo-500 font-medium text-white">
                                                {teacher.name.charAt(0).toUpperCase()}
                                            </div>

                                            {/* Teacher info */}
                                            <div>
                                                <h3 className="font-medium text-gray-900 group-hover:text-blue-700">
                                                    {teacher.name}
                                                </h3>
                                                <div className="mt-1 flex flex-wrap gap-2">
                                                    {teacher.specialization && (
                                                        <span className="inline-flex items-center rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-800">
                              {teacher.specialization}
                            </span>
                                                    )}
                                                    {teacher.role && (
                                                        <span className="inline-flex items-center rounded bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800">
                              {teacher.role}
                            </span>
                                                    )}
                                                </div>
                                            </div>
                                        </div>

                                        <svg
                                            xmlns="http://www.w3.org/2000/svg"
                                            className="h-5 w-5 text-gray-400 transition-all duration-200 group-hover:translate-x-1 group-hover:transform group-hover:text-blue-500"
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
                            ))}
                        </div>
                    )}
                </div>
            </main>
        </div>
    );
}