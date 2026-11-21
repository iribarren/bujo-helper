import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';

import {afterEach, beforeAll, describe, expect, it, vi} from 'vitest';

import {ExportPage, OpenFile, PreviewPage} from '../wailsjs/go/main/App';
import {useLanguage} from './language';
import {pageForms} from './pageForms';
import {initPages, parseDefaults} from './pages';
import {fakePdf, renders, resetFakePdfs} from './previewMock';

vi.mock('pdfjs-dist/legacy/build/pdf.mjs', async () => (await import('./previewMock')).pdfjsMock);
vi.mock('pdfjs-dist/legacy/build/pdf.worker.min.mjs?url', () => ({default: '/pdf.worker.min.mjs'}));

// The page kinds as PageKinds sends them (see app.go), plus a made-up kind
// that uses the cut mode and prints no title, to cover the shared Page and
// Lettering fieldsets for every kind.
const {kinds, papers, cuts, fonts} = vi.hoisted(() => ({
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
    fonts: [
        ['go', 'Default', 'sans', 'stroke'],
        ['pacifico', 'Pacifico', 'brush', 'stroke'],
        ['londrina-outline', 'Londrina Outline', 'hollow', 'native'],
    ].map(([id, label, group, hollow]) => ({
        id, label, group, hollow, licence: 'OFL-1.1', url: `/fonts/${id}.ttf`, licenceUrl: `/fonts/${id}/licence`, default: id === 'go',
    })),
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
    stubs.PreviewPage = vi.fn(async () => ({url: '/preview?id=1', pages: 1}));
    stubs.LetteringFonts = vi.fn(async () => fonts);
    return stubs;
});

const deps = {
    defaultDir: () => '/covers',
    showError: vi.fn(),
    hideError: vi.fn(),
    // No debounce: the preview tests cover it.
    previewDelay: 0,
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

function lastPreviewPage(): [string, {paper: string; cut: string; title: string; font: string; colourIn: boolean}, string] {
    const calls = vi.mocked(PreviewPage).mock.calls;
    expect(calls.length).toBeGreaterThan(0);
    return calls[calls.length - 1] as never;
}

// previewDone waits until the preview has drawn the latest options.
async function previewDone() {
    await vi.waitFor(() => expect(el('pages-sheet').getAttribute('aria-busy')).toBe('false'));
}

// pickFont chooses the font id in the Lettering font picker.
function pickFont(id: string) {
    el('pages-font').click();
    const option = el('pages-font-list').querySelector<HTMLElement>(`[role="option"][data-value="${id}"]`);
    if (!option) throw new Error(`missing font ${id}`);
    option.click();
}

function setColourIn(on: boolean) {
    const box = el<HTMLInputElement>('pages-colour-in');
    if (box.checked !== on) box.click();
}

function setTitle(text: string) {
    const input = el<HTMLInputElement>('pages-title');
    input.value = text;
    input.dispatchEvent(new Event('input'));
}

beforeAll(async () => {
    resetFakePdfs();
    fakePdf('/preview?id=1', 1);
    const html = readFileSync(resolve(__dirname, '../index.html'), 'utf8');
    document.body.innerHTML = html.slice(html.indexOf('<body>') + '<body>'.length, html.indexOf('</body>'));
    initPages(deps);
    await vi.waitFor(() => expect(el('pages-kinds').querySelectorAll('[role="radio"]').length).toBe(kinds.length));
    await vi.waitFor(() => expect(el('pages-paper').querySelectorAll('[role="radio"]').length).toBe(papers.length));
    await vi.waitFor(() => expect(el('pages-font-list').querySelectorAll('[role="option"]').length).toBe(fonts.length));
});

afterEach(() => {
    radio('pages-kinds', 'dotgrid').click();
    radio('pages-paper', 'a5').click();
    setTitle('');
    pickFont('go');
    setColourIn(false);
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
        await vi.waitFor(() => expect(lastPreviewPage()[0]).toBe('cutout'));
        expect(lastPreviewPage()[2]).toBe('{}');
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
        await vi.waitFor(() => expect(lastPreviewPage()[2]).toBe('{"spacing":6}'));
        expect(lastPreviewPage()[0]).toBe('dotgrid');
        radio('pages-dotgrid-spacing', '5').click();
        await vi.waitFor(() => expect(lastPreviewPage()[2]).toBe('{"spacing":5}'));
    });

    // spacingOf builds a dot grid form from defaults, as the tab does, and
    // returns the checked spacing and the spacing it sends.
    function spacingOf(defaults: Record<string, unknown>): {checked: string; sent: unknown} {
        const box = document.createElement('div');
        const form = pageForms.dotgrid(box, 'test-dotgrid', defaults, () => {});
        return {
            checked: box.querySelector<HTMLElement>('[role="radio"][aria-checked="true"]')?.dataset.value ?? '',
            sent: form.options().spacing,
        };
    }

    const fallbacks: Array<[name: string, defaults: Record<string, unknown>]> = [
        ['a default spacing it does not offer', {spacing: 7}],
        ['a default spacing that is not a number', {spacing: 'wide'}],
        ['no default spacing', {}],
        ['malformed defaults JSON', parseDefaults('{"spacing":')],
    ];
    for (const [name, defaults] of fallbacks) {
        it(`checks and sends 5 mm for ${name}`, () => {
            expect(spacingOf(defaults)).toEqual({checked: '5', sent: 5});
        });
    }

    it('checks and sends an offered default spacing', () => {
        expect(spacingOf({spacing: 4})).toEqual({checked: '4', sent: 4});
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
        await vi.waitFor(() => expect(lastPreviewPage()[1].paper).toBe('b5'));
    });

    it('shows "Loading preview…" over the preview well until the page is drawn', async () => {
        const indicator = document.querySelector<HTMLElement>('.pages-preview .page-well .preview-loading');
        expect(indicator).not.toBeNull();
        radio('pages-paper', 'b5').click();
        expect(indicator?.dataset.active).toBe('true');
        expect(indicator?.textContent).toBe('Loading preview…');
        await previewDone();
        expect(indicator?.dataset.active).toBe('false');
        expect(indicator?.textContent).toBe('');
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
        await vi.waitFor(() => expect(lastPreviewPage()[1]).toEqual(expect.objectContaining({paper: 'a4', cut: ''})));
    });

    it('shows the cut mode for a kind that uses it, and sends it', async () => {
        radio('pages-kinds', 'cutout').click();
        expect(el('pages-cut-field').hidden).toBe(false);
        const cut = el<HTMLSelectElement>('pages-cut');
        cut.value = 'bleed';
        cut.dispatchEvent(new Event('change'));
        await vi.waitFor(() => expect(lastPreviewPage()[1].cut).toBe('bleed'));
    });

    it('has no credits page option', () => {
        expect(el('pages-group-page').querySelector('input[type="checkbox"]')).toBeNull();
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
        await vi.waitFor(() => expect(lastPreviewPage()[1].title).toBe('Ideas'));
    });

    it('shows a muted hint instead for a kind that prints none, and sends no title', async () => {
        setTitle('Ideas');
        radio('pages-kinds', 'cutout').click();
        expect(el('pages-group-lettering').hidden).toBe(false);
        expect(el('pages-title-field').hidden).toBe(true);
        expect(el('pages-lettering-hint').hidden).toBe(false);
        expect(el('pages-lettering-hint').textContent).toBe('This page prints no title.');
        await vi.waitFor(() => expect(lastPreviewPage()[0]).toBe('cutout'));
        expect(lastPreviewPage()[1].title).toBe('');
    });

    for (const kind of ['dotgrid', 'cutout']) {
        it(`${kind}: has the font picker and Colour-in, the default font chosen`, () => {
            radio('pages-kinds', kind).click();
            expect(el('pages-font').closest('fieldset')?.id).toBe('pages-group-lettering');
            expect(el('pages-font-field').hidden).toBe(false);
            expect(el('pages-colour-in-field').hidden).toBe(false);
            expect(el('pages-font').querySelector('.font-picker-name')?.textContent).toBe('Default');
            expect(el<HTMLInputElement>('pages-colour-in').checked).toBe(false);
        });
    }

    it('sends the chosen font and Colour-in', async () => {
        pickFont('pacifico');
        setColourIn(true);
        await vi.waitFor(() => expect(lastPreviewPage()[1]).toEqual(expect.objectContaining({font: 'pacifico', colourIn: true})));
    });

    it('sends the stored Colour-in with a hollow font', async () => {
        pickFont('londrina-outline');
        expect(el<HTMLInputElement>('pages-colour-in').checked).toBe(true);
        expect(el('pages-colour-in-hint').textContent).toBe('This font is hollow by design, so letters always print as outlines.');
        await vi.waitFor(() => expect(lastPreviewPage()[1]).toEqual(expect.objectContaining({font: 'londrina-outline', colourIn: false})));
    });
});

