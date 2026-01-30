import { describe, it, expect } from "vitest";
import { getThemeColorsTailwind } from "./tailwind-theme";
import tailwindTheme from "./tailwind-theme";

describe("getThemeColorsTailwind", () => {
  it("returns an object with expected color categories", () => {
    const colors = getThemeColorsTailwind();

    expect(colors).toHaveProperty("primary");
    expect(colors).toHaveProperty("secondary");
    expect(colors).toHaveProperty("gray");
    expect(colors).toHaveProperty("success");
    expect(colors).toHaveProperty("warning");
    expect(colors).toHaveProperty("error");
    expect(colors).toHaveProperty("info");
  });

  it("returns primary colors with correct shades", () => {
    const colors = getThemeColorsTailwind();

    expect(colors.primary).toHaveProperty("500");
    expect(colors.primary).toHaveProperty("600");
    expect(colors.primary).toHaveProperty("800");

    expect(colors.primary![500]).toBe("#14b8a6");
    expect(colors.primary![600]).toBe("#0d9488");
    expect(colors.primary![800]).toBe("#115e59");
  });

  it("returns secondary colors with correct shades", () => {
    const colors = getThemeColorsTailwind();

    expect(colors.secondary).toHaveProperty("500");
    expect(colors.secondary).toHaveProperty("600");
    expect(colors.secondary).toHaveProperty("700");

    expect(colors.secondary![500]).toBe("#3b82f6");
    expect(colors.secondary![600]).toBe("#2563eb");
    expect(colors.secondary![700]).toBe("#1d4ed8");
  });

  it("returns gray colors with all expected shades", () => {
    const colors = getThemeColorsTailwind();

    expect(colors.gray).toHaveProperty("50");
    expect(colors.gray).toHaveProperty("100");
    expect(colors.gray).toHaveProperty("200");
    expect(colors.gray).toHaveProperty("300");
    expect(colors.gray).toHaveProperty("500");
    expect(colors.gray).toHaveProperty("600");
    expect(colors.gray).toHaveProperty("700");
    expect(colors.gray).toHaveProperty("800");
    expect(colors.gray).toHaveProperty("900");

    expect(colors.gray![50]).toBe("#f9fafb");
    expect(colors.gray![900]).toBe("#111827");
  });

  it("returns success colors", () => {
    const colors = getThemeColorsTailwind();

    expect(colors.success).toHaveProperty("50");
    expect(colors.success).toHaveProperty("100");
    expect(colors.success).toHaveProperty("700");
  });

  it("returns warning colors", () => {
    const colors = getThemeColorsTailwind();

    expect(colors.warning).toHaveProperty("50");
    expect(colors.warning).toHaveProperty("100");
    expect(colors.warning).toHaveProperty("700");
  });

  it("returns error colors", () => {
    const colors = getThemeColorsTailwind();

    expect(colors.error).toHaveProperty("50");
    expect(colors.error).toHaveProperty("100");
    expect(colors.error).toHaveProperty("600");
    expect(colors.error).toHaveProperty("700");
  });

  it("returns info colors", () => {
    const colors = getThemeColorsTailwind();

    expect(colors.info).toHaveProperty("50");
    expect(colors.info).toHaveProperty("100");
    expect(colors.info).toHaveProperty("700");
  });
});

describe("tailwindTheme default export", () => {
  it("exports an object with getThemeColorsTailwind function", () => {
    expect(tailwindTheme).toHaveProperty("getThemeColorsTailwind");
    expect(typeof tailwindTheme.getThemeColorsTailwind).toBe("function");
  });

  it("exported function returns same result as named export", () => {
    const result1 = getThemeColorsTailwind();
    const result2 = tailwindTheme.getThemeColorsTailwind();

    expect(result1).toEqual(result2);
  });
});
