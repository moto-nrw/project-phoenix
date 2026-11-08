import { describe, expect, it } from "vitest";
import { isValidContactEmail, isValidContactPhone } from "./contact-format";

describe("contact formats", () => {
  it("accepts a plus-addressed email like the backend", () => {
    expect(isValidContactEmail("anna+schule@example.de")).toBe(true);
  });

  it("rejects an email without a domain suffix", () => {
    expect(isValidContactEmail("anna@example")).toBe(false);
  });

  it("accepts a phone with country code and spaces", () => {
    expect(isValidContactPhone("+49 170 1234567")).toBe(true);
  });

  it("rejects a phone with letters", () => {
    expect(isValidContactPhone("0170 12345ab")).toBe(false);
  });
});
