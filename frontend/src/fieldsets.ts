// The Page and Lettering option groups that the Print covers and Make pages
// tabs share: Paper (a segmented group of the Go papers), Cut mode and, for
// Print covers, Credits page; and the tab's printed text (a header or a
// title) or, where the format or page kind prints none, a muted hint, and
// the Font picker and Colour-in checkbox (lettering.ts). Each
// tab renders them into its options column with its own ID prefix, e.g.
// "print-paper" and "pages-paper", and decides which fields apply.

import {main} from '../wailsjs/go/models';
import {label, t, type MessageKey} from './i18n';
import {renderLettering, type Lettering} from './lettering';
import {restoreSelect} from './prefs';
import {initRadioGroup, type RadioGroup} from './radioGroup';
import {chooseShown, visibleItems, type View} from './view';

// Preset is a Go preset with an ID, an English label and a default flag.
export interface Preset {
    id: string;
    label: string;
    default: boolean;
}

export interface SharedFieldsetsConfig {
    // The ID prefix: "<prefix>-group-page", "<prefix>-paper", …
    prefix: string;
    // The options column the two fieldsets are appended to.
    container: HTMLElement;
    // Tooltips of Paper and Cut mode.
    paperTitle: MessageKey;
    cutTitle: MessageKey;
    // The Credits page checkbox (checked at first), when the tab has one.
    credits?: {label: MessageKey; title: MessageKey};
    // The printed text field: its ID suffix, label and tooltip.
    text: {id: string; label: MessageKey; title: MessageKey};
    // The hint shown instead of the text field when it does not apply.
    hint: MessageKey;
    // Called when the user picks another paper.
    onPaperChange: () => void;
    // Called when the user picks another font or toggles Colour-in.
    onLetteringChange: () => void;
    // The sections and list items shown at first (see setView).
    view?: View;
}

// SharedPrefs are the remembered values of the shared fields; an unset or
// unlisted one keeps the default.
export interface SharedPrefs {
    paper?: string;
    cut?: string;
    credits?: boolean;
    text?: string;
    font?: string;
    colourIn?: boolean;
}

export interface SharedFieldsets {
    pageGroup: HTMLFieldSetElement;
    letteringGroup: HTMLFieldSetElement;
    paper: RadioGroup;
    cut: HTMLSelectElement;
    cutField: HTMLLabelElement;
    credits?: HTMLInputElement;
    text: HTMLInputElement;
    textField: HTMLLabelElement;
    hint: HTMLParagraphElement;
    // The Font picker and Colour-in; lettering.options() is what Go gets.
    lettering: Lettering;
    // Fills Paper from the Go papers the view shows, keeping the checked
    // paper when it is still shown, else checking the shown default.
    setPapers: (papers: main.PrintPaperInfo[]) => void;
    // Fills Cut mode from the Go cut modes the view shows, the shown
    // default selected.
    setCuts: (cuts: Preset[]) => void;
    // Shows only the papers and cut modes the view shows (Print covers and
    // Make pages share them), keeping the chosen ones that are still shown,
    // else the fallback ones (the remembered ones), else the shown default.
    // It calls no change callback.
    setView: (view: View, fallback: {paper?: string; cut?: string}) => void;
    // Shows the text field when it applies, the hint otherwise.
    showText: (on: boolean) => void;
    // Shows the Font picker and Colour-in when lettering applies.
    showLettering: (on: boolean) => void;
    // Shows remembered values; call it after the lists are set. It calls
    // no change callback.
    restore: (prefs: SharedPrefs) => void;
    // Names the papers and cut modes in the current language.
    relabel: () => void;
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, className = ''): HTMLElementTagNameMap[K] {
    const node = document.createElement(tag);
    if (className) node.className = className;
    return node;
}

// translated sets node's text to the message key, and marks it so applyDom
// translates it again on a language change.
export function translated<T extends HTMLElement>(node: T, key: MessageKey): T {
    node.dataset.i18n = key;
    node.textContent = t(key);
    return node;
}

// titled sets node's tooltip to the message key, like translated.
export function titled<T extends HTMLElement>(node: T, key: MessageKey): T {
    node.dataset.i18nTitle = key;
    node.title = t(key);
    return node;
}

// fillShown fills select with the presets of the list kind (a view list
// id, also their label kind) that view shows, named by text (by their IDs
// when unset), and selects the first of wanted that is shown, else the
// shown default (see chooseShown).
export function fillShown<T extends Preset>(
    select: HTMLSelectElement,
    kind: string,
    presets: readonly T[],
    view: View,
    wanted: ReadonlyArray<string | undefined>,
    text: (preset: T) => string = (preset) => label(kind, preset.id, preset.label),
) {
    select.replaceChildren(...visibleItems(view, kind, presets).map((preset) => {
        const option = el('option');
        option.textContent = text(preset);
        option.value = preset.id;
        return option;
    }));
    select.value = chooseShown(view, kind, presets, wanted);
}

// labelOptions names the options of a select by their preset IDs.
export function labelOptions(select: HTMLSelectElement, kind: string, presets: Array<{id: string; label: string}>) {
    for (const option of Array.from(select.options)) {
        const preset = presets.find((p) => p.id === option.value);
        if (preset) option.textContent = label(kind, preset.id, preset.label);
    }
}

