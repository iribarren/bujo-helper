// The Lettering controls the Print covers and Make pages tabs share (see
// fieldsets.ts): a font picker and the Colour-in checkbox.
//
// The font picker is a button that shows the chosen font's name in that
// font and opens a listbox of every bundled font, each in its own face with
// its style group and whether it can print as outlines. A native <select>
// cannot show custom fonts in WebKitGTK. The fonts come from Go's
// LetteringFonts and are served by its asset handler (/fonts/<id>.ttf), so
// the bundle never holds them: letteringFonts adds one @font-face per font.
//
// Keyboard: the button opens the list with a click, Enter, Space or the
// arrow keys; focus then moves to the listbox, which tracks the active
// option with aria-activedescendant. Up/Down/Home/End move, Enter or Space
// picks and closes, Escape closes and returns focus to the button, Tab
// closes and moves on. A click outside closes it.
//
// Colour-in prints outlines instead of fills. Its hint follows the font's
// hollow capability: "stroke" fonts print as outlines, "none" fonts are too
// thin and stay filled, "native" fonts are hollow by design. For a native
// font the box shows checked, but the user's own choice is kept and sent
// (Go never strokes those fonts), and comes back with the next font.

import {LetteringFonts} from '../wailsjs/go/main/App';
import {main} from '../wailsjs/go/models';
import {label, t, type MessageKey} from './i18n';

// The hollow capabilities of a font, as Go sends them.
type Hollow = 'stroke' | 'none' | 'native';

function hollowOf(font: main.LetteringFontInfo | undefined): Hollow {
    const hollow = font?.hollow;
    return hollow === 'none' || hollow === 'native' ? hollow : 'stroke';
}

// Font IDs are lower-case letters, digits and hyphens (see
// internal/sheet/lettering.go), safe in a CSS family name.
const FONT_ID = /^[a-z0-9-]+$/;

// fontFamily is the CSS font-family of a lettering font: its @font-face,
// then the UI font while it loads or when it is missing.
export function fontFamily(id: string): string {
    return `"lettering-${id}", system-ui, sans-serif`;
}

const FACES_ID = 'lettering-font-faces';

// addFontFaces adds one @font-face per font, from its served URL, in one
// <style> element; a second call changes nothing.
function addFontFaces(fonts: main.LetteringFontInfo[]) {
    if (document.getElementById(FACES_ID)) return;
    const style = document.createElement('style');
    style.id = FACES_ID;
    style.textContent = fonts
        .filter((f) => FONT_ID.test(f.id) && f.url)
        .map((f) => `@font-face {\n    font-family: "lettering-${f.id}";\n    src: url(${JSON.stringify(f.url)}) format("truetype");\n    font-display: swap;\n}`)
        .join('\n');
    document.head.append(style);
}

let fontsPromise: Promise<main.LetteringFontInfo[]> | undefined;

// letteringFonts is the bundled fonts from LetteringFonts, asked once for
// both tabs; the first answer adds their @font-face rules. A failure is
// forgotten, so the next call asks again.
export function letteringFonts(): Promise<main.LetteringFontInfo[]> {
    if (!fontsPromise) {
        fontsPromise = LetteringFonts().then((list) => {
            const fonts = list ?? [];
            addFontFaces(fonts);
            return fonts;
        });
        fontsPromise.catch(() => {
            fontsPromise = undefined;
        });
    }
    return fontsPromise;
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, className = ''): HTMLElementTagNameMap[K] {
    const node = document.createElement(tag);
    if (className) node.className = className;
    return node;
}

// fontLabel is a font's name in the current language: only the default
// font's ("Default") is translated.
function fontLabel(font: main.LetteringFontInfo): string {
    return label('font', font.id, font.label);
}

function groupLabel(group: string): string {
    return label('fontGroup', group, group);
}

export interface FontPicker {
    // The picker: the button and its listbox.
    root: HTMLDivElement;
    button: HTMLButtonElement;
    list: HTMLUListElement;
    // The chosen font ID; "" until the fonts load. Setting it reports no
    // change.
    value: string;
    // The chosen font, undefined until the fonts load.
    current: () => main.LetteringFontInfo | undefined;
    // Lists the fonts; the first time, the default font is chosen.
    setFonts: (fonts: main.LetteringFontInfo[]) => void;
    close: () => void;
    // Names the fonts, groups and badges in the current language.
    relabel: () => void;
}

