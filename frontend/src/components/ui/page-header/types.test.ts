/**
 * Tests for Types and Utility Functions
 * Tests the normalizeFilterValues utility function
 */
import { describe, it, expect } from "vitest";
import { normalizeFilterValues } from "./types";

describe("normalizeFilterValues", () => {
  it("returns array as-is when value is already an array", () => {
    const input = ["value1", "value2", "value3"];
    const result = normalizeFilterValues(input);

    expect(result).toEqual(["value1", "value2", "value3"]);
    expect(result).toBe(input); // Same reference
  });

  it("wraps string value in an array", () => {
    const result = normalizeFilterValues("single-value");

    expect(result).toEqual(["single-value"]);
  });

  it("returns empty array when value is undefined", () => {
    const result = normalizeFilterValues(undefined);

    expect(result).toEqual([]);
  });

  it("returns empty array when value is empty string", () => {
    const result = normalizeFilterValues("");

    expect(result).toEqual([]);
  });

  it("handles empty array", () => {
    const result = normalizeFilterValues([]);

    expect(result).toEqual([]);
  });

  it("preserves array with single element", () => {
    const result = normalizeFilterValues(["only-one"]);

    expect(result).toEqual(["only-one"]);
  });

  it("handles numeric string values", () => {
    const result = normalizeFilterValues("123");

    expect(result).toEqual(["123"]);
  });

  it("handles arrays with numeric strings", () => {
    const result = normalizeFilterValues(["1", "2", "3"]);

    expect(result).toEqual(["1", "2", "3"]);
  });

  it("handles special characters in string values", () => {
    const result = normalizeFilterValues("value-with-dash_and_underscore");

    expect(result).toEqual(["value-with-dash_and_underscore"]);
  });

  it("handles whitespace in string values", () => {
    const result = normalizeFilterValues("value with spaces");

    expect(result).toEqual(["value with spaces"]);
  });
});
