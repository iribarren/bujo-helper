import {afterEach, describe, expect, it, vi} from 'vitest';

import {en} from './en';
import {es} from './es';
import {applyDom, detectLang, getLang, label, onLangChange, pluralKey, setCatalog, setLang, t, tn} from './index';

afterEach(() => {
    setLang('en');
});

describe('detectLang', () => {
    const cases: Array<[setting: string, navigatorLang: string, want: string]> = [
        ['', 'es', 'es'],
        ['', 'es-ES', 'es'],
        ['', 'es-419', 'es'],
        ['', 'ES-es', 'es'],
        ['', 'en-US', 'en'],
        ['', 'fr', 'en'],
        ['', '', 'en'],
        ['en', 'es', 'en'],
        ['en', 'es-ES', 'en'],
        ['en', 'es-419', 'en'],
        ['en', 'en-US', 'en'],
        ['en', 'fr', 'en'],
        ['en', '', 'en'],
        ['es', 'es', 'es'],
        ['es', 'es-ES', 'es'],
        ['es', 'es-419', 'es'],
        ['es', 'en-US', 'es'],
        ['es', 'fr', 'es'],
        ['es', '', 'es'],
        ['de', 'es-ES', 'es'],
        ['de', 'fr', 'en'],
    ];
    it.each(cases)('setting %j, navigator %j → %s', (setting, navigatorLang, want) => {
        expect(detectLang(setting, navigatorLang)).toBe(want);
    });
});

describe('t', () => {
    it('looks up the current language', () => {
        expect(t('settings.title')).toBe('Settings');
        setLang('es');
        expect(t('settings.title')).toBe('Ajustes');
        expect(getLang()).toBe('es');
    });

    it('interpolates named parameters and keeps unknown ones', () => {
        expect(t('download.saved', {file: 'books.csv'})).toBe('Saved books.csv');
        setLang('es');
        expect(t('download.saved', {file: 'books.csv'})).toBe('Guardado: books.csv');
        expect(t('download.saved')).toBe('Guardado: {file}');
    });

    it('falls back to English, then to the key', () => {
        setLang('es');
        setCatalog('es', {});
        try {
            expect(t('settings.title')).toBe('Settings');
            expect(t('no.such.key' as never)).toBe('no.such.key');
        } finally {
            setCatalog('es', es);
        }
    });
});

describe('tn', () => {
    const cases: Array<[lang: 'en' | 'es', n: number, want: string]> = [
        ['en', 0, '0 items'],
        ['en', 1, '1 item'],
        ['en', 2, '2 items'],
        ['es', 0, '0 elementos'],
        ['es', 1, '1 elemento'],
        ['es', 2, '2 elementos'],
    ];
    it.each(cases)('%s %d → %s', (lang, n, want) => {
        setLang(lang);
        expect(tn('download.items', n)).toBe(want);
    });

    it('passes extra parameters through', () => {
        setLang('es');
        expect(tn('print.selected', 1)).toBe('1 seleccionada');
        expect(tn('print.selected', 3)).toBe('3 seleccionadas');
    });
});

describe('label', () => {
    it('translates by ID and falls back to the given label', () => {
        setLang('es');
        expect(label('media', 'books', 'Books')).toBe('Libros');
        expect(label('format', 'stubs', 'Ticket stubs')).toBe('Entradas');
        expect(label('media', 'comics', 'Comics')).toBe('Comics');
        expect(label('media', '', 'Fallback')).toBe('Fallback');
    });

    // A message "<kind>.undefined" must never answer for a missing ID, so
    // the tests add one and check it is not looked up.
    describe('without an ID', () => {
        const source: Record<string, string> = en;
        afterEach(() => {
            delete source['media.undefined'];
            delete source['media.undefined_other'];
        });

        it('is the fallback, not a message built from undefined', () => {
            source['media.undefined'] = 'Wrong';
            expect(label('media', undefined, 'Fallback')).toBe('Fallback');
            expect(label('media', '', 'Fallback')).toBe('Fallback');
        });

        it('has the fallback plural message', () => {
            source['media.undefined_other'] = 'Wrong';
            expect(pluralKey('media', undefined, 'download.items')).toBe('download.items');
            expect(pluralKey('media', '', 'download.items')).toBe('download.items');
        });
    });
});

