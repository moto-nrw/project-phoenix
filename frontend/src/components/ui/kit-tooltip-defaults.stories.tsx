import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { DataTable } from "./data-table";
import { Dropzone } from "./dropzone";
import { EditActions } from "./edit-actions";
import { ExportMenu, exportFormatOptions } from "./export-menu";
import { ConfirmDeleteModal } from "./confirm-delete-modal";
import { ConfirmationModal } from "./modal";
import { ModalProvider } from "./modal-context";
import { OverflowMenu } from "./page-header/OverflowMenu";
import { SectionCard } from "./section-card";

/**
 * Standardtexte der Kit-Bausteine (#6744): Mit der Maus auf einen Knopf
 * zeigen oder ihn mit Tab ansteuern zeigt den Satz, den der Baustein selbst
 * mitbringt. Aufrufer überschreiben ihn über die jeweilige Prop.
 */
function KitTooltipDefaults() {
  const rows = Array.from({ length: 4 }, (_, i) => ({ id: String(i + 1) }));
  return (
    <div className="flex max-w-xl flex-col gap-6">
      <SectionCard title="Notizen" collapsible>
        <p className="text-sm text-gray-600">
          Auf-/Zuklappen erklärt sich je Zustand.
        </p>
      </SectionCard>
      <div className="flex items-center gap-3">
        <OverflowMenu
          items={[{ label: "Archivieren", onClick: () => undefined }]}
        />
        <ExportMenu
          options={exportFormatOptions(["pdf", "csv"])}
          onSelect={() => undefined}
        />
      </div>
      <Dropzone
        accept=".csv"
        legend="Bereich zum Hochladen"
        label="Datei hierher ziehen"
        dragLabel="Datei hier ablegen"
        buttonLabel="Datei auswählen"
        onFile={() => undefined}
      />
      <DataTable
        columns={[{ key: "id", header: "Nr.", render: (row) => row.id }]}
        rows={rows}
        getRowKey={(row) => row.id}
        pageSize={2}
      />
      <EditActions
        onCancel={() => undefined}
        onSave={() => undefined}
        saveTooltip="Speichert die Zeiten und benachrichtigt die Eltern."
      />
    </div>
  );
}

const meta = {
  title: "components/ui/Knopf-Erklärungen",
  component: KitTooltipDefaults,
} satisfies Meta<typeof KitTooltipDefaults>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Standardtexte: Story = {};

/** Bestätigen, Abbrechen und das Schließen-X eines Bestätigungsdialogs. */
export const Bestaetigungsdialog: Story = {
  render: () => (
    <ModalProvider>
      <ConfirmationModal
        isOpen
        onClose={() => undefined}
        onConfirm={() => undefined}
        title="Kind archivieren?"
        confirmText="Archivieren"
      >
        Das Kind erscheint danach nicht mehr in den Listen.
      </ConfirmationModal>
    </ModalProvider>
  ),
};

/** Die zwei Schritte des Löschdialogs. */
export const Loeschdialog: Story = {
  render: () => (
    <ModalProvider>
      <ConfirmDeleteModal
        isOpen
        title="Raum löschen"
        description="Der Raum wird endgültig gelöscht."
        gate={{ mode: "twoStep" }}
        onConfirm={() => undefined}
        onClose={() => undefined}
        loading={false}
        error=""
      />
    </ModalProvider>
  ),
};
