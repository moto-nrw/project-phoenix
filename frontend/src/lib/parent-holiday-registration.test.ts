import { describe, expect, it } from "vitest";
import {
  holidayRegistrationLoginPath,
  holidayRegistrationPath,
  holidayRegistrationReturnPath,
} from "./parent-holiday-registration";

describe("holiday registration paths", () => {
  it("builds the form path and the login path that leads back to it", () => {
    expect(holidayRegistrationPath("abc123")).toBe("/ferienanmeldung/abc123");
    expect(holidayRegistrationLoginPath("abc123")).toBe(
      "/login?next=%2Fferienanmeldung%2Fabc123",
    );
  });

  it("returns to exactly one holiday registration form after the login", () => {
    expect(holidayRegistrationReturnPath("/ferienanmeldung/abc_1-2")).toBe(
      "/ferienanmeldung/abc_1-2",
    );
  });

  it.each([
    null,
    "",
    "/",
    "/children",
    "https://example.org/ferienanmeldung/abc",
    "//example.org/ferienanmeldung/abc",
    "/ferienanmeldung/",
    "/ferienanmeldung/abc/../../settings",
    "/ferienanmeldung/abc?next=https://example.org",
    "/ferienanmeldung/abc#x",
    "/parents/ferienanmeldung/abc",
  ])("refuses %s as a target after the login", (next) => {
    expect(holidayRegistrationReturnPath(next)).toBeNull();
  });
});
