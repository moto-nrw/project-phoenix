/**
 * Test-only access to SWR's provider and cache type (#5007). Production code
 * never reaches this module: swr/no-cache-access accepts only the public
 * modules of ~/lib/swr outside test files.
 */
// swr/no-direct-import permits package imports inside this shared wrapper only.
export { SWRConfig, type Cache } from "swr";
