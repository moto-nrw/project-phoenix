"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter, useParams } from "next/navigation";
import { useState, useEffect, useCallback } from "react";
import { PageHeader, SectionTitle } from "@/components/dashboard";
import TeacherForm from "@/components/teachers/teacher-form";
import { teacherService, type Teacher } from "@/lib/teacher-api";
import Link from "next/link";

export default function EditTeacherPage() {
    const router = useRouter();
    const params = useParams();
    const { id } = params;
    const [teacher, setTeacher] = useState<Teacher | null>(null);
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [rfidCards, setRfidCards] = useState<Array<{ id: string; label: string }>>([]);

    const { status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/login");
        },
    });

    // Function to fetch the teacher details
    const fetchData = useCallback(async () => {
        if (!id) return;

        try {
            setLoading(true);

            try {
                // Fetch teacher from API
                const teacherData = await teacherService.getTeacher(id as string);
                setTeacher(teacherData);

                // Fetch available RFID cards
                const response = await fetch("/api/rfid-cards");
                if (!response.ok) {
                    throw new Error("Failed to fetch RFID cards");
                }
                const cardsData = await response.json();
                setRfidCards(cardsData);

                setError(null);
            } catch {
                setError(
                    "Fehler beim Laden der Daten. Bitte versuchen Sie es später erneut.",
                );
                setTeacher(null);
            }
        } catch {
            setError(
                "Fehler beim Laden der Daten. Bitte versuchen Sie es später erneut.",
            );
            setTeacher(null);
        } finally {
            setLoading(false);
        }
    }, [id]);

    // Handle form submission
    const handleSubmit = async (formData: Partial<Teacher>) => {
        if (!id || !teacher) return;

        try {
            setSaving(true);

            // Ensure that all required data is included
            const dataToSubmit: Partial<Teacher> = {
                ...formData,
            };

            // Update the teacher
            await teacherService.updateTeacher(id as string, dataToSubmit);

            // Redirect back to teacher details
            router.push(`/database/teachers/${id as string}`);
        } catch (err) {
            setError(
                "Fehler beim Speichern des Lehrers. Bitte versuchen Sie es später erneut.",
            );
            throw err; // Rethrow so the form can handle it
        } finally {
            setSaving(false);
        }
    };

    // Initial data load
    useEffect(() => {
        void fetchData();
    }, [id, fetchData]);

    if (status === "loading" || loading) {
        return (
            <div className="flex min-h-screen items-center justify-center">
                <p>Loading...</p>
            </div>
        );
    }

    // Show error if loading failed
    if (error) {
        return (
            <div className="flex min-h-screen flex-col items-center justify-center p-4">
                <div className="max-w-md rounded-lg bg-red-50 p-4 text-red-800">
                    <h2 className="mb-2 font-semibold">Fehler</h2>
                    <p>{error}</p>
                    <button
                        onClick={() => fetchData()}
                        className="mt-4 rounded bg-red-100 px-4 py-2 text-red-800 transition-colors hover:bg-red-200"
                    >
                        Erneut versuchen
                    </button>
                </div>
            </div>
        );
    }

    if (!teacher) {
        return (
            <div className="flex min-h-screen flex-col items-center justify-center p-4">
                <div className="max-w-md rounded-lg bg-orange-50 p-4 text-orange-800">
                    <h2 className="mb-2 font-semibold">Lehrer nicht gefunden</h2>
                    <p>Der angeforderte Lehrer konnte nicht gefunden werden.</p>
                    <Link href="/database/teachers">
                        <button className="mt-4 rounded bg-orange-100 px-4 py-2 text-orange-800 transition-colors hover:bg-orange-200">
                            Zurück zur Übersicht
                        </button>
                    </Link>
                </div>
            </div>
        );
    }

    return (
        <div className="min-h-screen">
            <PageHeader
                title={`Lehrer bearbeiten: ${teacher.name}`}
                backUrl={`/database/teachers/${teacher.id}`}
            />

            <main className="mx-auto max-w-4xl p-4">
                <div className="mb-8">
                    <SectionTitle title="Lehrerdetails bearbeiten" />
                </div>

                <TeacherForm
                    initialData={teacher}
                    onSubmitAction={handleSubmit}
                    onCancelAction={() =>
                        router.push(`/database/teachers/${teacher.id}`)
                    }
                    isLoading={saving}
                    formTitle="Lehrer bearbeiten"
                    submitLabel="Änderungen speichern"
                    rfidCards={rfidCards}
                />
            </main>
        </div>
    );
}