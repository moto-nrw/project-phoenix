import type { StorybookConfig } from "@storybook/nextjs-vite";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { storybookProcessEnv, storybookPublicEnvDefines } from "./mocks/env.js";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const storybookEnvModule = path.resolve(dirname, "mocks/env.js");

Object.assign(process.env, storybookProcessEnv, {
  SKIP_ENV_VALIDATION: "true",
});

const config: StorybookConfig = {
  stories: ["../src/components/**/*.stories.@(ts|tsx)"],
  addons: ["@storybook/addon-a11y"],
  framework: { name: "@storybook/nextjs-vite", options: {} },
  async viteFinal(viteConfig) {
    viteConfig.publicDir = path.resolve(dirname, "../public");
    viteConfig.define = {
      ...viteConfig.define,
      ...storybookPublicEnvDefines,
    };
    viteConfig.resolve ??= {};
    viteConfig.resolve.alias = {
      ...viteConfig.resolve.alias,
      "next-auth/react": path.resolve(dirname, "mocks/next-auth-react.tsx"),
      "~/env": storybookEnvModule,
      "@/env": storybookEnvModule,
      "~": path.resolve(dirname, "../src"),
      "@": path.resolve(dirname, "../src"),
      "~storybook": path.resolve(dirname, "."),
    };
    return viteConfig;
  },
};
export default config;
