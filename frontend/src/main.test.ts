import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';

import {beforeAll, describe, expect, it, vi} from 'vitest';

// main.ts wires the whole page when imported, so these tests load the real
// markup, stub the Go bindings and the Wails runtime, and drive the page
// through the media type selector.

const {mediaTypes} = vi.hoisted(() => ({
    mediaTypes: [
        {id: 'books', label: 'Books', creatorLabel: 'Author'},
        {id: 'nature', label: 'Nature', creatorLabel: ''},
        {id: 'anime', label: 'Anime', creatorLabel: 'Year'},
        {id: 'manga', label: 'Manga', creatorLabel: 'Year'},
        // A label Go never sends (the json tag has no omitempty), or one
        // that is only blanks, still means "no creator column".
        {id: 'unlabelled', label: 'Unlabelled'},
        {id: 'blank', label: 'Blank', creatorLabel: '  '},
    ],
}));

vi.mock('../wailsjs/go/main/App', async (importOriginal) => {
    const original = await importOriginal<Record<string, unknown>>();
    const stubs: Record<string, unknown> = {};
    for (const name of Object.keys(original)) {
        stubs[name] = vi.fn(async () => undefined);
    }
    stubs.MediaTypes = vi.fn(async () => mediaTypes);
    stubs.GetSettings = vi.fn(async () => ({language: 'en'}));
    stubs.DefaultOutputDir = vi.fn(async () => '/covers');
    for (const name of ['PrintPapers', 'PrintFormats', 'PrintSizes', 'PrintCutModes']) {
        stubs[name] = vi.fn(async () => []);
    }
    return stubs;
});

vi.mock('../wailsjs/runtime/runtime', () => ({
    EventsOn: vi.fn(() => () => undefined),
    BrowserOpenURL: vi.fn(),
}));

function el<T extends HTMLElement>(id: string): T {
    const node = document.getElementById(id);
    if (!node) throw new Error(`missing #${id}`);
    return node as T;
}

// mediaRadio is the media type radio button for id.
function mediaRadio(id: string): HTMLButtonElement {
    const node = el('media-type').querySelector<HTMLButtonElement>(`[role="radio"][data-value="${id}"]`);
    if (!node) throw new Error(`missing media type radio ${id}`);
    return node;
}

function selectType(id: string) {
    mediaRadio(id).click();
}

beforeAll(async () => {
    const html = readFileSync(resolve(__dirname, '../index.html'), 'utf8');
    document.body.innerHTML = html.slice(html.indexOf('<body>') + '<body>'.length, html.indexOf('</body>'));
    await import('./main');
    await vi.waitFor(() => expect(el('media-type').querySelectorAll('[role="radio"]').length).toBe(mediaTypes.length));
});

describe('items placeholder', () => {
    const cases: Array<[name: string, type: string, want: string]> = [
        ['a creator column asks for "Title, Author"', 'books', 'Title, Author'],
        ['an empty creator label asks for a name alone', 'nature', 'Name\nOne per line'],
        ['a missing creator label asks for a name alone', 'unlabelled', 'Name\nOne per line'],
        ['a blank creator label asks for a name alone', 'blank', 'Name\nOne per line'],
    ];
    for (const [name, type, want] of cases) {
        it(name, () => {
            selectType(type);
            expect(el<HTMLTextAreaElement>('items').placeholder.slice(0, want.length)).toBe(want);
        });
    }

    const examples: Array<[type: string, example: string]> = [
        ['anime', 'Frieren, 2023'],
        ['manga', 'Berserk, 1989'],
    ];
    for (const [type, example] of examples) {
        it(`gives a "Title, Year" example for ${type}`, () => {
            selectType(type);
            const placeholder = el<HTMLTextAreaElement>('items').placeholder;
            expect(placeholder.startsWith('Title, Year')).toBe(true);
            expect(placeholder).toContain(example);
        });
    }
});

describe('tabs', () => {
    it('are named Download and Print covers', () => {
        expect(el('tab-download').textContent).toBe('Download');
        expect(el('tab-print').textContent).toBe('Print covers');
    });

    it('switch the panel and the app section on click', () => {
        el('tab-print').click();
        expect(el('app').dataset.tab).toBe('print');
        expect(el('panel-print').hidden).toBe(false);
        expect(el('panel-download').hidden).toBe(true);
        expect(el('tab-print').getAttribute('aria-selected')).toBe('true');
        expect(el('tab-download').getAttribute('aria-selected')).toBe('false');

        el('tab-download').click();
        expect(el('app').dataset.tab).toBe('download');
        expect(el('panel-download').hidden).toBe(false);
        expect(el('panel-print').hidden).toBe(true);
    });

    it('wrap with the arrow keys', () => {
        const key = (id: string, name: string) =>
            el(id).dispatchEvent(new KeyboardEvent('keydown', {key: name, bubbles: true, cancelable: true}));
        key('tab-download', 'ArrowLeft');
        expect(el('app').dataset.tab).toBe('print');
        key('tab-print', 'ArrowRight');
        expect(el('app').dataset.tab).toBe('download');
        expect(document.activeElement).toBe(el('tab-download'));
    });
});

describe('key tip', () => {
    it('suggests a BHL key for nature while none is saved', () => {
        selectType('nature');
        expect(el('key-tip').hidden).toBe(false);
        expect(el('key-tip-what').textContent).toBe('a BHL API key');
        expect(el('key-tip-why').textContent).toBe('for more natural-history plates.');
    });

    it('is hidden for a type whose sources need no key', () => {
        selectType('books');
        expect(el('key-tip').hidden).toBe(true);
    });
});

describe('media type', () => {
    it('is a radio group in the Download panel', () => {
        const group = el('media-type');
        expect(group.getAttribute('role')).toBe('radiogroup');
        expect(el('panel-download').contains(group)).toBe(true);
    });

    it('checks the clicked type and updates the placeholder', () => {
        selectType('nature');
        expect(mediaRadio('nature').getAttribute('aria-checked')).toBe('true');
        expect(mediaRadio('books').getAttribute('aria-checked')).toBe('false');
        expect(el<HTMLTextAreaElement>('items').placeholder.startsWith('Name')).toBe(true);
    });

    it('moves the selection with the arrow keys', () => {
        selectType('books');
        mediaRadio('books').dispatchEvent(new KeyboardEvent('keydown', {key: 'ArrowRight', bubbles: true, cancelable: true}));
        expect(mediaRadio('nature').getAttribute('aria-checked')).toBe('true');
        expect(el('key-tip').hidden).toBe(false);
        mediaRadio('nature').dispatchEvent(new KeyboardEvent('keydown', {key: 'ArrowLeft', bubbles: true, cancelable: true}));
        expect(mediaRadio('books').getAttribute('aria-checked')).toBe('true');
        expect(el('key-tip').hidden).toBe(true);
    });

    it('sets what the print cards are for', () => {
        selectType('anime');
        el('tab-print').click();
        expect(el<HTMLSelectElement>('print-media').value).toBe('anime');
        el('tab-download').click();
        selectType('manga');
        el('tab-print').click();
        expect(el<HTMLSelectElement>('print-media').value).toBe('manga');
        el('tab-download').click();
    });
});
