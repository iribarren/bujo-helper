import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';

import {afterEach, beforeAll, describe, expect, it, vi} from 'vitest';

import {GetSettings, LetteringFonts, SaveSettings} from '../wailsjs/go/main/App';
import {BrowserOpenURL} from '../wailsjs/runtime/runtime';
import {applyDom, setLang} from './i18n';
import {initSettings, type SettingsDialog} from './settings';

// Fonts as LetteringFonts sends them (see app.go): the Go fonts first, an
// OFL font and the Apache 2.0 one.
const {fontList} = vi.hoisted(() => {
    const font = (id: string, label: string, licence: string) => ({
        id, label, licence, group: 'display', hollow: 'stroke',
        url: `/fonts/${id}.ttf`, licenceUrl: `/fonts/${id}/licence`, default: id === 'go',
    });
    return {
        fontList: [
            font('go', 'Default', 'BSD 3-Clause (Go fonts)'),
            font('pacifico', 'Pacifico', 'SIL OFL 1.1'),
            font('luckiest-guy', 'Luckiest Guy', 'Apache 2.0'),
        ],
    };
});

vi.mock('../wailsjs/go/main/App', async (importOriginal) => {
    const original = await importOriginal<Record<string, unknown>>();
    const stubs: Record<string, unknown> = {};
    for (const name of Object.keys(original)) {
        stubs[name] = vi.fn(async () => undefined);
    }
    stubs.GetSettings = vi.fn(async () => ({tmdbApiKey: '', igdbClientId: '', igdbClientSecret: '', bhlApiKey: '', bggToken: '', language: ''}));
    stubs.LetteringFonts = vi.fn(async () => fontList);
    // The lists of the Sections tree.
    const items = (...ids: string[]) => ids.map((id, i) => ({id, label: id, default: i === 0}));
    stubs.MediaTypes = vi.fn(async () => items('books', 'movies'));
    stubs.PrintPapers = vi.fn(async () => items('a4', 'letter', 'a5'));
    stubs.PrintFormats = vi.fn(async () => items('covers', 'cards'));
    stubs.PrintSizes = vi.fn(async () => items('standard', 'mini'));
    stubs.PrintCropModes = vi.fn(async () => items('wallet'));
    stubs.PrintCutModes = vi.fn(async () => items('border', 'none'));
    stubs.PageKinds = vi.fn(async () => items('dotgrid', 'moon'));
    return stubs;
});
vi.mock('../wailsjs/runtime/runtime', () => ({BrowserOpenURL: vi.fn()}));

const fetchMock = vi.fn();
vi.stubGlobal('fetch', fetchMock);

function el<T extends HTMLElement>(id: string): T {
    const node = document.getElementById(id);
    if (!node) throw new Error(`missing #${id}`);
    return node as T;
}

// fontItem is the About entry of the font id.
function fontItem(id: string): HTMLDetailsElement {
    const node = el('about-fonts').querySelector<HTMLDetailsElement>(`details[data-font="${id}"]`);
    if (!node) throw new Error(`missing font ${id}`);
    return node;
}

// toggle opens or closes a font's licence as a click on its summary would,
// and waits for the "toggle" event jsdom queues.
async function toggle(item: HTMLDetailsElement, open: boolean) {
    item.open = open;
    await new Promise((done) => setTimeout(done, 0));
}

function licenceText(text: string) {
    return {ok: true, status: 200, text: async () => text} as Response;
}

let dialog: SettingsDialog;

beforeAll(() => {
    const html = readFileSync(resolve(__dirname, '../index.html'), 'utf8');
    document.body.innerHTML = html.slice(html.indexOf('<body>') + '<body>'.length, html.indexOf('</body>'));
    // jsdom has no modal dialogs.
    HTMLDialogElement.prototype.showModal ??= function (this: HTMLDialogElement) {
        this.open = true;
    };
    HTMLDialogElement.prototype.close ??= function (this: HTMLDialogElement) {
        this.open = false;
    };
    dialog = initSettings({onChange: () => {}});
});

afterEach(() => {
    setLang('en');
    applyDom(document);
});

