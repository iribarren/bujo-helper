import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';

import {beforeAll, describe, expect, it, vi} from 'vitest';

import {main} from '../wailsjs/go/models';
import {activeCrop, initPrint, perPageOf, sizePerPageOf, usesCutOf, usesSizeOf, type SelectionShape} from './print';

// The print formats as PrintFormats sends them (see internal/sheet.Formats),
// with every field set; for the option layout tests.
const {formatList, cropList} = vi.hoisted(() => {
    const format = (id: string, flags: {usesSize?: boolean; usesCut?: boolean; usesCrop?: boolean; usesHeader?: boolean}) => ({
        id,
        label: id,
        noun: 'cover',
        usesSize: false,
        usesCut: false,
        usesCrop: false,
        usesHeader: false,
        default: id === 'covers',
        ...flags,
    });
    return {
        formatList: [
            format('covers', {usesSize: true, usesCut: true, usesCrop: true}),
            format('cards', {}),
            format('spines', {usesSize: true}),
            format('stubs', {}),
            format('fullpage', {usesHeader: true}),
            format('checklist', {usesHeader: true}),
            format('polaroid', {usesSize: true}),
        ],
        cropList: [{id: 'none', label: 'None', usesSize: true, usesCut: true, frames: 1, default: true}],
    };
});

vi.mock('../wailsjs/go/main/App', async (importOriginal) => {
    const original = await importOriginal<Record<string, unknown>>();
    const stubs: Record<string, unknown> = {};
    for (const name of Object.keys(original)) {
        stubs[name] = vi.fn(async () => undefined);
    }
    for (const name of ['PrintPapers', 'PrintSizes', 'PrintCutModes']) {
        stubs[name] = vi.fn(async () => []);
    }
    stubs.PrintFormats = vi.fn(async () => formatList.map((f) => main.PrintFormatInfo.createFrom(f)));
    stubs.PrintCropModes = vi.fn(async () => cropList.map((c) => main.PrintCropModeInfo.createFrom(c)));
    return stubs;
});

// A4 counts as PrintPapers sends them (see app_test.go).
const a4 = main.PrintPaperInfo.createFrom({
    id: 'a4',
    perPage: {covers: {mini: 81, standard: 25, large: 9}, spines: {mini: 144, standard: 80, large: 48}, cards: {'': 8}},
    headerPerPage: {},
    squarePerPage: {covers: {mini: 54, standard: 15, large: 6}},
    cropPerPage: {instax: {'': 9}, square: {mini: 54, standard: 15, large: 6}, strip3: {'': 9}, strip4: {'': 12}, wallet: {'': 6}},
});
const covers = main.PrintFormatInfo.createFrom({id: 'covers', usesSize: true, usesCut: true, usesCrop: true});
const spines = main.PrintFormatInfo.createFrom({id: 'spines', usesSize: true, usesCut: false, usesCrop: false});
const cards = main.PrintFormatInfo.createFrom({id: 'cards', usesSize: false, usesCut: false, usesCrop: false});

describe('sizePerPageOf', () => {
    const paintings: SelectionShape = {allSquare: false, widePerPage: {mini: 36, standard: 10, large: 3}};
    const cases: Array<[name: string, format: main.PrintFormatInfo, size: string, shape: SelectionShape | undefined, want: number | undefined]> = [
        ['2:3 count before PrintPages answers', covers, 'standard', undefined, 25],
        ['2:3 count for mixed shapes', covers, 'standard', {allSquare: false, widePerPage: null}, 25],
        ['square count when every image is square', covers, 'standard', {allSquare: true}, 15],
        ['wide count when every image gets a wide cell', covers, 'standard', paintings, 10],
        ['wide count at another size', covers, 'mini', paintings, 36],
        ['2:3 count at a size with no wide count', covers, 'large', {allSquare: false, widePerPage: {mini: 30}}, 9],
        ['spines ignore the shape', spines, 'standard', {allSquare: true}, 80],
        ['no count for a format without sizes', cards, '', paintings, undefined],
    ];
    for (const [name, format, size, shape, want] of cases) {
        it(name, () => {
            expect(sizePerPageOf(a4, format, size, shape)).toBe(want);
        });
    }

    it('is undefined until the papers load', () => {
        expect(sizePerPageOf(undefined, covers, 'standard', paintings)).toBeUndefined();
    });
});

// Crop modes as PrintCropModes sends them (see app_test.go).
const crop = (id: string, usesSize: boolean, usesCut: boolean, frames = 1) =>
    main.PrintCropModeInfo.createFrom({id, usesSize, usesCut, frames, default: id === 'none'});
const none = crop('none', true, true);
const instax = crop('instax', false, false);
const square = crop('square', true, true);
const strip4 = crop('strip4', false, false, 4);
const wallet = crop('wallet', false, true);

describe('activeCrop', () => {
    it('is the chosen mode on a cover sheet', () => {
        expect(activeCrop(covers, instax)).toBe(instax);
    });
    it('is undefined for None, a format without cropping, or before they load', () => {
        expect(activeCrop(covers, none)).toBeUndefined();
        expect(activeCrop(spines, instax)).toBeUndefined();
        expect(activeCrop(undefined, instax)).toBeUndefined();
        expect(activeCrop(covers, undefined)).toBeUndefined();
    });
});

