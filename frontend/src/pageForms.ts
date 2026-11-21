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
    // dispose releases what the form subscribed to (e.g. language changes),
    // for a form that is discarded. The tab keeps its forms for its
    // lifetime; tests that build forms dispose them.
    dispose?: () => void;
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

// partialYear reports whether a committed year is still being typed: fewer
// than four digits (e.g. "20" while typing 2026), or not positive.
function partialYear(year: number): boolean {
    return year < 1000;
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
// as it is typed; on commit (change) a fractional year is rounded, an
// out-of-range whole year clamped to 1900–2100, and an empty or partial one
// (fewer than four digits) restored to the year sent, so Go never gets a
// year it rejects. The month names and choices follow the UI
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
    const stopRelabel = onLangChange(relabel);

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
        const typed = Math.round(Number(year.value));
        const next = year.value.trim() === '' || !Number.isFinite(typed) || partialYear(typed) ? current : clampYear(typed);
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
        dispose: stopRelabel,
    };
};

// The visited map's regions and fill colours (Go: sheet.MapRegions and
// sheet.MapColours), by value, with their labels' message keys and, for the
// colours, the swatch Go fills with; and the ones a form starts with when
// Go's default is not one of them. internal/sheet/mapcontract_test.go reads
// these two arrays and fails when they drift from Go.
const MAP_REGIONS: Array<[value: string, label: MessageKey]> = [
    ['world', 'pages.regionWorld'],
    ['africa', 'pages.regionAfrica'],
    ['asia', 'pages.regionAsia'],
    ['europe', 'pages.regionEurope'],
    ['north-america', 'pages.regionNorthAmerica'],
    ['south-america', 'pages.regionSouthAmerica'],
    ['oceania', 'pages.regionOceania'],
];
const MAP_COLOURS: Array<[value: string, label: MessageKey, swatch: string]> = [
    ['teal', 'pages.colourTeal', '#2a9d8f'],
    ['coral', 'pages.colourCoral', '#ee8466'],
    ['sage', 'pages.colourSage', '#8fae7e'],
    ['lavender', 'pages.colourLavender', '#a393c9'],
];
const DEFAULT_REGION = 'world';
const DEFAULT_COLOUR = 'teal';

// countryList is the countries typed in text: split on commas, semicolons
// and new lines, trimmed, blanks dropped. Go matches each name or code.
export function countryList(text: string): string[] {
    return text.split(/[,;\n\r]/).map((s) => s.trim()).filter((s) => s !== '');
}

// mapForm is the visited map's form: Region (a select), Visited countries
// (a textarea of names or ISO codes, sent as a list as it is typed, the
// text left as typed), Colour (segmented swatches) and List countries (a
// checkbox). The labels follow the UI language, keeping the options.
const mapForm: PageFormBuilder = (container, idPrefix, defaults, onChange) => {
    const regionField = document.createElement('label');
    regionField.className = 'field';
    const region = titled(document.createElement('select'), 'pages.mapRegionTitle');
    region.id = `${idPrefix}-region`;
    region.append(...MAP_REGIONS.map(([value]) => {
        const option = document.createElement('option');
        option.value = value;
        return option;
    }));
    regionField.append(translated(document.createElement('span'), 'pages.mapRegion'), region);

    const countriesField = document.createElement('div');
    countriesField.className = 'field map-countries-field';
    const countriesLabel = document.createElement('label');
    countriesLabel.htmlFor = `${idPrefix}-countries`;
    countriesLabel.append(translated(document.createElement('span'), 'pages.mapCountries'));
    const countries = document.createElement('textarea');
    countries.id = `${idPrefix}-countries`;
    countries.rows = 4;
    countries.spellcheck = false;
    countries.dataset.i18nPlaceholder = 'pages.mapCountriesPlaceholder';
    countries.placeholder = t('pages.mapCountriesPlaceholder');
    const countriesHint = translated(document.createElement('p'), 'pages.mapCountriesHint');
    countriesHint.className = 'field-hint muted';
    countriesHint.id = `${idPrefix}-countries-hint`;
    countries.setAttribute('aria-describedby', countriesHint.id);
    countriesField.append(countriesLabel, countries, countriesHint);
    container.append(regionField, countriesField);

    const colour = segmentedField(container, idPrefix, 'colour', 'pages.mapColour', 'pages.mapColourTitle', onChange);

    const listField = document.createElement('div');
    listField.className = 'field check-hint-field';
    const listRow = document.createElement('label');
    listRow.className = 'check-row';
    const list = document.createElement('input');
    list.type = 'checkbox';
    list.id = `${idPrefix}-list`;
    listRow.append(list, translated(document.createElement('span'), 'pages.mapList'));
    const listHint = translated(document.createElement('p'), 'pages.mapListHint');
    listHint.className = 'field-hint muted';
    listHint.id = `${idPrefix}-list-hint`;
    list.setAttribute('aria-describedby', listHint.id);
    listField.append(listRow, listHint);
    container.append(listField);

    function relabel() {
        Array.from(region.options).forEach((option, i) => {
            option.textContent = t(MAP_REGIONS[i][1]);
        });
        colour.replaceOptions(MAP_COLOURS.map(([value, key, swatch]) => ({value, label: t(key), swatch})));
    }
    relabel();
    const stopRelabel = onLangChange(relabel);

    region.value = offered(MAP_REGIONS, defaults.region, DEFAULT_REGION);
    colour.value = MAP_COLOURS.some(([v]) => v === defaults.colour) ? String(defaults.colour) : DEFAULT_COLOUR;
    list.checked = typeof defaults.list === 'boolean' ? defaults.list : true;
    // The countries Go gets, kept apart from the text being typed.
    let sent = Array.isArray(defaults.countries)
        ? defaults.countries.filter((c): c is string => typeof c === 'string' && c.trim() !== '').map((c) => c.trim())
        : [];
    countries.value = sent.join(', ');

    region.addEventListener('change', onChange);
    list.addEventListener('change', onChange);
    countries.addEventListener('input', () => {
        const next = countryList(countries.value);
        if (next.length === sent.length && next.every((c, i) => c === sent[i])) return;
        sent = next;
        onChange();
    });

    return {
        options: () => ({region: region.value, countries: [...sent], colour: colour.value, list: list.checked}),
        dispose: stopRelabel,
    };
};

export const pageForms: Record<string, PageFormBuilder> = {
    dotgrid: dotGridForm,
    moon: moonForm,
    map: mapForm,
};
