"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter, useParams } from "next/navigation";
import { useState, useEffect, useCallback } from "react";
import { PageHeader, SectionTitle } from "@/components/dashboard";
import { teacherService, type Teacher } from "@/lib/teacher-api";
import type { Activity } from "@/lib/activity-helpers";
import { DeleteModal } from "@/components/ui";
import Link from "next/link";

export default function TeacherDetailsPage() {
    const router = useRouter();
    const params = useParams();
    const { id } = params;
    const [teacher, setTeacher] = useState<Teacher | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
    const [isDeleting, setIsDeleting] = useState(false);

    const { status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/login");
        },
    });

    // Function to fetch the teacher details
    const fetchTeacher = useCallback(async () => {
        if (!id) return;

        try {
            setLoading(true);

            try {
                // Fetch teacher from API
                const data = await teacherService.getTeacher(id as string);
                setTeacher(data);
                setError(null);
            } catch (apiErr) {
                console.error("API error when fetching teacher:", apiErr);
                setError(
                    "Fehler beim Laden der Lehrerdaten. Bitte versuchen Sie es später erneut.",
                );
                setTeacher(null);
            }
        } catch (err) {
            console.error("Error fetching teacher:", err);
            setError(
                "Fehler beim Laden der Lehrerdaten. Bitte versuchen Sie es später erneut.",
            );
            setTeacher(null);
        } finally {
            setLoading(false);
        }
    }, [id]);

    // Function to handle teacher deletion
    const handleDeleteTeacher = async () => {
        if (!id) return;

        try {
            setIsDeleting(true);
            await teacherService.deleteTeacher(id as string);
            router.push("/database/teachers");
        } catch (err) {
            console.error("Error deleting teacher:", err);
            setError(
                "Fehler beim Löschen des Lehrers. Bitte versuchen Sie es später erneut.",
            );
            setShowDeleteConfirm(false);
        } finally {
            setIsDeleting(false);
        }
    };

    // Initial data load
    useEffect(() => {
        void fetchTeacher();
    }, [id, fetchTeacher]);

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
                        onClick={() => fetchTeacher()}
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
            <PageHeader title={teacher.name} backUrl="/database/teachers" />

            <main className="mx-auto max-w-4xl p-4">
                <div className="mb-8">
                    <SectionTitle title="Lehrerdetails" />
                </div>

                <div className="mb-6 rounded-lg border border-gray-100 bg-white p-6 shadow-sm">
                    {/* General Information */}
                    <div className="mb-8 grid grid-cols-1 gap-6 md:grid-cols-2">
                        <div>
                            <h3 className="mb-4 text-lg font-semibold text-gray-800">
                                Persönliche Informationen
                            </h3>
                            <dl className="space-y-2">
                                <div className="flex flex-col">
                                    <dt className="text-sm text-gray-500">Name</dt>
                                    <dd className="font-medium">{teacher.name}</dd>
                                </div>

                                <div className="flex flex-col">
                                    <dt className="text-sm text-gray-500">Vorname</dt>
                                    <dd className="font-medium">{teacher.first_name}</dd>
                                </div>

                                <div className="flex flex-col">
                                    <dt className="text-sm text-gray-500">Nachname</dt>
                                    <dd className="font-medium">{teacher.last_name}</dd>
                                </div>

                                {teacher.tag_id && (
                                    <div className="flex flex-col">
                                        <dt className="text-sm text-gray-500">Tag ID</dt>
                                        <dd className="font-medium">{teacher.tag_id}</dd>
                                    </div>
                                )}
                            </dl>
                        </div>

                        <div>
                            <h3 className="mb-4 text-lg font-semibold text-gray-800">
                                Berufliche Details
                            </h3>
                            <dl className="space-y-2">
                                <div className="flex flex-col">
                                    <dt className="text-sm text-gray-500">Fachgebiet</dt>
                                    <dd className="font-medium">{teacher.specialization}</dd>
                                </div>

                                {teacher.role && (
                                    <div className="flex flex-col">
                                        <dt className="text-sm text-gray-500">Rolle</dt>
                                        <dd className="font-medium">{teacher.role}</dd>
                                    </div>
                                )}

                                {teacher.qualifications && (
                                    <div className="flex flex-col">
                                        <dt className="text-sm text-gray-500">Qualifikationen</dt>
                                        <dd className="font-medium">{teacher.qualifications}</dd>
                                    </div>
                                )}

                                {teacher.staff_notes && (
                                    <div className="flex flex-col">
                                        <dt className="text-sm text-gray-500">Notizen</dt>
                                        <dd className="font-medium">{teacher.staff_notes}</dd>
                                    </div>
                                )}
                            </dl>
                        </div>
                    </div>

                    {/* Date Information */}
                    <div className="mb-6 grid grid-cols-1 gap-4 border-t border-gray-100 pt-6 sm:grid-cols-2">
                        <div className="flex flex-col">
                            <span className="text-sm text-gray-500">Erstellt am</span>
                            <span className="font-medium">
                {teacher.created_at
                    ? new Date(teacher.created_at).toLocaleDateString("de-DE")
                    : "Unbekannt"}
              </span>
                        </div>

                        <div className="flex flex-col">
                            <span className="text-sm text-gray-500">Aktualisiert am</span>
                            <span className="font-medium">
                {teacher.updated_at
                    ? new Date(teacher.updated_at).toLocaleDateString("de-DE")
                    : "Unbekannt"}
              </span>
                        </div>
                    </div>

                    {/* Action Buttons */}
                    <div className="mt-6 flex flex-col gap-3 sm:flex-row">
                        <Link href={`/database/teachers/${teacher.id}/edit`}>
                            <button className="w-full rounded-lg bg-blue-500 px-4 py-2 text-white transition-colors hover:bg-blue-600 sm:w-auto">
                                Lehrer bearbeiten
                            </button>
                        </Link>

                        {teacher.activities && teacher.activities.length > 0 && (
                            <Link href={`/database/teachers/${teacher.id}/activities`}>
                                <button className="w-full rounded-lg bg-green-500 px-4 py-2 text-white transition-colors hover:bg-green-600 sm:w-auto">
                                    Aktivitäten anzeigen ({teacher.activities.length})
                                </button>
                            </Link>
                        )}

                        <button
                            onClick={() => setShowDeleteConfirm(true)}
                            className="w-full rounded-lg bg-red-500 px-4 py-2 text-white transition-colors hover:bg-red-600 sm:w-auto"
                        >
                            Lehrer löschen
                        </button>
                    </div>

                    {/* Delete Confirmation Dialog */}
                    <DeleteModal
                        isOpen={showDeleteConfirm}
                        onClose={() => setShowDeleteConfirm(false)}
                        onDelete={handleDeleteTeacher}
                        title="Lehrer löschen"
                        isDeleting={isDeleting}
                    >
                        <p>
                            Sind Sie sicher, dass Sie den Lehrer &quot;{teacher.name}
                            &quot; löschen möchten? Dies kann nicht rückgängig gemacht werden.
                        </p>
                        {teacher.activities && teacher.activities.length > 0 && (
                            <p className="mt-2 text-sm text-red-600">
                                Achtung: Dieser Lehrer ist für {teacher.activities.length} Aktivitäten verantwortlich. Das Löschen kann Auswirkungen auf diese Aktivitäten haben.
                            </p>
                        )}
                    </DeleteModal>
                </div>

                {/* Activities Section (if applicable) */}
                {teacher.activities && teacher.activities.length > 0 && (
                    <div className="mb-6 rounded-lg border border-gray-100 bg-white p-6 shadow-sm">
                        <h3 className="mb-4 text-lg font-semibold text-gray-800">
                            Geleitete Aktivitäten
                        </h3>
                        <div className="space-y-2">
                            {teacher.activities.map((activity: Activity) => (
                                <div
                                    key={activity.id}
                                    className="group rounded-lg border border-gray-100 p-3 transition-all hover:border-blue-200 hover:bg-blue-50"
                                >
                                    <Link href={`/database/activities/${activity.id}`}>
                                        <div className="flex items-center justify-between">
                                            <div>
                        <span className="font-medium text-gray-900 transition-colors group-hover:text-blue-600">
                          {activity.name}
                        </span>
                                                {activity.category_name && (
                                                    <span className="ml-2 text-sm text-gray-500">
                            Kategorie: {activity.category_name}
                          </span>
                                                )}
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
                                    </Link>
                                </div>
                            ))}
                        </div>
                    </div>
                )}
            </main>
        </div>
    );
}