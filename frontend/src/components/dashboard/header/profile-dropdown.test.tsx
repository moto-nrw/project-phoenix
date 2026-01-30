/**
 * Tests for ProfileDropdown Components
 * Tests profile trigger and dropdown menu functionality
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { ProfileTrigger, ProfileDropdownMenu } from "./profile-dropdown";

// Mock next/link
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    onClick,
    className,
  }: {
    children: React.ReactNode;
    href: string;
    onClick?: () => void;
    className?: string;
  }) => (
    <a href={href} onClick={onClick} className={className}>
      {children}
    </a>
  ),
}));

// Mock next/image
vi.mock("next/image", () => ({
  default: ({ src, alt }: { src: string; alt: string }) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img src={src} alt={alt} />
  ),
}));

describe("ProfileTrigger", () => {
  it("renders user name and role", () => {
    const onClick = vi.fn();
    render(
      <ProfileTrigger
        displayName="Max Mustermann"
        userRole="Betreuer"
        isOpen={false}
        onClick={onClick}
      />,
    );

    expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
    expect(screen.getByText("Betreuer")).toBeInTheDocument();
  });

  it("calls onClick when clicked", () => {
    const onClick = vi.fn();
    render(
      <ProfileTrigger
        displayName="Max Mustermann"
        userRole="Betreuer"
        isOpen={false}
        onClick={onClick}
      />,
    );

    fireEvent.click(screen.getByRole("button"));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("shows chevron down icon", () => {
    const onClick = vi.fn();
    const { container } = render(
      <ProfileTrigger
        displayName="Max Mustermann"
        userRole="Betreuer"
        isOpen={false}
        onClick={onClick}
      />,
    );

    const chevron = container.querySelector("svg");
    expect(chevron).toBeInTheDocument();
  });

  it("rotates chevron when open", () => {
    const onClick = vi.fn();
    const { container } = render(
      <ProfileTrigger
        displayName="Max Mustermann"
        userRole="Betreuer"
        isOpen={true}
        onClick={onClick}
      />,
    );

    const chevron = container.querySelector(".rotate-180");
    expect(chevron).toBeInTheDocument();
  });

  it("renders avatar when provided", () => {
    const onClick = vi.fn();
    render(
      <ProfileTrigger
        displayName="Max Mustermann"
        displayAvatar="/avatar.jpg"
        userRole="Betreuer"
        isOpen={false}
        onClick={onClick}
      />,
    );

    const avatar = screen.getByAltText("Max Mustermann");
    expect(avatar).toBeInTheDocument();
  });

  it("renders initials when no avatar", () => {
    const onClick = vi.fn();
    const { container } = render(
      <ProfileTrigger
        displayName="Max Mustermann"
        userRole="Betreuer"
        isOpen={false}
        onClick={onClick}
      />,
    );

    // Should render "MM" as initials
    expect(container.textContent).toContain("MM");
  });
});

describe("ProfileDropdownMenu", () => {
  it("is hidden when closed", () => {
    const onClose = vi.fn();
    const onLogout = vi.fn();
    const { container } = render(
      <ProfileDropdownMenu
        isOpen={false}
        displayName="Max Mustermann"
        userEmail="max@example.com"
        onClose={onClose}
        onLogout={onLogout}
      />,
    );

    const menu = container.querySelector(".invisible");
    expect(menu).toBeInTheDocument();
  });

  it("is visible when open", () => {
    const onClose = vi.fn();
    const onLogout = vi.fn();
    const { container } = render(
      <ProfileDropdownMenu
        isOpen={true}
        displayName="Max Mustermann"
        userEmail="max@example.com"
        onClose={onClose}
        onLogout={onLogout}
      />,
    );

    const menu = container.querySelector(".visible");
    expect(menu).toBeInTheDocument();
  });

  it("displays user info", () => {
    const onClose = vi.fn();
    const onLogout = vi.fn();
    render(
      <ProfileDropdownMenu
        isOpen={true}
        displayName="Max Mustermann"
        userEmail="max@example.com"
        onClose={onClose}
        onLogout={onLogout}
      />,
    );

    expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
    expect(screen.getByText("max@example.com")).toBeInTheDocument();
  });

  it("renders settings link", () => {
    const onClose = vi.fn();
    const onLogout = vi.fn();
    render(
      <ProfileDropdownMenu
        isOpen={true}
        displayName="Max Mustermann"
        userEmail="max@example.com"
        onClose={onClose}
        onLogout={onLogout}
      />,
    );

    const settingsLink = screen.getByRole("link", { name: /einstellungen/i });
    expect(settingsLink).toHaveAttribute("href", "/settings");
  });

  it("renders help button", () => {
    const onClose = vi.fn();
    const onLogout = vi.fn();
    render(
      <ProfileDropdownMenu
        isOpen={true}
        displayName="Max Mustermann"
        userEmail="max@example.com"
        onClose={onClose}
        onLogout={onLogout}
      />,
    );

    expect(
      screen.getByRole("button", { name: /hilfe & support/i }),
    ).toBeInTheDocument();
  });

  it("renders logout button", () => {
    const onClose = vi.fn();
    const onLogout = vi.fn();
    render(
      <ProfileDropdownMenu
        isOpen={true}
        displayName="Max Mustermann"
        userEmail="max@example.com"
        onClose={onClose}
        onLogout={onLogout}
      />,
    );

    expect(
      screen.getByRole("button", { name: /abmelden/i }),
    ).toBeInTheDocument();
  });

  it("calls onLogout when logout clicked", () => {
    const onClose = vi.fn();
    const onLogout = vi.fn();
    render(
      <ProfileDropdownMenu
        isOpen={true}
        displayName="Max Mustermann"
        userEmail="max@example.com"
        onClose={onClose}
        onLogout={onLogout}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /abmelden/i }));
    expect(onClose).toHaveBeenCalled();
    expect(onLogout).toHaveBeenCalled();
  });

  it("calls onClose when settings clicked", () => {
    const onClose = vi.fn();
    const onLogout = vi.fn();
    render(
      <ProfileDropdownMenu
        isOpen={true}
        displayName="Max Mustermann"
        userEmail="max@example.com"
        onClose={onClose}
        onLogout={onLogout}
      />,
    );

    fireEvent.click(screen.getByRole("link", { name: /einstellungen/i }));
    expect(onClose).toHaveBeenCalled();
  });

  it("calls onClose when help button clicked", () => {
    const onClose = vi.fn();
    const onLogout = vi.fn();

    render(
      <ProfileDropdownMenu
        isOpen={true}
        displayName="Max Mustermann"
        userEmail="max@example.com"
        onClose={onClose}
        onLogout={onLogout}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /hilfe & support/i }));
    expect(onClose).toHaveBeenCalled();
  });

  it("renders backdrop on mobile when open", () => {
    const onClose = vi.fn();
    const onLogout = vi.fn();
    render(
      <ProfileDropdownMenu
        isOpen={true}
        displayName="Max Mustermann"
        userEmail="max@example.com"
        onClose={onClose}
        onLogout={onLogout}
      />,
    );

    const backdrop = screen.getByLabelText("Menü schließen");
    expect(backdrop).toBeInTheDocument();
  });

  it("closes when backdrop clicked", () => {
    const onClose = vi.fn();
    const onLogout = vi.fn();
    render(
      <ProfileDropdownMenu
        isOpen={true}
        displayName="Max Mustermann"
        userEmail="max@example.com"
        onClose={onClose}
        onLogout={onLogout}
      />,
    );

    fireEvent.click(screen.getByLabelText("Menü schließen"));
    expect(onClose).toHaveBeenCalled();
  });
});
