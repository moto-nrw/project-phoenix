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

describe("bauart/no-manage-surface-in-overlay", () => {
  it("rejects a per-row kebab with object actions inside a SlideOver", () => {
    const { status, output } = lintSource(
      `import { SlideOver, SlideOverContent } from "~/components/ui/slide-over";
      import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
      export function Probe({ items }: { items: { id: string; name: string }[] }) {
        return (
          <SlideOver open onOpenChange={() => {}}>
            <SlideOverContent>
              <ul>
                {items.map((item) => (
                  <li key={item.id}>
                    {item.name}
                    <OverflowMenu
                      ariaLabel={"Aktionen"}
                      items={[
                        { label: "Bearbeiten", onClick: () => {} },
                        { label: "Archivieren", onClick: () => {} },
                      ]}
                    />
                  </li>
                ))}
              </ul>
            </SlideOverContent>
          </SlideOver>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-manage-surface-in-overlay)");
    expect(output).toContain("Bearbeiten");
  });

  it("follows a helper the overlay renders", () => {
    const { status, output } = lintSource(
      `import { AnchoredPopover } from "~/components/ui/anchored-popover";
      import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
      export function Probe({ items }: { items: { id: string; name: string }[] }) {
        const manageView = (
          <ul>
            {items.map((item) => (
              <li key={item.id}>
                <OverflowMenu
                  ariaLabel={"Aktionen"}
                  items={[{ label: "Umbenennen", onClick: () => {} }]}
                />
              </li>
            ))}
          </ul>
        );
        return (
          <AnchoredPopover open onOpenChange={() => {}} renderTrigger={() => null}>
            {() => manageView}
          </AnchoredPopover>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-manage-surface-in-overlay)");
    expect(output).toContain("Umbenennen");
  });

  it("rejects a rename button in a listbox menu slot", () => {
    const { status, output } = lintSource(
      `import { ListboxDropdown } from "~/components/ui/listbox-dropdown";
      export function Probe() {
        return (
          <ListboxDropdown
            value=""
            options={[]}
            onChange={() => {}}
            renderOptionActions={(option) => (
              <button type="button" aria-label={option.label + " umbenennen"} />
            )}
          />
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-manage-surface-in-overlay)");
    expect(output).toContain("Auswahlfeld");
  });

  it("accepts the same list on a page", () => {
    const { status, output } = lintSource(
      `import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
      export function Probe({ items }: { items: { id: string; name: string }[] }) {
        return (
          <ul>
            {items.map((item) => (
              <li key={item.id}>
                {item.name}
                <OverflowMenu
                  ariaLabel={"Aktionen"}
                  items={[{ label: "Bearbeiten", onClick: () => {} }]}
                />
              </li>
            ))}
          </ul>
        );
      }`,
    );

    expect(status).toBe(0);
    expect(output).not.toContain("bauart(no-manage-surface-in-overlay)");
  });

  it("accepts a plain select inside an overlay", () => {
    const { status, output } = lintSource(
      `import { Modal } from "~/components/ui/modal";
      import { ListboxDropdown } from "~/components/ui/listbox-dropdown";
      export function Probe({ items }: { items: { id: string; name: string }[] }) {
        return (
          <Modal isOpen onClose={() => {}} title="Abwesenheit">
            <ListboxDropdown
              value=""
              options={items.map((item) => ({ value: item.id, label: item.name }))}
              onChange={() => {}}
            />
            <button type="button">Speichern</button>
          </Modal>
        );
      }`,
    );

    expect(status).toBe(0);
    expect(output).not.toContain("bauart(no-manage-surface-in-overlay)");
  });

  it("leaves the kit directory, tests and the other portals alone", () => {
    const source = `import { Modal } from "~/components/ui/modal";
      import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
      export function Probe({ items }: { items: { id: string }[] }) {
        return (
          <Modal isOpen onClose={() => {}} title="Liste">
            {items.map((item) => (
              <OverflowMenu
                key={item.id}
                ariaLabel={"Aktionen"}
                items={[{ label: "Bearbeiten", onClick: () => {} }]}
              />
            ))}
          </Modal>
        );
      }`;

    for (const path of [
      "src/components/ui/probe.tsx",
      "src/components/probe.test.tsx",
      "src/app/operator/probe.tsx",
      "src/components/parent/probe.tsx",
    ]) {
      const { output } = lintSource(source, path);
      expect(output).not.toContain("bauart(no-manage-surface-in-overlay)");
    }
  });
});

describe("bauart/one-detail-per-type", () => {
  const paneSource = `import { MasterDetailLayout } from "~/components/database/master-detail-layout";
    export function Probe() {
      return <MasterDetailLayout list={<div />} detail={<div />} />;
    }`;

  it("rejects MasterDetailLayout outside the types whose pane is the only object view", () => {
    const { status, output } = lintSource(
      paneSource,
      "src/components/students/students-master-detail.tsx",
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(one-detail-per-type)");
    expect(output).toContain("Zweiter Detailbaum");
  });

  it("allows the pane for the listed types", () => {
    for (const path of [
      "src/components/groups/groups-master-detail.tsx",
      "src/components/database/catalog/catalog-page.tsx",
    ]) {
      const { output } = lintSource(paneSource, path);
      expect(output).not.toContain("bauart(one-detail-per-type)");
    }
  });

  it("rejects the object-view field groups inside a SlideOver", () => {
    const { status, output } = lintSource(
      `import { SlideOver, SlideOverContent, SlideOverBody } from "~/components/ui/slide-over";
      import { InfoSection, DataGrid, DataField } from "~/components/ui/detail-modal-components";
      export function Probe({ room }: { room: { name: string } }) {
        return (
          <SlideOver open onOpenChange={() => {}}>
            <SlideOverContent>
              <SlideOverBody>
                <InfoSection title="Raum" icon={null}>
                  <DataGrid>
                    <DataField label="Name">{room.name}</DataField>
                  </DataGrid>
                </InfoSection>
              </SlideOverBody>
            </SlideOverContent>
          </SlideOver>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(one-detail-per-type)");
    expect(output).toContain("Detailansicht im Slide-over");
  });

  it("rejects the pane components inside a modal", () => {
    const { status, output } = lintSource(
      `import { Modal } from "~/components/ui/modal";
      import { DetailPanel } from "~/components/database/detail-panel";
      export function Probe() {
        return (
          <Modal isOpen onClose={() => {}} title="Raum">
            <DetailPanel header={<div />}>
              <div />
            </DetailPanel>
          </Modal>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(one-detail-per-type)");
    expect(output).toContain("DetailPanel");
  });

  it("allows field groups on a page and in a FormModal", () => {
    const { output } = lintSource(
      `import { FormModal } from "~/components/ui/modal";
      import { InfoSection, DataGrid, DataField } from "~/components/ui/detail-modal-components";
      export function Probe({ room }: { room: { name: string } }) {
        return (
          <>
            <InfoSection title="Raum" icon={null}>
              <DataGrid>
                <DataField label="Name">{room.name}</DataField>
              </DataGrid>
            </InfoSection>
            <FormModal isOpen onClose={() => {}} title="Kind nachtragen" onSubmit={() => {}}>
              <DataGrid>
                <DataField label="Raum">{room.name}</DataField>
              </DataGrid>
            </FormModal>
          </>
        );
      }`,
      "src/app/[tenant]/(protected)/rooms/[id]/page.tsx",
    );

    expect(output).not.toContain("bauart(one-detail-per-type)");
  });
});

describe("bauart/no-edit-overlay", () => {
  it("rejects a modal titled „… bearbeiten“ and a slide-over titled „… verwalten“", () => {
    const { status, output } = lintSource(
      `import { FormModal } from "~/components/ui/form-modal";
      import { SlideOver, SlideOverContent, SlideOverTitle } from "~/components/ui/slide-over";
      export function Probe({ name }: { name: string }) {
        return (
          <>
            <FormModal isOpen onClose={() => {}} title="Personal bearbeiten" onSubmit={() => {}}>
              <p>Felder</p>
            </FormModal>
            <SlideOver open onOpenChange={() => {}}>
              <SlideOverContent>
                <SlideOverTitle>{\`Rolle verwalten: \${name}\`}</SlideOverTitle>
              </SlideOverContent>
            </SlideOver>
          </>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-edit-overlay)");
    expect(output).toContain("Personal bearbeiten");
    expect(output).toContain("Rolle verwalten:");
  });

  it("sees the edit branch of a conditional title and of DatabaseFormModal mode", () => {
    const { status, output } = lintSource(
      `import { Modal } from "~/components/ui/modal";
      import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";
      export function Probe({ initial }: { initial: object | null }) {
        return (
          <>
            <Modal isOpen onClose={() => {}} title={initial ? "Ordner bearbeiten" : "Neuer Ordner"}>
              <p>Felder</p>
            </Modal>
            <DatabaseFormModal isOpen onClose={() => {}} mode={initial ? "edit" : "create"} config={{}} onSubmit={() => {}} />
          </>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("Ordner bearbeiten Neuer Ordner");
    expect(output).toContain('DatabaseFormModal mode="edit"');
  });

  it("accepts creating, adding and confirming in an overlay", () => {
    const { output } = lintSource(
      `import { FormModal } from "~/components/ui/form-modal";
      import { ConfirmationModal } from "~/components/ui/modal";
      import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";
      export function Probe() {
        return (
          <>
            <FormModal isOpen onClose={() => {}} title="Kind nachtragen" onSubmit={() => {}}>
              <p>Felder</p>
            </FormModal>
            <ConfirmationModal isOpen onClose={() => {}} onConfirm={() => {}} title="Bearbeitung abschließen?" confirmText="Abschließen">
              <p>Sicher?</p>
            </ConfirmationModal>
            <DatabaseFormModal isOpen onClose={() => {}} mode="create" config={{}} onSubmit={() => {}} />
          </>
        );
      }`,
    );

    expect(output).not.toContain("bauart(no-edit-overlay)");
  });

  it("leaves the kit directory, tests and the other portals alone", () => {
    const source = `import { FormModal } from "~/components/ui/form-modal";
      export function Probe() {
        return (
          <FormModal isOpen onClose={() => {}} title="Träger bearbeiten" onSubmit={() => {}}>
            <p>Felder</p>
          </FormModal>
        );
      }`;

    for (const path of [
      "src/components/ui/form-modal.tsx",
      "src/components/probe.test.tsx",
      "src/app/operator/provisioning/edit-organization-modal.tsx",
      "src/app/parents/page.tsx",
    ]) {
      expect(lintSource(source, path).output).not.toContain(
        "bauart(no-edit-overlay)",
      );
    }
  });

  it("tolerates only the baselined overlay at its recorded location", () => {
    const source = `import { Modal } from "~/components/ui/modal";
      export function Probe({ initial }: { initial: object | null }) {
        return (
          <Modal
            isOpen
            onClose={() => {}}
            title={initial ? "Schließtag bearbeiten" : "Schließtag anlegen"}
          >
            <p>Felder</p>
          </Modal>
        );
      }`;
    const baselined = lintSource(
      // Der Eintrag steht in der Baseline auf Zeile 89; darüber Leerzeilen,
      // damit das Element genau dort landet.
      `${"\n".repeat(85)}${source}`,
      "src/components/planning/closing-day-modal.tsx",
    );
    expect(baselined.output).not.toContain("bauart(no-edit-overlay)");

    const moved = lintSource(
      source,
      "src/components/planning/closing-day-modal.tsx",
    );
    expect(moved.status).toBe(1);
    expect(moved.output).toContain("bauart(no-edit-overlay)");
  });
});

describe("bauart/no-local-field-grid", () => {
  it("rejects a hand-written <dt>/<dd> field grid", () => {
    const { status, output } = lintSource(
      `export function Probe({ name }: { name: string }) {
        return (
          <dl className="space-y-3">
            <div>
              <dt className="text-xs text-gray-500">Vorname</dt>
              <dd className="text-sm text-gray-900">{name}</dd>
            </div>
          </dl>
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-local-field-grid)");
  });

  it("accepts the kit's DataField and DataGrid", () => {
    const { output } = lintSource(
      `import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
      export function Probe({ name }: { name: string }) {
        return (
          <DataGrid>
            <DataField label="Vorname">{name}</DataField>
          </DataGrid>
        );
      }`,
    );

    expect(output).not.toContain("bauart(no-local-field-grid)");
  });

  it("leaves the kit directory, tests and the other portals alone", () => {
    const source = `export function Probe() {
        return (
          <dl>
            <dt>Vorname</dt>
            <dd>Mila</dd>
          </dl>
        );
      }`;

    for (const path of [
      "src/components/ui/detail-modal-components.tsx",
      "src/components/probe.test.tsx",
      "src/app/operator/settings/page.tsx",
      "src/components/parent/child-master-data.tsx",
    ]) {
      expect(lintSource(source, path).output).not.toContain(
        "bauart(no-local-field-grid)",
      );
    }
  });

  it("tolerates exactly the baselined count per file and no more", () => {
    // custom-allowance-editor.tsx carries one <dt> in the baseline.
    const one = `export function Probe() {
        return (
          <dl>
            <dt>Zuschlag</dt>
            <dd>3 h</dd>
          </dl>
        );
      }`;
    expect(
      lintSource(one, "src/components/staff/custom-allowance-editor.tsx")
        .output,
    ).not.toContain("bauart(no-local-field-grid)");

    const two = `export function Probe() {
        return (
          <dl>
            <dt>Zuschlag</dt>
            <dd>3 h</dd>
            <dt>Grund</dt>
            <dd>Nachtdienst</dd>
          </dl>
        );
      }`;
    const grown = lintSource(
      two,
      "src/components/staff/custom-allowance-editor.tsx",
    );
    expect(grown.status).toBe(1);
    expect(grown.output).toContain("bauart(no-local-field-grid)");
  });
});

describe("bauart/no-own-skeleton", () => {
  it("rejects a hand-written pulse block on a gray fill", () => {
    const { status, output } = lintSource(
      `export function Probe() {
        return <div className="h-4 w-24 animate-pulse rounded bg-gray-200" />;
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-own-skeleton)");
  });

  it("sees through a template class list", () => {
    const { status, output } = lintSource(
      `export function Probe({ width }: { width: string }) {
        return (
          <div className={\`ml-3 h-4 \${width} animate-pulse rounded bg-gray-200\`} />
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-own-skeleton)");
  });

  it("rejects gray fills with Tailwind variants and opacity", () => {
    for (const className of [
      "animate-pulse bg-gray-200/50",
      "animate-pulse hover:bg-gray-200",
      "animate-pulse dark:bg-gray-700",
    ]) {
      const { status, output } = lintSource(
        `export function Probe() {
          return <div className="${className}" />;
        }`,
      );

      expect(status).toBe(1);
      expect(output).toContain("bauart(no-own-skeleton)");
    }
  });

  it("lets a pulsing live indicator through", () => {
    const source = `export function Probe({ occupied }: { occupied: boolean }) {
        return (
          <>
            <span className={occupied ? "bg-moto-red animate-pulse" : "bg-moto-green"} />
            <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-white" />
          </>
        );
      }`;

    expect(lintSource(source).output).not.toContain("bauart(no-own-skeleton)");
  });

  it("lets the kit skeleton and its consumers through", () => {
    const source = `import { Skeleton } from "~/components/ui/skeleton";
      export function Probe() {
        return <Skeleton className="h-4 w-24 bg-gray-700" />;
      }`;

    expect(lintSource(source).output).not.toContain("bauart(no-own-skeleton)");
  });

  it("exempts the kit, tests, stories and the other portals", () => {
    const source = `export function Probe() {
        return <div className="h-4 animate-pulse rounded bg-gray-200" />;
      }`;

    for (const path of [
      "src/components/ui/skeleton.tsx",
      "src/components/probe.test.tsx",
      "src/components/probe.stories.tsx",
      "src/app/parents/page.tsx",
      "src/components/school/room-board.tsx",
    ]) {
      expect(lintSource(source, path).output).not.toContain(
        "bauart(no-own-skeleton)",
      );
    }
  });

  it("tolerates exactly the baselined count per file and no more", () => {
    // birthday-list.tsx carries one pulse block in the baseline.
    const one = `export function Probe() {
        return <div className="h-12 animate-pulse rounded-xl bg-gray-100" />;
      }`;
    expect(
      lintSource(one, "src/components/dashboard/birthday-list.tsx").output,
    ).not.toContain("bauart(no-own-skeleton)");

    const two = `export function Probe() {
        return (
          <>
            <div className="h-12 animate-pulse rounded-xl bg-gray-100" />
            <div className="h-12 animate-pulse rounded-xl bg-gray-100" />
          </>
        );
      }`;
    const grown = lintSource(two, "src/components/dashboard/birthday-list.tsx");
    expect(grown.status).toBe(1);
    expect(grown.output).toContain("bauart(no-own-skeleton)");
  });
});

describe("bauart/no-raw-status-hex", () => {
  it("rejects a hex color inside an arbitrary-value class", () => {
    const { status, output } = lintSource(
      `export function Probe() {
        return <p className="mt-1 text-sm text-[#8A5600]">Offen</p>;
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-raw-status-hex)");
    expect(output).toContain("#8A5600");
  });

  it("rejects a hex constant and a template chunk", () => {
    const { status, output } = lintSource(
      `const FALLBACK = "#E5E7EB";
      export function Probe({ tone }: { tone: string }) {
        return (
          <div className={\`border-[#F78C10]/30 \${tone}\`} style={{ background: FALLBACK }} />
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("#E5E7EB");
    expect(output).toContain("#F78C10");
  });

  it("flags a bare three-digit hex but not an issue reference", () => {
    const short = lintSource(
      `export const MESSAGE_STYLE = { color: "#666" };`,
      "src/components/probe.ts",
    );
    expect(short.status).toBe(1);
    expect(short.output).toContain("bauart(no-raw-status-hex)");

    const reference = lintSource(
      `export const LABEL = "Siehe Rückfrage #405 und #3118";
      export const ANCHOR = "#section-2";`,
      "src/components/probe.ts",
    );
    expect(reference.output).not.toContain("bauart(no-raw-status-hex)");
  });

  it("flags short hex colors in arbitrary-value classes", () => {
    for (const color of ["#666", "#abcd"]) {
      const { status, output } = lintSource(
        `export function Probe() {
          return <p className="text-[${color}]">Offen</p>;
        }`,
      );

      expect(status).toBe(1);
      expect(output).toContain("bauart(no-raw-status-hex)");
      expect(output).toContain(color);
    }
  });

  it("exempts the token source, the manifest, test support and the other portals", () => {
    const source = `export const COLOR = "#83CD2D";`;

    for (const path of [
      "src/lib/location-helper.ts",
      "src/lib/favicon-variants.ts",
      "src/app/global-error.tsx",
      "src/test/fixtures/rooms.ts",
      "src/components/ui/status-badge.tsx",
      "src/components/probe.test.ts",
      "src/app/operator/tenants/page.tsx",
      "src/components/parent/child-card.tsx",
    ]) {
      expect(lintSource(source, path).output).not.toContain(
        "bauart(no-raw-status-hex)",
      );
    }
  });

  it("tolerates exactly the baselined count per file and no more", () => {
    // timetable-style.ts carries one hex literal in the baseline.
    const one = `export const EDGE = "#D1D5DB";`;
    expect(
      lintSource(one, "src/components/timetable/timetable-style.ts").output,
    ).not.toContain("bauart(no-raw-status-hex)");

    const two = `export const EDGE = "#D1D5DB";
      export const FILL = "#F3F4F6";`;
    const grown = lintSource(
      two,
      "src/components/timetable/timetable-style.ts",
    );
    expect(grown.status).toBe(1);
    expect(grown.output).toContain("bauart(no-raw-status-hex)");
  });
});

describe("bauart/no-disabled-menu-item", () => {
  it("rejects a menu entry that is disabled by literal", () => {
    const { status, output } = lintSource(
      `import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
      export function Probe() {
        return (
          <OverflowMenu
            items={[
              { label: "Exportieren", onClick: () => {}, disabled: true },
              { label: "Hilfe", href: "/help", disabled: true },
            ]}
          />
        );
      }`,
    );

    expect(status).toBe(1);
    expect(output).toContain("bauart(no-disabled-menu-item)");
  });

  it("lets a state-bound entry and a select placeholder through", () => {
    const source = `export function Probe({ busy }: { busy: boolean }) {
        const items = [
          { label: "Exportieren", onClick: () => {}, disabled: busy },
          { label: "Drucken", onClick: () => {}, disabled: !busy },
        ];
        const options = [{ value: "", label: "Bitte wählen", disabled: true }];
        return <div data-items={items.length} data-options={options.length} />;
      }`;

    expect(lintSource(source).output).not.toContain(
      "bauart(no-disabled-menu-item)",
    );
  });

  it("exempts the kit, tests and the other portals", () => {
    const source = `export const items = [
        { label: "Exportieren", onClick: () => {}, disabled: true },
      ];`;

    for (const path of [
      "src/components/ui/page-header/OverflowMenu.stories.tsx",
      "src/components/probe.test.tsx",
      "src/app/school/page.tsx",
      "src/components/operator/tenant-menu.tsx",
    ]) {
      expect(lintSource(source, path).output).not.toContain(
        "bauart(no-disabled-menu-item)",
      );
    }
  });
});
