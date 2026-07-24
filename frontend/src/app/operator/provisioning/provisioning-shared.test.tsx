import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import {
  FormField,
  FormError,
  FieldWarning,
  StatusBadge,
  DeliveryStatusBadge,
  EmptyState,
  SimpleEmptyState,
  SelectWithChevron,
  PlusIcon,
  CardSkeletons,
} from "./provisioning-shared";

describe("FormField", () => {
  it("should render label text", () => {
    render(
      <FormField label="Email" htmlFor="email">
        <input id="email" />
      </FormField>,
    );

    expect(screen.getByText("Email")).toBeInTheDocument();
  });

  it("should set htmlFor on label", () => {
    render(
      <FormField label="Email" htmlFor="email-field">
        <input id="email-field" />
      </FormField>,
    );

    const label = screen.getByText("Email");
    expect(label).toHaveAttribute("for", "email-field");
  });

  it("should render required indicator when required is true", () => {
    render(
      <FormField label="Name" htmlFor="name" required>
        <input id="name" />
      </FormField>,
    );

    expect(screen.getByText("*")).toBeInTheDocument();
  });

  it("should not render required indicator when required is false", () => {
    render(
      <FormField label="Name" htmlFor="name">
        <input id="name" />
      </FormField>,
    );

    expect(screen.queryByText("*")).not.toBeInTheDocument();
  });

  it("should render children", () => {
    render(
      <FormField label="Field" htmlFor="field">
        <input data-testid="child-input" />
      </FormField>,
    );

    expect(screen.getByTestId("child-input")).toBeInTheDocument();
  });
});

