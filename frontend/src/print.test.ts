import {describe, expect, it} from 'vitest';

import {main} from '../wailsjs/go/models';
import {activeCrop, perPageOf, sizePerPageOf, usesCutOf, usesSizeOf, type SelectionShape} from './print';

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
