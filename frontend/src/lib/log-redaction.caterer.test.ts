import { describe, expect, it } from "vitest";
import { redactSensitiveLogString } from "./log-redaction";

describe("caterer invitation redaction (ADR 0074)", () => {
  it("keeps the school but never the mailed token", () => {
    expect(
      redactSensitiveLogString(
        "GET /api/caterer/invitations/42/geheim/accept failed",
      ),
    ).not.toContain("geheim");
    expect(redactSensitiveLogString("/caterer/einladung/42/geheim")).toContain(
      "/einladung/42/",
    );
    expect(
      redactSensitiveLogString("/caterer/einladung/42/geheim"),
    ).not.toContain("geheim");
  });
});
