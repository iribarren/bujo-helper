import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';

import {afterEach, beforeAll, describe, expect, it, vi} from 'vitest';

import {ExportPage, OpenFile, PagePages} from '../wailsjs/go/main/App';
import {useLanguage} from './language';
import {initPages} from './pages';

// The page kinds as PageKinds sends them (see app.go), plus a made-up kind
// that uses the cut mode and prints no title, to cover the shared Page and
// Lettering fieldsets for every kind.
const {kinds, papers, cuts} = vi.hoisted(() => ({
    kinds: [
        {id: 'dotgrid', label: 'Dot grid', usesCut: false, usesTitle: true, defaults: '{"spacing":5}', default: true},
        {id: 'cutout', label: 'Cut-out', usesCut: true, usesTitle: false, defaults: '{}', default: false},
    ],
    papers: [
        {id: 'a4', label: 'A4', width: 210, height: 297, default: false},
        {id: 'letter', label: 'US Letter', width: 215.9, height: 279.4, default: false},
        {id: 'a5', label: 'A5', width: 148, height: 210, default: true},
        {id: 'b5', label: 'B5', width: 176, height: 250, default: false},
    ],
    cuts: [
        {id: 'border', label: 'Border', default: true},
        {id: 'bleed', label: 'Bleed + crop marks', default: false},
        {id: 'none', label: 'None', default: false},
    ],
}));

vi.mock('../wailsjs/go/main/App', async (importOriginal) => {
    const original = await importOriginal<Record<string, unknown>>();
    const stubs: Record<string, unknown> = {};
    for (const name of Object.keys(original)) {
        stubs[name] = vi.fn(async () => undefined);
    }
    stubs.PageKinds = vi.fn(async () => kinds);
    stubs.PrintPapers = vi.fn(async () => papers);
    stubs.PrintCutModes = vi.fn(async () => cuts);
    stubs.PagePages = vi.fn(async () => 1);
    return stubs;
});

const deps = {
    defaultDir: () => '/covers',
    showError: vi.fn(),
    hideError: vi.fn(),
};

function el<T extends HTMLElement>(id: string): T {
    const node = document.getElementById(id);
    if (!node) throw new Error(`missing #${id}`);
    return node as T;
}

// radio is the radio of value in the radio group #groupId.
function radio(groupId: string, value: string): HTMLButtonElement {
    const node = el(groupId).querySelector<HTMLButtonElement>(`[role="radio"][data-value="${value}"]`);
    if (!node) throw new Error(`missing radio ${value} in #${groupId}`);
    return node;
}

function checked(groupId: string): string {
    return el(groupId).querySelector<HTMLElement>('[role="radio"][aria-checked="true"]')?.dataset.value ?? '';
}

function lastPagePages(): [string, {paper: string; cut: string; title: string}, string] {
    const calls = vi.mocked(PagePages).mock.calls;
    expect(calls.length).toBeGreaterThan(0);
    return calls[calls.length - 1] as never;
}

function setTitle(text: string) {
    const input = el<HTMLInputElement>('pages-title');
    input.value = text;
    input.dispatchEvent(new Event('input'));
}

beforeAll(async () => {
    const html = readFileSync(resolve(__dirname, '../index.html'), 'utf8');
    document.body.innerHTML = html.slice(html.indexOf('<body>') + '<body>'.length, html.indexOf('</body>'));
    initPages(deps);
    await vi.waitFor(() => expect(el('pages-kinds').querySelectorAll('[role="radio"]').length).toBe(kinds.length));
    await vi.waitFor(() => expect(el('pages-paper').querySelectorAll('[role="radio"]').length).toBe(papers.length));
});

afterEach(() => {
    radio('pages-kinds', 'dotgrid').click();
    radio('pages-paper', 'a5').click();
    setTitle('');
});

