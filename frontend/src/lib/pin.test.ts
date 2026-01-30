import { describe, it, expect } from "vitest";
import { isPIN, asPIN, validatePinOrThrow } from "./pin";
import type { PIN } from "./pin";

describe("pin", () => {
  describe("isPIN", () => {
    it("should return true for valid 4-digit PINs", () => {
      expect(isPIN("0000")).toBe(true);
      expect(isPIN("1234")).toBe(true);
      expect(isPIN("9999")).toBe(true);
      expect(isPIN("5678")).toBe(true);
    });

    it("should return false for empty string", () => {
      expect(isPIN("")).toBe(false);
    });

    it("should return false for strings with less than 4 digits", () => {
      expect(isPIN("1")).toBe(false);
      expect(isPIN("12")).toBe(false);
      expect(isPIN("123")).toBe(false);
    });

    it("should return false for strings with more than 4 digits", () => {
      expect(isPIN("12345")).toBe(false);
      expect(isPIN("123456")).toBe(false);
    });

    it("should return false for strings containing non-digits", () => {
      expect(isPIN("abcd")).toBe(false);
      expect(isPIN("12ab")).toBe(false);
      expect(isPIN("ab12")).toBe(false);
      expect(isPIN("1 23")).toBe(false);
      expect(isPIN("12-3")).toBe(false);
    });

    it("should return false for strings with special characters", () => {
      expect(isPIN("12!4")).toBe(false);
      expect(isPIN("12.4")).toBe(false);
      expect(isPIN("12,4")).toBe(false);
    });
  });

  describe("asPIN", () => {
    it("should return PIN for valid 4-digit strings", () => {
      const pin1: PIN = asPIN("0000");
      expect(pin1).toBe("0000");

      const pin2: PIN = asPIN("1234");
      expect(pin2).toBe("1234");

      const pin3: PIN = asPIN("9999");
      expect(pin3).toBe("9999");
    });

    it("should throw error for empty string", () => {
      expect(() => asPIN("")).toThrow("PIN muss aus genau 4 Ziffern bestehen");
    });

    it("should throw error for strings with less than 4 digits", () => {
      expect(() => asPIN("1")).toThrow("PIN muss aus genau 4 Ziffern bestehen");
      expect(() => asPIN("12")).toThrow(
        "PIN muss aus genau 4 Ziffern bestehen",
      );
      expect(() => asPIN("123")).toThrow(
        "PIN muss aus genau 4 Ziffern bestehen",
      );
    });

    it("should throw error for strings with more than 4 digits", () => {
      expect(() => asPIN("12345")).toThrow(
        "PIN muss aus genau 4 Ziffern bestehen",
      );
      expect(() => asPIN("123456")).toThrow(
        "PIN muss aus genau 4 Ziffern bestehen",
      );
    });

    it("should throw error for strings containing non-digits", () => {
      expect(() => asPIN("abcd")).toThrow(
        "PIN muss aus genau 4 Ziffern bestehen",
      );
      expect(() => asPIN("12ab")).toThrow(
        "PIN muss aus genau 4 Ziffern bestehen",
      );
      expect(() => asPIN("ab12")).toThrow(
        "PIN muss aus genau 4 Ziffern bestehen",
      );
    });

    it("should throw error for strings with special characters", () => {
      expect(() => asPIN("12!4")).toThrow(
        "PIN muss aus genau 4 Ziffern bestehen",
      );
      expect(() => asPIN("12.4")).toThrow(
        "PIN muss aus genau 4 Ziffern bestehen",
      );
    });
  });

  describe("validatePinOrThrow", () => {
    it("should return PIN for valid 4-digit strings", () => {
      const pin1: PIN = validatePinOrThrow("0000");
      expect(pin1).toBe("0000");

      const pin2: PIN = validatePinOrThrow("1234");
      expect(pin2).toBe("1234");

      const pin3: PIN = validatePinOrThrow("9999");
      expect(pin3).toBe("9999");
    });

    it("should throw error for invalid PINs", () => {
      expect(() => validatePinOrThrow("")).toThrow(
        "PIN muss aus genau 4 Ziffern bestehen",
      );
      expect(() => validatePinOrThrow("123")).toThrow(
        "PIN muss aus genau 4 Ziffern bestehen",
      );
      expect(() => validatePinOrThrow("12345")).toThrow(
        "PIN muss aus genau 4 Ziffern bestehen",
      );
      expect(() => validatePinOrThrow("abcd")).toThrow(
        "PIN muss aus genau 4 Ziffern bestehen",
      );
    });

    it("should delegate to asPIN function", () => {
      // Testing that validatePinOrThrow and asPIN have identical behavior
      const validCases = ["0000", "1234", "9999"];
      const invalidCases = ["", "123", "12345", "abcd"];

      validCases.forEach((value) => {
        expect(validatePinOrThrow(value)).toBe(asPIN(value));
      });

      invalidCases.forEach((value) => {
        expect(() => validatePinOrThrow(value)).toThrow();
        expect(() => asPIN(value)).toThrow();
      });
    });
  });
});
