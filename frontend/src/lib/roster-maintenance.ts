import type {
  RosterMaintenanceMode,
  TemplateRosterMaintenance,
} from "~/lib/timetable-types";

/**
 * Wording of the Regeltermin indicator (#3140). The indicator names how
 * later children reach a Regeltermin; it is information, never a warning.
 * Every state keeps its own label and icon so colour is never the only cue.
 */
export interface RosterMaintenanceDescription {
  mode: RosterMaintenanceMode;
  label: string;
  explanation: string;
}

const LABELS: Record<RosterMaintenanceMode, string> = {
  automatic: "Automatisch",
  partial: "Teils automatisch",
  manual: "Manuell",
};

function joinNames(names: readonly string[]): string {
  if (names.length <= 1) return names[0] ?? "";
  return `${names.slice(0, -1).join(", ")} und ${names.at(-1)}`;
}

function offeringPhrase(names: readonly string[]): string {
  return names.length === 1
    ? `Betreuungsangebot „${names[0]}“`
    : `Betreuungsangebote ${joinNames(names.map((name) => `„${name}“`))}`;
}

function filterSentence(state: TemplateRosterMaintenance): string | null {
  if (state.schoolClasses.length > 0) {
    return state.schoolClasses.length === 1
      ? `Nur Klasse ${state.schoolClasses[0]}.`
      : `Nur Klassen ${joinNames(state.schoolClasses)}.`;
  }
  if (state.gradeLevels.length > 0) {
    const grades = [...state.gradeLevels].sort((a, b) => a - b).map(String);
    return grades.length === 1
      ? `Nur Jahrgang ${grades[0]}.`
      : `Nur Jahrgänge ${joinNames(grades)}.`;
  }
  return null;
}

function inactiveSentence(names: readonly string[]): string | null {
  if (names.length === 0) return null;
  return names.length === 1
    ? `Das Betreuungsangebot „${names[0]}“ ist ausgeschaltet.`
    : `Die Betreuungsangebote ${joinNames(names.map((name) => `„${name}“`))} sind ausgeschaltet.`;
}

function invalidSourceSentence(names: readonly string[]): string | null {
  if (names.length === 0) return null;
  return names.length === 1
    ? `Das Betreuungsangebot „${names[0]}“ passt nicht zu diesem Regeltermin.`
    : `Die Betreuungsangebote ${joinNames(names.map((name) => `„${name}“`))} passen nicht zu diesem Regeltermin.`;
}

const TARGETS_SENTENCE =
  "Klasse, Jahrgang oder Gruppe gelten nur für neu erzeugte Termine.";

export function describeRosterMaintenance(
  state: TemplateRosterMaintenance,
): RosterMaintenanceDescription {
  const sentences: string[] = [];
  if (state.mode === "automatic") {
    sentences.push("Neue Anmeldungen werden automatisch übernommen.");
    sentences.push(`Quelle: ${offeringPhrase(state.offeringNames)}.`);
    const filter = filterSentence(state);
    if (filter) sentences.push(filter);
  } else if (state.mode === "partial") {
    sentences.push(
      `Neue Anmeldungen für ${state.offeringNames.length === 1 ? "das" : "die"} ${offeringPhrase(state.offeringNames)} werden automatisch übernommen.`,
    );
    const filter = filterSentence(state);
    if (filter) sentences.push(filter);
    const inactive = inactiveSentence(state.inactiveOfferingNames);
    if (inactive) sentences.push(inactive);
    const invalid = invalidSourceSentence(state.invalidOfferingNames);
    if (invalid) sentences.push(invalid);
    if (state.dynamicTargets) sentences.push(TARGETS_SENTENCE);
    sentences.push("Weitere Kinder müssen Sie selbst ergänzen.");
  } else {
    sentences.push("Neue Kinder müssen Sie selbst ergänzen.");
    if (state.careOfferingsDisabled) {
      sentences.push("Betreuungsangebote sind an Ihrer Schule ausgeschaltet.");
    } else {
      const inactive = inactiveSentence(state.inactiveOfferingNames);
      if (inactive) sentences.push(inactive);
      const invalid = invalidSourceSentence(state.invalidOfferingNames);
      if (invalid) sentences.push(invalid);
    }
    if (state.dynamicTargets) sentences.push(TARGETS_SENTENCE);
  }
  return {
    mode: state.mode,
    label: LABELS[state.mode],
    explanation: sentences.join(" "),
  };
}
