import {GetSettings, SaveSettings} from '../wailsjs/go/main/App';
import {main, settings} from '../wailsjs/go/models';
import {BrowserOpenURL} from '../wailsjs/runtime/runtime';
import {onLangChange, t} from './i18n';
import {useLanguage} from './language';
import {letteringFonts} from './lettering';

// The Settings dialog: the UI language, the TMDB API key and the TMDB
// attribution notice, the IGDB (Twitch) Client ID and Client Secret, the
// Biodiversity Heritage Library (BHL) API key, and an About section that
// lists the bundled lettering fonts with their licences.

export interface SettingsDialog {
    // open shows the dialog, prefilled with the saved settings.
    open(): void;
    // isOpen reports whether the dialog is shown.
    isOpen(): boolean;
    // hasTMDBKey reports whether a TMDB API key is saved.
    hasTMDBKey(): boolean;
    // hasIGDBCredentials reports whether an IGDB Client ID and Client
    // Secret are both saved.
    hasIGDBCredentials(): boolean;
    // hasBHLKey reports whether a BHL API key is saved.
    hasBHLKey(): boolean;
}

export interface SettingsOptions {
    // onChange is called when the saved settings are loaded or saved.
    onChange(): void;
}

function byId<T extends HTMLElement>(id: string): T {
    const node = document.getElementById(id);
    if (!node) throw new Error(`missing #${id}`);
    return node as T;
}

function errorText(err: unknown): string {
    if (err instanceof Error) return err.message;
    return String(err);
}

interface About {
    // load lists the fonts, once; a failure shows and the next call retries.
    load(): void;
}

// initAbout fills the About section: one <details> per bundled font, from
// the list the Lettering controls already loaded (letteringFonts), with its
// licence name. A licence text is fetched from its served URL the first time
// its entry opens and shown in a scrollable <pre>, so the webview never
// navigates away. A failed fetch shows why and is tried again at the next
// open.
function initAbout(): About {
    const list = byId<HTMLUListElement>('about-fonts');
    const status = byId<HTMLParagraphElement>('about-fonts-status');
    // Relabels the dynamic text of each entry for the current language.
    const relabels: Array<() => void> = [];
    let listed = false;
    let loading = false;
    let listError = '';

    function showListError() {
        status.textContent = listError ? t('lettering.errFonts', {error: listError}) : '';
    }

    function fontEntry(font: main.LetteringFontInfo): HTMLLIElement {
        const item = document.createElement('li');
        const details = document.createElement('details');
        details.dataset.font = font.id;
        const summary = document.createElement('summary');
        const name = document.createElement('span');
        name.className = 'about-font-name';
        // The default font is named by its family, not "Default".
        if (font.default) name.dataset.i18n = 'about.goFonts';
        const licence = document.createElement('span');
        licence.className = 'about-font-licence';
        licence.textContent = font.licence;
        summary.append(name, ' ', licence);
        const message = document.createElement('p');
        message.className = 'settings-status error';
        message.setAttribute('role', 'alert');
        const text = document.createElement('pre');
        text.className = 'licence-text';
        // Focusable, so the keyboard can scroll it.
        text.tabIndex = 0;
        text.hidden = true;
        details.append(summary, message, text);
        item.append(details);

        // "idle" until the text loads; a failure returns to "idle".
        let state: 'idle' | 'loading' | 'done' = 'idle';
        let error = '';

        function relabel() {
            const fontName = font.default ? t('about.goFonts') : font.label;
            name.textContent = fontName;
            text.setAttribute('aria-label', t('about.licenceTitle', {font: fontName}));
            if (state === 'loading') text.textContent = t('about.loading');
            message.textContent = error ? t('about.errLicence', {error}) : '';
        }

        details.addEventListener('toggle', async () => {
            if (!details.open || state !== 'idle') return;
            state = 'loading';
            error = '';
            text.hidden = false;
            details.setAttribute('aria-busy', 'true');
            relabel();
            try {
                const res = await fetch(font.licenceUrl);
                if (!res.ok) throw new Error(`HTTP ${res.status}`);
                text.textContent = await res.text();
                state = 'done';
            } catch (err) {
                error = errorText(err);
                text.textContent = '';
                text.hidden = true;
                state = 'idle';
            } finally {
                details.removeAttribute('aria-busy');
            }
            relabel();
        });

        relabels.push(relabel);
        relabel();
        return item;
    }

    onLangChange(() => {
        for (const relabel of relabels) relabel();
        showListError();
    });

    return {
        load() {
            if (listed || loading) return;
            loading = true;
            letteringFonts().then((fonts) => {
                list.replaceChildren(...fonts.map(fontEntry));
                listed = true;
                listError = '';
            }, (err) => {
                listError = errorText(err);
            }).finally(() => {
                loading = false;
                showListError();
            });
        },
    };
}

