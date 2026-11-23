import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';

import {afterEach, beforeAll, beforeEach, describe, expect, it, vi} from 'vitest';

import {ExportPage, OpenFile, PreviewPage} from '../wailsjs/go/main/App';
import {useLanguage} from './language';
import {pageForms} from './pageForms';
import {initPages, parseDefaults} from './pages';
import {fakePdf, renders, resetFakePdfs} from './previewMock';

vi.mock('pdfjs-dist/legacy/build/pdf.mjs', async () => (await import('./previewMock')).pdfjsMock);
vi.mock('pdfjs-dist/legacy/build/pdf.worker.min.mjs?url', () => ({default: '/pdf.worker.min.mjs'}));

// The page kinds as PageKinds sends them (see app.go), plus a made-up kind
// that uses the cut mode and prints no title, to cover the shared Page and
// Lettering fieldsets for every kind.
const {kinds, papers, cuts, fonts} = vi.hoisted(() => ({
    kinds: [
        {id: 'dotgrid', label: 'Dot grid', usesCut: false, usesTitle: true, defaults: '{"spacing":5}', default: true},
        {
            id: 'moon', label: 'Moon calendar', usesCut: false, usesTitle: true,
            defaults: '{"month":0,"year":0,"layout":"grid","hemisphere":"north"}', default: false,
        },
        {
            id: 'calendar', label: 'Month calendar', usesCut: false, usesTitle: true,
            defaults: '{"month":0,"year":0,"weekStart":"mon"}', default: false,
        },
        {
            id: 'habits', label: 'Habit tracker', usesCut: false, usesTitle: true,
            defaults: '{"month":0,"year":0,"habits":[]}', default: false,
        },
        {
            id: 'quotes', label: 'Quote cards', usesCut: false, usesTitle: true,
            defaults: '{"quotes":[],"style":"classic","perPage":6}', default: false,
        },
        {
            id: 'map', label: 'Visited map', usesCut: false, usesTitle: true,
            defaults: '{"region":"world","countries":[],"colour":"teal","list":true}', default: false,
        },
        {id: 'recipes', label: 'Recipe cards', usesCut: false, usesTitle: false, defaults: '{"recipes":[],"size":"a6"}', default: false},
        {id: 'conversions', label: 'Kitchen conversions', usesCut: false, usesTitle: true, defaults: '{}', default: false},
        {id: 'cutout', label: 'Cut-out', usesCut: true, usesTitle: false, defaults: '{}', default: false},
    ],
    papers: [
        {id: 'a4', label: 'A4', width: 210, height: 297, default: false},
        {id: 'letter', label: 'US Letter', width: 215.9, height: 279.4, default: false},
        {id: 'a5', label: 'A5', width: 148, height: 210, default: true},
        {id: 'b5', label: 'B5', width: 176, height: 250, default: false},
    ],
    cuts: [
        {id: 'border', label: 'Border', default: true},
        {id: 'bleed', label: 'Bleed + crop marks', default: false},
        {id: 'none', label: 'None', default: false},
    ],
    fonts: [
        ['go', 'Default', 'sans', 'stroke'],
        ['pacifico', 'Pacifico', 'brush', 'stroke'],
        ['londrina-outline', 'Londrina Outline', 'hollow', 'native'],
    ].map(([id, label, group, hollow]) => ({
        id, label, group, hollow, licence: 'OFL-1.1', url: `/fonts/${id}.ttf`, licenceUrl: `/fonts/${id}/licence`, default: id === 'go',
    })),
}));

vi.mock('../wailsjs/go/main/App', async (importOriginal) => {
    const original = await importOriginal<Record<string, unknown>>();
    const stubs: Record<string, unknown> = {};
    for (const name of Object.keys(original)) {
        stubs[name] = vi.fn(async () => undefined);
    }
    stubs.PageKinds = vi.fn(async () => kinds);
    stubs.PrintPapers = vi.fn(async () => papers);
    stubs.PrintCutModes = vi.fn(async () => cuts);
    stubs.PagePages = vi.fn(async () => 1);
    stubs.PreviewPage = vi.fn(async () => ({url: '/preview?id=1', pages: 1}));
    stubs.LetteringFonts = vi.fn(async () => fonts);
    return stubs;
});

const deps = {
    defaultDir: () => '/covers',
    showError: vi.fn(),
    hideError: vi.fn(),
    // No debounce: the preview tests cover it.
    previewDelay: 0,
};

function el<T extends HTMLElement>(id: string): T {
    const node = document.getElementById(id);
    if (!node) throw new Error(`missing #${id}`);
    return node as T;
}

