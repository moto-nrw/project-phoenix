import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Teacher } from "~/lib/teacher-api";
import { StaffList } from "./staff-list";

vi.mock("~/components/database/database-list-layout", () => ({
  DatabaseListLayout: (props: { children: React.ReactNode }) => (
    <div data-testid="list-layout">{props.children}</div>
  ),
}));

vi.mock("~/components/ui/navigation-link", () => ({
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: React.ReactNode;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

function makeTeacher(id: string, overrides: Partial<Teacher> = {}): Teacher {
  return {
    id,
    name: `Person ${id}`,
    first_name: "Person",
    last_name: id,
    account_role: "teacher",
    ...overrides,
  };
}

describe("StaffList", () => {
  it("links every person to the staff record", () => {
    render(
      <StaffList
        groupDefinitions={[
          {
            id: "teacher",
            title: "Betreuung",
            items: [makeTeacher("1"), makeTeacher("2", { role: "Leitung" })],
          },
        ]}
        objectHref={(teacher) =>
          `/staff/${teacher.id}?from=%2Fdatabase%2Fpersonal`
        }
      />,
    );

    expect(screen.getByRole("link", { name: /Person 1/ })).toHaveAttribute(
      "href",
      "/staff/1?from=%2Fdatabase%2Fpersonal",
    );
    expect(screen.getByRole("link", { name: /Person 2/ })).toHaveTextContent(
      "Leitung",
    );
    // Zeilen sind Links, keine Knöpfe; der einzige Knopf klappt die Gruppe.
    expect(
      screen.queryByRole("button", { name: /Person/ }),
    ).not.toBeInTheDocument();
  });

  it("falls back to the e-mail and then a dash for the second line", () => {
    render(
      <StaffList
        groupDefinitions={[
          {
            id: "__no_role__",
            title: "Ohne Rolle",
            items: [
              makeTeacher("3", {
                account_role: null,
                email: "drei@example.test",
              }),
              makeTeacher("4", { account_role: null }),
            ],
          },
        ]}
        objectHref={(teacher) => `/staff/${teacher.id}`}
      />,
    );

    expect(screen.getByRole("link", { name: /Person 3/ })).toHaveTextContent(
      "drei@example.test",
    );
    expect(screen.getByRole("link", { name: /Person 4/ })).toHaveTextContent(
      "–",
    );
  });

  it("shows the empty state when no group has entries", () => {
    render(<StaffList groupDefinitions={[]} objectHref={() => "/staff/0"} />);

    expect(screen.getByText("Kein Personal gefunden.")).toBeInTheDocument();
  });
});
