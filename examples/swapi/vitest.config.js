import { defineConfig } from "vitest/config";

export default defineConfig({
    test: {
        include: [
            "resources/**/*.test.{ts,tsx}",
        ],
    },
});
