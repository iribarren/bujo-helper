import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';

import {applyDom, setLang} from './i18n';
import {pageForms, type PageForm} from './pageForms';
import {parseDefaults} from './pages';

// The moon kind's defaults as PageKinds sends them (Go: sheet.MoonKind).
const MOON_DEFAULTS = '{"month":0,"year":0,"layout":"grid","hemisphere":"north"}';

let box: HTMLDivElement;
let onChange: ReturnType<typeof vi.fn>;
// The forms a test built, disposed after it so their language listeners
// do not pile up.
let built: PageForm[] = [];

// moonForm builds a moon form from defaults (JSON), as the tab does.
function moonForm(defaults = MOON_DEFAULTS): PageForm {
    box = document.createElement('div');
    document.body.replaceChildren(box);
    onChange = vi.fn();
    const form = pageForms.moon(box, 'test-moon', parseDefaults(defaults), onChange);
    built.push(form);
    return form;
}

function q<T extends HTMLElement>(selector: string): T {
    const node = box.querySelector<T>(selector);
    if (!node) throw new Error(`missing ${selector}`);
    return node;
}

function radio(group: string, value: string): HTMLButtonElement {
    return q<HTMLButtonElement>(`#test-moon-${group} [role="radio"][data-value="${value}"]`);
}

function checked(group: string): string {
    return box.querySelector<HTMLElement>(`#test-moon-${group} [role="radio"][aria-checked="true"]`)?.dataset.value ?? '';
}

function month(): HTMLSelectElement {
    return q<HTMLSelectElement>('#test-moon-month');
}

function year(): HTMLInputElement {
    return q<HTMLInputElement>('#test-moon-year');
}

function typeYear(text: string, event: 'input' | 'change' = 'input') {
    year().value = text;
    year().dispatchEvent(new Event(event));
}

beforeEach(() => {
    // Only the clock: the forms use no timers.
    vi.useFakeTimers({toFake: ['Date']});
    vi.setSystemTime(new Date(2026, 10, 21, 12));
});

afterEach(() => {
    for (const form of built) form.dispose?.();
    built = [];
    vi.useRealTimers();
    setLang('en');
});

