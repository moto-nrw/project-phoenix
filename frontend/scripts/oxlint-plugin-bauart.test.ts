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