describe('live preview', () => {
    it('draws the page PreviewPage rendered and counts its pages', async () => {
        fakePdf('/preview?id=3', 3);
        vi.mocked(PreviewPage).mockResolvedValueOnce({url: '/preview?id=3', pages: 3});
        radio('pages-paper', 'letter').click();
        await vi.waitFor(() => expect(el('pages-count').textContent).toBe('Page 1 of 3'));
        await vi.waitFor(() => expect(el('pages-sheet').querySelector('canvas')).not.toBeNull());
        expect(renders.at(-1)).toEqual({url: '/preview?id=3', page: 1});
        expect(el<HTMLButtonElement>('pages-prev').disabled).toBe(true);
        el('pages-next').click();
        await vi.waitFor(() => expect(renders.at(-1)).toEqual({url: '/preview?id=3', page: 2}));
        expect(el('pages-count').textContent).toBe('Page 2 of 3');
    });

    it('renders again for every change of kind, form, Page and Lettering', async () => {
        await previewDone();
        const before = vi.mocked(PreviewPage).mock.calls.length;
        radio('pages-dotgrid-spacing', '4').click();
        await vi.waitFor(() => expect(vi.mocked(PreviewPage).mock.calls.length).toBe(before + 1));
        radio('pages-paper', 'b5').click();
        await vi.waitFor(() => expect(vi.mocked(PreviewPage).mock.calls.length).toBe(before + 2));
        setTitle('Ideas');
        await vi.waitFor(() => expect(vi.mocked(PreviewPage).mock.calls.length).toBe(before + 3));
        pickFont('pacifico');
        await vi.waitFor(() => expect(vi.mocked(PreviewPage).mock.calls.length).toBe(before + 4));
        setColourIn(true);
        await vi.waitFor(() => expect(vi.mocked(PreviewPage).mock.calls.length).toBe(before + 5));
        radio('pages-kinds', 'cutout').click();
        await vi.waitFor(() => expect(vi.mocked(PreviewPage).mock.calls.length).toBe(before + 6));
        expect(lastPreviewPage()).toEqual(['cutout', expect.objectContaining({paper: 'b5', title: '', font: 'pacifico', colourIn: true}), '{}']);
        radio('pages-dotgrid-spacing', '5').click();
    });

    it('ignores an answer to an older request', async () => {
        fakePdf('/preview?id=2', 2);
        fakePdf('/preview?id=9', 9);
        let answerOld: (info: {url: string; pages: number}) => void = () => {};
        vi.mocked(PreviewPage).mockImplementationOnce(() => new Promise((done) => { answerOld = done; }));
        vi.mocked(PreviewPage).mockResolvedValueOnce({url: '/preview?id=2', pages: 2});
        radio('pages-paper', 'a4').click();
        await vi.waitFor(() => expect(PreviewPage).toHaveBeenCalledTimes(1));
        radio('pages-paper', 'b5').click();
        await vi.waitFor(() => expect(el('pages-count').textContent).toBe('Page 1 of 2'));
        answerOld({url: '/preview?id=9', pages: 9});
        await new Promise((done) => setTimeout(done, 20));
        expect(el('pages-count').textContent).toBe('Page 1 of 2');
    });

    it('shows the error instead of a page when PreviewPage fails', async () => {
        vi.mocked(PreviewPage).mockRejectedValueOnce(new Error('boom'));
        radio('pages-paper', 'a4').click();
        await vi.waitFor(() => expect(el('pages-preview-message').textContent).toBe('Could not show the preview: boom'));
        expect(el('pages-preview-message').hidden).toBe(false);
        expect(el('pages-count').textContent).toBe('');
        expect(el('pages-sheet').querySelector('canvas')).toBeNull();
        radio('pages-paper', 'b5').click();
        await vi.waitFor(() => expect(el('pages-preview-message').hidden).toBe(true));
    });
});

