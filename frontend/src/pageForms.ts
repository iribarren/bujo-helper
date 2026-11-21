// The option forms of the page kinds in the Make pages tab, by kind ID. A
// form renders the kind's own options (the shared Page and Lettering
// options are the tab's) and returns them as the object Go decodes into the
// kind's options struct (see internal/sheet.PageKinds). A new kind adds its
// form here; a kind without one is exported with its defaults.

import {titled, translated} from './fieldsets';
import {onLangChange, t, type MessageKey} from './i18n';
import {initRadioGroup, type RadioGroup} from './radioGroup';

export interface PageForm {
    // The kind's options, sent to Go as JSON.
    options: () => Record<string, unknown>;
}

// PageFormBuilder renders a kind's form into container, starting from the
// kind's defaults (from PageKinds); idPrefix prefixes its element IDs, and
// onChange is called when the user changes an option.
export type PageFormBuilder = (
    container: HTMLElement,
    idPrefix: string,
    defaults: Record<string, unknown>,
    onChange: () => void,
) => PageForm;

// The dot spacings a dot grid offers, in mm (Go: sheet.DotSpacings), and
// the one it starts with when Go's default is not one of them.
const DOT_SPACINGS = [4, 5, 6];
const DEFAULT_DOT_SPACING = 5;

// segmentedField appends a field of a caption and a segmented radio group,
// labelled by the caption, to container: the group's ID is
// "<idPrefix>-<name>", its caption and tooltip are message keys, and
// onChange is called when the user checks another radio.
function segmentedField(
    container: HTMLElement,
    idPrefix: string,
    name: string,
    caption: MessageKey,
    title: MessageKey,
    onChange: () => void,
): RadioGroup {
    const field = document.createElement('div');
    field.className = 'field';
    const label = translated(document.createElement('span'), caption);
    label.id = `${idPrefix}-${name}-label`;
    const box = titled(document.createElement('div'), title);
    box.id = `${idPrefix}-${name}`;
    box.className = 'segmented';
    box.setAttribute('role', 'radiogroup');
    box.setAttribute('aria-labelledby', label.id);
    field.append(label, box);
    container.append(field);
    return initRadioGroup(box, onChange);
}

// dotGridForm is the dot grid's form: Dot spacing, a segmented group.
const dotGridForm: PageFormBuilder = (container, idPrefix, defaults, onChange) => {
    const spacing = segmentedField(container, idPrefix, 'spacing', 'pages.dotSpacing', 'pages.dotSpacingTitle', onChange);
    spacing.replaceOptions(DOT_SPACINGS.map((mm) => ({value: String(mm), label: `${mm} mm`})));
    const preset = Number(defaults.spacing);
    spacing.value = String(DOT_SPACINGS.includes(preset) ? preset : DEFAULT_DOT_SPACING);
    return {
        options: () => ({spacing: Number(spacing.value)}),
    };
};

// The years a moon calendar accepts (Go: the moon options' validation).
const MIN_YEAR = 1900;
const MAX_YEAR = 2100;

// The moon calendar's layouts and hemispheres (Go: the moon options), by
// value, with their labels' message keys, and the ones a form starts with
// when Go's default is not one of them.
const MOON_LAYOUTS: Array<[value: string, label: MessageKey]> = [
    ['row', 'pages.layoutRow'],
    ['grid', 'pages.layoutGrid'],
    ['year', 'pages.layoutYear'],
];
const HEMISPHERES: Array<[value: string, label: MessageKey]> = [
    ['north', 'pages.north'],
    ['south', 'pages.south'],
];
const DEFAULT_LAYOUT = 'grid';
const DEFAULT_HEMISPHERE = 'north';

// choices are the radio options of a list of values and message keys, in
// the current language.
function choices(list: Array<[value: string, label: MessageKey]>) {
    return list.map(([value, key]) => ({value, label: t(key)}));
}

// offered is value when list offers it, fallback otherwise.
function offered(list: Array<[value: string, label: MessageKey]>, value: unknown, fallback: string): string {
    return list.some(([v]) => v === value) ? String(value) : fallback;
}

function clampYear(year: number): number {
    return Math.min(MAX_YEAR, Math.max(MIN_YEAR, year));
}

