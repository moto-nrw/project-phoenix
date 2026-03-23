import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type React from "react";

import type { ChartConfig } from "./chart";

// Mock recharts before importing chart components
vi.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-container">{children}</div>
  ),
  Tooltip: () => <div data-testid="tooltip" />,
  Legend: () => <div data-testid="legend" />,
}));

// Dynamic import after mock is registered
const {
  ChartContainer,
  ChartTooltipContent,
  ChartLegendContent,
  ChartTooltip,
  ChartLegend,
} = await import("./chart");

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const baseConfig: ChartConfig = {
  revenue: {
    label: "Revenue",
    color: "hsl(220 70% 50%)",
  },
  expenses: {
    label: "Expenses",
    color: "hsl(0 70% 50%)",
  },
};

const themeConfig: ChartConfig = {
  revenue: {
    label: "Revenue",
    theme: { light: "hsl(220 70% 50%)", dark: "hsl(220 70% 30%)" },
  },
};

const emptyConfig: ChartConfig = {
  revenue: { label: "Revenue" },
};

function TestIcon() {
  return <svg data-testid="test-icon" />;
}

const configWithIcon: ChartConfig = {
  revenue: {
    label: "Revenue",
    color: "hsl(220 70% 50%)",
    icon: TestIcon,
  },
};

/**
 * Helper: render ChartTooltipContent inside a ChartContainer so the
 * useChart() context is available.
 */
function renderTooltipContent(
  props: React.ComponentProps<typeof ChartTooltipContent>,
  config: ChartConfig = baseConfig,
) {
  return render(
    <ChartContainer config={config}>
      <ChartTooltipContent {...props} />
    </ChartContainer>,
  );
}

/**
 * Helper: render ChartLegendContent inside a ChartContainer.
 */
function renderLegendContent(
  props: React.ComponentProps<typeof ChartLegendContent>,
  config: ChartConfig = baseConfig,
) {
  return render(
    <ChartContainer config={config}>
      <ChartLegendContent {...props} />
    </ChartContainer>,
  );
}

// ---------------------------------------------------------------------------
// useChart
// ---------------------------------------------------------------------------

describe("useChart", () => {
  it("throws when used outside ChartContainer", () => {
    // Suppress React error boundary noise
    // eslint-disable-next-line @typescript-eslint/no-empty-function
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});

    expect(() => render(<ChartTooltipContent active payload={[]} />)).toThrow(
      "useChart must be used within a <ChartContainer />",
    );

    spy.mockRestore();
  });
});

// ---------------------------------------------------------------------------
// ChartContainer
// ---------------------------------------------------------------------------

