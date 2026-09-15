/**
 * Tests for BrandLink Component
 * Tests rendering and basic interaction
 */
import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { BrandLink, BreadcrumbDivider } from "./brand-link";

// Mock next/link
vi.mock("next/link", () => ({
  useLinkStatus: () => ({ pending: false }),
  default: ({
    children,
    href,
    className,
  }: {
    children: React.ReactNode;
    href: string;
    className?: string;
  }) => (
    <a href={href} className={className}>
      {children}
    </a>
  ),
}));

// Mock next/image
vi.mock("next/image", () => ({
  default: ({
    src,
    alt,
    width,
    height,
  }: {
    src: string;
    alt: string;
    width: number;
    height: number;
  }) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img src={src} alt={alt} width={width} height={height} />
  ),
}));

describe("BrandLink", () => {
  it("renders the brand link with logo", () => {
    render(<BrandLink />);

    const logo = document.querySelector("img");
    expect(logo).toHaveAttribute("alt", "");
    expect(screen.getByText("moto")).toBeInTheDocument();
  });

  it("renders a tenant label instead of the moto wordmark", () => {
    render(<BrandLink label="Demo School" />);

    const brandText = screen.getByText("Demo School");
    expect(brandText).toHaveClass("font-semibold");
    expect(brandText).toHaveClass("text-gray-900");
    expect(brandText).toHaveClass("truncate");
    expect(brandText.parentElement).toHaveClass("min-w-0");
    expect(screen.queryByText("moto")).not.toBeInTheDocument();
  });

  it("links to /home", () => {
    render(<BrandLink />);

    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "/home");
  });

  it("renders the wordmark in one fixed size for the 48px topbar", () => {
    // Seit #2827 gibt es keinen Scroll-Zustand mehr, in den das Logo
    // schrumpfen könnte: eine Größe, keine Übergangsklassen.
    render(<BrandLink />);

    const brandText = screen.getByText("moto");
    expect(brandText).toHaveClass("text-xl");
    expect(brandText).not.toHaveClass("transition-all");
  });

  it("uses compact UI font sizes for tenant labels", () => {
    render(<BrandLink label="Demo School" />);

    const brandText = screen.getByText("Demo School");
    expect(brandText).toHaveClass("text-base");
    expect(brandText).not.toHaveClass("[font-family:var(--font-moto)]");
  });
});

describe("BreadcrumbDivider", () => {
  it("renders a vertical separator", () => {
    const { container } = render(<BreadcrumbDivider />);

    const divider = container.querySelector(".bg-gray-300");
    expect(divider).toBeInTheDocument();
  });

  it("is hidden on mobile", () => {
    const { container } = render(<BreadcrumbDivider />);

    const divider = container.querySelector(".hidden");
    expect(divider).toBeInTheDocument();
  });
});
