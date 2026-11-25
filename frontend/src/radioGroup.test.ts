import {beforeEach, describe, expect, it, type Mock, vi} from 'vitest';

import {initRadioGroup, type RadioGroup} from './radioGroup';

const OPTIONS = [
    {value: 'books', label: 'Books'},
    {value: 'movies', label: 'Movies'},
    {value: 'tv', label: 'TV'},
];

let container: HTMLDivElement;
let group: RadioGroup;
let onChange: Mock<() => void>;

function radio(value: string): HTMLButtonElement {
    const node = container.querySelector<HTMLButtonElement>(`[role="radio"][data-value="${value}"]`);
    if (!node) throw new Error(`missing radio ${value}`);
    return node;
}

function key(value: string, name: string) {
    radio(value).dispatchEvent(new KeyboardEvent('keydown', {key: name, bubbles: true, cancelable: true}));
}

// expectChecked checks that only value is checked and tabbable.
function expectChecked(value: string) {
    expect(group.value).toBe(value);
    for (const option of OPTIONS) {
        const on = option.value === value;
        expect(radio(option.value).getAttribute('aria-checked'), option.value).toBe(String(on));
        expect(radio(option.value).tabIndex, option.value).toBe(on ? 0 : -1);
    }
}

beforeEach(() => {
    document.body.innerHTML = '<div id="group" role="radiogroup"></div>';
    container = document.getElementById('group') as HTMLDivElement;
    onChange = vi.fn<() => void>();
    group = initRadioGroup(container, onChange);
    group.replaceOptions(OPTIONS);
});

describe('initRadioGroup', () => {
    it('renders a radio button per option, the first checked', () => {
        const radios = container.querySelectorAll('[role="radio"]');
        expect(Array.from(radios, (r) => r.textContent)).toEqual(['Books', 'Movies', 'TV']);
        expectChecked('books');
        expect(onChange).not.toHaveBeenCalled();
    });

    it('selects on click and reports the change', () => {
        radio('tv').click();
        expectChecked('tv');
        expect(onChange).toHaveBeenCalledTimes(1);
        radio('tv').click();
        expect(onChange).toHaveBeenCalledTimes(1);
    });

    it('moves and selects with the arrow keys, wrapping, and focuses the radio', () => {
        key('books', 'ArrowRight');
        expectChecked('movies');
        expect(document.activeElement).toBe(radio('movies'));
        key('movies', 'ArrowDown');
        expectChecked('tv');
        key('tv', 'ArrowRight');
        expectChecked('books');
        key('books', 'ArrowLeft');
        expectChecked('tv');
        key('tv', 'ArrowUp');
        expectChecked('movies');
        expect(onChange).toHaveBeenCalledTimes(5);
    });

    it('jumps to the first and last radio with Home and End', () => {
        key('books', 'End');
        expectChecked('tv');
        key('tv', 'Home');
        expectChecked('books');
    });

    it('sets the value without reporting a change', () => {
        group.value = 'movies';
        expectChecked('movies');
        expect(onChange).not.toHaveBeenCalled();
    });

    it('keeps the selection when the options are replaced', () => {
        group.value = 'tv';
        group.replaceOptions(OPTIONS.map((o) => ({...o, label: o.label.toUpperCase()})));
        expectChecked('tv');
        expect(radio('tv').textContent).toBe('TV');
        expect(radio('books').textContent).toBe('BOOKS');
    });

    it('falls back to the first option when the selection is gone', () => {
        group.value = 'tv';
        group.replaceOptions(OPTIONS.slice(0, 2));
        expect(group.value).toBe('books');
    });

    it('disables every radio, also after the options are replaced', () => {
        group.disabled = true;
        expect(group.disabled).toBe(true);
        group.replaceOptions(OPTIONS);
        for (const option of OPTIONS) expect(radio(option.value).disabled).toBe(true);
        group.disabled = false;
        for (const option of OPTIONS) expect(radio(option.value).disabled).toBe(false);
    });

    it('shows a colour dot named by its label for a swatch option', () => {
        group.replaceOptions([
            {value: 'teal', label: 'Teal', swatch: '#2a9d8f'},
            {value: 'coral', label: 'Coral', title: 'Coral fill', swatch: '#ee8466'},
        ]);
        const teal = radio('teal');
        expect(teal.classList.contains('swatch-button')).toBe(true);
        expect(teal.getAttribute('aria-label')).toBe('Teal');
        expect(teal.title).toBe('Teal');
        expect(radio('coral').title).toBe('Coral fill');
        const dot = teal.querySelector<HTMLElement>('.swatch');
        expect(dot?.getAttribute('aria-hidden')).toBe('true');
        expect(dot?.style.backgroundColor).toBe('rgb(42, 157, 143)');
        expect(teal.textContent).toBe('');
        radio('coral').click();
        expect(group.value).toBe('coral');
        expect(radio('coral').getAttribute('aria-checked')).toBe('true');
    });

    it('keeps plain options as text buttons', () => {
        expect(radio('books').classList.contains('swatch-button')).toBe(false);
        expect(radio('books').hasAttribute('aria-label')).toBe(false);
        expect(radio('books').children).toHaveLength(0);
    });
});
