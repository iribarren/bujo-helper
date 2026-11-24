import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';

import {describe, expect, it, vi} from 'vitest';

import {en} from './i18n/en';
import {SECTIONS, defaultItem, sectionShown, shownDefault, visibleItems, type View} from './view';

vi.mock('../wailsjs/go/main/App', () => ({
    MediaTypes: vi.fn(), PageKinds: vi.fn(), PrintCropModes: vi.fn(), PrintCutModes: vi.fn(),
    PrintFormats: vi.fn(), PrintPapers: vi.fn(), PrintSizes: vi.fn(),
}));

const papers = [
    {id: 'a4', label: 'A4', default: true},
    {id: 'letter', label: 'US Letter'},
    {id: 'a5', label: 'A5'},
];

describe('SECTIONS', () => {
    it('has a section for every tab, in tab order', () => {
        // A new tab must join the configuration: add it to SECTIONS.
        const html = readFileSync(resolve(__dirname, '../index.html'), 'utf8');
        const tabs = Array.from(html.matchAll(/role="tab"[^>]*data-tab="([^"]+)"/g), (m) => m[1]);
        expect(tabs.length).toBeGreaterThan(0);
        expect(SECTIONS.map((s) => s.id)).toEqual(tabs);
    });

    it('always shows Print', () => {
        expect(SECTIONS.find((s) => s.id === 'print')?.optional).toBe(false);
    });

    it('gives every section a list, and unique list ids with translated titles', () => {
        const ids = SECTIONS.flatMap((s) => s.lists.map((l) => l.id));
        expect(new Set(ids).size).toBe(ids.length);
        for (const section of SECTIONS) {
            expect(section.lists.length).toBeGreaterThan(0);
            expect(en[section.title]).toBeTruthy();
            for (const list of section.lists) expect(en[list.title]).toBeTruthy();
        }
    });
});

describe('sectionShown', () => {
    it('shows everything by default', () => {
        for (const s of SECTIONS) expect(sectionShown(undefined, s.id)).toBe(true);
    });

    it('hides an optional section, never Print or an unknown one', () => {
        const view: View = {hiddenSections: ['download', 'print', 'later']};
        expect(sectionShown(view, 'download')).toBe(false);
        expect(sectionShown(view, 'pages')).toBe(true);
        expect(sectionShown(view, 'print')).toBe(true);
        expect(sectionShown(view, 'later')).toBe(true);
    });
});

describe('visibleItems', () => {
    it('keeps every item, in order, when nothing is hidden', () => {
        expect(visibleItems(undefined, 'paper', papers)).toEqual(papers);
        expect(visibleItems({hidden: {cut: ['none']}}, 'paper', papers)).toEqual(papers);
    });

    it('leaves the hidden items out', () => {
        expect(visibleItems({hidden: {paper: ['letter']}}, 'paper', papers).map((p) => p.id)).toEqual(['a4', 'a5']);
    });

    it('keeps the default when every item is hidden', () => {
        expect(visibleItems({hidden: {paper: ['a4', 'letter', 'a5']}}, 'paper', papers).map((p) => p.id)).toEqual(['a4']);
        const noDefault = papers.map(({id, label}) => ({id, label}));
        expect(visibleItems({hidden: {paper: ['a4', 'letter', 'a5']}}, 'paper', noDefault).map((p) => p.id)).toEqual(['a4']);
        expect(visibleItems({hidden: {paper: ['x']}}, 'paper', [])).toEqual([]);
    });
});

describe('defaultItem and shownDefault', () => {
    it('picks the default, else the first item', () => {
        expect(defaultItem(papers)?.id).toBe('a4');
        expect(defaultItem(papers.slice(1))?.id).toBe('letter');
        expect(defaultItem([])).toBeUndefined();
    });

    it('falls back to the first shown item when the default is hidden', () => {
        expect(shownDefault(undefined, 'paper', papers)?.id).toBe('a4');
        expect(shownDefault({hidden: {paper: ['a4']}}, 'paper', papers)?.id).toBe('letter');
        expect(shownDefault({hidden: {paper: ['a4', 'letter']}}, 'paper', papers)?.id).toBe('a5');
    });
});
