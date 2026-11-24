import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';

import {afterEach, describe, expect, it} from 'vitest';

import {applyTheme} from './theme';

const root = document.documentElement;

afterEach(() => {
    delete root.dataset.theme;
});

describe('applyTheme', () => {
    it('sets data-theme for Light and Dark', () => {
        applyTheme('light');
        expect(root.dataset.theme).toBe('light');
        applyTheme('dark');
        expect(root.dataset.theme).toBe('dark');
    });

    it('removes data-theme for System, so the system setting decides', () => {
        for (const theme of ['', undefined, 'sepia', 'Dark']) {
            root.dataset.theme = 'dark';
            applyTheme(theme);
            expect(root.hasAttribute('data-theme'), String(theme)).toBe(false);
        }
    });
});

describe('the dark tokens in style.css', () => {
    // The tokens of a block, as "--name: value" lines.
    function tokens(css: string, selector: string): string[] {
        const start = css.indexOf(`${selector} {`);
        expect(start, selector).toBeGreaterThanOrEqual(0);
        const body = css.slice(start, css.indexOf('}', start));
        return Array.from(body.matchAll(/(--[\w-]+):\s*([^;]+);/g), (m) => `${m[1]}: ${m[2].trim()}`);
    }

    it('match for the Dark theme and for System while the system is dark', () => {
        const css = readFileSync(resolve(__dirname, 'style.css'), 'utf8');
        const dark = tokens(css, ':root[data-theme="dark"]');
        expect(dark.length).toBeGreaterThan(0);
        expect(tokens(css, ':root:not([data-theme="light"])')).toEqual(dark);
    });
});
