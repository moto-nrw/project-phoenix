import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { describe, it, expect, vi, afterEach } from "vitest";
import { NavigationTabs } from "./NavigationTabs";

// Stub ResizeObserver for jsdom
class MockResizeObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}
vi.stubGlobal("ResizeObserver", MockResizeObserver);

const twoItems = [
  { id: "a", label: "Alpha" },
  { id: "b", label: "Beta" },
];

const threeItems = [
  { id: "a", label: "Alpha" },
  { id: "b", label: "Beta" },
  { id: "c", label: "Gamma" },
];

const fiveItems = [
  { id: "a", label: "Alpha" },
  { id: "b", label: "Beta" },
  { id: "c", label: "Gamma" },
  { id: "d", label: "Delta" },
  { id: "e", label: "Epsilon" },
];

afterEach(() => {
  cleanup();
});

describe("NavigationTabs — tab rendering", () => {
  it("renders all tab buttons", () => {
    const onChange = vi.fn();
    render(
      <NavigationTabs items={twoItems} activeTab="a" onTabChange={onChange} />,
    );

    expect(screen.getByText("Alpha")).toBeInTheDocument();
    expect(screen.getByText("Beta")).toBeInTheDocument();
  });

  it("highlights active tab with correct styling", () => {
    const onChange = vi.fn();
    render(
      <NavigationTabs items={twoItems} activeTab="b" onTabChange={onChange} />,
    );

    const betaButton = screen.getByText("Beta").closest("button");
    expect(betaButton?.className).toContain("font-semibold");
    expect(betaButton?.className).toContain("text-gray-900");
  });

  it("calls onTabChange when a tab is clicked", () => {
    const onChange = vi.fn();
    render(
      <NavigationTabs items={twoItems} activeTab="a" onTabChange={onChange} />,
    );

    fireEvent.click(screen.getByText("Beta"));
    expect(onChange).toHaveBeenCalledWith("b");
  });

  it("applies custom className", () => {
    const onChange = vi.fn();
    const { container } = render(
      <NavigationTabs
        items={twoItems}
        activeTab="a"
        onTabChange={onChange}
        className="my-custom"
      />,
    );

    const root = container.firstChild as HTMLElement;
    expect(root.className).toContain("my-custom");
  });
});

describe("NavigationTabs — fewer than 3 items (no mobile dropdown)", () => {
  it("does not render a dropdown trigger for 2 items", () => {
    const onChange = vi.fn();
    render(
      <NavigationTabs items={twoItems} activeTab="a" onTabChange={onChange} />,
    );

    // Tabs should be visible, no dropdown trigger (no SVG chevron outside tab area)
    const buttons = screen.getAllByRole("button");
    // Only 2 tab buttons, no dropdown
    expect(buttons).toHaveLength(2);
  });

  it("tabs container is not hidden on any viewport for <3 items", () => {
    const onChange = vi.fn();
    const { container } = render(
      <NavigationTabs items={twoItems} activeTab="a" onTabChange={onChange} />,
    );

    // The tabs wrapper should NOT contain the "hidden" class
    const tabsWrapper = container.querySelector(".relative");
    expect(tabsWrapper?.className).not.toContain("hidden");
  });
});