// initFontPicker builds a font picker labelled by labelId. onShow is called
// whenever it shows a font, chosen by the user or not; onPick when the user
// picks another font.
function initFontPicker(prefix: string, labelId: string, onShow: () => void, onPick: () => void): FontPicker {
    let fonts: main.LetteringFontInfo[] = [];
    let value = '';
    let active = -1;

    const root = el('div', 'font-picker');
    const button = el('button', 'font-picker-button');
    button.type = 'button';
    button.id = `${prefix}-font`;
    button.disabled = true;
    button.setAttribute('aria-haspopup', 'listbox');
    button.setAttribute('aria-expanded', 'false');
    const name = el('span', 'font-picker-name');
    name.id = `${prefix}-font-name`;
    button.setAttribute('aria-labelledby', `${labelId} ${name.id}`);
    const chevron = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    chevron.setAttribute('class', 'font-picker-chevron');
    chevron.setAttribute('viewBox', '0 0 12 12');
    chevron.setAttribute('aria-hidden', 'true');
    const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    path.setAttribute('d', 'M2.5 4.5 6 8l3.5-3.5');
    chevron.append(path);
    button.append(name, chevron);

    const list = el('ul', 'font-list');
    list.id = `${prefix}-font-list`;
    list.setAttribute('role', 'listbox');
    list.tabIndex = -1;
    list.hidden = true;
    button.setAttribute('aria-controls', list.id);
    root.append(button, list);

    function options(): HTMLLIElement[] {
        return Array.from(list.querySelectorAll<HTMLLIElement>('[role="option"]'));
    }

    function current() {
        return fonts.find((f) => f.id === value);
    }

    function showValue() {
        const font = current();
        name.textContent = font ? fontLabel(font) : '';
        name.style.fontFamily = font ? fontFamily(font.id) : '';
        for (const option of options()) {
            option.setAttribute('aria-selected', String(option.dataset.value === value));
        }
        onShow();
    }

    function setActive(index: number) {
        const all = options();
        if (all.length === 0) return;
        active = Math.max(0, Math.min(all.length - 1, index));
        all.forEach((option, i) => option.classList.toggle('active', i === active));
        list.setAttribute('aria-activedescendant', all[active].id);
        all[active].scrollIntoView?.({block: 'nearest'});
    }

    function isOpen() {
        return !list.hidden;
    }

    function open() {
        if (isOpen() || fonts.length === 0) return;
        list.hidden = false;
        button.setAttribute('aria-expanded', 'true');
        setActive(Math.max(0, fonts.findIndex((f) => f.id === value)));
        list.focus();
        document.addEventListener('mousedown', onOutside, true);
    }

    function close(focusButton = false) {
        if (isOpen()) {
            list.hidden = true;
            button.setAttribute('aria-expanded', 'false');
            list.removeAttribute('aria-activedescendant');
            document.removeEventListener('mousedown', onOutside, true);
        }
        if (focusButton) button.focus();
    }

    function pick(id: string) {
        const changed = id !== value;
        value = id;
        showValue();
        close(true);
        if (changed) onPick();
    }

    function onOutside(event: MouseEvent) {
        if (!root.contains(event.target as Node)) close();
    }

    function renderOptions() {
        list.replaceChildren(...fonts.map((font) => {
            const option = el('li', 'font-option');
            option.id = `${prefix}-font-option-${font.id}`;
            option.dataset.value = font.id;
            option.setAttribute('role', 'option');
            const fontName = el('span', 'font-option-name');
            fontName.style.fontFamily = fontFamily(font.id);
            const group = el('span', 'font-option-group');
            const badge = el('span', 'font-badge');
            badge.dataset.hollow = hollowOf(font);
            option.append(fontName, group, badge);
            return option;
        }));
        labelOptions();
        showValue();
    }

    function labelOptions() {
        list.setAttribute('aria-label', t('lettering.font'));
        for (const option of options()) {
            const font = fonts.find((f) => f.id === option.dataset.value);
            if (!font) continue;
            option.querySelector('.font-option-name')!.textContent = fontLabel(font);
            option.querySelector('.font-option-group')!.textContent = groupLabel(font.group);
            option.querySelector('.font-badge')!.textContent = t(`lettering.badge.${hollowOf(font)}` as MessageKey);
        }
    }

    button.addEventListener('click', () => {
        if (isOpen()) close();
        else open();
    });
    button.addEventListener('keydown', (event) => {
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            event.preventDefault();
            open();
        }
    });
    list.addEventListener('keydown', (event) => {
        switch (event.key) {
        case 'ArrowDown':
            setActive(active + 1);
            break;
        case 'ArrowUp':
            setActive(active - 1);
            break;
        case 'Home':
            setActive(0);
            break;
        case 'End':
            setActive(options().length - 1);
            break;
        case 'Enter':
        case ' ': {
            const option = options()[active];
            if (option?.dataset.value) pick(option.dataset.value);
            break;
        }
        case 'Escape':
            close(true);
            break;
        case 'Tab':
            // Focus moves on as usual.
            close();
            return;
        default:
            return;
        }
        event.preventDefault();
    });
    list.addEventListener('click', (event) => {
        const option = (event.target as Element).closest<HTMLElement>('[role="option"]');
        if (option?.dataset.value) pick(option.dataset.value);
    });
    // Keep focus in the list while an option is clicked.
    list.addEventListener('mousedown', (event) => event.preventDefault());
    // Close when focus leaves the picker some other way.
    list.addEventListener('focusout', (event) => {
        if (!root.contains(event.relatedTarget as Node | null)) close();
    });

    return {
        root,
        button,
        list,
        get value() {
            return value;
        },
        set value(id: string) {
            value = id;
            showValue();
        },
        current,
        setFonts: (list) => {
            const first = fonts.length === 0;
            fonts = list;
            if (!fonts.some((f) => f.id === value)) value = '';
            if (first || value === '') value = fonts.find((f) => f.default)?.id ?? fonts[0]?.id ?? '';
            button.disabled = fonts.length === 0;
            renderOptions();
        },
        close: () => close(),
        relabel: () => {
            labelOptions();
            showValue();
        },
    };
}

