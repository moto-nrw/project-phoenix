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
  it("keeps an accessible name when the visible name is hidden on mobile", () => {
    render(
      <ProfileTrigger
        displayName="Administrator Operator"
        ariaLabel="Profile menu for Administrator Operator"
        userRole="Operator"
        isOpen={false}
        onClick={vi.fn()}
        menuId="profile-menu"
      />,
    );

    expect(
      screen.getByRole("button", {
        name: "Profile menu for Administrator Operator",
      }),
    ).toHaveAttribute("aria-controls", "profile-menu");
  });

  it("renders user name and role", () => {
    const onClick = vi.fn();
    render(
      <ProfileTrigger
        displayName="Max Mustermann"
        ariaLabel="Profilmenü von Max Mustermann"
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
        ariaLabel="Profilmenü von Max Mustermann"
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
        ariaLabel="Profilmenü von Max Mustermann"
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
        ariaLabel="Profilmenü von Max Mustermann"
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
        ariaLabel="Profilmenü von Max Mustermann"
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
        ariaLabel="Profilmenü von Max Mustermann"
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

  it("renders profile link when profileUrl is provided", () => {
    const onClose = vi.fn();
    const onLogout = vi.fn();
    render(
      <ProfileDropdownMenu
        isOpen={true}
        displayName="Max Mustermann"
        userEmail="max@example.com"
        onClose={onClose}
        onLogout={onLogout}
        profileUrl="/profile"
      />,
    );

    const profileLink = screen.getByRole("link", { name: /profil/i });
    expect(profileLink).toHaveAttribute("href", "/profile");
  });

  it("renders a custom profile link label when provided", () => {
    const onClose = vi.fn();
    const onLogout = vi.fn();
    render(
      <ProfileDropdownMenu
        isOpen={true}
        displayName="Max Mustermann"
        userEmail="max@example.com"
        onClose={onClose}
        onLogout={onLogout}
        profileUrl="/operator/settings"
        profileLabel="Profileinstellungen"
      />,
    );

    const profileLink = screen.getByRole("link", {
      name: /profileinstellungen/i,
    });
    expect(profileLink).toHaveAttribute("href", "/operator/settings");
  });

  it("does not render profile link when profileUrl is not provided", () => {
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
      screen.queryByRole("link", { name: /profil/i }),
    ).not.toBeInTheDocument();
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

  it("calls onClose when profile clicked", () => {
    const onClose = vi.fn();
    const onLogout = vi.fn();
    render(
      <ProfileDropdownMenu
        isOpen={true}
        displayName="Max Mustermann"
        userEmail="max@example.com"
        onClose={onClose}
        onLogout={onLogout}
        profileUrl="/profile"
      />,
    );

    fireEvent.click(screen.getByRole("link", { name: /profil/i }));
    expect(onClose).toHaveBeenCalled();
  });

  it("does not render its own full-screen dismissal layer", () => {
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

    expect(screen.queryByLabelText("Menü schließen")).not.toBeInTheDocument();
  });
});
