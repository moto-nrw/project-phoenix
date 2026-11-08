import { describe, expect, it } from "vitest";

import { contactListValueMissing } from "./enrollment-form-contact-inputs";

describe("contactListValueMissing", () => {
  it("treats malformed contact phone and email values as missing", () => {
    const malformedContacts = [
      {
        first_name: "Mara",
        last_name: "Muster",
        email: 42,
        phone_numbers: [null, {}, { phone_number: 7 }],
      },
    ];

    expect(() => contactListValueMissing(malformedContacts)).not.toThrow();
    expect(contactListValueMissing(malformedContacts)).toBe(true);
  });
});
