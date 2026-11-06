import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { UploadSection } from "./upload-section";

const noop = () => {
  // no-op for story fixtures
};

const meta: Meta<typeof UploadSection> = {
  title: "components/import/UploadSection",
  component: UploadSection,
  args: {
    isLoading: false,
    uploadedFile: null,
    onFileSelect: noop,
  },
};

export default meta;

type Story = StoryObj<typeof UploadSection>;

export const Default: Story = {};

export const Loading: Story = {
  args: {
    isLoading: true,
  },
};

export const WithUploadedFile: Story = {
  args: {
    uploadedFile: new File(["dummy content"], "schueler.csv", {
      type: "text/csv",
    }),
  },
};