describe('page kinds', () => {
    it('are listed from PageKinds in a radio group, the default checked', () => {
        const group = el('pages-kinds');
        expect(group.getAttribute('role')).toBe('radiogroup');
        expect(Array.from(group.querySelectorAll('[role="radio"]'), (r) => r.textContent)).toEqual(['Dot grid', 'Cut-out']);
        expect(checked('pages-kinds')).toBe('dotgrid');
        expect(el('pages-kind-title').textContent).toBe('Dot grid');
    });

    it('switch the form with a click or the arrow keys', () => {
        radio('pages-kinds', 'cutout').click();
        expect(el('pages-kind-title').textContent).toBe('Cut-out');
        expect(el('pages-form-dotgrid').hidden).toBe(true);
        radio('pages-kinds', 'cutout').dispatchEvent(new KeyboardEvent('keydown', {key: 'ArrowDown', bubbles: true, cancelable: true}));
        expect(checked('pages-kinds')).toBe('dotgrid');
        expect(el('pages-kind-title').textContent).toBe('Dot grid');
        expect(el('pages-form-dotgrid').hidden).toBe(false);
    });

    it('send the chosen kind and its defaults when it has no form', async () => {
        radio('pages-kinds', 'cutout').click();
        await vi.waitFor(() => expect(lastPagePages()[0]).toBe('cutout'));
        expect(lastPagePages()[2]).toBe('{}');
    });

    it('follow the language', async () => {
        await useLanguage('es');
        try {
            expect(radio('pages-kinds', 'dotgrid').textContent).toBe('Cuadrícula de puntos');
            expect(el('pages-kind-title').textContent).toBe('Cuadrícula de puntos');
            // A kind without a translation keeps the Go label.
            expect(radio('pages-kinds', 'cutout').textContent).toBe('Cut-out');
        } finally {
            await useLanguage('en');
        }
    });
});

describe('dot grid form', () => {
    it('has a Dot spacing segmented group of 4, 5 and 6 mm, 5 by default', () => {
        const group = el('pages-dotgrid-spacing');
        expect(group.getAttribute('role')).toBe('radiogroup');
        expect(group.classList.contains('segmented')).toBe(true);
        expect(Array.from(group.querySelectorAll('[role="radio"]'), (r) => r.textContent)).toEqual(['4 mm', '5 mm', '6 mm']);
        expect(checked('pages-dotgrid-spacing')).toBe('5');
    });

    it('sends the chosen spacing as the options JSON', async () => {
        radio('pages-dotgrid-spacing', '6').click();
        await vi.waitFor(() => expect(lastPagePages()[2]).toBe('{"spacing":6}'));
        expect(lastPagePages()[0]).toBe('dotgrid');
        radio('pages-dotgrid-spacing', '5').click();
        await vi.waitFor(() => expect(lastPagePages()[2]).toBe('{"spacing":5}'));
    });
});

describe('Page fieldset', () => {
    it('has a Paper segmented group from PrintPapers, the default checked', () => {
        const group = el('pages-paper');
        expect(group.classList.contains('segmented')).toBe(true);
        expect(Array.from(group.querySelectorAll('[role="radio"]'), (r) => r.textContent)).toEqual(['A4', 'US Letter', 'A5', 'B5']);
        expect(checked('pages-paper')).toBe('a5');
        expect(el('pages-paper').closest('fieldset')?.id).toBe('pages-group-page');
    });

    it('sends the chosen paper', async () => {
        radio('pages-paper', 'b5').click();
        await vi.waitFor(() => expect(lastPagePages()[1].paper).toBe('b5'));
    });

    it('shapes the preview page like the paper', () => {
        radio('pages-paper', 'a4').click();
        expect(Number(el('pages-sheet').style.getPropertyValue('--page-ratio'))).toBeCloseTo(210 / 297, 5);
        radio('pages-paper', 'b5').click();
        expect(Number(el('pages-sheet').style.getPropertyValue('--page-ratio'))).toBeCloseTo(176 / 250, 5);
    });

    it('hides the cut mode for a kind that does not use it, and sends none', async () => {
        expect(el('pages-cut-field').hidden).toBe(true);
        radio('pages-paper', 'a4').click();
        await vi.waitFor(() => expect(lastPagePages()[1]).toEqual(expect.objectContaining({paper: 'a4', cut: ''})));
    });

    it('shows the cut mode for a kind that uses it, and sends it', async () => {
        radio('pages-kinds', 'cutout').click();
        expect(el('pages-cut-field').hidden).toBe(false);
        const cut = el<HTMLSelectElement>('pages-cut');
        cut.value = 'bleed';
        cut.dispatchEvent(new Event('change'));
        await vi.waitFor(() => expect(lastPagePages()[1].cut).toBe('bleed'));
    });

    it('has no credits page option', () => {
        expect(el('panel-pages').querySelector('input[type="checkbox"]')).toBeNull();
        expect(document.getElementById('pages-credits')).toBeNull();
    });
});

