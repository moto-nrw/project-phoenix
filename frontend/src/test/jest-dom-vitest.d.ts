import type * as matchers from "@testing-library/jest-dom/matchers";

// Resolve the Vitest module from this project. The declaration bundled by
// jest-dom resolves its own Vitest peer and misses this installation.
declare module "vitest" {
  interface Assertion<T = any> extends matchers.TestingLibraryMatchers<
    any,
    T
  > {}
  interface AsymmetricMatchersContaining extends matchers.TestingLibraryMatchers<
    any,
    any
  > {}
}
