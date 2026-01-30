import { describe, it, expect } from "vitest";
import { theme } from "./theme";
import defaultTheme from "./theme";

describe("theme object structure", () => {
  it("has all expected top-level keys", () => {
    expect(theme).toHaveProperty("colors");
    expect(theme).toHaveProperty("typography");
    expect(theme).toHaveProperty("spacing");
    expect(theme).toHaveProperty("borderRadius");
    expect(theme).toHaveProperty("boxShadow");
    expect(theme).toHaveProperty("transition");
    expect(theme).toHaveProperty("breakpoints");
    expect(theme).toHaveProperty("zIndex");
  });
});

describe("theme.colors", () => {
  it("has all expected color categories", () => {
    expect(theme.colors).toHaveProperty("primary");
    expect(theme.colors).toHaveProperty("secondary");
    expect(theme.colors).toHaveProperty("gray");
    expect(theme.colors).toHaveProperty("success");
    expect(theme.colors).toHaveProperty("warning");
    expect(theme.colors).toHaveProperty("error");
    expect(theme.colors).toHaveProperty("info");
    expect(theme.colors).toHaveProperty("background");
    expect(theme.colors).toHaveProperty("text");
  });

  it("has correct primary color values", () => {
    expect(theme.colors.primary[500]).toBe("#14b8a6");
    expect(theme.colors.primary[600]).toBe("#0d9488");
    expect(theme.colors.primary[800]).toBe("#115e59");
  });

  it("has correct secondary color values", () => {
    expect(theme.colors.secondary[500]).toBe("#3b82f6");
    expect(theme.colors.secondary[600]).toBe("#2563eb");
    expect(theme.colors.secondary[700]).toBe("#1d4ed8");
  });

  it("has all gray shades", () => {
    expect(theme.colors.gray[50]).toBe("#f9fafb");
    expect(theme.colors.gray[100]).toBe("#f3f4f6");
    expect(theme.colors.gray[200]).toBe("#e5e7eb");
    expect(theme.colors.gray[300]).toBe("#d1d5db");
    expect(theme.colors.gray[500]).toBe("#6b7280");
    expect(theme.colors.gray[600]).toBe("#4b5563");
    expect(theme.colors.gray[700]).toBe("#374151");
    expect(theme.colors.gray[800]).toBe("#1f2937");
    expect(theme.colors.gray[900]).toBe("#111827");
  });

  it("has success colors", () => {
    expect(theme.colors.success[50]).toBe("#ecfdf5");
    expect(theme.colors.success[100]).toBe("#d1fae5");
    expect(theme.colors.success[700]).toBe("#047857");
  });

  it("has warning colors", () => {
    expect(theme.colors.warning[50]).toBe("#fffbeb");
    expect(theme.colors.warning[100]).toBe("#fef3c7");
    expect(theme.colors.warning[700]).toBe("#b45309");
  });

  it("has error colors", () => {
    expect(theme.colors.error[50]).toBe("#fef2f2");
    expect(theme.colors.error[100]).toBe("#fee2e2");
    expect(theme.colors.error[600]).toBe("#dc2626");
    expect(theme.colors.error[700]).toBe("#b91c1c");
  });

  it("has info colors", () => {
    expect(theme.colors.info[50]).toBe("#eff6ff");
    expect(theme.colors.info[100]).toBe("#dbeafe");
    expect(theme.colors.info[700]).toBe("#1d4ed8");
  });

  it("has background colors", () => {
    expect(theme.colors.background.canvas).toBe("#ffffff");
    expect(theme.colors.background.card).toBe("rgba(255, 255, 255, 0.95)");
    expect(theme.colors.background.wrapper).toBe("rgba(255, 255, 255, 0.2)");
    expect(Array.isArray(theme.colors.background.gradientColors)).toBe(true);
    expect(theme.colors.background.gradientColors).toHaveLength(5);
  });

  it("has text colors", () => {
    expect(theme.colors.text.primary).toBe("#1f2937");
    expect(theme.colors.text.secondary).toBe("#4b5563");
    expect(theme.colors.text.disabled).toBe("#9ca3af");
    expect(theme.colors.text.onPrimary).toBe("#ffffff");
  });
});

