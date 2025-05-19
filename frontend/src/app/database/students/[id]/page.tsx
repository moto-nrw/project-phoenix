"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { PageHeader } from "@/components/dashboard";
import StudentForm from "@/components/students/student-form";
import type { Student } from "@/lib/api";
import { studentService } from "@/lib/api";

export default function StudentDetailPage() {
  const router = useRouter();
  const params = useParams();
  const studentId = params.id as string;

  const [student, setStudent] = useState<Student | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);

  useEffect(() => {
    const fetchStudent = async () => {
      try {
        setLoading(true);
        const data = await studentService.getStudent(studentId);
        setStudent(data);
        setError(null);
      } catch (err) {
        console.error("Error fetching student:", err);
        setError(
          err instanceof Error ? err.message : "Fehler beim Laden der Schülerdaten. Bitte versuchen Sie es später erneut.",
        );
        setStudent(null);
      } finally {
        setLoading(false);
      }
    };

    if (studentId) {
      void fetchStudent();
    }
  }, [studentId]);

  const handleUpdate = async (formData: Partial<Student>) => {
    try {
      setLoading(true);
      setError(null);

      // Prepare update data with custom_users_id
      const updateData: Partial<Student> = {
        ...formData,
        custom_users_id: formData.custom_users_id ?? student?.custom_users_id,
      };

      // Update student
      const updatedStudent = await studentService.updateStudent(
        studentId,
        updateData,
      );
      
      // After updating, fetch the student again to make sure we have the latest data
      try {
        const refreshedStudent = await studentService.getStudent(studentId);
        setStudent(refreshedStudent);
        setIsEditing(false);
      } catch (refreshErr) {
        console.error("Error refreshing student data:", refreshErr);
        // Fallback to the updated student data if refresh fails
        setStudent(updatedStudent);
        setIsEditing(false);
      }
    } catch (err) {
      console.error("Error updating student:", err);
      setError(
        "Fehler beim Aktualisieren des Schülers. Bitte versuchen Sie es später erneut.",
      );
      throw err; // Re-throw to be caught by the form component
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async () => {
    if (
      window.confirm(
        "Sind Sie sicher, dass Sie diesen Schüler löschen möchten?",
      )
    ) {
      try {
        setLoading(true);
        await studentService.deleteStudent(studentId);
        router.push("/database/students");
      } catch (err) {
        console.error("Error deleting student:", err);
        setError(
          "Fehler beim Löschen des Schülers. Bitte versuchen Sie es später erneut.",
        );
        setLoading(false);
      }
    }
  };

  if (loading && !student) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-50">
        <div className="flex animate-pulse flex-col items-center">
          <div className="h-12 w-12 rounded-full bg-gradient-to-r from-teal-400 to-blue-500"></div>
          <p className="mt-4 text-gray-500">Daten werden geladen...</p>
        </div>
      </div>
    );
  }

  if (error && !student) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 p-4">
        <div className="max-w-md rounded-lg bg-red-50 p-6 text-red-800 shadow-md">
          <h2 className="mb-3 text-lg font-semibold">Fehler</h2>
          <p className="mb-4">{error}</p>
          <button
            onClick={() => router.back()}
            className="rounded-lg bg-red-100 px-4 py-2 text-red-800 shadow-sm transition-colors hover:bg-red-200"
          >
            Zurück
          </button>
        </div>
      </div>
    );
  }

  if (!student) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 p-4">
        <div className="max-w-md rounded-lg bg-yellow-50 p-6 text-yellow-800 shadow-md">
          <h2 className="mb-3 text-lg font-semibold">Schüler nicht gefunden</h2>
          <p className="mb-4">
            Der angeforderte Schüler konnte nicht gefunden werden.
          </p>
          <button
            onClick={() => router.push("/database/students")}
            className="rounded-lg bg-yellow-100 px-4 py-2 text-yellow-800 shadow-sm transition-colors hover:bg-yellow-200"
          >
            Zurück zur Übersicht
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <PageHeader
        title={isEditing ? "Schüler bearbeiten" : "Schülerdetails"}
        backUrl="/database/students"
      />

      {/* Main Content */}
      <main className="mx-auto max-w-4xl p-4">
        {isEditing ? (
          <StudentForm
            initialData={student}
            onSubmitAction={handleUpdate}
            onCancelAction={() => setIsEditing(false)}
            isLoading={loading}
            formTitle="Schüler bearbeiten"
            submitLabel="Speichern"
          />
        ) : (
          <div className="overflow-hidden rounded-lg bg-white shadow-md">
            {/* Student card header with image placeholder */}
            <div className="relative bg-gradient-to-r from-teal-500 to-blue-600 p-6 text-white">
              <div className="flex items-center">
                <div className="mr-5 flex h-20 w-20 items-center justify-center rounded-full bg-white/30 text-3xl font-bold">
                  {student.first_name?.[0] ?? ""}
                  {student.second_name?.[0] ?? ""}
                </div>
                <div>
                  <h1 className="text-2xl font-bold">{student.name}</h1>
                  <p className="opacity-90">{student.school_class}</p>
                  {student.group_name && (
                    <p className="text-sm opacity-75">
                      Gruppe: {student.group_name}
                    </p>
                  )}
                </div>
              </div>

              {/* Status badges */}
              <div className="absolute top-6 right-6 flex flex-col space-y-2">
                {student.in_house && (
                  <span className="rounded-full bg-green-400/80 px-2 py-1 text-xs text-white">
                    Im Haus
                  </span>
                )}
                {student.wc && (
                  <span className="rounded-full bg-blue-400/80 px-2 py-1 text-xs text-white">
                    Toilette
                  </span>
                )}
                {student.school_yard && (
                  <span className="rounded-full bg-yellow-400/80 px-2 py-1 text-xs text-white">
                    Schulhof
                  </span>
                )}
                {student.bus && (
                  <span className="rounded-full bg-orange-400/80 px-2 py-1 text-xs text-white">
                    Bus
                  </span>
                )}
              </div>
            </div>

            {/* Content */}
            <div className="p-6">
              <div className="mb-6 flex items-center justify-between">
                <h2 className="text-xl font-medium text-gray-700">
                  Schülerdetails
                </h2>
                <div className="flex space-x-2">
                  <button
                    onClick={() => setIsEditing(true)}
                    className="rounded-lg bg-blue-50 px-4 py-2 text-blue-600 shadow-sm transition-colors hover:bg-blue-100"
                  >
                    Bearbeiten
                  </button>
                  <button
                    onClick={handleDelete}
                    className="rounded-lg bg-red-50 px-4 py-2 text-red-600 shadow-sm transition-colors hover:bg-red-100"
                  >
                    Löschen
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-1 gap-8 md:grid-cols-2">
                {/* Personal Information */}
                <div className="space-y-4">
                  <h3 className="border-b border-blue-200 pb-2 text-lg font-medium text-blue-800">
                    Persönliche Daten
                  </h3>

                  <div>
                    <div className="text-sm text-gray-500">Vorname</div>
                    <div className="text-base">{student.first_name}</div>
                  </div>

                  <div>
                    <div className="text-sm text-gray-500">Nachname</div>
                    <div className="text-base">{student.second_name}</div>
                  </div>

                  <div>
                    <div className="text-sm text-gray-500">Klasse</div>
                    <div className="text-base">{student.school_class}</div>
                  </div>

                  <div>
                    <div className="text-sm text-gray-500">Gruppe</div>
                    <div className="text-base">
                      {student.group_id && student.group_name ? (
                        <a
                          href={`/database/groups/${student.group_id}`}
                          className="text-blue-600 transition-colors hover:text-blue-800 hover:underline"
                        >
                          {student.group_name}
                        </a>
                      ) : (
                        "Keine Gruppe zugewiesen"
                      )}
                    </div>
                  </div>

                  <div>
                    <div className="text-sm text-gray-500">IDs</div>
                    <div className="flex flex-col text-xs text-gray-600">
                      <span>Student: {student.id}</span>
                      {student.custom_users_id && (
                        <span>Benutzer: {student.custom_users_id}</span>
                      )}
                      {student.group_id && (
                        <span>Gruppe: {student.group_id}</span>
                      )}
                    </div>
                  </div>
                </div>

                {/* Guardian Information and Status */}
                <div className="space-y-8">
                  <div className="space-y-4">
                    <h3 className="border-b border-purple-200 pb-2 text-lg font-medium text-purple-800">
                      Erziehungsberechtigte
                    </h3>

                    <div>
                      <div className="text-sm text-gray-500">Name</div>
                      <div className="text-base">
                        {student.name_lg ?? "Nicht angegeben"}
                      </div>
                    </div>

                    <div>
                      <div className="text-sm text-gray-500">Kontakt</div>
                      <div className="text-base">
                        {student.contact_lg ?? "Nicht angegeben"}
                      </div>
                    </div>
                  </div>

                  <div className="space-y-4">
                    <h3 className="border-b border-green-200 pb-2 text-lg font-medium text-green-800">
                      Status
                    </h3>

                    <div className="grid grid-cols-2 gap-4">
                      <div
                        className={`rounded-lg p-3 ${student.in_house ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-500"}`}
                      >
                        <span className="flex items-center">
                          <span
                            className={`mr-2 inline-block h-3 w-3 rounded-full ${student.in_house ? "bg-green-500" : "bg-gray-300"}`}
                          ></span>
                          Im Haus
                        </span>
                      </div>

                      <div
                        className={`rounded-lg p-3 ${student.wc ? "bg-blue-100 text-blue-800" : "bg-gray-100 text-gray-500"}`}
                      >
                        <span className="flex items-center">
                          <span
                            className={`mr-2 inline-block h-3 w-3 rounded-full ${student.wc ? "bg-blue-500" : "bg-gray-300"}`}
                          ></span>
                          Toilette
                        </span>
                      </div>

                      <div
                        className={`rounded-lg p-3 ${student.school_yard ? "bg-yellow-100 text-yellow-800" : "bg-gray-100 text-gray-500"}`}
                      >
                        <span className="flex items-center">
                          <span
                            className={`mr-2 inline-block h-3 w-3 rounded-full ${student.school_yard ? "bg-yellow-500" : "bg-gray-300"}`}
                          ></span>
                          Schulhof
                        </span>
                      </div>

                      <div
                        className={`rounded-lg p-3 ${student.bus ? "bg-orange-100 text-orange-800" : "bg-gray-100 text-gray-500"}`}
                      >
                        <span className="flex items-center">
                          <span
                            className={`mr-2 inline-block h-3 w-3 rounded-full ${student.bus ? "bg-orange-500" : "bg-gray-300"}`}
                          ></span>
                          Bus
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
