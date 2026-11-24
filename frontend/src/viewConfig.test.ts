import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';

import {PrintSizes} from '../wailsjs/go/main/App';
import {setLang} from './i18n';
import {initViewConfig, type ViewConfig} from './viewConfig';
import type {View} from './view';

// The lists as Go sends them: English labels, a default per list (Make
// pages' default is not its first item).
const {lists} = vi.hoisted(() => ({
    lists: {
        media: [
            {id: 'books', label: 'Books', default: true},
            {id: 'movies', label: 'Movies'},
            {id: 'games', label: 'Games'},
        ],
        paper: [
            {id: 'a4', label: 'A4', default: true},
            {id: 'letter', label: 'US Letter'},
            {id: 'a5', label: 'A5'},
        ],
        format: [
            {id: 'covers', label: 'Cover sheet', default: true},
            {id: 'cards', label: 'Review cards'},
        ],
        size: [
            {id: 'standard', label: 'Standard', default: true},
            {id: 'mini', label: 'Mini'},
            {id: 'large', label: 'Large'},
        ],
        crop: [
            {id: 'wallet', label: 'Wallet', default: true},
            {id: 'strip3', label: 'Photo strip (3 frames)'},
        ],
        cut: [
            {id: 'border', label: 'Border', default: true},
            {id: 'bleed', label: 'Bleed + crop marks'},
            {id: 'none', label: 'None'},
        ],
        pageKind: [
            {id: 'dotgrid', label: 'Dot grid'},
            {id: 'moon', label: 'Moon calendar', default: true},
            {id: 'calendar', label: 'Month calendar'},
        ],
    },
}));

vi.mock('../wailsjs/go/main/App', () => ({
    MediaTypes: vi.fn(async () => lists.media),
    PrintPapers: vi.fn(async () => lists.paper),
    PrintFormats: vi.fn(async () => lists.format),
    PrintSizes: vi.fn(async () => lists.size),
    PrintCropModes: vi.fn(async () => lists.crop),
    PrintCutModes: vi.fn(async () => lists.cut),
    PageKinds: vi.fn(async () => lists.pageKind),
}));

let container: HTMLElement;
let config: ViewConfig;
let changed: ReturnType<typeof vi.fn>;

function section(id: string): HTMLElement {
    const node = container.querySelector<HTMLElement>(`[data-section="${id}"]`);
    if (!node) throw new Error(`missing section ${id}`);
    return node;
}

function sectionBox(id: string): HTMLInputElement {
    const node = section(id).querySelector<HTMLInputElement>(':scope > legend input');
    if (!node) throw new Error(`missing section checkbox ${id}`);
    return node;
}

function list(id: string): HTMLElement {
    const node = container.querySelector<HTMLElement>(`[data-list="${id}"]`);
    if (!node) throw new Error(`missing list ${id}`);
    return node;
}

// master is the all/none checkbox of the list.
function master(id: string): HTMLInputElement {
    const node = list(id).querySelector<HTMLInputElement>(':scope > legend input');
    if (!node) throw new Error(`missing master of ${id}`);
    return node;
}

function box(listId: string, item: string): HTMLInputElement {
    const node = list(listId).querySelector<HTMLInputElement>(`input[data-item="${item}"]`);
    if (!node) throw new Error(`missing ${listId}/${item}`);
    return node;
}

function items(listId: string): HTMLInputElement[] {
    return Array.from(list(listId).querySelectorAll<HTMLInputElement>('input[data-item]'));
}

function ticked(listId: string): string[] {
    return items(listId).filter((input) => input.checked).map((input) => input.dataset.item ?? '');
}

function text(node: Element | null | undefined): string {
    return node?.textContent?.replace(/\s+/g, ' ').trim() ?? '';
}

async function build(view?: View): Promise<void> {
    container = document.createElement('div');
    document.body.replaceChildren(container);
    changed = vi.fn();
    config = initViewConfig(container, changed);
    await config.load();
    config.fill(view);
}

beforeEach(async () => {
    await build();
});

afterEach(() => {
    setLang('en');
});

describe('the sections tree', () => {
    it('has one block per section in order, with its lists and their items', () => {
        const sections = Array.from(container.querySelectorAll<HTMLElement>('[data-section]'));
        expect(sections.map((s) => s.dataset.section)).toEqual(['download', 'print', 'pages']);
        expect(sections.map((s) => text(s.querySelector('legend label')))).toEqual(['Download', 'Print covers', 'Make pages']);
        const listsOf = (id: string) => Array.from(section(id).querySelectorAll<HTMLElement>('[data-list]'), (l) => l.dataset.list);
        expect(listsOf('download')).toEqual(['media']);
        expect(listsOf('print')).toEqual(['paper', 'format', 'size', 'crop', 'cut']);
        expect(listsOf('pages')).toEqual(['pageKind']);
        expect(text(master('paper').closest('label'))).toBe('Paper');
        expect(items('paper').map((input) => text(input.closest('label')))).toEqual(['A4', 'US Letter', 'A5']);
        for (const input of container.querySelectorAll('input')) expect(input.type).toBe('checkbox');
    });

    it('translates the titles and item labels on a language change', () => {
        setLang('es');
        expect(text(section('download').querySelector('legend label'))).toBe('Descargar');
        expect(text(master('media').closest('label'))).toBe('Tipos de medio');
        expect(text(box('media', 'books').closest('label'))).toBe('Libros');
        expect(text(box('paper', 'letter').closest('label'))).toBe('Carta (EE. UU.)');
        expect(text(section('print').querySelector('.view-note'))).toBe('Siempre visible');
    });

    it('ticks everything when no view is saved', () => {
        for (const input of container.querySelectorAll<HTMLInputElement>('input')) expect(input.checked).toBe(true);
        for (const id of ['media', 'paper', 'pageKind']) expect(master(id).indeterminate).toBe(false);
        const read = config.read();
        expect(read.hiddenSections).toBeUndefined();
        expect(read.hidden).toBeUndefined();
    });

    it('shows a saved view, and the saved one again at every fill', () => {
        config.fill({hiddenSections: ['pages'], hidden: {paper: ['letter'], pageKind: ['dotgrid']}});
        expect(ticked('paper')).toEqual(['a4', 'a5']);
        expect(sectionBox('pages').checked).toBe(false);
        expect(ticked('pageKind')).toEqual(['moon', 'calendar']);
        box('paper', 'a5').click();
        config.fill(undefined);
        expect(ticked('paper')).toEqual(['a4', 'letter', 'a5']);
        expect(sectionBox('pages').checked).toBe(true);
    });

    it('shows the default ticked when a saved view hides every item of a list', () => {
        config.fill({hidden: {pageKind: ['dotgrid', 'moon', 'calendar']}});
        expect(ticked('pageKind')).toEqual(['moon']);
        expect(box('pageKind', 'moon').disabled).toBe(true);
    });

    it('fills the ticks of lists that load after the fill', async () => {
        container = document.createElement('div');
        document.body.replaceChildren(container);
        config = initViewConfig(container, () => {});
        config.fill({hidden: {paper: ['a5']}});
        await config.load();
        expect(ticked('paper')).toEqual(['a4', 'letter']);
    });
});

