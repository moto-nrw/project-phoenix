"use client";

import { roomsConfig } from "@/lib/database/configs/rooms.config";
import { configToFormSection } from "@/lib/database/types";
import { isSystemRoom, type Room } from "@/lib/room-helpers";
import type { FormSection } from "~/components/ui/database/database-form";

const STANDARD_CATEGORIES = new Set([
  "Normaler Raum",
  "Gruppenraum",
  "Themenraum",
  "Sport",
]);

export function buildRoomFormSections(
  room: Room | null | undefined,
): FormSection[] {
  let sections = roomsConfig.form.sections.map(configToFormSection);
  const legacyCategory = room?.category;

  if (legacyCategory && !STANDARD_CATEGORIES.has(legacyCategory)) {
    sections = sections.map((section) => ({
      ...section,
      fields: section.fields.map((field) => {
        if (field.name !== "category" || !Array.isArray(field.options)) {
          return field;
        }

        const hasLegacyOption = field.options.some(
          (option) => option.value === legacyCategory,
        );
        if (hasLegacyOption) {
          return field;
        }

        return {
          ...field,
          options: [
            ...field.options,
            { value: legacyCategory, label: `${legacyCategory} (Legacy)` },
          ],
        };
      }),
    }));
  }

  if (isSystemRoom(room)) {
    sections = sections.map((section) => ({
      ...section,
      fields: section.fields.map((field) =>
        field.name === "name"
          ? {
              ...field,
              disabled: true,
              helperText: "Systemraum: Name kann nicht geändert werden",
            }
          : field,
      ),
    }));
  }

  return sections;
}