describe('usesSizeOf and usesCutOf', () => {
    const cases: Array<[name: string, format: main.PrintFormatInfo | undefined, crop: main.PrintCropModeInfo | undefined, size: boolean, cut: boolean]> = [
        ['cover sheet, no crop', covers, none, true, true],
        ['instant mini', covers, instax, false, false],
        ['square', covers, square, true, true],
        ['photo strip', covers, strip4, false, false],
        ['wallet', covers, wallet, false, true],
        ['spines ignore the crop', spines, instax, true, false],
        ['cards', cards, wallet, false, false],
        ['before the formats load', undefined, undefined, true, true],
    ];
    for (const [name, format, c, size, cut] of cases) {
        it(name, () => {
            expect(usesSizeOf(format, c)).toBe(size);
            expect(usesCutOf(format, c)).toBe(cut);
        });
    }
});

describe('perPageOf with a crop mode', () => {
    const cases: Array<[name: string, format: main.PrintFormatInfo, c: main.PrintCropModeInfo | undefined, size: string, want: number | undefined]> = [
        ['no crop keeps the cover count', covers, none, 'standard', 25],
        ['instant minis', covers, instax, '', 9],
        ['squares follow the size', covers, square, 'large', 6],
        ['photos in strips', covers, strip4, '', 12],
        ['wallets', covers, wallet, '', 6],
        ['spines ignore the crop', spines, instax, 'standard', 80],
    ];
    for (const [name, format, c, size, want] of cases) {
        it(name, () => {
            expect(perPageOf(a4, format, size, false, c)).toBe(want);
        });
    }
});

describe('sizePerPageOf with a crop mode', () => {
    const paintings: SelectionShape = {allSquare: false, widePerPage: {mini: 36, standard: 10, large: 3}};
    it('labels square sizes with the square count, whatever the shape', () => {
        expect(sizePerPageOf(a4, covers, 'mini', paintings, square)).toBe(54);
        expect(sizePerPageOf(a4, covers, 'standard', {allSquare: true}, square)).toBe(15);
    });
    it('has no count for a crop mode without sizes', () => {
        expect(sizePerPageOf(a4, covers, 'standard', paintings, instax)).toBeUndefined();
    });
    it('keeps the shaped counts with no crop', () => {
        expect(sizePerPageOf(a4, covers, 'standard', paintings, none)).toBe(10);
    });
});

describe('option groups', () => {
    function el<T extends HTMLElement>(id: string): T {
        const node = document.getElementById(id);
        if (!node) throw new Error(`missing #${id}`);
        return node as T;
    }

    beforeAll(async () => {
        const html = readFileSync(resolve(__dirname, '../index.html'), 'utf8');
        document.body.innerHTML = html.slice(html.indexOf('<body>') + '<body>'.length, html.indexOf('</body>'));
        initPrint({
            outputFolder: () => '/covers',
            showError: () => {},
            hideError: () => {},
            mediaTypes: () => [],
            mediaType: () => 'books',
        });
        await vi.waitFor(() => expect(el<HTMLSelectElement>('print-format').options.length).toBe(formatList.length));
    });

    function chooseFormat(id: string) {
        const select = el<HTMLSelectElement>('print-format');
        select.value = id;
        select.dispatchEvent(new Event('change'));
    }

    it('group the options in fieldsets with legends', () => {
        const legends = Array.from(el('print-options').querySelectorAll('fieldset > legend'), (l) => l.textContent);
        expect(legends).toEqual(['Covers', 'Layout', 'Page', 'Lettering']);
        const groupOf = (id: string) => el(id).closest('fieldset')?.id;
        expect(groupOf('print-dir')).toBe('print-group-covers');
        expect(groupOf('print-format')).toBe('print-group-layout');
        expect(groupOf('print-size')).toBe('print-group-layout');
        expect(groupOf('print-crop')).toBe('print-group-layout');
        expect(groupOf('print-media')).toBe('print-group-layout');
        expect(groupOf('print-paper')).toBe('print-group-page');
        expect(groupOf('print-cut')).toBe('print-group-page');
        expect(groupOf('print-credits')).toBe('print-group-page');
        expect(groupOf('print-header')).toBe('print-group-lettering');
    });

    // Lettering shows only for the formats that print a header; the other
    // groups always have a visible field.
    const cases: Array<[format: string, lettering: boolean]> = [
        ['covers', false],
        ['cards', false],
        ['spines', false],
        ['stubs', false],
        ['fullpage', true],
        ['checklist', true],
        ['polaroid', false],
    ];
    for (const [format, lettering] of cases) {
        it(`${format}: Lettering ${lettering ? 'shown' : 'hidden'}`, () => {
            chooseFormat(format);
            expect(el('print-group-covers').hidden).toBe(false);
            expect(el('print-group-layout').hidden).toBe(false);
            expect(el('print-group-page').hidden).toBe(false);
            expect(el('print-group-lettering').hidden).toBe(!lettering);
            expect(el('print-header-field').hidden).toBe(!lettering);
        });
    }

    it('hide a group when none of its fields is shown', () => {
        // Paper and Credits page are always shown; hidden here, the Page
        // group follows the cut mode, which review cards do not use.
        const paper = el('print-paper').closest<HTMLElement>('.field')!;
        const credits = el('print-credits').closest<HTMLElement>('.field')!;
        paper.hidden = true;
        credits.hidden = true;
        try {
            chooseFormat('cards');
            expect(el('print-group-page').hidden).toBe(true);
            chooseFormat('covers');
            expect(el('print-group-page').hidden).toBe(false);
        } finally {
            paper.hidden = false;
            credits.hidden = false;
        }
    });
});
