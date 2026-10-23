import { defineConfig } from "vitest/config";
export default defineConfig({ resolve: { alias: { "cloudflare:workers": new URL("./test/cfstub.ts", import.meta.url).pathname } } });
