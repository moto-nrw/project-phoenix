import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import {
  DataField,
  DataGrid,
  DetailIcons,
  InfoSection,
  InfoText,
} from "./detail-modal-components";

const meta = {
  title: "components/ui/DetailModalComponents",
} satisfies Meta;

export default meta;

type Story = StoryObj<typeof meta>;

export const DataFieldDefault: Story = {
  render: () => (
    <dl>
      <DataField label="Name">Max Mustermann</DataField>
    </dl>
  ),
};

export const DataFieldMono: Story = {
  render: () => (
    <dl>
      <DataField label="ID" mono>
        a1b2c3d4-e5f6
      </DataField>
    </dl>
  ),
};

export const DataFieldFullWidth: Story = {
  render: () => (
    <DataGrid>
      <DataField label="Vorname">Max</DataField>
      <DataField label="Nachname">Mustermann</DataField>
      <DataField label="Adresse" fullWidth>
        Musterstraße 1, 12345 Musterstadt
      </DataField>
    </DataGrid>
  ),
};

export const DataGridExample: Story = {
  render: () => (
    <DataGrid>
      <DataField label="Vorname">Max</DataField>
      <DataField label="Nachname">Mustermann</DataField>
      <DataField label="Geburtsdatum">01.01.2015</DataField>
      <DataField label="Gruppe">Löwen</DataField>
    </DataGrid>
  ),
};

export const InfoTextDefault: Story = {
  render: () => (
    <InfoText>
      Dies ist ein Beispieltext für einen Informationsabschnitt ohne
      Datenraster.
    </InfoText>
  ),
};

export const InfoSectionDefault: Story = {
  render: () => (
    <InfoSection title="Persönliche Daten" icon={DetailIcons.person}>
      <DataGrid>
        <DataField label="Vorname">Max</DataField>
        <DataField label="Nachname">Mustermann</DataField>
      </DataGrid>
    </InfoSection>
  ),
};

export const InfoSectionColors: Story = {
  render: () => (
    <div className="flex flex-col gap-3">
      <InfoSection title="Blau" icon={DetailIcons.person} accentColor="blue">
        <InfoText>Blauer Akzent</InfoText>
      </InfoSection>
      <InfoSection
        title="Orange"
        icon={DetailIcons.building}
        accentColor="orange"
      >
        <InfoText>Oranger Akzent</InfoText>
      </InfoSection>
      <InfoSection title="Grün" icon={DetailIcons.check} accentColor="green">
        <InfoText>Grüner Akzent</InfoText>
      </InfoSection>
      <InfoSection title="Rot" icon={DetailIcons.x} accentColor="red">
        <InfoText>Roter Akzent</InfoText>
      </InfoSection>
    </div>
  ),
};

/**
 * Realistic composition: multiple InfoSections with nested DataGrid/DataField,
 * mirroring how these parts are combined in an actual detail modal.
 */
export const Composition: Story = {
  render: () => (
    <div className="flex max-w-md flex-col gap-3">
      <InfoSection
        title="Stammdaten"
        icon={DetailIcons.person}
        accentColor="blue"
      >
        <DataGrid>
          <DataField label="Vorname">Max</DataField>
          <DataField label="Nachname">Mustermann</DataField>
          <DataField label="Geburtsdatum">01.01.2015</DataField>
          <DataField label="ID" mono>
            a1b2c3d4-e5f6
          </DataField>
        </DataGrid>
      </InfoSection>
      <InfoSection title="Notizen" icon={DetailIcons.notes} accentColor="amber">
        <InfoText>Keine besonderen Vorkommnisse.</InfoText>
      </InfoSection>
    </div>
  ),
};