describe("ChartContainer", () => {
  it("renders children inside a ResponsiveContainer", () => {
    render(
      <ChartContainer config={baseConfig}>
        <span data-testid="child">Hello</span>
      </ChartContainer>,
    );

    expect(screen.getByTestId("responsive-container")).toBeInTheDocument();
    expect(screen.getByTestId("child")).toBeInTheDocument();
  });

  it("applies custom id to data-chart attribute", () => {
    const { container } = render(
      <ChartContainer config={baseConfig} id="my-chart">
        <span>Content</span>
      </ChartContainer>,
    );

    const wrapper = container.querySelector("[data-chart]");
    expect(wrapper).toHaveAttribute("data-chart", "chart-my-chart");
  });

  it("generates a unique id when none is provided", () => {
    const { container } = render(
      <ChartContainer config={baseConfig}>
        <span>Content</span>
      </ChartContainer>,
    );

    const wrapper = container.querySelector("[data-chart]");
    const dataChart = wrapper?.getAttribute("data-chart");
    expect(dataChart).toBeTruthy();
    expect(dataChart).toMatch(/^chart-/);
    // React.useId produces colons which get stripped
    expect(dataChart).not.toContain(":");
  });

  it("merges custom className", () => {
    const { container } = render(
      <ChartContainer config={baseConfig} className="extra-class">
        <span>Content</span>
      </ChartContainer>,
    );

    const wrapper = container.querySelector("[data-chart]");
    expect(wrapper?.className).toContain("extra-class");
  });

  it("forwards additional HTML props", () => {
    const { container } = render(
      <ChartContainer config={baseConfig} data-custom="hello">
        <span>Content</span>
      </ChartContainer>,
    );

    const wrapper = container.querySelector("[data-chart]");
    expect(wrapper).toHaveAttribute("data-custom", "hello");
  });

  it("renders a <style> element when config has color values", () => {
    const { container } = render(
      <ChartContainer config={baseConfig} id="styled">
        <span>Content</span>
      </ChartContainer>,
    );

    const style = container.querySelector("style");
    expect(style).toBeInTheDocument();
    expect(style?.innerHTML).toContain("--color-revenue");
    expect(style?.innerHTML).toContain("--color-expenses");
  });

  it("does not render a <style> element when config has no color/theme", () => {
    const { container } = render(
      <ChartContainer config={emptyConfig} id="no-style">
        <span>Content</span>
      </ChartContainer>,
    );

    const style = container.querySelector("style");
    expect(style).toBeNull();
  });

  it("renders theme-based CSS variables for light and dark", () => {
    const { container } = render(
      <ChartContainer config={themeConfig} id="themed">
        <span>Content</span>
      </ChartContainer>,
    );

    const style = container.querySelector("style");
    expect(style).toBeInTheDocument();
    const html = style?.innerHTML ?? "";
    expect(html).toContain("[data-chart=chart-themed]");
    expect(html).toContain("hsl(220 70% 50%)"); // light
    expect(html).toContain("hsl(220 70% 30%)"); // dark
    expect(html).toContain(".dark");
  });
});

// ---------------------------------------------------------------------------
// ChartTooltip / ChartLegend (re-exports)
// ---------------------------------------------------------------------------

describe("ChartTooltip (re-export)", () => {
  it("is the recharts Tooltip component", () => {
    expect(ChartTooltip).toBeDefined();
  });
});

describe("ChartLegend (re-export)", () => {
  it("is the recharts Legend component", () => {
    expect(ChartLegend).toBeDefined();
  });
});

// ---------------------------------------------------------------------------
// ChartTooltipContent
// ---------------------------------------------------------------------------

