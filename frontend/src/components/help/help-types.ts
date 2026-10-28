import type { HelpIconName } from "./help-icons";
import type { HelpAudience } from "~/lib/help-topic-availability";
import type { HelpTopicId } from "~/lib/help-topics";
export type { HelpTopicId } from "~/lib/help-topics";
export type {
  HelpGroupMode,
  HelpPresenceMode,
} from "~/lib/help-topic-availability";

export type HelpTopicGroup =
  // Betreuungskraefte
  | "einstieg"
  | "tagesplanung"
  | "kinder"
  | "gruppen"
  | "team"
  | "arbeitszeit"
  | "nfc"
  | "probleme"
  // OGS-Leitung. „anmeldeverwaltung" heisst absichtlich nicht „anmeldung":
  // so heisst die Eltern-Gruppe, in der sich Eltern selbst anmelden.
  | "einrichten"
  | "kinderdaten"
  | "elternarbeit"
  | "anmeldeverwaltung"
  | "personal"
  | "planung"
  | "auswertung"
  | "konfiguration"
  // Eltern
  | "mein-kind"
  | "nachrichten"
  | "anmeldung"
  // Lehrkraefte
  | "klasse"
  | "aufsicht";

export interface HelpTopic {
  readonly id: HelpTopicId;
  readonly title: string;
  readonly question: string;
  readonly summary: string;
  readonly group: HelpTopicGroup;
  /**
   * Wer den Artikel in Seitenleiste und Suche sieht. Eine Liste, wo mehrere
   * Rollen denselben Ablauf haben -- eine Leitung meldet sich genauso an wie
   * eine Betreuungskraft. Zwei fast gleiche Artikel waeren sonst die Folge,
   * und einer davon liefe irgendwann hinterher. `"all"` heisst jede Rolle.
   */
  readonly audience: HelpAudience;
  readonly icon: HelpIconName;
  readonly requirements?: readonly string[];
  readonly steps: readonly string[];
  readonly instructionGroups?: readonly {
    readonly title: string;
    readonly description?: string;
    readonly steps: readonly string[];
    readonly ordered?: boolean;
  }[];
  readonly result?: string;
  readonly differences?: readonly string[];
  /**
   * Abschnitt „Tipp": optionale Hinweise, die den Ablauf weder aendern noch
   * voraussetzen -- „das geht auch noch". Nicht zu verwechseln mit
   * `differences` („Wenn es anders aussieht": der Bildschirm weicht ab) und
   * `troubleshootingDetails` („Wenn es nicht klappt": etwas fehlt).
   */
  readonly notes?: readonly string[];
  readonly troubleshooting?: HelpTopicId;
  readonly troubleshootingDetails?: readonly string[];
  readonly image?: string;
  readonly imageAlt?: string;
  readonly related: readonly HelpTopicId[];
}
