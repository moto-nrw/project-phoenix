import { describe, expect, it } from "vitest";

import {
  guardianAddressNextStep,
  mailStatusCopy,
} from "./email-delivery-status";

describe("mailStatusCopy", () => {
  it("never calls anything but a reported delivery delivered", () => {
    for (const outcome of [
      "queued",
      "accepted",
      "delayed",
      "undeliverable",
      "blocked",
      "complained",
      "send_failed",
      "unknown",
      "cancelled",
    ] as const) {
      expect(mailStatusCopy(outcome)?.label).not.toBe("Zugestellt");
    }
    expect(mailStatusCopy("delivered")?.label).toBe("Zugestellt");
  });

  it("lets the mark of an address outrank the mail", () => {
    expect(mailStatusCopy("delivered", "failed")?.label).toBe("Adresse prüfen");
    expect(mailStatusCopy("accepted", "", true)?.label).toBe(
      "Versand gestoppt",
    );
  });

  it("has no status before a mail or a mark exists", () => {
    expect(mailStatusCopy()).toBeNull();
    expect(mailStatusCopy("", "")).toBeNull();
  });

  it("explains every block in words a person can act on", () => {
    for (const mark of ["failed", "complained"] as const) {
      expect(mailStatusCopy(undefined, mark)?.explanation).toBeTruthy();
      expect(guardianAddressNextStep(mark)).toBeTruthy();
    }
    expect(mailStatusCopy("delayed")?.explanation).toMatch(/nicht noch einmal/);
  });

  it("keeps a stop and a marked address apart", () => {
    expect(mailStatusCopy("accepted", "failed", true)?.label).toBe(
      "Adresse prüfen",
    );
    expect(mailStatusCopy(undefined, undefined, true)?.label).not.toBe(
      "Adresse prüfen",
    );
  });

  it("does not blame the address for a mail that failed for another reason", () => {
    expect(mailStatusCopy("undeliverable")?.explanation).not.toMatch(/Adresse/);
  });

  it("does not claim a block was lifted", () => {
    expect(mailStatusCopy("blocked")?.explanation).not.toMatch(/aufgehoben/);
  });

  it("names an unsubscribe as one, not as a failed mail", () => {
    const copy = mailStatusCopy("unsubscribed");
    expect(copy?.label).toBe("Abgemeldet");
    expect(copy?.explanation).not.toMatch(/kommt nicht an|kam nicht an/);
  });

  it("offers a new mail after a delay ran past its limit", () => {
    expect(mailStatusCopy("overdue")?.explanation).toMatch(
      /noch einmal senden/,
    );
  });
});
