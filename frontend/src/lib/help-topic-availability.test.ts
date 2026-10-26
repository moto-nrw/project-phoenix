import { describe, expect, it } from "vitest";

import {
  getHelpTopics,
  helpTopicMatchesRole,
  HELP_ROLES,
} from "~/components/help/help-content";
import {
  HELP_TOPIC_AUDIENCE,
  isHelpTopicAvailable,
  type HelpGroupMode,
  type HelpPresenceMode,
} from "~/lib/help-topic-availability";
import { HELP_TOPICS } from "~/lib/help-topics";

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

describe("HELP_TOPIC_AUDIENCE", () => {
  it("matches the audience of every article", () => {
    const articles = getHelpTopics("unknown", "unknown", null);
    expect(
      Object.fromEntries(articles.map((topic) => [topic.id, topic.audience])),
    ).toEqual(HELP_TOPIC_AUDIENCE);
  });
});

// Das Fragezeichen im Seitenkopf entscheidet ohne die Artikeltexte. Es muss
// für jede Rolle und Einstellung dasselbe sagen wie die Anleitung selbst.
describe("isHelpTopicAvailable", () => {
  for (const presenceMode of PRESENCE_MODES) {
    for (const groupMode of GROUP_MODES) {
      for (const nfcEnabled of NFC_STATES) {
        it(`agrees with getHelpTopics (${presenceMode}, ${groupMode}, nfc ${String(nfcEnabled)})`, () => {
          const topics = getHelpTopics(presenceMode, groupMode, nfcEnabled);
          for (const role of HELP_ROLES) {
            for (const id of Object.values(HELP_TOPICS)) {
              const expected = topics.some(
                (topic) => topic.id === id && helpTopicMatchesRole(topic, role),
              );
              expect(
                isHelpTopicAvailable(
                  id,
                  role,
                  presenceMode,
                  groupMode,
                  nfcEnabled,
                ),
                `${id} für ${role}`,
              ).toBe(expected);
            }
          }
        });
      }
    }
  }
});
