import {GetSettings, SaveSettings} from '../wailsjs/go/main/App';
import {settings} from '../wailsjs/go/models';
import {BrowserOpenURL} from '../wailsjs/runtime/runtime';

// The Settings dialog: the TMDB API key and the TMDB attribution notice,
// and the IGDB (Twitch) Client ID and Client Secret.

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
        key: byId<HTMLInputElement>('tmdb-key'),
        toggle: byId<HTMLButtonElement>('tmdb-key-toggle'),
        igdbId: byId<HTMLInputElement>('igdb-id'),
        igdbSecret: byId<HTMLInputElement>('igdb-secret'),
        igdbToggle: byId<HTMLButtonElement>('igdb-secret-toggle'),
        links: [byId<HTMLAnchorElement>('tmdb-link'), byId<HTMLAnchorElement>('twitch-link')],
        status: byId<HTMLSpanElement>('settings-status'),
        save: byId<HTMLButtonElement>('settings-save'),
        close: byId<HTMLButtonElement>('settings-close'),
    };

    let current = new settings.Settings({tmdbApiKey: '', igdbClientId: '', igdbClientSecret: ''});

    function setStatus(text: string, kind: '' | 'ok' | 'error' = '') {
        ui.status.textContent = text;
        ui.status.className = `settings-status ${kind}`.trim();
    }

    // showSecret shows or masks a secret input and names its toggle.
    function showSecret(input: HTMLInputElement, toggle: HTMLButtonElement, on: boolean) {
        input.type = on ? 'text' : 'password';
        toggle.textContent = on ? 'Hide' : 'Show';
    }

    // fill shows the current settings in the inputs.
    function fill() {
        ui.key.value = current.tmdbApiKey || '';
        ui.igdbId.value = current.igdbClientId || '';
        ui.igdbSecret.value = current.igdbClientSecret || '';
    }

    async function load(): Promise<void> {
        current = await GetSettings();
        opts.onChange();
    }

    async function open() {
        setStatus('');
        showSecret(ui.key, ui.toggle, false);
        showSecret(ui.igdbSecret, ui.igdbToggle, false);
        try {
            await load();
        } catch (err) {
            setStatus(`Could not read the settings: ${errorText(err)}`, 'error');
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
        setStatus('Saving…');
        try {
            await SaveSettings(new settings.Settings({
                tmdbApiKey: ui.key.value,
                igdbClientId: ui.igdbId.value,
                igdbClientSecret: ui.igdbSecret.value,
            }));
        } catch (err) {
            setStatus(errorText(err), 'error');
            return;
        } finally {
            saving = false;
            ui.save.disabled = false;
        }
        try {
            await load();
            fill();
            setStatus('Saved', 'ok');
        } catch (err) {
            setStatus(`Saved, but could not reload the settings: ${errorText(err)}`, 'error');
        }
    }

    ui.toggle.addEventListener('click', () => showSecret(ui.key, ui.toggle, ui.key.type === 'password'));
    ui.igdbToggle.addEventListener('click', () =>
        showSecret(ui.igdbSecret, ui.igdbToggle, ui.igdbSecret.type === 'password'));
    for (const link of ui.links) {
        link.addEventListener('click', (e) => {
            // Open the page in the system browser; never navigate the webview.
            e.preventDefault();
            BrowserOpenURL(link.href);
        });
    }
    ui.save.addEventListener('click', save);
    for (const input of [ui.key, ui.igdbId, ui.igdbSecret]) {
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

    load().catch(() => {
        // The app still works without saved settings; open() shows the error.
    });

    return {
        open: () => void open(),
        isOpen: () => ui.dialog.open,
        hasTMDBKey: () => (current.tmdbApiKey || '').trim() !== '',
        hasIGDBCredentials: () =>
            (current.igdbClientId || '').trim() !== '' && (current.igdbClientSecret || '').trim() !== '',
    };
}