describe('export', () => {
    it('saves the page with ExportPage and offers to open it', async () => {
        vi.mocked(ExportPage).mockResolvedValueOnce('/out/dotgrid_A5.pdf');
        radio('pages-dotgrid-spacing', '4').click();
        setTitle('Notes');
        // The page count comes from the preview of these options.
        await previewDone();
        el('pages-export').click();
        await vi.waitFor(() => expect(el('pages-status').textContent).toBe('Saved dotgrid_A5.pdf: 1 page'));
        const [kind, dir, settings, options] = vi.mocked(ExportPage).mock.calls[0];
        expect(kind).toBe('dotgrid');
        expect(dir).toBe('/covers');
        expect({...settings}).toEqual({paper: 'a5', cut: '', title: 'Notes', font: 'go', colourIn: false});
        expect(options).toBe('{"spacing":4}');
        expect(el('pages-open-pdf').hidden).toBe(false);
        expect(el<HTMLButtonElement>('pages-export').disabled).toBe(false);

        el('pages-open-pdf').click();
        expect(OpenFile).toHaveBeenCalledWith('/out/dotgrid_A5.pdf');
        radio('pages-dotgrid-spacing', '5').click();
    });

    it('keeps the last result when the dialog is cancelled', async () => {
        vi.mocked(ExportPage).mockResolvedValueOnce('/out/first.pdf');
        await previewDone();
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