// radio is the radio of value in the radio group #groupId.
function radio(groupId: string, value: string): HTMLButtonElement {
    const node = el(groupId).querySelector<HTMLButtonElement>(`[role="radio"][data-value="${value}"]`);
    if (!node) throw new Error(`missing radio ${value} in #${groupId}`);
    return node;
}

function checked(groupId: string): string {
    return el(groupId).querySelector<HTMLElement>('[role="radio"][aria-checked="true"]')?.dataset.value ?? '';
}

function lastPreviewPage(): [string, {paper: string; cut: string; title: string; font: string; colourIn: boolean}, string] {
    const calls = vi.mocked(PreviewPage).mock.calls;
    expect(calls.length).toBeGreaterThan(0);
    return calls[calls.length - 1] as never;
}

// previewDone waits until the preview has drawn the latest options.
async function previewDone() {
    await vi.waitFor(() => expect(el('pages-sheet').getAttribute('aria-busy')).toBe('false'));
}

// pickFont chooses the font id in the Lettering font picker.
function pickFont(id: string) {
    el('pages-font').click();
    const option = el('pages-font-list').querySelector<HTMLElement>(`[role="option"][data-value="${id}"]`);
    if (!option) throw new Error(`missing font ${id}`);
    option.click();
}

function setColourIn(on: boolean) {
    const box = el<HTMLInputElement>('pages-colour-in');
    if (box.checked !== on) box.click();
}

function setTitle(text: string) {
    const input = el<HTMLInputElement>('pages-title');
    input.value = text;
    input.dispatchEvent(new Event('input'));
}

beforeAll(async () => {
    resetFakePdfs();
    fakePdf('/preview?id=1', 1);
    const html = readFileSync(resolve(__dirname, '../index.html'), 'utf8');
    document.body.innerHTML = html.slice(html.indexOf('<body>') + '<body>'.length, html.indexOf('</body>'));
    initPages(deps);
    await vi.waitFor(() => expect(el('pages-kinds').querySelectorAll('[role="radio"]').length).toBe(kinds.length));
    await vi.waitFor(() => expect(el('pages-paper').querySelectorAll('[role="radio"]').length).toBe(papers.length));
    await vi.waitFor(() => expect(el('pages-font-list').querySelectorAll('[role="option"]').length).toBe(fonts.length));
});

afterEach(() => {
    radio('pages-kinds', 'dotgrid').click();
    radio('pages-paper', 'a5').click();
    setTitle('');
    pickFont('go');
    setColourIn(false);
});

describe('page kinds', () => {
    it('are listed from PageKinds in a radio group, the default checked', () => {
        const group = el('pages-kinds');
        expect(group.getAttribute('role')).toBe('radiogroup');
        expect(Array.from(group.querySelectorAll('[role="radio"]'), (r) => r.textContent)).toEqual(['Dot grid', 'Moon calendar', 'Month calendar', 'Habit tracker', 'Quote cards', 'Visited map', 'Recipe cards', 'Kitchen conversions', 'Cut-out']);
        expect(checked('pages-kinds')).toBe('dotgrid');
        expect(el('pages-kind-title').textContent).toBe('Dot grid');
    });

    it('switch the form with a click or the arrow keys', () => {
        radio('pages-kinds', 'cutout').click();
        expect(el('pages-kind-title').textContent).toBe('Cut-out');
        expect(el('pages-form-dotgrid').hidden).toBe(true);
        radio('pages-kinds', 'cutout').dispatchEvent(new KeyboardEvent('keydown', {key: 'ArrowDown', bubbles: true, cancelable: true}));
        expect(checked('pages-kinds')).toBe('dotgrid');
        expect(el('pages-kind-title').textContent).toBe('Dot grid');
        expect(el('pages-form-dotgrid').hidden).toBe(false);
    });

    it('send the chosen kind and its defaults when it has no form', async () => {
        radio('pages-kinds', 'cutout').click();
        await vi.waitFor(() => expect(lastPreviewPage()[0]).toBe('cutout'));
        expect(lastPreviewPage()[2]).toBe('{}');
    });

    it('follow the language', async () => {
        await useLanguage('es');
        try {
            expect(radio('pages-kinds', 'dotgrid').textContent).toBe('Cuadrícula de puntos');
            expect(el('pages-kind-title').textContent).toBe('Cuadrícula de puntos');
            expect(radio('pages-kinds', 'moon').textContent).toBe('Calendario lunar');
            expect(radio('pages-kinds', 'calendar').textContent).toBe('Calendario mensual');
            expect(radio('pages-kinds', 'habits').textContent).toBe('Registro de hábitos');
            expect(radio('pages-kinds', 'quotes').textContent).toBe('Tarjetas de citas');
            expect(radio('pages-kinds', 'map').textContent).toBe('Mapa de viajes');
            expect(radio('pages-kinds', 'recipes').textContent).toBe('Fichas de recetas');
            expect(radio('pages-kinds', 'conversions').textContent).toBe('Conversiones de cocina');
            // A kind without a translation keeps the Go label.
            expect(radio('pages-kinds', 'cutout').textContent).toBe('Cut-out');
        } finally {
            await useLanguage('en');
        }
    });
});

