import { describe, expect, it } from "vitest";
import {
  PASSWORD_REQUIREMENTS,
  getPasswordRequirementStatus,
  validatePassword,
} from "./password-validation";

describe("password-validation", () => {
  describe("PASSWORD_REQUIREMENTS", () => {
    it("has 5 requirements", () => {
      expect(PASSWORD_REQUIREMENTS).toHaveLength(5);
    });

    describe("minimum length requirement", () => {
      const requirement = PASSWORD_REQUIREMENTS[0];

      it("has correct label", () => {
        expect(requirement.label).toBe("Mindestens 8 Zeichen");
      });

      it("passes for 8 character password", () => {
        expect(requirement.test("12345678")).toBe(true);
      });

      it("passes for longer password", () => {
        expect(requirement.test("123456789")).toBe(true);
      });

      it("fails for 7 character password", () => {
        expect(requirement.test("1234567")).toBe(false);
      });

      it("fails for empty password", () => {
        expect(requirement.test("")).toBe(false);
      });
    });

    describe("uppercase letter requirement", () => {
      const requirement = PASSWORD_REQUIREMENTS[1];

      it("has correct label", () => {
        expect(requirement.label).toBe("Ein Großbuchstabe");
      });

      it("passes for password with uppercase", () => {
        expect(requirement.test("Password")).toBe(true);
      });

      it("passes for password with multiple uppercase", () => {
        expect(requirement.test("PASSWORD")).toBe(true);
      });

      it("fails for all lowercase password", () => {
        expect(requirement.test("password")).toBe(false);
      });

      it("fails for numeric password", () => {
        expect(requirement.test("12345678")).toBe(false);
      });
    });

    describe("lowercase letter requirement", () => {
      const requirement = PASSWORD_REQUIREMENTS[2];

      it("has correct label", () => {
        expect(requirement.label).toBe("Ein Kleinbuchstabe");
      });

      it("passes for password with lowercase", () => {
        expect(requirement.test("PASSWORD1a")).toBe(true);
      });

      it("passes for all lowercase password", () => {
        expect(requirement.test("password")).toBe(true);
      });

      it("fails for all uppercase password", () => {
        expect(requirement.test("PASSWORD")).toBe(false);
      });

      it("fails for numeric password", () => {
        expect(requirement.test("12345678")).toBe(false);
      });
    });

    describe("digit requirement", () => {
      const requirement = PASSWORD_REQUIREMENTS[3];

      it("has correct label", () => {
        expect(requirement.label).toBe("Eine Zahl");
      });

      it("passes for password with digit", () => {
        expect(requirement.test("Password1")).toBe(true);
      });

      it("passes for numeric password", () => {
        expect(requirement.test("12345678")).toBe(true);
      });

      it("fails for alphabetic password", () => {
        expect(requirement.test("Password")).toBe(false);
      });

      it("fails for password with special chars only", () => {
        expect(requirement.test("!@#$%^&*")).toBe(false);
      });
    });

    describe("special character requirement", () => {
      const requirement = PASSWORD_REQUIREMENTS[4];

      it("has correct label", () => {
        expect(requirement.label).toBe("Ein Sonderzeichen");
      });

      it("passes for password with special char", () => {
        expect(requirement.test("Password1!")).toBe(true);
      });

      it("passes for various special characters", () => {
        expect(requirement.test("@")).toBe(true);
        expect(requirement.test("#")).toBe(true);
        expect(requirement.test("$")).toBe(true);
        expect(requirement.test("%")).toBe(true);
        expect(requirement.test("^")).toBe(true);
        expect(requirement.test("&")).toBe(true);
        expect(requirement.test("*")).toBe(true);
        expect(requirement.test("_")).toBe(true);
        expect(requirement.test("-")).toBe(true);
        expect(requirement.test(" ")).toBe(true);
      });

      it("fails for alphanumeric password", () => {
        expect(requirement.test("Password1")).toBe(false);
      });

      it("fails for alphabetic password", () => {
        expect(requirement.test("Password")).toBe(false);
      });
    });
  });

  describe("validatePassword", () => {
    it("returns true for valid password meeting all requirements", () => {
      expect(validatePassword("Password1!")).toBe(true);
    });

    it("returns true for another valid password", () => {
      expect(validatePassword("Test1234%")).toBe(true);
    });

    it("returns true for complex valid password", () => {
      expect(validatePassword("MySecure@Pass123")).toBe(true);
    });

    it("returns false when missing uppercase", () => {
      expect(validatePassword("password1!")).toBe(false);
    });

    it("returns false when missing lowercase", () => {
      expect(validatePassword("PASSWORD1!")).toBe(false);
    });

    it("returns false when missing digit", () => {
      expect(validatePassword("Password!")).toBe(false);
    });

    it("returns false when missing special character", () => {
      expect(validatePassword("Password1")).toBe(false);
    });

    it("returns false when too short", () => {
      expect(validatePassword("Pass1!")).toBe(false);
    });

    it("returns false for empty password", () => {
      expect(validatePassword("")).toBe(false);
    });

    it("returns false when missing multiple requirements", () => {
      expect(validatePassword("pass")).toBe(false);
    });
  });

  describe("getPasswordRequirementStatus", () => {
    it("returns status for all requirements", () => {
      const status = getPasswordRequirementStatus("Password1!");
      expect(status).toHaveLength(5);
    });

    it("returns all met for valid password", () => {
      const status = getPasswordRequirementStatus("Password1!");
      expect(status.every((s) => s.met)).toBe(true);
    });

    it("returns correct labels", () => {
      const status = getPasswordRequirementStatus("");
      expect(status.map((s) => s.label)).toEqual([
        "Mindestens 8 Zeichen",
        "Ein Großbuchstabe",
        "Ein Kleinbuchstabe",
        "Eine Zahl",
        "Ein Sonderzeichen",
      ]);
    });

    it("shows only length requirement met for 8-char simple password", () => {
      const status = getPasswordRequirementStatus("--------");
      expect(status[0]).toEqual({ label: "Mindestens 8 Zeichen", met: true });
      expect(status[1]).toEqual({ label: "Ein Großbuchstabe", met: false });
      expect(status[2]).toEqual({ label: "Ein Kleinbuchstabe", met: false });
      expect(status[3]).toEqual({ label: "Eine Zahl", met: false });
      expect(status[4]).toEqual({ label: "Ein Sonderzeichen", met: true });
    });

    it("shows only uppercase met for single uppercase letter", () => {
      const status = getPasswordRequirementStatus("A");
      expect(status[0].met).toBe(false); // length
      expect(status[1].met).toBe(true); // uppercase
      expect(status[2].met).toBe(false); // lowercase
      expect(status[3].met).toBe(false); // digit
      expect(status[4].met).toBe(false); // special
    });

    it("shows only lowercase met for single lowercase letter", () => {
      const status = getPasswordRequirementStatus("a");
      expect(status[0].met).toBe(false); // length
      expect(status[1].met).toBe(false); // uppercase
      expect(status[2].met).toBe(true); // lowercase
      expect(status[3].met).toBe(false); // digit
      expect(status[4].met).toBe(false); // special
    });

    it("shows only digit met for single digit", () => {
      const status = getPasswordRequirementStatus("1");
      expect(status[0].met).toBe(false); // length
      expect(status[1].met).toBe(false); // uppercase
      expect(status[2].met).toBe(false); // lowercase
      expect(status[3].met).toBe(true); // digit
      expect(status[4].met).toBe(false); // special
    });

    it("shows only special met for single special char", () => {
      const status = getPasswordRequirementStatus("!");
      expect(status[0].met).toBe(false); // length
      expect(status[1].met).toBe(false); // uppercase
      expect(status[2].met).toBe(false); // lowercase
      expect(status[3].met).toBe(false); // digit
      expect(status[4].met).toBe(true); // special
    });

    it("returns all not met for empty password", () => {
      const status = getPasswordRequirementStatus("");
      expect(status.every((s) => !s.met)).toBe(true);
    });

    it("progressively shows requirements met as password improves", () => {
      // Start with just length
      let status = getPasswordRequirementStatus("________");
      expect(status[0].met).toBe(true);
      expect(status.filter((s) => s.met)).toHaveLength(2); // length + special

      // Add uppercase
      status = getPasswordRequirementStatus("_______A");
      expect(status[0].met).toBe(true); // length
      expect(status[1].met).toBe(true); // uppercase
      expect(status.filter((s) => s.met)).toHaveLength(3);

      // Add lowercase
      status = getPasswordRequirementStatus("______Aa");
      expect(status[2].met).toBe(true); // lowercase
      expect(status.filter((s) => s.met)).toHaveLength(4);

      // Add digit - now all met
      status = getPasswordRequirementStatus("_____Aa1");
      expect(status[3].met).toBe(true); // digit
      expect(status.every((s) => s.met)).toBe(true);
    });
  });
});
