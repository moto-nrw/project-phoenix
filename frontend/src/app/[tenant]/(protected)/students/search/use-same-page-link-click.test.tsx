import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { useSamePageLinkClick } from "./use-same-page-link-click";

function Harness({ onClick }: { readonly onClick: () => void }) {
  useSamePageLinkClick(onClick);
  return <button type="button">Kein Link</button>;
}

// The shell renders these links outside the page, so they are plain anchors in
// the document here too. The label sits in a child element, like in the
// sidebar, so the click target is not the anchor itself.
function addLink(label: string, href: string, target?: string) {
  const anchor = document.createElement("a");
  anchor.href = href;
  if (target) anchor.target = target;
  const text = document.createElement("span");
  text.textContent = label;
  anchor.appendChild(text);
  document.body.appendChild(anchor);
}

describe("useSamePageLinkClick", () => {
  beforeEach(() => {
    window.history.replaceState(null, "", "/students/search?pickup_time=14:00");
    addLink("Alle Kinder", "/students/search");
    addLink("Kranke Kinder", "/students/search?status=sick");
    addLink("Zur Liste", "/students/search#liste");
    addLink("Räume", "/rooms");
    addLink("Neuer Tab", "/students/search", "_blank");
    addLink("Extern", "https://example.com/students/search");
  });

  afterEach(() => {
    document.body.innerHTML = "";
    window.history.replaceState(null, "", "/");
  });

  it("fires for a link to the current page without query parameters", () => {
    const onClick = vi.fn();
    render(<Harness onClick={onClick} />);

    // The click lands on the label inside the link, like in the sidebar.
    fireEvent.click(screen.getByText("Alle Kinder"));

    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("does not cancel the link's own navigation", () => {
    render(<Harness onClick={vi.fn()} />);

    const notCancelled = fireEvent.click(screen.getByText("Alle Kinder"));

    expect(notCancelled).toBe(true);
  });

  it.each([
    "Kranke Kinder",
    "Zur Liste",
    "Räume",
    "Neuer Tab",
    "Extern",
    "Kein Link",
  ])("ignores %s", (label) => {
    const onClick = vi.fn();
    render(<Harness onClick={onClick} />);

    fireEvent.click(screen.getByText(label));

    expect(onClick).not.toHaveBeenCalled();
  });

  it.each([
    { metaKey: true },
    { ctrlKey: true },
    { shiftKey: true },
    { altKey: true },
    { button: 1 },
  ])("ignores a modified click %o", (init) => {
    const onClick = vi.fn();
    render(<Harness onClick={onClick} />);

    fireEvent.click(screen.getByText("Alle Kinder"), init);

    expect(onClick).not.toHaveBeenCalled();
  });

  it("calls the latest handler and stops listening after unmount", () => {
    const first = vi.fn();
    const second = vi.fn();
    const { rerender, unmount } = render(<Harness onClick={first} />);
    rerender(<Harness onClick={second} />);

    fireEvent.click(screen.getByText("Alle Kinder"));
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledTimes(1);

    const outside = document.createElement("a");
    outside.href = "/students/search";
    document.body.appendChild(outside);
    unmount();
    fireEvent.click(outside);
    outside.remove();

    expect(second).toHaveBeenCalledTimes(1);
  });
});
