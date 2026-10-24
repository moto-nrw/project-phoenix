import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
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

  // #4432: the uploaded picture, initials only for people without one.
  it("shows the uploaded profile picture and initials without one", () => {
    const { container } = render(
      <StaffList
        groupDefinitions={[
          {
            id: "teacher",
            title: "Betreuung",
            items: [
              makeTeacher("1", { avatar: "/uploads/avatars/global/1.png" }),
              makeTeacher("2", { avatar: null }),
            ],
          },
        ]}
        objectHref={(teacher) => `/staff/${teacher.id}`}
      />,
    );

    const images = container.querySelectorAll("img");
    expect(images).toHaveLength(1);
    expect(images[0]?.getAttribute("src")).toMatch(/\/api\/staff\/1\/avatar$/);
    expect(screen.getByRole("link", { name: /Person 2/ })).toHaveTextContent(
      "P2",
    );
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

// #4573: externe Personen zeigen Organisation und Kontostand; ohne Konto
// steht „Konto einladen“ im Kebab der Zeile.
describe("StaffList external caregivers", () => {
  const externals = [
    makeTeacher("7", {
      name: "Elli Alt",
      account_role: null,
      is_external: true,
      external_organization: "Sportverein",
      external_account_status: "none",
    }),
    makeTeacher("8", {
      name: "Mira Neu",
      account_role: null,
      is_external: true,
      external_organization: "Musikschule",
      external_account_status: "invitation_pending",
    }),
    makeTeacher("9", {
      name: "Jonas Konto",
      account_role: null,
      is_external: true,
      external_account_status: "account",
    }),
  ];

  function renderExternals(onInviteExternal?: (teacher: Teacher) => void) {
    render(
      <StaffList
        groupDefinitions={[
          { id: "__external__", title: "Extern", items: externals },
        ]}
        objectHref={(teacher) => `/staff/${teacher.id}`}
        onInviteExternal={onInviteExternal}
      />,
    );
  }

  it("shows organization and account state of each external person", () => {
    renderExternals();

    const elli = screen.getByRole("link", { name: /Elli Alt/ });
    expect(elli).toHaveTextContent("Extern · Sportverein");
    expect(elli).toHaveTextContent("Kein Konto");
    expect(screen.getByRole("link", { name: /Mira Neu/ })).toHaveTextContent(
      "Einladung offen",
    );
    const jonas = screen.getByRole("link", { name: /Jonas Konto/ });
    expect(jonas).toHaveTextContent("Extern");
    expect(jonas).not.toHaveTextContent("Kein Konto");
    expect(jonas).not.toHaveTextContent("Einladung offen");
  });

  it("offers the invitation in the row menu of a person without account", () => {
    const onInviteExternal = vi.fn();
    renderExternals(onInviteExternal);

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Elli Alt" }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Konto einladen" }));

    expect(onInviteExternal).toHaveBeenCalledWith(
      expect.objectContaining({ id: "7" }),
    );
  });

  it("offers a new invitation while the first is still open", () => {
    renderExternals(vi.fn());

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Mira Neu" }),
    );

    expect(
      screen.getByRole("menuitem", { name: "Einladung erneut senden" }),
    ).toBeInTheDocument();
  });

  it("has no row menu for a person with account", () => {
    renderExternals(vi.fn());
    // Anker: die Zeile mit Konto ist da, nur ohne Kebab.
    expect(
      screen.getByRole("link", { name: /Jonas Konto/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Aktionen für Jonas Konto" }),
    ).not.toBeInTheDocument();
  });

  it("has no row menu at all without the invite permission", () => {
    renderExternals();
    // Anker: die Zeile ohne Konto ist da, nur ohne Kebab.
    expect(screen.getByRole("link", { name: /Elli Alt/ })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Aktionen für Elli Alt" }),
    ).not.toBeInTheDocument();
  });
});
