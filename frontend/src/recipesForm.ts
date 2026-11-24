// The recipe list and editor of the recipe cards form (pageForms.ts): one
// button per recipe, with a remove button, Add recipe and Import… (a
// Markdown file, see ImportRecipes), and the selected recipe's fields:
// Title, Serves, Time, Ingredients and Steps (one per line) and its photo
// (ChooseRecipePhoto). Also recipeLines, the typed lists.
//
// The card list (cardList), its text fields (cardTextField) and its image
// picker (imagePicker) are shared with the plant care cards form
// (plantsForm.ts).
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

export const isString = (v: unknown): v is string => typeof v === 'string';

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

// baseName is the file name of path, directory dropped.
export function baseName(path: string): string {
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

// CardListKeys are the message keys of a card list (see cardList).
export interface CardListKeys {
    list: MessageKey; // the list's caption
    untitled: MessageKey; // the button of a card with no name
    remove: MessageKey; // a remove button's label, with the card's {title}
    add: MessageKey; // the Add button
    hint: MessageKey; // the muted line under the list
}

export interface CardListConfig<T> {
    // Element IDs are "<idPrefix>-list", "<idPrefix>-add", …; class names
    // "<className>-list", "<className>-pick", … (style.css).
    idPrefix: string;
    className: string;
    keys: CardListKeys;
    // The cards at first, in list order; none is selected until select.
    initial: T[];
    // blank is the card Add appends.
    blank: () => T;
    // name is the card's name in its button, "" for the untitled text.
    name: (card: T) => string;
    // render fills the editor's fields from the selected card.
    render: (card: T) => void;
    // onAdd is called after Add appended and selected a blank card.
    onAdd: () => void;
    // onChange is called when a card is added, removed or edited.
    onChange: () => void;
}

export interface CardList<T> {
    idPrefix: string;
    className: string;
    // The cards, in list order. Change them through the list only.
    cards: readonly T[];
    // The row of Add, for more buttons; the selected card's fields, hidden
    // when none is selected.
    actions: HTMLElement;
    editor: HTMLElement;
    current: () => T | undefined;
    // select selects card (none for undefined) and fills the editor.
    select: (card: T | undefined) => void;
    // append adds cards at the end, selecting the first, and calls onChange.
    append: (cards: T[]) => void;
    // renderList redraws the card buttons, e.g. when a name changes.
    renderList: () => void;
    // setStatus shows error in the status line, or clears it.
    setStatus: (error?: string) => void;
    onChange: () => void;
    // isDisposed is set by dispose: a dialog that answers afterwards is
    // dropped, touching neither the cards nor the page.
    isDisposed: () => boolean;
    dispose: () => void;
}

// cardList appends to container a list of cards, one button each (the
// selected one current) with a remove button, Add, a status line and a
// muted hint; then the editor of the selected card, which the caller fills.
// Selecting a card changes nothing; adding, removing or editing one calls
// onChange. The buttons follow the UI language.
export function cardList<T>(container: HTMLElement, config: CardListConfig<T>): CardList<T> {
    const {idPrefix, className, keys} = config;
    const listField = el('div', `field ${className}-list-field`);
    const listLabel = translated(el('span'), keys.list);
    listLabel.id = `${idPrefix}-list-label`;
    const list = el('ul', `${className}-list`);
    list.id = `${idPrefix}-list`;
    list.setAttribute('aria-labelledby', listLabel.id);
    const actions = el('div', `row ${className}-actions`);
    const add = button(`${idPrefix}-add`, keys.add);
    actions.append(add);
    const status = el('p', `field-hint ${className}-status`);
    status.id = `${idPrefix}-status`;
    status.setAttribute('role', 'status');
    const hint = translated(el('p', 'field-hint muted'), keys.hint);
    hint.id = `${idPrefix}-hint`;
    listField.append(listLabel, list, actions, status, hint);
    const editor = el('div', `${className}-editor`);
    editor.id = `${idPrefix}-editor`;
    container.append(listField, editor);

    const cards: T[] = [...config.initial];
    let current: T | undefined;
    let disposed = false;

    const nameOf = (card: T) => config.name(card) || t(keys.untitled);

    function renderList() {
        list.replaceChildren(...cards.map((card) => {
            const item = el('li', `${className}-item`);
            const pick = el('button', `${className}-pick`);
            pick.type = 'button';
            pick.textContent = nameOf(card);
            if (card === current) pick.setAttribute('aria-current', 'true');
            pick.addEventListener('click', () => select(card));
            const remove = el('button', 'icon-button');
            remove.type = 'button';
            remove.textContent = '×';
            remove.setAttribute('aria-label', t(keys.remove, {title: nameOf(card)}));
            remove.title = remove.getAttribute('aria-label') ?? '';
            remove.addEventListener('click', () => removeCard(card));
            item.append(pick, remove);
            return item;
        }));
    }

    function select(card: T | undefined) {
        current = card;
        renderList();
        editor.hidden = !current;
        if (current) config.render(current);
    }

    function removeCard(card: T) {
        const i = cards.indexOf(card);
        if (i < 0) return;
        cards.splice(i, 1);
        if (current === card) current = cards[Math.min(i, cards.length - 1)];
        select(current);
        config.onChange();
        (list.querySelector<HTMLElement>('[aria-current="true"]') ?? add).focus();
    }

    function append(added: T[]) {
        cards.push(...added);
        select(added[0]);
        config.onChange();
    }

    add.addEventListener('click', () => {
        append([config.blank()]);
        config.onAdd();
    });

    const stopRelabel = onLangChange(renderList);

    return {
        idPrefix,
        className,
        cards,
        actions,
        editor,
        current: () => current,
        select,
        append,
        renderList,
        setStatus: (error = '') => {
            status.textContent = error;
            status.classList.toggle('error', error !== '');
        },
        onChange: config.onChange,
        isDisposed: () => disposed,
        dispose: () => {
            disposed = true;
            stopRelabel();
        },
    };
}

export interface CardTextOptions {
    // rows makes the field a textarea of that many rows.
    rows?: number;
    // hint is a muted line under the field that describes it.
    hint?: MessageKey;
    // placeholder is shown in the field while it is empty.
    placeholder?: MessageKey;
    // renames redraws the card buttons on input: the field is the name.
    renames?: boolean;
}

// cardTextField is a labelled text field, "<idPrefix>-<name>", for field
// name of the selected card of list: input sets it, as typed, and calls
// onChange.
export function cardTextField<K extends string, T extends Record<K, string>>(
    list: CardList<T>, name: K, key: MessageKey, options: CardTextOptions = {},
) {
    const id = `${list.idPrefix}-${name}`;
    const field = el('div', `field ${list.className}-${name}-field`);
    const label = el('label');
    label.htmlFor = id;
    label.append(translated(el('span'), key));
    const input = options.rows ? el('textarea') : el('input');
    input.id = id;
    if (input instanceof HTMLTextAreaElement) input.rows = options.rows ?? 0;
    else input.type = 'text';
    if (options.placeholder) {
        input.dataset.i18nPlaceholder = options.placeholder;
        input.placeholder = t(options.placeholder);
    }
    field.append(label, input);
    if (options.hint) {
        const hint = translated(el('p', 'field-hint muted'), options.hint);
        hint.id = `${id}-hint`;
        input.setAttribute('aria-describedby', hint.id);
        field.append(hint);
    }
    input.addEventListener('input', () => {
        const card = list.current();
        if (!card) return;
        (card as Record<K, string>)[name] = input.value;
        if (options.renames) list.renderList();
        list.onChange();
    });
    return {field, input};
}

// ImageKeys are the message keys of an image picker (see imagePicker).
export interface ImageKeys {
    label: MessageKey; // the field's caption
    choose: MessageKey; // the button that opens the dialog
    none: MessageKey; // shown for no image
    remove: MessageKey; // the button that takes the image off
}

export interface ImagePicker {
    field: HTMLElement;
    // render shows the selected card's image.
    render: () => void;
    dispose: () => void;
}

// imagePicker is the image field name of the selected card of list, an
// absolute path or "": a caption and a row of Choose… ("<idPrefix>-<name>"),
// the file name (the path as tooltip, the none text for none) and Remove.
// Choose… sets the image picked in dialog on the card selected when it was
// clicked; a cancelled dialog, a card removed meanwhile or a disposed list
// changes nothing, and a dialog error shows in the list's status line.
export function imagePicker<K extends string, T extends Record<K, string>>(
    list: CardList<T>, name: K, keys: ImageKeys, dialog: () => Promise<string>,
): ImagePicker {
    const id = `${list.idPrefix}-${name}`;
    const field = el('div', `field ${list.className}-${name}-field`);
    const caption = translated(el('span'), keys.label);
    caption.id = `${id}-label`;
    const row = el('div', `row ${list.className}-${name}-row`);
    row.setAttribute('role', 'group');
    row.setAttribute('aria-labelledby', caption.id);
    const choose = button(id, keys.choose);
    const fileName = el('span', `${list.className}-${name}-name`);
    fileName.id = `${id}-name`;
    const remove = button(`${id}-remove`, keys.remove, 'link-button');
    row.append(choose, fileName, remove);
    field.append(caption, row);

    function render() {
        const path = list.current()?.[name] ?? '';
        fileName.textContent = path ? baseName(path) : t(keys.none);
        fileName.title = path;
        fileName.classList.toggle('muted', path === '');
        remove.hidden = path === '';
    }

    function set(card: T, path: string) {
        (card as Record<K, string>)[name] = path;
        list.renderList();
        render();
        list.onChange();
    }

    choose.addEventListener('click', async () => {
        const target = list.current();
        if (!target) return;
        choose.disabled = true;
        try {
            const path = await dialog();
            if (list.isDisposed()) return;
            list.setStatus();
            if (!path || path === target[name] || !list.cards.includes(target)) return;
            set(target, path);
        } catch (err) {
            if (!list.isDisposed()) list.setStatus(errorText(err));
        } finally {
            if (!list.isDisposed()) choose.disabled = false;
        }
    });

    remove.addEventListener('click', () => {
        const card = list.current();
        if (!card || card[name] === '') return;
        set(card, '');
        choose.focus();
    });

    return {field, render, dispose: onLangChange(render)};
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
    const list: CardList<Draft> = cardList(container, {
        idPrefix,
        className: 'recipes',
        keys: {
            list: 'pages.recipesList',
            untitled: 'pages.recipesUntitled',
            remove: 'pages.recipesRemove',
            add: 'pages.recipesAdd',
            hint: 'pages.recipesHint',
        },
        initial: initial.map(draftOf),
        blank: () => draftOf({title: '', servings: '', time: '', ingredients: [], steps: [], photo: ''}),
        name: (d) => d.title.trim(),
        render: (d) => {
            title.input.value = d.title;
            servings.input.value = d.servings;
            time.input.value = d.time;
            ingredients.input.value = d.ingredients;
            steps.input.value = d.steps;
            photo.render();
        },
        onAdd: () => title.input.focus(),
        onChange,
    });

    const importButton = button(`${idPrefix}-import`, 'pages.recipesImport');
    importButton.dataset.i18nTitle = 'pages.recipesImportTitle';
    importButton.title = t('pages.recipesImportTitle');
    list.actions.append(importButton);

    // The selected recipe's fields.
    const lines = {rows: 4, hint: 'pages.recipesLinesHint'} as const;
    const title = cardTextField(list, 'title', 'pages.recipesTitle', {renames: true});
    const servings = cardTextField(list, 'servings', 'pages.recipesServings');
    const time = cardTextField(list, 'time', 'pages.recipesTime');
    const ingredients = cardTextField(list, 'ingredients', 'pages.recipesIngredients', lines);
    const steps = cardTextField(list, 'steps', 'pages.recipesSteps', lines);
    const pair = el('div', 'field-pair');
    pair.append(servings.field, time.field);
    const photo = imagePicker(list, 'photo', {
        label: 'pages.recipesPhoto',
        choose: 'pages.recipesPhotoChoose',
        none: 'pages.recipesPhotoNone',
        remove: 'pages.recipesPhotoRemove',
    }, ChooseRecipePhoto);
    list.editor.append(title.field, pair, ingredients.field, steps.field, photo.field);

    importButton.addEventListener('click', async () => {
        importButton.disabled = true;
        try {
            const result = await ImportRecipes();
            if (list.isDisposed()) return;
            const imported = recipeList(result);
            list.setStatus();
            if (imported.length > 0) list.append(imported.map(draftOf));
        } catch (err) {
            if (!list.isDisposed()) list.setStatus(errorText(err));
        } finally {
            if (!list.isDisposed()) importButton.disabled = false;
        }
    });

    list.select(list.cards[0]);

    return {
        recipes: () => list.cards.map(recipeOf),
        dispose: () => {
            list.dispose();
            photo.dispose();
        },
    };
}
