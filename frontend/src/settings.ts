import {GetSettings, SaveSettings} from '../wailsjs/go/main/App';
import {settings} from '../wailsjs/go/models';
import {BrowserOpenURL} from '../wailsjs/runtime/runtime';
import {onLangChange, t} from './i18n';
import {useLanguage} from './language';

// The Settings dialog: the UI language, the TMDB API key and the TMDB
// attribution notice, the IGDB (Twitch) Client ID and Client Secret, and the
// Biodiversity Heritage Library (BHL) API key.

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
