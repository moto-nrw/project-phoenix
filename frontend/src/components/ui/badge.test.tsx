import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Badge } from "./badge";

describe("Badge", () => {
  describe("rendering", () => {
    it("renders with default variant", () => {
      render(<Badge data-testid="badge">Default</Badge>);
      const badge = screen.getByTestId("badge");
      expect(badge).toBeInTheDocument();
      expect(badge).toHaveTextContent("Default");
      expect(badge).toHaveClass("bg-gray-100", "text-gray-800");
    });

    it("renders children content", () => {
      render(<Badge>Content Text</Badge>);
      expect(screen.getByText("Content Text")).toBeInTheDocument();
    });
  });

  describe("variants", () => {
    it("applies default variant styling", () => {
      render(<Badge variant="default" data-testid="badge" />);
      const badge = screen.getByTestId("badge");
      expect(badge).toHaveClass("bg-gray-100", "text-gray-800");
    });

    it("applies success variant styling", () => {
      render(<Badge variant="success" data-testid="badge" />);
      const badge = screen.getByTestId("badge");
      expect(badge).toHaveClass("bg-green-100", "text-green-800");
    });

    it("applies warning variant styling", () => {
      render(<Badge variant="warning" data-testid="badge" />);
      const badge = screen.getByTestId("badge");
      expect(badge).toHaveClass("bg-yellow-100", "text-yellow-800");
    });

    it("applies danger variant styling", () => {
      render(<Badge variant="danger" data-testid="badge" />);
      const badge = screen.getByTestId("badge");
      expect(badge).toHaveClass("bg-red-100", "text-red-800");
    });

    it("applies secondary variant styling", () => {
      render(<Badge variant="secondary" data-testid="badge" />);
      const badge = screen.getByTestId("badge");
      expect(badge).toHaveClass("bg-gray-200", "text-gray-700");
    });
  });

  describe("props forwarding", () => {
    it("applies custom className", () => {
      render(<Badge className="custom-class" data-testid="badge" />);
      const badge = screen.getByTestId("badge");
      expect(badge).toHaveClass("custom-class");
    });

    it("merges custom className with variant classes", () => {
      render(
        <Badge variant="success" className="extra-style" data-testid="badge" />,
      );
      const badge = screen.getByTestId("badge");
      expect(badge).toHaveClass("bg-green-100", "extra-style");
    });

    it("spreads additional HTML attributes", () => {
      render(<Badge id="my-badge" title="Test Title" data-testid="badge" />);
      const badge = screen.getByTestId("badge");
      expect(badge).toHaveAttribute("id", "my-badge");
      expect(badge).toHaveAttribute("title", "Test Title");
    });

    it("forwards data attributes", () => {
      render(<Badge data-custom="value" data-testid="badge" />);
      const badge = screen.getByTestId("badge");
      expect(badge).toHaveAttribute("data-custom", "value");
    });
  });

  describe("base styles", () => {
    it("applies base styling classes", () => {
      render(<Badge data-testid="badge" />);
      const badge = screen.getByTestId("badge");
      expect(badge).toHaveClass(
        "inline-flex",
        "items-center",
        "rounded-full",
        "px-2.5",
        "py-0.5",
        "text-xs",
        "font-medium",
        "transition-colors",
      );
    });
  });
});
