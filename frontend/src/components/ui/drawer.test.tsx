import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
  DrawerDescription,
} from "./drawer";

// Mock vaul library
vi.mock("vaul", () => ({
  Drawer: {
    Root: ({ children }: { children: React.ReactNode }) => (
      <div data-testid="drawer-root">{children}</div>
    ),
    Portal: ({ children }: { children: React.ReactNode }) => (
      <div data-testid="drawer-portal">{children}</div>
    ),
    Overlay: ({ children, ...props }: { children?: React.ReactNode }) => (
      <div data-testid="drawer-overlay" {...props}>
        {children}
      </div>
    ),
    Content: ({ children, ...props }: { children: React.ReactNode }) => (
      <div data-testid="drawer-content" {...props}>
        {children}
      </div>
    ),
    Title: ({ children, ...props }: { children: React.ReactNode }) => (
      <div data-testid="drawer-title" {...props}>
        {children}
      </div>
    ),
    Description: ({ children, ...props }: { children: React.ReactNode }) => (
      <div data-testid="drawer-description" {...props}>
        {children}
      </div>
    ),
  },
}));

// Mock cn utility
vi.mock("~/lib/utils", () => ({
  cn: (...classes: (string | undefined)[]) => classes.filter(Boolean).join(" "),
}));

describe("Drawer", () => {
  it("renders Drawer component", () => {
    render(
      <Drawer>
        <div>Content</div>
      </Drawer>,
    );

    expect(screen.getByTestId("drawer-root")).toBeInTheDocument();
  });

  it("passes shouldScaleBackground prop", () => {
    render(
      <Drawer shouldScaleBackground={false}>
        <div>Content</div>
      </Drawer>,
    );

    expect(screen.getByTestId("drawer-root")).toBeInTheDocument();
  });
});

describe("DrawerContent", () => {
  it("renders DrawerContent with children", () => {
    render(
      <DrawerContent>
        <div>Drawer content</div>
      </DrawerContent>,
    );

    expect(screen.getByTestId("drawer-portal")).toBeInTheDocument();
    expect(screen.getByTestId("drawer-overlay")).toBeInTheDocument();
    expect(screen.getByTestId("drawer-content")).toBeInTheDocument();
    expect(screen.getByText("Drawer content")).toBeInTheDocument();
  });

  it("renders iOS-style drag handle", () => {
    const { container } = render(
      <DrawerContent>
        <div>Content</div>
      </DrawerContent>,
    );

    const handle = container.querySelector(
      ".h-1.w-12.rounded-full.bg-gray-300",
    );
    expect(handle).toBeInTheDocument();
  });

  it("applies custom className", () => {
    render(
      <DrawerContent className="custom-class">
        <div>Content</div>
      </DrawerContent>,
    );

    const content = screen.getByTestId("drawer-content");
    expect(content).toHaveClass("custom-class");
  });
});

describe("DrawerHeader", () => {
  it("renders DrawerHeader with children", () => {
    render(
      <DrawerHeader>
        <div>Header content</div>
      </DrawerHeader>,
    );

    expect(screen.getByText("Header content")).toBeInTheDocument();
  });

  it("applies default classes", () => {
    const { container } = render(
      <DrawerHeader>
        <div>Header</div>
      </DrawerHeader>,
    );

    const header = container.querySelector(".grid.gap-1\\.5.p-4");
    expect(header).toBeInTheDocument();
  });
});

describe("DrawerTitle", () => {
  it("renders DrawerTitle with text", () => {
    render(<DrawerTitle>Title Text</DrawerTitle>);

    expect(screen.getByTestId("drawer-title")).toBeInTheDocument();
    expect(screen.getByText("Title Text")).toBeInTheDocument();
  });

  it("applies custom className", () => {
    render(<DrawerTitle className="custom-title">Title</DrawerTitle>);

    const title = screen.getByTestId("drawer-title");
    expect(title).toHaveClass("custom-title");
  });
});

describe("DrawerDescription", () => {
  it("renders DrawerDescription with text", () => {
    render(<DrawerDescription>Description text</DrawerDescription>);

    expect(screen.getByTestId("drawer-description")).toBeInTheDocument();
    expect(screen.getByText("Description text")).toBeInTheDocument();
  });

  it("applies custom className", () => {
    render(
      <DrawerDescription className="custom-description">
        Description
      </DrawerDescription>,
    );

    const description = screen.getByTestId("drawer-description");
    expect(description).toHaveClass("custom-description");
  });
});
