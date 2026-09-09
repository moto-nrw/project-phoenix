import { afterEach, describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";

const temporaryDirectories: string[] = [];

function lintSource(source: string, relativePath = "src/components/probe.tsx") {
  const directory = mkdtempSync(join(tmpdir(), "bauart-"));
  temporaryDirectories.push(directory);
  const sourcePath = join(directory, relativePath);
  mkdirSync(dirname(sourcePath), { recursive: true });
  writeFileSync(sourcePath, source);

  const result = spawnSync(
    resolve("node_modules/.bin/oxlint"),
    ["-c", resolve(".oxlintrc.json"), sourcePath],
    { encoding: "utf8" },
  );
  return { status: result.status, output: `${result.stdout}${result.stderr}` };
}

afterEach(() => {
  for (const directory of temporaryDirectories.splice(0)) {
    rmSync(directory, { recursive: true, force: true });
  }
});

describe("bauart/one-delete-confirm", () => {
  it("rejects a ConfirmationModal whose action deletes", () => {
    const { status, output } = lintSource(
      `import { ConfirmationModal } from "~/components/ui/modal";
      export function Probe() {
        return (
          <ConfirmationModal
            isOpen
            onClose={() => {}}
            onConfirm={() => {}}
            title="Eintrag löschen?"
            confirmText="Ja, endgültig löschen"
          >
            <p>Weg damit.</p>
          </ConfirmationModal>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(one-delete-confirm)");
    expect(output).toContain("Ja, endgültig löschen");
  });

  it("sees through a conditional action label", () => {
    const { status, output } = lintSource(
      `import { ConfirmationModal } from "~/components/ui/modal";
      export function Probe({ busy }: { busy: boolean }) {
        return (
          <ConfirmationModal
            isOpen
            onClose={() => {}}
            onConfirm={() => {}}
            title="Eintrag"
            confirmText={busy ? "Wird gelöscht…" : "Entfernen"}
          >
            <p>Weg damit.</p>
          </ConfirmationModal>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(one-delete-confirm)");
  });

  it("accepts a state change whose side effect removes data", () => {
    const { status, output } = lintSource(
      `import { ConfirmationModal } from "~/components/ui/modal";
      export function Probe() {
        return (
          <ConfirmationModal
            isOpen
            onClose={() => {}}
            onConfirm={() => {}}
            title="Alle Betreuungstage entfernen?"
            confirmText="Änderung speichern"
          >
            <p>Danach ist kein Betreuungstag mehr gebucht.</p>
          </ConfirmationModal>
        );
      }`,
    );

    expect(output).not.toContain("bauart(one-delete-confirm)");
    expect(status).toBe(0);
  });

  it("rejects a hand-built delete modal and a delete ChoiceModal", () => {
    const { status, output } = lintSource(
      `import { Modal } from "~/components/ui/modal";
      import { ChoiceModal } from "~/components/ui/choice-modal";
      export function Probe() {
        return (
          <>
            <Modal isOpen onClose={() => {}} title={\`Termin \${"löschen"}\`}>
              <p>Weg damit.</p>
            </Modal>
            <ChoiceModal
              isOpen
              onClose={() => {}}
              title="Wiederholenden Termin löschen"
              options={[]}
              onSelect={() => {}}
            />
          </>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output.match(/bauart\(one-delete-confirm\)/g)).toHaveLength(2);
  });

  it("rejects window.confirm", () => {
    const { status, output } = lintSource(
      `export function probe() {
        return window.confirm("Wirklich?");
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("window.confirm");
  });

  it("leaves the kit directory and tests alone", () => {
    const source = `import { Modal } from "./modal";
      export function Probe() {
        return (
          <Modal isOpen onClose={() => {}} title="Eintrag löschen">
            <p>Kit-intern.</p>
          </Modal>
        );
      }`;

    expect(
      lintSource(source, "src/components/ui/confirm-delete-modal.tsx").output,
    ).not.toContain("bauart(one-delete-confirm)");
    expect(
      lintSource(source, "src/components/probe.test.tsx").output,
    ).not.toContain("bauart(one-delete-confirm)");
  });
});

describe("bauart/no-row-action-buttons", () => {
  const ROW_ACTION = "bauart(no-row-action-buttons)";

  it("rejects an icon row of edit/delete buttons per list item", () => {
    const { status, output } = lintSource(
      `import { Button } from "~/components/ui/button";
      import { Pencil, Trash2 } from "lucide-react";
      export function Probe({ rows }: { rows: { id: string; name: string }[] }) {
        return (
          <ul>
            {rows.map((row) => (
              <li key={row.id}>
                {row.name}
                <Button type="button" variant="ghost" size="icon" aria-label={\`\${row.name} bearbeiten\`}>
                  <Pencil className="h-4 w-4" />
                </Button>
                <Button type="button" variant="ghost" size="icon" aria-label="Löschen">
                  <Trash2 className="h-4 w-4" />
                </Button>
              </li>
            ))}
          </ul>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output.match(/bauart\(no-row-action-buttons\)/g)).toHaveLength(2);
    expect(output).toContain("bearbeiten");
  });

  it("rejects text buttons in a DataTable column render", () => {
    const { status, output } = lintSource(
      `import { Button } from "~/components/ui/button";
      export const columns = [
        {
          key: "actions",
          render: (day: { id: string }) => (
            <div>
              <Button type="button" variant="ghost" size="compact">
                Bearbeiten
              </Button>
              <button type="button">{day.id ? "Löscht…" : "Endgültig löschen"}</button>
            </div>
          ),
        },
      ];`,
    );

    expect(status).toBe(1);
    expect(output.match(/bauart\(no-row-action-buttons\)/g)).toHaveLength(2);
  });

  it("accepts the kebab, reorder arrows and chip removers", () => {
    const { output } = lintSource(
      `import { Button } from "~/components/ui/button";
      import { ChevronUp, X } from "lucide-react";
      import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
      export function Probe({ rows }: { rows: { id: string; name: string }[] }) {
        return (
          <ul>
            {rows.map((row) => (
              <li key={row.id}>
                <Button type="button" variant="ghost" size="icon" aria-label={\`\${row.name} nach oben\`}>
                  <ChevronUp className="size-4" />
                </Button>
                <Button type="button" variant="ghost" size="icon" aria-label={\`\${row.name} entfernen\`}>
                  <X className="size-4" />
                </Button>
                <OverflowMenu
                  ariaLabel={\`Aktionen für \${row.name}\`}
                  items={[
                    { label: "Bearbeiten", onClick: () => {} },
                    { label: "Löschen", destructive: true, onClick: () => {} },
                  ]}
                />
              </li>
            ))}
          </ul>
        );
      }`,
    );

    expect(output).not.toContain(ROW_ACTION);
  });

  it("ignores buttons outside a per-item render", () => {
    const { output } = lintSource(
      `import { Button } from "~/components/ui/button";
      export function Probe() {
        return (
          <footer>
            <Button type="button" variant="danger">Löschen</Button>
            <Button type="button">Bearbeiten</Button>
          </footer>
        );
      }`,
    );

    expect(output).not.toContain(ROW_ACTION);
  });

  it("leaves the other portals alone", () => {
    const source = `import { Button } from "~/components/ui/button";
      export function Probe({ rows }: { rows: string[] }) {
        return rows.map((row) => (
          <Button key={row} type="button">Löschen</Button>
        ));
      }`;

    for (const path of [
      "src/app/operator/persons/page.tsx",
      "src/app/parents/(protected)/page.tsx",
      "src/app/school/page.tsx",
      "src/components/operator/persons-table.tsx",
      "src/components/parent/guardians-panel.tsx",
      "src/components/school/list.tsx",
    ]) {
      expect(lintSource(source, path).output).not.toContain(ROW_ACTION);
    }
    expect(
      lintSource(source, "src/app/[tenant]/(protected)/probe/page.tsx").output,
    ).toContain(ROW_ACTION);
  });

  it("tolerates a baselined file only up to its count", () => {
    const button = `<Button key={row} type="button">Löschen</Button>`;
    const probe = (
      count: number,
    ) => `import { Button } from "~/components/ui/button";
      export function Probe({ rows }: { rows: string[] }) {
        return rows.map((row) => (
          <>${button.repeat(count)}</>
        ));
      }`;
    const path = "src/components/admin/pending-invitations-list.tsx";

    expect(lintSource(probe(1), path).output).not.toContain(ROW_ACTION);
    expect(lintSource(probe(2), path).output).toContain(ROW_ACTION);
  });
});
