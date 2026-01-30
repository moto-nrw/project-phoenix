import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import {
  DataField,
  InfoSection,
  DataGrid,
  InfoText,
  DetailIcons,
} from "./detail-modal-components";

describe("DataField", () => {
  it("renders label and children", () => {
    render(
      <dl>
        <DataField label="Name">John Doe</DataField>
      </dl>,
    );

    expect(screen.getByText("Name")).toBeInTheDocument();
    expect(screen.getByText("John Doe")).toBeInTheDocument();
  });

  it("applies fullWidth class when specified", () => {
    const { container } = render(
      <dl>
        <DataField label="Description" fullWidth>
          Long description text
        </DataField>
      </dl>,
    );

    const div = container.querySelector(".col-span-1");
    expect(div).toBeInTheDocument();
  });

  it("applies monospace font when mono is true", () => {
    render(
      <dl>
        <DataField label="ID" mono>
          12345
        </DataField>
      </dl>,
    );

    const dd = screen.getByText("12345");
    expect(dd).toHaveClass("font-mono");
  });
});

describe("InfoSection", () => {
  it("renders title and children", () => {
    render(
      <InfoSection title="Personal Information" icon={DetailIcons.person}>
        <p>Content here</p>
      </InfoSection>,
    );

    expect(screen.getByText("Personal Information")).toBeInTheDocument();
    expect(screen.getByText("Content here")).toBeInTheDocument();
  });

  it("applies default gray accent color", () => {
    const { container } = render(
      <InfoSection title="Test" icon={DetailIcons.person}>
        Content
      </InfoSection>,
    );

    const section = container.querySelector(".bg-gray-50");
    expect(section).toBeInTheDocument();
  });

  it("applies blue accent color", () => {
    const { container } = render(
      <InfoSection title="Test" icon={DetailIcons.person} accentColor="blue">
        Content
      </InfoSection>,
    );

    const section = container.querySelector(".bg-blue-50\\/30");
    expect(section).toBeInTheDocument();
  });

  it("applies orange accent color", () => {
    const { container } = render(
      <InfoSection title="Test" icon={DetailIcons.person} accentColor="orange">
        Content
      </InfoSection>,
    );

    const section = container.querySelector(".bg-orange-50\\/30");
    expect(section).toBeInTheDocument();
  });

  it("renders icon with correct color class", () => {
    const { container } = render(
      <InfoSection title="Test" icon={DetailIcons.person} accentColor="indigo">
        Content
      </InfoSection>,
    );

    const iconSpan = container.querySelector(".text-indigo-600");
    expect(iconSpan).toBeInTheDocument();
  });
});

describe("DataGrid", () => {
  it("renders children in a grid layout", () => {
    render(
      <DataGrid>
        <div>Item 1</div>
        <div>Item 2</div>
      </DataGrid>,
    );

    expect(screen.getByText("Item 1")).toBeInTheDocument();
    expect(screen.getByText("Item 2")).toBeInTheDocument();
  });

  it("has correct grid classes", () => {
    const { container } = render(
      <DataGrid>
        <div>Content</div>
      </DataGrid>,
    );

    const dl = container.querySelector("dl");
    expect(dl).toHaveClass("grid", "grid-cols-1", "sm:grid-cols-2");
  });
});

describe("InfoText", () => {
  it("renders text content", () => {
    render(<InfoText>This is some information text</InfoText>);

    expect(
      screen.getByText("This is some information text"),
    ).toBeInTheDocument();
  });

  it("has correct styling classes", () => {
    const { container } = render(<InfoText>Test</InfoText>);

    const p = container.querySelector("p");
    expect(p).toHaveClass("text-xs", "text-gray-700", "whitespace-pre-wrap");
  });
});

describe("DetailIcons", () => {
  it("exports person icon", () => {
    const { container } = render(<>{DetailIcons.person}</>);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("exports group icon", () => {
    const { container } = render(<>{DetailIcons.group}</>);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("exports heart icon", () => {
    const { container } = render(<>{DetailIcons.heart}</>);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("exports check icon", () => {
    const { container } = render(<>{DetailIcons.check}</>);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("exports x icon", () => {
    const { container } = render(<>{DetailIcons.x}</>);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });
});
