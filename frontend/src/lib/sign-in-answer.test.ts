import { describe, expect, it } from "vitest";
import { ApiError } from "./api-error";
import { PORTAL_KEYS, type PortalKey } from "./portal-paths";
import { signInAnswer } from "./sign-in-answer";

const code = {
  status: "mfa_required",
  challenge_token: "challenge",
  masked_email: "a***@b.de",
};

describe("signInAnswer", () => {
  it("reads the code step the same way for every portal", () => {
    for (const portal of PORTAL_KEYS) {
      expect(
        signInAnswer(portal, {
          ...code,
          trusted_device_enabled: true,
          trusted_device_days: 30,
        }),
      ).toEqual({
        status: "mfa_required",
        challengeToken: "challenge",
        maskedEmail: "a***@b.de",
        organizationId: "",
        trustedDeviceEnabled: true,
        trustedDeviceDays: 30,
      });
      expect(
        signInAnswer(portal, { ...code, trusted_device_enabled: false }),
      ).toMatchObject({ trustedDeviceEnabled: false });
    }
  });

  // Tenant, school and operator miss the field only from an older backend;
  // the other portals offer nothing to remember without it.
  it.each<[PortalKey, boolean]>([
    ["tenant", true],
    ["school", true],
    ["operator", true],
    ["caterer", false],
    ["parent", false],
    ["traeger", false],
  ])(
    "takes a missing remember-device field of %s as %s",
    (portal, remembered) => {
      const answer = signInAnswer(portal, code);
      expect(answer).toMatchObject({
        status: "mfa_required",
        trustedDeviceEnabled: remembered,
      });
      expect(answer).not.toHaveProperty("trustedDeviceDays", expect.anything());
    },
  );

  it("reads the setup step with its enrollment token", () => {
    expect(
      signInAnswer("tenant", {
        status: "mfa_enrollment_required",
        access_token: "enrollment",
        masked_email: "a***@b.de",
        mfa_enrollment_required: true,
      }),
    ).toEqual({
      status: "mfa_enrollment_required",
      enrollmentToken: "enrollment",
      maskedEmail: "a***@b.de",
      organizationId: "",
    });
  });

  it("names the Träger of a Träger-Portal answer and offers the choice", () => {
    expect(signInAnswer("traeger", { ...code, organization_id: 7 })).toMatchObject(
      { organizationId: "7" },
    );
    expect(
      signInAnswer("traeger", {
        status: "organization_selection_required",
        organizations: [{ id: 7, name: "Träger Nord" }],
      }),
    ).toEqual({
      status: "organization_selection_required",
      organizations: [{ id: "7", name: "Träger Nord" }],
    });
  });

  it("takes an answer without a further step as signed in", () => {
    for (const portal of PORTAL_KEYS) {
      expect(signInAnswer(portal, { status: "authenticated" })).toEqual({
        status: "authenticated",
      });
    }
    expect(signInAnswer("caterer", {})).toEqual({ status: "authenticated" });
    expect(signInAnswer("tenant", null)).toEqual({ status: "authenticated" });
  });

  // Only the Träger-Portal refuses what it cannot read; the other portals'
  // pages go on to their session check.
  it("refuses an unreadable Träger-Portal answer", () => {
    for (const body of [
      {},
      { status: "mfa_required" },
      { status: "mfa_enrollment_required" },
      { status: "something_else" },
    ]) {
      expect(() => signInAnswer("traeger", body)).toThrow(ApiError);
    }
    expect(signInAnswer("parent", { status: "mfa_required" })).toMatchObject({
      status: "mfa_required",
      challengeToken: "",
    });
    expect(
      signInAnswer("school", { status: "mfa_enrollment_required" }),
    ).toMatchObject({ status: "mfa_enrollment_required", enrollmentToken: "" });
  });
});