describe('dot grid form', () => {
    it('has a Dot spacing segmented group of 4, 5 and 6 mm, 5 by default', () => {
        const group = el('pages-dotgrid-spacing');
        expect(group.getAttribute('role')).toBe('radiogroup');
        expect(group.classList.contains('segmented')).toBe(true);
        expect(Array.from(group.querySelectorAll('[role="radio"]'), (r) => r.textContent)).toEqual(['4 mm', '5 mm', '6 mm']);
        expect(checked('pages-dotgrid-spacing')).toBe('5');
    });

    it('sends the chosen spacing as the options JSON', async () => {
        radio('pages-dotgrid-spacing', '6').click();
        await vi.waitFor(() => expect(lastPreviewPage()[2]).toBe('{"spacing":6}'));
        expect(lastPreviewPage()[0]).toBe('dotgrid');
        radio('pages-dotgrid-spacing', '5').click();
        await vi.waitFor(() => expect(lastPreviewPage()[2]).toBe('{"spacing":5}'));
    });

    // spacingOf builds a dot grid form from defaults, as the tab does, and
    // returns the checked spacing and the spacing it sends.
    function spacingOf(defaults: Record<string, unknown>): {checked: string; sent: unknown} {
        const box = document.createElement('div');
        const form = pageForms.dotgrid(box, 'test-dotgrid', defaults, () => {});
        return {
            checked: box.querySelector<HTMLElement>('[role="radio"][aria-checked="true"]')?.dataset.value ?? '',
            sent: form.options().spacing,
        };
    }

    const fallbacks: Array<[name: string, defaults: Record<string, unknown>]> = [
        ['a default spacing it does not offer', {spacing: 7}],
        ['a default spacing that is not a number', {spacing: 'wide'}],
        ['no default spacing', {}],
        ['malformed defaults JSON', parseDefaults('{"spacing":')],
    ];
    for (const [name, defaults] of fallbacks) {
        it(`checks and sends 5 mm for ${name}`, () => {
            expect(spacingOf(defaults)).toEqual({checked: '5', sent: 5});
        });
    }

    it('checks and sends an offered default spacing', () => {
        expect(spacingOf({spacing: 4})).toEqual({checked: '4', sent: 4});
    });
});

describe('moon calendar form', () => {
    // The moon options a form starts with under the test clock.
    const defaults = {month: 3, year: 2024, layout: 'grid', hemisphere: 'north'};

    // setMoon sets the moon form's options as the user does.
    function setMoon(options: {month: number; year: number; layout: string; hemisphere: string}) {
        const month = el<HTMLSelectElement>('pages-moon-month');
        month.value = String(options.month);
        month.dispatchEvent(new Event('change'));
        const year = el<HTMLInputElement>('pages-moon-year');
        year.value = String(options.year);
        year.dispatchEvent(new Event('change'));
        radio('pages-moon-layout', options.layout).click();
        radio('pages-moon-hemisphere', options.hemisphere).click();
    }

    // Every test shows the moon kind under a clock in March 2024: the tab
    // builds the form on first use, in whichever test runs first, and each
    // test leaves the form at its defaults again.
    beforeEach(() => {
        vi.useFakeTimers({toFake: ['Date']});
        vi.setSystemTime(new Date(2024, 2, 15, 12));
        radio('pages-kinds', 'moon').click();
    });

    afterEach(() => {
        setMoon(defaults);
        vi.useRealTimers();
    });

    it('previews the current month and year in a grid by default', async () => {
        expect(el('pages-form-moon').hidden).toBe(false);
        expect(el('pages-form-dotgrid').hidden).toBe(true);
        expect(el<HTMLSelectElement>('pages-moon-month').value).toBe('3');
        expect(el<HTMLInputElement>('pages-moon-year').value).toBe('2024');
        await vi.waitFor(() => expect(JSON.parse(lastPreviewPage()[2])).toEqual(defaults));
        expect(lastPreviewPage()[0]).toBe('moon');
        // The moon prints the shared title.
        expect(el('pages-title-field').hidden).toBe(false);
    });

    it('previews and exports the chosen options', async () => {
        const month = el<HTMLSelectElement>('pages-moon-month');
        month.value = '7';
        month.dispatchEvent(new Event('change'));
        const year = el<HTMLInputElement>('pages-moon-year');
        year.value = '2030';
        year.dispatchEvent(new Event('input'));
        radio('pages-moon-layout', 'year').click();
        radio('pages-moon-hemisphere', 'south').click();
        const want = {month: 7, year: 2030, layout: 'year', hemisphere: 'south'};
        await vi.waitFor(() => expect(JSON.parse(lastPreviewPage()[2])).toEqual(want));
        expect(el('pages-moon-month-field').hidden).toBe(true);

        vi.mocked(ExportPage).mockResolvedValueOnce('/out/moon_A5.pdf');
        await previewDone();
        el('pages-export').click();
        await vi.waitFor(() => expect(el('pages-status').textContent).toBe('Saved moon_A5.pdf: 1 page'));
        const [kind, , , options] = vi.mocked(ExportPage).mock.calls.at(-1)!;
        expect(kind).toBe('moon');
        expect(JSON.parse(options)).toEqual(want);
    });

    it('keeps its options while another kind is shown', async () => {
        const chosen = {month: 11, year: 2001, layout: 'row', hemisphere: 'south'};
        setMoon(chosen);
        await vi.waitFor(() => expect(JSON.parse(lastPreviewPage()[2])).toEqual(chosen));
        radio('pages-kinds', 'dotgrid').click();
        await vi.waitFor(() => expect(lastPreviewPage()[0]).toBe('dotgrid'));
        radio('pages-kinds', 'moon').click();
        await vi.waitFor(() => expect(lastPreviewPage()[0]).toBe('moon'));
        expect(JSON.parse(lastPreviewPage()[2])).toEqual(chosen);
        expect(el<HTMLInputElement>('pages-moon-year').value).toBe('2001');
    });
});

