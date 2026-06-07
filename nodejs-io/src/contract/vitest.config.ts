// Vitest configuration for contract tests.
//
// The default vitest test runner expects module resolution to match
// the test file extension. NodeNext/ESM requires `.js` extensions on
// relative imports, but vitest with vite-node can resolve `.ts` files
// without an extension.
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["src/contract/**/*.test.ts"],
  },
});
