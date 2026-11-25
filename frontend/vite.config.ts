import {defineConfig} from 'vite';

// pdf.js (the live preview) is most of the app's JavaScript, so it goes in
// its own chunk; the worker is a separate asset already
// (pdf.worker.min.mjs?url in src/preview.ts). The legacy pdf.js build alone
// is about 510 kB, just over Vite's 500 kB default warning. The app loads
// from disk, not the network, so the limit is raised to 600 kB: it still
// warns if pdf.js or the app's own chunk grows much.
export default defineConfig({
    build: {
        chunkSizeWarningLimit: 600,
        rollupOptions: {
            output: {
                manualChunks(id) {
                    if (id.includes('/node_modules/pdfjs-dist/')) {
                        return 'pdfjs';
                    }
                    return undefined;
                },
            },
        },
    },
});