describe('month calendar form', () => {
    // The tab builds the form on first use, under a clock in March 2024;
    // each test leaves it at these options again.
    const defaults = {month: 3, year: 2024, weekStart: 'mon'};

    function setCalendar(options: {month: number; year: number; weekStart: string}) {
        const month = el<HTMLSelectElement>('pages-calendar-month');
        month.value = String(options.month);
        month.dispatchEvent(new Event('change'));
        const year = el<HTMLInputElement>('pages-calendar-year');
        year.value = String(options.year);
        year.dispatchEvent(new Event('change'));
        radio('pages-calendar-weekStart', options.weekStart).click();
    }

    beforeEach(() => {
        vi.useFakeTimers({toFake: ['Date']});
        vi.setSystemTime(new Date(2024, 2, 15, 12));
        radio('pages-kinds', 'calendar').click();
    });

    afterEach(() => {
        setCalendar(defaults);
        vi.useRealTimers();
    });

    it('previews the current month and year from Monday by default', async () => {
        expect(el('pages-kind-title').textContent).toBe('Month calendar');
        expect(el('pages-form-calendar').hidden).toBe(false);
        expect(checked('pages-calendar-weekStart')).toBe('mon');
        await vi.waitFor(() => expect(lastPreviewPage()[0]).toBe('calendar'));
        expect(JSON.parse(lastPreviewPage()[2])).toEqual(defaults);
        expect(el('pages-title-field').hidden).toBe(false);
    });

    it('previews and exports the chosen options', async () => {
        const want = {month: 9, year: 2024, weekStart: 'sun'};
        setCalendar(want);
        await vi.waitFor(() => expect(JSON.parse(lastPreviewPage()[2])).toEqual(want));
        expect(lastPreviewPage()[0]).toBe('calendar');

        vi.mocked(ExportPage).mockResolvedValueOnce('/out/calendar_A5.pdf');
        await previewDone();
        el('pages-export').click();
        await vi.waitFor(() => expect(el('pages-status').textContent).toBe('Saved calendar_A5.pdf: 1 page'));
        const [kind, , , options] = vi.mocked(ExportPage).mock.calls.at(-1)!;
        expect(kind).toBe('calendar');
        expect(JSON.parse(options)).toEqual(want);
    });
});

