import {afterEach, beforeAll, describe, expect, it, vi} from 'vitest';

import {LetteringFonts} from '../wailsjs/go/main/App';
import {main} from '../wailsjs/go/models';
import {applyDom, setLang} from './i18n';
import {fontFamily, letteringFonts, renderLettering, type Lettering} from './lettering';

// Fonts as LetteringFonts sends them (see app.go), one of each hollow
// capability and several style groups.
const {fontList} = vi.hoisted(() => {
    const font = (id: string, label: string, group: string, hollow: string) => ({
        id, label, group, hollow,
        licence: 'OFL-1.1',
        url: `/fonts/${id}.ttf`,
        licenceUrl: `/fonts/${id}/licence`,
        default: id === 'go',
    });
    return {
        fontList: [
            font('go', 'Default', 'sans', 'stroke'),
            font('kaushan-script', 'Kaushan Script', 'brush', 'stroke'),
            font('sacramento', 'Sacramento', 'monoline', 'none'),
            font('titan-one', 'Titan One', 'display', 'stroke'),
            font('bungee-outline', 'Bungee Outline', 'hollow', 'native'),
        ],
    };
});

vi.mock('../wailsjs/go/main/App', async (importOriginal) => {
    const original = await importOriginal<Record<string, unknown>>();
    const stubs: Record<string, unknown> = {};
    for (const name of Object.keys(original)) {
        stubs[name] = vi.fn(async () => undefined);
    }
    stubs.LetteringFonts = vi.fn(async () => fontList);
    return stubs;
});

function key(target: HTMLElement, name: string) {
    target.dispatchEvent(new KeyboardEvent('keydown', {key: name, bubbles: true, cancelable: true}));
}

describe('letteringFonts', () => {
    it('loads the fonts once and adds one @font-face per font, from the served URLs', async () => {
        const [first, second] = await Promise.all([letteringFonts(), letteringFonts()]);
        expect(first.map((f) => f.id)).toEqual(fontList.map((f) => f.id));
        expect(second).toBe(first);
        await letteringFonts();
        expect(LetteringFonts).toHaveBeenCalledTimes(1);
        const styles = document.head.querySelectorAll('style#lettering-font-faces');
        expect(styles.length).toBe(1);
        const css = styles[0].textContent ?? '';
        expect(css.match(/@font-face/g)?.length).toBe(fontList.length);
        for (const font of fontList) {
            expect(css).toContain(`font-family: "lettering-${font.id}"`);
            expect(css).toContain(`src: url("/fonts/${font.id}.ttf") format("truetype")`);
        }
        expect(css).toContain('font-display: swap');
    });

    it('names each font family by its ID, falling back to the UI font', () => {
        expect(fontFamily('titan-one')).toMatch(/^"lettering-titan-one", /);
    });
});

