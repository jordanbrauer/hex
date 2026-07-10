// Vite-bundled TypeScript entry. Alpine + htmx are pulled from
// node_modules; vite tree-shakes what isn't used. The layout only
// references /js/app.js — no /vendor/ paths in TS mode.

import "htmx.org";
import Alpine from "alpinejs";

declare global {
    interface Window {
        Alpine: typeof Alpine;
    }
}

window.Alpine = Alpine;
Alpine.start();

document.body.addEventListener("htmx:beforeRequest", (event: Event) => {
    const detail = (event as CustomEvent).detail as {
        requestConfig?: { path?: string };
    };
    console.debug("htmx \u2192", detail.requestConfig?.path);
});
