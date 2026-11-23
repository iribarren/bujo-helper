// The recipe list and editor of the recipe cards form (pageForms.ts): one
// button per recipe, with a remove button, Add recipe and Import… (a
// Markdown file, see ImportRecipes), and the selected recipe's fields:
// Title, Serves, Time, Ingredients and Steps (one per line) and its photo
// (ChooseRecipePhoto). Also recipeLines, the typed lists.
//
// The photo shows as its file name: the thumbnail handler only serves the
// covers of a listing, not any local file.

import {ChooseRecipePhoto, ImportRecipes} from '../wailsjs/go/main/App';
import {translated} from './fieldsets';
import {onLangChange, t, type MessageKey} from './i18n';

// Recipe is one recipe as Go decodes it (Go: sheet.Recipe).
export interface Recipe {
    title: string;
    servings: string;
    time: string;
    ingredients: string[];
    steps: string[];
    // The absolute path of the user's photo, "" for none.
    photo: string;
}

// Draft is a recipe being edited: its lists kept as typed.
interface Draft {
    title: string;
    servings: string;
    time: string;
    ingredients: string;
    steps: string;
    photo: string;
}

// recipeLines is the items typed in text, one per line: trimmed, blank
// lines dropped.
export function recipeLines(text: string): string[] {
    return text.split(/\r\n|\r|\n/).map((s) => s.trim()).filter((s) => s !== '');
}

const isString = (v: unknown): v is string => typeof v === 'string';

// recipeList is the well-formed recipes of a kind's defaults or of an
// import: a recipe needs a string title; other fields of the wrong type
// read as empty.
export function recipeList(value: unknown): Recipe[] {
    if (!Array.isArray(value)) return [];
    return value.filter((r) => isString(r?.title)).map((r) => ({
        title: r.title,
        servings: isString(r.servings) ? r.servings : '',
        time: isString(r.time) ? r.time : '',
        ingredients: Array.isArray(r.ingredients) ? r.ingredients.filter(isString) : [],
        steps: Array.isArray(r.steps) ? r.steps.filter(isString) : [],
        photo: isString(r.photo) ? r.photo : '',
    }));
}

function draftOf(r: Recipe): Draft {
    return {...r, ingredients: r.ingredients.join('\n'), steps: r.steps.join('\n')};
}

function recipeOf(d: Draft): Recipe {
    return {
        title: d.title,
        servings: d.servings,
        time: d.time,
        ingredients: recipeLines(d.ingredients),
        steps: recipeLines(d.steps),
        photo: d.photo,
    };
}

function errorText(err: unknown): string {
    if (err instanceof Error) return err.message;
    return String(err);
}

function baseName(path: string): string {
    return path.split(/[\\/]/).pop() || path;
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, className = ''): HTMLElementTagNameMap[K] {
    const node = document.createElement(tag);
    if (className) node.className = className;
    return node;
}

function button(id: string, key: MessageKey, className = ''): HTMLButtonElement {
    const node = translated(el('button', className), key);
    node.type = 'button';
    node.id = id;
    return node;
}

export interface RecipeEditor {
    // The recipes as Go decodes them, in list order.
    recipes: () => Recipe[];
    dispose: () => void;
}