// validYear is the year text as a number when it is a whole year the moon
// calendar accepts, undefined otherwise (e.g. "20" while typing 2026).
function validYear(text: string): number | undefined {
    const year = Number(text);
    return text.trim() !== '' && Number.isInteger(year) && year >= MIN_YEAR && year <= MAX_YEAR ? year : undefined;
}

// moonForm is the moon calendar's form: Month and Year on one row, then
// Layout and Hemisphere, segmented groups. A default month or year of 0
// (or none) is the current one. Month is hidden for the Year layout, which
// prints the whole year, but still sent. The Year input sends a valid year
// as it is typed; on commit (change) an out-of-range year is clamped to
// 1900–2100, a fractional one rounded, and an empty one restored, so Go
// never gets a year it rejects. The month names and choices follow the UI
// language, keeping the selection.
const moonForm: PageFormBuilder = (container, idPrefix, defaults, onChange) => {
    const now = new Date();

    const pair = document.createElement('div');
    pair.className = 'field-pair';
    const monthField = document.createElement('label');
    monthField.className = 'field grow';
    monthField.id = `${idPrefix}-month-field`;
    const month = document.createElement('select');
    month.id = `${idPrefix}-month`;
    month.append(...Array.from({length: 12}, (_, i) => {
        const option = document.createElement('option');
        option.value = String(i + 1);
        return option;
    }));
    monthField.append(translated(document.createElement('span'), 'pages.month'), month);
    const yearField = document.createElement('label');
    yearField.className = 'field year-field';
    const year = titled(document.createElement('input'), 'pages.yearTitle');
    year.id = `${idPrefix}-year`;
    year.type = 'number';
    year.min = String(MIN_YEAR);
    year.max = String(MAX_YEAR);
    year.step = '1';
    year.inputMode = 'numeric';
    yearField.append(translated(document.createElement('span'), 'pages.year'), year);
    pair.append(monthField, yearField);
    container.append(pair);

    const layout = segmentedField(container, idPrefix, 'layout', 'pages.moonLayout', 'pages.moonLayoutTitle', () => {
        syncMonth();
        onChange();
    });
    const hemisphere = segmentedField(container, idPrefix, 'hemisphere', 'pages.hemisphere', 'pages.hemisphereTitle', onChange);

    function syncMonth() {
        monthField.hidden = layout.value === 'year';
    }

    function relabel() {
        const names = t('pages.months').split(',');
        Array.from(month.options).forEach((option, i) => {
            option.textContent = names[i] ?? option.value;
        });
        layout.replaceOptions(choices(MOON_LAYOUTS));
        hemisphere.replaceOptions(choices(HEMISPHERES));
    }
    relabel();
    onLangChange(relabel);

    const presetMonth = Number(defaults.month);
    month.value = String(Number.isInteger(presetMonth) && presetMonth >= 1 && presetMonth <= 12
        ? presetMonth
        : now.getMonth() + 1);
    const presetYear = Number(defaults.year);
    // The year Go gets: always valid, the input shows it after a commit.
    let current = clampYear(Number.isInteger(presetYear) && presetYear !== 0 ? presetYear : now.getFullYear());
    year.value = String(current);
    layout.value = offered(MOON_LAYOUTS, defaults.layout, DEFAULT_LAYOUT);
    hemisphere.value = offered(HEMISPHERES, defaults.hemisphere, DEFAULT_HEMISPHERE);
    syncMonth();

    // setYear sends year when it is not the one already sent.
    const setYear = (next: number) => {
        if (next === current) return;
        current = next;
        onChange();
    };
    month.addEventListener('change', onChange);
    year.addEventListener('input', () => {
        const typed = validYear(year.value);
        if (typed !== undefined) setYear(typed);
    });
    year.addEventListener('change', () => {
        const typed = Number(year.value);
        const next = year.value.trim() === '' || !Number.isFinite(typed) ? current : clampYear(Math.round(typed));
        year.value = String(next);
        setYear(next);
    });

    return {
        options: () => ({
            month: Number(month.value),
            year: current,
            layout: layout.value,
            hemisphere: hemisphere.value,
        }),
    };
};

export const pageForms: Record<string, PageFormBuilder> = {
    dotgrid: dotGridForm,
    moon: moonForm,
};
