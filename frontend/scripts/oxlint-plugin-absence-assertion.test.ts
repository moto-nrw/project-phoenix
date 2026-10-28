import { afterEach, describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";

const temporaryDirectories: string[] = [];

function lintSource(source: string, relativePath = "src/probe.test.tsx") {
  const directory = mkdtempSync(join(tmpdir(), "absence-assertion-"));
  temporaryDirectories.push(directory);
  const sourcePath = join(directory, relativePath);
  mkdirSync(dirname(sourcePath), { recursive: true });
  writeFileSync(sourcePath, source);

  const result = spawnSync(
    resolve("node_modules/.bin/oxlint"),
    ["-c", resolve(".oxlintrc.json"), sourcePath],
    { encoding: "utf8" },
  );
  return { ...result, output: `${result.stdout}${result.stderr}` };
}

function count(output: string, rule: string): number {
  return (
    output.match(new RegExp(`absence-assertion\\(${rule}\\)`, "g"))?.length ?? 0
  );
}

afterEach(() => {
  for (const directory of temporaryDirectories.splice(0)) {
    rmSync(directory, { recursive: true, force: true });
  }
});

describe("absence-assertion/needs-anchor", () => {
  it("reports a test whose only DOM assertions are absences", () => {
    const result = lintSource(`
      import { render, screen } from "@testing-library/react";
      import { expect, it } from "vitest";

      it("hides the admin entry for staff", () => {
        const { container } = render(<Nav />);
        expect(screen.queryByText("Datenverwaltung")).not.toBeInTheDocument();
        expect(screen.queryAllByRole("row")).toHaveLength(0);
        expect(container.querySelector(".badge")).toBeNull();
        expect(container).not.toHaveTextContent("Admin");
        expect(onSelect).not.toHaveBeenCalled();
      });
    `);

    expect(count(result.output, "needs-anchor")).toBe(1);
    expect(result.output).toContain("hides the admin entry for staff");
  });

  it("does not count mock assertions as positive DOM evidence", () => {
    const result = lintSource(`
      import { render, screen } from "@testing-library/react";
      import { expect, it } from "vitest";

      it("redirects without the role choice", () => {
        render(<Page />);
        expect(assign).toHaveBeenCalledWith("/");
        expect(screen.queryByText("Rolle wählen")).toBeNull();
      });
    `);

    expect(count(result.output, "needs-anchor")).toBe(1);
  });

  it("accepts an anchor in the test, a hook or a same-file helper", () => {
    const result = lintSource(`
      import { render, screen } from "@testing-library/react";
      import { beforeEach, describe, expect, it } from "vitest";

      async function renderLoaded() {
        render(<Nav />);
        await screen.findByText("Meine Gruppen");
      }

      it("anchors on a neighbour entry", () => {
        render(<Nav />);
        // Anker: die Navigation ist geladen.
        expect(screen.getByText("Meine Gruppen")).toBeInTheDocument();
        expect(screen.queryByText("Datenverwaltung")).not.toBeInTheDocument();
      });

      it("anchors through an interaction", async () => {
        render(<Nav />);
        await user.click(screen.getByRole("button", { name: "Mehr" }));
        expect(screen.queryByText("Datenverwaltung")).toBeNull();
      });

      it("anchors through a same-file helper", async () => {
        await renderLoaded();
        expect(screen.queryByText("Datenverwaltung")).toBeNull();
      });

      it("anchors on a counted list", () => {
        const { container } = render(<Nav />);
        const rows = container.querySelectorAll("li");
        expect(rows).toHaveLength(3);
        expect(container.querySelector(".badge")).toBeNull();
      });

      describe("with a loaded hook", () => {
        beforeEach(async () => {
          render(<Nav />);
          await screen.findByText("Meine Gruppen");
        });

        it("anchors through beforeEach", () => {
          expect(screen.queryByText("Datenverwaltung")).toBeNull();
        });
      });
    `);

    expect(count(result.output, "needs-anchor")).toBe(0);
    expect(count(result.output, "no-early-wait-for")).toBe(0);
  });

  it("does not let a hook of a sibling describe anchor the test", () => {
    const result = lintSource(`
      import { render, screen } from "@testing-library/react";
      import { beforeEach, describe, expect, it } from "vitest";

      describe("loaded", () => {
        beforeEach(async () => {
          await screen.findByText("Meine Gruppen");
        });
      });

      describe("bare", () => {
        it("has nothing positive", () => {
          render(<Nav />);
          expect(screen.queryByText("Datenverwaltung")).toBeNull();
        });
      });
    `);

    expect(count(result.output, "needs-anchor")).toBe(1);
  });

  it.each([false, true])(
    "resolves sibling helpers lexically (reverse: %s)",
    (reverse) => {
      const groups = [
        `describe("anchored", () => {
        function load() { screen.getByText("Loaded"); }
        it("has its own anchor", async () => {
          load();
          await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
        });
      });`,
        `describe("bare", () => {
        const load = () => {};
        it("has no anchor", async () => {
          load();
          await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
        });
      });`,
      ];
      if (reverse) groups.reverse();
      const result = lintSource(`
      import { screen, waitFor } from "@testing-library/react";
      import { describe, expect, it } from "vitest";
      function load() { screen.getByText("Outer"); }
      ${groups.join("\n")}
      it("uses the outer helper", async () => {
        load();
        await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
      });
    `);

      expect(count(result.output, "needs-anchor")).toBe(1);
      expect(count(result.output, "no-early-wait-for")).toBe(1);
      expect(result.output).toContain("has no anchor");
    },
  );

  it.each([
    'expect(screen.queryAllByRole("row")).toBeTruthy()',
    'expect(screen.queryAllByRole("row")).not.toBeNull()',
    'expect(screen.queryAllByRole("row")).not.toEqual(null)',
    'expect(screen.queryAllByRole("row")).not.toStrictEqual(null)',
    'expect(screen.queryAllByRole("row")).toBeDefined()',
    'expect(screen.queryAllByRole("row")).not.toBeUndefined()',
    'expect(screen.queryAllByRole("row")).not.toHaveLength(3)',
    'expect(screen.queryAllByRole("row").length).not.toBe(3)',
    'expect(screen.queryAllByRole("row").length).not.toEqual(3)',
    'expect(screen.queryAllByRole("row").length).not.toStrictEqual(3)',
    'expect(screen.queryByText("Loaded")).toBeDefined()',
    'expect(screen.queryByText("Loaded")).not.toBeUndefined()',
    "expect(document.body).not.toBeEmptyDOMElement()",
  ])("does not accept empty-tolerant evidence: %s", (assertion) => {
    const result = lintSource(`
      import { screen, waitFor } from "@testing-library/react";
      import { expect, it } from "vitest";
      it("has no anchor", async () => {
        ${assertion};
        await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
      });
    `);

    expect(count(result.output, "needs-anchor")).toBe(1);
    expect(count(result.output, "no-early-wait-for")).toBe(1);
  });

  it.each([
    'expect(screen.queryAllByRole("row")).toHaveLength(3)',
    'expect(screen.queryAllByRole("row")).not.toHaveLength(0)',
    'expect(screen.queryAllByRole("row").length).toBe(3)',
    'expect(screen.queryAllByRole("row").length).not.toEqual(0)',
    'expect(screen.queryByText("Loaded")).not.toBeNull()',
    'expect(screen.queryByText("Loaded")).not.toBe(null)',
    'expect(screen.queryByText("Loaded")).not.toEqual(null)',
    'expect(screen.queryByText("Loaded")).not.toStrictEqual(null)',
    'expect(screen.queryAllByRole("row")).not.toEqual([])',
    'expect(screen.queryAllByRole("row")).not.toStrictEqual([])',
    'expect(screen.queryByText("Loaded")).toBeTruthy()',
  ])("accepts nonempty evidence: %s", (assertion) => {
    const result = lintSource(`
      import { screen, waitFor } from "@testing-library/react";
      import { expect, it } from "vitest";
      it("has an anchor", async () => {
        ${assertion};
        await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
      });
    `);

    expect(count(result.output, "needs-anchor")).toBe(0);
    expect(count(result.output, "no-early-wait-for")).toBe(0);
    expect(result.status).toBe(0);
  });

  it.each(["toBe", "toEqual", "toStrictEqual"])(
    "treats %s(null) as an absence without another absence matcher",
    (matcher) => {
      const result = lintSource(`
        import { screen, waitFor } from "@testing-library/react";
        import { expect, it } from "vitest";
        it("has no loaded content", async () => {
          await waitFor(() => {
            expect(screen.queryByText("Loaded")).${matcher}(null);
          });
        });
      `);

      expect(count(result.output, "needs-anchor")).toBe(1);
      expect(count(result.output, "no-early-wait-for")).toBe(1);
    },
  );

  it("exempts the exact empty-render contract", () => {
    const result = lintSource(`
      import { render, screen } from "@testing-library/react";
      import { expect, it } from "vitest";

      it("renders nothing without warnings", () => {
        const { container } = render(<Banner warnings={[]} />);
        expect(container.firstChild).toBeNull();
      });

      it("renders an empty container without warnings", () => {
        const { container } = render(<Banner warnings={[]} />);
        expect(container).toBeEmptyDOMElement();
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
      });

      it("renders no markup without warnings", () => {
        const { container } = render(<Banner warnings={[]} />);
        expect(container.innerHTML).toBe("");
      });
    `);

    expect(count(result.output, "needs-anchor")).toBe(0);
  });

  it.each([
    'expect(container.textContent).toBe("")',
    'expect(container.textContent).toEqual("")',
    "expect(container.textContent).toHaveLength(0)",
  ])(
    "treats empty text as an absence, not an empty render: %s",
    (assertion) => {
      // A skeleton of bare boxes has no text either.
      const result = lintSource(`
      import { render } from "@testing-library/react";
      import { expect, it } from "vitest";
      it("renders no text", () => {
        const { container } = render(<Banner warnings={[]} />);
        ${assertion};
      });
    `);

      expect(count(result.output, "needs-anchor")).toBe(1);
    },
  );

  it("covers it.each and test.skip variants", () => {
    const result = lintSource(`
      import { render, screen } from "@testing-library/react";
      import { expect, it, test } from "vitest";

      it.each(["a", "b"])("hides %s", (label) => {
        render(<Nav />);
        expect(screen.queryByText(label)).not.toBeInTheDocument();
      });

      test.skip("hides the badge", () => {
        render(<Nav />);
        expect(screen.queryByTestId("badge")).toBeNull();
      });
    `);

    expect(count(result.output, "needs-anchor")).toBe(2);
  });
});

describe("absence-assertion/no-early-wait-for", () => {
  it("reports a waitFor that checks an absence before any positive state", () => {
    const result = lintSource(`
      import { render, screen, waitFor } from "@testing-library/react";
      import { expect, it } from "vitest";

      it("shows no error after loading", async () => {
        render(<Page />);
        await waitFor(() => {
          expect(screen.queryByRole("alert")).not.toBeInTheDocument();
        });
        expect(screen.getByText("Geladen")).toBeInTheDocument();
      });
    `);

    expect(count(result.output, "no-early-wait-for")).toBe(1);
    expect(count(result.output, "needs-anchor")).toBe(0);
  });

  it.each([false, true])(
    "accepts positive evidence anywhere in the same callback (absence first: %s)",
    (absenceFirst) => {
      const assertions = [
        'expect(screen.queryByText("Loaded")).not.toBeNull();',
        'expect(screen.queryByRole("alert")).toBeNull();',
      ];
      if (absenceFirst) assertions.reverse();
      const result = lintSource(`
        import { screen, waitFor } from "@testing-library/react";
        import { expect, it } from "vitest";
        it("waits for loaded content without an alert", async () => {
          await waitFor(() => { ${assertions.join("\n")} });
        });
      `);

      expect(count(result.output, "needs-anchor")).toBe(0);
      expect(count(result.output, "no-early-wait-for")).toBe(0);
      expect(result.status).toBe(0);
    },
  );

  it("accepts a waitFor after the loaded state or a positive check inside it", () => {
    const result = lintSource(`
      import { render, screen, waitFor } from "@testing-library/react";
      import { expect, it } from "vitest";

      it("waits for the dialog to close", async () => {
        render(<Page />);
        await user.click(await screen.findByRole("button", { name: "Öffnen" }));
        expect(screen.getByRole("dialog")).toBeInTheDocument();
        await user.click(screen.getByRole("button", { name: "Abbrechen" }));
        await waitFor(() => {
          expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        });
      });

      it("checks the loaded state first inside the same waitFor", async () => {
        render(<Page />);
        await waitFor(() => {
          expect(screen.getByText("Geladen")).toBeInTheDocument();
          expect(screen.queryByRole("alert")).not.toBeInTheDocument();
        });
      });
    `);

    expect(count(result.output, "no-early-wait-for")).toBe(0);
  });
});