export function initSettings(opts: SettingsOptions): SettingsDialog {
    const ui = {
        dialog: byId<HTMLDialogElement>('settings-dialog'),
        language: byId<HTMLSelectElement>('language'),
        key: byId<HTMLInputElement>('tmdb-key'),
        toggle: byId<HTMLButtonElement>('tmdb-key-toggle'),
        igdbId: byId<HTMLInputElement>('igdb-id'),
        igdbSecret: byId<HTMLInputElement>('igdb-secret'),
        igdbToggle: byId<HTMLButtonElement>('igdb-secret-toggle'),
        bhlKey: byId<HTMLInputElement>('bhl-key'),
        bhlToggle: byId<HTMLButtonElement>('bhl-key-toggle'),
        links: [
            byId<HTMLAnchorElement>('tmdb-link'),
            byId<HTMLAnchorElement>('twitch-link'),
            byId<HTMLAnchorElement>('bhl-link'),
        ],
        status: byId<HTMLSpanElement>('settings-status'),
        save: byId<HTMLButtonElement>('settings-save'),
        close: byId<HTMLButtonElement>('settings-close'),
    };

    const about = initAbout();

    let current = new settings.Settings({
        tmdbApiKey: '',
        igdbClientId: '',
        igdbClientSecret: '',
        bhlApiKey: '',
        language: '',
    });

    function setStatus(text: string, kind: '' | 'ok' | 'error' = '') {
        ui.status.textContent = text;
        ui.status.className = `settings-status ${kind}`.trim();
    }

    // showSecret shows or masks a secret input and names its toggle.
    function showSecret(input: HTMLInputElement, toggle: HTMLButtonElement, on: boolean) {
        input.type = on ? 'text' : 'password';
        toggle.textContent = on ? t('settings.hide') : t('settings.show');
    }

    // labelToggles names the secret toggles for how their inputs are shown.
    function labelToggles() {
        showSecret(ui.key, ui.toggle, ui.key.type === 'text');
        showSecret(ui.igdbSecret, ui.igdbToggle, ui.igdbSecret.type === 'text');
        showSecret(ui.bhlKey, ui.bhlToggle, ui.bhlKey.type === 'text');
    }

    // fill shows the current settings in the inputs.
    function fill() {
        ui.language.value = current.language || '';
        ui.key.value = current.tmdbApiKey || '';
        ui.igdbId.value = current.igdbClientId || '';
        ui.igdbSecret.value = current.igdbClientSecret || '';
        ui.bhlKey.value = current.bhlApiKey || '';
    }

    async function load(): Promise<void> {
        current = await GetSettings();
        opts.onChange();
    }

    async function open() {
        about.load();
        setStatus('');
        showSecret(ui.key, ui.toggle, false);
        showSecret(ui.igdbSecret, ui.igdbToggle, false);
        showSecret(ui.bhlKey, ui.bhlToggle, false);
        try {
            await load();
        } catch (err) {
            setStatus(t('settings.errRead', {error: errorText(err)}), 'error');
        }
        fill();
        if (!ui.dialog.open) ui.dialog.showModal();
        ui.key.focus();
    }

    let saving = false;

    async function save() {
        // Enter in a field bypasses the disabled button, so guard here.
        if (saving) return;
        saving = true;
        ui.save.disabled = true;
        setStatus(t('settings.saving'));
        try {
            await SaveSettings(new settings.Settings({
                tmdbApiKey: ui.key.value,
                igdbClientId: ui.igdbId.value,
                igdbClientSecret: ui.igdbSecret.value,
                bhlApiKey: ui.bhlKey.value,
                language: ui.language.value,
            }));
        } catch (err) {
            setStatus(errorText(err), 'error');
            return;
        } finally {
            saving = false;
            ui.save.disabled = false;
        }
        // Switch the UI (and Go) to the saved language before saying so.
        await useLanguage(ui.language.value);
        try {
            await load();
            fill();
            setStatus(t('settings.saved'), 'ok');
        } catch (err) {
            setStatus(t('settings.errReload', {error: errorText(err)}), 'error');
        }
    }

    ui.toggle.addEventListener('click', () => showSecret(ui.key, ui.toggle, ui.key.type === 'password'));
    ui.igdbToggle.addEventListener('click', () =>
        showSecret(ui.igdbSecret, ui.igdbToggle, ui.igdbSecret.type === 'password'));
    ui.bhlToggle.addEventListener('click', () => showSecret(ui.bhlKey, ui.bhlToggle, ui.bhlKey.type === 'password'));
    for (const link of ui.links) {
        link.addEventListener('click', (e) => {
            // Open the page in the system browser; never navigate the webview.
            e.preventDefault();
            BrowserOpenURL(link.href);
        });
    }
    ui.save.addEventListener('click', save);
    ui.language.addEventListener('change', () => setStatus(''));
    for (const input of [ui.key, ui.igdbId, ui.igdbSecret, ui.bhlKey]) {
        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.ctrlKey && !e.metaKey) {
                e.preventDefault();
                save();
            }
        });
        input.addEventListener('input', () => setStatus(''));
    }
    ui.close.addEventListener('click', () => ui.dialog.close());
    // Escape closes a modal <dialog> natively (the "cancel" event).

    labelToggles();
    // The status line is cleared on a language change: it may name the
    // previous language's message.
    onLangChange(() => {
        labelToggles();
        setStatus('');
    });

    load().catch(() => {
        // The app still works without saved settings; open() shows the error.
    });

    return {
        open: () => void open(),
        isOpen: () => ui.dialog.open,
        hasTMDBKey: () => (current.tmdbApiKey || '').trim() !== '',
        hasIGDBCredentials: () =>
            (current.igdbClientId || '').trim() !== '' && (current.igdbClientSecret || '').trim() !== '',
        hasBHLKey: () => (current.bhlApiKey || '').trim() !== '',
    };
}