describe('Settings About', () => {
    it('shows the font list error and lists the fonts at the next open', async () => {
        vi.mocked(LetteringFonts).mockRejectedValueOnce(new Error('boom'));
        dialog.open();
        await vi.waitFor(() => expect(el('about-fonts-status').textContent).toBe('Could not load lettering fonts: boom'));
        expect(el('about-fonts').children.length).toBe(0);

        dialog.open();
        await vi.waitFor(() => expect(el('about-fonts').querySelectorAll('details').length).toBe(fontList.length));
        expect(el('about-fonts-status').textContent).toBe('');
    });

    it('lists every bundled font with its licence name, and pdf.js', async () => {
        dialog.open();
        await vi.waitFor(() => expect(el('about-fonts').querySelectorAll('details').length).toBe(fontList.length));
        const rows = Array.from(el('about-fonts').querySelectorAll('summary')).map((s) => s.textContent?.replace(/\s+/g, ' ').trim());
        expect(rows).toEqual([
            'Go fonts (default) BSD 3-Clause (Go fonts)',
            'Pacifico SIL OFL 1.1',
            'Luckiest Guy Apache 2.0',
        ]);
        expect(el('settings-about').textContent).toContain('pdf.js');
        expect(el('settings-about').textContent).toContain('Apache 2.0');
        // The dialog lists the fonts once, however often it opens.
        dialog.open();
        await Promise.resolve();
        expect(el('about-fonts').querySelectorAll('details').length).toBe(fontList.length);
    });

    it('fetches a licence once, when it is first opened, and shows it', async () => {
        fetchMock.mockResolvedValueOnce(licenceText('SIL OPEN FONT LICENSE Version 1.1'));
        const item = fontItem('pacifico');
        expect(fetchMock).not.toHaveBeenCalled();
        await toggle(item, true);
        await vi.waitFor(() => expect(item.querySelector('pre')?.textContent).toBe('SIL OPEN FONT LICENSE Version 1.1'));
        expect(fetchMock).toHaveBeenCalledTimes(1);
        expect(fetchMock).toHaveBeenCalledWith('/fonts/pacifico/licence');
        await toggle(item, false);
        await toggle(item, true);
        expect(fetchMock).toHaveBeenCalledTimes(1);
        expect(item.querySelector('pre')?.textContent).toBe('SIL OPEN FONT LICENSE Version 1.1');
    });

    it('shows an error when the licence cannot be read, and tries again at the next open', async () => {
        fetchMock.mockResolvedValueOnce({ok: false, status: 404, text: async () => 'not found'} as Response);
        const item = fontItem('luckiest-guy');
        await toggle(item, true);
        await vi.waitFor(() => expect(item.querySelector('[role="alert"]')?.textContent).toBe('Could not read the licence: HTTP 404'));
        expect(item.querySelector('pre')?.hidden).toBe(true);

        fetchMock.mockRejectedValueOnce(new Error('offline'));
        await toggle(item, false);
        await toggle(item, true);
        await vi.waitFor(() => expect(item.querySelector('[role="alert"]')?.textContent).toBe('Could not read the licence: offline'));

        fetchMock.mockResolvedValueOnce(licenceText('Apache License Version 2.0'));
        await toggle(item, false);
        await toggle(item, true);
        await vi.waitFor(() => expect(item.querySelector('pre')?.textContent).toBe('Apache License Version 2.0'));
        expect(item.querySelector('pre')?.hidden).toBe(false);
        expect(item.querySelector('[role="alert"]')?.textContent).toBe('');
        expect(fetchMock).toHaveBeenCalledTimes(3);
    });

    it('names the Go fonts and the section in Spanish', () => {
        setLang('es');
        applyDom(document);
        expect(el('settings-about').querySelector('h3')?.textContent).toBe('Acerca de');
        expect(fontItem('go').querySelector('summary')?.textContent).toContain('Fuentes Go (predeterminadas)');
    });
});

