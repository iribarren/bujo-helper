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