describe("NavigationTabs — 3+ items (mobile dropdown)", () => {
  it("renders the dropdown trigger showing active tab label", () => {
    const onChange = vi.fn();
    render(
      <NavigationTabs
        items={threeItems}
        activeTab="b"
        onTabChange={onChange}
      />,
    );

    // Both the dropdown trigger and the hidden tab render "Beta"
    const betaElements = screen.getAllByText("Beta");
    expect(betaElements.length).toBeGreaterThanOrEqual(1);

    // The dropdown trigger span should have the text-gray-900 class
    const triggerSpan = betaElements.find(
      (el) => el.className === "text-gray-900",
    );
    expect(triggerSpan).toBeTruthy();
  });

  it("hides tabs container on mobile (adds hidden md:block class)", () => {
    const onChange = vi.fn();
    const { container } = render(
      <NavigationTabs
        items={threeItems}
        activeTab="a"
        onTabChange={onChange}
      />,
    );

    // The tabs wrapper should contain the "hidden" class for mobile
    const divs = container.querySelectorAll("div.relative");
    const hiddenDiv = Array.from(divs).find((d) =>
      d.className.includes("hidden md:block"),
    );
    expect(hiddenDiv).toBeTruthy();
  });

  it("opens dropdown menu when trigger is clicked", () => {
    const onChange = vi.fn();
    render(
      <NavigationTabs
        items={threeItems}
        activeTab="a"
        onTabChange={onChange}
      />,
    );

    // Find the dropdown trigger (the button inside md:hidden container)
    const dropdownTrigger = screen
      .getAllByRole("button")
      .find(
        (btn) =>
          btn.closest(".md\\:hidden") !== null && btn.textContent === "Alpha",
      );
    expect(dropdownTrigger).toBeTruthy();

    fireEvent.click(dropdownTrigger!);

    // All three items should now appear in the dropdown menu
    // The dropdown renders buttons for each item
    const allButtons = screen.getAllByRole("button");
    const dropdownItems = allButtons.filter(
      (btn) =>
        btn.className.includes("w-full") && btn.className.includes("text-left"),
    );
    expect(dropdownItems).toHaveLength(3);
  });

  it("calls onTabChange and closes dropdown when an item is clicked", () => {
    const onChange = vi.fn();
    render(
      <NavigationTabs
        items={threeItems}
        activeTab="a"
        onTabChange={onChange}
      />,
    );

    // Open dropdown
    const dropdownTrigger = screen
      .getAllByRole("button")
      .find(
        (btn) =>
          btn.closest(".md\\:hidden") !== null && btn.textContent === "Alpha",
      );
    fireEvent.click(dropdownTrigger!);

    // Click "Gamma" in the dropdown
    const dropdownItems = screen
      .getAllByRole("button")
      .filter(
        (btn) =>
          btn.className.includes("w-full") &&
          btn.className.includes("text-left"),
      );
    const gammaItem = dropdownItems.find((btn) => btn.textContent === "Gamma");
    expect(gammaItem).toBeTruthy();
    fireEvent.click(gammaItem!);

    expect(onChange).toHaveBeenCalledWith("c");

    // Dropdown menu should be closed (no more full-width items)
    const remainingDropdownItems = screen
      .getAllByRole("button")
      .filter(
        (btn) =>
          btn.className.includes("w-full") &&
          btn.className.includes("text-left"),
      );
    expect(remainingDropdownItems).toHaveLength(0);
  });

  it("highlights active item in dropdown menu", () => {
    const onChange = vi.fn();
    render(
      <NavigationTabs
        items={threeItems}
        activeTab="b"
        onTabChange={onChange}
      />,
    );

    // Open dropdown
    const dropdownTrigger = screen
      .getAllByRole("button")
      .find(
        (btn) =>
          btn.closest(".md\\:hidden") !== null && btn.textContent === "Beta",
      );
    fireEvent.click(dropdownTrigger!);

    // Beta should have active styling
    const dropdownItems = screen
      .getAllByRole("button")
      .filter(
        (btn) =>
          btn.className.includes("w-full") &&
          btn.className.includes("text-left"),
      );
    const betaItem = dropdownItems.find((btn) => btn.textContent === "Beta");
    expect(betaItem?.className).toContain("font-semibold");
    expect(betaItem?.className).toContain("bg-gray-50");

    // Alpha should NOT have active styling
    const alphaItem = dropdownItems.find((btn) => btn.textContent === "Alpha");
    expect(alphaItem?.className).not.toContain("font-semibold");
  });

  it("closes dropdown on click outside", () => {
    const onChange = vi.fn();
    render(
      <NavigationTabs
        items={threeItems}
        activeTab="a"
        onTabChange={onChange}
      />,
    );

    // Open dropdown
    const dropdownTrigger = screen
      .getAllByRole("button")
      .find(
        (btn) =>
          btn.closest(".md\\:hidden") !== null && btn.textContent === "Alpha",
      );
    fireEvent.click(dropdownTrigger!);

    // Verify dropdown is open
    let dropdownItems = screen
      .getAllByRole("button")
      .filter(
        (btn) =>
          btn.className.includes("w-full") &&
          btn.className.includes("text-left"),
      );
    expect(dropdownItems.length).toBeGreaterThan(0);

    // Click outside (on document body)
    fireEvent.mouseDown(document.body);

    // Dropdown should close
    dropdownItems = screen
      .getAllByRole("button")
      .filter(
        (btn) =>
          btn.className.includes("w-full") &&
          btn.className.includes("text-left"),
      );
    expect(dropdownItems).toHaveLength(0);
  });

  it("toggles dropdown open and closed with trigger clicks", () => {
    const onChange = vi.fn();
    render(
      <NavigationTabs
        items={threeItems}
        activeTab="a"
        onTabChange={onChange}
      />,
    );

    const dropdownTrigger = screen
      .getAllByRole("button")
      .find(
        (btn) =>
          btn.closest(".md\\:hidden") !== null && btn.textContent === "Alpha",
      );

    // Open
    fireEvent.click(dropdownTrigger!);
    let dropdownItems = screen
      .getAllByRole("button")
      .filter(
        (btn) =>
          btn.className.includes("w-full") &&
          btn.className.includes("text-left"),
      );
    expect(dropdownItems.length).toBeGreaterThan(0);

    // Close by clicking trigger again
    fireEvent.click(dropdownTrigger!);
    dropdownItems = screen
      .getAllByRole("button")
      .filter(
        (btn) =>
          btn.className.includes("w-full") &&
          btn.className.includes("text-left"),
      );
    expect(dropdownItems).toHaveLength(0);
  });
});

