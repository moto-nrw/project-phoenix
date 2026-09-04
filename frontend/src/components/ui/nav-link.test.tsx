import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import {
  AppRouterContext,
  type AppRouterInstance,
} from "next/dist/shared/lib/app-router-context.shared-runtime";
import type { ComponentProps, ReactNode } from "react";

import { NavLink } from "./nav-link";

const mockPrefetch = vi.fn();
const linkProps = vi.fn();
const mockRouter = { prefetch: mockPrefetch } as unknown as AppRouterInstance;

vi.mock("next/link", () => ({
  // NavLink hängt seit #2828 einen Melder in den Link, der den ausstehenden
  // Wechsel über useLinkStatus abfragt.
  useLinkStatus: () => ({ pending: false }),
  default: ({
    href,
    children,
    prefetch,
    ...rest
  }: ComponentProps<"a"> & { prefetch?: boolean | null }) => {
    linkProps({ href, prefetch });
    return (
      <a href={href} {...rest}>
        {children}
      </a>
    );
  },
}));

// The same context next/link reads. The shell tests render without it.
function withRouter(children: ReactNode) {
  return (
    <AppRouterContext.Provider value={mockRouter}>
      {children}
    </AppRouterContext.Provider>
  );
}

describe("NavLink", () => {
  beforeEach(() => {
    mockPrefetch.mockClear();
    linkProps.mockClear();
  });

  it("switches the viewport prefetch of next/link off", () => {
    render(withRouter(<NavLink href="/rooms">Räume</NavLink>));

    expect(screen.getByRole("link", { name: "Räume" })).toHaveAttribute(
      "href",
      "/rooms",
    );
    expect(linkProps).toHaveBeenCalledWith({ href: "/rooms", prefetch: false });
    expect(mockPrefetch).not.toHaveBeenCalled();
  });

  it("prefetches when the pointer enters the link", () => {
    render(withRouter(<NavLink href="/rooms">Räume</NavLink>));

    fireEvent.pointerEnter(screen.getByRole("link", { name: "Räume" }));

    expect(mockPrefetch).toHaveBeenCalledTimes(1);
    expect(mockPrefetch).toHaveBeenCalledWith("/rooms");
  });

  it("prefetches when the link receives keyboard focus", () => {
    render(withRouter(<NavLink href="/staff">Mitarbeitende</NavLink>));

    fireEvent.focus(screen.getByRole("link", { name: "Mitarbeitende" }));

    expect(mockPrefetch).toHaveBeenCalledWith("/staff");
  });

  it("keeps the caller's pointer and focus handlers", () => {
    const onPointerEnter = vi.fn();
    const onFocus = vi.fn();
    render(
      withRouter(
        <NavLink
          href="/rooms"
          onPointerEnter={onPointerEnter}
          onFocus={onFocus}
        >
          Räume
        </NavLink>,
      ),
    );

    const link = screen.getByRole("link", { name: "Räume" });
    fireEvent.pointerEnter(link);
    fireEvent.focus(link);

    expect(onPointerEnter).toHaveBeenCalledTimes(1);
    expect(onFocus).toHaveBeenCalledTimes(1);
    expect(mockPrefetch).toHaveBeenCalledTimes(2);
  });

  it("never prefetches links that open a new tab", () => {
    render(
      withRouter(
        <NavLink href="/help" target="_blank" rel="noopener noreferrer">
          Hilfe
        </NavLink>,
      ),
    );

    const link = screen.getByRole("link", { name: "Hilfe" });
    fireEvent.pointerEnter(link);
    fireEvent.focus(link);

    expect(link).toHaveAttribute("target", "_blank");
    expect(mockPrefetch).not.toHaveBeenCalled();
  });

  it("never prefetches hrefs that are not same-origin paths", () => {
    render(
      withRouter(
        <>
          <NavLink href="https://example.org/extern">Extern</NavLink>
          <NavLink href="//example.org/extern">Protokollrelativ</NavLink>
          <NavLink href="mailto:info@example.org">Mail</NavLink>
        </>,
      ),
    );

    for (const name of ["Extern", "Protokollrelativ", "Mail"]) {
      fireEvent.pointerEnter(screen.getByRole("link", { name }));
    }

    expect(mockPrefetch).not.toHaveBeenCalled();
  });

  it("renders without an app router and then never prefetches", () => {
    render(<NavLink href="/rooms">Räume</NavLink>);

    const link = screen.getByRole("link", { name: "Räume" });
    fireEvent.pointerEnter(link);
    fireEvent.focus(link);

    expect(link).toHaveAttribute("href", "/rooms");
    expect(linkProps).toHaveBeenCalledWith({ href: "/rooms", prefetch: false });
    expect(mockPrefetch).not.toHaveBeenCalled();
  });

  it("passes className, aria and click handlers through to the anchor", () => {
    const onClick = vi.fn();
    render(
      withRouter(
        <NavLink
          href="/rooms"
          className="nav-item"
          aria-label="Räume öffnen"
          onClick={onClick}
        >
          Räume
        </NavLink>,
      ),
    );

    const link = screen.getByRole("link", { name: "Räume öffnen" });
    expect(link).toHaveClass("nav-item");
    fireEvent.click(link);
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});
