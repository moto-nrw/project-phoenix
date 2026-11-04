import { afterEach, describe, expect, it, vi } from "vitest";
import { displayEmail } from "./demo-build";

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("displayEmail", () => {
  // The visitor's caregiver and parent keep the seed address of the person
  // whose name they took, so the demo shows no seed address at all (#5033).
  it("hides the seed addresses of a demo school in the demo build", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_ENV", "demo");
    expect(displayEmail("sabine.schneider@demo-ogs-nord.moto-ogs.de")).toBe("");
    expect(displayEmail("julia.klein@demo-ogs-nord-2.moto-ogs.de")).toBe("");
    expect(displayEmail("anna@example.com")).toBe("anna@example.com");
    expect(displayEmail(undefined)).toBe("");
  });

  it("shows every address outside the demo build", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_ENV", "production");
    expect(displayEmail("sabine.schneider@demo-ogs-nord.moto-ogs.de")).toBe(
      "sabine.schneider@demo-ogs-nord.moto-ogs.de",
    );
  });
});