describe('habit tracker form', () => {
    // The tab builds the form on first use, under a clock in March 2024;
    // each test leaves it with no habits again.
    beforeEach(() => {
        vi.useFakeTimers({toFake: ['Date']});
        vi.setSystemTime(new Date(2024, 2, 15, 12));
        radio('pages-kinds', 'habits').click();
    });

    function setHabits(text: string) {
        const habits = el<HTMLTextAreaElement>('pages-habits-habits');
        habits.value = text;
        habits.dispatchEvent(new Event('input'));
    }

    afterEach(() => {
        setHabits('');
        vi.useRealTimers();
    });

    it('previews the current month with no habits by default', async () => {
        expect(el('pages-kind-title').textContent).toBe('Habit tracker');
        expect(el('pages-form-habits').hidden).toBe(false);
        expect(el('pages-form-calendar').hidden).toBe(true);
        await vi.waitFor(() => expect(lastPreviewPage()[0]).toBe('habits'));
        expect(JSON.parse(lastPreviewPage()[2])).toEqual({month: 3, year: 2024, habits: []});
        expect(el('pages-title-field').hidden).toBe(false);
    });

    it('previews the typed habits, one per line', async () => {
        setHabits('Read\n\n  Walk  \nWater');
        await vi.waitFor(() => expect(JSON.parse(lastPreviewPage()[2]))
            .toEqual({month: 3, year: 2024, habits: ['Read', 'Walk', 'Water']}));
        expect(lastPreviewPage()[0]).toBe('habits');
    });
});

describe('quote cards form', () => {
    beforeEach(() => {
        radio('pages-kinds', 'quotes').click();
    });

    afterEach(() => {
        const own = el<HTMLTextAreaElement>('pages-quotes-own');
        own.value = '';
        own.dispatchEvent(new Event('input'));
        radio('pages-quotes-style', 'classic').click();
        radio('pages-quotes-perPage', '6').click();
    });

    it('previews no quotes, classic, six per page by default', async () => {
        expect(el('pages-kind-title').textContent).toBe('Quote cards');
        expect(el('pages-form-quotes').hidden).toBe(false);
        expect(el('pages-form-habits').hidden).toBe(true);
        await vi.waitFor(() => expect(lastPreviewPage()[0]).toBe('quotes'));
        expect(JSON.parse(lastPreviewPage()[2])).toEqual({quotes: [], style: 'classic', perPage: 6});
        // The cards print the shared title; they have their own cut lines.
        expect(el('pages-title-field').hidden).toBe(false);
    });

    it('previews the typed quotes and the chosen layout', async () => {
        const own = el<HTMLTextAreaElement>('pages-quotes-own');
        own.value = 'Less is more, Mies';
        own.dispatchEvent(new Event('input'));
        radio('pages-quotes-style', 'framed').click();
        radio('pages-quotes-perPage', '4').click();
        await vi.waitFor(() => expect(JSON.parse(lastPreviewPage()[2]))
            .toEqual({quotes: [{text: 'Less is more', author: 'Mies', source: ''}], style: 'framed', perPage: 4}));
        expect(lastPreviewPage()[0]).toBe('quotes');
    });
});

describe('recipe cards form', () => {
    beforeEach(() => {
        radio('pages-kinds', 'recipes').click();
    });

    it('previews blank A6 cards by default, without the title field', async () => {
        expect(el('pages-kind-title').textContent).toBe('Recipe cards');
        expect(el('pages-form-recipes').hidden).toBe(false);
        await vi.waitFor(() => expect(lastPreviewPage()[0]).toBe('recipes'));
        expect(JSON.parse(lastPreviewPage()[2])).toEqual({recipes: [], size: 'a6'});
        // Each card prints its recipe's title; cards have their own cut lines.
        expect(el('pages-title-field').hidden).toBe(true);
        expect(el('pages-cut-field').hidden).toBe(true);
    });

    it('previews an added recipe and the chosen size', async () => {
        el<HTMLButtonElement>('pages-recipes-add').click();
        const title = el<HTMLInputElement>('pages-recipes-title');
        title.value = 'Toast';
        title.dispatchEvent(new Event('input'));
        radio('pages-recipes-size', 'page').click();
        await vi.waitFor(() => expect(JSON.parse(lastPreviewPage()[2])).toEqual({
            recipes: [{title: 'Toast', servings: '', time: '', ingredients: [], steps: [], photo: ''}],
            size: 'page',
        }));
        el('pages-recipes-list').querySelector<HTMLButtonElement>('.icon-button')!.click();
        radio('pages-recipes-size', 'a6').click();
        await vi.waitFor(() => expect(JSON.parse(lastPreviewPage()[2])).toEqual({recipes: [], size: 'a6'}));
    });
});

describe('kitchen conversions form', () => {
    it('says what the chart holds and previews it with the shared title', async () => {
        radio('pages-kinds', 'conversions').click();
        expect(el('pages-kind-title').textContent).toBe('Kitchen conversions');
        expect(el('pages-conversions-hint').textContent)
            .toBe('A ready-made chart of volume, weight, oven temperatures and cups to grams.');
        expect(el('pages-title-field').hidden).toBe(false);
        await vi.waitFor(() => expect(lastPreviewPage()[0]).toBe('conversions'));
        expect(lastPreviewPage()[2]).toBe('{}');
    });
});

