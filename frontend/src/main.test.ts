import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';

import {afterEach, beforeAll, describe, expect, it, vi} from 'vitest';

import {ChooseFolder, ExportPage, GetSettings, ListCovers, Start} from '../wailsjs/go/main/App';
import {BrowserOpenURL, EventsOn} from '../wailsjs/runtime/runtime';
import {useLanguage} from './language';

// main.ts wires the whole page when imported, so these tests load the real
// markup, stub the Go bindings and the Wails runtime, and drive the page
// through the media type selector.

// builtInBGGToken settles what HasBuiltInBGGToken answers; until a test
// calls it, the answer is pending, as if Go had not replied yet.
const {mediaTypes, builtInBGGToken} = vi.hoisted(() => {
    let settle: (has: boolean) => void = () => undefined;
    const answer = new Promise<boolean>((resolve) => {
        settle = resolve;
    });
    return {
        builtInBGGToken: {answer, settle: (has: boolean) => settle(has)},
        mediaTypes: [
        {id: 'books', label: 'Books', creatorLabel: 'Author', sources: ['Apple Books', 'Open Library', 'Amazon']},
        {id: 'nature', label: 'Nature', creatorLabel: '', sources: ['Wikimedia Commons', 'Biodiversity Heritage Library']},
        {id: 'anime', label: 'Anime', creatorLabel: 'Year'},
        {id: 'manga', label: 'Manga', creatorLabel: 'Year'},
        // A label Go never sends (the json tag has no omitempty), or one
        // that is only blanks, still means "no creator column".
        {id: 'unlabelled', label: 'Unlabelled'},
        {id: 'blank', label: 'Blank', creatorLabel: '  '},
        {id: 'boardgames', label: 'Board games', creatorLabel: 'Year', sources: ['BoardGameGeek']},
        {id: 'podcasts', label: 'Podcasts', creatorLabel: 'Host', sources: ['Apple Podcasts']},
        // Both are best served by TMDB, which needs a key.
        {id: 'movies', label: 'Movies', creatorLabel: 'Year', sources: ['TMDB', 'Wikidata']},
        {id: 'tv', label: 'TV series', creatorLabel: 'Year', sources: ['TMDB', 'IMDb']},
    ],
    };
});