// syncFieldsets hides each option group under root whose fields and hints
// are all hidden.
export function syncFieldsets(root: HTMLElement) {
    for (const group of Array.from(root.querySelectorAll<HTMLFieldSetElement>('fieldset'))) {
        const parts = Array.from(group.querySelectorAll<HTMLElement>('.field, .group-hint'));
        group.hidden = parts.length > 0 && parts.every((part) => part.hidden);
    }
}

// renderSharedFieldsets appends the Page and Lettering groups to the
// options column and wires Paper.
export function renderSharedFieldsets(config: SharedFieldsetsConfig): SharedFieldsets {
    const {prefix} = config;
    let view = config.view;
    // Every paper and cut mode from Go; the view picks those shown.
    let papers: main.PrintPaperInfo[] = [];
    let cuts: Preset[] = [];

    const pageGroup = el('fieldset', 'group');
    pageGroup.id = `${prefix}-group-page`;
    pageGroup.append(translated(el('legend'), 'print.groupPage'));

    // Paper: a segmented group, labelled by its caption.
    const paperField = el('div', 'field');
    const paperLabel = translated(el('span'), 'print.paper');
    paperLabel.id = `${prefix}-paper-label`;
    const paperBox = titled(el('div', 'segmented'), config.paperTitle);
    paperBox.id = `${prefix}-paper`;
    paperBox.setAttribute('role', 'radiogroup');
    paperBox.setAttribute('aria-labelledby', paperLabel.id);
    paperField.append(paperLabel, paperBox);
    const paper = initRadioGroup(paperBox, config.onPaperChange);

    const cutField = el('label', 'field');
    cutField.id = `${prefix}-cut-field`;
    const cut = titled(el('select'), config.cutTitle);
    cut.id = `${prefix}-cut`;
    cutField.append(translated(el('span'), 'print.cut'), cut);
    pageGroup.append(paperField, cutField);

    let credits: HTMLInputElement | undefined;
    if (config.credits) {
        const field = el('label', 'field check-field');
        credits = titled(el('input'), config.credits.title);
        credits.id = `${prefix}-credits`;
        credits.type = 'checkbox';
        credits.checked = true;
        field.append(credits, translated(el('span'), config.credits.label));
        pageGroup.append(field);
    }

    const letteringGroup = el('fieldset', 'group');
    letteringGroup.id = `${prefix}-group-lettering`;
    letteringGroup.append(translated(el('legend'), 'print.groupLettering'));
    const textField = el('label', 'field');
    textField.id = `${prefix}-${config.text.id}-field`;
    const text = titled(el('input'), config.text.title);
    text.id = `${prefix}-${config.text.id}`;
    text.type = 'text';
    text.maxLength = 120;
    text.autocomplete = 'off';
    textField.append(translated(el('span'), config.text.label), text);
    const hint = translated(el('p', 'group-hint muted'), config.hint);
    hint.id = `${prefix}-lettering-hint`;
    const lettering = renderLettering(prefix, config.onLetteringChange);
    letteringGroup.append(textField, hint, lettering.fontField, lettering.colourInField);

    config.container.append(pageGroup, letteringGroup);

    // showPapers shows the papers the view shows, named in the current
    // language, and checks the first of wanted that is shown, else the
    // shown default.
    function showPapers(wanted: ReadonlyArray<string | undefined>) {
        paper.replaceOptions(visibleItems(view, 'paper', papers).map((p) => {
            const name = label('paper', p.id, p.label);
            return {value: p.id, label: name, title: `${name} · ${p.width} × ${p.height} mm`};
        }));
        paper.value = chooseShown(view, 'paper', papers, wanted);
    }

    function showText(on: boolean) {
        text.disabled = !on;
        textField.hidden = !on;
        hint.hidden = on;
    }
    showText(false);

    return {
        pageGroup,
        letteringGroup,
        paper,
        cut,
        cutField,
        credits,
        text,
        textField,
        hint,
        lettering,
        setPapers: (list) => {
            papers = list;
            showPapers([paper.value]);
        },
        setCuts: (list) => {
            cuts = list;
            fillShown(cut, 'cut', cuts, view, []);
        },
        setView: (next, fallback) => {
            view = next;
            showPapers([paper.value, fallback.paper]);
            fillShown(cut, 'cut', cuts, view, [cut.value, fallback.cut]);
        },
        showText,
        showLettering: lettering.show,
        restore: (prefs) => {
            if (prefs.paper && visibleItems(view, 'paper', papers).some((p) => p.id === prefs.paper)) paper.value = prefs.paper;
            restoreSelect(cut, prefs.cut);
            if (credits && prefs.credits !== undefined) credits.checked = prefs.credits;
            if (prefs.text) text.value = prefs.text;
            lettering.restore(prefs.font, prefs.colourIn ?? false);
        },
        relabel: () => {
            showPapers([paper.value]);
            labelOptions(cut, 'cut', cuts);
            lettering.relabel();
        },
    };
}