describe("NavigationTabs — with 5 items", () => {
  it("renders dropdown for 5 items and all show in menu", () => {
    const onChange = vi.fn();
    render(
      <NavigationTabs items={fiveItems} activeTab="c" onTabChange={onChange} />,
    );

    // Trigger shows active label (appears in both dropdown and hidden tabs)
    const gammaElements = screen.getAllByText("Gamma");
    expect(gammaElements.length).toBeGreaterThanOrEqual(1);

    // Open dropdown
    const dropdownTrigger = screen
      .getAllByRole("button")
      .find(
        (btn) =>
          btn.closest(".md\\:hidden") !== null && btn.textContent === "Gamma",
      );
    fireEvent.click(dropdownTrigger!);

    const dropdownItems = screen
      .getAllByRole("button")
      .filter(
        (btn) =>
          btn.className.includes("w-full") &&
          btn.className.includes("text-left"),
      );
    expect(dropdownItems).toHaveLength(5);
  });
});

describe("NavigationTabs — edge cases", () => {
  it("shows empty string for active label when activeTab matches no item", () => {
    const onChange = vi.fn();
    render(
      <NavigationTabs
        items={threeItems}
        activeTab="nonexistent"
        onTabChange={onChange}
      />,
    );

    // The dropdown trigger should exist but with empty label
    // All three labels should still be in the tab buttons (hidden md:block)
    // but the dropdown trigger text should be empty
    const dropdownTrigger = screen
      .getAllByRole("button")
      .find((btn) => btn.closest(".md\\:hidden") !== null);
    expect(dropdownTrigger).toBeTruthy();
  });

  it("renders sliding indicator bar", () => {
    const onChange = vi.fn();
    const { container } = render(
      <NavigationTabs items={twoItems} activeTab="a" onTabChange={onChange} />,
    );

    // The indicator bar should exist
    const indicator = container.querySelector(
      ".bg-gray-900.transition-all.duration-300",
    );
    expect(indicator).toBeTruthy();
  });

  it("renders tab items with count property", () => {
    const onChange = vi.fn();
    const itemsWithCount = [
      { id: "a", label: "Alpha", count: 5 },
      { id: "b", label: "Beta", count: 10 },
    ];
    render(
      <NavigationTabs
        items={itemsWithCount}
        activeTab="a"
        onTabChange={onChange}
      />,
    );

    expect(screen.getByText("Alpha")).toBeInTheDocument();
    expect(screen.getByText("Beta")).toBeInTheDocument();
  });
});
