// UI translation: the current language, message lookup with {name}
// interpolation and plurals, and filling static markup from data-i18n*
// attributes. Hand-rolled, with no runtime dependency.

import {en, type MessageKey} from './en';
import {es} from './es';

export type {MessageKey, Messages} from './en';

export type Lang = 'en' | 'es';

// Params are the values of a message's {name} placeholders.
export type Params = Record<string, string | number>;

// PluralKey is the base of a plural message pair, "<key>_one"/"<key>_other".
export type PluralKey = {[K in MessageKey]: K extends `${infer B}_one` ? B : never}[MessageKey];

type Catalog = Partial<Record<string, string>>;

const catalogs: Record<Lang, Catalog> = {en, es};

let current: Lang = 'en';
const listeners = new Set<(lang: Lang) => void>();

// detectLang resolves the language setting: "en" or "es" wins; anything
// else ("" is System) follows the navigator language, Spanish for any "es"
// locale and English otherwise.
export function detectLang(setting: string, navigatorLang: string): Lang {
    const chosen = setting.trim().toLowerCase();
    if (chosen === 'en' || chosen === 'es') return chosen;
    return /^es(?:[-_]|$)/i.test(navigatorLang.trim()) ? 'es' : 'en';
}

export function getLang(): Lang {
    return current;
}

// setLang switches the language and, when it changed, notifies the
// onLangChange listeners in the order they subscribed. A listener that
// throws is logged and skipped, so the rest of the UI still relabels.
export function setLang(lang: Lang): void {
    if (lang === current) return;
    current = lang;
    for (const cb of [...listeners]) {
        try {
            cb(lang);
        } catch (err) {
            console.error('language change listener failed:', err);
        }
    }
}

// onLangChange calls cb after every language change; the returned function
// unsubscribes it.
export function onLangChange(cb: (lang: Lang) => void): () => void {
    listeners.add(cb);
    return () => listeners.delete(cb);
}

// setCatalog replaces the messages of a language (for tests).
export function setCatalog(lang: Lang, messages: Catalog): void {
    catalogs[lang] = messages;
}

// has reports whether key is a message (in English, the source dictionary).
export function has(key: string): key is MessageKey {
    return Object.prototype.hasOwnProperty.call(en, key);
}

function lookup(key: string): string {
    return catalogs[current][key] ?? catalogs.en[key] ?? key;
}

function interpolate(text: string, params?: Params): string {
    if (!params) return text;
    return text.replace(/\{(\w+)\}/g, (match, name: string) =>
        Object.prototype.hasOwnProperty.call(params, name) ? String(params[name]) : match);
}

// t is the message key in the current language, falling back to English and
// then to the key itself, with its {name} placeholders filled from params.
export function t(key: MessageKey, params?: Params): string {
    return interpolate(lookup(key), params);
}

// tn is the plural message key for n: "<key>_one" or "<key>_other", by the
// plural rules of the current language, with {n} and params filled in.
// Categories other than one (e.g. "many") use "_other".
export function tn(key: PluralKey, n: number, params?: Params): string {
    const category = new Intl.PluralRules(current).select(n);
    const exact = `${key}_${category}`;
    const text = category !== 'other' && has(exact) ? lookup(exact) : lookup(`${key}_other`);
    return interpolate(text, {...params, n});
}

// label is the translated label of a Go preset by its ID, the message
// "<kind>.<id>", or fallback (Go's English label) when there is none or the
// ID is missing or empty.
export function label(kind: string, id: string | undefined, fallback: string): string {
    if (!id) return fallback;
    const key = `${kind}.${id}`;
    return has(key) ? t(key) : fallback;
}

// pluralKey is the plural message "<kind>.<id>" when it exists, else
// fallback (also for a missing or empty ID).
export function pluralKey(kind: string, id: string | undefined, fallback: PluralKey): PluralKey {
    if (!id) return fallback;
    const key = `${kind}.${id}`;
    return has(`${key}_other`) ? (key as PluralKey) : fallback;
}

// The attributes applyDom fills, and what each sets.
const DOM_ATTRIBUTES: Array<[attribute: string, apply: (el: Element, text: string) => void]> = [
    ['data-i18n', (el, text) => { el.textContent = text; }],
    ['data-i18n-title', (el, text) => el.setAttribute('title', text)],
    ['data-i18n-placeholder', (el, text) => el.setAttribute('placeholder', text)],
    ['data-i18n-aria-label', (el, text) => el.setAttribute('aria-label', text)],
];

// applyDom fills the elements under root from their data-i18n (text),
// data-i18n-title, data-i18n-placeholder and data-i18n-aria-label message
// keys, leaving an element as it is when its key is unknown, and sets the
// document language.
export function applyDom(root: ParentNode): void {
    for (const [attribute, apply] of DOM_ATTRIBUTES) {
        for (const el of Array.from(root.querySelectorAll(`[${attribute}]`))) {
            const key = el.getAttribute(attribute) ?? '';
            if (has(key)) apply(el, t(key));
        }
    }
    document.documentElement.lang = current;
}