describe('visited map form', () => {
    const defaults = {region: 'world', countries: [], colour: 'teal', list: true};

    // setMap sets the map form's options as the user does.
    function setMap(options: {region: string; countries: string; colour: string; list: boolean}) {
        const region = el<HTMLSelectElement>('pages-map-region');
        region.value = options.region;
        region.dispatchEvent(new Event('change'));
        const countries = el<HTMLTextAreaElement>('pages-map-countries');
        countries.value = options.countries;
        countries.dispatchEvent(new Event('input'));
        radio('pages-map-colour', options.colour).click();
        const list = el<HTMLInputElement>('pages-map-list');
        if (list.checked !== options.list) list.click();
    }

    // The tab builds the form on first use, in whichever test runs first;
    // each test leaves it at its defaults again.
    beforeEach(() => {
        radio('pages-kinds', 'map').click();
    });

    afterEach(() => {
        setMap({region: 'world', countries: '', colour: 'teal', list: true});
    });

    it('previews the world with no countries by default', async () => {
        expect(el('pages-kind-title').textContent).toBe('Visited map');
        expect(el('pages-form-map').hidden).toBe(false);
        expect(el('pages-form-moon').hidden).toBe(true);
        await vi.waitFor(() => expect(lastPreviewPage()[0]).toBe('map'));
        expect(JSON.parse(lastPreviewPage()[2])).toEqual(defaults);
        // The map prints the shared title.
        expect(el('pages-title-field').hidden).toBe(false);
    });

    it('previews the chosen options', async () => {
        setMap({region: 'europe', countries: 'Spain, Portugal\nFR', colour: 'coral', list: false});
        await vi.waitFor(() => expect(JSON.parse(lastPreviewPage()[2]))
            .toEqual({region: 'europe', countries: ['Spain', 'Portugal', 'FR'], colour: 'coral', list: false}));
        expect(lastPreviewPage()[0]).toBe('map');
    });

    it('keeps its options while another kind is shown', async () => {
        setMap({region: 'oceania', countries: 'Australia; NZ', colour: 'lavender', list: false});
        const chosen = {region: 'oceania', countries: ['Australia', 'NZ'], colour: 'lavender', list: false};
        await vi.waitFor(() => expect(JSON.parse(lastPreviewPage()[2])).toEqual(chosen));
        radio('pages-kinds', 'dotgrid').click();
        await vi.waitFor(() => expect(lastPreviewPage()[0]).toBe('dotgrid'));
        radio('pages-kinds', 'map').click();
        await vi.waitFor(() => expect(lastPreviewPage()[0]).toBe('map'));
        expect(JSON.parse(lastPreviewPage()[2])).toEqual(chosen);
        expect(el<HTMLTextAreaElement>('pages-map-countries').value).toBe('Australia; NZ');
    });
});

describe('Page fieldset', () => {
    it('has a Paper segmented group from PrintPapers, the default checked', () => {
        const group = el('pages-paper');
        expect(group.classList.contains('segmented')).toBe(true);
        expect(Array.from(group.querySelectorAll('[role="radio"]'), (r) => r.textContent)).toEqual(['A4', 'US Letter', 'A5', 'B5']);
        expect(checked('pages-paper')).toBe('a5');
        expect(el('pages-paper').closest('fieldset')?.id).toBe('pages-group-page');
    });

    it('sends the chosen paper', async () => {
        radio('pages-paper', 'b5').click();
        await vi.waitFor(() => expect(lastPreviewPage()[1].paper).toBe('b5'));
    });

    it('shows "Loading preview…" over the preview well until the page is drawn', async () => {
        const indicator = document.querySelector<HTMLElement>('.pages-preview .page-well .preview-loading');
        expect(indicator).not.toBeNull();
        radio('pages-paper', 'b5').click();
        expect(indicator?.dataset.active).toBe('true');
        expect(indicator?.textContent).toBe('Loading preview…');
        await previewDone();
        expect(indicator?.dataset.active).toBe('false');
        expect(indicator?.textContent).toBe('');
    });

    it('shapes the preview page like the paper', () => {
        radio('pages-paper', 'a4').click();
        expect(Number(el('pages-sheet').style.getPropertyValue('--page-ratio'))).toBeCloseTo(210 / 297, 5);
        radio('pages-paper', 'b5').click();
        expect(Number(el('pages-sheet').style.getPropertyValue('--page-ratio'))).toBeCloseTo(176 / 250, 5);
    });

    it('hides the cut mode for a kind that does not use it, and sends none', async () => {
        expect(el('pages-cut-field').hidden).toBe(true);
        radio('pages-paper', 'a4').click();
        await vi.waitFor(() => expect(lastPreviewPage()[1]).toEqual(expect.objectContaining({paper: 'a4', cut: ''})));
    });

    it('shows the cut mode for a kind that uses it, and sends it', async () => {
        radio('pages-kinds', 'cutout').click();
        expect(el('pages-cut-field').hidden).toBe(false);
        const cut = el<HTMLSelectElement>('pages-cut');
        cut.value = 'bleed';
        cut.dispatchEvent(new Event('change'));
        await vi.waitFor(() => expect(lastPreviewPage()[1].cut).toBe('bleed'));
    });

    it('has no credits page option', () => {
        expect(el('pages-group-page').querySelector('input[type="checkbox"]')).toBeNull();
        expect(document.getElementById('pages-credits')).toBeNull();
    });
});

