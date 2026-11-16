import {defineConfig} from 'vitest/config';

// Unit tests run in jsdom so DOM helpers (applyDom) can be tested.
export default defineConfig({
    test: {
        environment: 'jsdom',
        include: ['src/**/*.test.ts'],
    },
});
