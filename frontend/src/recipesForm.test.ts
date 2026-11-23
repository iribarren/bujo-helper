import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';

import {ChooseRecipePhoto, ImportRecipes} from '../wailsjs/go/main/App';
import {applyDom, setLang} from './i18n';
import {pageForms, type PageForm} from './pageForms';
import {parseDefaults} from './pages';
import {recipeLines} from './recipesForm';

vi.mock('../wailsjs/go/main/App', async (importOriginal) => {
    const original = await importOriginal<Record<string, unknown>>();
    return {...original, ImportRecipes: vi.fn(), ChooseRecipePhoto: vi.fn()};
});

// The recipes kind's defaults as PageKinds sends them (Go: sheet.RecipesKind).
const RECIPES_DEFAULTS = '{"recipes":[],"size":"a6"}';

// recipe is a recipe as Go sends and decodes it (Go: sheet.Recipe).
function recipe(title: string, extra: Partial<Record<string, unknown>> = {}) {
    return {title, servings: '', time: '', ingredients: [], steps: [], photo: '', ...extra};
}

// deferred is a promise resolved or rejected by hand, for the bindings.
function deferred<T>() {
    let resolve!: (v: T) => void;
    let reject!: (e: unknown) => void;
    const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return {promise, resolve, reject};
}

let box: HTMLDivElement;
let onChange: ReturnType<typeof vi.fn>;
let built: PageForm[] = [];

function recipesForm(defaults = RECIPES_DEFAULTS): PageForm {
    box = document.createElement('div');
    document.body.replaceChildren(box);
    onChange = vi.fn();
    const form = pageForms.recipes(box, 'test-recipes', parseDefaults(defaults), onChange);
    built.push(form);
    return form;
}

function q<T extends HTMLElement>(selector: string): T {
    const node = box.querySelector<T>(selector);
    if (!node) throw new Error(`missing ${selector}`);
    return node;
}

const items = () => Array.from(box.querySelectorAll<HTMLLIElement>('#test-recipes-list li'));
const pills = () => items().map((li) => li.querySelector<HTMLButtonElement>('.recipes-pick')!);
const removes = () => items().map((li) => li.querySelector<HTMLButtonElement>('.icon-button')!);
const pillTexts = () => pills().map((p) => p.textContent);
const selected = () => pills().findIndex((p) => p.getAttribute('aria-current') === 'true');
const addButton = () => q<HTMLButtonElement>('#test-recipes-add');
const importButton = () => q<HTMLButtonElement>('#test-recipes-import');
const status = () => q<HTMLElement>('#test-recipes-status');
const editor = () => q<HTMLElement>('#test-recipes-editor');
const field = (name: string) => q<HTMLInputElement | HTMLTextAreaElement>(`#test-recipes-${name}`);
const photoName = () => q<HTMLElement>('#test-recipes-photo-name');
const photoChoose = () => q<HTMLButtonElement>('#test-recipes-photo');
const photoRemove = () => q<HTMLButtonElement>('#test-recipes-photo-remove');

function type(name: string, text: string) {
    const node = field(name);
    node.value = text;
    node.dispatchEvent(new Event('input'));
}

function radio(value: string): HTMLButtonElement {
    return q<HTMLButtonElement>(`#test-recipes-size [role="radio"][data-value="${value}"]`);
}

function checkedSize(): string {
    return box.querySelector<HTMLElement>('#test-recipes-size [role="radio"][aria-checked="true"]')?.dataset.value ?? '';
}

function sent(form: PageForm) {
    return form.options() as {recipes: Array<Record<string, unknown>>; size: string};
}

// importing clicks Import… with ImportRecipes answering result, and waits
// until the import is over.
async function importing(result: unknown) {
    vi.mocked(ImportRecipes).mockResolvedValueOnce(result as never);
    importButton().click();
    await vi.waitFor(() => expect(importButton().disabled).toBe(false));
}

// settle lets pending promise callbacks run.
const settle = () => new Promise<void>((resolve) => setTimeout(resolve, 0));

beforeEach(() => {
    vi.mocked(ImportRecipes).mockReset();
    vi.mocked(ChooseRecipePhoto).mockReset();
});

