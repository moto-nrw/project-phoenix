import { render, screen } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it } from "vitest";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "./card";

describe("Card", () => {
  describe("Card component", () => {
    it("renders with default styles", () => {
      render(<Card data-testid="card">Card content</Card>);
      const card = screen.getByTestId("card");
      expect(card).toBeInTheDocument();
      expect(card).toHaveTextContent("Card content");
      expect(card).toHaveClass(
        "rounded-xl",
        "border",
        "border-gray-200",
        "bg-white",
        "shadow-sm",
      );
    });

    it("forwards ref correctly", () => {
      const ref = createRef<HTMLDivElement>();
      render(<Card ref={ref}>Card</Card>);
      expect(ref.current).toBeInstanceOf(HTMLDivElement);
    });

    it("applies custom className", () => {
      render(
        <Card className="custom-class" data-testid="card">
          Card
        </Card>,
      );
      const card = screen.getByTestId("card");
      expect(card).toHaveClass("custom-class");
    });

    it("spreads additional props", () => {
      render(
        <Card id="my-card" title="Card Title" data-testid="card">
          Card
        </Card>,
      );
      const card = screen.getByTestId("card");
      expect(card).toHaveAttribute("id", "my-card");
      expect(card).toHaveAttribute("title", "Card Title");
    });

    it("has correct displayName", () => {
      expect(Card.displayName).toBe("Card");
    });
  });

  describe("CardHeader component", () => {
    it("renders with default styles", () => {
      render(<CardHeader data-testid="header">Header content</CardHeader>);
      const header = screen.getByTestId("header");
      expect(header).toBeInTheDocument();
      expect(header).toHaveTextContent("Header content");
      expect(header).toHaveClass("flex", "flex-col", "space-y-1.5", "p-6");
    });

    it("forwards ref correctly", () => {
      const ref = createRef<HTMLDivElement>();
      render(<CardHeader ref={ref}>Header</CardHeader>);
      expect(ref.current).toBeInstanceOf(HTMLDivElement);
    });

    it("applies custom className", () => {
      render(
        <CardHeader className="custom-header" data-testid="header">
          Header
        </CardHeader>,
      );
      const header = screen.getByTestId("header");
      expect(header).toHaveClass("custom-header");
    });

    it("has correct displayName", () => {
      expect(CardHeader.displayName).toBe("CardHeader");
    });
  });

  describe("CardTitle component", () => {
    it("renders as h3 with default styles", () => {
      render(<CardTitle>Title text</CardTitle>);
      const title = screen.getByRole("heading", { level: 3 });
      expect(title).toBeInTheDocument();
      expect(title).toHaveTextContent("Title text");
      expect(title).toHaveClass(
        "text-lg",
        "leading-none",
        "font-semibold",
        "tracking-tight",
      );
    });

    it("forwards ref correctly", () => {
      const ref = createRef<HTMLHeadingElement>();
      render(<CardTitle ref={ref}>Title</CardTitle>);
      expect(ref.current).toBeInstanceOf(HTMLHeadingElement);
    });

    it("applies custom className", () => {
      render(<CardTitle className="custom-title">Title</CardTitle>);
      const title = screen.getByRole("heading", { level: 3 });
      expect(title).toHaveClass("custom-title");
    });

    it("has correct displayName", () => {
      expect(CardTitle.displayName).toBe("CardTitle");
    });
  });

  describe("CardDescription component", () => {
    it("renders as paragraph with default styles", () => {
      render(
        <CardDescription data-testid="description">
          Description text
        </CardDescription>,
      );
      const description = screen.getByTestId("description");
      expect(description).toBeInTheDocument();
      expect(description).toHaveTextContent("Description text");
      expect(description.tagName).toBe("P");
      expect(description).toHaveClass("text-sm", "text-gray-500");
    });

    it("forwards ref correctly", () => {
      const ref = createRef<HTMLParagraphElement>();
      render(<CardDescription ref={ref}>Description</CardDescription>);
      expect(ref.current).toBeInstanceOf(HTMLParagraphElement);
    });

    it("applies custom className", () => {
      render(
        <CardDescription className="custom-desc" data-testid="description">
          Description
        </CardDescription>,
      );
      const description = screen.getByTestId("description");
      expect(description).toHaveClass("custom-desc");
    });

    it("has correct displayName", () => {
      expect(CardDescription.displayName).toBe("CardDescription");
    });
  });

  describe("CardContent component", () => {
    it("renders with default styles", () => {
      render(<CardContent data-testid="content">Content text</CardContent>);
      const content = screen.getByTestId("content");
      expect(content).toBeInTheDocument();
      expect(content).toHaveTextContent("Content text");
      expect(content).toHaveClass("p-6", "pt-0");
    });

    it("forwards ref correctly", () => {
      const ref = createRef<HTMLDivElement>();
      render(<CardContent ref={ref}>Content</CardContent>);
      expect(ref.current).toBeInstanceOf(HTMLDivElement);
    });

    it("applies custom className", () => {
      render(
        <CardContent className="custom-content" data-testid="content">
          Content
        </CardContent>,
      );
      const content = screen.getByTestId("content");
      expect(content).toHaveClass("custom-content");
    });

    it("has correct displayName", () => {
      expect(CardContent.displayName).toBe("CardContent");
    });
  });

  describe("CardFooter component", () => {
    it("renders with default styles", () => {
      render(<CardFooter data-testid="footer">Footer content</CardFooter>);
      const footer = screen.getByTestId("footer");
      expect(footer).toBeInTheDocument();
      expect(footer).toHaveTextContent("Footer content");
      expect(footer).toHaveClass("flex", "items-center", "p-6", "pt-0");
    });

    it("forwards ref correctly", () => {
      const ref = createRef<HTMLDivElement>();
      render(<CardFooter ref={ref}>Footer</CardFooter>);
      expect(ref.current).toBeInstanceOf(HTMLDivElement);
    });

    it("applies custom className", () => {
      render(
        <CardFooter className="custom-footer" data-testid="footer">
          Footer
        </CardFooter>,
      );
      const footer = screen.getByTestId("footer");
      expect(footer).toHaveClass("custom-footer");
    });

    it("has correct displayName", () => {
      expect(CardFooter.displayName).toBe("CardFooter");
    });
  });

  describe("composition", () => {
    it("renders a complete card with all sub-components", () => {
      render(
        <Card data-testid="full-card">
          <CardHeader>
            <CardTitle>Card Title</CardTitle>
            <CardDescription>Card description text</CardDescription>
          </CardHeader>
          <CardContent>Main content goes here</CardContent>
          <CardFooter>Footer actions</CardFooter>
        </Card>,
      );

      expect(screen.getByTestId("full-card")).toBeInTheDocument();
      expect(
        screen.getByRole("heading", { name: "Card Title" }),
      ).toBeInTheDocument();
      expect(screen.getByText("Card description text")).toBeInTheDocument();
      expect(screen.getByText("Main content goes here")).toBeInTheDocument();
      expect(screen.getByText("Footer actions")).toBeInTheDocument();
    });
  });
});
