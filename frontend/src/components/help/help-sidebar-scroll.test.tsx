import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { useHelpSidebarScrollRestoration } from "./help-sidebar-scroll";

function ScrollArea() {
  const scrollAreaRef = useHelpSidebarScrollRestoration();
  return <div ref={scrollAreaRef} data-testid="sidebar-scroll-area" />;
}

describe("useHelpSidebarScrollRestoration", () => {
  it("restores the sidebar position after the route remounts its shell", () => {
    const firstRender = render(<ScrollArea />);
    screen.getByTestId("sidebar-scroll-area").scrollTop = 900;
    firstRender.unmount();

    const secondRender = render(<ScrollArea />);
    expect(screen.getByTestId("sidebar-scroll-area").scrollTop).toBe(900);

    screen.getByTestId("sidebar-scroll-area").scrollTop = 0;
    secondRender.unmount();
  });
});
