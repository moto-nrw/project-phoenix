import { describe, it, expect } from "vitest";
import {
  mapInvitationDelivery,
  presentDeliveryAttempt,
  presentLatestAttempt,
  type BackendInvitationDelivery,
  type InvitationDelivery,
  type InvitationDeliveryAttempt,
} from "./guardian-delivery-helpers";

function attempt(
  overrides: Partial<InvitationDeliveryAttempt> = {},
): InvitationDeliveryAttempt {
  return {
    outboxId: "1",
    dispatchStatus: "sent",
    deliveryStatus: "unknown",
    queuedAt: "2026-07-25T10:00:00Z",
    attempts: 1,
    events: [],
    ...overrides,
  };
}

describe("mapInvitationDelivery", () => {
  it("maps backend ids to strings and snake_case to camelCase", () => {
    const backend: BackendInvitationDelivery = {
      invitation_id: 42,
      attempts: [
        {
          outbox_id: 7,
          dispatch_status: "sent",
          delivery_status: "bounced",
          queued_at: "2026-07-25T10:00:00Z",
          sent_at: "2026-07-25T10:01:00Z",
          delivery_status_at: "2026-07-25T10:02:00Z",
          attempts: 2,
          last_error: "smtp timeout",
          delivery_detail: "5.1.1 User unknown",
          events: [
            {
              type: "bounced",
              occurred_at: "2026-07-25T10:02:00Z",
              reason_code: "5.1.1",
              reason: "User unknown",
            },
          ],
        },
      ],
    };

    const result = mapInvitationDelivery(backend);

    expect(result.invitationId).toBe("42");
    expect(result.attempts[0]?.outboxId).toBe("7");
    expect(result.attempts[0]?.deliveryStatus).toBe("bounced");
    expect(result.attempts[0]?.deliveryStatusAt).toBe("2026-07-25T10:02:00Z");
    expect(result.attempts[0]?.events[0]?.reasonCode).toBe("5.1.1");
  });

  it("tolerates a payload with no attempts or events", () => {
    const result = mapInvitationDelivery({ invitation_id: 1 });
    expect(result.attempts).toEqual([]);
  });
});

describe("presentDeliveryAttempt", () => {
  // The whole point of the feature: "handed to the mail server" must never be
  // presented as "delivered".
  it("does not claim delivery for a dispatched mail with no feedback", () => {
    const p = presentDeliveryAttempt(
      attempt({ dispatchStatus: "sent", deliveryStatus: "unknown" }),
    );
    expect(p.label).toBe("E-Mail übergeben");
    expect(p.label).not.toContain("Zugestellt");
    expect(p.retryInProgress).toBe(false);
  });

  it("reports a queued mail as eingeplant, not gesendet", () => {
    const p = presentDeliveryAttempt(attempt({ dispatchStatus: "pending" }));
    expect(p.label).toBe("Einladung eingeplant");
    expect(p.retryInProgress).toBe(true);
  });

  // A delay must block the resend button: a second invitation would not speed
  // anything up and confuses the family with two links.
  it("marks a deferred delivery as retry-in-progress and not a failure", () => {
    const p = presentDeliveryAttempt(
      attempt({ dispatchStatus: "sent", deliveryStatus: "deferred" }),
    );
    expect(p.label).toBe("Zustellung verzögert");
    expect(p.retryInProgress).toBe(true);
    expect(p.needsAddressFix).toBe(false);
    expect(p.hint).toContain("nicht erneut senden");
  });

  it.each([
    ["bounced", "Nicht zustellbar"],
    ["suppressed", "Blockiert"],
    ["complained", "Als Spam markiert"],
    ["soft_bounced", "Vorübergehend nicht zustellbar"],
  ] as const)("flags %s as needing an address fix", (status, label) => {
    const p = presentDeliveryAttempt(
      attempt({ dispatchStatus: "sent", deliveryStatus: status }),
    );
    expect(p.label).toBe(label);
    expect(p.needsAddressFix).toBe(true);
    expect(p.retryInProgress).toBe(false);
  });

  it("reports delivered", () => {
    const p = presentDeliveryAttempt(
      attempt({ dispatchStatus: "sent", deliveryStatus: "delivered" }),
    );
    expect(p.label).toBe("Zugestellt");
    expect(p.needsAddressFix).toBe(false);
  });

  // Dispatch state wins while the message has not left us: the receiving side
  // cannot have reported anything yet.
  it("prefers the dispatch failure over any delivery status", () => {
    const p = presentDeliveryAttempt(
      attempt({ dispatchStatus: "failed", deliveryStatus: "delivered" }),
    );
    expect(p.label).toBe("Versand fehlgeschlagen");
  });

  it("shows sending while the mail is being handed over", () => {
    const p = presentDeliveryAttempt(attempt({ dispatchStatus: "sending" }));
    expect(p.label).toBe("Wird gesendet");
    expect(p.retryInProgress).toBe(true);
  });
});

describe("presentLatestAttempt", () => {
  it("uses the newest attempt", () => {
    const delivery: InvitationDelivery = {
      invitationId: "1",
      attempts: [
        attempt({ outboxId: "2", deliveryStatus: "delivered" }),
        attempt({ outboxId: "1", deliveryStatus: "bounced" }),
      ],
    };
    expect(presentLatestAttempt(delivery)?.label).toBe("Zugestellt");
  });

  it("returns undefined when there is nothing to show", () => {
    expect(presentLatestAttempt(undefined)).toBeUndefined();
    expect(
      presentLatestAttempt({ invitationId: "1", attempts: [] }),
    ).toBeUndefined();
  });
});