describe('BoardGameGeek token', () => {
    const saved = {tmdbApiKey: '', igdbClientId: '', igdbClientSecret: '', bhlApiKey: '', bggToken: 'saved-token', language: ''};

    afterEach(() => {
        el('settings-close').click();
        vi.mocked(SaveSettings).mockClear();
    });

    it('loads the saved token into a masked input that the toggle shows', async () => {
        vi.mocked(GetSettings).mockResolvedValueOnce(saved as never);
        dialog.open();
        await vi.waitFor(() => expect(el<HTMLInputElement>('bgg-token').value).toBe('saved-token'));
        expect(dialog.hasBGGToken()).toBe(true);
        const input = el<HTMLInputElement>('bgg-token');
        const toggle = el<HTMLButtonElement>('bgg-token-toggle');
        expect(input.type).toBe('password');
        expect(toggle.textContent).toBe('Show');
        toggle.click();
        expect(input.type).toBe('text');
        expect(toggle.textContent).toBe('Hide');
        toggle.click();
        expect(input.type).toBe('password');
    });

    it('saves the token with the other settings when Enter is pressed in it', async () => {
        dialog.open();
        await vi.waitFor(() => expect(el<HTMLDialogElement>('settings-dialog').open).toBe(true));
        const input = el<HTMLInputElement>('bgg-token');
        input.value = 'new-token';
        input.dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', bubbles: true, cancelable: true}));
        await vi.waitFor(() => expect(SaveSettings).toHaveBeenCalledTimes(1));
        expect(vi.mocked(SaveSettings).mock.calls[0][0]).toMatchObject({bggToken: 'new-token'});
    });

    it('reports no token when none is saved', async () => {
        dialog.open();
        await vi.waitFor(() => expect(el<HTMLInputElement>('bgg-token').value).toBe(''));
        expect(dialog.hasBGGToken()).toBe(false);
    });

    it('says when it is needed and links to the BoardGameGeek applications page', () => {
        expect(el('bgg-token').closest('.field')?.textContent).toContain('Only needed when this build has no BoardGameGeek token built in.');
        const link = el<HTMLAnchorElement>('bgg-link');
        expect(link.href).toBe('https://boardgamegeek.com/applications');
        const click = new MouseEvent('click', {bubbles: true, cancelable: true});
        link.dispatchEvent(click);
        expect(click.defaultPrevented).toBe(true);
        expect(BrowserOpenURL).toHaveBeenCalledWith('https://boardgamegeek.com/applications');
    });
});

describe('Settings About BoardGameGeek credit', () => {
    it('credits BoardGameGeek with the Powered by BGG logo linked to it', () => {
        const about = el('settings-about');
        expect(about.textContent).toContain('BoardGameGeek');
        const link = el<HTMLAnchorElement>('bgg-logo-about');
        expect(about.contains(link)).toBe(true);
        expect(link.href).toBe('https://boardgamegeek.com/');
        expect(link.getAttribute('aria-label')).toBe('Powered by BGG');
        const images = Array.from(link.querySelectorAll('img'));
        expect(images.map((img) => img.getAttribute('src') ?? '')).toEqual([
            expect.stringContaining('powered-by-bgg-rgb'),
            expect.stringContaining('powered-by-bgg-reversed-rgb'),
        ]);
        for (const img of images) expect(img.alt).toBe('Powered by BGG');
        const click = new MouseEvent('click', {bubbles: true, cancelable: true});
        link.dispatchEvent(click);
        expect(click.defaultPrevented).toBe(true);
        expect(BrowserOpenURL).toHaveBeenCalledWith('https://boardgamegeek.com/');
    });

    it('names the token field in Spanish', () => {
        setLang('es');
        applyDom(document);
        expect(document.querySelector('label[for="bgg-token"]')?.textContent).toBe('Token de BoardGameGeek');
    });
});

