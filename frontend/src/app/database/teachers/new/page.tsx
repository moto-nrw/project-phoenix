"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import { PageHeader, SectionTitle } from "@/components/dashboard";
import TeacherForm from "@/components/teachers/teacher-form";
import { teacherService, type Teacher } from "@/lib/teacher-api";

export default function NewTeacherPage() {
    const router = useRouter();
    const [loading, setLoading] = useState(false);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [rfidCards, setRfidCards] = useState<Array<{ id: string; label: string }>>([]);

    const { status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/login");
        },
    });

    // Function to fetch RFID cards
    const fetchRfidCards = async () => {
        try {
            setLoading(true);

            // Fetch available RFID cards
            const response = await fetch("/api/rfid-cards");
            if (response.ok) {
                const cardsData = await response.json();
                setRfidCards(cardsData);
            } else {
                console.error("Failed to fetch RFID cards");
            }
        } catch (err) {
            console.error("Error fetching RFID cards:", err);
            // Don't set an error that would block the UI
        } finally {
            setLoading(false);
        }
    };

    // Handle form submission
    const handleSubmit = async (formData: Partial<Teacher>) => {
        try {
            setSaving(true);

            // Ensure all required fields are set
            if (
                !formData.first_name ||
                !formData.last_name ||
                !formData.specialization
            ) {
                setError("Bitte füllen Sie alle Pflichtfelder aus.");
                return;
            }

            // Create a complete teacher object with required fields
            const teacherData: Omit<Teacher, "id" | "name" | "created_at" | "updated_at"> = {
                first_name: formData.first_name,
                last_name: formData.last_name,
                specialization: formData.specialization,
                role: formData.role || null,
                qualifications: formData.qualifications || null,
                tag_id: formData.tag_id || null,
                staff_notes: formData.staff_notes || null
            };

            // Create the teacher
            const newTeacher = await teacherService.createTeacher(teacherData);

            // Redirect to the new teacher
            router.push(`/database/teachers/${newTeacher.id}`);
        } catch (err) {
            setError(
                "Fehler beim Erstellen des Lehrers. Bitte versuchen Sie es später erneut."
            );
            throw err; // Rethrow so the form can handle it
        } finally {
            setSaving(false);
        }
    };

    // Initial data load
    useEffect(() => {
        void fetchRfidCards();
    }, []);

    if (status === "loading") {
        return (
            <div className="flex min-h-screen items-center justify-center">
                <p>Loading...</p>
            </div>
        );
    }

    return (
        <div className="min-h-screen">
            <PageHeader
                title="Neuen Lehrer erstellen"
                backUrl="/database/teachers"
            />

            <main className="mx-auto max-w-4xl p-4">
                <div className="mb-8">
                    <SectionTitle title="Lehrerdetails" />
                </div>

                {error && (
                    <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4 text-red-800">
                        {error}
                    </div>
                )}

                <TeacherForm
                    initialData={{
                        first_name: "",
                        last_name: "",
                        specialization: "",
                    }}
                    onSubmitAction={handleSubmit}
                    onCancelAction={() => router.push("/database/teachers")}
                    isLoading={saving}
                    formTitle="Neuen Lehrer erstellen"
                    submitLabel="Lehrer erstellen"
                    rfidCards={rfidCards}
                />

                <div className="mt-6 rounded-lg bg-gray-50 p-4 text-sm text-gray-500">
                    <p>
                        Hinweis: Nach dem Erstellen können Sie zusätzliche Details und Aktivitäten hinzufügen.
                    </p>
                </div>
            </main>
        </div>
    );
}