describe('Lettering fieldset', () => {
    it('has the title for a kind that prints one', async () => {
        expect(el('pages-title').closest('fieldset')?.id).toBe('pages-group-lettering');
        expect(el('pages-title-field').hidden).toBe(false);
        expect(el('pages-lettering-hint').hidden).toBe(true);
        expect(el('pages-title-field').textContent).toContain('Title (optional)');
        setTitle('  Ideas  ');
        await vi.waitFor(() => expect(lastPagePages()[1].title).toBe('Ideas'));
    });

    it('shows a muted hint instead for a kind that prints none, and sends no title', async () => {
        setTitle('Ideas');
        radio('pages-kinds', 'cutout').click();
        expect(el('pages-group-lettering').hidden).toBe(false);
        expect(el('pages-title-field').hidden).toBe(true);
        expect(el('pages-lettering-hint').hidden).toBe(false);
        expect(el('pages-lettering-hint').textContent).toBe('This page prints no title.');
        await vi.waitFor(() => expect(lastPagePages()[0]).toBe('cutout'));
        expect(lastPagePages()[1].title).toBe('');
    });
});

describe('page count', () => {
    it('shows the count from PagePages', async () => {
        vi.mocked(PagePages).mockResolvedValueOnce(3);
        radio('pages-paper', 'letter').click();
        await vi.waitFor(() => expect(el('pages-count').textContent).toBe('Page 1 of 3'));
    });

    it('ignores an answer to an older request', async () => {
        let answerOld: (n: number) => void = () => {};
        vi.mocked(PagePages).mockImplementationOnce(() => new Promise((done) => { answerOld = done; }));
        vi.mocked(PagePages).mockResolvedValueOnce(2);
        radio('pages-paper', 'a4').click();
        radio('pages-paper', 'b5').click();
        await vi.waitFor(() => expect(el('pages-count').textContent).toBe('Page 1 of 2'));
        answerOld(9);
        await Promise.resolve();
        await Promise.resolve();
        expect(el('pages-count').textContent).toBe('Page 1 of 2');
    });
});

describe('export', () => {
    it('saves the page with ExportPage and offers to open it', async () => {
        vi.mocked(ExportPage).mockResolvedValueOnce('/out/dotgrid_A5.pdf');
        radio('pages-dotgrid-spacing', '4').click();
        setTitle('Notes');
        el('pages-export').click();
        await vi.waitFor(() => expect(el('pages-status').textContent).toBe('Saved dotgrid_A5.pdf: 1 page'));
        const [kind, dir, settings, options] = vi.mocked(ExportPage).mock.calls[0];
        expect(kind).toBe('dotgrid');
        expect(dir).toBe('/covers');
        expect({...settings}).toEqual({paper: 'a5', cut: '', title: 'Notes'});
        expect(options).toBe('{"spacing":4}');
        expect(el('pages-open-pdf').hidden).toBe(false);
        expect(el<HTMLButtonElement>('pages-export').disabled).toBe(false);

        el('pages-open-pdf').click();
        expect(OpenFile).toHaveBeenCalledWith('/out/dotgrid_A5.pdf');
        radio('pages-dotgrid-spacing', '5').click();
    });

    it('keeps the last result when the dialog is cancelled', async () => {
        vi.mocked(ExportPage).mockResolvedValueOnce('/out/first.pdf');
        el('pages-export').click();
        await vi.waitFor(() => expect(el('pages-status').textContent).toBe('Saved first.pdf: 1 page'));
        vi.mocked(ExportPage).mockResolvedValueOnce('');
        el('pages-export').click();
        await vi.waitFor(() => expect(el<HTMLButtonElement>('pages-export').disabled).toBe(false));
        expect(el('pages-status').textContent).toBe('Saved first.pdf: 1 page');
        expect(el('pages-open-pdf').hidden).toBe(false);
    });

    it('reports an error', async () => {
        vi.mocked(ExportPage).mockRejectedValueOnce(new Error('boom'));
        el('pages-export').click();
        await vi.waitFor(() => expect(deps.showError).toHaveBeenCalledWith('Could not export PDF: boom'));
        expect(el('pages-status').textContent).toBe('');
        expect(el('pages-open-pdf').hidden).toBe(true);
        expect(el<HTMLButtonElement>('pages-export').disabled).toBe(false);
    });
});
