"use client";

import { createElement, type ComponentProps } from "react";
import { roomsConfig } from "@/components/database/configs/rooms.config";
import { RoomColorField } from "@/components/ui/database/room-color-field";
import { configToFormSection } from "@/lib/database/types";
import { LOCATION_COLORS } from "@/lib/location-helper";
import { isColorLockedRoom, isSystemRoom, type Room } from "@/lib/room-helpers";
import type { FormSection } from "~/components/ui/database/database-form";

/**
 * The shared {@link RoomColorField} with the schoolyard's preview and hint
 * bound in (#2405): without a chosen colour the Schulhof badge renders orange,
 * not the generic room blue, so "Standard" has to preview orange.
 *
 * DatabaseForm hands custom fields a fixed prop set, so these two props cannot
 * travel through the field config and are bound here instead of by a second
 * component in the UI kit. Module scope keeps the component identity stable
 * across renders, so the picker is never remounted mid-edit.
 */
const SchulhofColorField = (props: ComponentProps<typeof RoomColorField>) =>
  createElement(RoomColorField, {
    ...props,
    defaultHex: LOCATION_COLORS.SCHOOLYARD,
    hint: "Ohne eigene Farbe erscheint der Schulhof in Orange. Wähle eine Farbe, die du noch nicht für einen anderen Raum benutzt.",
  });

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
    // Colour stays editable for the Schulhof (#2405) — schools colour-code
    // rooms and tablets and need the yard in that scheme; without a chosen
    // colour it keeps rendering in the orange Schulhof default. Only the
    // toilet rooms drop the picker: they have no badge of their own, so a
    // colour there would configure nothing, and the backend rejects the
    // change anyway. Name stays locked for every system room.
    const dropColor = isColorLockedRoom(room);
    sections = sections.map((section) => ({
      ...section,
      fields: section.fields
        .filter((field) => !(dropColor && field.name === "color"))
        .map((field) => {
          if (field.name === "name") {
            return {
              ...field,
              disabled: true,
              helperText: "Systemraum: Name kann nicht geändert werden",
            };
          }
          if (field.name === "color") {
            // Swap in the Schulhof-bound picker. Custom fields do not render
            // `helperText`, so the copy has to reach the component as a prop.
            return { ...field, component: SchulhofColorField };
          }
          return field;
        }),
    }));
  }

  return sections;
}
