"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import { PageHeader, SectionTitle } from "@/components/dashboard";
import TeacherForm from "@/components/teachers/teacher-form";
import { teacherService, type Teacher } from "@/lib/teacher-api";

export default function NewTeacherPage() {
    const router = useRouter();
    const [, setLoading] = useState(false);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [rfidCards, setRfidCards] = useState<Array<{ id: string; label: string }>>([]);
    const [createdCredentials, setCreatedCredentials] = useState<{ email: string; password: string } | null>(null);

    const { status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/");
        },
    });

    // Function to fetch RFID cards
    const fetchRfidCards = async () => {
        try {
            setLoading(true);

            // Fetch available RFID cards from the Next.js API route
            const response = await fetch("/api/users/rfid-cards/available");
            if (response.ok) {
                const responseData = await response.json() as { data?: Array<{ TagID: string }> } | Array<{ TagID: string }>;
                
                // Handle wrapped response from route handler
                let cards: Array<{ TagID: string }>;
                
                if (responseData && typeof responseData === 'object' && 'data' in responseData && responseData.data) {
                    // Response is wrapped (from route handler)
                    cards = responseData.data;
                } else if (Array.isArray(responseData)) {
                    // Direct array response
                    cards = responseData;
                } else {
                    console.error("Unexpected RFID cards response format:", responseData);
                    setRfidCards([]);
                    return;
                }
                
                // Transform the backend response to match frontend expectations
                const transformedCards = cards.map((card) => ({
                    id: card.TagID,
                    label: `RFID: ${card.TagID}` // Create a display label
                }));
                
                setRfidCards(transformedCards);
            } else {
                console.error("Failed to fetch RFID cards:", response.status);
                // Set empty array if fetch fails to avoid UI issues
                setRfidCards([]);
            }
        } catch (err) {
            console.error("Error fetching RFID cards:", err);
            // Set empty array to avoid UI blocking
            setRfidCards([]);
        } finally {
            setLoading(false);
        }
    };

    // Handle form submission
    const handleSubmit = async (formData: Partial<Teacher> & { password?: string }) => {
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
            const teacherData: Omit<Teacher, "id" | "name" | "created_at" | "updated_at"> & { password?: string } = {
                first_name: formData.first_name,
                last_name: formData.last_name,
                email: formData.email,
                specialization: formData.specialization,
                role: formData.role ?? null,
                qualifications: formData.qualifications ?? null,
                tag_id: formData.tag_id ?? null,
                staff_notes: formData.staff_notes ?? null,
                password: formData.password  // Include password from form data
            };

            // Create the teacher using the teacher service
            const newTeacher = await teacherService.createTeacher(teacherData);

            // If we got temporary credentials, show them to the user
            if (newTeacher.temporaryCredentials) {
                setCreatedCredentials(newTeacher.temporaryCredentials);
                // Don't redirect yet, show the credentials first
            } else {
                // Redirect to the new teacher if no credentials to show
                router.push(`/database/teachers/${newTeacher.id}`);
            }
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

                {createdCredentials && (
                    <div className="mb-6 rounded-lg border border-green-200 bg-green-50 p-6">
                        <h3 className="mb-4 text-lg font-semibold text-green-800">
                            Lehrer erfolgreich erstellt!
                        </h3>
                        <p className="mb-4 text-green-700">
                            Bitte notieren Sie sich die folgenden temporären Zugangsdaten:
                        </p>
                        <div className="mb-4 rounded bg-white p-4 font-mono text-sm">
                            <p className="mb-2">
                                <span className="font-semibold">E-Mail:</span> {createdCredentials.email}
                            </p>
                            <p>
                                <span className="font-semibold">Passwort:</span> {createdCredentials.password}
                            </p>
                        </div>
                        <p className="mb-4 text-sm text-green-600">
                            Der Lehrer sollte das Passwort bei der ersten Anmeldung ändern.
                        </p>
                        <button
                            onClick={() => router.push(`/database/teachers`)}
                            className="rounded bg-green-600 px-4 py-2 text-white hover:bg-green-700"
                        >
                            Zur Lehrerübersicht
                        </button>
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