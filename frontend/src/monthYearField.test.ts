import {afterEach, beforeEach, describe, expect, it, type Mock, vi} from 'vitest';

import {applyDom, setLang} from './i18n';
import {monthYearField, type MonthYear} from './monthYearField';

let box: HTMLDivElement;
let onChange: Mock<() => void>;
let built: MonthYear[] = [];

// field builds the Month and Year row from defaults, as a kind's form does.
function field(defaults: Record<string, unknown> = {month: 0, year: 0}): MonthYear {
    box = document.createElement('div');
    document.body.replaceChildren(box);
    onChange = vi.fn<() => void>();
    const row = monthYearField(box, 'test', defaults, onChange);
    built.push(row);
    return row;
}

const month = () => box.querySelector<HTMLSelectElement>('#test-month')!;
const year = () => box.querySelector<HTMLInputElement>('#test-year')!;

function typeYear(text: string, event: 'input' | 'change' = 'input') {
    year().value = text;
    year().dispatchEvent(new Event(event));
}

beforeEach(() => {
    vi.useFakeTimers({toFake: ['Date']});
    vi.setSystemTime(new Date(2026, 10, 21, 12));
});

afterEach(() => {
    for (const row of built) row.dispose();
    built = [];
    vi.useRealTimers();
    setLang('en');
});

describe('monthYearField', () => {
    it('renders a Month select and a Year input on one row, the current month and year by default', () => {
        const row = field();
        const pair = box.querySelector('.field-pair');
        expect(pair).not.toBeNull();
        expect(month().closest('.field-pair')).toBe(pair);
        expect(year().closest('.field-pair')).toBe(pair);
        expect(row.monthField.id).toBe('test-month-field');
        expect(row.monthField.contains(month())).toBe(true);
        expect(month().options).toHaveLength(12);
        expect(month().options[0].textContent).toBe('January');
        expect(year().closest('.year-field')).not.toBeNull();
        expect([year().type, year().min, year().max, year().step]).toEqual(['number', '1900', '2100', '1']);
        expect(year().title).toBe('A year from 1900 to 2100.');
        expect([row.month(), row.year()]).toEqual([11, 2026]);
        expect([month().value, year().value]).toEqual(['11', '2026']);
    });

    it('starts from explicit defaults and falls back on unknown ones', () => {
        const row = field({month: 2, year: 2024});
        expect([row.month(), row.year()]).toEqual([2, 2024]);
        const odd = field({month: 0.5, year: 'soon'});
        expect([odd.month(), odd.year()]).toEqual([11, 2026]);
        expect(field({year: 1800}).year()).toBe(1900);
        expect(field({}).month()).toBe(11);
    });

    it('reports a month change and a valid typed year', () => {
        const row = field();
        month().value = '2';
        month().dispatchEvent(new Event('change'));
        expect(onChange).toHaveBeenCalledTimes(1);
        typeYear('20');
        expect(onChange).toHaveBeenCalledTimes(1);
        typeYear('2028');
        expect(onChange).toHaveBeenCalledTimes(2);
        expect([row.month(), row.year()]).toEqual([2, 2028]);
        typeYear('3000', 'change');
        expect(year().value).toBe('2100');
        expect(row.year()).toBe(2100);
        typeYear('', 'change');
        expect(year().value).toBe('2100');
        expect(onChange).toHaveBeenCalledTimes(3);
    });

    it('names the months in the UI language, keeping the selection, until disposed', () => {
        const row = field({month: 4, year: 2026});
        setLang('es');
        applyDom(box);
        expect(month().selectedOptions[0].textContent).toBe('Abril');
        expect(box.querySelector('#test-month-field > span')?.textContent).toBe('Mes');
        expect(row.month()).toBe(4);
        row.dispose();
        setLang('en');
        expect(month().selectedOptions[0].textContent).toBe('Abril');
    });
});
