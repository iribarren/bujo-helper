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

function selectType(id: string) {
    const select = el<HTMLSelectElement>('media-type');
    select.value = id;
    select.dispatchEvent(new Event('change'));
}

beforeAll(async () => {
    const html = readFileSync(resolve(__dirname, '../index.html'), 'utf8');
    document.body.innerHTML = html.slice(html.indexOf('<body>') + '<body>'.length, html.indexOf('</body>'));
    await import('./main');
    await vi.waitFor(() => expect(el<HTMLSelectElement>('media-type').options.length).toBe(mediaTypes.length));
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
});