export interface Lettering {
    // The Font field (its label and the picker) and the Colour-in field
    // (the checkbox and its hint); the caller places them.
    fontField: HTMLDivElement;
    colourInField: HTMLDivElement;
    picker: FontPicker;
    colourIn: HTMLInputElement;
    hint: HTMLParagraphElement;
    // Lists the fonts from letteringFonts.
    setFonts: (fonts: main.LetteringFontInfo[]) => void;
    // Shows the two fields where lettering applies, hides them otherwise.
    show: (on: boolean) => void;
    // What Go gets: the chosen font ID and the user's Colour-in choice.
    options: () => {font: string; colourIn: boolean};
    // Shows a remembered font, when it is listed, and Colour-in choice;
    // call it after setFonts. It does not call onChange.
    restore: (font: string | undefined, colourIn: boolean) => void;
    relabel: () => void;
}

// renderLettering builds the Font and Colour-in fields with IDs
// "<prefix>-font", "<prefix>-colour-in", …; onChange is called when the
// user picks a font or toggles Colour-in.
export function renderLettering(prefix: string, onChange: () => void): Lettering {
    // The user's Colour-in choice; a native font shows checked over it.
    let colourIn = false;

    const fontField = el('div', 'field');
    fontField.id = `${prefix}-font-field`;
    const fontLabelNode = el('span');
    fontLabelNode.id = `${prefix}-font-label`;
    fontLabelNode.dataset.i18n = 'lettering.font';
    fontLabelNode.textContent = t('lettering.font');
    const picker = initFontPicker(prefix, fontLabelNode.id, () => sync(), onChange);
    picker.button.dataset.i18nTitle = 'lettering.fontTitle';
    picker.button.title = t('lettering.fontTitle');
    fontField.append(fontLabelNode, picker.root);

    const colourInField = el('div', 'field colour-in-field');
    colourInField.id = `${prefix}-colour-in-field`;
    const row = el('label', 'check-row');
    row.dataset.i18nTitle = 'lettering.colourInTitle';
    row.title = t('lettering.colourInTitle');
    const box = el('input');
    box.type = 'checkbox';
    box.id = `${prefix}-colour-in`;
    const text = el('span');
    text.dataset.i18n = 'lettering.colourIn';
    text.textContent = t('lettering.colourIn');
    row.append(box, text);
    const hint = el('p', 'field-hint muted');
    hint.id = `${prefix}-colour-in-hint`;
    box.setAttribute('aria-describedby', hint.id);
    colourInField.append(row, hint);

    function hintKey(hollow: Hollow): MessageKey {
        if (hollow === 'native') return 'lettering.hint.native';
        if (hollow === 'none') return 'lettering.hint.none';
        return colourIn ? 'lettering.hint.strokeOn' : 'lettering.hint.stroke';
    }

    // sync shows the Colour-in box and hint for the chosen font.
    function sync() {
        const hollow = hollowOf(picker.current());
        box.checked = colourIn || hollow === 'native';
        const key = hintKey(hollow);
        hint.dataset.i18n = key;
        hint.textContent = t(key);
    }

    box.addEventListener('change', () => {
        // A hollow font prints outlines whatever the choice: keep the
        // choice and show the box checked again.
        const native = hollowOf(picker.current()) === 'native';
        if (!native) colourIn = box.checked;
        sync();
        if (!native) onChange();
    });

    sync();

    return {
        fontField,
        colourInField,
        picker,
        colourIn: box,
        hint,
        setFonts: picker.setFonts,
        show: (on) => {
            fontField.hidden = !on;
            colourInField.hidden = !on;
            if (!on) picker.close();
        },
        options: () => ({font: picker.value, colourIn}),
        restore: (font, on) => {
            colourIn = on;
            const before = picker.value;
            if (font) picker.value = font;
            // An unlisted font keeps the one shown.
            if (!picker.current()) picker.value = before;
            sync();
        },
        relabel: picker.relabel,
    };
}
