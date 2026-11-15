import {GetSettings, SaveSettings} from '../wailsjs/go/main/App';
import {settings} from '../wailsjs/go/models';
import {BrowserOpenURL} from '../wailsjs/runtime/runtime';

// The Settings dialog: the TMDB API key and the TMDB attribution notice.

export interface SettingsDialog {
    // open shows the dialog, prefilled with the saved settings.
    open(): void;
    // isOpen reports whether the dialog is shown.
    isOpen(): boolean;
    // hasTMDBKey reports whether a TMDB API key is saved.
    hasTMDBKey(): boolean;
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
        link: byId<HTMLAnchorElement>('tmdb-link'),
        status: byId<HTMLSpanElement>('settings-status'),
        save: byId<HTMLButtonElement>('settings-save'),
        close: byId<HTMLButtonElement>('settings-close'),
    };

    let current = new settings.Settings({tmdbApiKey: ''});

    function setStatus(text: string, kind: '' | 'ok' | 'error' = '') {
        ui.status.textContent = text;
        ui.status.className = `settings-status ${kind}`.trim();
    }

    function showKey(on: boolean) {
        ui.key.type = on ? 'text' : 'password';
        ui.toggle.textContent = on ? 'Hide' : 'Show';
    }

    async function load(): Promise<void> {
        current = await GetSettings();
        opts.onChange();
    }

    async function open() {
        setStatus('');
        showKey(false);
        try {
            await load();
        } catch (err) {
            setStatus(`Could not read the settings: ${errorText(err)}`, 'error');
        }
        ui.key.value = current.tmdbApiKey || '';
        if (!ui.dialog.open) ui.dialog.showModal();
        ui.key.focus();
    }

    let saving = false;

    async function save() {
        // Enter in the key field bypasses the disabled button, so guard here.
        if (saving) return;
        saving = true;
        ui.save.disabled = true;
        setStatus('Saving…');
        try {
            await SaveSettings(new settings.Settings({tmdbApiKey: ui.key.value}));
        } catch (err) {
            setStatus(errorText(err), 'error');
            return;
        } finally {
            saving = false;
            ui.save.disabled = false;
        }
        try {
            await load();
            ui.key.value = current.tmdbApiKey || '';
            setStatus('Saved', 'ok');
        } catch (err) {
            setStatus(`Saved, but could not reload the settings: ${errorText(err)}`, 'error');
        }
    }

    ui.toggle.addEventListener('click', () => showKey(ui.key.type === 'password'));
    ui.link.addEventListener('click', (e) => {
        // Open the page in the system browser; never navigate the webview.
        e.preventDefault();
        BrowserOpenURL(ui.link.href);
    });
    ui.save.addEventListener('click', save);
    ui.key.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && !e.ctrlKey && !e.metaKey) {
            e.preventDefault();
            save();
        }
    });
    ui.key.addEventListener('input', () => setStatus(''));
    ui.close.addEventListener('click', () => ui.dialog.close());
    // Escape closes a modal <dialog> natively (the "cancel" event).

    load().catch(() => {
        // The app still works without saved settings; open() shows the error.
    });

    return {
        open: () => void open(),
        isOpen: () => ui.dialog.open,
        hasTMDBKey: () => (current.tmdbApiKey || '').trim() !== '',
    };
}
