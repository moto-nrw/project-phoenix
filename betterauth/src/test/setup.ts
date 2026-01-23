/**
 * Vitest test setup for betterauth.
 * Configures global mocks and environment variables.
 */
import { vi } from "vitest";

// Set test environment variables
process.env.DATABASE_URL = "postgres://test:test@localhost:5432/test";
process.env.INTERNAL_API_KEY = "dev-internal-key";
process.env.INTERNAL_API_URL = "http://test-server:8080";
process.env.BASE_DOMAIN = "example.com";
process.env.PORT = "3001";

// Suppress console output during tests (optional - comment out for debugging)
vi.spyOn(console, "log").mockImplementation(vi.fn());
vi.spyOn(console, "error").mockImplementation(vi.fn());
