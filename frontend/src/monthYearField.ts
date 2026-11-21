// The Month and Year row shared by the calendar page kinds' forms (moon,
// month calendar, habit tracker): a Month select and a Year number input
// side by side, Month first.

import {titled, translated} from './fieldsets';
import {onLangChange, t} from './i18n';

// The years the calendar kinds accept (Go: sheet.MinCalendarYear and
// sheet.MaxCalendarYear, and the moon options' validation).
export const MIN_YEAR = 1900;
export const MAX_YEAR = 2100;

export interface MonthYear {
    // The Month field (its label and select), e.g. to hide it.
    monthField: HTMLElement;
    // The month (1–12) and year Go gets: always valid.
    month: () => number;
    year: () => number;
    // dispose stops following the UI language.
    dispose: () => void;
}

function clampYear(year: number): number {
    return Math.min(MAX_YEAR, Math.max(MIN_YEAR, year));
}

// partialYear reports whether a committed year is still being typed: fewer
// than four digits (e.g. "20" while typing 2026), or not positive.
function partialYear(year: number): boolean {
    return year < 1000;
}

// validYear is the year text as a number when it is a whole year the
// calendar kinds accept, undefined otherwise (e.g. "20" while typing 2026).
function validYear(text: string): number | undefined {
    const year = Number(text);
    return text.trim() !== '' && Number.isInteger(year) && year >= MIN_YEAR && year <= MAX_YEAR ? year : undefined;
}

// monthYearField appends the Month and Year row to container, with the IDs
// "<idPrefix>-month-field", "<idPrefix>-month" and "<idPrefix>-year". A
// default month or year of 0 (or none, or an invalid one) is the current
// one. onChange is called when the user changes the month or the year Go
// gets. The Year input sends a valid year as it is typed; on commit
// (change) a fractional year is rounded, an out-of-range whole year
// clamped to 1900–2100, and an empty or partial one (fewer than four
// digits) restored to the year sent, so Go never gets a year it rejects.
// The month names follow the UI language, keeping the selection.
export function monthYearField(
    container: HTMLElement,
    idPrefix: string,
    defaults: Record<string, unknown>,
    onChange: () => void,
): MonthYear {
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

    function relabel() {
        const names = t('pages.months').split(',');
        Array.from(month.options).forEach((option, i) => {
            option.textContent = names[i] ?? option.value;
        });
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
        monthField,
        month: () => Number(month.value),
        year: () => current,
        dispose: stopRelabel,
    };
}
