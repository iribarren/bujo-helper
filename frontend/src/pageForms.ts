// The option forms of the page kinds in the Make pages tab, by kind ID. A
// form renders the kind's own options (the shared Page and Lettering
// options are the tab's) and returns them as the object Go decodes into the
// kind's options struct (see internal/sheet.PageKinds). A new kind adds its
// form here; a kind without one is exported with its defaults.

import {titled, translated} from './fieldsets';
import {onLangChange, t, type MessageKey} from './i18n';
import {monthYearField} from './monthYearField';
import {quoteLines, quoteSearch, type Quote} from './quotesForm';
import {initRadioGroup, type RadioGroup} from './radioGroup';
import {recipeEditor, recipeList} from './recipesForm';

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

// moonForm is the moon calendar's form: Month and Year on one row (see
// monthYearField), then Layout and Hemisphere, segmented groups. Month is
// hidden for the Year layout, which prints the whole year, but still sent.
// The choices follow the UI language, keeping the selection.
const moonForm: PageFormBuilder = (container, idPrefix, defaults, onChange) => {
    const date = monthYearField(container, idPrefix, defaults, onChange);

    const layout = segmentedField(container, idPrefix, 'layout', 'pages.moonLayout', 'pages.moonLayoutTitle', () => {
        syncMonth();
        onChange();
    });
    const hemisphere = segmentedField(container, idPrefix, 'hemisphere', 'pages.hemisphere', 'pages.hemisphereTitle', onChange);

    function syncMonth() {
        date.monthField.hidden = layout.value === 'year';
    }

    function relabel() {
        layout.replaceOptions(choices(MOON_LAYOUTS));
        hemisphere.replaceOptions(choices(HEMISPHERES));
    }
    relabel();
    const stopRelabel = onLangChange(relabel);

    layout.value = offered(MOON_LAYOUTS, defaults.layout, DEFAULT_LAYOUT);
    hemisphere.value = offered(HEMISPHERES, defaults.hemisphere, DEFAULT_HEMISPHERE);
    syncMonth();

    return {
        options: () => ({
            month: date.month(),
            year: date.year(),
            layout: layout.value,
            hemisphere: hemisphere.value,
        }),
        dispose: () => {
            stopRelabel();
            date.dispose();
        },
    };
};

// The month calendar's week starts (Go: the calendar options), by value,
// with their labels' message keys, and the one a form starts with when
// Go's default is not one of them.
const WEEK_STARTS: Array<[value: string, label: MessageKey]> = [
    ['mon', 'pages.weekMonday'],
    ['sun', 'pages.weekSunday'],
];
const DEFAULT_WEEK_START = 'mon';

// calendarForm is the month calendar's form: Month and Year on one row
// (see monthYearField), then Week starts on, a segmented group. It always
// sends an explicit month and year. The choices follow the UI language,
// keeping the selection.
const calendarForm: PageFormBuilder = (container, idPrefix, defaults, onChange) => {
    const date = monthYearField(container, idPrefix, defaults, onChange);
    const weekStart = segmentedField(container, idPrefix, 'weekStart', 'pages.weekStart', 'pages.weekStartTitle', onChange);

    const relabel = () => weekStart.replaceOptions(choices(WEEK_STARTS));
    relabel();
    const stopRelabel = onLangChange(relabel);
    weekStart.value = offered(WEEK_STARTS, defaults.weekStart, DEFAULT_WEEK_START);

    return {
        options: () => ({month: date.month(), year: date.year(), weekStart: weekStart.value}),
        dispose: () => {
            stopRelabel();
            date.dispose();
        },
    };
};

// habitList is the habits typed in text, one per line: trimmed, blank
// lines dropped.
export function habitList(text: string): string[] {
    return text.split(/\r\n|\r|\n/).map((s) => s.trim()).filter((s) => s !== '');
}

