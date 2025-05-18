"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { PageHeader } from "@/components/dashboard";
import { GroupForm } from "@/components/groups";
import { StudentList } from "@/components/students";
import type { Group, Student } from "@/lib/api";
import { groupService } from "@/lib/api";

export default function GroupDetailPage() {
  const router = useRouter();
  const params = useParams();
  const groupId = params.id as string;

  const [group, setGroup] = useState<Group | null>(null);
  const [students, setStudents] = useState<Student[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);

  useEffect(() => {
    const fetchGroupDetails = async () => {
      try {
        setLoading(true);

        // TODO: Implement student fetching when student API is complete
        // For now, only fetch group data
        const groupData = await groupService.getGroup(groupId);
        console.log("Group data received in page:", groupData);

        if (!groupData) {
          console.error("No group data returned from API");
          setError("Gruppe konnte nicht gefunden werden.");
          setGroup(null);
          setStudents([]);
          return;
        }

        setGroup(groupData);
        console.log("Group state set to:", groupData);
        setStudents([]); // Empty array for now
        setError(null);
      } catch (err) {
        console.error("Error fetching group details:", err);
        setError(
          "Fehler beim Laden der Gruppendaten. Bitte versuchen Sie es später erneut.",
        );
        setGroup(null);
        setStudents([]);
      } finally {
        setLoading(false);
      }
    };

    if (groupId) {
      void fetchGroupDetails();
    }
  }, [groupId]);

  const handleUpdate = async (formData: Partial<Group>) => {
    try {
      setLoading(true);
      setError(null);

      // Update group
      await groupService.updateGroup(groupId, formData);
      
      // Re-fetch the data to ensure we have all the nested fields properly populated
      const updatedGroup = await groupService.getGroup(groupId);
      setGroup(updatedGroup);
      setIsEditing(false);
    } catch (err) {
      console.error("Error updating group:", err);
      setError(
        "Fehler beim Aktualisieren der Gruppe. Bitte versuchen Sie es später erneut.",
      );
      throw err; // Re-throw to be caught by the form component
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async () => {
    if (
      window.confirm("Sind Sie sicher, dass Sie diese Gruppe löschen möchten?")
    ) {
      try {
        setLoading(true);
        await groupService.deleteGroup(groupId);
        router.push("/database/groups");
      } catch (err) {
        console.error("Error deleting group:", err);
        // Check if the error has a specific message
        const errorMessage =
          err instanceof Error ? err.message : "Unbekannter Fehler";

        // Handle the specific "cannot delete group with students" error case
        if (
          errorMessage.includes("cannot delete group with students") ||
          (errorMessage.includes("cannot delete") &&
            errorMessage.includes("students"))
        ) {
          setError(
            "Die Gruppe kann nicht gelöscht werden, da sie Schüler enthält. Bitte entfernen Sie zuerst alle Schüler aus der Gruppe.",
          );
        } else {
          setError(`Fehler beim Löschen der Gruppe: ${errorMessage}`);
        }

        setLoading(false);
      }
    }
  };

  if (loading && !group) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-50">
        <div className="flex animate-pulse flex-col items-center">
          <div className="h-12 w-12 rounded-full bg-gradient-to-r from-teal-400 to-blue-500"></div>
          <p className="mt-4 text-gray-500">Daten werden geladen...</p>
        </div>
      </div>
    );
  }

  if (error) {
    if (!group) {
      // Full page error when no group is loaded
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
    } else {
      // Add an error alert to the page content when group is still loaded
      // This handles the case where delete fails but we still have the group data

      // For important errors related to deletion constraints, keep them visible longer
      const clearTimeout = error.includes("Schüler enthält") ? 15000 : 5000;
      setTimeout(() => {
        // Auto-clear error after timeout period
        if (error) setError(null);
      }, clearTimeout);
    }
  }

  if (!group) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 p-4">
        <div className="max-w-md rounded-lg bg-yellow-50 p-6 text-yellow-800 shadow-md">
          <h2 className="mb-3 text-lg font-semibold">Gruppe nicht gefunden</h2>
          <p className="mb-4">
            Die angeforderte Gruppe konnte nicht gefunden werden.
          </p>
          <button
            onClick={() => router.push("/database/groups")}
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
        title={isEditing ? "Gruppe bearbeiten" : "Gruppendetails"}
        backUrl="/database/groups"
      />

      {/* Main Content */}
      <main className="mx-auto max-w-4xl p-4">
        {/* Error Alert */}
        {error && group && (
          <div
            className="mb-4 rounded border-l-4 border-red-500 bg-red-100 p-4 text-red-700 shadow-md"
            role="alert"
          >
            <div className="flex items-start">
              <div className="flex-shrink-0">
                {/* Warning icon */}
                <svg
                  className="mr-2 h-5 w-5 text-red-500"
                  xmlns="http://www.w3.org/2000/svg"
                  viewBox="0 0 20 20"
                  fill="currentColor"
                >
                  <path
                    fillRule="evenodd"
                    d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                    clipRule="evenodd"
                  />
                </svg>
              </div>
              <div className="flex-1">
                <p className="font-bold">Aktion nicht möglich</p>
                <p className="mt-1 text-sm">{error}</p>
                {error.includes("Schüler enthält") && (
                  <div className="mt-2 rounded bg-red-50 p-2 text-sm">
                    <p className="font-medium">Hinweis zur Lösung:</p>
                    <ol className="mt-1 ml-2 list-inside list-decimal">
                      <li>Gehen Sie zur Schülerliste</li>
                      <li>
                        Weisen Sie alle Schüler dieser Gruppe einer anderen
                        Gruppe zu
                      </li>
                      <li>
                        Kehren Sie zurück und versuchen Sie erneut, die Gruppe
                        zu löschen
                      </li>
                    </ol>
                  </div>
                )}
              </div>
              <button
                className="ml-2 flex-shrink-0 text-red-500 transition-colors hover:text-red-700"
                onClick={() => setError(null)}
                aria-label="Schließen"
              >
                <span className="text-xl">&times;</span>
              </button>
            </div>
          </div>
        )}

        {isEditing ? (
          <GroupForm
            initialData={group}
            onSubmitAction={handleUpdate}
            onCancelAction={() => setIsEditing(false)}
            isLoading={loading}
            formTitle="Gruppe bearbeiten"
            submitLabel="Speichern"
          />
        ) : (
          <div className="overflow-hidden rounded-lg bg-white shadow-md">
            {/* Group card header */}
            <div className="relative bg-gradient-to-r from-teal-500 to-blue-600 p-6 text-white">
              <div className="flex items-center">
                <div className="mr-5 flex h-20 w-20 items-center justify-center rounded-full bg-white/30 text-3xl font-bold">
                  {group.name?.[0] ?? "G"}
                </div>
                <div>
                  <h1 className="text-2xl font-bold">{group.name}</h1>
                  {group.room_name && (
                    <p className="opacity-90">Raum: {group.room_name}</p>
                  )}
                  {group.representative_name && (
                    <p className="text-sm opacity-75">
                      Vertreter: {group.representative_name}
                    </p>
                  )}
                </div>
              </div>
            </div>

            {/* Content */}
            <div className="p-6">
              <div className="mb-6 flex items-center justify-between">
                <h2 className="text-xl font-medium text-gray-700">
                  Gruppendetails
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

              {/* Group Information (Top Section) */}
              <div className="mb-8">
                <div className="grid grid-cols-1 gap-8 md:grid-cols-2 lg:grid-cols-3">
                  {/* Group Details */}
                  <div className="space-y-4">
                    <h3 className="border-b border-blue-200 pb-2 text-lg font-medium text-blue-800">
                      Gruppendaten
                    </h3>

                    <div>
                      <div className="text-sm text-gray-500">Name</div>
                      <div className="text-base">{group.name}</div>
                    </div>

                    <div>
                      <div className="text-sm text-gray-500">Raum</div>
                      <div className="text-base">
                        {group.room_name ?? "Nicht zugewiesen"}
                      </div>
                    </div>

                    <div>
                      <div className="text-sm text-gray-500">Vertreter</div>
                      <div className="text-base">
                        {group.representative_name ?? "Nicht zugewiesen"}
                      </div>
                    </div>

                    <div>
                      <div className="text-sm text-gray-500">IDs</div>
                      <div className="flex flex-col text-xs text-gray-600">
                        <span>Gruppe: {group.id}</span>
                        {group.room_id && <span>Raum: {group.room_id}</span>}
                        {group.representative_id && (
                          <span>Vertreter: {group.representative_id}</span>
                        )}
                      </div>
                    </div>
                  </div>

                  {/* Supervisors */}
                  <div className="space-y-4">
                    <h3 className="border-b border-purple-200 pb-2 text-lg font-medium text-purple-800">
                      Aufsichtspersonen
                    </h3>

                    {group.supervisors && group.supervisors.length > 0 ? (
                      <div className="space-y-2">
                        {group.supervisors.map((supervisor) => (
                          <div
                            key={supervisor.id}
                            className="rounded-lg bg-purple-50 p-3 transition-colors hover:bg-purple-100"
                          >
                            <span>{supervisor.name}</span>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-gray-500">
                        Keine Aufsichtspersonen zugewiesen.
                      </p>
                    )}
                  </div>
                </div>
              </div>

              {/* Students Section (Full Width) */}
              <div className="border-t border-gray-200 pt-6">
                <div className="mb-4 flex items-center justify-between">
                  <h3 className="border-b border-green-200 pb-2 text-lg font-medium text-green-800">
                    Schüler in dieser Gruppe
                  </h3>

                  <div className="flex items-center gap-4">
                    <div className="text-sm text-gray-500">
                      {students.length > 0
                        ? `${students.length} Schüler`
                        : "Keine Schüler"}
                    </div>

                    <button
                      onClick={() =>
                        router.push("/database/students/new?groupId=" + groupId)
                      }
                      className="flex items-center gap-1 rounded-md bg-green-50 px-3 py-1.5 text-green-600 transition-colors hover:bg-green-100"
                    >
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        className="h-4 w-4"
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
                      <span>Schüler hinzufügen</span>
                    </button>
                  </div>
                </div>

                {students.length === 0 ? (
                  <div className="rounded-lg bg-yellow-50 p-4 text-center text-yellow-800">
                    <p className="mb-2 font-semibold">
                      TODO: Student-API Integration ausstehend
                    </p>
                    <p className="text-sm">
                      Die Anzeige der Schüler in dieser Gruppe wird implementiert, 
                      sobald die Student-API fertiggestellt ist.
                    </p>
                    <div className="mt-3 text-xs text-gray-600">
                      (Entwicklungshinweis: Endpoint /api/groups/{"{id}"}/students fehlt noch)
                    </div>
                  </div>
                ) : (
                  <StudentList students={students} showDetails={true} />
                )}
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
