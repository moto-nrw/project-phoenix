import { describe, expect, it } from "vitest";

import { parentAttentionLabel } from "~/components/students/parent-attention-badge";
import { parentMessagesInSnapshot } from "~/components/students/parent-attention-dialog";
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

describe("parentMessagesInSnapshot", () => {
  it("shows the complete parent snapshot, never staff replies or new-request pills", () => {
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

    expect(parentMessagesInSnapshot(thread).map((m) => m.id)).toEqual([
      "1",
      "3",
      "5",
    ]);
    expect(parentMessagesInSnapshot([])).toEqual([]);
  });

  it("attributes system events to their actor and excludes new requests on either side", () => {
    const thread = [
      message("parent-event", "system", {
        event_type: "request_status",
        event_actor_kind: "guardian",
      }),
      message("staff-event", "system", {
        event_type: "request_status",
        event_actor_kind: "staff",
      }),
      message("unknown-event", "system", { event_type: "request_status" }),
      message("new-request", "system", {
        event_type: "request_created",
        event_actor_kind: "guardian",
      }),
      message("parent-request", "guardian", { event_type: "request_created" }),
    ];
    expect(parentMessagesInSnapshot(thread).map((m) => m.id)).toEqual([
      "parent-event",
    ]);
  });
});