describe('Lettering fieldset', () => {
    it('has the title for a kind that prints one', async () => {
        expect(el('pages-title').closest('fieldset')?.id).toBe('pages-group-lettering');
        expect(el('pages-title-field').hidden).toBe(false);
        expect(el('pages-lettering-hint').hidden).toBe(true);
        expect(el('pages-title-field').textContent).toContain('Title (optional)');
        setTitle('  Ideas  ');
        await vi.waitFor(() => expect(lastPreviewPage()[1].title).toBe('Ideas'));
    });

    it('shows a muted hint instead for a kind that prints none, and sends no title', async () => {
        setTitle('Ideas');
        radio('pages-kinds', 'cutout').click();
        expect(el('pages-group-lettering').hidden).toBe(false);
        expect(el('pages-title-field').hidden).toBe(true);
        expect(el('pages-lettering-hint').hidden).toBe(false);
        expect(el('pages-lettering-hint').textContent).toBe('This page prints no title.');
        await vi.waitFor(() => expect(lastPreviewPage()[0]).toBe('cutout'));
        expect(lastPreviewPage()[1].title).toBe('');
    });

    for (const kind of ['dotgrid', 'cutout']) {
        it(`${kind}: has the font picker and Colour-in, the default font chosen`, () => {
            radio('pages-kinds', kind).click();
            expect(el('pages-font').closest('fieldset')?.id).toBe('pages-group-lettering');
            expect(el('pages-font-field').hidden).toBe(false);
            expect(el('pages-colour-in-field').hidden).toBe(false);
            expect(el('pages-font').querySelector('.font-picker-name')?.textContent).toBe('Default');
            expect(el<HTMLInputElement>('pages-colour-in').checked).toBe(false);
        });
    }

    it('sends the chosen font and Colour-in', async () => {
        pickFont('pacifico');
        setColourIn(true);
        await vi.waitFor(() => expect(lastPreviewPage()[1]).toEqual(expect.objectContaining({font: 'pacifico', colourIn: true})));
    });

    it('sends the stored Colour-in with a hollow font', async () => {
        pickFont('londrina-outline');
        expect(el<HTMLInputElement>('pages-colour-in').checked).toBe(true);
        expect(el('pages-colour-in-hint').textContent).toBe('This font is hollow by design, so letters always print as outlines.');
        await vi.waitFor(() => expect(lastPreviewPage()[1]).toEqual(expect.objectContaining({font: 'londrina-outline', colourIn: false})));
    });
});

