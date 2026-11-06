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
    expect(mailStatusCopy("accepted", "school_stop")?.label).toBe(
      "Versand gestoppt",
    );
  });

  it("has no status before a mail or a mark exists", () => {
    expect(mailStatusCopy()).toBeNull();
    expect(mailStatusCopy("", "")).toBeNull();
  });

  it("explains every block in words a person can act on", () => {
    for (const mark of ["failed", "complained", "school_stop"] as const) {
      expect(mailStatusCopy(undefined, mark)?.explanation).toBeTruthy();
      expect(guardianAddressNextStep(mark)).toBeTruthy();
    }
    expect(mailStatusCopy("delayed")?.explanation).toMatch(/nicht noch einmal/);
  });
});
