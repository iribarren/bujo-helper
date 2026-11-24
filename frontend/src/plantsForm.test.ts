import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';

import {ChoosePlantImage} from '../wailsjs/go/main/App';
import {applyDom, setLang} from './i18n';
import {pageForms, type PageForm} from './pageForms';
import {parseDefaults} from './pages';
import {plantList} from './plantsForm';

vi.mock('../wailsjs/go/main/App', async (importOriginal) => {
    const original = await importOriginal<Record<string, unknown>>();
    return {...original, ChoosePlantImage: vi.fn()};
});

// The plants kind's defaults as PageKinds sends them (Go: sheet.PlantsKind).
const PLANTS_DEFAULTS = '{"plants":[],"size":"a6"}';

// plant is a plant as Go sends and decodes it (Go: sheet.Plant).
function plant(name: string, extra: Partial<Record<string, unknown>> = {}) {
    return {name, light: '', water: '', soil: '', notes: '', image: '', ...extra};
}

// deferred is a promise resolved or rejected by hand, for the binding.
function deferred<T>() {
    let resolve!: (v: T) => void;
    const promise = new Promise<T>((res) => {
        resolve = res;
    });
    return {promise, resolve};
}

let box: HTMLDivElement;
let onChange: ReturnType<typeof vi.fn>;
let built: PageForm[] = [];

function plantsForm(defaults = PLANTS_DEFAULTS): PageForm {
    box = document.createElement('div');
    document.body.replaceChildren(box);
    onChange = vi.fn();
    const form = pageForms.plants(box, 'test-plants', parseDefaults(defaults), onChange);
    built.push(form);
    return form;
}

function q<T extends HTMLElement>(selector: string): T {
    const node = box.querySelector<T>(selector);
    if (!node) throw new Error(`missing ${selector}`);
    return node;
}

const items = () => Array.from(box.querySelectorAll<HTMLLIElement>('#test-plants-list li'));
const pills = () => items().map((li) => li.querySelector<HTMLButtonElement>('.plants-pick')!);
const removes = () => items().map((li) => li.querySelector<HTMLButtonElement>('.icon-button')!);
const pillTexts = () => pills().map((p) => p.textContent);
const selected = () => pills().findIndex((p) => p.getAttribute('aria-current') === 'true');
const addButton = () => q<HTMLButtonElement>('#test-plants-add');
const status = () => q<HTMLElement>('#test-plants-status');
const editor = () => q<HTMLElement>('#test-plants-editor');
const field = (name: string) => q<HTMLInputElement | HTMLTextAreaElement>(`#test-plants-${name}`);
const imageName = () => q<HTMLElement>('#test-plants-image-name');
const imageChoose = () => q<HTMLButtonElement>('#test-plants-image');
const imageRemove = () => q<HTMLButtonElement>('#test-plants-image-remove');
const labelOf = (name: string) => box.querySelector(`label[for="${field(name).id}"]`)?.textContent;
const sizeLabels = () => Array.from(q('#test-plants-size').children, (r) => r.textContent);

function type(name: string, text: string) {
    const node = field(name);
    node.value = text;
    node.dispatchEvent(new Event('input'));
}

function radio(value: string): HTMLButtonElement {
    return q<HTMLButtonElement>(`#test-plants-size [role="radio"][data-value="${value}"]`);
}

function checkedSize(): string {
    return box.querySelector<HTMLElement>('#test-plants-size [role="radio"][aria-checked="true"]')?.dataset.value ?? '';
}

function sent(form: PageForm) {
    return form.options() as {plants: Array<Record<string, unknown>>; size: string};
}

// settle lets pending promise callbacks run.
const settle = () => new Promise<void>((resolve) => setTimeout(resolve, 0));

beforeEach(() => {
    vi.mocked(ChoosePlantImage).mockReset();
});

afterEach(() => {
    for (const form of built) form.dispose?.();
    built = [];
    setLang('en');
});

