import { defineConfig } from "vite";
import tailwindcss from "@tailwindcss/vite";

// Vite drives the frontend build for TS mode. Inputs live under
// resources/, outputs land under public/ so they can be served by
// hex/web (public serves at /).
//
// Consumers who want dev-server HMR can add `server: { origin: ... }`
// and reference bundled paths via vite's manifest.json. Out of scope
// for the starter — production build only.
export default defineConfig({
    plugins: [tailwindcss()],
    // outDir is public/, so disable vite's default publicDir behaviour
    // to avoid the "outDir === publicDir" warning.
    publicDir: false,
    build: {
        outDir: "public",
        emptyOutDir: false,
        manifest: false,
        rollupOptions: {
            input: {
                app: "resources/js/app.ts",
                style: "resources/css/app.css",
            },
            output: {
                entryFileNames: "js/[name].js",
                chunkFileNames: "js/[name].js",
                assetFileNames: (info) =>
                    info.name?.endsWith(".css")
                        ? "css/app.css"
                        : "assets/[name][extname]",
            },
        },
    },
});
