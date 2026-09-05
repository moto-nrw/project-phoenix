import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { EmptyStudentResults } from "./empty-student-results";

describe("EmptyStudentResults", () => {
  it("renders the empty state message", () => {
    render(<EmptyStudentResults totalCount={100} filteredCount={0} />);

    expect(screen.getByText("Keine Kinder gefunden")).toBeInTheDocument();
  });

  it("displays the search icon", () => {
    const { container } = render(
      <EmptyStudentResults totalCount={100} filteredCount={0} />,
    );

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("displays the help text", () => {
    render(<EmptyStudentResults totalCount={100} filteredCount={0} />);

    expect(
      screen.getByText("Versuchen Sie, Ihre Suchkriterien anzupassen."),
    ).toBeInTheDocument();
  });

  it("displays the total count correctly", () => {
    render(<EmptyStudentResults totalCount={100} filteredCount={0} />);

    expect(
      screen.getByText("100 Kinder insgesamt, 0 nach Filtern"),
    ).toBeInTheDocument();
  });

  it("displays the filtered count correctly", () => {
    render(<EmptyStudentResults totalCount={50} filteredCount={5} />);

    expect(
      screen.getByText("50 Kinder insgesamt, 5 nach Filtern"),
    ).toBeInTheDocument();
  });
});
