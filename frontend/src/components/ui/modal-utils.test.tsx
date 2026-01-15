import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import {
  dialogAriaProps,
  getModalAnimationClass,
  scrollableContentClassName,
  getContentAnimationClassName,
  getApiErrorMessage,
  renderModalCloseButton,
  renderModalLoadingSpinner,
  renderModalErrorAlert,
  renderButtonSpinner,
  ModalWrapper,
} from "./modal-utils";

// =============================================================================
// Pure Constants Tests
// =============================================================================

describe("dialogAriaProps", () => {
  it("should have correct role", () => {
    expect(dialogAriaProps.role).toBe("dialog");
  });

  it("should have aria-modal set to true", () => {
    expect(dialogAriaProps["aria-modal"]).toBe(true);
  });
});

describe("scrollableContentClassName", () => {
  it("should contain scrollbar styling classes", () => {
    expect(scrollableContentClassName).toContain("scrollbar-thin");
    expect(scrollableContentClassName).toContain("scrollbar-thumb-gray-300");
    expect(scrollableContentClassName).toContain("scrollbar-track-gray-100");
  });

  it("should contain max-height constraints", () => {
    expect(scrollableContentClassName).toContain("max-h-[calc(100vh-12rem)]");
    expect(scrollableContentClassName).toContain("overflow-y-auto");
  });
});

// =============================================================================
// Animation Class Functions Tests
// =============================================================================

describe("getModalAnimationClass", () => {
  it("returns enter animation when animating and not exiting", () => {
    expect(getModalAnimationClass(true, false)).toBe("animate-modalEnter");
  });

  it("returns exit animation when exiting (regardless of isAnimating)", () => {
    expect(getModalAnimationClass(true, true)).toBe("animate-modalExit");
    expect(getModalAnimationClass(false, true)).toBe("animate-modalExit");
  });

  it("returns initial hidden state when not animating and not exiting", () => {
    const result = getModalAnimationClass(false, false);
    expect(result).toContain("translate-y-8");
    expect(result).toContain("scale-75");
    expect(result).toContain("-rotate-1");
    expect(result).toContain("opacity-0");
  });
});

describe("getContentAnimationClassName", () => {
  it("returns reveal animation when animating and not exiting", () => {
    const result = getContentAnimationClassName(true, false);
    expect(result).toContain("p-4");
    expect(result).toContain("md:p-6");
    expect(result).toContain("sm:animate-contentReveal");
  });

  it("returns opacity-0 when exiting", () => {
    const result = getContentAnimationClassName(true, true);
    expect(result).toContain("sm:opacity-0");
    expect(result).not.toContain("sm:animate-contentReveal");
  });

  it("returns opacity-0 when not animating", () => {
    const result = getContentAnimationClassName(false, false);
    expect(result).toContain("sm:opacity-0");
    expect(result).not.toContain("sm:animate-contentReveal");
  });
});

// =============================================================================
// API Error Message Tests
// =============================================================================

describe("getApiErrorMessage", () => {
  const action = "erstellen";
  const entityType = "Aktivitäten";
  const defaultMessage = "Ein Fehler ist aufgetreten";

  it("returns default message for non-Error objects", () => {
    expect(getApiErrorMessage(null, action, entityType, defaultMessage)).toBe(
      defaultMessage,
    );
    expect(
      getApiErrorMessage(undefined, action, entityType, defaultMessage),
    ).toBe(defaultMessage);
    expect(
      getApiErrorMessage("string error", action, entityType, defaultMessage),
    ).toBe(defaultMessage);
    expect(getApiErrorMessage(123, action, entityType, defaultMessage)).toBe(
      defaultMessage,
    );
  });

  it("returns authentication message for 'user is not authenticated' error", () => {
    const error = new Error("user is not authenticated");
    const result = getApiErrorMessage(
      error,
      action,
      entityType,
      defaultMessage,
    );
    expect(result).toBe(
      "Sie müssen angemeldet sein, um Aktivitäten zu erstellen.",
    );
  });

  it("returns session expired message for 401 error", () => {
    const error = new Error("Request failed with status 401");
    const result = getApiErrorMessage(
      error,
      action,
      entityType,
      defaultMessage,
    );
    expect(result).toBe(
      "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.",
    );
  });

  it("returns access denied message for 403 error", () => {
    const error = new Error("Forbidden 403");
    const result = getApiErrorMessage(
      error,
      action,
      entityType,
      defaultMessage,
    );
    expect(result).toBe("Zugriff verweigert. Bitte melden Sie sich erneut an.");
  });

  it("returns invalid input message for 400 error", () => {
    const error = new Error("Bad request 400");
    const result = getApiErrorMessage(
      error,
      action,
      entityType,
      defaultMessage,
    );
    expect(result).toBe(
      "Ungültige Eingabedaten. Bitte überprüfen Sie Ihre Eingaben.",
    );
  });

  it("returns original error message for other errors", () => {
    const error = new Error("Network timeout");
    const result = getApiErrorMessage(
      error,
      action,
      entityType,
      defaultMessage,
    );
    expect(result).toBe("Network timeout");
  });

  it("uses dynamic action and entityType in authentication message", () => {
    const error = new Error("user is not authenticated");
    const result = getApiErrorMessage(
      error,
      "bearbeiten",
      "Gruppen",
      defaultMessage,
    );
    expect(result).toBe(
      "Sie müssen angemeldet sein, um Gruppen zu bearbeiten.",
    );
  });
});

