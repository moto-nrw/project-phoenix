import { describe, expect, it } from "vitest";

import {
  getHelpTopics,
  helpTopicMatchesRole,
  HELP_GROUPS,
  HELP_ROLES,
  HOME_TOPIC_COUNT,
  type HelpGroupMode,
  type HelpPresenceMode,
  type HelpTopic,
} from "./help-content";

const PRESENCE_MODES: readonly HelpPresenceMode[] = [
  "detailed",
  "binary",
  "unknown",
];
const GROUP_MODES: readonly HelpGroupMode[] = [
  "fixed_groups",
  "open_care",
  "unknown",
];
const NFC_STATES: readonly (boolean | null)[] = [true, false, null];

function duplicateIconTitles(topics: readonly HelpTopic[]): string[][] {
  const byIcon = new Map<unknown, string[]>();
  for (const topic of topics) {
    byIcon.set(topic.icon, [...(byIcon.get(topic.icon) ?? []), topic.title]);
  }
  return [...byIcon.values()].filter((titles) => titles.length > 1);
}

// Eine Kacheluebersicht zeigt jedes Thema mit seinem Icon. Zwei gleiche
// Icons nebeneinander lassen die Themen gleich aussehen.
describe("help overview icons", () => {
  for (const presenceMode of PRESENCE_MODES) {
    for (const groupMode of GROUP_MODES) {
      for (const nfcEnabled of NFC_STATES) {
        const topics = getHelpTopics(presenceMode, groupMode, nfcEnabled);

        for (const role of HELP_ROLES) {
          const roleTopics = topics.filter((topic) =>
            helpTopicMatchesRole(topic, role),
          );
          const settings = `${role}, ${presenceMode}, ${groupMode}, nfc ${String(nfcEnabled)}`;

          it(`uses distinct icons on the start page (${settings})`, () => {
            expect(
              duplicateIconTitles(roleTopics.slice(0, HOME_TOPIC_COUNT)),
            ).toEqual([]);
          });

          it(`uses distinct icons in every category (${settings})`, () => {
            const duplicates = HELP_GROUPS.flatMap((group) =>
              duplicateIconTitles(
                roleTopics.filter((topic) => topic.group === group),
              ),
            );
            expect(duplicates).toEqual([]);
          });
        }
      }
    }
  }
});
