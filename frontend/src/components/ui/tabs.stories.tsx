import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";

const meta: Meta<typeof Tabs> = {
  title: "components/ui/Tabs",
  component: Tabs,
};

export default meta;
type Story = StoryObj<typeof Tabs>;

export const Default: Story = {
  render: () => (
    <Tabs defaultValue="overview" className="w-[400px]">
      <TabsList variant="default">
        <TabsTrigger value="overview">Übersicht</TabsTrigger>
        <TabsTrigger value="details">Details</TabsTrigger>
        <TabsTrigger value="settings">Einstellungen</TabsTrigger>
      </TabsList>
      <TabsContent value="overview">Inhalt der Übersicht.</TabsContent>
      <TabsContent value="details">Inhalt der Details.</TabsContent>
      <TabsContent value="settings">Inhalt der Einstellungen.</TabsContent>
    </Tabs>
  ),
};

export const Line: Story = {
  render: () => (
    <Tabs defaultValue="overview" className="w-[400px]">
      <TabsList variant="line">
        <TabsTrigger value="overview">Übersicht</TabsTrigger>
        <TabsTrigger value="details">Details</TabsTrigger>
        <TabsTrigger value="settings">Einstellungen</TabsTrigger>
      </TabsList>
      <TabsContent value="overview">Inhalt der Übersicht.</TabsContent>
      <TabsContent value="details">Inhalt der Details.</TabsContent>
      <TabsContent value="settings">Inhalt der Einstellungen.</TabsContent>
    </Tabs>
  ),
};
