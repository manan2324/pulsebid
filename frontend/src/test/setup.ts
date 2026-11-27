import { afterAll, afterEach, beforeAll } from "vitest";
import { server } from "@/mocks/server";
process.env.PULSEBID_API_KEY = "test-server-key-never-public";
process.env.ADMIN_PASSWORD = "test-password";
process.env.SESSION_SECRET = "test-session-secret-at-least-32-characters";
beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