describe("FormError", () => {
  it("should render error message", () => {
    render(<FormError message="Something went wrong" />);

    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
  });

  it("should have role alert", () => {
    render(<FormError message="Error occurred" />);

    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("should render error message inside alert role element", () => {
    render(<FormError message="Validation failed" />);

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Validation failed");
  });
});

describe("FieldWarning", () => {
  it("should render warning message", () => {
    render(<FieldWarning message="This slug is already taken" />);

    expect(screen.getByText("This slug is already taken")).toBeInTheDocument();
  });
});

describe("StatusBadge", () => {
  it("should render 'Aktiv' when active is true", () => {
    render(<StatusBadge active={true} />);

    expect(screen.getByText("Aktiv")).toBeInTheDocument();
  });

  it("should render 'Inaktiv' when active is false", () => {
    render(<StatusBadge active={false} />);

    expect(screen.getByText("Inaktiv")).toBeInTheDocument();
  });

  it("should apply green styling when active", () => {
    render(<StatusBadge active={true} />);

    const badge = screen.getByText("Aktiv");
    expect(badge.className).toContain("bg-green-100");
    expect(badge.className).toContain("text-green-700");
  });

  it("should apply gray styling when inactive", () => {
    render(<StatusBadge active={false} />);

    const badge = screen.getByText("Inaktiv");
    expect(badge.className).toContain("bg-gray-100");
    expect(badge.className).toContain("text-gray-500");
  });
});

describe("DeliveryStatusBadge", () => {
  it("should render 'Gesendet' for sent status", () => {
    render(<DeliveryStatusBadge status="sent" />);

    expect(screen.getByText("Gesendet")).toBeInTheDocument();
  });

  it("should render 'Ausstehend' for pending status", () => {
    render(<DeliveryStatusBadge status="pending" />);

    expect(screen.getByText("Ausstehend")).toBeInTheDocument();
  });

  it("should render 'Fehlgeschlagen' for failed status", () => {
    render(<DeliveryStatusBadge status="failed" />);

    expect(screen.getByText("Fehlgeschlagen")).toBeInTheDocument();
  });

  it("should apply green styling for sent", () => {
    render(<DeliveryStatusBadge status="sent" />);

    const badge = screen.getByText("Gesendet");
    expect(badge.className).toContain("bg-green-100");
    expect(badge.className).toContain("text-green-700");
  });

  it("should apply yellow styling for pending", () => {
    render(<DeliveryStatusBadge status="pending" />);

    const badge = screen.getByText("Ausstehend");
    expect(badge.className).toContain("bg-yellow-100");
    expect(badge.className).toContain("text-yellow-700");
  });

  it("should apply red styling for failed", () => {
    render(<DeliveryStatusBadge status="failed" />);

    const badge = screen.getByText("Fehlgeschlagen");
    expect(badge.className).toContain("bg-red-100");
    expect(badge.className).toContain("text-red-700");
  });

  it("should fall back to raw status text for unknown status", () => {
    render(<DeliveryStatusBadge status="unknown_status" />);

    expect(screen.getByText("unknown_status")).toBeInTheDocument();
  });

  it("should apply gray styling for unknown status", () => {
    render(<DeliveryStatusBadge status="something_else" />);

    const badge = screen.getByText("something_else");
    expect(badge.className).toContain("bg-gray-100");
    expect(badge.className).toContain("text-gray-500");
  });
});

describe("EmptyState", () => {
  it("should render title", () => {
    render(
      <EmptyState
        title="No items"
        description="Create your first item"
        buttonLabel="Add"
        onAction={vi.fn()}
      />,
    );

    expect(screen.getByText("No items")).toBeInTheDocument();
  });

  it("should render description", () => {
    render(
      <EmptyState
        title="No items"
        description="Create your first item"
        buttonLabel="Add"
        onAction={vi.fn()}
      />,
    );

    expect(screen.getByText("Create your first item")).toBeInTheDocument();
  });

  it("should render button with label", () => {
    render(
      <EmptyState
        title="No items"
        description="Description"
        buttonLabel="Add Item"
        onAction={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Add Item" }),
    ).toBeInTheDocument();
  });

  it("should call onAction when button is clicked", () => {
    const onAction = vi.fn();

    render(
      <EmptyState
        title="No items"
        description="Description"
        buttonLabel="Add"
        onAction={onAction}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Add" }));

    expect(onAction).toHaveBeenCalledTimes(1);
  });

  it("should render an SVG icon", () => {
    const { container } = render(
      <EmptyState
        title="Empty"
        description="Nothing here"
        buttonLabel="Create"
        onAction={vi.fn()}
      />,
    );

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });
});

describe("SimpleEmptyState", () => {
  it("should render title", () => {
    render(
      <SimpleEmptyState title="No data" description="Nothing to display" />,
    );

    expect(screen.getByText("No data")).toBeInTheDocument();
  });

  it("should render description", () => {
    render(
      <SimpleEmptyState title="No data" description="Nothing to display" />,
    );

    expect(screen.getByText("Nothing to display")).toBeInTheDocument();
  });

  it("should not render a button", () => {
    render(
      <SimpleEmptyState title="No data" description="Nothing to display" />,
    );

    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});

describe("SelectWithChevron", () => {
  it("should render a select-like control exposing the given options", () => {
    render(
      <SelectWithChevron aria-label="Test" value="">
        <option value="a">Option A</option>
        <option value="b">Option B</option>
      </SelectWithChevron>,
    );

    const select = screen.getByRole("combobox", { name: "Test" });
    fireEvent.click(select);
    expect(
      screen.getByRole("option", { name: "Option A" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: "Option B" }),
    ).toBeInTheDocument();
  });

  it("should render a chevron SVG icon", () => {
    const { container } = render(
      <SelectWithChevron>
        <option>Test</option>
      </SelectWithChevron>,
    );

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("should pass through value and disabled and emit selection changes", () => {
    const onChange = vi.fn();

    const { rerender } = render(
      <SelectWithChevron
        aria-label="Test"
        value="b"
        onChange={onChange}
        disabled
      >
        <option value="a">Option A</option>
        <option value="b">Option B</option>
      </SelectWithChevron>,
    );

    const disabledSelect = screen.getByRole("combobox", { name: "Test" });
    expect(disabledSelect).toHaveTextContent("Option B");
    expect(disabledSelect).toBeDisabled();

    rerender(
      <SelectWithChevron aria-label="Test" value="b" onChange={onChange}>
        <option value="a">Option A</option>
        <option value="b">Option B</option>
      </SelectWithChevron>,
    );

    fireEvent.click(screen.getByRole("combobox", { name: "Test" }));
    fireEvent.click(screen.getByRole("option", { name: "Option A" }));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange.mock.calls[0]![0].target.value).toBe("a");
  });

  it("should render option elements as children", () => {
    render(
      <SelectWithChevron aria-label="Test" value="">
        <option value="1">First</option>
        <option value="2">Second</option>
        <option value="3">Third</option>
      </SelectWithChevron>,
    );

    fireEvent.click(screen.getByRole("combobox", { name: "Test" }));
    expect(screen.getAllByRole("option")).toHaveLength(3);
  });
});

describe("PlusIcon", () => {
  it("should render an SVG element", () => {
    const { container } = render(<PlusIcon />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("should have a path element", () => {
    const { container } = render(<PlusIcon />);

    const path = container.querySelector("path");
    expect(path).toBeInTheDocument();
  });
});

describe("CardSkeletons", () => {
  it("should render 3 skeleton cards", () => {
    const { container } = render(<CardSkeletons />);

    const cards = container.querySelectorAll(".rounded-3xl");
    expect(cards).toHaveLength(3);
  });

  it("should render skeleton elements inside each card", () => {
    const { container } = render(<CardSkeletons />);

    // Each card has 3 skeleton divs (via Skeleton component which renders animate-pulse divs)
    const skeletons = container.querySelectorAll(".animate-pulse");
    expect(skeletons).toHaveLength(9);
  });
});
