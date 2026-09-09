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

describe("bauart/no-unconfirmed-destructive-click", () => {
  it("rejects a delete button that fires the removal from the click", () => {
    const { status, output } = lintSource(
      `import { Button } from "~/components/ui/button";
      export function Probe({ remove }: { remove: () => Promise<void> }) {
        return (
          <Button type="button" onClick={() => void remove()}>
            Entfernen
          </Button>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-unconfirmed-destructive-click)");
    expect(output).toContain("„Entfernen“");
  });

  it("rejects a direct returned action regardless of its function name", () => {
    const { status, output } = lintSource(
      `import { Button } from "~/components/ui/button";
      export function Probe({ archiveTrack }: { archiveTrack: (id: string) => Promise<void> }) {
        return (
          <Button type="button" onClick={() => archiveTrack("1")}>
            Archivieren
          </Button>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-unconfirmed-destructive-click)");
  });

  it("rejects an implicitly returned action from an async handler", () => {
    const { status, output } = lintSource(
      `import { Button } from "~/components/ui/button";
      export function Probe({ revokePasskey }: { revokePasskey: (id: string) => Promise<void> }) {
        return (
          <Button type="button" onClick={async () => revokePasskey("1")}>
            Passkey entfernen
          </Button>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-unconfirmed-destructive-click)");
  });

  it("sees the removal inside a branch, an aria-label and an async handler", () => {
    const { status, output } = lintSource(
      `export function Probe({
        archive,
        ready,
      }: {
        archive: () => Promise<void>;
        ready: boolean;
      }) {
        return (
          <button
            type="button"
            aria-label="Spur archivieren"
            onClick={async () => {
              if (ready) await archive();
            }}
          />
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-unconfirmed-destructive-click)");
  });

  it("rejects a menu item that deletes from its click", () => {
    const { status, output } = lintSource(
      `export function items(remove: () => Promise<void>) {
        return [
          { label: "Bearbeiten", onClick: () => {} },
          { label: "Löschen", destructive: true, onClick: () => void remove() },
        ];
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("Menüeintrag „Löschen“");
  });

  it("accepts a click that only opens the confirmation", () => {
    const { status, output } = lintSource(
      `import { Button } from "~/components/ui/button";
      export function Probe({
        setTarget,
      }: {
        setTarget: (id: string) => void;
      }) {
        return (
          <>
            <Button type="button" onClick={() => setTarget("1")}>
              Löschen
            </Button>
            {[{ label: "Entfernen", onClick: () => setTarget("2") }].map(
              (item) => item.label,
            )}
          </>
        );
      }`,
    );

    expect(output).not.toContain("bauart(no-unconfirmed-destructive-click)");
    expect(status).toBe(0);
  });

  it("accepts a void state setter that opens the confirmation", () => {
    const { status, output } = lintSource(
      `import { Button } from "~/components/ui/button";
      export function Probe({
        setDeleteTarget,
      }: {
        setDeleteTarget: (id: string) => void;
      }) {
        return (
          <Button type="button" onClick={() => void setDeleteTarget("1")}>
            Löschen
          </Button>
        );
      }`,
    );

    expect(output).not.toContain("bauart(no-unconfirmed-destructive-click)");
    expect(status).toBe(0);
  });

  it("accepts a non-destructive action fired from the click", () => {
    const { status, output } = lintSource(
      `import { Button } from "~/components/ui/button";
      export function Probe({ save }: { save: () => Promise<void> }) {
        return (
          <Button type="button" onClick={() => void save()}>
            Speichern
          </Button>
        );
      }`,
    );

    expect(output).not.toContain("bauart(no-unconfirmed-destructive-click)");
    expect(status).toBe(0);
  });

  it("leaves the kit directory alone", () => {
    const { output } = lintSource(
      `import { Button } from "./button";
      export function Probe({ remove }: { remove: () => Promise<void> }) {
        return (
          <Button type="button" onClick={() => void remove()}>
            Endgültig löschen
          </Button>
        );
      }`,
      "src/components/ui/confirm-delete-modal.tsx",
    );

    expect(output).not.toContain("bauart(no-unconfirmed-destructive-click)");
  });
});
