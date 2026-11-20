import {beforeEach, describe, expect, it, vi} from 'vitest';

import {initTabs, type Tabs} from './tabs';

// A tab list like index.html's: each tab names its panel with aria-controls
// and its own name with data-tab.
const MARKUP = `
    <div id="tabs" role="tablist">
        <button id="tab-one" role="tab" data-tab="one" aria-selected="true" aria-controls="panel-one">One</button>
        <button id="tab-two" role="tab" data-tab="two" aria-selected="false" aria-controls="panel-two" tabindex="-1">Two</button>
        <button id="tab-three" role="tab" data-tab="three" aria-selected="false" aria-controls="panel-three" tabindex="-1">Three</button>
    </div>
    <div id="panel-one" role="tabpanel"></div>
    <div id="panel-two" role="tabpanel" hidden></div>
    <div id="panel-three" role="tabpanel" hidden></div>`;

function el(id: string): HTMLElement {
    const node = document.getElementById(id);
    if (!node) throw new Error(`missing #${id}`);
    return node;
}

function key(id: string, name: string) {
    el(id).dispatchEvent(new KeyboardEvent('keydown', {key: name, bubbles: true, cancelable: true}));
}

// expectSelected checks that only tab name is selected, tabbable and shown.
function expectSelected(name: string) {
    for (const other of ['one', 'two', 'three']) {
        const on = other === name;
        expect(el(`tab-${other}`).getAttribute('aria-selected'), other).toBe(String(on));
        expect(el(`tab-${other}`).tabIndex, other).toBe(on ? 0 : -1);
        expect(el(`panel-${other}`).hidden, other).toBe(!on);
    }
}

let tabs: Tabs;
let onSelect: ReturnType<typeof vi.fn>;

beforeEach(() => {
    document.body.innerHTML = MARKUP;
    onSelect = vi.fn();
    tabs = initTabs(el('tabs'), onSelect);
});

describe('initTabs', () => {
    it('lists the tabs of the tab list by name', () => {
        expect(tabs.names()).toEqual(['one', 'two', 'three']);
    });

    it('throws when a tab has no panel', () => {
        document.body.innerHTML = '<div id="t" role="tablist"><button role="tab" data-tab="x" aria-controls="nope"></button></div>';
        expect(() => initTabs(el('t'), () => {})).toThrow('#nope');
    });

    it('switches panel, aria-selected and tab stop on click', () => {
        el('tab-two').click();
        expectSelected('two');
        expect(onSelect).toHaveBeenLastCalledWith('two');
    });

    it('selects by name', () => {
        tabs.select('three');
        expectSelected('three');
        expect(onSelect).toHaveBeenLastCalledWith('three');
    });

    it('moves right and left with the arrows, wrapping, and focuses the tab', () => {
        key('tab-one', 'ArrowRight');
        expectSelected('two');
        expect(document.activeElement).toBe(el('tab-two'));
        key('tab-two', 'ArrowRight');
        key('tab-three', 'ArrowRight');
        expectSelected('one');
        key('tab-one', 'ArrowLeft');
        expectSelected('three');
        expect(document.activeElement).toBe(el('tab-three'));
    });

    it('jumps to the first and last tab with Home and End', () => {
        key('tab-one', 'End');
        expectSelected('three');
        key('tab-three', 'Home');
        expectSelected('one');
    });

    it('ignores other keys', () => {
        key('tab-one', 'ArrowDown');
        expectSelected('one');
        expect(onSelect).not.toHaveBeenCalled();
    });
});
