"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import { teacherService, type Teacher } from "@/lib/teacher-api";
import { DatabaseListPage } from "@/components/ui";
import { TeacherListItem } from "@/components/teachers";

export default function TeachersPage() {
    const router = useRouter();
    const [teachers, setTeachers] = useState<Teacher[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [searchFilter, setSearchFilter] = useState("");

    const { data: session, status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/");
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
                
                // Ensure we got an array back
                if (!Array.isArray(data)) {
                    console.error("API did not return an array:", data);
                    setTeachers([]);
                    setError("Unerwartetes Datenformat vom Server.");
                    return;
                }

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

    if (status === "loading") {
        return <div />; // Let DatabaseListPage handle the loading state
    }

    const handleSelectTeacher = (teacher: Teacher) => {
        router.push(`/database/teachers/${teacher.id}`);
    };

    return (
        <DatabaseListPage
            userName={session?.user?.name ?? "Root"}
            title="Lehrer auswählen"
            description="Verwalten Sie Lehrerprofile und Zuordnungen"
            listTitle="Lehrerliste"
            searchPlaceholder="Lehrer suchen..."
            searchValue={searchFilter}
            onSearchChange={setSearchFilter}
            addButton={{
                label: "Neuen Lehrer erstellen",
                href: "/database/teachers/new"
            }}
            items={teachers}
            loading={loading}
            error={error}
            onRetry={() => fetchTeachers()}
            itemLabel={{ singular: "Lehrer", plural: "Lehrer" }}
            emptyIcon={
                <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                </svg>
            }
            renderItem={(teacher: Teacher) => (
                <TeacherListItem
                    key={teacher.id}
                    teacher={teacher}
                    onClick={handleSelectTeacher}
                />
            )}
        />
    );
}