describe('moon calendar form', () => {
    it('has Month, Year, Layout and Hemisphere, the current month and year by default', () => {
        const form = moonForm();
        expect(Array.from(month().options, (o) => [o.value, o.textContent])).toEqual([
            ['1', 'January'], ['2', 'February'], ['3', 'March'], ['4', 'April'], ['5', 'May'], ['6', 'June'],
            ['7', 'July'], ['8', 'August'], ['9', 'September'], ['10', 'October'], ['11', 'November'], ['12', 'December'],
        ]);
        expect(month().value).toBe('11');
        expect(year().type).toBe('number');
        expect([year().min, year().max, year().step]).toEqual(['1900', '2100', '1']);
        expect(year().value).toBe('2026');
        // Month and Year share a row, Month first.
        expect(month().closest('.field-pair')).toBe(year().closest('.field-pair'));

        for (const [group, values, value] of [
            ['layout', ['row', 'grid', 'year'], 'grid'],
            ['hemisphere', ['north', 'south'], 'north'],
        ] as const) {
            const node = q(`#test-moon-${group}`);
            expect(node.getAttribute('role')).toBe('radiogroup');
            expect(node.classList.contains('segmented')).toBe(true);
            expect(document.getElementById(node.getAttribute('aria-labelledby') ?? '')).not.toBeNull();
            expect(Array.from(node.querySelectorAll<HTMLElement>('[role="radio"]'), (r) => r.dataset.value)).toEqual(values);
            expect(checked(group)).toBe(value);
        }
        expect(Array.from(q('#test-moon-layout').children, (r) => r.textContent)).toEqual(['Row', 'Grid', 'Year']);
        expect(Array.from(q('#test-moon-hemisphere').children, (r) => r.textContent)).toEqual(['North', 'South']);
        expect(q('#test-moon-hemisphere').title).toBe('Flips which side of the moon is lit.');
        expect(form.options()).toEqual({month: 11, year: 2026, layout: 'grid', hemisphere: 'north'});
    });

    it('starts from explicit defaults and falls back on unknown ones', () => {
        expect(moonForm('{"month":3,"year":2024,"layout":"row","hemisphere":"south"}').options())
            .toEqual({month: 3, year: 2024, layout: 'row', hemisphere: 'south'});
        expect(moonForm('{"month":13,"year":"soon","layout":"spiral","hemisphere":"east"}').options())
            .toEqual({month: 11, year: 2026, layout: 'grid', hemisphere: 'north'});
        expect(moonForm('{"month":').options()).toEqual({month: 11, year: 2026, layout: 'grid', hemisphere: 'north'});
        expect(moonForm('{"year":3000}').options().year).toBe(2100);
    });

    it('sends every change and reports it', () => {
        const form = moonForm();
        month().value = '2';
        month().dispatchEvent(new Event('change'));
        expect(onChange).toHaveBeenCalledTimes(1);
        radio('layout', 'row').click();
        expect(onChange).toHaveBeenCalledTimes(2);
        radio('hemisphere', 'south').click();
        expect(onChange).toHaveBeenCalledTimes(3);
        typeYear('2030');
        expect(onChange).toHaveBeenCalledTimes(4);
        expect(form.options()).toEqual({month: 2, year: 2030, layout: 'row', hemisphere: 'south'});
    });

    it('reaches the segmented groups with the keyboard', () => {
        const form = moonForm();
        // One tab stop per group: the checked radio.
        expect(radio('layout', 'grid').tabIndex).toBe(0);
        expect(radio('layout', 'row').tabIndex).toBe(-1);
        radio('layout', 'grid').dispatchEvent(new KeyboardEvent('keydown', {key: 'ArrowRight', bubbles: true, cancelable: true}));
        expect(checked('layout')).toBe('year');
        expect(document.activeElement).toBe(radio('layout', 'year'));
        radio('hemisphere', 'north').dispatchEvent(new KeyboardEvent('keydown', {key: 'End', bubbles: true, cancelable: true}));
        expect(checked('hemisphere')).toBe('south');
        expect(onChange).toHaveBeenCalledTimes(2);
        expect(form.options()).toMatchObject({layout: 'year', hemisphere: 'south'});
    });

    it('hides Month for the Year layout and still sends it', () => {
        const form = moonForm();
        const field = q('#test-moon-month-field');
        expect(field.hidden).toBe(false);
        radio('layout', 'year').click();
        expect(field.hidden).toBe(true);
        expect(form.options()).toEqual({month: 11, year: 2026, layout: 'year', hemisphere: 'north'});
        radio('layout', 'row').click();
        expect(field.hidden).toBe(false);
    });

    it('starts with Month hidden when the default layout is Year', () => {
        moonForm('{"layout":"year"}');
        expect(q('#test-moon-month-field').hidden).toBe(true);
    });

    describe('year', () => {
        it('ignores partial and invalid input while typing', () => {
            const form = moonForm();
            for (const partial of ['2', '20', '203', '', '2030.5', '2101', '1899']) {
                typeYear(partial);
                expect(form.options().year, partial).toBe(2026);
            }
            expect(onChange).not.toHaveBeenCalled();
            typeYear('2031');
            expect(form.options().year).toBe(2031);
            expect(onChange).toHaveBeenCalledTimes(1);
        });

        // A whole year out of range is clamped; a partial one (fewer than
        // four digits) or an empty one restores the year sent.
        const committed: Array<[typed: string, shown: string]> = [
            ['1800', '1900'],
            ['1850', '1900'],
            ['2500', '2100'],
            ['3000', '2100'],
            ['2024.6', '2025'],
            ['', '2026'],
            ['2', '2026'],
            ['20', '2026'],
            ['203', '2026'],
            ['-2024', '2026'],
        ];
        it.each(committed)('commits %j as %s', (typed, shown) => {
            const form = moonForm();
            typeYear(typed, 'change');
            expect(year().value).toBe(shown);
            expect(form.options().year).toBe(Number(shown));
            expect(onChange).toHaveBeenCalledTimes(shown === '2026' ? 0 : 1);
        });

        it('restores the last year sent on a partial commit', () => {
            const form = moonForm();
            typeYear('2030');
            typeYear('20', 'change');
            expect(year().value).toBe('2030');
            expect(form.options().year).toBe(2030);
            expect(onChange).toHaveBeenCalledTimes(1);
        });

        it('does not report a commit of the value it already sends', () => {
            moonForm();
            typeYear('2030');
            typeYear('2030', 'change');
            expect(onChange).toHaveBeenCalledTimes(1);
        });
    });

    it('names the months and choices in Spanish and follows a language change, keeping them', () => {
        setLang('es');
        const form = moonForm();
        expect(Array.from(month().options, (o) => o.textContent)).toEqual([
            'Enero', 'Febrero', 'Marzo', 'Abril', 'Mayo', 'Junio',
            'Julio', 'Agosto', 'Septiembre', 'Octubre', 'Noviembre', 'Diciembre',
        ]);
        month().value = '4';
        month().dispatchEvent(new Event('change'));
        radio('hemisphere', 'south').click();

        setLang('en');
        applyDom(box);
        expect(month().selectedOptions[0].textContent).toBe('April');
        expect(Array.from(q('#test-moon-layout').children, (r) => r.textContent)).toEqual(['Row', 'Grid', 'Year']);
        expect(q('#test-moon-month-field > span').textContent).toBe('Month');
        expect(checked('hemisphere')).toBe('south');

        setLang('es');
        applyDom(box);
        expect(month().selectedOptions[0].textContent).toBe('Abril');
        expect(Array.from(q('#test-moon-layout').children, (r) => r.textContent)).toEqual(['Fila', 'Cuadrícula', 'Año']);
        expect(Array.from(q('#test-moon-hemisphere').children, (r) => r.textContent)).toEqual(['Norte', 'Sur']);
        expect(q('#test-moon-month-field > span').textContent).toBe('Mes');
        expect(q('#test-moon-hemisphere').title).toBe('Cambia qué lado de la luna aparece iluminado.');
        expect(form.options()).toEqual({month: 4, year: 2026, layout: 'grid', hemisphere: 'south'});
    });

    it('stops following the language once disposed', () => {
        const form = moonForm();
        expect(form.dispose).toBeTypeOf('function');
        form.dispose?.();
        setLang('es');
        expect(month().options[0].textContent).toBe('January');
        expect(Array.from(q('#test-moon-layout').children, (r) => r.textContent)).toEqual(['Row', 'Grid', 'Year']);
    });
});

