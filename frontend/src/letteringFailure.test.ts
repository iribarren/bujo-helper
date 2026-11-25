import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';

import {beforeAll, describe, expect, it, vi} from 'vitest';

import {PreviewPage, PrintPages} from '../wailsjs/go/main/App';
import {initPages} from './pages';
import {initPrint} from './print';
import {fakePdf, resetFakePdfs} from './previewMock';

// Both tabs when LetteringFonts fails: the error shows, every other preset
// still loads, the font picker stays disabled and Go gets no font (its
// default) instead of an exception.

vi.mock('pdfjs-dist/legacy/build/pdf.mjs', async () => (await import('./previewMock')).pdfjsMock);
vi.mock('pdfjs-dist/legacy/build/pdf.worker.min.mjs?url', () => ({default: '/pdf.worker.min.mjs'}));

// Vitest clears mock calls before each test, so the errors and LetteringFonts
// calls made while the tabs load are kept here.
const {formats, papers, cuts, kinds, covers, fontCalls} = vi.hoisted(() => ({
    fontCalls: {n: 0},
    formats: [
        {id: 'covers', label: 'covers', noun: 'cover', usesSize: true, usesCut: true, usesCrop: true, usesHeader: false, usesLettering: false, default: true},
        {id: 'fullpage', label: 'fullpage', noun: 'cover', usesSize: false, usesCut: false, usesCrop: false, usesHeader: true, usesLettering: true, default: false},
    ],
    papers: [{
        id: 'a4', label: 'A4', width: 210, height: 297, default: true,
        perPage: {covers: {standard: 25}, fullpage: {'': 1}}, headerPerPage: {fullpage: 1}, squarePerPage: {}, cropPerPage: {},
    }],
    cuts: [{id: 'border', label: 'Border', default: true}],
    kinds: [{id: 'dotgrid', label: 'Dot grid', usesCut: false, usesTitle: true, defaults: '{"spacing":5}', default: true}],
    covers: [{path: '/covers/a.jpg', name: 'a.jpg', width: 400, height: 600, thumbUrl: 'a'}],
}));

vi.mock('../wailsjs/go/main/App', async (importOriginal) => {
    const original = await importOriginal<Record<string, unknown>>();
    const stubs: Record<string, unknown> = {};
    for (const name of Object.keys(original)) {
        stubs[name] = vi.fn(async () => undefined);
    }
    stubs.PrintFormats = vi.fn(async () => formats);
    stubs.PrintSizes = vi.fn(async () => [{id: 'standard', label: 'Standard', coverHeight: 70, default: true}]);
    stubs.PrintPapers = vi.fn(async () => papers);
    stubs.PrintCutModes = vi.fn(async () => cuts);
    stubs.PrintCropModes = vi.fn(async () => [{id: 'none', label: 'None', usesSize: true, usesCut: true, frames: 1, default: true}]);
    stubs.ListCovers = vi.fn(async () => covers);
    stubs.PrintPages = vi.fn(async () => ({pages: 1, allSquare: false, widePerPage: null}));
    stubs.PreviewPDF = vi.fn(async () => ({url: '/preview?id=1', pages: 1}));
    stubs.PageKinds = vi.fn(async () => kinds);
    stubs.PagePages = vi.fn(async () => 1);
    stubs.PreviewPage = vi.fn(async () => ({url: '/preview?id=1', pages: 1}));
    stubs.LetteringFonts = vi.fn(async () => {
        fontCalls.n++;
        throw new Error('boom');
    });
    return stubs;
});

function el<T extends HTMLElement>(id: string): T {
    const node = document.getElementById(id);
    if (!node) throw new Error(`missing #${id}`);
    return node as T;
}

const printError: string[] = [];
const pagesError: string[] = [];

beforeAll(async () => {
    resetFakePdfs();
    fakePdf('/preview?id=1', 1);
    const html = readFileSync(resolve(__dirname, '../index.html'), 'utf8');
    document.body.innerHTML = html.slice(html.indexOf('<body>') + '<body>'.length, html.indexOf('</body>'));
    const tab = initPrint({
        outputFolder: () => '/covers',
        showError: (text: string) => printError.push(text),
        hideError: () => {},
        mediaTypes: () => [],
        mediaType: () => 'books',
        previewDelay: 0,
    });
    tab.activate();
    initPages({defaultDir: () => '/covers', showError: (text: string) => pagesError.push(text), hideError: () => {}, previewDelay: 0});
});

describe('LetteringFonts fails', () => {
    for (const [name, showError, prefix] of [['Print covers', printError, 'print'], ['Make pages', pagesError, 'pages']] as const) {
        it(`${name}: shows the error and keeps the picker disabled`, async () => {
            await vi.waitFor(() => expect(showError).toContain('Could not load lettering fonts: boom'));
            expect(el<HTMLButtonElement>(`${prefix}-font`).disabled).toBe(true);
            expect(el(`${prefix}-font-list`).querySelectorAll('[role="option"]').length).toBe(0);
            expect(el(`${prefix}-paper`).querySelectorAll('[role="radio"]').length).toBe(papers.length);
        });
    }

    it('asks LetteringFonts once for both tabs while the call is pending', () => {
        // letteringLoad.test.ts covers the retry after a failure.
        expect(fontCalls.n).toBe(1);
    });

    it('Print covers: loads the other presets and sends no font', async () => {
        await vi.waitFor(() => expect(el<HTMLSelectElement>('print-format').options.length).toBe(formats.length));
        await vi.waitFor(() => expect(el('gallery').children.length).toBe(covers.length));
        const select = el<HTMLSelectElement>('print-format');
        select.value = 'fullpage';
        select.dispatchEvent(new Event('change'));
        expect(el('print-font-field').hidden).toBe(false);
        el('select-all').click();
        await vi.waitFor(() => {
            const calls = vi.mocked(PrintPages).mock.calls;
            expect(calls.length).toBeGreaterThan(0);
            expect({...calls[calls.length - 1][1]}).toEqual(expect.objectContaining({format: 'fullpage', font: '', colourIn: false}));
        });
    });

    it('Make pages: loads the page kinds and previews with no font', async () => {
        await vi.waitFor(() => expect(el('pages-kinds').querySelectorAll('[role="radio"]').length).toBe(kinds.length));
        const title = el<HTMLInputElement>('pages-title');
        title.value = 'Ideas';
        title.dispatchEvent(new Event('input'));
        await vi.waitFor(() => {
            const calls = vi.mocked(PreviewPage).mock.calls;
            expect(calls.length).toBeGreaterThan(0);
            expect({...calls[calls.length - 1][1]}).toEqual(expect.objectContaining({title: 'Ideas', font: '', colourIn: false}));
        });
    });
});