describe('Lettering controls', () => {
    let lettering: Lettering;
    const onChange = vi.fn();

    beforeAll(() => {
        document.body.innerHTML = '';
        lettering = renderLettering('test', onChange);
        document.body.append(lettering.fontField, lettering.colourInField);
        const outside = document.createElement('button');
        outside.id = 'outside';
        document.body.append(outside);
        lettering.setFonts(fontList.map((f) => main.LetteringFontInfo.createFrom(f)));
    });

    afterEach(() => {
        lettering.picker.close();
        lettering.picker.value = 'go';
        lettering.colourIn.checked = false;
        lettering.colourIn.dispatchEvent(new Event('change'));
        onChange.mockClear();
    });

    const button = () => document.getElementById('test-font') as HTMLButtonElement;
    const list = () => document.getElementById('test-font-list') as HTMLUListElement;
    const options = () => Array.from(list().querySelectorAll<HTMLLIElement>('[role="option"]'));
    const option = (id: string) => list().querySelector<HTMLLIElement>(`[role="option"][data-value="${id}"]`)!;
    const active = () => list().getAttribute('aria-activedescendant');
    const hint = () => document.getElementById('test-colour-in-hint')!.textContent;

    describe('font picker', () => {
        it('lists every font in its own face, with its group and outline badge', () => {
            expect(list().getAttribute('role')).toBe('listbox');
            expect(list().getAttribute('aria-label')).toBe('Font');
            const rows = options().map((row) => [
                row.dataset.value,
                row.querySelector('.font-option-name')?.textContent,
                row.querySelector('.font-option-group')?.textContent,
                row.querySelector('.font-badge')?.textContent,
            ]);
            expect(rows).toEqual([
                ['go', 'Default', 'Sans', 'outline ok'],
                ['kaushan-script', 'Kaushan Script', 'Brush script', 'outline ok'],
                ['sacramento', 'Sacramento', 'Monoline script', 'no outline'],
                ['titan-one', 'Titan One', 'Bold display', 'outline ok'],
                ['bungee-outline', 'Bungee Outline', 'Hollow', 'hollow'],
            ]);
            const name = option('sacramento').querySelector<HTMLElement>('.font-option-name')!;
            expect(name.style.fontFamily).toContain('lettering-sacramento');
            expect(option('bungee-outline').querySelector<HTMLElement>('.font-badge')?.dataset.hollow).toBe('native');
        });

        it('shows the chosen font on a closed button, in that font', () => {
            expect(button().getAttribute('aria-haspopup')).toBe('listbox');
            expect(button().getAttribute('aria-expanded')).toBe('false');
            expect(button().getAttribute('aria-controls')).toBe('test-font-list');
            expect(list().hidden).toBe(true);
            const shown = button().querySelector<HTMLElement>('.font-picker-name')!;
            expect(shown.textContent).toBe('Default');
            expect(shown.style.fontFamily).toContain('lettering-go');
            expect(option('go').getAttribute('aria-selected')).toBe('true');
            expect(option('titan-one').getAttribute('aria-selected')).toBe('false');
            expect(lettering.options()).toEqual({font: 'go', colourIn: false});
        });

        it('opens on click with the chosen font active, and closes on a second click', () => {
            lettering.picker.value = 'titan-one';
            button().click();
            expect(list().hidden).toBe(false);
            expect(button().getAttribute('aria-expanded')).toBe('true');
            expect(document.activeElement).toBe(list());
            expect(active()).toBe(option('titan-one').id);
            expect(option('titan-one').classList.contains('active')).toBe(true);
            button().click();
            expect(list().hidden).toBe(true);
            expect(button().getAttribute('aria-expanded')).toBe('false');
        });

        it('moves with the arrows, Home and End, and selects with Enter', () => {
            button().click();
            key(list(), 'ArrowDown');
            expect(active()).toBe(option('kaushan-script').id);
            key(list(), 'End');
            expect(active()).toBe(option('bungee-outline').id);
            key(list(), 'ArrowDown');
            expect(active()).toBe(option('bungee-outline').id);
            key(list(), 'Home');
            expect(active()).toBe(option('go').id);
            key(list(), 'ArrowUp');
            expect(active()).toBe(option('go').id);
            key(list(), 'ArrowDown');
            key(list(), 'ArrowDown');
            key(list(), 'Enter');
            expect(lettering.picker.value).toBe('sacramento');
            expect(onChange).toHaveBeenCalledTimes(1);
            expect(list().hidden).toBe(true);
            expect(document.activeElement).toBe(button());
            expect(option('sacramento').getAttribute('aria-selected')).toBe('true');
            expect(option('go').getAttribute('aria-selected')).toBe('false');
            expect(button().querySelector('.font-picker-name')?.textContent).toBe('Sacramento');
        });

        it('selects with Space', () => {
            button().click();
            key(list(), 'ArrowDown');
            key(list(), ' ');
            expect(lettering.picker.value).toBe('kaushan-script');
            expect(list().hidden).toBe(true);
        });

        it('opens from the button with the arrow keys', () => {
            key(button(), 'ArrowDown');
            expect(list().hidden).toBe(false);
            expect(document.activeElement).toBe(list());
        });

        it('closes with Escape, keeping the font and focusing the button', () => {
            button().click();
            key(list(), 'End');
            key(list(), 'Escape');
            expect(list().hidden).toBe(true);
            expect(lettering.picker.value).toBe('go');
            expect(onChange).not.toHaveBeenCalled();
            expect(document.activeElement).toBe(button());
        });

        it('closes with Tab, keeping the font', () => {
            button().click();
            key(list(), 'ArrowDown');
            key(list(), 'Tab');
            expect(list().hidden).toBe(true);
            expect(lettering.picker.value).toBe('go');
        });

        it('selects a font with a click', () => {
            button().click();
            option('titan-one').click();
            expect(lettering.picker.value).toBe('titan-one');
            expect(onChange).toHaveBeenCalledTimes(1);
            expect(list().hidden).toBe(true);
            expect(document.activeElement).toBe(button());
            expect(lettering.options().font).toBe('titan-one');
        });

        it('does not report picking the font already chosen', () => {
            button().click();
            option('go').click();
            expect(onChange).not.toHaveBeenCalled();
            expect(list().hidden).toBe(true);
        });

        it('closes on a click outside', () => {
            button().click();
            document.getElementById('outside')!.dispatchEvent(new MouseEvent('mousedown', {bubbles: true}));
            expect(list().hidden).toBe(true);
            expect(button().getAttribute('aria-expanded')).toBe('false');
        });
    });

    describe('Colour-in', () => {
        function pick(id: string) {
            button().click();
            option(id).click();
        }

        it('is a checkbox with a hint, enabled for every font', () => {
            const box = lettering.colourIn;
            expect(box.type).toBe('checkbox');
            expect(box.closest('label')?.textContent).toContain('Colour-in (outlines only)');
            for (const font of fontList) {
                lettering.picker.value = font.id;
                expect(box.disabled).toBe(false);
            }
        });

        it('says how a font that can be outlined prints', () => {
            pick('titan-one');
            expect(hint()).toBe('Prints outlines only, ready to colour by hand.');
            lettering.colourIn.click();
            expect(lettering.options()).toEqual({font: 'titan-one', colourIn: true});
            expect(hint()).toBe('Letters print as outlines, ready to colour by hand.');
            expect(onChange).toHaveBeenCalledTimes(2);
        });

        it('says a thin font stays filled, keeping the choice', () => {
            lettering.colourIn.click();
            pick('sacramento');
            expect(lettering.colourIn.checked).toBe(true);
            expect(lettering.options()).toEqual({font: 'sacramento', colourIn: true});
            expect(hint()).toBe('This font is too thin for outlines; letters stay filled.');
        });

        it('shows checked for a hollow font without changing the stored choice', () => {
            expect(lettering.colourIn.checked).toBe(false);
            pick('bungee-outline');
            expect(lettering.colourIn.checked).toBe(true);
            expect(hint()).toBe('This font is hollow by design, so letters always print as outlines.');
            // The user's own choice is sent: Go never strokes a hollow font.
            expect(lettering.options()).toEqual({font: 'bungee-outline', colourIn: false});
            // A click cannot uncheck it while the font is hollow.
            lettering.colourIn.click();
            expect(lettering.colourIn.checked).toBe(true);
            expect(lettering.options().colourIn).toBe(false);
            pick('titan-one');
            expect(lettering.colourIn.checked).toBe(false);
            expect(lettering.options()).toEqual({font: 'titan-one', colourIn: false});
        });

        it('restores a remembered font and choice without reporting a change', () => {
            lettering.restore('titan-one', true);
            expect(lettering.options()).toEqual({font: 'titan-one', colourIn: true});
            expect(lettering.colourIn.checked).toBe(true);
            expect(hint()).toBe('Letters print as outlines, ready to colour by hand.');
            expect(onChange).not.toHaveBeenCalled();
        });

        it('keeps the font shown for a remembered font no longer listed', () => {
            lettering.restore('gone', false);
            expect(lettering.options()).toEqual({font: 'go', colourIn: false});
            lettering.restore(undefined, true);
            expect(lettering.options()).toEqual({font: 'go', colourIn: true});
        });

        it('restores the choice under a hollow font, which shows checked', () => {
            lettering.restore('bungee-outline', false);
            expect(lettering.colourIn.checked).toBe(true);
            expect(lettering.options()).toEqual({font: 'bungee-outline', colourIn: false});
        });

        it('hides with the font picker where lettering does not apply', () => {
            lettering.show(false);
            expect(lettering.fontField.hidden).toBe(true);
            expect(lettering.colourInField.hidden).toBe(true);
            lettering.show(true);
            expect(lettering.fontField.hidden).toBe(false);
            expect(lettering.colourInField.hidden).toBe(false);
        });
    });

    describe('in Spanish', () => {
        afterEach(() => {
            setLang('en');
            lettering.relabel();
            applyDom(document);
        });

        it('names the groups, badges, default font and hints', () => {
            setLang('es');
            lettering.relabel();
            applyDom(document);
            expect(list().getAttribute('aria-label')).toBe('Fuente');
            expect(option('go').querySelector('.font-option-name')?.textContent).toBe('Predeterminada');
            expect(button().querySelector('.font-picker-name')?.textContent).toBe('Predeterminada');
            expect(option('kaushan-script').querySelector('.font-option-group')?.textContent).toBe('Caligrafía de pincel');
            expect(option('kaushan-script').querySelector('.font-badge')?.textContent).toBe('admite contorno');
            expect(option('sacramento').querySelector('.font-badge')?.textContent).toBe('sin contorno');
            expect(option('bungee-outline').querySelector('.font-badge')?.textContent).toBe('hueca');
            expect(lettering.colourIn.closest('label')?.textContent).toContain('Para colorear (solo contornos)');
            expect(hint()).toBe('Imprime solo contornos, listos para colorear a mano.');
        });
    });
});
