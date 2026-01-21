import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { SimpleAlert } from "./SimpleAlert";

vi.mock("~/contexts/AlertContext", () => ({
  AlertContext: {
    Provider: ({ children }: { children: React.ReactNode }) => children,
    Consumer: ({ children }: { children: (value: null) => React.ReactNode }) =>
      children(null),
  },
}));

describe("SimpleAlert", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders success alert with message", async () => {
    render(<SimpleAlert type="success" message="Operation successful" />);

    await act(async () => {
      vi.advanceTimersByTime(50);
    });

    expect(screen.getByText("Operation successful")).toBeInTheDocument();
  });

  it("renders error alert with message", async () => {
    render(<SimpleAlert type="error" message="Something went wrong" />);

    await act(async () => {
      vi.advanceTimersByTime(50);
    });

    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
  });

  it("renders info alert with message", async () => {
    render(<SimpleAlert type="info" message="Information here" />);

    await act(async () => {
      vi.advanceTimersByTime(50);
    });

    expect(screen.getByText("Information here")).toBeInTheDocument();
  });

  it("renders warning alert with message", async () => {
    render(<SimpleAlert type="warning" message="Warning message" />);

    await act(async () => {
      vi.advanceTimersByTime(50);
    });

    expect(screen.getByText("Warning message")).toBeInTheDocument();
  });

  it("shows close button when onClose is provided", async () => {
    const onClose = vi.fn();
    render(
      <SimpleAlert type="success" message="Test" onClose={onClose} />,
    );

    await act(async () => {
      vi.advanceTimersByTime(50);
    });

    const closeButton = screen.getByRole("button");
    expect(closeButton).toBeInTheDocument();
  });

  it("calls onClose when close button clicked", async () => {
    const onClose = vi.fn();
    render(
      <SimpleAlert type="success" message="Test" onClose={onClose} />,
    );

    await act(async () => {
      vi.advanceTimersByTime(50);
    });

    const closeButton = screen.getByRole("button");
    fireEvent.click(closeButton);

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("hides close button when onClose is not provided", async () => {
    render(<SimpleAlert type="success" message="Test" />);

    await act(async () => {
      vi.advanceTimersByTime(50);
    });

    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("auto closes after duration when autoClose is true", async () => {
    const onClose = vi.fn();
    render(
      <SimpleAlert
        type="success"
        message="Test"
        onClose={onClose}
        autoClose={true}
        duration={1000}
      />,
    );

    // Wait for entrance animation
    await act(async () => {
      vi.advanceTimersByTime(50);
    });

    expect(onClose).not.toHaveBeenCalled();

    // Wait for auto close + exit animation
    await act(async () => {
      vi.advanceTimersByTime(1000);
    });

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("renders progress bar when autoClose is true", async () => {
    const { container } = render(
      <SimpleAlert
        type="success"
        message="Test"
        autoClose={true}
        duration={3000}
      />,
    );

    await act(async () => {
      vi.advanceTimersByTime(50);
    });

    const progressBar = container.querySelector("[style*='animation']");
    expect(progressBar).toBeInTheDocument();
  });

  it("does not render progress bar when autoClose is false", async () => {
    const { container } = render(
      <SimpleAlert type="success" message="Test" autoClose={false} />,
    );

    await act(async () => {
      vi.advanceTimersByTime(50);
    });

    const progressBar = container.querySelector("[style*='animation: shrink']");
    expect(progressBar).not.toBeInTheDocument();
  });

  it("uses default duration of 3000ms", async () => {
    const onClose = vi.fn();
    render(
      <SimpleAlert
        type="success"
        message="Test"
        onClose={onClose}
        autoClose={true}
      />,
    );

    await act(async () => {
      vi.advanceTimersByTime(50);
    });

    // Should not close before 3000ms
    await act(async () => {
      vi.advanceTimersByTime(2500);
    });
    expect(onClose).not.toHaveBeenCalled();

    // Should close after 3000ms + exit animation
    await act(async () => {
      vi.advanceTimersByTime(500);
    });

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("has SVG icon for each alert type", async () => {
    const { container, rerender } = render(
      <SimpleAlert type="success" message="Test" />,
    );

    await act(async () => {
      vi.advanceTimersByTime(50);
    });

    expect(container.querySelector("svg")).toBeInTheDocument();

    rerender(<SimpleAlert type="error" message="Test" />);
    expect(container.querySelector("svg")).toBeInTheDocument();

    rerender(<SimpleAlert type="info" message="Test" />);
    expect(container.querySelector("svg")).toBeInTheDocument();

    rerender(<SimpleAlert type="warning" message="Test" />);
    expect(container.querySelector("svg")).toBeInTheDocument();
  });
});
