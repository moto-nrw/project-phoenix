import { describe, expect, it } from "vitest";

import { parentAttentionLabel } from "~/components/students/parent-attention-badge";
import { newParentMessages } from "~/components/students/parent-attention-dialog";
import type { Message } from "~/lib/parent-messages-api";

function message(
  id: string,
  senderKind: Message["sender_kind"],
  extra: Partial<Message> = {},
): Message {
  return {
    id,
    sender_kind: senderKind,
    sender_name: "",
    body: `Nachricht ${id}`,
    created_at: "2026-09-09T08:00:00Z",
    ...extra,
  };
}

describe("parentAttentionLabel", () => {
  it("names what is new and for which child", () => {
    expect(
      parentAttentionLabel({ messages: 2, requests: 1, total: 3 }, "Mia Kurz"),
    ).toBe(
      "2 neue Nachrichten und 1 offene Anfrage der Eltern von Mia Kurz öffnen",
    );
    expect(
      parentAttentionLabel({ messages: 0, requests: 2, total: 2 }, "Ben"),
    ).toBe("2 offene Anfragen der Eltern von Ben öffnen");
  });
});

describe("newParentMessages", () => {
  it("shows the last unread parent messages, never staff replies or new-request pills", () => {
    const thread = [
      message("1", "guardian"),
      message("2", "staff"),
      message("3", "guardian"),
      message("4", "system", {
        kind: "event",
        event_type: "request_created",
      }),
      message("5", "guardian"),
    ];

    expect(newParentMessages(thread, 2).map((m) => m.id)).toEqual(["3", "5"]);
    expect(newParentMessages(thread, 0)).toEqual([]);
  });
});