describe("ChartTooltipContent", () => {
  const makePayload = (overrides: Record<string, unknown> = {}) =>
    [
      {
        dataKey: "revenue",
        name: "revenue",
        value: 1234,
        color: "hsl(220 70% 50%)",
        payload: { fill: "hsl(220 70% 50%)" },
        graphicalItemId: 0,
        type: undefined,
        ...overrides,
      },
    ] as never;

  it("returns null when not active", () => {
    const { container } = renderTooltipContent({
      active: false,
      payload: makePayload(),
    });
    // The ChartContainer wrapper renders, but the tooltip content itself should not
    expect(container.querySelector(".border-border\\/50")).toBeNull();
  });

  it("returns null when payload is empty", () => {
    const { container } = renderTooltipContent({
      active: true,
      payload: [],
    });
    expect(container.querySelector(".border-border\\/50")).toBeNull();
  });

  it("returns null when payload is undefined", () => {
    const { container } = renderTooltipContent({
      active: true,
      payload: undefined,
    });
    expect(container.querySelector(".border-border\\/50")).toBeNull();
  });

  it("renders tooltip content when active with payload", () => {
    renderTooltipContent({
      active: true,
      payload: makePayload(),
      label: "January",
    });

    expect(screen.getByText("Revenue")).toBeInTheDocument();
    expect(screen.getByText("1,234")).toBeInTheDocument();
  });

  it("shows label from config when label matches a config key", () => {
    renderTooltipContent({
      active: true,
      payload: makePayload(),
      label: "revenue",
    });

    // "Revenue" appears as both the label (from config[label].label) and the item label
    const elements = screen.getAllByText("Revenue");
    expect(elements.length).toBeGreaterThanOrEqual(1);
  });

  it("shows raw label string when it does not match a config key", () => {
    renderTooltipContent({
      active: true,
      payload: makePayload(),
      label: "January",
    });

    expect(screen.getByText("January")).toBeInTheDocument();
  });

  it("hides label when hideLabel is true", () => {
    renderTooltipContent({
      active: true,
      payload: makePayload(),
      label: "January",
      hideLabel: true,
    });

    expect(screen.queryByText("January")).toBeNull();
  });

  it("hides indicator when hideIndicator is true", () => {
    const { container } = renderTooltipContent({
      active: true,
      payload: makePayload(),
      hideIndicator: true,
    });

    // No indicator div with --color-bg style
    const indicators = container.querySelectorAll("[style*='--color-bg']");
    expect(indicators.length).toBe(0);
  });

  it("renders dot indicator by default", () => {
    const { container } = renderTooltipContent({
      active: true,
      payload: makePayload(),
    });

    const indicator = container.querySelector("[style*='--color-bg']");
    expect(indicator).toBeInTheDocument();
    expect(indicator?.className).toContain("h-2.5");
    expect(indicator?.className).toContain("w-2.5");
  });

  it("renders line indicator variant", () => {
    const { container } = renderTooltipContent({
      active: true,
      payload: makePayload(),
      indicator: "line",
    });

    const indicator = container.querySelector("[style*='--color-bg']");
    expect(indicator).toBeInTheDocument();
    expect(indicator?.className).toContain("w-1");
  });

  it("renders dashed indicator variant", () => {
    const { container } = renderTooltipContent({
      active: true,
      payload: makePayload(),
      indicator: "dashed",
    });

    const indicator = container.querySelector("[style*='--color-bg']");
    expect(indicator).toBeInTheDocument();
    expect(indicator?.className).toContain("border-dashed");
  });

  it("uses custom formatter when provided", () => {
    const formatter = vi.fn(
      (value: unknown, name: unknown) => `${String(name)}: ${String(value)}`,
    );

    renderTooltipContent({
      active: true,
      payload: makePayload(),
      formatter,
    });

    expect(formatter).toHaveBeenCalledWith(
      1234,
      "revenue",
      expect.objectContaining({ dataKey: "revenue" }),
      0,
      expect.objectContaining({ fill: "hsl(220 70% 50%)" }),
    );
  });

  it("uses labelFormatter when provided", () => {
    const labelFormatter = vi.fn(() => "Formatted Label");

    renderTooltipContent({
      active: true,
      payload: makePayload(),
      label: "January",
      labelFormatter,
    });

    expect(screen.getByText("Formatted Label")).toBeInTheDocument();
    expect(labelFormatter).toHaveBeenCalled();
  });

  it("renders icon from config instead of indicator", () => {
    renderTooltipContent(
      {
        active: true,
        payload: makePayload(),
      },
      configWithIcon,
    );

    expect(screen.getByTestId("test-icon")).toBeInTheDocument();
  });

  it("filters out payload items with type 'none'", () => {
    const payload = [
      {
        dataKey: "revenue",
        name: "revenue",
        value: 100,
        color: "blue",
        type: "none" as const,
        payload: {},
      },
    ] as never;

    const { container } = renderTooltipContent({
      active: true,
      payload,
      hideLabel: true,
    });

    // The item should be filtered out, so no value text
    expect(screen.queryByText("100")).toBeNull();
    // But the wrapper div still renders (since payload.length > 0 passes the guard)
    expect(container.querySelector(".grid.gap-1\\.5")).toBeInTheDocument();
  });

  it("applies custom className", () => {
    const { container } = renderTooltipContent({
      active: true,
      payload: makePayload(),
      className: "my-tooltip",
    });

    // Find the tooltip wrapper div (outermost within the chart container)
    const tooltipWrapper = container.querySelector(".my-tooltip");
    expect(tooltipWrapper).toBeInTheDocument();
  });

  it("nests label when single payload item and indicator is not dot", () => {
    renderTooltipContent({
      active: true,
      payload: makePayload(),
      label: "January",
      indicator: "line",
    });

    // With nestLabel = true, the label appears nested inside the item row
    expect(screen.getByText("January")).toBeInTheDocument();
  });

  it("does not nest label when multiple payload items", () => {
    const payload = [
      {
        dataKey: "revenue",
        name: "revenue",
        value: 1000,
        color: "blue",
        payload: { fill: "blue" },
      },
      {
        dataKey: "expenses",
        name: "expenses",
        value: 500,
        color: "red",
        payload: { fill: "red" },
      },
    ] as never;

    renderTooltipContent({
      active: true,
      payload,
      label: "January",
    });

    expect(screen.getByText("January")).toBeInTheDocument();
    expect(screen.getByText("1,000")).toBeInTheDocument();
    expect(screen.getByText("500")).toBeInTheDocument();
  });

  it("uses nameKey to resolve config", () => {
    const config: ChartConfig = {
      custom_revenue: {
        label: "Custom Revenue Label",
        color: "green",
      },
    };

    const payload = [
      {
        dataKey: "revenue",
        name: "revenue",
        value: 42,
        color: "green",
        payload: { custom_key: "custom_revenue" },
      },
    ] as never;

    renderTooltipContent(
      {
        active: true,
        payload,
        nameKey: "custom_key",
        hideLabel: true,
      },
      config,
    );

    expect(screen.getByText("Custom Revenue Label")).toBeInTheDocument();
  });

  it("uses labelKey to resolve label from config", () => {
    const config: ChartConfig = {
      month: {
        label: "Month Label",
        color: "blue",
      },
      revenue: {
        label: "Revenue",
        color: "blue",
      },
    };

    const payload = [
      {
        dataKey: "revenue",
        name: "revenue",
        value: 100,
        color: "blue",
        payload: { month: "month" },
      },
    ] as never;

    renderTooltipContent(
      {
        active: true,
        payload,
        labelKey: "month",
      },
      config,
    );

    expect(screen.getByText("Month Label")).toBeInTheDocument();
  });

  it("uses color prop over payload fill and item color", () => {
    const { container } = renderTooltipContent({
      active: true,
      payload: makePayload(),
      color: "rgb(255, 0, 0)",
    });

    const indicator = container.querySelector("[style*='--color-bg']");
    expect(indicator).toBeInTheDocument();
    const style = indicator?.getAttribute("style") ?? "";
    expect(style).toContain("rgb(255, 0, 0)");
  });

  it("handles payload item with function dataKey", () => {
    const payload = [
      {
        dataKey: () => "computed",
        name: "revenue",
        value: 999,
        color: "blue",
        payload: { fill: "blue" },
      },
    ] as never;

    renderTooltipContent({
      active: true,
      payload,
      hideLabel: true,
    });

    // Should still render the value
    expect(screen.getByText("999")).toBeInTheDocument();
  });

  it("shows item.name when config label is not available", () => {
    const config: ChartConfig = {};

    const payload = [
      {
        dataKey: "unknown",
        name: "My Name",
        value: 50,
        color: "blue",
        payload: {},
      },
    ] as never;

    renderTooltipContent(
      {
        active: true,
        payload,
        hideLabel: true,
      },
      config,
    );

    expect(screen.getByText("My Name")).toBeInTheDocument();
  });

  it("handles function dataKey in tooltip label resolution", () => {
    renderTooltipContent({
      active: true,
      payload: makePayload({ dataKey: () => "computed" }),
    });

    // Should still render without crashing; label falls back to item.name
    expect(screen.getByText("1,234")).toBeInTheDocument();
  });

  it("uses index as key when itemDataKey is undefined (function dataKey)", () => {
    const payload = [
      {
        dataKey: () => "computed",
        name: "revenue",
        value: 777,
        color: "blue",
        payload: { fill: "blue" },
      },
      {
        dataKey: () => "computed2",
        name: "expenses",
        value: 333,
        color: "red",
        payload: { fill: "red" },
      },
    ] as never;

    renderTooltipContent({
      active: true,
      payload,
      hideLabel: true,
    });

    // Both items render (keyed by index fallback)
    expect(screen.getByText("777")).toBeInTheDocument();
    expect(screen.getByText("333")).toBeInTheDocument();
  });

  it("applies labelClassName to label element", () => {
    const { container } = renderTooltipContent({
      active: true,
      payload: makePayload(),
      label: "January",
      labelClassName: "custom-label-class",
    });

    const labelEl = container.querySelector(".custom-label-class");
    expect(labelEl).toBeInTheDocument();
    expect(labelEl?.textContent).toBe("January");
  });
});

