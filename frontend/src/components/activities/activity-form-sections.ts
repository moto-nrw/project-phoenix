"use client";

import { activitiesConfig } from "@/components/database/configs/activities.config";
import { configToFormSection } from "@/lib/database/types";
import { isSystemActivity, type Activity } from "@/lib/activity-helpers";
import type { FormSection } from "~/components/ui/database/database-form";

export function buildActivityFormSections(
  activity: Activity | null | undefined,
): FormSection[] {
  const sections = activitiesConfig.form.sections.map(configToFormSection);

  if (!isSystemActivity(activity)) return sections;

  return sections.map((section) => ({
    ...section,
    fields: section.fields.map((field) =>
      field.name === "name"
        ? {
            ...field,
            disabled: true,
            helperText: "Systemaktivität: Name kann nicht geändert werden",
          }
        : field,
    ),
  }));
}
