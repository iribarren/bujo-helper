import {describe, expect, it} from 'vitest';

import {main} from '../wailsjs/go/models';
import {sizePerPageOf, type SelectionShape} from './print';

// A4 counts as PrintPapers sends them (see app_test.go).
const a4 = main.PrintPaperInfo.createFrom({
    id: 'a4',
    perPage: {covers: {mini: 81, standard: 25, large: 9}, spines: {mini: 144, standard: 80, large: 48}, cards: {'': 8}},
    headerPerPage: {},
    squarePerPage: {covers: {mini: 54, standard: 15, large: 6}},
});
const covers = main.PrintFormatInfo.createFrom({id: 'covers', usesSize: true});
const spines = main.PrintFormatInfo.createFrom({id: 'spines', usesSize: true});
const cards = main.PrintFormatInfo.createFrom({id: 'cards', usesSize: false});

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