describe("theme.typography", () => {
  it("has font family configuration", () => {
    expect(theme.typography.fontFamily).toHaveProperty("primary");
    expect(theme.typography.fontFamily.primary).toContain("Geist Sans");
  });

  it("has font sizes", () => {
    expect(theme.typography.fontSize.xs).toBe("0.75rem");
    expect(theme.typography.fontSize.sm).toBe("0.875rem");
    expect(theme.typography.fontSize.base).toBe("1rem");
    expect(theme.typography.fontSize.lg).toBe("1.125rem");
    expect(theme.typography.fontSize.xl).toBe("1.25rem");
    expect(theme.typography.fontSize["2xl"]).toBe("1.5rem");
    expect(theme.typography.fontSize["3xl"]).toBe("1.875rem");
  });

  it("has font weights", () => {
    expect(theme.typography.fontWeight.normal).toBe("400");
    expect(theme.typography.fontWeight.medium).toBe("500");
    expect(theme.typography.fontWeight.bold).toBe("700");
  });

  it("has line heights", () => {
    expect(theme.typography.lineHeight.tight).toBe("1.25");
    expect(theme.typography.lineHeight.normal).toBe("1.5");
    expect(theme.typography.lineHeight.relaxed).toBe("1.75");
  });
});

describe("theme.spacing", () => {
  it("has all spacing values", () => {
    expect(theme.spacing[0]).toBe("0px");
    expect(theme.spacing[1]).toBe("0.25rem");
    expect(theme.spacing[2]).toBe("0.5rem");
    expect(theme.spacing[3]).toBe("0.75rem");
    expect(theme.spacing[4]).toBe("1rem");
    expect(theme.spacing[6]).toBe("1.5rem");
    expect(theme.spacing[8]).toBe("2rem");
    expect(theme.spacing[10]).toBe("2.5rem");
    expect(theme.spacing[12]).toBe("3rem");
    expect(theme.spacing[16]).toBe("4rem");
  });
});

describe("theme.borderRadius", () => {
  it("has all border radius values", () => {
    expect(theme.borderRadius.none).toBe("0");
    expect(theme.borderRadius.sm).toBe("0.125rem");
    expect(theme.borderRadius.md).toBe("0.375rem");
    expect(theme.borderRadius.lg).toBe("0.5rem");
    expect(theme.borderRadius.xl).toBe("0.75rem");
    expect(theme.borderRadius["2xl"]).toBe("1rem");
    expect(theme.borderRadius.full).toBe("9999px");
  });
});

describe("theme.boxShadow", () => {
  it("has all shadow values", () => {
    expect(theme.boxShadow.sm).toBe("0 1px 2px 0 rgba(0, 0, 0, 0.05)");
    expect(theme.boxShadow.md).toBe(
      "0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)",
    );
    expect(theme.boxShadow.lg).toBe(
      "0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05)",
    );
    expect(theme.boxShadow.xl).toBe(
      "0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04)",
    );
  });
});

describe("theme.transition", () => {
  it("has all transition values", () => {
    expect(theme.transition.default).toBe("all 0.2s ease");
    expect(theme.transition.fast).toBe("all 0.1s ease");
    expect(theme.transition.slow).toBe("all 0.3s ease");
  });
});

describe("theme.breakpoints", () => {
  it("has all breakpoint values", () => {
    expect(theme.breakpoints.sm).toBe("640px");
    expect(theme.breakpoints.md).toBe("768px");
    expect(theme.breakpoints.lg).toBe("1024px");
    expect(theme.breakpoints.xl).toBe("1280px");
    expect(theme.breakpoints["2xl"]).toBe("1536px");
  });
});

describe("theme.zIndex", () => {
  it("has all z-index values", () => {
    expect(theme.zIndex.background).toBe(-10);
    expect(theme.zIndex.backgroundWrapper).toBe(-5);
    expect(theme.zIndex.base).toBe(0);
    expect(theme.zIndex.dropdown).toBe(10);
    expect(theme.zIndex.modal).toBe(50);
    expect(theme.zIndex.toast).toBe(100);
  });
});

describe("default export", () => {
  it("exports the same theme object as named export", () => {
    expect(defaultTheme).toBe(theme);
  });
});