afterEach(() => {
    for (const form of built) form.dispose?.();
    built = [];
    setLang('en');
});

describe('recipe cards form', () => {
    it('starts with no recipes, A6 cards and the blank cards hint', () => {
        const form = recipesForm();
        expect(items()).toEqual([]);
        expect(editor().hidden).toBe(true);
        expect(q('#test-recipes-hint').textContent).toBe('No recipes prints blank cards to fill in by hand.');
        expect(q('#test-recipes-hint').classList.contains('muted')).toBe(true);
        expect(addButton().textContent).toBe('Add recipe');
        expect(importButton().textContent).toBe('Import…');
        expect(Array.from(q('#test-recipes-size').children, (r) => r.textContent)).toEqual(['A6', '4 × 6 in', 'Full page']);
        expect(checkedSize()).toBe('a6');
        expect(form.options()).toEqual({recipes: [], size: 'a6'});
        expect(onChange).not.toHaveBeenCalled();
    });

    it('starts from the defaults, the first recipe selected, and falls back on an unknown size', () => {
        const form = recipesForm(JSON.stringify({
            recipes: [recipe('Soup', {servings: '4', ingredients: ['water', 'salt'], steps: ['Boil.']}), recipe(''), {title: 3}],
            size: 'huge',
        }));
        expect(pillTexts()).toEqual(['Soup', 'Untitled recipe']);
        expect(selected()).toBe(0);
        expect(field('title').value).toBe('Soup');
        expect(field('servings').value).toBe('4');
        expect(field('ingredients').value).toBe('water\nsalt');
        expect(field('steps').value).toBe('Boil.');
        expect(checkedSize()).toBe('a6');
        expect(sent(form).recipes).toEqual([
            recipe('Soup', {servings: '4', ingredients: ['water', 'salt'], steps: ['Boil.']}),
            recipe(''),
        ]);
    });

    it('adds a recipe, selects it and sends what is typed', () => {
        const form = recipesForm();
        addButton().click();
        expect(pillTexts()).toEqual(['Untitled recipe']);
        expect(selected()).toBe(0);
        expect(editor().hidden).toBe(false);
        expect(onChange).toHaveBeenCalledTimes(1);

        type('title', 'Pancakes');
        type('servings', '4');
        type('time', '20 min');
        type('ingredients', '  200 g flour \n\n2 eggs\r\n   \n');
        type('steps', 'Mix.\nFry.');
        expect(pillTexts()).toEqual(['Pancakes']);
        expect(onChange).toHaveBeenCalledTimes(6);
        expect(sent(form)).toEqual({
            recipes: [{title: 'Pancakes', servings: '4', time: '20 min', ingredients: ['200 g flour', '2 eggs'], steps: ['Mix.', 'Fry.'], photo: ''}],
            size: 'a6',
        });
        // The text stays as typed (the textarea normalises line endings).
        expect(field('ingredients').value).toBe('  200 g flour \n\n2 eggs\n   \n');
    });

    it('selects a recipe with a click and edits that one', () => {
        const form = recipesForm(JSON.stringify({recipes: [recipe('A'), recipe('B')], size: 'a6'}));
        pills()[1].click();
        expect(selected()).toBe(1);
        expect(field('title').value).toBe('B');
        type('title', 'Bread');
        expect(pillTexts()).toEqual(['A', 'Bread']);
        expect(sent(form).recipes.map((r) => r.title)).toEqual(['A', 'Bread']);

        pills()[0].click();
        expect(selected()).toBe(0);
        expect(field('title').value).toBe('A');
        // Selecting changes no option.
        expect(onChange).toHaveBeenCalledTimes(1);
    });

    // Keyboard selection is the browser's own: Tab reaches a native
    // <button type="button"> and Enter or Space activates it as a click.
    it('makes each recipe pill and remove button a focusable native button', () => {
        recipesForm(JSON.stringify({recipes: [recipe('A'), recipe('B')], size: 'a6'}));
        for (const b of [...pills(), ...removes()]) {
            expect(b).toBeInstanceOf(HTMLButtonElement);
            expect(b.type).toBe('button');
            expect(b.disabled).toBe(false);
            expect(b.tabIndex).toBe(0);
            b.focus();
            expect(document.activeElement).toBe(b);
        }
    });

    it('removes a recipe and selects its neighbour', () => {
        const form = recipesForm(JSON.stringify({recipes: [recipe('A'), recipe('B'), recipe('C')], size: 'a6'}));
        expect(removes().map((b) => b.getAttribute('aria-label'))).toEqual(['Remove recipe: A', 'Remove recipe: B', 'Remove recipe: C']);
        pills()[2].click();
        removes()[2].click();
        expect(pillTexts()).toEqual(['A', 'B']);
        expect(selected()).toBe(1);
        expect(field('title').value).toBe('B');
        removes()[0].click();
        expect(pillTexts()).toEqual(['B']);
        expect(selected()).toBe(0);
        removes()[0].click();
        expect(items()).toEqual([]);
        expect(editor().hidden).toBe(true);
        expect(sent(form).recipes).toEqual([]);
        expect(onChange).toHaveBeenCalledTimes(3);
    });

    it('names an untitled recipe in its remove button', () => {
        recipesForm();
        addButton().click();
        expect(removes()[0].getAttribute('aria-label')).toBe('Remove recipe: Untitled recipe');
    });

    it('imports recipes, appending them and selecting the first new one', async () => {
        const form = recipesForm(JSON.stringify({recipes: [recipe('Mine')], size: 'a6'}));
        await importing([recipe('Toast', {ingredients: ['bread']}), recipe('Tea', {photo: '/photos/tea.jpg'})]);
        expect(ImportRecipes).toHaveBeenCalledTimes(1);
        expect(pillTexts()).toEqual(['Mine', 'Toast', 'Tea']);
        expect(selected()).toBe(1);
        expect(field('title').value).toBe('Toast');
        expect(field('ingredients').value).toBe('bread');
        expect(status().textContent).toBe('');
        expect(onChange).toHaveBeenCalledTimes(1);
        expect(sent(form).recipes[2]).toEqual(recipe('Tea', {photo: '/photos/tea.jpg'}));
        pills()[2].click();
        expect(photoName().textContent).toBe('tea.jpg');
    });

    it('changes nothing when the import is cancelled', async () => {
        const form = recipesForm(JSON.stringify({recipes: [recipe('Mine')], size: 'a6'}));
        await importing(null);
        await importing([]);
        expect(pillTexts()).toEqual(['Mine']);
        expect(selected()).toBe(0);
        expect(status().textContent).toBe('');
        expect(onChange).not.toHaveBeenCalled();
        expect(sent(form).recipes).toEqual([recipe('Mine')]);
    });

    it('shows an import error in the status line until the next import', async () => {
        recipesForm();
        const pending = deferred<never>();
        vi.mocked(ImportRecipes).mockReturnValueOnce(pending.promise);
        importButton().click();
        expect(importButton().disabled).toBe(true);
        pending.reject('recipes.md: no recipes found');
        await vi.waitFor(() => expect(importButton().disabled).toBe(false));
        expect(status().textContent).toBe('recipes.md: no recipes found');
        expect(status().classList.contains('error')).toBe(true);
        expect(status().getAttribute('role')).toBe('status');
        expect(items()).toEqual([]);
        expect(onChange).not.toHaveBeenCalled();

        await importing([recipe('Toast')]);
        expect(status().textContent).toBe('');
        expect(status().classList.contains('error')).toBe(false);
    });

    it('chooses and removes the photo of the selected recipe', async () => {
        const form = recipesForm(JSON.stringify({recipes: [recipe('A'), recipe('B')], size: 'a6'}));
        expect(photoName().textContent).toBe('No photo');
        expect(photoRemove().hidden).toBe(true);

        vi.mocked(ChooseRecipePhoto).mockResolvedValueOnce('/home/me/photos/soup_1.png');
        photoChoose().click();
        await vi.waitFor(() => expect(photoName().textContent).toBe('soup_1.png'));
        expect(photoName().title).toBe('/home/me/photos/soup_1.png');
        expect(photoRemove().hidden).toBe(false);
        expect(sent(form).recipes.map((r) => r.photo)).toEqual(['/home/me/photos/soup_1.png', '']);
        expect(onChange).toHaveBeenCalledTimes(1);

        // A cancelled dialog keeps the photo.
        vi.mocked(ChooseRecipePhoto).mockResolvedValueOnce('');
        photoChoose().click();
        await vi.waitFor(() => expect(photoChoose().disabled).toBe(false));
        expect(photoName().textContent).toBe('soup_1.png');
        expect(onChange).toHaveBeenCalledTimes(1);

        pills()[1].click();
        expect(photoName().textContent).toBe('No photo');
        pills()[0].click();
        photoRemove().click();
        expect(photoName().textContent).toBe('No photo');
        expect(photoRemove().hidden).toBe(true);
        expect(sent(form).recipes.map((r) => r.photo)).toEqual(['', '']);
        expect(onChange).toHaveBeenCalledTimes(2);
    });

    it('sets the photo of the recipe selected when Choose… was clicked', async () => {
        const form = recipesForm(JSON.stringify({recipes: [recipe('A'), recipe('B')], size: 'a6'}));
        const pending = deferred<string>();
        vi.mocked(ChooseRecipePhoto).mockReturnValueOnce(pending.promise);
        photoChoose().click();
        pills()[1].click();
        pending.resolve('/p/a.jpg');
        await vi.waitFor(() => expect(sent(form).recipes[0].photo).toBe('/p/a.jpg'));
        expect(photoName().textContent).toBe('No photo');
    });

    it('shows a photo dialog error in the status line', async () => {
        recipesForm(JSON.stringify({recipes: [recipe('A')], size: 'a6'}));
        vi.mocked(ChooseRecipePhoto).mockRejectedValueOnce(new Error('dialog failed'));
        photoChoose().click();
        await vi.waitFor(() => expect(status().textContent).toBe('dialog failed'));
        expect(status().classList.contains('error')).toBe(true);
    });

    it('drops an import that ends after the form is disposed', async () => {
        const form = recipesForm(JSON.stringify({recipes: [recipe('Mine')], size: 'a6'}));
        const ok = deferred<unknown>();
        vi.mocked(ImportRecipes).mockReturnValueOnce(ok.promise as never);
        importButton().click();
        form.dispose?.();
        ok.resolve([recipe('Toast')]);
        await settle();
        expect(pillTexts()).toEqual(['Mine']);
        expect(sent(form).recipes).toEqual([recipe('Mine')]);
        expect(onChange).not.toHaveBeenCalled();
        // No DOM write: the button stays as it was left.
        expect(importButton().disabled).toBe(true);

        const failed = recipesForm();
        const bad = deferred<never>();
        vi.mocked(ImportRecipes).mockReturnValueOnce(bad.promise);
        importButton().click();
        failed.dispose?.();
        bad.reject('recipes.md: no recipes found');
        await settle();
        expect(status().textContent).toBe('');
        expect(importButton().disabled).toBe(true);
    });

    it('drops a photo picked after the form is disposed', async () => {
        const form = recipesForm(JSON.stringify({recipes: [recipe('A')], size: 'a6'}));
        const pending = deferred<string>();
        vi.mocked(ChooseRecipePhoto).mockReturnValueOnce(pending.promise);
        photoChoose().click();
        form.dispose?.();
        pending.resolve('/p/a.jpg');
        await settle();
        expect(sent(form).recipes[0].photo).toBe('');
        expect(photoName().textContent).toBe('No photo');
        expect(onChange).not.toHaveBeenCalled();
        expect(photoChoose().disabled).toBe(true);
    });

    it('drops a photo picked for a recipe removed meanwhile, keeping it for one still there', async () => {
        const form = recipesForm(JSON.stringify({recipes: [recipe('A'), recipe('B'), recipe('C')], size: 'a6'}));
        const gone = deferred<string>();
        vi.mocked(ChooseRecipePhoto).mockReturnValueOnce(gone.promise);
        photoChoose().click();
        removes()[0].click();
        expect(onChange).toHaveBeenCalledTimes(1);
        gone.resolve('/p/a.jpg');
        await vi.waitFor(() => expect(photoChoose().disabled).toBe(false));
        expect(sent(form).recipes.map((r) => r.photo)).toEqual(['', '']);
        expect(photoName().textContent).toBe('No photo');
        expect(onChange).toHaveBeenCalledTimes(1);

        // Positive control: removing another recipe keeps the pick.
        const kept = deferred<string>();
        vi.mocked(ChooseRecipePhoto).mockReturnValueOnce(kept.promise);
        photoChoose().click();
        removes()[1].click();
        kept.resolve('/p/b.jpg');
        await vi.waitFor(() => expect(sent(form).recipes.map((r) => r.photo)).toEqual(['/p/b.jpg']));
        expect(photoName().textContent).toBe('b.jpg');
        expect(onChange).toHaveBeenCalledTimes(3);
    });

    it('sends the chosen card size', () => {
        const form = recipesForm('{"recipes":[],"size":"4x6"}');
        expect(checkedSize()).toBe('4x6');
        radio('page').click();
        expect(onChange).toHaveBeenCalledTimes(1);
        expect(form.options()).toEqual({recipes: [], size: 'page'});
    });

    it('labels its fields and describes the line lists', () => {
        recipesForm(JSON.stringify({recipes: [recipe('A')], size: 'a6'}));
        const labelOf = (name: string) => box.querySelector(`label[for="${field(name).id}"]`)?.textContent;
        expect(['title', 'servings', 'time', 'ingredients', 'steps'].map(labelOf))
            .toEqual(['Title', 'Serves', 'Time', 'Ingredients', 'Steps']);
        for (const name of ['ingredients', 'steps']) {
            expect(document.getElementById(field(name).getAttribute('aria-describedby') ?? '')?.textContent).toBe('One per line.');
        }
        expect(q(`#${q('#test-recipes-list').getAttribute('aria-labelledby')}`).textContent).toBe('Recipes');
    });

    it('follows a language change, keeping the recipes, until disposed', () => {
        const form = recipesForm(JSON.stringify({recipes: [recipe(''), recipe('Sopa')], size: '4x6'}));
        setLang('es');
        applyDom(box);
        expect(pillTexts()).toEqual(['Receta sin título', 'Sopa']);
        expect(removes()[0].getAttribute('aria-label')).toBe('Quitar receta: Receta sin título');
        expect(addButton().textContent).toBe('Añadir receta');
        expect(importButton().textContent).toBe('Importar…');
        expect(q('#test-recipes-hint').textContent).toBe('Sin recetas se imprimen fichas en blanco para rellenar a mano.');
        expect(box.querySelector(`label[for="${field('servings').id}"]`)?.textContent).toBe('Raciones');
        expect(box.querySelector(`label[for="${field('ingredients').id}"]`)?.textContent).toBe('Ingredientes');
        expect(box.querySelector(`label[for="${field('steps').id}"]`)?.textContent).toBe('Pasos');
        expect(photoName().textContent).toBe('Sin foto');
        expect(photoChoose().textContent).toBe('Elegir…');
        expect(Array.from(q('#test-recipes-size').children, (r) => r.textContent)).toEqual(['A6', '4 × 6 in', 'Página completa']);
        expect(checkedSize()).toBe('4x6');
        expect(sent(form).recipes.length).toBe(2);

        form.dispose?.();
        setLang('en');
        expect(pillTexts()).toEqual(['Receta sin título', 'Sopa']);
    });
});

describe('kitchen conversions form', () => {
    it('has no options and says what the chart holds', () => {
        box = document.createElement('div');
        document.body.replaceChildren(box);
        const change = vi.fn();
        const form = pageForms.conversions(box, 'test-conversions', {}, change);
        built.push(form);
        const hint = q('#test-conversions-hint');
        expect(hint.textContent).toBe('A ready-made chart of volume, weight, oven temperatures and cups to grams.');
        expect(hint.classList.contains('muted')).toBe(true);
        expect(form.options()).toEqual({});
        setLang('es');
        applyDom(box);
        expect(hint.textContent).toBe('Una tabla lista para usar de volumen, peso, temperaturas del horno y tazas a gramos.');
        expect(change).not.toHaveBeenCalled();
    });
});

describe('recipeLines', () => {
    it.each([
        ['one\ntwo', ['one', 'two']],
        ['  a  \r\n\r\n b\rc ', ['a', 'b', 'c']],
        ['\n \n\t\n', []],
        ['', []],
    ])('reads %j', (text, want) => {
        expect(recipeLines(text)).toEqual(want);
    });
});
