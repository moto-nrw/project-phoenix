import type { Icon as PhosphorIcon, IconProps } from "@phosphor-icons/react";
import type { MotoDuotoneTone } from "~/lib/location-helper";

export type MotoConceptKind = "core" | "status" | "function";
export type MotoConceptSection =
  | "people"
  | "presence"
  | "planning"
  | "communication"
  | "administration"
  | "system";

export interface MotoConceptDefinition<K extends string = string> {
  readonly key: K;
  readonly label: string;
  readonly icon: PhosphorIcon;
  readonly tone: MotoDuotoneTone;
  readonly kind: MotoConceptKind;
  readonly section: MotoConceptSection;
  readonly weight: IconProps["weight"];
}

/**
 * Builds one concept of the concept system. Every concept lives in its own
 * module under `~/lib/concepts/`, so a page that imports one concept carries
 * only that concept's Phosphor glyph. Importing the whole registry
 * (`MOTO_CONCEPTS` in `~/lib/moto-concepts`) carries every glyph; keep that
 * for lookups whose key is only known at runtime.
 */
export function defineMotoConcept<const K extends string>(
  key: K,
  label: string,
  icon: PhosphorIcon,
  tone: MotoDuotoneTone,
  kind: MotoConceptKind,
  section: MotoConceptSection,
  weight: IconProps["weight"] = "duotone",
): MotoConceptDefinition<K> {
  return { key, label, icon, tone, kind, section, weight };
}
