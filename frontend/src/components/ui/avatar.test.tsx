// Avatar fallback regressions. The component renders next/image when an
// imageUrl is provided and falls back to deterministic initials otherwise.
// What this test pins: when the URL is present but the image FAILS to
// load — stale photo URL after another tab deleted the photo, feature was
// disabled, or the file was unlinked — the component must drop to the
// initials fallback instead of rendering the browser's broken-image glyph.
//
// Real-world hit cases driving this contract:
//   - Card renders with the old /api/students/.../photo/{name} URL while
//     SWR hasn't revalidated yet → backend 404 → broken avatar across
//     every student surface using <Avatar />.
//   - Operator disables the photo feature → the serve route returns 403
//     for cached URLs until TenantContext re-resolves.
//
// Also covers the reset behavior: a fresh imageUrl after a previous
// failure must clear the failed state, so re-uploading a student's photo
// (or remounting the card with a new student) doesn't keep showing the
// gradient when the new image is fine.

import { describe, it, expect } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { Avatar } from "./avatar";

describe("Avatar", () => {
  it("renders the image when a URL is provided and loads ok", () => {
    render(
      <Avatar imageUrl="/uploads/student-photos/a.jpg" name="Alice Bee" />,
    );
    const img = screen.getByRole("img");
    expect(img).toBeTruthy();
    // No initials text — the image is showing.
    expect(screen.queryByText("AB")).toBeNull();
  });

  it("falls back to initials when the image fails to load", () => {
    render(
      <Avatar
        imageUrl="/uploads/student-photos/missing.jpg"
        name="Alice Bee"
      />,
    );
    const img = screen.getByRole("img");
    fireEvent.error(img);
    // After the error event the image is gone and initials are visible.
    expect(screen.queryByRole("img")).toBeNull();
    expect(screen.getByText("AB")).toBeTruthy();
  });

  it("renders initials directly when imageUrl is empty", () => {
    render(<Avatar imageUrl="" name="Alice Bee" />);
    expect(screen.queryByRole("img")).toBeNull();
    expect(screen.getByText("AB")).toBeTruthy();
  });

  it("renders initials directly when imageUrl is null", () => {
    render(<Avatar imageUrl={null} name="Alice Bee" />);
    expect(screen.queryByRole("img")).toBeNull();
    expect(screen.getByText("AB")).toBeTruthy();
  });

  it("can render a decorative rounded initials avatar", () => {
    render(
      <Avatar
        name="Alice Bee"
        size="md"
        shape="rounded"
        decorative
        className="h-10 w-10"
      />,
    );

    const avatar = screen.getByText("AB");
    expect(avatar).toHaveAttribute("aria-hidden", "true");
    expect(avatar).not.toHaveAttribute("aria-label");
    expect(avatar.className).toContain("rounded-xl");
    expect(avatar.className).toContain("h-10");
    expect(avatar.className).not.toContain("h-11");
  });

  it("resets the failed state when imageUrl changes", () => {
    const { rerender } = render(
      <Avatar imageUrl="/uploads/student-photos/old.jpg" name="Alice Bee" />,
    );
    fireEvent.error(screen.getByRole("img"));
    expect(screen.getByText("AB")).toBeTruthy();

    // New URL — failed state must clear so the new image gets a chance.
    rerender(
      <Avatar imageUrl="/uploads/student-photos/new.jpg" name="Alice Bee" />,
    );
    expect(screen.getByRole("img")).toBeTruthy();
    expect(screen.queryByText("AB")).toBeNull();
  });
});