describe('key tips', () => {
    // Translators see each tip as one sentence, with {settings} where the
    // Settings button goes.
    it.each([['en', en], ['es', es]] as const)('are whole %s sentences with one {settings}', (_lang, messages) => {
        const catalog: Record<string, string> = messages;
        const tips = Object.keys(catalog).filter((key) => key.startsWith('tip.')).sort();
        expect(tips).toEqual(['tip.bgg', 'tip.bhl', 'tip.igdb', 'tip.tmdb']);
        for (const key of tips) expect(catalog[key].split('{settings}'), key).toHaveLength(2);
    });
});

describe('onLangChange', () => {
    // What a test subscribed or spied on, released after it even when an
    // assertion fails, before the language is set back to English.
    const releases: Array<() => void> = [];
    afterEach(() => {
        for (const release of releases.splice(0).reverse()) release();
    });

    it('notifies on a change until unsubscribed', () => {
        const cb = vi.fn();
        const off = onLangChange(cb);
        releases.push(off);
        setLang('es');
        expect(cb).toHaveBeenCalledTimes(1);
        expect(cb).toHaveBeenCalledWith('es');
        setLang('es');
        expect(cb).toHaveBeenCalledTimes(1);
        off();
        setLang('en');
        expect(cb).toHaveBeenCalledTimes(1);
    });

    it('keeps notifying the other listeners when one throws', () => {
        const error = vi.spyOn(console, 'error').mockImplementation(() => {});
        releases.push(() => error.mockRestore());
        releases.push(onLangChange(() => {
            throw new Error('relabel failed');
        }));
        const cb = vi.fn();
        releases.push(onLangChange(cb));
        expect(() => setLang('es')).not.toThrow();
        expect(cb).toHaveBeenCalledWith('es');
        expect(error).toHaveBeenCalled();
    });
});

describe('applyDom', () => {
    it('fills text, title, placeholder and aria-label, and sets the html lang', () => {
        document.body.innerHTML = `
            <div id="root">
                <button id="b" data-i18n="settings.title">Settings</button>
                <input id="i" data-i18n-placeholder="print.headerExample" data-i18n-title="print.headerTitle"/>
                <select id="s" data-i18n-aria-label="app.mediaType"></select>
                <span id="u" data-i18n="no.such.key">Kept</span>
            </div>`;
        setLang('es');
        applyDom(document);
        const byId = (id: string) => document.getElementById(id)!;
        expect(byId('b').textContent).toBe('Ajustes');
        expect(byId('i').getAttribute('placeholder')).toBe('Libros de 2026');
        expect(byId('i').title).toBe(es['print.headerTitle']);
        expect(byId('s').getAttribute('aria-label')).toBe('Tipo de medio');
        expect(byId('u').textContent).toBe('Kept');
        expect(document.documentElement.lang).toBe('es');

        setLang('en');
        applyDom(document.getElementById('root')!);
        expect(byId('b').textContent).toBe('Settings');
        expect(document.documentElement.lang).toBe('en');
    });
});

describe('dictionaries', () => {
    it('have the same keys in English and Spanish', () => {
        expect(Object.keys(es).sort()).toEqual(Object.keys(en).sort());
    });

    it('have no empty messages', () => {
        for (const [key, value] of [...Object.entries(en), ...Object.entries(es)]) {
            expect(value.trim(), key).not.toBe('');
        }
    });

    it('use the same placeholders in both languages', () => {
        const names = (s: string) => (s.match(/\{\w+\}/g) ?? []).sort();
        for (const key of Object.keys(en) as Array<keyof typeof en>) {
            expect(names(es[key]), key).toEqual(names(en[key]));
        }
    });

    it('name the twelve months, in order, in both languages', () => {
        for (const messages of [en, es]) {
            const months = messages['pages.months'].split(',');
            expect(months).toHaveLength(12);
            for (const name of months) expect(name.trim()).toBe(name);
        }
    });

    it('give every plural an _other form', () => {
        for (const key of Object.keys(en)) {
            if (key.endsWith('_one')) expect(en, key).toHaveProperty([key.replace(/_one$/, '_other')]);
        }
    });
});
