import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';

import {afterEach, beforeAll, describe, expect, it, vi} from 'vitest';

import {LetteringFonts} from '../wailsjs/go/main/App';
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
    stubs.GetSettings = vi.fn(async () => ({tmdbApiKey: '', igdbClientId: '', igdbClientSecret: '', bhlApiKey: '', language: ''}));
    stubs.LetteringFonts = vi.fn(async () => fontList);
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
