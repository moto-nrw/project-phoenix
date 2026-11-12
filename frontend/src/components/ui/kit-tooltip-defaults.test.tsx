import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { ConfirmDeleteModal } from "./confirm-delete-modal";
import { DataTable } from "./data-table";
import { DatePicker } from "./date-picker";
import { Dropzone } from "./dropzone";
import { EditActions } from "./edit-actions";
import { ExportMenu, exportFormatOptions } from "./export-menu";
import { ConfirmationModal, Modal } from "./modal";
import { ModalProvider } from "./modal-context";
import { OverflowMenu } from "./page-header/OverflowMenu";
import { SectionCard } from "./section-card";

// Kit-Bausteine bringen ihre Knopf-Erklärung mit (#6744): Der Standard kommt
// als zugängliche Beschreibung an, und eine Prop ersetzt ihn.

function inModal(children: ReactNode) {
  return render(<ModalProvider>{children}</ModalProvider>);
}

const button = (name: string | RegExp) => screen.getByRole("button", { name });

describe("kit tooltip defaults", () => {
  it("Modal: the close X explains itself and takes a translated sentence", () => {
    const { unmount } = inModal(
      <Modal isOpen onClose={vi.fn()} title="Fenster">
        Inhalt
      </Modal>,
    );
    expect(button("Modal schließen")).toHaveAccessibleDescription(
      "Schließt das Fenster.",
    );
    unmount();

    inModal(
      <Modal
        isOpen
        onClose={vi.fn()}
        title="Window"
        closeLabel="Close"
        closeTooltip="Closes the window."
      >
        Content
      </Modal>,
    );
    expect(button("Close")).toHaveAccessibleDescription("Closes the window.");
  });

  it("Modal: a translated close label drops the German default", () => {
    inModal(
      <Modal isOpen onClose={vi.fn()} title="Window" closeLabel="Close">
        Content
      </Modal>,
    );
    expect(button("Close")).not.toHaveAccessibleDescription();
  });

  it("ConfirmationModal: confirm has a default, cancel stays plain", () => {
    const { unmount } = inModal(
      <ConfirmationModal
        isOpen
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        title="Archivieren?"
        confirmText="Archivieren"
      >
        Text
      </ConfirmationModal>,
    );
    expect(button("Archivieren")).toHaveAccessibleDescription(
      "Führt die Aktion sofort aus, ohne weitere Nachfrage.",
    );
    expect(button("Abbrechen")).not.toHaveAccessibleDescription();
    unmount();

    inModal(
      <ConfirmationModal
        isOpen
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        title="Archivieren?"
        confirmText="Archivieren"
        confirmTooltip="Archiviert das Kind und benachrichtigt die Eltern."
      >
        Text
      </ConfirmationModal>,
    );
    expect(button("Archivieren")).toHaveAccessibleDescription(
      "Archiviert das Kind und benachrichtigt die Eltern.",
    );
  });

  it("ConfirmDeleteModal: both steps explain themselves", () => {
    const props = {
      isOpen: true,
      title: "Raum löschen",
      description: "Der Raum wird gelöscht.",
      onConfirm: vi.fn(),
      onClose: vi.fn(),
      loading: false,
      error: "",
    } as const;
    const { unmount } = inModal(
      <ConfirmDeleteModal {...props} gate={{ mode: "twoStep" }} />,
    );
    expect(button("Ja, löschen")).toHaveAccessibleDescription(
      "Löscht noch nicht, moto fragt ein letztes Mal nach.",
    );
    unmount();

    inModal(
      <ConfirmDeleteModal
        {...props}
        gate={{
          mode: "textConfirm",
          expected: "Raum 1",
          inputId: "confirm",
          label: "Name eingeben",
        }}
        confirmTooltip="Löscht den Raum sofort, seine Termine bleiben."
      />,
    );
    expect(button("Endgültig löschen")).toHaveAccessibleDescription(
      "Löscht den Raum sofort, seine Termine bleiben.",
    );
  });

  it("EditActions: plain save stays plain, a side effect is named", () => {
    const { unmount } = render(
      <EditActions onCancel={vi.fn()} onSave={vi.fn()} />,
    );
    expect(button("Speichern")).not.toHaveAccessibleDescription();
    unmount();

    render(
      <EditActions
        onCancel={vi.fn()}
        onSave={vi.fn()}
        saveTooltip="Speichert die Zeiten und benachrichtigt die Eltern."
        disabled
        disabledReason="Ändern Sie zuerst ein Feld."
      />,
    );
    const save = button("Speichern");
    expect(save).toHaveAttribute("aria-disabled", "true");
    expect(save).toHaveAccessibleDescription("Ändern Sie zuerst ein Feld.");
  });

  it("Dropzone: the file button explains itself", () => {
    const props = {
      accept: ".csv",
      legend: "Bereich zum Hochladen",
      label: "Datei hierher ziehen",
      dragLabel: "Datei hier ablegen",
      buttonLabel: "Datei auswählen",
      onFile: vi.fn(),
    };
    const { unmount } = render(<Dropzone {...props} />);
    expect(button("Datei auswählen")).toHaveAccessibleDescription(
      "Öffnet die Dateiauswahl Ihres Geräts.",
    );
    unmount();

    render(<Dropzone {...props} buttonTooltip="Lädt die Liste sofort hoch." />);
    expect(button("Datei auswählen")).toHaveAccessibleDescription(
      "Lädt die Liste sofort hoch.",
    );
  });

  it("SectionCard: the toggle names what each state shows", () => {
    const { unmount } = render(
      <SectionCard title="Notizen" collapsible defaultCollapsed>
        Inhalt
      </SectionCard>,
    );
    expect(button("Notizen ausklappen")).toHaveAccessibleDescription(
      "Zeigt den Inhalt dieses Abschnitts.",
    );
    unmount();

    render(
      <SectionCard
        title="Notizen"
        collapsible
        collapseTooltip={{
          expand: "Lädt und zeigt die Notizen.",
          collapse: "Blendet die Notizen aus.",
        }}
      >
        Inhalt
      </SectionCard>,
    );
    expect(button("Notizen einklappen")).toHaveAccessibleDescription(
      "Blendet die Notizen aus.",
    );
  });

  it("DataTable: „Mehr laden“ explains itself", () => {
    const rows = Array.from({ length: 3 }, (_, i) => ({ id: String(i) }));
    const columns = [
      { key: "id", header: "Nr.", render: (row: { id: string }) => row.id },
    ];
    const { unmount } = render(
      <DataTable
        columns={columns}
        rows={rows}
        getRowKey={(row) => row.id}
        pageSize={1}
      />,
    );
    expect(button(/Mehr laden/)).toHaveAccessibleDescription(
      "Zeigt die nächsten Einträge unter den bisherigen.",
    );
    unmount();

    render(
      <DataTable
        columns={columns}
        rows={rows}
        getRowKey={(row) => row.id}
        pageSize={1}
        loadMoreTooltip="Zeigt die nächsten Kinder."
      />,
    );
    expect(button(/Mehr laden/)).toHaveAccessibleDescription(
      "Zeigt die nächsten Kinder.",
    );
  });

  it("OverflowMenu: the kebab says it shows more actions", () => {
    const items = [{ label: "Archivieren", onClick: vi.fn() }];
    const { unmount } = render(<OverflowMenu items={items} />);
    expect(button("Weitere Aktionen")).toHaveAccessibleDescription(
      "Zeigt weitere Aktionen.",
    );
    unmount();

    render(<OverflowMenu items={items} triggerTooltip="Shows more actions." />);
    expect(button("Weitere Aktionen")).toHaveAccessibleDescription(
      "Shows more actions.",
    );
  });

  it("ExportMenu: the trigger says nothing downloads yet", () => {
    const options = exportFormatOptions(["pdf", "csv"]);
    const { unmount } = render(
      <ExportMenu options={options} onSelect={vi.fn()} />,
    );
    expect(button("Herunterladen")).toHaveAccessibleDescription(
      "Zeigt die Formate zur Auswahl, noch wird nichts heruntergeladen.",
    );
    unmount();

    render(
      <ExportMenu
        options={options}
        onSelect={vi.fn()}
        tooltip="Zeigt die Formate der Liste."
      />,
    );
    expect(button("Herunterladen")).toHaveAccessibleDescription(
      "Zeigt die Formate der Liste.",
    );
  });

  it("DatePicker: clearing explains itself and takes translated labels", () => {
    const { unmount } = render(
      <DatePicker value={new Date(2026, 8, 9)} onChange={vi.fn()} />,
    );
    expect(button("Datum löschen")).toHaveAccessibleDescription(
      "Leert das Feld, danach ist kein Tag mehr gewählt.",
    );
    unmount();

    render(
      <DatePicker
        value={new Date(2026, 8, 9)}
        onChange={vi.fn()}
        labels={{ clear: "Clear date", clearHint: "Empties the field." }}
      />,
    );
    expect(button("Clear date")).toHaveAccessibleDescription(
      "Empties the field.",
    );
  });
});
