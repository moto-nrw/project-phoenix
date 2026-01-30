import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { PageHeader } from "./page-header";

// Mock next/link
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
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
    <img
      src={src}
      alt={alt}
      width={width}
      height={height}
      data-testid="next-image"
    />
  ),
}));

// Mock Button component
vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    variant,
    className,
    ...props
  }: {
    children: React.ReactNode;
    variant?: string;
    className?: string;
  }) => (
    <button data-variant={variant} className={className} {...props}>
      {children}
    </button>
  ),
}));

describe("PageHeader", () => {
  it("renders the title", () => {
    render(<PageHeader title="Dashboard" />);

    expect(screen.getByText("Dashboard")).toBeInTheDocument();
  });

  it("renders the logo", () => {
    render(<PageHeader title="Dashboard" />);

    const logo = screen.getByAltText("Logo");
    expect(logo).toBeInTheDocument();
    expect(logo).toHaveAttribute("src", "/images/moto_transparent.png");
  });

  it("renders description when provided", () => {
    render(
      <PageHeader title="Dashboard" description="Welcome to the dashboard" />,
    );

    expect(screen.getByText("Welcome to the dashboard")).toBeInTheDocument();
  });

  it("does not render description when not provided", () => {
    const { container } = render(<PageHeader title="Dashboard" />);

    const description = container.querySelector(".text-gray-500");
    expect(description).not.toBeInTheDocument();
  });

  it("renders back button with default URL", () => {
    render(<PageHeader title="Dashboard" />);

    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "/dashboard");
  });

  it("renders back button with custom URL", () => {
    render(<PageHeader title="Groups" backUrl="/overview" />);

    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "/overview");
  });

  it("renders back button text", () => {
    render(<PageHeader title="Dashboard" />);

    expect(screen.getByText("Zurück")).toBeInTheDocument();
  });

  it("renders back button icon", () => {
    const { container } = render(<PageHeader title="Dashboard" />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("accepts ReactNode as title", () => {
    render(
      <PageHeader
        title={
          <span>
            Custom <strong>Title</strong>
          </span>
        }
      />,
    );

    expect(screen.getByText("Custom")).toBeInTheDocument();
    expect(screen.getByText("Title")).toBeInTheDocument();
  });
});