// habitsForm is the habit tracker's form: Month and Year on one row (see
// monthYearField), then Habits, a textarea of one habit per line, sent as a
// list as it is typed, the text left as typed. Go accepts at most 40
// habits of at most 60 characters: a muted hint says so, and the form
// sends what was typed, so a habit list over the limits shows Go's error
// in the preview rather than a page silently missing habits.
const habitsForm: PageFormBuilder = (container, idPrefix, defaults, onChange) => {
    const date = monthYearField(container, idPrefix, defaults, onChange);

    const field = document.createElement('div');
    field.className = 'field habits-field';
    const label = document.createElement('label');
    label.htmlFor = `${idPrefix}-habits`;
    label.append(translated(document.createElement('span'), 'pages.habits'));
    const habits = document.createElement('textarea');
    habits.id = `${idPrefix}-habits`;
    habits.rows = 5;
    const hint = translated(document.createElement('p'), 'pages.habitsHint');
    hint.className = 'field-hint muted';
    hint.id = `${idPrefix}-habits-hint`;
    habits.setAttribute('aria-describedby', hint.id);
    field.append(label, habits, hint);
    container.append(field);

    // The habits Go gets, kept apart from the text being typed.
    let sent = Array.isArray(defaults.habits)
        ? defaults.habits.filter((h): h is string => typeof h === 'string' && h.trim() !== '').map((h) => h.trim())
        : [];
    habits.value = sent.join('\n');
    habits.addEventListener('input', () => {
        const next = habitList(habits.value);
        if (next.length === sent.length && next.every((h, i) => h === sent[i])) return;
        sent = next;
        onChange();
    });

    return {
        options: () => ({month: date.month(), year: date.year(), habits: [...sent]}),
        dispose: date.dispose,
    };
};

// The quote cards' styles and cards per page (Go: the quotes options), by
// value, with the styles' labels' message keys, and the ones a form starts
// with when Go's default is not one of them.
const QUOTE_STYLES: Array<[value: string, label: MessageKey]> = [
    ['classic', 'pages.styleClassic'],
    ['bold', 'pages.styleBold'],
    ['framed', 'pages.styleFramed'],
];
const CARDS_PER_PAGE = [4, 6, 8];
const DEFAULT_QUOTE_STYLE = 'classic';
const DEFAULT_CARDS_PER_PAGE = 6;

// quoteList is the well-formed quotes of a kind's defaults.
function quoteList(value: unknown): Quote[] {
    if (!Array.isArray(value)) return [];
    return value.filter((q): q is Quote => typeof q?.text === 'string' && typeof q?.author === 'string')
        .map((q) => ({text: q.text, author: q.author, source: q.source === 'wikiquote' ? 'wikiquote' : ''}));
}

