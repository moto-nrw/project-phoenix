import { describe, expect, it } from "vitest";
import { sanitizeEndpoint } from "./log-sanitize";

describe("sanitizeEndpoint", () => {
  it("removes the query string entirely (issue #2105)", () => {
    expect(
      sanitizeEndpoint("/api/students?search=Mustermann&first_name=Erika"),
    ).toBe("/api/students");
    expect(
      sanitizeEndpoint(
        "http://api.test/api/students?email=erika%40example.com",
      ),
    ).toBe("http://api.test/api/students");
  });

  it("masks numeric IDs, UUIDs, and token-like segments", () => {
    expect(sanitizeEndpoint("/api/students/12345/visits")).toBe(
      "/api/students/{id}/visits",
    );
    expect(
      sanitizeEndpoint("/api/x/0d1f3c62-9a7b-4c1d-8e2f-aa55bb66cc77"),
    ).toBe("/api/x/{uuid}");
    expect(sanitizeEndpoint("/invite/AbCdEfGhIjKlMnOp123")).toBe(
      "/invite/{token}",
    );
  });

  it("leaves plain paths untouched", () => {
    expect(sanitizeEndpoint("/api/staff")).toBe("/api/staff");
    expect(sanitizeEndpoint("")).toBe("");
  });
});