describe('visited map form', () => {
    // The map kind's defaults as PageKinds sends them (Go: sheet.MapKind).
    const MAP_DEFAULTS = '{"region":"world","countries":[],"colour":"teal","list":true}';

    function mapForm(defaults = MAP_DEFAULTS): PageForm {
        box = document.createElement('div');
        document.body.replaceChildren(box);
        onChange = vi.fn();
        const form = pageForms.map(box, 'test-map', parseDefaults(defaults), onChange);
        built.push(form);
        return form;
    }

    const region = () => q<HTMLSelectElement>('#test-map-region');
    const countries = () => q<HTMLTextAreaElement>('#test-map-countries');
    const list = () => q<HTMLInputElement>('#test-map-list');
    const swatch = (value: string) => q<HTMLButtonElement>(`#test-map-colour [role="radio"][data-value="${value}"]`);
    const colour = () => box.querySelector<HTMLElement>('#test-map-colour [role="radio"][aria-checked="true"]')?.dataset.value ?? '';

    function typeCountries(text: string) {
        countries().value = text;
        countries().dispatchEvent(new Event('input'));
    }

    // labelOf is the text of the label tied to control.
    function labelOf(control: HTMLElement): string {
        const tied = control.closest('label') ?? box.querySelector(`label[for="${control.id}"]`);
        return tied?.querySelector('span')?.textContent ?? '';
    }

    it('has Region, Visited countries, Colour and List countries, from the defaults', () => {
        const form = mapForm();
        expect(Array.from(region().options, (o) => [o.value, o.textContent])).toEqual([
            ['world', 'World'], ['africa', 'Africa'], ['asia', 'Asia'], ['europe', 'Europe'],
            ['north-america', 'North America'], ['south-america', 'South America'], ['oceania', 'Oceania'],
        ]);
        expect(region().value).toBe('world');
        expect(labelOf(region())).toBe('Region');

        expect(countries().tagName).toBe('TEXTAREA');
        expect(countries().rows).toBe(4);
        expect(countries().value).toBe('');
        expect(countries().placeholder).toBe('Spain, Portugal, FR, Japan, Mexico');
        expect(labelOf(countries())).toBe('Visited countries (names or ISO codes)');
        const hint = document.getElementById(countries().getAttribute('aria-describedby') ?? '');
        expect(hint?.textContent).toContain('English or Spanish');

        const group = q('#test-map-colour');
        expect(group.getAttribute('role')).toBe('radiogroup');
        expect(group.classList.contains('segmented')).toBe(true);
        expect(document.getElementById(group.getAttribute('aria-labelledby') ?? '')?.textContent).toBe('Colour');
        expect(Array.from(group.querySelectorAll<HTMLElement>('[role="radio"]'), (r) => [r.dataset.value, r.getAttribute('aria-label')]))
            .toEqual([['teal', 'Teal'], ['coral', 'Coral'], ['sage', 'Sage'], ['lavender', 'Lavender']]);
        expect(swatch('coral').querySelector<HTMLElement>('.swatch')?.style.backgroundColor).toBe('rgb(238, 132, 102)');
        expect(colour()).toBe('teal');

        expect(list().type).toBe('checkbox');
        expect(list().checked).toBe(true);
        expect(labelOf(list())).toBe('List countries');
        expect(document.getElementById(list().getAttribute('aria-describedby') ?? '')?.textContent)
            .toBe('Prints the visited countries under the map.');

        expect(form.options()).toEqual({region: 'world', countries: [], colour: 'teal', list: true});
    });

    it('starts from explicit defaults and falls back on unknown ones', () => {
        const form = mapForm('{"region":"europe","countries":["Spain","FR"],"colour":"sage","list":false}');
        expect(form.options()).toEqual({region: 'europe', countries: ['Spain', 'FR'], colour: 'sage', list: false});
        expect(countries().value).toBe('Spain, FR');
        expect(mapForm('{"region":"antarctica","countries":"Spain","colour":"red","list":"yes"}').options())
            .toEqual({region: 'world', countries: [], colour: 'teal', list: true});
        expect(mapForm('{"region":').options()).toEqual({region: 'world', countries: [], colour: 'teal', list: true});
    });

    it('sends each region', () => {
        const form = mapForm();
        const values = ['world', 'africa', 'asia', 'europe', 'north-america', 'south-america', 'oceania'];
        for (const value of [...values.slice(1), 'world']) {
            region().value = value;
            region().dispatchEvent(new Event('change'));
            expect(form.options().region).toBe(value);
        }
        expect(onChange).toHaveBeenCalledTimes(values.length);
    });

    const splits: Array<[typed: string, sent: string[]]> = [
        ['Spain, Portugal, FR', ['Spain', 'Portugal', 'FR']],
        ['Spain; Portugal;FR', ['Spain', 'Portugal', 'FR']],
        ['Spain\nPortugal\n\nFR\n', ['Spain', 'Portugal', 'FR']],
        ['  United States ,\n\n  Japan  ;; , ', ['United States', 'Japan']],
        ['', []],
        [' , ;\n', []],
        ['Côte d’Ivoire', ['Côte d’Ivoire']],
    ];
    it.each(splits)('sends %j as the countries %j', (typed, sent) => {
        const form = mapForm();
        typeCountries(typed);
        expect(form.options().countries).toEqual(sent);
        // The field keeps what the user typed.
        expect(countries().value).toBe(typed);
    });

    it('reports a change only when the countries change', () => {
        mapForm();
        typeCountries('Spain');
        expect(onChange).toHaveBeenCalledTimes(1);
        typeCountries('Spain,');
        typeCountries('Spain, ');
        expect(onChange).toHaveBeenCalledTimes(1);
        typeCountries('Spain, J');
        expect(onChange).toHaveBeenCalledTimes(2);
    });

    it('sends the chosen colour and marks it checked', () => {
        const form = mapForm();
        swatch('lavender').click();
        expect(form.options().colour).toBe('lavender');
        expect(swatch('lavender').getAttribute('aria-checked')).toBe('true');
        expect(swatch('teal').getAttribute('aria-checked')).toBe('false');
        expect(onChange).toHaveBeenCalledTimes(1);
        // One tab stop; the arrow keys move along the swatches.
        expect(swatch('lavender').tabIndex).toBe(0);
        swatch('lavender').dispatchEvent(new KeyboardEvent('keydown', {key: 'ArrowRight', bubbles: true, cancelable: true}));
        expect(form.options().colour).toBe('teal');
        expect(document.activeElement).toBe(swatch('teal'));
    });

    it('toggles List countries', () => {
        const form = mapForm();
        list().click();
        expect(form.options().list).toBe(false);
        list().click();
        expect(form.options().list).toBe(true);
        expect(onChange).toHaveBeenCalledTimes(2);
    });

    it('follows a language change, keeping the options', () => {
        const form = mapForm();
        region().value = 'south-america';
        region().dispatchEvent(new Event('change'));
        typeCountries('Spain;\nJapan');
        swatch('sage').click();
        list().click();

        setLang('es');
        applyDom(box);
        expect(Array.from(region().options, (o) => o.textContent)).toEqual([
            'Mundo', 'África', 'Asia', 'Europa', 'América del Norte', 'América del Sur', 'Oceanía',
        ]);
        expect(labelOf(region())).toBe('Región');
        expect(labelOf(countries())).toBe('Países visitados (nombres o códigos ISO)');
        expect(countries().placeholder).toBe('España, Portugal, FR, Japón, México');
        expect(Array.from(q('#test-map-colour').children, (r) => r.getAttribute('aria-label')))
            .toEqual(['Verde azulado', 'Coral', 'Salvia', 'Lavanda']);
        expect(labelOf(list())).toBe('Lista de países');
        expect(countries().value).toBe('Spain;\nJapan');
        expect(colour()).toBe('sage');
        expect(form.options()).toEqual({region: 'south-america', countries: ['Spain', 'Japan'], colour: 'sage', list: false});
    });

    it('stops following the language once disposed', () => {
        const form = mapForm();
        expect(form.dispose).toBeTypeOf('function');
        form.dispose?.();
        setLang('es');
        expect(region().options[0].textContent).toBe('World');
        expect(swatch('teal').getAttribute('aria-label')).toBe('Teal');
    });
});