describe('all or none of a list', () => {
    it('leaves only the default when every item is ticked, then ticks them all', () => {
        expect(master('pageKind').checked).toBe(true);
        master('pageKind').click();
        expect(ticked('pageKind')).toEqual(['moon']);
        expect(master('pageKind').checked).toBe(false);
        expect(master('pageKind').indeterminate).toBe(true);
        master('pageKind').click();
        expect(ticked('pageKind')).toEqual(['dotgrid', 'moon', 'calendar']);
        expect(master('pageKind').checked).toBe(true);
        expect(master('pageKind').indeterminate).toBe(false);
        expect(changed).toHaveBeenCalled();
    });

    it('is mixed when some items are ticked, and ticks them all', () => {
        box('paper', 'letter').click();
        expect(master('paper').checked).toBe(false);
        expect(master('paper').indeterminate).toBe(true);
        master('paper').click();
        expect(ticked('paper')).toEqual(['a4', 'letter', 'a5']);
        expect(master('paper').indeterminate).toBe(false);
    });
});

describe('the required minimum', () => {
    it('keeps the last ticked item of a list from being unticked', () => {
        box('paper', 'a4').click();
        box('paper', 'a5').click();
        expect(ticked('paper')).toEqual(['letter']);
        expect(box('paper', 'letter').disabled).toBe(true);
        box('paper', 'letter').click();
        expect(ticked('paper')).toEqual(['letter']);
        box('paper', 'a4').click();
        expect(box('paper', 'letter').disabled).toBe(false);
        expect(box('paper', 'a4').disabled).toBe(false);
        expect(changed).toHaveBeenCalled();
    });
});

describe('optional sections', () => {
    it('greys out the lists of a hidden section, keeping their choices', () => {
        box('pageKind', 'dotgrid').click();
        sectionBox('pages').click();
        expect(section('pages').classList.contains('off')).toBe(true);
        expect(master('pageKind').disabled).toBe(true);
        for (const input of items('pageKind')) expect(input.disabled).toBe(true);
        expect(ticked('pageKind')).toEqual(['moon', 'calendar']);
        expect(config.read()).toMatchObject({hiddenSections: ['pages'], hidden: {pageKind: ['dotgrid']}});

        sectionBox('pages').click();
        expect(section('pages').classList.contains('off')).toBe(false);
        expect(box('pageKind', 'moon').disabled).toBe(false);
        expect(config.read().hiddenSections).toBeUndefined();
    });

    it('always shows Print', () => {
        expect(sectionBox('print').checked).toBe(true);
        expect(sectionBox('print').disabled).toBe(true);
        expect(text(section('print').querySelector('.view-note'))).toBe('Always shown');
        expect(section('download').querySelector('.view-note')).toBeNull();
        sectionBox('download').click();
        config.fill({hiddenSections: ['print', 'download']});
        expect(sectionBox('print').checked).toBe(true);
        expect(config.read().hiddenSections).toEqual(['download']);
    });
});

describe('reading the view', () => {
    it('saves the unticked IDs per list, dropping IDs no longer listed', () => {
        config.fill({hidden: {paper: ['b5', 'letter'], cut: ['gone']}});
        box('media', 'games').click();
        const read = config.read();
        expect(read.hidden).toEqual({paper: ['letter'], media: ['games']});
        expect(read.hiddenSections).toBeUndefined();
    });

    it('keeps the saved IDs of a list that could not load, and loads it at the next try', async () => {
        vi.mocked(PrintSizes).mockRejectedValueOnce(new Error('boom'));
        await build({hidden: {size: ['mini', 'gone'], paper: ['letter']}});
        expect(items('size')).toEqual([]);
        expect(text(list('size').querySelector('.view-note'))).toBe('Could not load this list.');
        expect(master('size').disabled).toBe(true);
        expect(config.read().hidden).toEqual({paper: ['letter'], size: ['mini', 'gone']});

        await config.load();
        expect(ticked('size')).toEqual(['standard', 'large']);
        expect(list('size').querySelector('.view-note')).toBeNull();
        expect(config.read().hidden).toEqual({paper: ['letter'], size: ['mini']});
    });
});