describe('plant care cards form', () => {
    it('starts with no plants, A6 cards and the blank cards hint', () => {
        const form = plantsForm();
        expect(items()).toEqual([]);
        expect(editor().hidden).toBe(true);
        expect(q('#test-plants-hint').textContent).toBe('No plants prints blank cards to fill in by hand.');
        expect(q('#test-plants-hint').classList.contains('muted')).toBe(true);
        expect(addButton().textContent).toBe('Add plant');
        expect(sizeLabels()).toEqual(['A6', '4 × 6 in', 'Full page']);
        expect(checkedSize()).toBe('a6');
        expect(form.options()).toEqual({plants: [], size: 'a6'});
        expect(onChange).not.toHaveBeenCalled();
    });

    it('starts from the defaults, the first plant selected, and falls back on an unknown size', () => {
        const form = plantsForm(JSON.stringify({
            plants: [plant('Fern', {light: 'Shade', notes: 'Mist.'}), {image: '/n/Ivy.png', water: 3}, 'junk'],
            size: 'huge',
        }));
        expect(pillTexts()).toEqual(['Fern', 'Ivy']);
        expect(selected()).toBe(0);
        expect(field('name').value).toBe('Fern');
        expect(field('light').value).toBe('Shade');
        expect(field('notes').value).toBe('Mist.');
        expect(checkedSize()).toBe('a6');
        expect(sent(form).plants).toEqual([plant('Fern', {light: 'Shade', notes: 'Mist.'}), plant('', {image: '/n/Ivy.png'})]);
    });

    it('adds a plant, selects it and sends what is typed', () => {
        const form = plantsForm();
        addButton().click();
        expect(pillTexts()).toEqual(['Unnamed plant']);
        expect(selected()).toBe(0);
        expect(editor().hidden).toBe(false);
        expect(document.activeElement).toBe(field('name'));
        expect(onChange).toHaveBeenCalledTimes(1);

        type('name', 'Monstera');
        type('light', 'Bright, indirect');
        type('water', 'When the top 3 cm are dry');
        type('soil', 'Chunky aroid mix');
        type('notes', 'Wipe the leaves.\nRepot in spring.');
        expect(pillTexts()).toEqual(['Monstera']);
        expect(onChange).toHaveBeenCalledTimes(6);
        expect(sent(form)).toEqual({
            plants: [plant('Monstera', {
                light: 'Bright, indirect',
                water: 'When the top 3 cm are dry',
                soil: 'Chunky aroid mix',
                notes: 'Wipe the leaves.\nRepot in spring.',
            })],
            size: 'a6',
        });
    });

    it('selects a plant with a click and edits that one', () => {
        const form = plantsForm(JSON.stringify({plants: [plant('A'), plant('B')], size: 'a6'}));
        pills()[1].click();
        expect(selected()).toBe(1);
        expect(field('name').value).toBe('B');
        type('soil', 'Peat-free');
        expect(sent(form).plants.map((p) => p.soil)).toEqual(['', 'Peat-free']);
        pills()[0].click();
        expect(field('soil').value).toBe('');
        expect(onChange).toHaveBeenCalledTimes(1);
    });

    it('removes a plant and selects its neighbour', () => {
        const form = plantsForm(JSON.stringify({plants: [plant('A'), plant('B'), plant('C')], size: 'a6'}));
        expect(removes().map((b) => b.getAttribute('aria-label'))).toEqual(['Remove plant: A', 'Remove plant: B', 'Remove plant: C']);
        pills()[2].click();
        removes()[2].click();
        expect(pillTexts()).toEqual(['A', 'B']);
        expect(selected()).toBe(1);
        removes()[0].click();
        removes()[0].click();
        expect(items()).toEqual([]);
        expect(editor().hidden).toBe(true);
        expect(sent(form).plants).toEqual([]);
        expect(onChange).toHaveBeenCalledTimes(3);
    });

    it('names an unnamed plant after its image file, and says the name is taken from it', () => {
        const form = plantsForm(JSON.stringify({plants: [plant('', {image: '/n/Monstera - Monstera deliciosa.jpg'}), plant('')], size: 'a6'}));
        expect(pillTexts()).toEqual(['Monstera', 'Unnamed plant']);
        expect(removes()[0].getAttribute('aria-label')).toBe('Remove plant: Monstera');
        expect(field('name').value).toBe('');
        expect(field('name').getAttribute('placeholder')).toBe('Taken from the image file name when empty');
        // The name is sent empty: Go reads it from the file name.
        expect(sent(form).plants[0].name).toBe('');
        type('name', 'Swiss cheese plant');
        expect(pillTexts()).toEqual(['Swiss cheese plant', 'Unnamed plant']);
    });

    it('chooses and removes the image of the selected plant', async () => {
        const form = plantsForm(JSON.stringify({plants: [plant(''), plant('B')], size: 'a6'}));
        expect(imageName().textContent).toBe('No image');
        expect(imageRemove().hidden).toBe(true);

        vi.mocked(ChoosePlantImage).mockResolvedValueOnce('/home/me/Nature/Ivy - Hedera helix.jpg');
        imageChoose().click();
        await vi.waitFor(() => expect(imageName().textContent).toBe('Ivy - Hedera helix.jpg'));
        expect(imageName().title).toBe('/home/me/Nature/Ivy - Hedera helix.jpg');
        expect(imageRemove().hidden).toBe(false);
        expect(pillTexts()).toEqual(['Ivy', 'B']);
        expect(sent(form).plants.map((p) => p.image)).toEqual(['/home/me/Nature/Ivy - Hedera helix.jpg', '']);
        expect(onChange).toHaveBeenCalledTimes(1);

        // A cancelled dialog keeps the image.
        vi.mocked(ChoosePlantImage).mockResolvedValueOnce('');
        imageChoose().click();
        await vi.waitFor(() => expect(imageChoose().disabled).toBe(false));
        expect(imageName().textContent).toBe('Ivy - Hedera helix.jpg');
        expect(sent(form).plants[0].image).toBe('/home/me/Nature/Ivy - Hedera helix.jpg');
        expect(onChange).toHaveBeenCalledTimes(1);

        pills()[1].click();
        expect(imageName().textContent).toBe('No image');
        pills()[0].click();
        imageRemove().click();
        expect(imageName().textContent).toBe('No image');
        expect(imageRemove().hidden).toBe(true);
        expect(pillTexts()).toEqual(['Unnamed plant', 'B']);
        expect(sent(form).plants.map((p) => p.image)).toEqual(['', '']);
        expect(onChange).toHaveBeenCalledTimes(2);
    });

    it('sets the image of the plant selected when Choose… was clicked', async () => {
        const form = plantsForm(JSON.stringify({plants: [plant('A'), plant('B')], size: 'a6'}));
        const pending = deferred<string>();
        vi.mocked(ChoosePlantImage).mockReturnValueOnce(pending.promise);
        imageChoose().click();
        pills()[1].click();
        pending.resolve('/p/a.png');
        await vi.waitFor(() => expect(sent(form).plants[0].image).toBe('/p/a.png'));
        expect(imageName().textContent).toBe('No image');
    });

    it('shows an image dialog error in the status line', async () => {
        plantsForm(JSON.stringify({plants: [plant('A')], size: 'a6'}));
        vi.mocked(ChoosePlantImage).mockRejectedValueOnce(new Error('dialog failed'));
        imageChoose().click();
        await vi.waitFor(() => expect(status().textContent).toBe('dialog failed'));
        expect(status().classList.contains('error')).toBe(true);
        expect(status().getAttribute('role')).toBe('status');
    });

    it('drops an image picked after the form is disposed', async () => {
        const form = plantsForm(JSON.stringify({plants: [plant('A')], size: 'a6'}));
        const pending = deferred<string>();
        vi.mocked(ChoosePlantImage).mockReturnValueOnce(pending.promise);
        imageChoose().click();
        form.dispose?.();
        pending.resolve('/p/a.jpg');
        await settle();
        expect(sent(form).plants[0].image).toBe('');
        expect(imageName().textContent).toBe('No image');
        expect(onChange).not.toHaveBeenCalled();
    });

    it('sends the chosen card size', () => {
        const form = plantsForm('{"plants":[],"size":"4x6"}');
        expect(checkedSize()).toBe('4x6');
        radio('page').click();
        expect(onChange).toHaveBeenCalledTimes(1);
        expect(form.options()).toEqual({plants: [], size: 'page'});
    });

    it('labels its fields', () => {
        plantsForm(JSON.stringify({plants: [plant('A')], size: 'a6'}));
        expect(['name', 'light', 'water', 'soil', 'notes'].map(labelOf)).toEqual(['Name', 'Light', 'Water', 'Soil', 'Notes']);
        expect(field('notes')).toBeInstanceOf(HTMLTextAreaElement);
        expect(imageChoose().textContent).toBe('Choose…');
        expect(q(`#${imageChoose().parentElement!.getAttribute('aria-labelledby')}`).textContent).toBe('Image');
        expect(q(`#${q('#test-plants-list').getAttribute('aria-labelledby')}`).textContent).toBe('Plants');
    });

    it('follows a language change, keeping the plants, until disposed', () => {
        const form = plantsForm(JSON.stringify({plants: [plant(''), plant('Helecho')], size: '4x6'}));
        setLang('es');
        applyDom(box);
        expect(pillTexts()).toEqual(['Planta sin nombre', 'Helecho']);
        expect(removes()[0].getAttribute('aria-label')).toBe('Quitar planta: Planta sin nombre');
        expect(addButton().textContent).toBe('Añadir planta');
        expect(q('#test-plants-hint').textContent).toBe('Sin plantas se imprimen fichas en blanco para rellenar a mano.');
        expect(['name', 'light', 'water', 'soil', 'notes'].map(labelOf)).toEqual(['Nombre', 'Luz', 'Riego', 'Sustrato', 'Notas']);
        expect(field('name').getAttribute('placeholder')).toBe('Se toma del nombre del archivo de la imagen si está vacío');
        expect(imageName().textContent).toBe('Sin imagen');
        expect(imageChoose().textContent).toBe('Elegir…');
        expect(sizeLabels()).toEqual(['A6', '4 × 6 in', 'Página completa']);
        expect(checkedSize()).toBe('4x6');
        expect(sent(form).plants.length).toBe(2);

        form.dispose?.();
        setLang('en');
        expect(pillTexts()).toEqual(['Planta sin nombre', 'Helecho']);
    });
});

describe('plantList', () => {
    it('keeps objects, reading fields of the wrong type as empty', () => {
        expect(plantList([{name: 'A', light: 1}, null, 'x', {}])).toEqual([plant('A'), plant('')]);
        expect(plantList('nope')).toEqual([]);
    });
});
