"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { PageHeader } from "@/components/dashboard";
import { CombinedGroupForm } from "@/components/groups";
import type { CombinedGroup } from "@/lib/api";
import { combinedGroupService } from "@/lib/api";

export default function NewCombinedGroupPage() {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleCreateCombinedGroup = async (
    combinedGroupData: Partial<CombinedGroup>,
  ) => {
    try {
      setLoading(true);
      setError(null);

      // Prepare combined group data
      const newCombinedGroup: Omit<CombinedGroup, "id"> = {
        ...combinedGroupData,
        name: combinedGroupData.name ?? "",
        is_active: combinedGroupData.is_active ?? true,
        access_policy: combinedGroupData.access_policy ?? "manual",
      };

      // Create combined group
      await combinedGroupService.createCombinedGroup(newCombinedGroup);

      // Navigate back to combined groups list on success
      router.push("/database/groups/combined");
    } catch (err) {
      console.error("Error creating combined group:", err);
      setError(
        "Fehler beim Erstellen der Gruppenkombination. Bitte versuchen Sie es später erneut.",
      );
      throw err; // Re-throw to be caught by the form component
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <PageHeader
        title="Neue Gruppenkombination"
        backUrl="/database/groups/combined"
      />

      {/* Main Content */}
      <main className="mx-auto max-w-4xl p-4">
        {error && (
          <div className="mb-6 rounded-lg bg-red-50 p-4 text-red-800">
            <p>{error}</p>
          </div>
        )}

        <CombinedGroupForm
          initialData={{
            name: "",
            is_active: true,
            access_policy: "manual",
            valid_until: "",
            specific_group_id: "",
          }}
          onSubmitAction={handleCreateCombinedGroup}
          onCancelAction={() => router.back()}
          isLoading={loading}
          formTitle="Gruppenkombination erstellen"
          submitLabel="Erstellen"
        />
      </main>
    </div>
  );
}
