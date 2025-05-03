"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { PageHeader } from "@/components/dashboard";
import { CombinedGroupForm } from "@/components/groups";
import type { CombinedGroup } from "@/lib/api";
import { combinedGroupService } from "@/lib/api";

export default function CombinedGroupDetailPage() {
  const router = useRouter();
  const params = useParams();
  const combinedGroupId = params.id as string;

  const [combinedGroup, setCombinedGroup] = useState<CombinedGroup | null>(
    null,
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);

  useEffect(() => {
    const fetchCombinedGroupDetails = async () => {
      try {
        setLoading(true);
        const combinedGroupData =
          await combinedGroupService.getCombinedGroup(combinedGroupId);
        setCombinedGroup(combinedGroupData);
        setError(null);
      } catch (err) {
        console.error("Error fetching combined group details:", err);
        setError(
          "Fehler beim Laden der Gruppenkombination. Bitte versuchen Sie es später erneut.",
        );
        setCombinedGroup(null);
      } finally {
        setLoading(false);
      }
    };

    if (combinedGroupId) {
      void fetchCombinedGroupDetails();
    }
  }, [combinedGroupId]);

  const handleUpdate = async (formData: Partial<CombinedGroup>) => {
    try {
      setLoading(true);
      setError(null);

      // Update combined group
      const updatedCombinedGroup =
        await combinedGroupService.updateCombinedGroup(
          combinedGroupId,
          formData,
        );
      setCombinedGroup(updatedCombinedGroup);
      setIsEditing(false);
    } catch (err) {
      console.error("Error updating combined group:", err);
      setError(
        "Fehler beim Aktualisieren der Gruppenkombination. Bitte versuchen Sie es später erneut.",
      );
      throw err; // Re-throw to be caught by the form component
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async () => {
    if (
      window.confirm(
        "Sind Sie sicher, dass Sie diese Gruppenkombination löschen möchten?",
      )
    ) {
      try {
        setLoading(true);
        await combinedGroupService.deleteCombinedGroup(combinedGroupId);
        router.push("/database/groups/combined");
      } catch (err) {
        console.error("Error deleting combined group:", err);
        setError(
          "Fehler beim Löschen der Gruppenkombination. Bitte versuchen Sie es später erneut.",
        );
        setLoading(false);
      }
    }
  };

  const handleAddGroup = async (groupId: string) => {
    try {
      setLoading(true);
      await combinedGroupService.addGroupToCombined(combinedGroupId, groupId);

      // Refresh the combined group data
      const updatedCombinedGroup =
        await combinedGroupService.getCombinedGroup(combinedGroupId);
      setCombinedGroup(updatedCombinedGroup);
    } catch (err) {
      console.error("Error adding group to combined group:", err);
      setError(
        "Fehler beim Hinzufügen der Gruppe zur Kombination. Bitte versuchen Sie es später erneut.",
      );
    } finally {
      setLoading(false);
    }
  };

  const handleRemoveGroup = async (groupId: string) => {
    try {
      setLoading(true);
      await combinedGroupService.removeGroupFromCombined(
        combinedGroupId,
        groupId,
      );

      // Refresh the combined group data
      const updatedCombinedGroup =
        await combinedGroupService.getCombinedGroup(combinedGroupId);
      setCombinedGroup(updatedCombinedGroup);
    } catch (err) {
      console.error("Error removing group from combined group:", err);
      setError(
        "Fehler beim Entfernen der Gruppe aus der Kombination. Bitte versuchen Sie es später erneut.",
      );
    } finally {
      setLoading(false);
    }
  };

  if (loading && !combinedGroup) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-50">
        <div className="flex animate-pulse flex-col items-center">
          <div className="h-12 w-12 rounded-full bg-gradient-to-r from-teal-400 to-blue-500"></div>
          <p className="mt-4 text-gray-500">Daten werden geladen...</p>
        </div>
      </div>
    );
  }

  if (error && !combinedGroup) {
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

  if (!combinedGroup) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 p-4">
        <div className="max-w-md rounded-lg bg-yellow-50 p-6 text-yellow-800 shadow-md">
          <h2 className="mb-3 text-lg font-semibold">
            Gruppenkombination nicht gefunden
          </h2>
          <p className="mb-4">
            Die angeforderte Gruppenkombination konnte nicht gefunden werden.
          </p>
          <button
            onClick={() => router.push("/database/groups/combined")}
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
        title={
          isEditing
            ? "Gruppenkombination bearbeiten"
            : "Gruppenkombination Details"
        }
        backUrl="/database/groups/combined"
      />

      {/* Main Content */}
      <main className="mx-auto max-w-4xl p-4">
        {isEditing ? (
          <CombinedGroupForm
            initialData={combinedGroup}
            onSubmitAction={handleUpdate}
            onCancelAction={() => setIsEditing(false)}
            isLoading={loading}
            formTitle="Gruppenkombination bearbeiten"
            submitLabel="Speichern"
          />
        ) : (
          <div className="overflow-hidden rounded-lg bg-white shadow-md">
            {/* Combined Group card header */}
            <div className="relative bg-gradient-to-r from-purple-500 to-blue-600 p-6 text-white">
              <div className="flex items-center">
                <div className="mr-5 flex h-20 w-20 items-center justify-center rounded-full bg-white/30 text-3xl font-bold">
                  {combinedGroup.name[0] ?? "G"}
                </div>
                <div>
                  <h1 className="text-2xl font-bold">{combinedGroup.name}</h1>
                  <div className="mt-1 flex flex-wrap gap-2">
                    {combinedGroup.is_active && !combinedGroup.is_expired && (
                      <span className="rounded-full bg-green-400/80 px-2 py-0.5 text-xs text-white">
                        Aktiv
                      </span>
                    )}
                    {combinedGroup.is_expired && (
                      <span className="rounded-full bg-red-400/80 px-2 py-0.5 text-xs text-white">
                        Abgelaufen
                      </span>
                    )}
                    {combinedGroup.valid_until && (
                      <span className="rounded-full bg-blue-400/80 px-2 py-0.5 text-xs text-white">
                        Gültig bis:{" "}
                        {new Date(
                          combinedGroup.valid_until,
                        ).toLocaleDateString()}
                      </span>
                    )}
                  </div>
                  {combinedGroup.time_until_expiration && (
                    <p className="text-sm opacity-75">
                      Läuft ab in: {combinedGroup.time_until_expiration}
                    </p>
                  )}
                </div>
              </div>
            </div>

            {/* Content */}
            <div className="p-6">
              <div className="mb-6 flex items-center justify-between">
                <h2 className="text-xl font-medium text-gray-700">
                  Kombinationsdetails
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

              {error && (
                <div className="mb-6 rounded-lg bg-red-50 p-4 text-red-800">
                  <p>{error}</p>
                </div>
              )}

              <div className="grid grid-cols-1 gap-8 md:grid-cols-2">
                {/* Combined Group Information */}
                <div className="space-y-4">
                  <h3 className="border-b border-blue-200 pb-2 text-lg font-medium text-blue-800">
                    Basisdaten
                  </h3>

                  <div>
                    <div className="text-sm text-gray-500">Name</div>
                    <div className="text-base">{combinedGroup.name}</div>
                  </div>

                  <div>
                    <div className="text-sm text-gray-500">Status</div>
                    <div className="text-base">
                      {combinedGroup.is_active && !combinedGroup.is_expired
                        ? "Aktiv"
                        : combinedGroup.is_expired
                          ? "Abgelaufen"
                          : "Inaktiv"}
                    </div>
                  </div>

                  <div>
                    <div className="text-sm text-gray-500">Gültig bis</div>
                    <div className="text-base">
                      {combinedGroup.valid_until
                        ? new Date(
                            combinedGroup.valid_until,
                          ).toLocaleDateString()
                        : "Kein Ablaufdatum"}
                    </div>
                  </div>

                  <div>
                    <div className="text-sm text-gray-500">Zugriffsmethode</div>
                    <div className="text-base">
                      {combinedGroup.access_policy === "all"
                        ? "Alle Gruppen"
                        : combinedGroup.access_policy === "first"
                          ? "Erste Gruppe"
                          : combinedGroup.access_policy === "specific"
                            ? "Spezifische Gruppe"
                            : "Manuell"}
                    </div>
                  </div>

                  {combinedGroup.access_policy === "specific" &&
                    combinedGroup.specific_group && (
                      <div>
                        <div className="text-sm text-gray-500">
                          Spezifische Gruppe
                        </div>
                        <div className="text-base">
                          {combinedGroup.specific_group.name}
                        </div>
                      </div>
                    )}

                  <div>
                    <div className="text-sm text-gray-500">IDs</div>
                    <div className="flex flex-col text-xs text-gray-600">
                      <span>Kombination: {combinedGroup.id}</span>
                      {combinedGroup.specific_group_id && (
                        <span>
                          Spezifische Gruppe: {combinedGroup.specific_group_id}
                        </span>
                      )}
                    </div>
                  </div>
                </div>

                {/* Groups in Combination */}
                <div className="space-y-8">
                  <div className="space-y-4">
                    <h3 className="border-b border-green-200 pb-2 text-lg font-medium text-green-800">
                      Gruppen in dieser Kombination
                    </h3>

                    {combinedGroup.groups && combinedGroup.groups.length > 0 ? (
                      <div className="space-y-2">
                        {combinedGroup.groups.map((group) => (
                          <div
                            key={group.id}
                            className="flex items-center justify-between rounded-lg bg-green-50 p-3"
                          >
                            <div>
                              <span className="font-medium">{group.name}</span>
                              {group.room_name && (
                                <span className="ml-2 text-xs text-gray-500">
                                  Raum: {group.room_name}
                                </span>
                              )}
                            </div>
                            <button
                              onClick={() => handleRemoveGroup(group.id)}
                              className="text-red-500 transition-colors hover:text-red-700"
                              title="Gruppe entfernen"
                            >
                              <svg
                                xmlns="http://www.w3.org/2000/svg"
                                className="h-5 w-5"
                                viewBox="0 0 20 20"
                                fill="currentColor"
                              >
                                <path
                                  fillRule="evenodd"
                                  d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                                  clipRule="evenodd"
                                />
                              </svg>
                            </button>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-gray-500">
                        Keine Gruppen in dieser Kombination.
                      </p>
                    )}

                    {/* Simple form to add a group */}
                    <div className="mt-4 rounded-lg border border-green-200 bg-white p-3">
                      <h4 className="mb-2 text-sm font-medium text-green-800">
                        Gruppe hinzufügen
                      </h4>
                      <div className="flex space-x-2">
                        <input
                          type="text"
                          id="add_group_id"
                          name="add_group_id"
                          placeholder="Gruppen-ID"
                          className="flex-1 rounded-lg border border-gray-300 px-3 py-2 transition-all duration-200 focus:ring-2 focus:ring-green-500 focus:outline-none"
                        />
                        <button
                          onClick={() => {
                            const input = document.getElementById(
                              "add_group_id",
                            ) as HTMLInputElement;
                            if (input?.value) {
                              void handleAddGroup(input.value);
                              input.value = "";
                            }
                          }}
                          className="rounded-lg bg-green-500 px-4 py-2 text-white transition-colors hover:bg-green-600"
                          disabled={loading}
                        >
                          Hinzufügen
                        </button>
                      </div>
                    </div>
                  </div>

                  <div className="space-y-4">
                    <h3 className="border-b border-purple-200 pb-2 text-lg font-medium text-purple-800">
                      Zugriffsberechtigte Personen
                    </h3>

                    {combinedGroup.access_specialists &&
                    combinedGroup.access_specialists.length > 0 ? (
                      <div className="space-y-2">
                        {combinedGroup.access_specialists.map((specialist) => (
                          <div
                            key={specialist.id}
                            className="rounded-lg bg-purple-50 p-2"
                          >
                            <span>{specialist.name}</span>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-gray-500">
                        Keine speziellen Zugriffspersonen festgelegt.
                      </p>
                    )}
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