describe('Theme', () => {
    const saved = {tmdbApiKey: '', igdbClientId: '', igdbClientSecret: '', bhlApiKey: '', bggToken: '', language: ''};
    const root = document.documentElement;

    // themeRadio is the Theme radio for value ("" is System).
    function themeRadio(value: string): HTMLButtonElement {
        const node = el('theme').querySelector<HTMLButtonElement>(`[role="radio"][data-value="${value}"]`);
        if (!node) throw new Error(`missing theme radio "${value}"`);
        return node;
    }

    function checkedTheme(): string | undefined {
        return el('theme').querySelector<HTMLButtonElement>('[aria-checked="true"]')?.dataset.value;
    }

    afterEach(() => {
        el('settings-close').click();
        vi.mocked(SaveSettings).mockClear();
        delete root.dataset.theme;
    });

    it('comes right after the language, with System, Light and Dark', () => {
        const field = el('theme').closest('.field');
        expect(field?.previousElementSibling).toBe(el('language').closest('.field'));
        expect(el('theme').getAttribute('role')).toBe('radiogroup');
        const radios = Array.from(el('theme').querySelectorAll<HTMLButtonElement>('[role="radio"]'));
        expect(radios.map((r) => [r.dataset.value, r.textContent])).toEqual([
            ['', 'System'],
            ['light', 'Light'],
            ['dark', 'Dark'],
        ]);
        expect(document.getElementById(el('theme').getAttribute('aria-labelledby') ?? '')?.textContent).toBe('Theme');
    });

    it('shows the saved theme, and System when none is saved', async () => {
        vi.mocked(GetSettings).mockResolvedValueOnce({...saved, theme: 'light'} as never);
        dialog.open();
        await vi.waitFor(() => expect(checkedTheme()).toBe('light'));
        el('settings-close').click();

        vi.mocked(GetSettings).mockResolvedValueOnce(saved as never);
        dialog.open();
        await vi.waitFor(() => expect(checkedTheme()).toBe(''));
    });

    it('saves the picked theme with the other settings and applies it at once', async () => {
        dialog.open();
        await vi.waitFor(() => expect(checkedTheme()).toBe(''));
        themeRadio('dark').click();
        expect(root.hasAttribute('data-theme')).toBe(false);
        el('settings-save').click();
        await vi.waitFor(() => expect(SaveSettings).toHaveBeenCalledTimes(1));
        expect(vi.mocked(SaveSettings).mock.calls[0][0]).toMatchObject({theme: 'dark', language: '', bggToken: ''});
        await vi.waitFor(() => expect(root.dataset.theme).toBe('dark'));
    });

    it('follows the system again when System is saved', async () => {
        root.dataset.theme = 'dark';
        vi.mocked(GetSettings).mockResolvedValueOnce({...saved, theme: 'dark'} as never);
        dialog.open();
        await vi.waitFor(() => expect(checkedTheme()).toBe('dark'));
        themeRadio('').click();
        el('settings-save').click();
        await vi.waitFor(() => expect(SaveSettings).toHaveBeenCalledTimes(1));
        expect(vi.mocked(SaveSettings).mock.calls[0][0]).toMatchObject({theme: ''});
        await vi.waitFor(() => expect(root.hasAttribute('data-theme')).toBe(false));
    });

    it('keeps the current theme when saving fails', async () => {
        vi.mocked(SaveSettings).mockRejectedValueOnce(new Error('disk full'));
        dialog.open();
        await vi.waitFor(() => expect(checkedTheme()).toBe(''));
        themeRadio('light').click();
        el('settings-save').click();
        await vi.waitFor(() => expect(el('settings-status').textContent).toBe('disk full'));
        expect(root.hasAttribute('data-theme')).toBe(false);
    });

    it('names the theme and its options in Spanish, keeping the choice', async () => {
        vi.mocked(GetSettings).mockResolvedValueOnce({...saved, theme: 'dark'} as never);
        dialog.open();
        await vi.waitFor(() => expect(checkedTheme()).toBe('dark'));
        setLang('es');
        applyDom(document);
        expect(document.getElementById('theme-label')?.textContent).toBe('Tema');
        const labels = Array.from(el('theme').querySelectorAll('[role="radio"]')).map((r) => r.textContent);
        expect(labels).toEqual(['Sistema', 'Claro', 'Oscuro']);
        expect(checkedTheme()).toBe('dark');
    });
});

