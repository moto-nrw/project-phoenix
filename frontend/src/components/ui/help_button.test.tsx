import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { HelpButton } from "./help_button";

// Mock next/navigation
const mockPathname = vi.fn();
vi.mock("next/navigation", () => ({
  usePathname: (): string => mockPathname() as string,
}));

// Mock Modal component
vi.mock("./modal", () => ({
  Modal: ({
    isOpen,
    onClose,
    title,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <h2>{title}</h2>
        <div>{children}</div>
        <button onClick={onClose}>Close Modal</button>
      </div>
    ) : null,
}));

describe("HelpButton", () => {
  it("renders the help button", () => {
    mockPathname.mockReturnValue("/dashboard");

    render(<HelpButton title="Help" content="Help content" />);

    const button = screen.getByLabelText("Hilfe anzeigen");
    expect(button).toBeInTheDocument();
  });

  it("renders the help icon", () => {
    mockPathname.mockReturnValue("/dashboard");

    const { container } = render(
      <HelpButton title="Help" content="Help content" />,
    );

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("opens modal on button click", () => {
    mockPathname.mockReturnValue("/dashboard");

    render(<HelpButton title="Help Title" content="Help content here" />);

    const button = screen.getByLabelText("Hilfe anzeigen");
    fireEvent.click(button);

    expect(screen.getByTestId("modal")).toBeInTheDocument();
    expect(screen.getByText("Help Title")).toBeInTheDocument();
    expect(screen.getByText("Help content here")).toBeInTheDocument();
  });

  it("closes modal on close button click", () => {
    mockPathname.mockReturnValue("/dashboard");

    render(<HelpButton title="Help" content="Content" />);

    const button = screen.getByLabelText("Hilfe anzeigen");
    fireEvent.click(button);

    expect(screen.getByTestId("modal")).toBeInTheDocument();

    const closeButton = screen.getByText("Close Modal");
    fireEvent.click(closeButton);

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("shows Impressum link on login page", () => {
    mockPathname.mockReturnValue("/");

    render(<HelpButton title="Help" content="Content" />);

    const button = screen.getByLabelText("Hilfe anzeigen");
    fireEvent.click(button);

    const impressumLink = screen.getByText("Impressum");
    expect(impressumLink).toBeInTheDocument();
    expect(impressumLink.closest("a")).toHaveAttribute(
      "href",
      "https://moto.nrw/impressum/",
    );
  });

  it("does not show Impressum link on non-login pages", () => {
    mockPathname.mockReturnValue("/dashboard");

    render(<HelpButton title="Help" content="Content" />);

    const button = screen.getByLabelText("Hilfe anzeigen");
    fireEvent.click(button);

    expect(screen.queryByText("Impressum")).not.toBeInTheDocument();
  });

  it("applies custom button className", () => {
    mockPathname.mockReturnValue("/dashboard");

    render(
      <HelpButton
        title="Help"
        content="Content"
        buttonClassName="custom-class"
      />,
    );

    const button = screen.getByLabelText("Hilfe anzeigen");
    expect(button).toHaveClass("custom-class");
  });

  it("renders content as ReactNode", () => {
    mockPathname.mockReturnValue("/dashboard");

    render(
      <HelpButton
        title="Help"
        content={
          <div>
            <p>Paragraph 1</p>
            <p>Paragraph 2</p>
          </div>
        }
      />,
    );

    const button = screen.getByLabelText("Hilfe anzeigen");
    fireEvent.click(button);

    expect(screen.getByText("Paragraph 1")).toBeInTheDocument();
    expect(screen.getByText("Paragraph 2")).toBeInTheDocument();
  });
});
