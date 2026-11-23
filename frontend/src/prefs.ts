// Remembering the Print covers and Make pages options between runs. Each tab
// restores its saved options once its lists have loaded (an id no longer
// offered leaves the default) and saves them again on each user change,
// through Go (SavePrintPrefs, SavePagesPrefs) into the settings file.

function errorText(err: unknown): string {
    return err instanceof Error ? err.message : String(err);
}

// restoreSelect selects id in select when it is one of its options, and
// reports whether it did; otherwise the current choice stays.
export function restoreSelect(select: HTMLSelectElement, id: string | undefined): boolean {
    if (!id || !Array.from(select.options).some((option) => option.value === id)) return false;
    select.value = id;
    return true;
}

export interface PrefsSaver {
    // Starts saving, from the options shown now: call it once the saved
    // options are restored, so startup never overwrites them.
    start: () => void;
    // Saves the options, unless they are what was saved last. Saves run
    // one at a time; changes made meanwhile are saved after.
    save: () => void;
}

// prefsSaver saves read() with save on each change once started. A failed
// save is logged, never shown: the options still work, they are only not
// remembered. what names the tab in the log.
export function prefsSaver<T>(what: string, read: () => T, save: (prefs: T) => Promise<void>): PrefsSaver {
    let started = false;
    let saving = false;
    let again = false;
    let last = '';

    async function flush() {
        if (saving) {
            again = true;
            return;
        }
        saving = true;
        try {
            do {
                again = false;
                const prefs = read();
                const key = JSON.stringify(prefs);
                if (key === last) continue;
                try {
                    await save(prefs);
                    last = key;
                } catch (err) {
                    console.warn(`Could not remember the ${what} options: ${errorText(err)}`);
                }
            } while (again);
        } finally {
            saving = false;
        }
    }

    return {
        start: () => {
            started = true;
            last = JSON.stringify(read());
        },
        save: () => {
            if (started) void flush();
        },
    };
}