// ---------------------------------------------------------------------------
// ChartLegendContent
// ---------------------------------------------------------------------------

describe("ChartLegendContent", () => {
  const makeLegendPayload = (overrides: Record<string, unknown> = {}) => [
    {
      value: "revenue",
      dataKey: "revenue",
      color: "hsl(220 70% 50%)",
      ...overrides,
    },
  ];

  it("returns null when payload is empty", () => {
    const { container } = renderLegendContent({ payload: [] });
    // Only the chart container wrapper renders, not the legend itself
    expect(
      container.querySelector(".flex.items-center.justify-center"),
    ).toBeNull();
  });

  it("returns null when payload is undefined", () => {
    const { container } = renderLegendContent({ payload: undefined });
    expect(
      container.querySelector(".flex.items-center.justify-center"),
    ).toBeNull();
  });

  it("renders legend items with labels from config", () => {
    renderLegendContent({ payload: makeLegendPayload() });

    expect(screen.getByText("Revenue")).toBeInTheDocument();
  });

  it("renders colored indicator div when no icon in config", () => {
    const { container } = renderLegendContent({
      payload: makeLegendPayload(),
    });

    const colorDiv = container.querySelector(
      ".h-2.w-2.shrink-0.rounded-\\[2px\\]",
    );
    expect(colorDiv).toBeInTheDocument();
    expect(colorDiv).toHaveStyle({
      backgroundColor: "hsl(220 70% 50%)",
    });
  });

  it("renders icon from config instead of color div", () => {
    renderLegendContent({ payload: makeLegendPayload() }, configWithIcon);

    expect(screen.getByTestId("test-icon")).toBeInTheDocument();
  });

  it("hides icon when hideIcon is true", () => {
    const { container } = renderLegendContent(
      { payload: makeLegendPayload(), hideIcon: true },
      configWithIcon,
    );

    // hideIcon suppresses the config icon, but the fallback color div still renders
    expect(screen.queryByTestId("test-icon")).toBeNull();
    const colorDiv = container.querySelector(
      ".h-2.w-2.shrink-0.rounded-\\[2px\\]",
    );
    expect(colorDiv).toBeInTheDocument();
  });

  it("applies pt-3 class when verticalAlign is bottom (default)", () => {
    const { container } = renderLegendContent({
      payload: makeLegendPayload(),
    });

    const legendWrapper = container.querySelector(
      ".flex.items-center.justify-center",
    );
    expect(legendWrapper?.className).toContain("pt-3");
  });

  it("applies pb-3 class when verticalAlign is top", () => {
    const { container } = renderLegendContent({
      payload: makeLegendPayload(),
      verticalAlign: "top",
    });

    const legendWrapper = container.querySelector(
      ".flex.items-center.justify-center",
    );
    expect(legendWrapper?.className).toContain("pb-3");
  });

  it("applies custom className", () => {
    const { container } = renderLegendContent({
      payload: makeLegendPayload(),
      className: "my-legend",
    });

    const legendWrapper = container.querySelector(".my-legend");
    expect(legendWrapper).toBeInTheDocument();
  });

  it("renders multiple legend items", () => {
    const payload = [
      { value: "revenue", dataKey: "revenue", color: "blue" },
      { value: "expenses", dataKey: "expenses", color: "red" },
    ];

    renderLegendContent({ payload });

    expect(screen.getByText("Revenue")).toBeInTheDocument();
    expect(screen.getByText("Expenses")).toBeInTheDocument();
  });

  it("uses nameKey to resolve config key", () => {
    const config: ChartConfig = {
      custom_key: {
        label: "Custom Legend Label",
        color: "green",
      },
    };

    const payload = [
      {
        value: "something",
        dataKey: "other",
        color: "green",
      },
    ];

    renderLegendContent({ payload, nameKey: "custom_key" }, config);

    expect(screen.getByText("Custom Legend Label")).toBeInTheDocument();
  });
});
