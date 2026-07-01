import type { StorybookConfig } from "@storybook/nextjs-vite";
import path from "node:path";
import { fileURLToPath } from "node:url";

const dirname = path.dirname(fileURLToPath(import.meta.url));

const config: StorybookConfig = {
  stories: ["../src/components/**/*.stories.@(ts|tsx)"],
  addons: ["@storybook/addon-a11y"],
  framework: { name: "@storybook/nextjs-vite", options: {} },
  staticDirs: [
    { from: "../public", to: "/" },
    {
      from: path.resolve(
        dirname,
        "../node_modules/@fontsource/geist-sans/files",
      ),
      to: "/assets/files",
    },
  ],
  async viteFinal(viteConfig) {
    viteConfig.publicDir = false;
    viteConfig.resolve ??= {};
    viteConfig.resolve.alias = {
      ...viteConfig.resolve.alias,
      "next-auth/react": path.resolve(dirname, "mocks/next-auth-react.tsx"),
      "~": path.resolve(dirname, "../src"),
      "@": path.resolve(dirname, "../src"),
      "~storybook": path.resolve(dirname, "."),
    };
    return viteConfig;
  },
};
export default config;