describe('Sections shown', () => {
    const saved = {tmdbApiKey: '', igdbClientId: '', igdbClientSecret: '', bhlApiKey: '', bggToken: '', language: ''};

    function sectionBox(id: string): HTMLInputElement {
        const node = el('view-config').querySelector<HTMLInputElement>(`[data-section="${id}"] > legend input`);
        if (!node) throw new Error(`missing section ${id}`);
        return node;
    }

    function box(list: string, item: string): HTMLInputElement {
        const node = el('view-config').querySelector<HTMLInputElement>(`[data-list="${list}"] input[data-item="${item}"]`);
        if (!node) throw new Error(`missing ${list}/${item}`);
        return node;
    }

    async function openWith(view?: object) {
        vi.mocked(GetSettings).mockResolvedValueOnce({...saved, view} as never);
        dialog.open();
        await vi.waitFor(() => expect(el<HTMLDialogElement>('settings-dialog').open).toBe(true));
        await vi.waitFor(() => expect(box('paper', 'a4')).toBeTruthy());
    }

    afterEach(() => {
        el('settings-close').click();
        vi.mocked(SaveSettings).mockClear();
        delete document.documentElement.dataset.theme;
    });

    it('comes after the theme and before the API keys, under its heading', () => {
        const part = el('settings-sections');
        expect(part.previousElementSibling).toBe(el('theme').closest('.field'));
        expect(part.nextElementSibling?.contains(el('tmdb-key'))).toBe(true);
        expect(document.getElementById(part.getAttribute('aria-labelledby') ?? '')?.textContent).toBe('Sections shown');
        expect(part.contains(el('view-config'))).toBe(true);
    });

    it('lists the sections and the items Go sends, all ticked when no view is saved', async () => {
        await openWith(undefined);
        const sections = Array.from(el('view-config').querySelectorAll<HTMLElement>('[data-section]'), (s) => s.dataset.section);
        expect(sections).toEqual(['download', 'print', 'pages']);
        for (const input of el('view-config').querySelectorAll<HTMLInputElement>('input[data-item]')) expect(input.checked).toBe(true);
        expect(sectionBox('print').disabled).toBe(true);
    });

    it('shows the saved view each time it opens', async () => {
        await openWith({hiddenSections: ['download'], hidden: {paper: ['letter']}});
        expect(sectionBox('download').checked).toBe(false);
        expect(box('paper', 'letter').checked).toBe(false);
        box('paper', 'a5').click();
        el('settings-close').click();

        await openWith(undefined);
        expect(sectionBox('download').checked).toBe(true);
        expect(box('paper', 'letter').checked).toBe(true);
        expect(box('paper', 'a5').checked).toBe(true);
    });

    it('shows the default ticked when the saved view hides a whole list', async () => {
        await openWith({hidden: {pageKind: ['dotgrid', 'moon']}});
        expect(box('pageKind', 'dotgrid').checked).toBe(true);
        expect(box('pageKind', 'moon').checked).toBe(false);
        expect(box('pageKind', 'dotgrid').disabled).toBe(true);
    });

    it('saves the view with the other settings and the theme', async () => {
        await openWith(undefined);
        box('paper', 'letter').click();
        sectionBox('pages').click();
        document.querySelector<HTMLButtonElement>('#theme [data-value="dark"]')?.click();
        el<HTMLInputElement>('bgg-token').value = 'token';
        el('settings-save').click();
        await vi.waitFor(() => expect(SaveSettings).toHaveBeenCalledTimes(1));
        expect(vi.mocked(SaveSettings).mock.calls[0][0]).toMatchObject({
            theme: 'dark',
            bggToken: 'token',
            language: '',
            view: {hiddenSections: ['pages'], hidden: {paper: ['letter']}},
        });
        await vi.waitFor(() => expect(el('settings-status').textContent).toBe('Saved'));
    });

    it('gives the saved view once loaded, and the new one once saved', async () => {
        await openWith({hiddenSections: ['pages'], hidden: {paper: ['letter']}});
        expect(dialog.view()).toEqual({hiddenSections: ['pages'], hidden: {paper: ['letter']}});
        box('paper', 'a5').click();
        // Unsaved ticks are not the saved view.
        expect(dialog.view()).toEqual({hiddenSections: ['pages'], hidden: {paper: ['letter']}});
        vi.mocked(GetSettings).mockResolvedValueOnce({...saved, view: {hidden: {paper: ['letter', 'a5']}}} as never);
        el('settings-save').click();
        await vi.waitFor(() => expect(el('settings-status').textContent).toBe('Saved'));
        expect(dialog.view()).toEqual({hidden: {paper: ['letter', 'a5']}});
    });

    it('gives no view for an older settings file', async () => {
        await openWith(undefined);
        expect(dialog.view()).toBeUndefined();
    });

    it('saves an empty view when everything is shown', async () => {
        await openWith(undefined);
        el('settings-save').click();
        await vi.waitFor(() => expect(SaveSettings).toHaveBeenCalledTimes(1));
        const view = vi.mocked(SaveSettings).mock.calls[0][0].view;
        expect(view?.hiddenSections).toBeUndefined();
        expect(view?.hidden).toBeUndefined();
    });

    it('keeps the ticks when saving fails', async () => {
        vi.mocked(SaveSettings).mockRejectedValueOnce(new Error('disk full'));
        await openWith(undefined);
        box('paper', 'letter').click();
        el('settings-save').click();
        await vi.waitFor(() => expect(el('settings-status').textContent).toBe('disk full'));
        expect(box('paper', 'letter').checked).toBe(false);
    });

    it('clears the status when a box changes', async () => {
        vi.mocked(SaveSettings).mockRejectedValueOnce(new Error('disk full'));
        await openWith(undefined);
        el('settings-save').click();
        await vi.waitFor(() => expect(el('settings-status').textContent).toBe('disk full'));
        box('media', 'movies').click();
        expect(el('settings-status').textContent).toBe('');
    });

    it('names the part in Spanish', async () => {
        await openWith(undefined);
        setLang('es');
        applyDom(document);
        expect(el('sections-title').textContent).toBe('Secciones visibles');
        expect(el('view-config').querySelector('[data-section="pages"] > legend label')?.textContent?.trim()).toBe('Crear páginas');
        expect(el('view-config').querySelector('[data-section="print"] .view-note')?.textContent).toBe('Siempre visible');
    });
});