// =============================================================================
// Render Functions Tests
// =============================================================================

describe("renderModalCloseButton", () => {
  it("renders a button with close icon", () => {
    const onClose = vi.fn();
    render(renderModalCloseButton({ onClose }));

    const button = screen.getByRole("button", { name: "Modal schließen" });
    expect(button).toBeInTheDocument();
  });

  it("calls onClose when clicked", () => {
    const onClose = vi.fn();
    render(renderModalCloseButton({ onClose }));

    fireEvent.click(screen.getByRole("button"));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("accepts custom aria-label", () => {
    const onClose = vi.fn();
    render(renderModalCloseButton({ onClose, ariaLabel: "Fenster schließen" }));

    expect(
      screen.getByRole("button", { name: "Fenster schließen" }),
    ).toBeInTheDocument();
  });

  it("renders SVG icon", () => {
    const onClose = vi.fn();
    render(renderModalCloseButton({ onClose }));

    const svg = document.querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveClass("h-5", "w-5");
  });
});

describe("renderModalLoadingSpinner", () => {
  it("renders with default message", () => {
    render(renderModalLoadingSpinner());

    expect(screen.getByText("Wird geladen...")).toBeInTheDocument();
  });

  it("renders with custom message", () => {
    render(renderModalLoadingSpinner({ message: "Daten werden geladen..." }));

    expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
    expect(screen.queryByText("Wird geladen...")).not.toBeInTheDocument();
  });

  it("renders spinner animation element", () => {
    render(renderModalLoadingSpinner());

    const spinner = document.querySelector(".animate-spin");
    expect(spinner).toBeInTheDocument();
  });
});

describe("renderModalErrorAlert", () => {
  it("renders error message", () => {
    render(renderModalErrorAlert({ message: "Test error message" }));

    expect(screen.getByText("Test error message")).toBeInTheDocument();
  });

  it("renders error icon", () => {
    render(renderModalErrorAlert({ message: "Error" }));

    const svg = document.querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveClass("text-red-600");
  });

  it("has proper styling classes", () => {
    const { container } = render(renderModalErrorAlert({ message: "Error" }));

    const alert = container.firstChild;
    expect(alert).toHaveClass("rounded-2xl", "border-red-100", "bg-red-50/80");
  });
});

describe("renderButtonSpinner", () => {
  it("renders spinning SVG", () => {
    render(renderButtonSpinner());

    const svg = document.querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveClass("animate-spin", "h-4", "w-4");
  });

  it("has circle and path elements", () => {
    render(renderButtonSpinner());

    expect(document.querySelector("circle")).toBeInTheDocument();
    expect(document.querySelector("path")).toBeInTheDocument();
  });
});

// =============================================================================
// ModalWrapper Component Tests
// =============================================================================

describe("ModalWrapper", () => {
  it("renders children content", () => {
    render(
      <ModalWrapper onClose={vi.fn()} isAnimating={true} isExiting={false}>
        <div>Modal Content</div>
      </ModalWrapper>,
    );

    expect(screen.getByText("Modal Content")).toBeInTheDocument();
  });

  it("renders backdrop button", () => {
    render(
      <ModalWrapper onClose={vi.fn()} isAnimating={true} isExiting={false}>
        <div>Content</div>
      </ModalWrapper>,
    );

    expect(
      screen.getByRole("button", { name: /hintergrund.*schließen/i }),
    ).toBeInTheDocument();
  });

  it("calls onClose when backdrop is clicked", () => {
    const onClose = vi.fn();
    render(
      <ModalWrapper onClose={onClose} isAnimating={true} isExiting={false}>
        <div>Content</div>
      </ModalWrapper>,
    );

    fireEvent.click(
      screen.getByRole("button", { name: /hintergrund.*schließen/i }),
    );
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("applies dialog aria props to container", () => {
    render(
      <ModalWrapper onClose={vi.fn()} isAnimating={true} isExiting={false}>
        <div>Content</div>
      </ModalWrapper>,
    );

    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAttribute("aria-modal", "true");
  });

  it("has fixed positioning for full-screen overlay", () => {
    const { container } = render(
      <ModalWrapper onClose={vi.fn()} isAnimating={true} isExiting={false}>
        <div>Content</div>
      </ModalWrapper>,
    );

    const wrapper = container.firstChild;
    expect(wrapper).toHaveClass("fixed", "inset-0", "z-[9999]");
  });

  it("applies enter animation class when animating", () => {
    render(
      <ModalWrapper onClose={vi.fn()} isAnimating={true} isExiting={false}>
        <div>Content</div>
      </ModalWrapper>,
    );

    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveClass("animate-modalEnter");
  });

  it("applies exit animation class when exiting", () => {
    render(
      <ModalWrapper onClose={vi.fn()} isAnimating={true} isExiting={true}>
        <div>Content</div>
      </ModalWrapper>,
    );

    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveClass("animate-modalExit");
  });

  it("applies initial state class when not animating", () => {
    render(
      <ModalWrapper onClose={vi.fn()} isAnimating={false} isExiting={false}>
        <div>Content</div>
      </ModalWrapper>,
    );

    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveClass("opacity-0");
  });
});
