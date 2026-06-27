"use client";

import { createLogger } from "~/lib/logger";
import { useState } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { useTenantRouter } from "~/lib/tenant-router";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import { CombinedGroupForm } from "@/components/groups";
import type { CombinedGroup } from "@/lib/api";
import { combinedGroupService } from "@/lib/api";

const logger = createLogger({ component: "NewCombinedGroupPage" });
const COMBINED_GROUPS_BACK_URL = "/database/groups/combined";

export default function NewCombinedGroupPage() {
  const router = useTenantRouter();
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
      logger.error("failed to create combined group", {
        error: err instanceof Error ? err.message : String(err),
      });
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
      <main className="mx-auto max-w-4xl p-4">
        <MobileBackButton
          href={COMBINED_GROUPS_BACK_URL}
          ariaLabel="Zurück zu Gruppenkombinationen"
        />

        <div className="mb-4 hidden md:block">
          <Link
            href={COMBINED_GROUPS_BACK_URL}
            className="mb-3 inline-flex items-center gap-2 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
          >
            <ArrowLeft className="h-4 w-4" aria-hidden />
            Zurück
          </Link>
          <h1 className="text-2xl font-semibold text-gray-900">
            Neue Gruppenkombination
          </h1>
        </div>

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