describe('live preview', () => {
    it('draws the page PreviewPage rendered and counts its pages', async () => {
        fakePdf('/preview?id=3', 3);
        vi.mocked(PreviewPage).mockResolvedValueOnce({url: '/preview?id=3', pages: 3});
        radio('pages-paper', 'letter').click();
        await vi.waitFor(() => expect(el('pages-count').textContent).toBe('Page 1 of 3'));
        await vi.waitFor(() => expect(el('pages-sheet').querySelector('canvas')).not.toBeNull());
        expect(renders.at(-1)).toEqual({url: '/preview?id=3', page: 1});
        expect(el<HTMLButtonElement>('pages-prev').disabled).toBe(true);
        el('pages-next').click();
        await vi.waitFor(() => expect(renders.at(-1)).toEqual({url: '/preview?id=3', page: 2}));
        expect(el('pages-count').textContent).toBe('Page 2 of 3');
    });

    it('renders again for every change of kind, form, Page and Lettering', async () => {
        await previewDone();
        const before = vi.mocked(PreviewPage).mock.calls.length;
        radio('pages-dotgrid-spacing', '4').click();
        await vi.waitFor(() => expect(vi.mocked(PreviewPage).mock.calls.length).toBe(before + 1));
        radio('pages-paper', 'b5').click();
        await vi.waitFor(() => expect(vi.mocked(PreviewPage).mock.calls.length).toBe(before + 2));
        setTitle('Ideas');
        await vi.waitFor(() => expect(vi.mocked(PreviewPage).mock.calls.length).toBe(before + 3));
        pickFont('pacifico');
        await vi.waitFor(() => expect(vi.mocked(PreviewPage).mock.calls.length).toBe(before + 4));
        setColourIn(true);
        await vi.waitFor(() => expect(vi.mocked(PreviewPage).mock.calls.length).toBe(before + 5));
        radio('pages-kinds', 'cutout').click();
        await vi.waitFor(() => expect(vi.mocked(PreviewPage).mock.calls.length).toBe(before + 6));
        expect(lastPreviewPage()).toEqual(['cutout', expect.objectContaining({paper: 'b5', title: '', font: 'pacifico', colourIn: true}), '{}']);
        radio('pages-dotgrid-spacing', '5').click();
    });

    it('ignores an answer to an older request', async () => {
        fakePdf('/preview?id=2', 2);
        fakePdf('/preview?id=9', 9);
        let answerOld: (info: {url: string; pages: number}) => void = () => {};
        vi.mocked(PreviewPage).mockImplementationOnce(() => new Promise((done) => { answerOld = done; }));
        vi.mocked(PreviewPage).mockResolvedValueOnce({url: '/preview?id=2', pages: 2});
        radio('pages-paper', 'a4').click();
        await vi.waitFor(() => expect(PreviewPage).toHaveBeenCalledTimes(1));
        radio('pages-paper', 'b5').click();
        await vi.waitFor(() => expect(el('pages-count').textContent).toBe('Page 1 of 2'));
        answerOld({url: '/preview?id=9', pages: 9});
        await new Promise((done) => setTimeout(done, 20));
        expect(el('pages-count').textContent).toBe('Page 1 of 2');
    });

    it('shows the error instead of a page when PreviewPage fails', async () => {
        vi.mocked(PreviewPage).mockRejectedValueOnce(new Error('boom'));
        radio('pages-paper', 'a4').click();
        await vi.waitFor(() => expect(el('pages-preview-message').textContent).toBe('Could not show the preview: boom'));
        expect(el('pages-preview-message').hidden).toBe(false);
        expect(el('pages-count').textContent).toBe('');
        expect(el('pages-sheet').querySelector('canvas')).toBeNull();
        radio('pages-paper', 'b5').click();
        await vi.waitFor(() => expect(el('pages-preview-message').hidden).toBe(true));
    });
});

describe('export', () => {
    it('saves the page with ExportPage and offers to open it', async () => {
        vi.mocked(ExportPage).mockResolvedValueOnce('/out/dotgrid_A5.pdf');
        radio('pages-dotgrid-spacing', '4').click();
        setTitle('Notes');
        // The page count comes from the preview of these options.
        await previewDone();
        el('pages-export').click();
        await vi.waitFor(() => expect(el('pages-status').textContent).toBe('Saved dotgrid_A5.pdf: 1 page'));
        const [kind, dir, settings, options] = vi.mocked(ExportPage).mock.calls[0];
        expect(kind).toBe('dotgrid');
        expect(dir).toBe('/covers');
        expect({...settings}).toEqual({paper: 'a5', cut: '', title: 'Notes', font: 'go', colourIn: false});
        expect(options).toBe('{"spacing":4}');
        expect(el('pages-open-pdf').hidden).toBe(false);
        expect(el<HTMLButtonElement>('pages-export').disabled).toBe(false);

        el('pages-open-pdf').click();
        expect(OpenFile).toHaveBeenCalledWith('/out/dotgrid_A5.pdf');
        radio('pages-dotgrid-spacing', '5').click();
    });

    it('keeps the last result when the dialog is cancelled', async () => {
        vi.mocked(ExportPage).mockResolvedValueOnce('/out/first.pdf');
        await previewDone();
        el('pages-export').click();
        await vi.waitFor(() => expect(el('pages-status').textContent).toBe('Saved first.pdf: 1 page'));
        vi.mocked(ExportPage).mockResolvedValueOnce('');
        el('pages-export').click();
        await vi.waitFor(() => expect(el<HTMLButtonElement>('pages-export').disabled).toBe(false));
        expect(el('pages-status').textContent).toBe('Saved first.pdf: 1 page');
        expect(el('pages-open-pdf').hidden).toBe(false);
    });

    it('reports an error', async () => {
        vi.mocked(ExportPage).mockRejectedValueOnce(new Error('boom'));
        el('pages-export').click();
        await vi.waitFor(() => expect(deps.showError).toHaveBeenCalledWith('Could not export PDF: boom'));
        expect(el('pages-status').textContent).toBe('');
        expect(el('pages-open-pdf').hidden).toBe(true);
        expect(el<HTMLButtonElement>('pages-export').disabled).toBe(false);
    });
});
