import { afterEach, describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
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
                <span className="inline-flex rounded-full">
                  <Button type="button" variant="ghost" size="icon" aria-label={\`\${row.name} entfernen\`}>
                    <X className="size-4" />
                  </Button>
                </span>
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

  it("rejects an icon-only object removal outside a form chip", () => {
    const { status, output } = lintSource(
      `import { Button } from "~/components/ui/button";
      import { Trash2 } from "lucide-react";
      export function Probe({ rows }: { rows: { id: string; name: string }[] }) {
        return rows.map((row) => (
          <div key={row.id}>
            <span>{row.name}</span>
            <Button type="button" variant="ghost" size="icon" aria-label={\`\${row.name} entfernen\`}>
              <Trash2 className="size-4" />
            </Button>
          </div>
        ));
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain(ROW_ACTION);
  });

  it("rejects removal of an object beside an editable field", () => {
    const { status, output } = lintSource(
      `import { Button } from "~/components/ui/button";
      import { Trash2 } from "lucide-react";
      export function Probe({ rows }: { rows: { id: string; name: string }[] }) {
        return rows.map((row) => (
          <div key={row.id}>
            <input value={row.name} onChange={() => {}} />
            <Button type="button" variant="ghost" size="icon" aria-label={\`\${row.name} entfernen\`} onClick={() => remove(row.id)}>
              <Trash2 className="size-4" />
            </Button>
          </div>
        ));
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain(ROW_ACTION);
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

  it("tolerates only the baselined row action at its recorded location", () => {
    const path = "src/components/planning/calendar-periods-editor.tsx";
    const source = readFileSync(resolve(path), "utf8");

    expect(lintSource(source, path).output).not.toContain(ROW_ACTION);
    expect(
      lintSource(
        source.replace(
          />\s*Bearbeiten\s*<\//,
          ">\\n            Archivieren\\n          </",
        ),
        path,
      ).output,
    ).toContain(ROW_ACTION);
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

  it("recognizes a translated destructive label", () => {
    const { status, output } = lintSource(
      `import { Button } from "~/components/ui/button";
      export function Probe({
        archiveTrack,
        t,
      }: {
        archiveTrack: () => Promise<void>;
        t: (key: string) => string;
      }) {
        return (
          <Button type="button" onClick={() => void archiveTrack()}>
            {t("remove")}
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

describe("bauart/no-toast-form-error", () => {
  it("rejects a validation toast in a submit handler", () => {
    const { status, output } = lintSource(
      `import { useToast } from "~/contexts/ToastContext";
      export function Probe() {
        const toast = useToast();
        const handleSubmit = (event: React.FormEvent) => {
          event.preventDefault();
          toast.warning("Bitte einen Titel eintragen.");
        };
        return <form onSubmit={handleSubmit} />;
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-toast-form-error)");
    expect(output).toContain("handleSubmit");
  });

  it("rejects a save error toasted from the catch of a save handler", () => {
    const { status, output } = lintSource(
      `import { useToast } from "~/contexts/ToastContext";
      export function Probe({ save }: { save: () => Promise<void> }) {
        const toast = useToast();
        const handleSave = useCallback(async () => {
          try {
            await save();
          } catch (err) {
            toast.error("Speichern fehlgeschlagen.");
          }
        }, [save, toast]);
        return <button type="button" onClick={() => void handleSave()} />;
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-toast-form-error)");
    expect(output).toContain("handleSave");
  });

  it("sees the destructured alias and a promise callback inside the handler", () => {
    const { status, output } = lintSource(
      `import { useToast } from "~/contexts/ToastContext";
      export function Probe({ save }: { save: () => Promise<void> }) {
        const { error: toastError } = useToast();
        return (
          <form
            onSubmit={(event) => {
              event.preventDefault();
              save().catch(() => toastError("Speichern fehlgeschlagen."));
            }}
          />
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-toast-form-error)");
  });

  it("accepts a success toast and an error toast outside a submit handler", () => {
    const { status, output } = lintSource(
      `import { useToast } from "~/contexts/ToastContext";
      export function Probe({ save, load }: { save: () => Promise<void>; load: () => Promise<void> }) {
        const toast = useToast();
        const handleSave = async () => {
          await save();
          toast.success("Gespeichert.");
        };
        const handleReload = async () => {
          try {
            await load();
          } catch {
            toast.error("Laden fehlgeschlagen.");
          }
        };
        return (
          <>
            <button type="button" onClick={() => void handleSave()} />
            <button type="button" onClick={() => void handleReload()} />
          </>
        );
      }`,
    );

    expect(status).toBe(0);
    expect(output).not.toContain("bauart(no-toast-form-error)");
  });

  it("leaves the kit directory, tests and the other portals alone", () => {
    const source = `import { useToast } from "~/contexts/ToastContext";
      export function Probe() {
        const toast = useToast();
        const handleSubmit = () => toast.error("Nein.");
        return <form onSubmit={handleSubmit} />;
      }`;

    for (const path of [
      "src/components/ui/probe.tsx",
      "src/components/probe.test.tsx",
      "src/app/operator/probe.tsx",
      "src/components/parent/probe.tsx",
    ]) {
      const { output } = lintSource(source, path);
      expect(output).not.toContain("bauart(no-toast-form-error)");
    }
  });
});