// quotesForm is the quote cards' form: the Wikiquote search with its
// results and picked quotes (see quoteSearch), Your own quotes (a textarea
// of "Quote, Author" lines, see quoteLines), then Style and Cards per page,
// segmented groups. It sends the picked quotes, in pick order, followed by
// the typed ones, as typed: Go enforces the limits, and its error shows in
// the preview. The labels follow the UI language, keeping the options.
const quotesForm: PageFormBuilder = (container, idPrefix, defaults, onChange) => {
    const initial = quoteList(defaults.quotes);
    const search = quoteSearch(container, idPrefix, initial.filter((q) => q.source === 'wikiquote'), onChange);

    const field = document.createElement('div');
    field.className = 'field quotes-own-field';
    const label = document.createElement('label');
    label.htmlFor = `${idPrefix}-own`;
    label.append(translated(document.createElement('span'), 'pages.quotesOwn'));
    const own = document.createElement('textarea');
    own.id = `${idPrefix}-own`;
    own.rows = 4;
    const hint = translated(document.createElement('p'), 'pages.quotesOwnHint');
    hint.className = 'field-hint muted';
    hint.id = `${idPrefix}-own-hint`;
    own.setAttribute('aria-describedby', hint.id);
    field.append(label, own, hint);
    container.append(field);

    // The typed quotes Go gets, kept apart from the text being typed.
    let typed = initial.filter((q) => q.source === '');
    own.value = typed.map((q) => (q.author ? `${q.text}, ${q.author}` : q.text)).join('\n');
    own.addEventListener('input', () => {
        const next = quoteLines(own.value);
        if (next.length === typed.length && next.every((q, i) => q.text === typed[i].text && q.author === typed[i].author)) return;
        typed = next;
        onChange();
    });

    const style = segmentedField(container, idPrefix, 'style', 'pages.quotesStyle', 'pages.quotesStyleTitle', onChange);
    const perPage = segmentedField(container, idPrefix, 'perPage', 'pages.quotesPerPage', 'pages.quotesPerPageTitle', onChange);
    perPage.replaceOptions(CARDS_PER_PAGE.map((n) => ({value: String(n), label: String(n)})));

    const relabel = () => style.replaceOptions(choices(QUOTE_STYLES));
    relabel();
    const stopRelabel = onLangChange(relabel);
    style.value = offered(QUOTE_STYLES, defaults.style, DEFAULT_QUOTE_STYLE);
    const preset = Number(defaults.perPage);
    perPage.value = String(CARDS_PER_PAGE.includes(preset) ? preset : DEFAULT_CARDS_PER_PAGE);

    return {
        options: () => ({
            quotes: [...search.picked(), ...typed.map((q) => ({...q}))],
            style: style.value,
            perPage: Number(perPage.value),
        }),
        dispose: () => {
            stopRelabel();
            search.dispose();
        },
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

// The recipe card sizes (Go: sheet.RecipeSizes), by value, with their
// labels' message keys, and the one a form starts with when Go's default is
// not one of them.
const RECIPE_SIZES: Array<[value: string, label: MessageKey]> = [
    ['a6', 'pages.recipesSizeA6'],
    ['4x6', 'pages.recipesSize4x6'],
    ['page', 'pages.recipesSizePage'],
];
const DEFAULT_RECIPE_SIZE = 'a6';

// recipesForm is the recipe cards' form: the recipe list and the selected
// recipe's editor (see recipeEditor), then Card size, a segmented group. It
// sends the recipes as typed, their lists as non-empty trimmed lines: Go
// normalises them and enforces the limits, and its error shows in the
// preview. No recipes prints blank cards. The labels follow the UI
// language, keeping the options.
const recipesForm: PageFormBuilder = (container, idPrefix, defaults, onChange) => {
    const editor = recipeEditor(container, idPrefix, recipeList(defaults.recipes), onChange);
    const size = segmentedField(container, idPrefix, 'size', 'pages.recipesSize', 'pages.recipesSizeTitle', onChange);

    const relabel = () => size.replaceOptions(choices(RECIPE_SIZES));
    relabel();
    const stopRelabel = onLangChange(relabel);
    size.value = offered(RECIPE_SIZES, defaults.size, DEFAULT_RECIPE_SIZE);

    return {
        options: () => ({recipes: editor.recipes(), size: size.value}),
        dispose: () => {
            stopRelabel();
            editor.dispose();
        },
    };
};

// conversionsForm is the kitchen conversions' form: no options, only a
// muted line saying what the chart holds. It sends the kind's defaults.
const conversionsForm: PageFormBuilder = (container, idPrefix, defaults) => {
    const hint = translated(document.createElement('p'), 'pages.conversionsHint');
    hint.className = 'field-hint muted';
    hint.id = `${idPrefix}-hint`;
    container.append(hint);
    return {options: () => ({...defaults})};
};

export const pageForms: Record<string, PageFormBuilder> = {
    dotgrid: dotGridForm,
    moon: moonForm,
    calendar: calendarForm,
    habits: habitsForm,
    quotes: quotesForm,
    map: mapForm,
    recipes: recipesForm,
    conversions: conversionsForm,
};