// recipeEditor appends the recipe list and editor to container, with
// element IDs prefixed by idPrefix, starting with the recipes initial, the
// first one selected. onChange is called when a recipe is added, removed,
// imported or edited; selecting one changes nothing.
export function recipeEditor(container: HTMLElement, idPrefix: string, initial: Recipe[], onChange: () => void): RecipeEditor {
    // Recipes: the list, Add recipe and Import…, the status and the hint.
    const listField = el('div', 'field recipes-list-field');
    const listLabel = translated(el('span'), 'pages.recipesList');
    listLabel.id = `${idPrefix}-list-label`;
    const list = el('ul', 'recipes-list');
    list.id = `${idPrefix}-list`;
    list.setAttribute('aria-labelledby', listLabel.id);
    const actions = el('div', 'row recipes-actions');
    const add = button(`${idPrefix}-add`, 'pages.recipesAdd');
    const importButton = button(`${idPrefix}-import`, 'pages.recipesImport');
    importButton.dataset.i18nTitle = 'pages.recipesImportTitle';
    importButton.title = t('pages.recipesImportTitle');
    actions.append(add, importButton);
    const status = el('p', 'field-hint recipes-status');
    status.id = `${idPrefix}-status`;
    status.setAttribute('role', 'status');
    const hint = translated(el('p', 'field-hint muted'), 'pages.recipesHint');
    hint.id = `${idPrefix}-hint`;
    listField.append(listLabel, list, actions, status, hint);

    // The selected recipe's fields.
    const editor = el('div', 'recipes-editor');
    editor.id = `${idPrefix}-editor`;

    function textField(name: keyof Draft, key: MessageKey, multiline: boolean) {
        const field = el('div', `field recipes-${name}-field`);
        const label = el('label');
        label.htmlFor = `${idPrefix}-${name}`;
        label.append(translated(el('span'), key));
        const input = multiline ? el('textarea') : el('input');
        input.id = `${idPrefix}-${name}`;
        if (input instanceof HTMLTextAreaElement) {
            input.rows = 4;
            const lines = translated(el('p', 'field-hint muted'), 'pages.recipesLinesHint');
            lines.id = `${idPrefix}-${name}-hint`;
            input.setAttribute('aria-describedby', lines.id);
            field.append(label, input, lines);
        } else {
            input.type = 'text';
            field.append(label, input);
        }
        input.addEventListener('input', () => {
            if (!current) return;
            current[name] = input.value;
            if (name === 'title') renderList();
            onChange();
        });
        return {field, input};
    }
    const title = textField('title', 'pages.recipesTitle', false);
    const servings = textField('servings', 'pages.recipesServings', false);
    const time = textField('time', 'pages.recipesTime', false);
    const ingredients = textField('ingredients', 'pages.recipesIngredients', true);
    const steps = textField('steps', 'pages.recipesSteps', true);
    const pair = el('div', 'field-pair');
    pair.append(servings.field, time.field);

    const photoField = el('div', 'field recipes-photo-field');
    const photoLabel = translated(el('span'), 'pages.recipesPhoto');
    photoLabel.id = `${idPrefix}-photo-label`;
    const photoRow = el('div', 'row recipes-photo-row');
    photoRow.setAttribute('role', 'group');
    photoRow.setAttribute('aria-labelledby', photoLabel.id);
    const choose = button(`${idPrefix}-photo`, 'pages.recipesPhotoChoose');
    const photoName = el('span', 'recipes-photo-name');
    photoName.id = `${idPrefix}-photo-name`;
    const removePhoto = button(`${idPrefix}-photo-remove`, 'pages.recipesPhotoRemove', 'link-button');
    photoRow.append(choose, photoName, removePhoto);
    photoField.append(photoLabel, photoRow);

    editor.append(title.field, pair, ingredients.field, steps.field, photoField);
    container.append(listField, editor);

    const drafts: Draft[] = initial.map(draftOf);
    let current: Draft | undefined = drafts[0];

    const nameOf = (d: Draft) => d.title.trim() || t('pages.recipesUntitled');

    function setStatus(error = '') {
        status.textContent = error;
        status.classList.toggle('error', error !== '');
    }

    // renderList shows a button per recipe, the selected one current, in
    // the current language.
    function renderList() {
        list.replaceChildren(...drafts.map((d) => {
            const item = el('li', 'recipes-item');
            const pick = el('button', 'recipes-pick');
            pick.type = 'button';
            pick.textContent = nameOf(d);
            if (d === current) pick.setAttribute('aria-current', 'true');
            pick.addEventListener('click', () => select(d));
            const remove = el('button', 'icon-button');
            remove.type = 'button';
            remove.textContent = '×';
            remove.setAttribute('aria-label', t('pages.recipesRemove', {title: nameOf(d)}));
            remove.title = remove.getAttribute('aria-label') ?? '';
            remove.addEventListener('click', () => removeDraft(d));
            item.append(pick, remove);
            return item;
        }));
    }

    function renderPhoto() {
        const path = current?.photo ?? '';
        photoName.textContent = path ? baseName(path) : t('pages.recipesPhotoNone');
        photoName.title = path;
        photoName.classList.toggle('muted', path === '');
        removePhoto.hidden = path === '';
    }

    // renderEditor fills the fields from the selected recipe.
    function renderEditor() {
        editor.hidden = !current;
        if (!current) return;
        title.input.value = current.title;
        servings.input.value = current.servings;
        time.input.value = current.time;
        ingredients.input.value = current.ingredients;
        steps.input.value = current.steps;
        renderPhoto();
    }

    function select(d: Draft | undefined) {
        current = d;
        renderList();
        renderEditor();
    }

    function removeDraft(d: Draft) {
        const i = drafts.indexOf(d);
        if (i < 0) return;
        drafts.splice(i, 1);
        if (current === d) current = drafts[Math.min(i, drafts.length - 1)];
        select(current);
        onChange();
        (list.querySelector<HTMLElement>('[aria-current="true"]') ?? add).focus();
    }

    add.addEventListener('click', () => {
        const d = draftOf({title: '', servings: '', time: '', ingredients: [], steps: [], photo: ''});
        drafts.push(d);
        select(d);
        onChange();
        title.input.focus();
    });

    importButton.addEventListener('click', async () => {
        importButton.disabled = true;
        try {
            const imported = recipeList(await ImportRecipes());
            setStatus();
            if (imported.length === 0) return;
            const added = imported.map(draftOf);
            drafts.push(...added);
            select(added[0]);
            onChange();
        } catch (err) {
            setStatus(errorText(err));
        } finally {
            importButton.disabled = false;
        }
    });

    choose.addEventListener('click', async () => {
        // The photo is the recipe's that was selected when Choose… was
        // clicked, even if another one is selected by the time it is picked.
        const target = current;
        if (!target) return;
        choose.disabled = true;
        try {
            const path = await ChooseRecipePhoto();
            setStatus();
            if (!path || path === target.photo) return;
            target.photo = path;
            renderPhoto();
            onChange();
        } catch (err) {
            setStatus(errorText(err));
        } finally {
            choose.disabled = false;
        }
    });

    removePhoto.addEventListener('click', () => {
        if (!current || current.photo === '') return;
        current.photo = '';
        renderPhoto();
        onChange();
        choose.focus();
    });

    select(current);
    const stopRelabel = onLangChange(() => {
        renderList();
        renderPhoto();
    });

    return {
        recipes: () => drafts.map(recipeOf),
        dispose: stopRelabel,
    };
}
