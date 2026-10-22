"use client";

import { roomsConfig } from "@/components/database/configs/rooms.config";
import { configToFormSection } from "@/lib/database/types";
import { LOCATION_COLORS } from "@/lib/location-helper";
import {
  isColorLockedRoom,
  isSystemRoom,
  isToiletRoom,
  type Room,
} from "@/lib/room-helpers";
import type {
  FormField,
  FormSection,
} from "~/components/ui/database/database-form";

/**
 * Preview and hint for the schoolyard's colour picker (#2405): without a
 * chosen colour the Schulhof badge renders orange, not the generic room blue,
 * so "Standard" has to preview orange and the copy has to say so.
 *
 * Bound through the field config, so the shared CatalogColorField is reused as
 * is. The copy travels as a prop because custom fields do not render
 * `helperText`.
 */
const SCHULHOF_COLOR_FIELD_PROPS = {
  defaultHex: LOCATION_COLORS.SCHOOLYARD,
  hint: "Ohne eigene Farbe erscheint der Schulhof in Orange.",
};

type Option = { value: string; label: string };

/**
 * Keeps a room's current category selectable when the catalog no longer has
 * it (deleted, or a value from before the catalog, #4352): otherwise saving
 * the form would silently drop the category.
 */
function withCurrentCategory(
  options: FormField["options"],
  current: string,
  systemRoom: boolean,
): FormField["options"] {
  // A system room's category (Schulhof, WC) is no catalog entry by design.
  const label = systemRoom
    ? current
    : `${current} (nicht in den Raumkategorien)`;
  const append = (list: Option[]): Option[] =>
    list.some((option) => option.value.toLowerCase() === current.toLowerCase())
      ? list
      : [...list, { value: current, label }];
  if (typeof options === "function") {
    return async () => append(await options());
  }
  return append(options ?? []);
}

export function buildRoomFormSections(
  room: Room | null | undefined,
): FormSection[] {
  let sections = roomsConfig.form.sections.map(configToFormSection);
  const currentCategory = room?.category;

  sections = sections.map((section) => ({
    ...section,
    fields: section.fields.map((field) => {
      if (field.name === "category" && currentCategory) {
        return {
          ...field,
          options: withCurrentCategory(
            field.options,
            currentCategory,
            isSystemRoom(room),
          ),
        };
      }
      // Without an own color the room shows its category color, so the
      // picker previews that one instead of the generic default.
      if (field.name === "color" && room?.categoryColor) {
        return {
          ...field,
          componentProps: {
            ...field.componentProps,
            defaultHex: room.categoryColor,
          },
        };
      }
      return field;
    }),
  }));

  // Toilet rooms can never be an open room — the backend refuses the release
  // with ErrToiletRoomNotReleasable. A switch that always fails is worse than
  // no switch, so it is dropped rather than disabled (#3064).
  if (isToiletRoom(room)) {
    sections = sections.map((section) => ({
      ...section,
      fields: section.fields.filter((field) => field.name !== "isOpenRoom"),
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
            return { ...field, componentProps: SCHULHOF_COLOR_FIELD_PROPS };
          }
          return field;
        }),
    }));
  }

  return sections;
}
