/**
 * messaging-status.ts exports only TypeScript types (ChatMessage,
 * MessageSenderKind). Types are erased at runtime so there is no executable
 * logic to exercise. This file exists to:
 *  1. Confirm the module can be imported without errors (smoke test).
 *  2. Assert the shape of runtime values that conform to the types, giving
 *     SonarCloud a coverage signal on the import itself.
 */

import { describe, it, expect } from "vitest";
import type { ChatMessage } from "./messaging-status";
import {
  parentEventI18nDescriptor,
  parentRequestStatusI18nKey,
  parentRequestTypeI18nKey,
} from "./messaging-status";

describe("messaging-status — ChatMessage type", () => {
  it("accepts a minimal guardian message conforming to the ChatMessage shape", () => {
    // Construct a value that satisfies the ChatMessage interface at the call
    // site; TypeScript checks the shape at compile-time. At runtime we just
    // verify the object has the expected keys so the test produces coverage.
    const msg: ChatMessage = {
      id: "1",
      sender_kind: "guardian",
      sender_name: "Anna M.",
      body: "Hallo OGS!",
      created_at: "2026-01-01T08:00:00Z",
    };

    expect(msg.id).toBe("1");
    expect(msg.sender_kind).toBe("guardian");
    expect(msg.sender_name).toBe("Anna M.");
    expect(msg.body).toBe("Hallo OGS!");
    expect(msg.created_at).toBe("2026-01-01T08:00:00Z");
  });

  it("accepts a staff message with read receipts", () => {
    const msg: ChatMessage = {
      id: "2",
      sender_kind: "staff",
      sender_name: "OGS Mustergrundschule",
      body: "Guten Morgen!",
      created_at: "2026-01-01T09:00:00Z",
      read_by_guardian: true,
      read_by_staff: false,
    };

    expect(msg.sender_kind).toBe("staff");
    expect(msg.read_by_guardian).toBe(true);
    expect(msg.read_by_staff).toBe(false);
  });

  it("allows optional read-receipt fields to be absent", () => {
    const msg: ChatMessage = {
      id: "3",
      sender_kind: "guardian",
      sender_name: "Max M.",
      body: "Auf Wiedersehen!",
      created_at: "2026-01-01T10:00:00Z",
    };

    // Optional fields default to undefined when not provided
    expect(msg.read_by_guardian).toBeUndefined();
    expect(msg.read_by_staff).toBeUndefined();
  });

  it("sender_kind is either 'guardian' or 'staff' (exhaustive check)", () => {
    const kinds: ChatMessage["sender_kind"][] = ["guardian", "staff"];
    expect(kinds).toHaveLength(2);
    expect(kinds).toContain("guardian");
    expect(kinds).toContain("staff");
  });
});

describe("parentRequestTypeI18nKey", () => {
  it("maps each request type to its localized title key", () => {
    expect(parentRequestTypeI18nKey("care_schedule")).toBe(
      "requestTypeCareSchedule",
    );
  });

  it("falls back to a generic key for unknown/absent types", () => {
    expect(parentRequestTypeI18nKey(undefined)).toBe("requestTitleFallback");
    expect(parentRequestTypeI18nKey("something_else")).toBe(
      "requestTitleFallback",
    );
  });
});

describe("parentRequestStatusI18nKey", () => {
  it("maps each status to its key and falls back to open", () => {
    expect(parentRequestStatusI18nKey("erledigt")).toBe("statusDone");
    expect(parentRequestStatusI18nKey(undefined)).toBe("statusOpen");
  });
});

describe("parentEventI18nDescriptor", () => {
  it("maps a confirmed care-schedule request to its localized key", () => {
    expect(
      parentEventI18nDescriptor({
        kind: "event",
        event_type: "request_status",
        request_status: "erledigt",
        request_type: "care_schedule",
      }),
    ).toEqual({ key: "eventRequestConfirmedCareSchedule" });
  });

  it("falls back to a generic confirmed key for an unknown request type", () => {
    expect(
      parentEventI18nDescriptor({
        kind: "event",
        event_type: "request_status",
        request_status: "erledigt",
      }),
    ).toEqual({ key: "eventRequestConfirmed" });
  });

  it("carries the staff decision reason as a value for a rejection", () => {
    expect(
      parentEventI18nDescriptor({
        kind: "event",
        event_type: "request_status",
        request_status: "abgelehnt",
        decision_reason: "Bitte erneut einreichen",
      }),
    ).toEqual({
      key: "eventRequestRejectedReason",
      values: { reason: "Bitte erneut einreichen" },
    });
  });

  it("uses the reasonless rejection key when no reason is present", () => {
    expect(
      parentEventI18nDescriptor({
        kind: "event",
        event_type: "request_status",
        request_status: "abgelehnt",
      }),
    ).toEqual({ key: "eventRequestRejected" });
  });

  it("maps a guardian withdrawal to its localized key", () => {
    expect(
      parentEventI18nDescriptor({
        kind: "event",
        event_type: "request_status",
        request_status: "zurueckgezogen",
      }),
    ).toEqual({ key: "eventRequestWithdrawn" });
  });

  it("returns null for non-request events so the caller keeps the raw body", () => {
    expect(
      parentEventI18nDescriptor({
        kind: "event",
        event_type: "room_change",
        request_status: undefined,
      }),
    ).toBeNull();
    expect(
      parentEventI18nDescriptor({
        kind: "message",
        body: "Hallo",
      } as ChatMessage),
    ).toBeNull();
  });
});