vi.mock('../wailsjs/go/main/App', async (importOriginal) => {
    const original = await importOriginal<Record<string, unknown>>();
    const stubs: Record<string, unknown> = {};
    for (const name of Object.keys(original)) {
        stubs[name] = vi.fn(async () => undefined);
    }
    stubs.MediaTypes = vi.fn(async () => mediaTypes);
    // Startup reads the language and the remembered tab options.
    stubs.GetSettings = vi.fn(async () => ({language: 'en', print: {header: 'Saved header'}, pages: {title: 'Saved title'}}));
    stubs.DefaultOutputDir = vi.fn(async () => '/covers');
    stubs.HasBuiltInBGGToken = vi.fn(() => builtInBGGToken.answer);
    for (const name of ['PrintPapers', 'PrintFormats', 'PrintSizes', 'PrintCutModes']) {
        stubs[name] = vi.fn(async () => []);
    }
    // One page kind on one paper, so Make pages can export.
    stubs.PageKinds = vi.fn(async () => [
        {id: 'dotgrid', label: 'Dot grid', usesCut: false, usesTitle: true, defaults: '{"spacing":5}', default: true},
    ]);
    stubs.PrintPapers = vi.fn(async () => [{id: 'a5', label: 'A5', width: 148, height: 210, default: true}]);
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

const handlers = new Map<string, (payload: unknown) => void>();

// handler is the Wails event handler main.ts registered for name.
function handler(name: string): (payload: unknown) => void {
    const found = handlers.get(name);
    if (!found) throw new Error(`no ${name} handler`);
    return found;
}

// finishJob runs a download into outputDir to its end.
async function finishJob(outputDir: string) {
    el<HTMLInputElement>('output-dir').value = outputDir;
    el<HTMLTextAreaElement>('items').value = 'Dune, Frank Herbert';
    vi.mocked(Start).mockResolvedValueOnce([]);
    el('start').click();
    await vi.waitFor(() => expect(el('cancel').hidden).toBe(false));
    handler('job:done')({ok: 1, failed: 0, cancelled: 0});
    await vi.waitFor(() => expect(el('start').hidden).toBe(false));
}

// resetFolders undoes a test's folder changes, which main.ts (imported once)
// would keep: a job into no folder clears the job folder, Print these
// covers then unpins the Print covers folder (it follows the output folder
// again), and the output folder and the mocks are as they started.
async function resetFolders() {
    await finishJob('');
    vi.mocked(ListCovers).mockResolvedValueOnce([]);
    el('print-these').click();
    el('tab-download').click();
    el<HTMLTextAreaElement>('items').value = '';
    el<HTMLInputElement>('output-dir').value = '/covers';
    for (const mock of [Start, ListCovers, ChooseFolder, ExportPage]) vi.mocked(mock).mockReset();
}

beforeAll(async () => {
    // jsdom has no modal dialogs.
    HTMLDialogElement.prototype.showModal ??= function (this: HTMLDialogElement) {
        this.open = true;
    };
    HTMLDialogElement.prototype.close ??= function (this: HTMLDialogElement) {
        this.open = false;
    };
    const html = readFileSync(resolve(__dirname, '../index.html'), 'utf8');
    document.body.innerHTML = html.slice(html.indexOf('<body>') + '<body>'.length, html.indexOf('</body>'));
    await import('./main');
    // The Wails event handlers main.ts registered; mock calls are cleared
    // before each test.
    for (const [event, handler] of vi.mocked(EventsOn).mock.calls) {
        handlers.set(event, handler as (payload: unknown) => void);
    }
    await vi.waitFor(() => expect(el('media-type').querySelectorAll('[role="radio"]').length).toBe(mediaTypes.length));
});

describe('items placeholder', () => {
    const cases: Array<[name: string, type: string, want: string]> = [
        ['a creator column asks for "Title, Author"', 'books', 'Title, Author'],
        ['a host column asks for "Title, Host"', 'podcasts', 'Title, Host'],
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
        ['boardgames', 'Catan, 1995'],
    ];
    for (const [type, example] of examples) {
        it(`gives a "Title, Year" example for ${type}`, () => {
            selectType(type);
            const placeholder = el<HTMLTextAreaElement>('items').placeholder;
            expect(placeholder.startsWith('Title, Year')).toBe(true);
            expect(placeholder).toContain(example);
        });
    }

    it('gives a show and host example for podcasts', () => {
        selectType('podcasts');
        expect(el<HTMLTextAreaElement>('items').placeholder).toContain('The Daily, The New York Times');
    });
});

describe('tabs', () => {
    const key = (id: string, name: string) =>
        el(id).dispatchEvent(new KeyboardEvent('keydown', {key: name, bubbles: true, cancelable: true}));

    afterEach(() => {
        el('tab-download').click();
    });

    it('are named Download, Print covers and Make pages', () => {
        expect(el('tab-download').textContent).toBe('Download');
        expect(el('tab-print').textContent).toBe('Print covers');
        expect(el('tab-pages').textContent).toBe('Make pages');
        expect(el('tab-pages').getAttribute('aria-controls')).toBe('panel-pages');
    });

    it('switch the panel and the app section on click', () => {
        el('tab-print').click();
        expect(el('app').dataset.tab).toBe('print');
        expect(el('panel-print').hidden).toBe(false);
        expect(el('panel-download').hidden).toBe(true);
        expect(el('panel-pages').hidden).toBe(true);
        expect(el('tab-print').getAttribute('aria-selected')).toBe('true');
        expect(el('tab-download').getAttribute('aria-selected')).toBe('false');

        el('tab-pages').click();
        expect(el('app').dataset.tab).toBe('pages');
        expect(el('panel-pages').hidden).toBe(false);
        expect(el('panel-print').hidden).toBe(true);
        expect(el('panel-download').hidden).toBe(true);
        expect(el('tab-pages').getAttribute('aria-selected')).toBe('true');

        el('tab-download').click();
        expect(el('app').dataset.tab).toBe('download');
        expect(el('panel-download').hidden).toBe(false);
        expect(el('panel-print').hidden).toBe(true);
        expect(el('panel-pages').hidden).toBe(true);
    });

    it('wrap across the three tabs with the arrow keys', () => {
        el('tab-download').click();
        key('tab-download', 'ArrowLeft');
        expect(el('app').dataset.tab).toBe('pages');
        expect(document.activeElement).toBe(el('tab-pages'));
        key('tab-pages', 'ArrowRight');
        expect(el('app').dataset.tab).toBe('download');
        key('tab-download', 'ArrowRight');
        expect(el('app').dataset.tab).toBe('print');
        key('tab-print', 'ArrowRight');
        expect(el('app').dataset.tab).toBe('pages');
        key('tab-pages', 'Home');
        expect(el('app').dataset.tab).toBe('download');
        key('tab-download', 'End');
        expect(el('app').dataset.tab).toBe('pages');
        expect(document.activeElement).toBe(el('tab-pages'));
    });

    it('do not start a download with Ctrl+Enter outside the Download tab', () => {
        el<HTMLTextAreaElement>('items').value = 'Dune, Frank Herbert';
        try {
            for (const tab of ['tab-print', 'tab-pages']) {
                el(tab).click();
                window.dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', ctrlKey: true, bubbles: true, cancelable: true}));
                document.dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', ctrlKey: true, bubbles: true, cancelable: true}));
            }
            expect(Start).not.toHaveBeenCalled();
        } finally {
            el<HTMLTextAreaElement>('items').value = '';
        }
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

    // openSettings opens the Settings dialog with saved settings, which
    // reloads them, and closes it again.
    async function openSettings(saved: Record<string, string>) {
        vi.mocked(GetSettings).mockResolvedValueOnce(saved as never);
        el('settings-open').click();
        await vi.waitFor(() => expect(el<HTMLDialogElement>('settings-dialog').open).toBe(true));
        await vi.waitFor(() => expect(el<HTMLInputElement>('bgg-token').value).toBe(saved.bggToken ?? ''));
        el('settings-close').click();
    }

    it('suggests a TMDB key for movies and TV series until one is saved', async () => {
        for (const type of ['movies', 'tv']) {
            selectType(type);
            expect(el('key-tip').hidden, type).toBe(false);
            expect(el('key-tip-what').textContent, type).toBe('a TMDB API key');
            expect(el('key-tip-why').textContent, type).toBe('for better movie and TV posters.');
        }

        await openSettings({language: 'en', tmdbApiKey: 'saved-key'});
        expect(el('key-tip').hidden).toBe(true);
        selectType('movies');
        expect(el('key-tip').hidden).toBe(true);
        // A key of blanks is no key.
        await openSettings({language: 'en', tmdbApiKey: '  '});
        expect(el('key-tip').hidden).toBe(false);
        selectType('tv');
        expect(el('key-tip').hidden).toBe(false);
        selectType('books');
        expect(el('key-tip').hidden).toBe(true);
    });

    // One sequential case: the built-in token answer arrives once, at
    // startup, and cannot be taken back.
    it('asks for a BoardGameGeek token for board games until one is saved or built in', async () => {
        selectType('boardgames');
        expect(el('key-tip').hidden).toBe(false);
        expect(el('key-tip-what').textContent).toBe('a BoardGameGeek token');
        expect(el('key-tip-why').textContent).toBe('to download board game box art; this build has none built in.');

        await openSettings({language: 'en', bggToken: 'saved-token'});
        expect(el('key-tip').hidden).toBe(true);
        await openSettings({language: 'en', bggToken: ''});
        expect(el('key-tip').hidden).toBe(false);

        builtInBGGToken.settle(true);
        await vi.waitFor(() => expect(el('key-tip').hidden).toBe(true));
        selectType('books');
        selectType('boardgames');
        expect(el('key-tip').hidden).toBe(true);
    });
});

describe('remembered options', () => {
    it('reach the Print covers tab at startup', async () => {
        await vi.waitFor(() => expect(el<HTMLInputElement>('print-header').value).toBe('Saved header'));
    });

    it('reach the Make pages tab at startup', async () => {
        await vi.waitFor(() => expect(el<HTMLInputElement>('pages-title').value).toBe('Saved title'));
    });
});

describe('Powered by BGG logo', () => {
    it('shows in the Download tab only while board games are selected', () => {
        selectType('boardgames');
        expect(el('bgg-logo-download').hidden).toBe(false);
        selectType('books');
        expect(el('bgg-logo-download').hidden).toBe(true);
        selectType('nature');
        expect(el('bgg-logo-download').hidden).toBe(true);
    });

    it('links to BoardGameGeek with both theme variants of the official logo', () => {
        const link = el<HTMLAnchorElement>('bgg-logo-download');
        expect(link.href).toBe('https://boardgamegeek.com/');
        expect(link.getAttribute('aria-label')).toBe('Powered by BGG');
        const images = Array.from(link.querySelectorAll('img'));
        expect(images.map((img) => img.className)).toEqual(['bgg-logo-light', 'bgg-logo-dark']);
        expect(images[0].getAttribute('src')).toContain('powered-by-bgg-rgb');
        expect(images[1].getAttribute('src')).toContain('powered-by-bgg-reversed-rgb');
        for (const img of images) expect(img.alt).toBe('Powered by BGG');
    });

    it('opens BoardGameGeek in the system browser', () => {
        const click = new MouseEvent('click', {bubbles: true, cancelable: true});
        el('bgg-logo-download').dispatchEvent(click);
        expect(click.defaultPrevented).toBe(true);
        expect(BrowserOpenURL).toHaveBeenCalledWith('https://boardgamegeek.com/');
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

describe('sources line', () => {
    afterEach(async () => {
        await useLanguage('en');
    });

    const cases: Array<[type: string, want: string]> = [
        ['books', 'Sources tried in order: Apple Books, Open Library, Amazon'],
        ['nature', 'Sources tried in order: Wikimedia Commons, Biodiversity Heritage Library'],
    ];
    for (const [type, want] of cases) {
        it(`lists the ${type} sources under the media types`, () => {
            selectType(type);
            expect(el('sources').hidden).toBe(false);
            expect(el('sources').textContent).toBe(want);
        });
    }

    it('is hidden for a type without sources', () => {
        selectType('anime');
        expect(el('sources').hidden).toBe(true);
    });

    it('follows the language', async () => {
        selectType('books');
        await useLanguage('es');
        expect(el('sources').textContent).toBe('Fuentes consultadas por orden: Apple Books, Open Library, Amazon');
    });
});

describe('print these covers', () => {
    afterEach(resetFolders);

    it('sits in the download footer after Open folder', () => {
        const link = el('print-these');
        expect(link.closest('footer')).toBe(el('open-folder').closest('footer'));
        expect(el('open-folder').compareDocumentPosition(link) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
        expect(link.textContent).toBe('Print these covers');
    });

    it('shows the finished job\'s folder in Print covers', async () => {
        // A folder picked in Print covers earlier.
        vi.mocked(ChooseFolder).mockResolvedValueOnce('/elsewhere');
        el('tab-print').click();
        el('print-browse').click();
        await vi.waitFor(() => expect(el<HTMLInputElement>('print-dir').value).toBe('/elsewhere'));
        el('tab-download').click();

        // A download into /job, whose output folder is changed afterwards.
        await finishJob('/job');
        el<HTMLInputElement>('output-dir').value = '/other';

        vi.mocked(ListCovers).mockClear();
        el('print-these').click();
        expect(el('app').dataset.tab).toBe('print');
        expect(el<HTMLInputElement>('print-dir').value).toBe('/job');
        expect(ListCovers).toHaveBeenCalledWith('/job');
    });

    it('switches to the Print covers tab', () => {
        el('print-these').click();
        expect(el('app').dataset.tab).toBe('print');
        expect(el('panel-print').hidden).toBe(false);
        expect(el('panel-download').hidden).toBe(true);
        expect(el('tab-print').getAttribute('aria-selected')).toBe('true');
    });
});

describe('make pages', () => {
    afterEach(resetFolders);

    // exportDir exports from Make pages, cancelling the save dialog, and
    // returns the folder the dialog starts in.
    async function exportDir(): Promise<string> {
        vi.mocked(ExportPage).mockClear();
        vi.mocked(ExportPage).mockResolvedValueOnce('');
        el('tab-pages').click();
        await vi.waitFor(() => expect(el<HTMLButtonElement>('pages-export').disabled).toBe(false));
        el('pages-export').click();
        await vi.waitFor(() => expect(ExportPage).toHaveBeenCalledTimes(1));
        await vi.waitFor(() => expect(el<HTMLButtonElement>('pages-export').disabled).toBe(false));
        return vi.mocked(ExportPage).mock.calls[0][1];
    }

    it('starts the save dialog in the folder Print covers shows', async () => {
        vi.mocked(ChooseFolder).mockResolvedValueOnce('/prints');
        el('tab-print').click();
        el('print-browse').click();
        await vi.waitFor(() => expect(el<HTMLInputElement>('print-dir').value).toBe('/prints'));
        expect(await exportDir()).toBe('/prints');
    });

    it('starts the save dialog in the output folder when Print covers shows that', async () => {
        el<HTMLInputElement>('output-dir').value = '/out';
        expect(await exportDir()).toBe('/out');
    });
});
